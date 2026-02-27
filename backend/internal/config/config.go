package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 应用配置结构体
// 包含服务器、数据库和其他配置信息
type Config struct {
	Server           ServerConfig    `yaml:"server"`             // 服务器配置
	Database         DatabaseConfig  `yaml:"database"`           // 数据库配置
	LogRetentionDays int             `yaml:"log_retention_days"` // 日志保留天数
	CORS             CORSConfig      `yaml:"cors"`               // CORS 配置
	RateLimit        RateLimitConfig `yaml:"rate_limit"`         // 限流配置
	Auth             AuthConfig      `yaml:"auth"`               // 认证配置
	UDP              UDPConfig       `yaml:"udp"`                // UDP 日志接收配置
}

// UDPConfig UDP 日志接收配置
type UDPConfig struct {
	Enabled       bool   `yaml:"enabled"`        // 是否启用 UDP 接收
	Host          string `yaml:"host"`           // 监听地址
	Port          int    `yaml:"port"`           // 监听端口
	Secret        string `yaml:"secret"`         // 可选，非空时校验 payload 中的 secret/api_key
	BufferSize    int    `yaml:"buffer_size"`    // 内存缓冲条数
	FlushInterval string `yaml:"flush_interval"` // 批量落库间隔，如 100ms
	FlushSize     int    `yaml:"flush_size"`     // 达到该条数立即落库
}

// AuthConfig 认证配置
type AuthConfig struct {
	APIKey          string `yaml:"api_key"`           // API Key，agent 使用；为空则不做认证
	LoginEnabled    bool   `yaml:"login_enabled"`     // 是否启用 Web 管理界面登录
	AdminUsername   string `yaml:"admin_username"`    // 管理员用户名
	AdminPassword   string `yaml:"admin_password"`    // 管理员密码
	JWTSecret       string `yaml:"jwt_secret"`        // JWT 签名密钥
	JWTExpireHours  int    `yaml:"jwt_expire_hours"`  // JWT 有效期（小时）
}

// ServerConfig 服务器配置结构体
// 定义服务器监听地址和端口
type ServerConfig struct {
	Host    string `yaml:"host"`     // 监听地址
	Port    int    `yaml:"port"`     // 监听端口
	WebDist string `yaml:"web_dist"` // 前端静态文件目录，如 ./web 或 ../frontend/build；为空则不托管前端
}

// DatabaseConfig 数据库配置结构体
// 定义数据库连接信息
type DatabaseConfig struct {
	Type            string `yaml:"type"`              // 数据库类型
	DSN             string `yaml:"dsn"`               // 数据库连接字符串
	MaxOpenConns    int    `yaml:"max_open_conns"`    // 最大打开连接数
	MaxIdleConns    int    `yaml:"max_idle_conns"`    // 最大空闲连接数
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"` // 连接最大生存时间（秒）
	LogLevel        string `yaml:"log_level"`         // 日志级别：silent, error, warn, info
}

// CORSConfig CORS 配置结构体
// 定义跨域资源共享配置
type CORSConfig struct {
	Enabled      bool     `yaml:"enabled"`       // 是否启用 CORS
	AllowOrigins []string `yaml:"allow_origins"` // 允许的来源
	AllowMethods []string `yaml:"allow_methods"` // 允许的 HTTP 方法
	AllowHeaders []string `yaml:"allow_headers"` // 允许的请求头
}

// RateLimitConfig 限流配置结构体
// 定义请求限流参数
type RateLimitConfig struct {
	Enabled       bool `yaml:"enabled"`        // 是否启用限流
	Rate          int  `yaml:"rate"`           // 每秒允许的请求数（默认 API）
	Capacity      int  `yaml:"capacity"`       // 桶容量（可选，默认为 rate）
	BatchRate     int  `yaml:"batch_rate"`     // 批量接口每秒请求数（logs/batch、metrics/batch，默认 1000）
	BatchCapacity int  `yaml:"batch_capacity"` // 批量接口桶容量（可选，默认等于 batch_rate）
}

// LoadConfig 加载配置文件
// filePath: 配置文件路径
// 返回: Config 实例和错误信息
func LoadConfig(filePath string) (*Config, error) {
	// 读取配置文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析 YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8888
	}
	if cfg.Database.Type == "" {
		cfg.Database.Type = "sqlite"
	}
	if cfg.Database.DSN == "" {
		cfg.Database.DSN = "./log_manager.db"
	}
	if cfg.Database.MaxOpenConns <= 0 {
		cfg.Database.MaxOpenConns = 25 // 默认最大打开连接数
	}
	if cfg.Database.MaxIdleConns <= 0 {
		cfg.Database.MaxIdleConns = 5 // 默认最大空闲连接数
	}
	if cfg.Database.ConnMaxLifetime <= 0 {
		cfg.Database.ConnMaxLifetime = 300 // 默认5分钟
	}
	if cfg.Database.LogLevel == "" {
		cfg.Database.LogLevel = "info" // 默认日志级别
	}
	if cfg.RateLimit.Rate <= 0 {
		cfg.RateLimit.Rate = 100 // 默认每秒100个请求
	}
	if cfg.RateLimit.Capacity <= 0 {
		cfg.RateLimit.Capacity = cfg.RateLimit.Rate // 默认容量等于速率
	}
	if cfg.RateLimit.BatchRate <= 0 {
		cfg.RateLimit.BatchRate = 1000 // 批量接口默认 1000 req/s，支撑亿级吞吐
	}
	if cfg.RateLimit.BatchCapacity <= 0 {
		cfg.RateLimit.BatchCapacity = cfg.RateLimit.BatchRate
	}
	if cfg.Auth.JWTExpireHours <= 0 {
		cfg.Auth.JWTExpireHours = 24 // 默认 24 小时
	}
	if cfg.Auth.JWTSecret == "" && cfg.Auth.LoginEnabled {
		cfg.Auth.JWTSecret = "log-manager-default-secret-change-in-production"
	}
	if cfg.UDP.Host == "" {
		cfg.UDP.Host = "0.0.0.0"
	}
	if cfg.UDP.Port <= 0 {
		cfg.UDP.Port = 8889
	}
	if cfg.UDP.BufferSize <= 0 {
		cfg.UDP.BufferSize = 10000
	}
	if cfg.UDP.FlushInterval == "" {
		cfg.UDP.FlushInterval = "100ms"
	}
	if cfg.UDP.FlushSize <= 0 {
		cfg.UDP.FlushSize = 500
	}

	return &cfg, nil
}
