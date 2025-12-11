package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 应用配置结构体
// 包含服务器、数据库和其他配置信息
type Config struct {
	Server           ServerConfig   `yaml:"server"`            // 服务器配置
	Database         DatabaseConfig `yaml:"database"`          // 数据库配置
	LogRetentionDays int            `yaml:"log_retention_days"` // 日志保留天数
	CORS             CORSConfig     `yaml:"cors"`              // CORS 配置
}

// ServerConfig 服务器配置结构体
// 定义服务器监听地址和端口
type ServerConfig struct {
	Host string `yaml:"host"` // 监听地址
	Port int    `yaml:"port"` // 监听端口
}

// DatabaseConfig 数据库配置结构体
// 定义数据库连接信息
type DatabaseConfig struct {
	Type string `yaml:"type"` // 数据库类型
	DSN  string `yaml:"dsn"`  // 数据库连接字符串
}

// CORSConfig CORS 配置结构体
// 定义跨域资源共享配置
type CORSConfig struct {
	Enabled      bool     `yaml:"enabled"`       // 是否启用 CORS
	AllowOrigins []string `yaml:"allow_origins"` // 允许的来源
	AllowMethods []string `yaml:"allow_methods"` // 允许的 HTTP 方法
	AllowHeaders []string `yaml:"allow_headers"` // 允许的请求头
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
		cfg.Server.Port = 8080
	}
	if cfg.Database.Type == "" {
		cfg.Database.Type = "sqlite"
	}
	if cfg.Database.DSN == "" {
		cfg.Database.DSN = "./log_manager.db"
	}

	return &cfg, nil
}
