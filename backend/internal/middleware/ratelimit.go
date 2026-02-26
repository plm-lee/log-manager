package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter 限流器结构体
// 使用令牌桶算法实现简单的限流
type RateLimiter struct {
	rate       int        // 每秒允许的请求数
	capacity   int        // 桶容量
	tokens     int        // 当前令牌数
	lastUpdate time.Time  // 上次更新时间
	mu         sync.Mutex // 保护令牌数的互斥锁
}

// NewRateLimiter 创建限流器
// rate: 每秒允许的请求数
// capacity: 桶容量（可选，默认为 rate）
// 返回: RateLimiter 实例
func NewRateLimiter(rate int, capacity int) *RateLimiter {
	if capacity <= 0 {
		capacity = rate
	}
	return &RateLimiter{
		rate:       rate,
		capacity:   capacity,
		tokens:     capacity,
		lastUpdate: time.Now(),
	}
}

// Allow 检查是否允许请求
// 返回: 是否允许
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	// 计算应该添加的令牌数（基于时间差）
	elapsed := now.Sub(rl.lastUpdate)
	tokensToAdd := int(elapsed.Seconds() * float64(rl.rate))

	if tokensToAdd > 0 {
		rl.tokens = rl.tokens + tokensToAdd
		if rl.tokens > rl.capacity {
			rl.tokens = rl.capacity
		}
		rl.lastUpdate = now
	}

	// 检查是否有可用令牌
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

// RateLimitMiddleware 限流中间件
// rate: 每秒允许的请求数
// capacity: 桶容量（可选）
// 返回: Gin 中间件函数
func RateLimitMiddleware(rate int, capacity int) gin.HandlerFunc {
	limiter := NewRateLimiter(rate, capacity)

	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "请求过于频繁",
				"message": "请稍后再试",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// DualRateLimitMiddleware 双轨限流中间件
// 对 /logs/batch、/metrics/batch 使用更高限额，其他 API 使用默认限额
func DualRateLimitMiddleware(rate, capacity, batchRate, batchCapacity int) gin.HandlerFunc {
	defaultLimiter := NewRateLimiter(rate, capacity)
	batchLimiter := NewRateLimiter(batchRate, batchCapacity)

	return func(c *gin.Context) {
		path := c.Request.URL.Path
		useBatch := strings.HasSuffix(path, "/api/v1/logs/batch") || strings.HasSuffix(path, "/api/v1/metrics/batch")
		limiter := defaultLimiter
		if useBatch {
			limiter = batchLimiter
		}
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "请求过于频繁",
				"message": "请稍后再试",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
