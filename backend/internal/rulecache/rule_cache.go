package rulecache

import (
	"log"
	"strings"
	"sync"

	"log-manager/internal/models"

	"gorm.io/gorm"
)

// Cache 规则名称内存缓存，用于快速判断 rule_name 是否已存在
type Cache struct {
	mu   sync.RWMutex
	set  map[string]struct{}
	db   *gorm.DB
}

// New 创建 RuleCache 实例
func New(db *gorm.DB) *Cache {
	return &Cache{
		set: make(map[string]struct{}),
		db:  db,
	}
}

// LoadFromDB 从 rule_names 表加载全部规则名到缓存
func (c *Cache) LoadFromDB() error {
	var names []string
	if err := c.db.Model(&models.RuleName{}).Pluck("name", &names).Error; err != nil {
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
	log.Printf("[rulecache] 已加载 %d 个 rule_name 到缓存", len(c.set))
	return nil
}

// EnsureRuleName 确保规则名存在：若不在缓存中则插入 rule_names 表并加入缓存
func (c *Cache) EnsureRuleName(name string) error {
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
	if err := c.db.Where("name = ?", name).FirstOrCreate(&models.RuleName{Name: name}).Error; err != nil {
		return err
	}
	c.set[name] = struct{}{}
	return nil
}

// Reload 重新从数据库加载缓存
func (c *Cache) Reload() error {
	return c.LoadFromDB()
}

// BackfillFromLogEntries 从 log_entries 分页回填历史 rule_name（首次部署时调用）
func (c *Cache) BackfillFromLogEntries() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.set) > 0 {
		return nil
	}
	var maxID uint
	for {
		var rows []struct {
			ID       uint
			RuleName string
		}
		err := c.db.Table("log_entries").Select("id, rule_name").
			Where("rule_name != '' AND rule_name IS NOT NULL AND id > ?", maxID).
			Order("id ASC").
			Limit(5000).
			Scan(&rows).Error
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		seen := make(map[string]struct{})
		for _, r := range rows {
			name := strings.TrimSpace(r.RuleName)
			if name != "" {
				seen[name] = struct{}{}
			}
			if r.ID > maxID {
				maxID = r.ID
			}
		}
		for name := range seen {
			if err := c.db.Where("name = ?", name).FirstOrCreate(&models.RuleName{Name: name}).Error; err != nil {
				return err
			}
			c.set[name] = struct{}{}
		}
		if len(rows) < 5000 {
			break
		}
	}
	return nil
}
