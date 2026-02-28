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

// BillingConfig 计费配置模型
// 定义计费类型与单价，用于按日志匹配统计计费
type BillingConfig struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	BillKey     string    `gorm:"size:100;not null;index" json:"bill_key"`      // 计费类型标识
	BillingTag  string    `gorm:"size:500;not null;default:'';index" json:"billing_tag"` // 逗号拼接的 tag 列表，该规则对列出的 tag 生效
	MatchType   string    `gorm:"size:32;not null" json:"match_type"`           // tag / rule_name / log_line_contains
	MatchValue  string    `gorm:"size:255;not null" json:"match_value"`         // 匹配值
	TagScope    string    `gorm:"size:500;default:''" json:"tag_scope"`         // 已废弃，保留兼容；匹配时用 billing_tag
	UnitPrice   float64   `gorm:"type:decimal(12,4);not null" json:"unit_price"` // 单价
	Description string    `gorm:"type:text" json:"description"`                 // 备注
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName 指定表名
func (BillingConfig) TableName() string {
	return "billing_configs"
}

// BillingEntry 计费明细聚合（按天+bill_key+tag）
// 计费日志在接收时直接写入此表，不进入 log_entries，不受保留策略清除
type BillingEntry struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Date      string    `gorm:"size:10;not null;uniqueIndex:idx_billing_date_key_tag" json:"date"`   // YYYY-MM-DD
	BillKey   string    `gorm:"size:100;not null;uniqueIndex:idx_billing_date_key_tag" json:"bill_key"`
	Tag       string    `gorm:"size:100;default:'';uniqueIndex:idx_billing_date_key_tag" json:"tag"` // 标签（实际日志的 tag）
	Count     int64     `gorm:"not null" json:"count"`
	Amount    float64   `gorm:"type:decimal(14,4);not null" json:"amount"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (BillingEntry) TableName() string {
	return "billing_entries"
}

// TagProject 大项目（tag 聚合）
// Type=billing 时为系统默认的计费项目，归属该项目的 tag 即视为计费类型
type TagProject struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:100;not null" json:"name"`        // 项目名称
	Type        string    `gorm:"size:32;default:'normal'" json:"type"` // normal | billing
	Description string    `gorm:"type:text" json:"description"`         // 描述
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (TagProject) TableName() string {
	return "tag_projects"
}

// Tag 标签（独立存储，用于缓存与分类管理）
type Tag struct {
	ID        uint        `gorm:"primaryKey" json:"id"`
	Name      string      `gorm:"size:100;uniqueIndex;not null" json:"name"` // tag 字符串
	ProjectID *uint       `gorm:"index" json:"project_id"`                   // 所属大项目（可选）
	Project   *TagProject `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

func (Tag) TableName() string {
	return "tags"
}

// AgentConfig Agent 配置下发（供 log-filter-monitor 拉取）
// agent_id 为 "default" 时作为默认配置
type AgentConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   string    `gorm:"size:64;not null;uniqueIndex" json:"agent_id"` // 如 default
	ConfigYAML string   `gorm:"type:longtext;not null" json:"config_yaml"`
	Version   int64     `gorm:"not null" json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AgentConfig) TableName() string {
	return "agent_configs"
}
