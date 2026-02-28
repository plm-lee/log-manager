package taglogcount

import (
	"log"
	"strings"
	"time"

	"log-manager/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// parseLogTags 解析逗号分隔的 tag 字符串
func parseLogTags(s string) []string {
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

// IncrByTagDeltas 按 tag -> delta 增量更新 tag_log_counts
func IncrByTagDeltas(db *gorm.DB, deltas map[string]int64) error {
	if len(deltas) == 0 {
		return nil
	}
	now := time.Now()
	for tag, delta := range deltas {
		tag = strings.TrimSpace(tag)
		if tag == "" || delta <= 0 {
			continue
		}
		row := models.TagLogCount{
			Tag:         tag,
			Count:       delta,
			LastUpdated: now,
		}
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tag"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"count":        gorm.Expr("count + ?", delta),
				"last_updated": now,
			}),
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// DecrByTagDeltas 按 tag -> delta 减量更新 tag_log_counts（用于 retention 删除日志时，delta 为负）
func DecrByTagDeltas(db *gorm.DB, deltas map[string]int64) error {
	if len(deltas) == 0 {
		return nil
	}
	now := time.Now()
	for tag, delta := range deltas {
		tag = strings.TrimSpace(tag)
		if tag == "" || delta >= 0 {
			continue
		}
		if err := db.Model(&models.TagLogCount{}).Where("tag = ?", tag).
			Updates(map[string]interface{}{
				"count":        gorm.Expr("CASE WHEN count + ? < 0 THEN 0 ELSE count + ? END", delta, delta),
				"last_updated": now,
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

// BackfillFromLogEntries 从 log_entries 分页回填 tag_log_counts（首次部署或表为空时调用）
func BackfillFromLogEntries(db *gorm.DB) error {
	var cnt int64
	if err := db.Model(&models.TagLogCount{}).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}
	agg := make(map[string]int64)
	var maxID uint
	for {
		var rows []struct {
			ID  uint
			Tag string
		}
		if err := db.Table("log_entries").Select("id, tag").
			Where("deleted_at IS NULL AND tag != '' AND tag IS NOT NULL AND id > ?", maxID).
			Order("id ASC").
			Limit(5000).
			Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			for _, t := range parseLogTags(r.Tag) {
				agg[t]++
			}
			if r.ID > maxID {
				maxID = r.ID
			}
		}
		if len(rows) < 5000 {
			break
		}
	}
	if err := IncrByTagDeltas(db, agg); err != nil {
		return err
	}
	if len(agg) > 0 {
		log.Printf("[taglogcount] 从 log_entries 回填 %d 个 tag 的计数", len(agg))
	}
	return nil
}
