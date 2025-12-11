package database

import (
	"fmt"

	"log-manager/internal/config"
	"log-manager/internal/models"

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
	default:
		return fmt.Errorf("不支持的数据库类型: %s", cfg.Type)
	}

	// 连接数据库
	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// 自动迁移数据库表
	if err := DB.AutoMigrate(
		&models.LogEntry{},
		&models.MetricsEntry{},
	); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
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
