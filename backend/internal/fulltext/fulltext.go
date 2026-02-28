package fulltext

import (
	"log-manager/internal/database"

	"gorm.io/gorm"
)

// ApplyLogLineKeyword 对 log_entries 查询应用关键词过滤，使用全文检索替代 LIKE
func ApplyLogLineKeyword(db *gorm.DB, keyword string) *gorm.DB {
	if keyword == "" {
		return db
	}
	switch database.Type {
	case "mysql":
		return db.Where("MATCH(log_line) AGAINST(? IN NATURAL LANGUAGE MODE)", keyword)
	case "sqlite":
		return db.Joins("INNER JOIN log_entries_fts ON log_entries.id = log_entries_fts.rowid").
			Where("log_entries_fts MATCH ?", keyword)
	default:
		// 回退到 LIKE（如 postgres 等未实现全文检索时）
		return db.Where("log_line LIKE ?", "%"+keyword+"%")
	}
}
