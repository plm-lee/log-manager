package dashstats

import (
	"context"
	"log"
	"time"

	"log-manager/internal/database"
	"log-manager/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Refresh 更新 dashboard_stats 表，供定时任务调用
func Refresh(db *gorm.DB) error {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()

	var totalLogs, totalMetrics, todayLogs, todayMetrics, distinctTags, distinctRules int64
	db.Model(&models.LogEntry{}).Count(&totalLogs)
	db.Model(&models.MetricsEntry{}).Count(&totalMetrics)
	db.Model(&models.LogEntry{}).Where("timestamp >= ?", todayStart).Count(&todayLogs)
	db.Model(&models.MetricsEntry{}).Where("timestamp >= ?", todayStart).Count(&todayMetrics)
	db.Model(&models.Tag{}).Where("name != ? AND name IS NOT NULL", "").Count(&distinctTags)
	db.Model(&models.RuleName{}).Where("name != ? AND name IS NOT NULL", "").Count(&distinctRules)

	stat := models.DashboardStat{
		ID:            1,
		TotalLogs:     totalLogs,
		TotalMetrics:  totalMetrics,
		TodayLogs:     todayLogs,
		TodayMetrics:  todayMetrics,
		DistinctTags:  distinctTags,
		DistinctRules: distinctRules,
		LastUpdated:   now,
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"total_logs", "total_metrics", "today_logs", "today_metrics", "distinct_tags", "distinct_rules", "last_updated"}),
	}).Create(&stat).Error
}

// StartRefreshJob 启动 dashboard_stats 定时刷新任务（每分钟）
func StartRefreshJob(ctx context.Context) {
	if err := Refresh(database.DB); err != nil {
		log.Printf("[dashstats] 首次刷新失败: %v", err)
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := Refresh(database.DB); err != nil {
				log.Printf("[dashstats] 刷新失败: %v", err)
			}
		}
	}
}
