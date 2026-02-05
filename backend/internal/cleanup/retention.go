package cleanup

import (
	"context"
	"log"
	"time"

	"log-manager/internal/config"
	"log-manager/internal/database"
	"log-manager/internal/models"
)

// StartRetentionJob 启动数据保留定时任务
// 根据 log_retention_days 配置定期清理过期的日志和指标数据
// retentionDays 为 0 时表示永久保留，不执行清理
func StartRetentionJob(ctx context.Context, cfg *config.Config) {
	if cfg.LogRetentionDays <= 0 {
		log.Println("数据保留策略: 永久保留（log_retention_days=0）")
		return
	}

	// 每天凌晨 2 点执行一次
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// 启动时立即执行一次
	runRetention(cfg.LogRetentionDays)

	for {
		select {
		case <-ctx.Done():
			log.Println("数据保留任务已停止")
			return
		case <-ticker.C:
			runRetention(cfg.LogRetentionDays)
		}
	}
}

func runRetention(retentionDays int) {
	if retentionDays <= 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays).Unix()

	// 永久删除过期的日志（Unscoped 跳过软删除）
	result := database.DB.Unscoped().Where("timestamp < ?", cutoff).Delete(&models.LogEntry{})
	if result.Error != nil {
		log.Printf("清理过期日志失败: %v\n", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("数据保留: 已清理 %d 条过期日志\n", result.RowsAffected)
	}

	// 永久删除过期的指标
	result = database.DB.Unscoped().Where("timestamp < ?", cutoff).Delete(&models.MetricsEntry{})
	if result.Error != nil {
		log.Printf("清理过期指标失败: %v\n", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("数据保留: 已清理 %d 条过期指标\n", result.RowsAffected)
	}
}
