package handler

import (
	"net/http"

	"log-manager/internal/database"
	"log-manager/internal/dashstats"
	"log-manager/internal/models"

	"github.com/gin-gonic/gin"
)

func refreshDashboardStats() error {
	return dashstats.Refresh(database.DB)
}

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
// 从 dashboard_stats 读取（定时任务更新），避免全表 Count 慢查询
func (h *DashboardHandler) GetStats(c *gin.Context) {
	var stat models.DashboardStat
	if err := database.DB.Where("id = ?", 1).First(&stat).Error; err != nil {
		// 首次请求或无数据时同步刷新一次
		_ = refreshDashboardStats()
		if err := database.DB.Where("id = ?", 1).First(&stat).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "获取仪表盘统计失败",
				"message": err.Error(),
			})
			return
		}
	}
	c.JSON(http.StatusOK, DashboardStats{
		TotalLogs:    stat.TotalLogs,
		TotalMetrics: stat.TotalMetrics,
		TodayLogs:    stat.TodayLogs,
		TodayMetrics: stat.TodayMetrics,
		DistinctTags: stat.DistinctTags,
		DistinctRules: stat.DistinctRules,
	})
}
