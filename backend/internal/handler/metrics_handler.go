package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"log-manager/internal/database"
	"log-manager/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// MetricsHandler 指标处理器
// 负责处理指标相关的 HTTP 请求
type MetricsHandler struct {
	db *gorm.DB
}

// NewMetricsHandler 创建指标处理器实例
// 返回: MetricsHandler 实例
func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{
		db: database.DB,
	}
}

// ReceiveMetricsRequest 接收指标请求结构体
// 对应 log-filter-monitor 上报的指标数据格式
type ReceiveMetricsRequest struct {
	Timestamp  int64            `json:"timestamp" binding:"required"`   // 时间戳
	RuleCounts map[string]int64 `json:"rule_counts" binding:"required"` // 规则计数
	TotalCount int64            `json:"total_count" binding:"required"` // 总计数
	Duration   int64            `json:"duration" binding:"required"`    // 统计时长（秒）
	Tag        string           `json:"tag"`                             // 标签
}

// ReceiveMetrics 接收指标数据
// 处理来自 log-filter-monitor 的指标上报请求
func (h *MetricsHandler) ReceiveMetrics(c *gin.Context) {
	var req ReceiveMetricsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}

	// 将规则计数序列化为 JSON
	ruleCountsJSON, err := json.Marshal(req.RuleCounts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "序列化规则计数失败",
			"message": err.Error(),
		})
		return
	}

	// 创建指标条目
	metricsEntry := models.MetricsEntry{
		Timestamp:  req.Timestamp,
		RuleCounts: string(ruleCountsJSON),
		TotalCount: req.TotalCount,
		Duration:   req.Duration,
		Tag:        req.Tag,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// 保存到数据库
	if err := h.db.Create(&metricsEntry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "保存指标失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"id":      metricsEntry.ID,
	})
}

// QueryMetricsRequest 查询指标请求结构体
// 定义指标查询的筛选条件
type QueryMetricsRequest struct {
	Tag       string `form:"tag"`        // 标签筛选
	StartTime int64  `form:"start_time"` // 开始时间戳
	EndTime   int64  `form:"end_time"`   // 结束时间戳
	Page      int    `form:"page"`       // 页码（从1开始）
	PageSize  int    `form:"page_size"`  // 每页数量
}

// MetricsEntryWithRuleCounts 带解析后的规则计数的指标条目
// 将 JSON 格式的规则计数解析为 map
type MetricsEntryWithRuleCounts struct {
	ID         uint              `json:"id"`
	Timestamp  int64             `json:"timestamp"`
	RuleCounts map[string]int64  `json:"rule_counts"`
	TotalCount int64             `json:"total_count"`
	Duration   int64             `json:"duration"`
	Tag        string            `json:"tag"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// QueryMetricsResponse 查询指标响应结构体
// 包含指标列表和分页信息
type QueryMetricsResponse struct {
	Metrics   []MetricsEntryWithRuleCounts `json:"metrics"`    // 指标列表
	Total     int64                        `json:"total"`      // 总记录数
	Page      int                          `json:"page"`       // 当前页码
	PageSize  int                          `json:"page_size"`  // 每页数量
	TotalPage int                          `json:"total_page"` // 总页数
}

// QueryMetrics 查询指标数据
// 支持按标签、时间范围等条件查询
func (h *MetricsHandler) QueryMetrics(c *gin.Context) {
	var req QueryMetricsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100 // 限制最大页面大小
	}

	// 构建查询
	query := h.db.Model(&models.MetricsEntry{})

	// 应用筛选条件
	if req.Tag != "" {
		query = query.Where("tag = ?", req.Tag)
	}
	if req.StartTime > 0 {
		query = query.Where("timestamp >= ?", req.StartTime)
	}
	if req.EndTime > 0 {
		query = query.Where("timestamp <= ?", req.EndTime)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "查询指标失败",
			"message": err.Error(),
		})
		return
	}

	// 分页查询
	var metrics []models.MetricsEntry
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("timestamp DESC").Offset(offset).Limit(req.PageSize).Find(&metrics).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "查询指标失败",
			"message": err.Error(),
		})
		return
	}

	// 解析规则计数
	result := make([]MetricsEntryWithRuleCounts, 0, len(metrics))
	for _, m := range metrics {
		var ruleCounts map[string]int64
		if err := json.Unmarshal([]byte(m.RuleCounts), &ruleCounts); err != nil {
			ruleCounts = make(map[string]int64)
		}

		result = append(result, MetricsEntryWithRuleCounts{
			ID:         m.ID,
			Timestamp:  m.Timestamp,
			RuleCounts: ruleCounts,
			TotalCount: m.TotalCount,
			Duration:   m.Duration,
			Tag:        m.Tag,
			CreatedAt:  m.CreatedAt,
			UpdatedAt:  m.UpdatedAt,
		})
	}

	// 计算总页数
	totalPage := int((total + int64(req.PageSize) - 1) / int64(req.PageSize))

	c.JSON(http.StatusOK, QueryMetricsResponse{
		Metrics:   result,
		Total:     total,
		Page:      req.Page,
		PageSize:  req.PageSize,
		TotalPage: totalPage,
	})
}
