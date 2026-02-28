package tagcache

import (
	"log"
	"strings"
	"sync"

	"log-manager/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Cache tag 名称内存缓存，用于快速判断 tag 是否已存在
type Cache struct {
	mu   sync.RWMutex
	set  map[string]struct{}
	db   *gorm.DB
}

// New 创建 TagCache 实例
func New(db *gorm.DB) *Cache {
	return &Cache{
		set: make(map[string]struct{}),
		db:  db,
	}
}

// LoadFromDB 从 tags 表加载全部 tag 到缓存
func (c *Cache) LoadFromDB() error {
	var names []string
	if err := c.db.Model(&models.Tag{}).Pluck("name", &names).Error; err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.set = make(map[string]struct{}, len(names))
	for _, n := range names {
		if n != "" {
			c.set[n] = struct{}{}
		}
	}
	log.Printf("[tagcache] 已加载 %d 个 tag 到缓存", len(c.set))
	return nil
}

// EnsureTag 确保 tag 存在：若不在缓存中则插入 tags 表并加入缓存
func (c *Cache) EnsureTag(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	c.mu.RLock()
	if _, ok := c.set[name]; ok {
		c.mu.RUnlock()
		return nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.set[name]; ok {
		return nil
	}
	if err := c.db.Where("name = ?", name).FirstOrCreate(&models.Tag{Name: name}).Error; err != nil {
		return err
	}
	c.set[name] = struct{}{}
	return nil
}

// Reload 重新从数据库加载缓存（分类管理中修改 tag 后调用）
func (c *Cache) Reload() error {
	return c.LoadFromDB()
}

// parseTags 解析逗号分隔的 tag 字符串
func parseTags(s string) []string {
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

// BackfillFromLegacyTables 从 log_entries、billing_entries 分页回填历史 tag 到 tags 表（首次部署时调用）
// 避免全表 Distinct 慢查询，改用分页扫描 + 批量插入
func (c *Cache) BackfillFromLegacyTables() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.set) > 0 {
		return nil
	}
	seen := make(map[string]struct{})
	// 分页扫描 log_entries
	var maxID uint
	for {
		var rows []struct {
			ID  uint
			Tag string
		}
		if err := c.db.Table("log_entries").Select("id, tag").
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
			for _, t := range parseTags(r.Tag) {
				seen[t] = struct{}{}
			}
			if r.ID > maxID {
				maxID = r.ID
			}
		}
		if len(rows) < 5000 {
			break
		}
	}
	// 分页扫描 billing_entries
	maxID = 0
	for {
		var rows []struct {
			ID  uint
			Tag string
		}
		if err := c.db.Table("billing_entries").Select("id, tag").
			Where("tag != '' AND id > ?", maxID).
			Order("id ASC").
			Limit(5000).
			Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			for _, t := range parseTags(r.Tag) {
				seen[t] = struct{}{}
			}
			if r.ID > maxID {
				maxID = r.ID
			}
		}
		if len(rows) < 5000 {
			break
		}
	}
	// 批量插入 tags（ON CONFLICT DO NOTHING）
	tags := make([]models.Tag, 0, len(seen))
	for name := range seen {
		tags = append(tags, models.Tag{Name: name})
		c.set[name] = struct{}{}
	}
	if len(tags) > 0 {
		if err := c.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).CreateInBatches(tags, 100).Error; err != nil {
			return err
		}
	}
	if len(seen) > 0 {
		log.Printf("[tagcache] 从历史表回填 %d 个 tag", len(seen))
	}
	return nil
}
