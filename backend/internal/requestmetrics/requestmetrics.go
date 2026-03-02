package requestmetrics

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 只统计日志/指标上报接口（path 以这些结尾）
var trackedSuffixes = []string{"/logs", "/logs/batch", "/metrics", "/metrics/batch"}

type entry struct {
	ts int64
	ms float64
}

const windowSec = 60
const maxEntries = 10000

var (
	mu      sync.RWMutex
	entries []entry
)

func isTracked(path string) bool {
	for _, suf := range trackedSuffixes {
		if len(path) >= len(suf) && path[len(path)-len(suf):] == suf {
			return true
		}
	}
	return false
}

// Record 记录一次请求
func Record(path string, latencyMs float64) {
	if !isTracked(path) {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	now := time.Now().Unix()
	entries = append(entries, entry{ts: now, ms: latencyMs})
	// 淘汰 60 秒前的
	cutoff := now - windowSec
	i := 0
	for i < len(entries) && entries[i].ts < cutoff {
		i++
	}
	if i > 0 {
		entries = append(entries[:0], entries[i:]...)
	}
	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}
}

// RequestsLastMinute 近 1 分钟请求数
func RequestsLastMinute() int {
	mu.RLock()
	defer mu.RUnlock()
	cutoff := time.Now().Unix() - windowSec
	n := 0
	for _, e := range entries {
		if e.ts >= cutoff {
			n++
		}
	}
	return n
}

// AvgLatencyMs 近 1 分钟平均耗时（ms）
func AvgLatencyMs() float64 {
	mu.RLock()
	defer mu.RUnlock()
	cutoff := time.Now().Unix() - windowSec
	var sum float64
	n := 0
	for _, e := range entries {
		if e.ts >= cutoff {
			sum += e.ms
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// Middleware 请求指标中间件
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		latencyMs := float64(time.Since(start).Microseconds()) / 1000
		Record(path, latencyMs)
	}
}
