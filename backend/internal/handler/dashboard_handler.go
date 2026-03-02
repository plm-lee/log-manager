package handler

import (
	"net/http"

	"log-manager/internal/config"
	"log-manager/internal/database"
	"log-manager/internal/dashstats"
	"log-manager/internal/models"
	"log-manager/internal/requestmetrics"
	"log-manager/internal/storage"
	"log-manager/internal/sysstats"

	"github.com/gin-gonic/gin"
)

func refreshDashboardStats() error {
	return dashstats.Refresh(database.DB)
}

// DashboardHandler 仪表盘处理器
type DashboardHandler struct {
	cfg *config.Config
}

// NewDashboardHandler 创建仪表盘处理器
func NewDashboardHandler(cfg *config.Config) *DashboardHandler {
	return &DashboardHandler{cfg: cfg}
}

// DashboardStats 仪表盘统计数据
type DashboardStats struct {
	TotalLogs       int64                   `json:"total_logs"`
	TotalMetrics    int64                   `json:"total_metrics"`
	TodayLogs       int64                   `json:"today_logs"`
	TodayMetrics    int64                   `json:"today_metrics"`
	DistinctTags    int64                   `json:"distinct_tags"`
	DistinctRules   int64                   `json:"distinct_rules"`
	Storage         *storage.Info           `json:"storage,omitempty"`
	Process         *sysstats.ProcessStats  `json:"process,omitempty"`
	RequestMetrics  *RequestMetricsResp     `json:"request_metrics,omitempty"`
	AgentNodes      []models.AgentNodeStat  `json:"agent_nodes,omitempty"`
}

// RequestMetricsResp 请求指标
type RequestMetricsResp struct {
	RequestsLastMinute int     `json:"requests_last_minute"`
	AvgLatencyMs       float64 `json:"avg_latency_ms"`
}

// GetStats 获取仪表盘概览统计
func (h *DashboardHandler) GetStats(c *gin.Context) {
	var stat models.DashboardStat
	if err := database.DB.Where("id = ?", 1).First(&stat).Error; err != nil {
		_ = refreshDashboardStats()
		if err := database.DB.Where("id = ?", 1).First(&stat).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "获取仪表盘统计失败",
				"message": err.Error(),
			})
			return
		}
	}

	resp := DashboardStats{
		TotalLogs:    stat.TotalLogs,
		TotalMetrics: stat.TotalMetrics,
		TodayLogs:    stat.TodayLogs,
		TodayMetrics: stat.TodayMetrics,
		DistinctTags: stat.DistinctTags,
		DistinctRules: stat.DistinctRules,
	}

	if h.cfg != nil {
		st, err := storage.GetInfo(h.cfg.Database.Type, h.cfg.Database.DSN, h.cfg.StorageWarnMB, h.cfg.StorageCriticalMB, database.DB)
		if err == nil {
			resp.Storage = st
		}
		ps, err := sysstats.GetProcessStats()
		if err == nil {
			resp.Process = ps
		}
		resp.RequestMetrics = &RequestMetricsResp{
			RequestsLastMinute: requestmetrics.RequestsLastMinute(),
			AvgLatencyMs:       requestmetrics.AvgLatencyMs(),
		}
	}

	var nodes []models.AgentNodeStat
	if err := database.DB.Order("last_reported_at DESC").Find(&nodes).Error; err == nil && len(nodes) > 0 {
		resp.AgentNodes = nodes
	}

	c.JSON(http.StatusOK, resp)
}
