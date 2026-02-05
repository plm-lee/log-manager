package models

import (
	"time"

	"gorm.io/gorm"
)

// LogEntry 日志条目模型
// 存储从 log-filter-monitor 上报的日志数据或手动上传的日志
type LogEntry struct {
	ID        uint           `gorm:"primaryKey" json:"id"`                   // 主键 ID
	Timestamp int64          `gorm:"index;not null" json:"timestamp"`        // 时间戳
	RuleName  string         `gorm:"index;size:255" json:"rule_name"`        // 规则名称
	RuleDesc  string         `gorm:"type:text" json:"rule_desc"`             // 规则描述
	LogLine   string         `gorm:"type:text;not null" json:"log_line"`     // 日志行内容
	LogFile   string         `gorm:"size:500" json:"log_file"`               // 日志文件路径
	Pattern   string         `gorm:"type:text" json:"pattern"`               // 匹配模式
	Tag       string         `gorm:"index;size:100" json:"tag"`              // 标签（用于区分不同项目）
	Source    string         `gorm:"size:20;default:agent" json:"source"`    // 来源：agent / manual
	CreatedAt time.Time      `json:"created_at"`                             // 创建时间
	UpdatedAt time.Time      `json:"updated_at"`                             // 更新时间
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`      // 软删除时间
}

// TableName 指定表名
// 返回: 数据库表名
func (LogEntry) TableName() string {
	return "log_entries"
}

// MetricsEntry 指标条目模型
// 存储从 log-filter-monitor 上报的指标数据
type MetricsEntry struct {
	ID         uint           `gorm:"primaryKey" json:"id"`                      // 主键 ID
	Timestamp  int64          `gorm:"index;not null" json:"timestamp"`           // 时间戳
	RuleCounts string         `gorm:"type:text;not null" json:"rule_counts"`     // 规则计数（JSON 格式）
	TotalCount int64          `gorm:"not null" json:"total_count"`               // 总计数
	Duration   int64          `gorm:"not null" json:"duration"`                  // 统计时长（秒）
	Tag        string         `gorm:"index;size:100" json:"tag"`                 // 标签（用于区分不同项目）
	CreatedAt  time.Time      `json:"created_at"`                                // 创建时间
	UpdatedAt  time.Time      `json:"updated_at"`                                // 更新时间
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`         // 软删除时间
}

// TableName 指定表名
// 返回: 数据库表名
func (MetricsEntry) TableName() string {
	return "metrics_entries"
}
