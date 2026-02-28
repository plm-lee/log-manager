package handler

import (
	"encoding/json"
	"fmt"
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
	Tag        string           `json:"tag"`                            // 标签
}

// ReceiveMetrics 接收指标数据
// 处理来自 log-filter-monitor 的指标上报请求
// 支持两种格式：
// 1. 点格式数组（log-filter-monitor 发送的格式）
// 2. 单个指标对象（向后兼容）
func (h *MetricsHandler) ReceiveMetrics(c *gin.Context) {
	// 先尝试解析为点格式数组（log-filter-monitor 发送的格式）
	var points []map[string]interface{}
	if err := c.ShouldBindJSON(&points); err == nil && len(points) > 0 {
		// 成功解析为数组，处理点格式数据
		if err := h.handlePointsFormat(points); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "处理指标数据失败",
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
		return
	}

	// 如果不是数组格式，尝试解析为单个指标对象（向后兼容）
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

// handlePointsFormat 处理点格式数组
// 将点格式数组聚合为指标对象并存储
// points: 点格式数组
// 返回: 错误信息
func (h *MetricsHandler) handlePointsFormat(points []map[string]interface{}) error {
	// 聚合点数据：按时间戳、标签分组
	// 使用 map 来聚合：key = timestamp + tagString, value = 聚合后的数据
	aggregated := make(map[string]*aggregatedMetrics)

	for _, point := range points {
		// 提取字段
		metric, _ := point["metric"].(string)
		timestamp, ok := point["timestamp"]
		if !ok {
			continue
		}

		var ts int64
		switch v := timestamp.(type) {
		case int64:
			ts = v
		case float64:
			ts = int64(v)
		case int:
			ts = int64(v)
		default:
			continue
		}

		value, ok := point["value"]
		if !ok {
			continue
		}

		var count float64
		switch v := value.(type) {
		case float64:
			count = v
		case int64:
			count = float64(v)
		case int:
			count = float64(v)
		default:
			continue
		}

		// 提取标签
		tagString := ""
		if tags, ok := point["tags"].(string); ok && tags != "" {
			tagString = tags
		}

		// 提取 step（统计间隔）
		var step int64 = 60 // 默认60秒
		if stepVal, ok := point["step"]; ok {
			switch v := stepVal.(type) {
			case int64:
				step = v
			case float64:
				step = int64(v)
			case int:
				step = int64(v)
			}
		}

		// 创建聚合键：时间戳对齐到 step
		alignedTs := ts - (ts % step)
		key := fmt.Sprintf("%d_%s", alignedTs, tagString)

		// 获取或创建聚合对象
		agg, exists := aggregated[key]
		if !exists {
			agg = &aggregatedMetrics{
				Timestamp:  alignedTs,
				Tag:        tagString,
				RuleCounts: make(map[string]int64),
				TotalCount: 0,
				Duration:   step,
			}
			aggregated[key] = agg
		}

		// 累加计数
		if metric != "" {
			agg.RuleCounts[metric] += int64(count)
		}
		agg.TotalCount += int64(count)
	}

	// 将聚合后的数据保存到数据库
	now := time.Now()
	entries := make([]models.MetricsEntry, 0, len(aggregated))
	for _, agg := range aggregated {
		// 将规则计数序列化为 JSON
		ruleCountsJSON, err := json.Marshal(agg.RuleCounts)
		if err != nil {
			continue
		}

		entries = append(entries, models.MetricsEntry{
			Timestamp:  agg.Timestamp,
			RuleCounts: string(ruleCountsJSON),
			TotalCount: agg.TotalCount,
			Duration:   agg.Duration,
			Tag:        agg.Tag,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	// 批量保存
	if len(entries) > 0 {
		return h.db.CreateInBatches(&entries, 50).Error
	}

	return nil
}

// aggregatedMetrics 聚合后的指标数据
type aggregatedMetrics struct {
	Timestamp  int64
	Tag        string
	RuleCounts map[string]int64
	TotalCount int64
	Duration   int64
}

// BatchReceiveMetricsRequest 批量接收指标请求结构体
// 支持一次性接收多条指标数据
type BatchReceiveMetricsRequest struct {
	Metrics []ReceiveMetricsRequest `json:"metrics" binding:"required,min=1,max=100"` // 指标列表（最多100条）
}

// BatchReceiveMetricsResponse 批量接收指标响应结构体
type BatchReceiveMetricsResponse struct {
	Success int    `json:"success"` // 成功数量
	Failed  int    `json:"failed"`  // 失败数量
	IDs     []uint `json:"ids"`     // 成功创建的ID列表
}

// BatchReceiveMetrics 批量接收指标数据
// 支持一次性接收多条指标，提高吞吐量
func (h *MetricsHandler) BatchReceiveMetrics(c *gin.Context) {
	var req BatchReceiveMetricsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}

	// 限制批量大小
	if len(req.Metrics) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "批量大小超过限制",
			"message": "最多支持100条指标",
		})
		return
	}

	// 批量创建指标条目
	metricsEntries := make([]models.MetricsEntry, 0, len(req.Metrics))
	now := time.Now()
	for _, metricsReq := range req.Metrics {
		// 将规则计数序列化为 JSON
		ruleCountsJSON, err := json.Marshal(metricsReq.RuleCounts)
		if err != nil {
			// 如果序列化失败，使用空JSON
			ruleCountsJSON = []byte("{}")
		}

		metricsEntries = append(metricsEntries, models.MetricsEntry{
			Timestamp:  metricsReq.Timestamp,
			RuleCounts: string(ruleCountsJSON),
			TotalCount: metricsReq.TotalCount,
			Duration:   metricsReq.Duration,
			Tag:        metricsReq.Tag,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	// 批量保存到数据库
	var successIDs []uint
	var successCount, failedCount int

	// 使用事务批量插入
	if err := h.db.CreateInBatches(&metricsEntries, 50).Error; err != nil {
		// 如果批量插入失败，尝试逐条插入
		for _, entry := range metricsEntries {
			if err := h.db.Create(&entry).Error; err != nil {
				failedCount++
			} else {
				successCount++
				successIDs = append(successIDs, entry.ID)
			}
		}
	} else {
		// 批量插入成功
		successCount = len(metricsEntries)
		for _, entry := range metricsEntries {
			successIDs = append(successIDs, entry.ID)
		}
	}

	c.JSON(http.StatusOK, BatchReceiveMetricsResponse{
		Success: successCount,
		Failed:  failedCount,
		IDs:     successIDs,
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
	ID         uint             `json:"id"`
	Timestamp  int64            `json:"timestamp"`
	RuleCounts map[string]int64 `json:"rule_counts"`
	TotalCount int64            `json:"total_count"`
	Duration   int64            `json:"duration"`
	Tag        string           `json:"tag"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
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

// QueryMetricsStatsRequest 查询指标统计请求结构体
// 用于图表展示，按时间聚合指标数据
type QueryMetricsStatsRequest struct {
	Tag       string `form:"tag"`        // 标签筛选
	StartTime int64  `form:"start_time"` // 开始时间戳
	EndTime   int64  `form:"end_time"`   // 结束时间戳
	Interval  string `form:"interval"`   // 聚合间隔：1m, 5m, 15m, 1h, 1d（默认1h）
}

// MetricsStatsData 指标统计数据
type MetricsStatsData struct {
	Time       int64            `json:"time"`        // 时间戳
	TimeStr    string           `json:"time_str"`    // 时间字符串
	TotalCount int64            `json:"total_count"` // 总计数
	RuleCounts map[string]int64 `json:"rule_counts"` // 规则计数
}

// QueryMetricsStatsResponse 查询指标统计响应结构体
type QueryMetricsStatsResponse struct {
	Stats []MetricsStatsData `json:"stats"` // 统计数据列表
}

// QueryMetricsStats 查询指标统计数据（用于图表展示）
// 按时间间隔聚合指标数据，支持图表展示
func (h *MetricsHandler) QueryMetricsStats(c *gin.Context) {
	var req QueryMetricsStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}

	// 设置默认值
	if req.Interval == "" {
		req.Interval = "1h" // 默认1小时
	}
	if req.StartTime == 0 {
		// 默认查询最近24小时
		req.StartTime = time.Now().Add(-24 * time.Hour).Unix()
	}
	if req.EndTime == 0 {
		req.EndTime = time.Now().Unix()
	}

	// 解析时间间隔（秒）
	var intervalSec int64
	switch req.Interval {
	case "1m":
		intervalSec = 60
	case "5m":
		intervalSec = 300
	case "15m":
		intervalSec = 900
	case "1h":
		intervalSec = 3600
	case "1d":
		intervalSec = 86400
	default:
		intervalSec = 3600 // 默认1小时
	}

	// SQL 层时间桶聚合，避免全量 Find 慢查询
	type bucketRow struct {
		BucketTs   int64 `gorm:"column:bucket_ts"`
		TotalCount int64 `gorm:"column:total_count"`
	}
	var bucketRows []bucketRow
	sql := "SELECT (timestamp / ?) * ? as bucket_ts, SUM(total_count) as total_count FROM metrics_entries WHERE deleted_at IS NULL AND timestamp >= ? AND timestamp <= ?"
	args := []interface{}{intervalSec, intervalSec, req.StartTime, req.EndTime}
	if req.Tag != "" {
		sql += " AND tag = ?"
		args = append(args, req.Tag)
	}
	sql += " GROUP BY 1 ORDER BY 1 ASC"
	if err := h.db.Raw(sql, args...).Scan(&bucketRows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "查询指标失败",
			"message": err.Error(),
		})
		return
	}

	// 构建按桶的 TotalCount，RuleCounts 需要从明细行合并
	statsMap := make(map[int64]*MetricsStatsData)
	for _, r := range bucketRows {
		statsMap[r.BucketTs] = &MetricsStatsData{
			Time:       r.BucketTs,
			TimeStr:    time.Unix(r.BucketTs, 0).Format("2006-01-02 15:04:05"),
			TotalCount: r.TotalCount,
			RuleCounts: make(map[string]int64),
		}
	}

	// 拉取有限明细行用于合并 RuleCounts（Limit 50000 防止大范围锁表）
	var metrics []models.MetricsEntry
	detailQ := h.db.Model(&models.MetricsEntry{}).
		Where("timestamp >= ? AND timestamp <= ?", req.StartTime, req.EndTime)
	if req.Tag != "" {
		detailQ = detailQ.Where("tag = ?", req.Tag)
	}
	if err := detailQ.Order("timestamp ASC").Limit(50000).Find(&metrics).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "查询指标失败",
			"message": err.Error(),
		})
		return
	}
	for _, m := range metrics {
		alignedTime := (m.Timestamp / intervalSec) * intervalSec
		stat, exists := statsMap[alignedTime]
		if !exists {
			stat = &MetricsStatsData{
				Time:       alignedTime,
				TimeStr:    time.Unix(alignedTime, 0).Format("2006-01-02 15:04:05"),
				TotalCount: 0,
				RuleCounts: make(map[string]int64),
			}
			statsMap[alignedTime] = stat
		}
		var ruleCounts map[string]int64
		if err := json.Unmarshal([]byte(m.RuleCounts), &ruleCounts); err != nil {
			ruleCounts = make(map[string]int64)
		}
		for ruleName, count := range ruleCounts {
			stat.RuleCounts[ruleName] += count
		}
	}

	stats := make([]MetricsStatsData, 0, len(statsMap))
	for _, stat := range statsMap {
		stats = append(stats, *stat)
	}
	for i := 0; i < len(stats)-1; i++ {
		for j := i + 1; j < len(stats); j++ {
			if stats[i].Time > stats[j].Time {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}

	c.JSON(http.StatusOK, QueryMetricsStatsResponse{
		Stats: stats,
	})
}
