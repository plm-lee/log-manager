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

// Init 初始化数据库连接
// cfg: 数据库配置
// 返回: 错误信息
func Init(cfg *config.DatabaseConfig) error {
	var err error
	var dialector gorm.Dialector

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

	return nil
}

// migrateBillingConfigBillingTag 为旧计费配置回填 billing_tag
// match_type=tag 时用 match_value；否则用 tag_scope 首个值
func migrateBillingConfigBillingTag() error {
	var configs []models.BillingConfig
	if err := DB.Where("billing_tag = '' OR billing_tag IS NULL").Find(&configs).Error; err != nil {
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
