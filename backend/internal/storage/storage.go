package storage

import (
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

// Info 存储信息
type Info struct {
	StorageType     string  `json:"storage_type"`
	UsedBytes       int64   `json:"used_bytes"`
	WarnBytes       int64   `json:"warn_bytes"`
	CriticalBytes   int64   `json:"critical_bytes"`
}

// GetInfo 获取存储用量信息
// dbType: sqlite / mysql
// dsn: 数据库连接串（sqlite 时用于获取文件路径）
// warnMB, criticalMB: 告警阈值（MB）
// db: MySQL 时需传入以执行查询，sqlite 时可传 nil
func GetInfo(dbType, dsn string, warnMB, criticalMB int, db *gorm.DB) (*Info, error) {
	if warnMB <= 0 {
		warnMB = 500
	}
	if criticalMB <= 0 {
		criticalMB = 1000
	}
	warnBytes := int64(warnMB) * 1024 * 1024
	criticalBytes := int64(criticalMB) * 1024 * 1024

	info := &Info{
		StorageType:   dbType,
		WarnBytes:     warnBytes,
		CriticalBytes: criticalBytes,
	}

	switch dbType {
	case "sqlite":
		path := dsn
		if strings.HasPrefix(dsn, "file:") {
			path = strings.TrimPrefix(dsn, "file:")
			if idx := strings.Index(path, "?"); idx >= 0 {
				path = path[:idx]
			}
		}
		abs, err := filepath.Abs(path)
		if err == nil {
			path = abs
		}
		fi, err := os.Stat(path)
		if err != nil {
			return info, err
		}
		info.UsedBytes = fi.Size()
		return info, nil
	case "mysql":
		if db != nil {
			var usedBytes int64
			err := db.Raw("SELECT COALESCE(SUM(data_length + index_length), 0) FROM information_schema.TABLES WHERE table_schema = DATABASE()").Scan(&usedBytes).Error
			if err == nil {
				info.UsedBytes = usedBytes
			}
		}
		return info, nil
	default:
		return info, nil
	}
}
