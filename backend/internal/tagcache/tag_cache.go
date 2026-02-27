package tagcache

import (
	"log"
	"strings"
	"sync"

	"log-manager/internal/models"

	"gorm.io/gorm"
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

// BackfillFromLegacyTables 从 log_entries、billing_entries 回填历史 tag 到 tags 表（首次部署时调用）
func (c *Cache) BackfillFromLegacyTables() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.set) > 0 {
		return nil // 已有数据，无需回填
	}
	var names []string
	if err := c.db.Table("log_entries").Where("tag != '' AND tag IS NOT NULL").Distinct("tag").Pluck("tag", &names).Error; err != nil {
		return err
	}
	seen := make(map[string]struct{})
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n != "" {
			seen[n] = struct{}{}
		}
	}
	var billingTags []string
	if err := c.db.Table("billing_entries").Where("tag != ''").Distinct("tag").Pluck("tag", &billingTags).Error; err != nil {
		return err
	}
	for _, n := range billingTags {
		n = strings.TrimSpace(n)
		if n != "" {
			seen[n] = struct{}{}
		}
	}
	for name := range seen {
		if err := c.db.Where("name = ?", name).FirstOrCreate(&models.Tag{Name: name}).Error; err != nil {
			return err
		}
		c.set[name] = struct{}{}
	}
	if len(seen) > 0 {
		log.Printf("[tagcache] 从历史表回填 %d 个 tag", len(seen))
	}
	return nil
}
