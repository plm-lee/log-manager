package database

import (
	"fmt"
	"strings"
	"time"

	"log-manager/internal/config"
	"log-manager/internal/models"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB 全局数据库实例
var DB *gorm.DB

// Type 数据库类型（sqlite / mysql），用于全文检索等分支
var Type string

// Init 初始化数据库连接
// cfg: 数据库配置
// 返回: 错误信息
func Init(cfg *config.DatabaseConfig) error {
	var err error
	var dialector gorm.Dialector

	Type = cfg.Type
	// 根据数据库类型创建连接器
	switch cfg.Type {
	case "sqlite":
		dialector = sqlite.Open(cfg.DSN)
	case "mysql":
		dialector = mysql.Open(cfg.DSN)
	default:
		return fmt.Errorf("不支持的数据库类型: %s", cfg.Type)
	}

	// 解析日志级别
	var logLevel logger.LogLevel
	switch cfg.LogLevel {
	case "silent":
		logLevel = logger.Silent
	case "error":
		logLevel = logger.Error
	case "warn":
		logLevel = logger.Warn
	case "info":
		logLevel = logger.Info
	default:
		logLevel = logger.Info
	}

	// 连接数据库
	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// 配置连接池（仅对非 SQLite 数据库有效，SQLite 不支持连接池）
	if cfg.Type != "sqlite" {
		sqlDB, err := DB.DB()
		if err != nil {
			return fmt.Errorf("获取数据库实例失败: %w", err)
		}

		// 设置连接池参数
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	}

	// 自动迁移数据库表
	if err := DB.AutoMigrate(
		&models.LogEntry{},
		&models.MetricsEntry{},
		&models.BillingConfig{},
		&models.BillingEntry{},
		&models.AgentConfig{},
		&models.TagProject{},
		&models.Tag{},
		&models.RuleName{},
		&models.TagLogCount{},
		&models.DashboardStat{},
	); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	// 确保存在唯一的计费项目（Type=billing）
	if err := EnsureBillingProject(); err != nil {
		return fmt.Errorf("初始化计费项目失败: %w", err)
	}

	// 迁移：为 billing_configs 中 billing_tag 为空的旧数据回填
	if err := migrateBillingConfigBillingTag(); err != nil {
		return fmt.Errorf("迁移 billing_tag 失败: %w", err)
	}

	// 全文检索索引（替代 log_line LIKE '%keyword%' 慢查询）
	if err := ensureFullTextSearch(cfg.Type); err != nil {
		return fmt.Errorf("初始化全文检索失败: %w", err)
	}

	return nil
}

// ensureFullTextSearch 根据数据库类型创建全文检索索引
func ensureFullTextSearch(dbType string) error {
	switch dbType {
	case "mysql":
		// MySQL FULLTEXT 索引，已存在则忽略错误
		if err := DB.Exec("ALTER TABLE log_entries ADD FULLTEXT INDEX ft_log_line(log_line)").Error; err != nil {
			if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "1061") {
				return nil // 索引已存在
			}
			return err
		}
	case "sqlite":
		// SQLite FTS5 虚拟表
		if err := DB.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS log_entries_fts USING fts5(log_line, content='log_entries', content_rowid='id')`).Error; err != nil {
			return err
		}
		// 触发器保持 FTS 与主表同步（忽略已存在错误）
		for _, tr := range []string{
			`CREATE TRIGGER IF NOT EXISTS log_entries_fts_insert AFTER INSERT ON log_entries BEGIN
				INSERT INTO log_entries_fts(rowid, log_line) VALUES (new.id, new.log_line);
			END`,
			`CREATE TRIGGER IF NOT EXISTS log_entries_fts_delete AFTER DELETE ON log_entries BEGIN
				INSERT INTO log_entries_fts(log_entries_fts, rowid, log_line) VALUES ('delete', old.id, old.log_line);
			END`,
			`CREATE TRIGGER IF NOT EXISTS log_entries_fts_update AFTER UPDATE ON log_entries BEGIN
				INSERT INTO log_entries_fts(log_entries_fts, rowid, log_line) VALUES ('delete', old.id, old.log_line);
				INSERT INTO log_entries_fts(rowid, log_line) VALUES (new.id, new.log_line);
			END`,
		} {
			if err := DB.Exec(tr).Error; err != nil && !strings.Contains(err.Error(), "already exists") {
				return err
			}
		}
		// 回填已有数据到 FTS（content 表模式下 rebuild 会从 content 表重建）
		_ = DB.Exec(`INSERT INTO log_entries_fts(log_entries_fts) VALUES ('rebuild')`).Error
	}
	return nil
}

// migrateBillingConfigBillingTag 为旧计费配置回填 billing_tag
// match_type=tag 时用 match_value；否则用 tag_scope 首个值
func migrateBillingConfigBillingTag() error {
	var configs []models.BillingConfig
	if err := DB.Where("billing_tag = '' OR billing_tag IS NULL").Limit(1000).Find(&configs).Error; err != nil {
		return err
	}
	for _, cfg := range configs {
		var billingTag string
		if cfg.MatchType == "tag" {
			billingTag = strings.TrimSpace(cfg.MatchValue)
		} else if cfg.TagScope != "" {
			parts := strings.SplitN(strings.TrimSpace(cfg.TagScope), ",", 2)
			billingTag = strings.TrimSpace(parts[0])
		}
		if billingTag != "" {
			if err := DB.Model(&models.BillingConfig{}).Where("id = ?", cfg.ID).Update("billing_tag", billingTag).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// EnsureBillingProject 确保存在唯一的计费项目，不存在则自动创建
func EnsureBillingProject() error {
	var count int64
	if err := DB.Model(&models.TagProject{}).Where("type = ?", "billing").Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		p := models.TagProject{
			Name:        "计费项目",
			Type:        "billing",
			Description: "系统默认计费项目，归属此项目的 tag 视为计费类型",
		}
		return DB.Create(&p).Error
	}
	return nil
}

// Close 关闭数据库连接
func Close() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
