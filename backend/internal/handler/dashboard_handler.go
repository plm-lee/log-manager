package handler

import (
	"net/http"
	"time"

	"log-manager/internal/database"
	"log-manager/internal/models"

	"github.com/gin-gonic/gin"
)

// DashboardHandler 仪表盘处理器
type DashboardHandler struct{}

// NewDashboardHandler 创建仪表盘处理器
func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{}
}

// DashboardStats 仪表盘统计数据
type DashboardStats struct {
	TotalLogs       int64 `json:"total_logs"`        // 日志总数
	TotalMetrics    int64 `json:"total_metrics"`     // 指标总数
	TodayLogs       int64 `json:"today_logs"`        // 今日日志数
	TodayMetrics    int64 `json:"today_metrics"`     // 今日指标数
	DistinctTags    int64 `json:"distinct_tags"`     // 不同标签数
	DistinctRules   int64 `json:"distinct_rules"`    // 不同规则数
}

// GetStats 获取仪表盘概览统计
func (h *DashboardHandler) GetStats(c *gin.Context) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()

	var totalLogs, totalMetrics, todayLogs, todayMetrics, distinctTags, distinctRules int64

	database.DB.Model(&models.LogEntry{}).Count(&totalLogs)
	database.DB.Model(&models.MetricsEntry{}).Count(&totalMetrics)
	database.DB.Model(&models.LogEntry{}).Where("timestamp >= ?", todayStart).Count(&todayLogs)
	database.DB.Model(&models.MetricsEntry{}).Where("timestamp >= ?", todayStart).Count(&todayMetrics)
	database.DB.Model(&models.LogEntry{}).Where("tag != ? AND tag IS NOT NULL", "").Select("COUNT(DISTINCT tag)").Scan(&distinctTags)
	database.DB.Model(&models.LogEntry{}).Where("rule_name != ? AND rule_name IS NOT NULL", "").Select("COUNT(DISTINCT rule_name)").Scan(&distinctRules)

	c.JSON(http.StatusOK, DashboardStats{
		TotalLogs:    totalLogs,
		TotalMetrics: totalMetrics,
		TodayLogs:    todayLogs,
		TodayMetrics: todayMetrics,
		DistinctTags: distinctTags,
		DistinctRules: distinctRules,
	})
}
