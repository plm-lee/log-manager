package cleanup

import (
	"context"
	"log"
	"strings"
	"time"

	"log-manager/internal/config"
	"log-manager/internal/database"
	"log-manager/internal/models"
	"log-manager/internal/taglogcount"
)

const retentionBatchSize = 10000 // 每批删除条数，避免大事务锁表

// StartRetentionJob 启动数据保留定时任务
// 根据 log_retention_days 配置定期清理过期的日志和指标数据
// retentionDays 为 0 时表示永久保留，不执行清理
func StartRetentionJob(ctx context.Context, cfg *config.Config) {
	if cfg.LogRetentionDays <= 0 {
		log.Println("数据保留策略: 永久保留（log_retention_days=0）")
		return
	}

	// 每天执行一次
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

	// 分批删除过期日志，控制单次事务大小，避免大表长时间锁
	var totalLogsDeleted int64
	for {
		var batch []models.LogEntry
		if err := database.DB.Unscoped().
			Where("timestamp < ?", cutoff).
			Limit(retentionBatchSize).
			Find(&batch).Error; err != nil {
			log.Printf("清理过期日志查询失败: %v\n", err)
			break
		}
		if len(batch) == 0 {
			break
		}
		ids := make([]uint, 0, len(batch))
		deltas := make(map[string]int64)
		for _, e := range batch {
			ids = append(ids, e.ID)
			for _, t := range parseRetentionTags(e.Tag) {
				deltas[t]--
			}
		}
		if len(deltas) > 0 {
			if err := taglogcount.DecrByTagDeltas(database.DB, deltas); err != nil {
				log.Printf("更新 tag_log_counts 失败: %v\n", err)
			}
		}
		result := database.DB.Unscoped().Delete(&models.LogEntry{}, ids)
		if result.Error != nil {
			log.Printf("清理过期日志失败: %v\n", result.Error)
			break
		}
		totalLogsDeleted += result.RowsAffected
		time.Sleep(100 * time.Millisecond)
	}
	if totalLogsDeleted > 0 {
		log.Printf("数据保留: 已清理 %d 条过期日志\n", totalLogsDeleted)
	}

	// 分批删除过期指标
	var totalMetricsDeleted int64
	for {
		result := database.DB.Unscoped().
			Where("timestamp < ?", cutoff).
			Limit(retentionBatchSize).
			Delete(&models.MetricsEntry{})
		if result.Error != nil {
			log.Printf("清理过期指标失败: %v\n", result.Error)
			break
		}
		if result.RowsAffected == 0 {
			break
		}
		totalMetricsDeleted += result.RowsAffected
		time.Sleep(100 * time.Millisecond)
	}
	if totalMetricsDeleted > 0 {
		log.Printf("数据保留: 已清理 %d 条过期指标\n", totalMetricsDeleted)
	}
}

func parseRetentionTags(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
