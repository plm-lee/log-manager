package main

import (
	"log"
	"log-manager/internal/app"
	"log-manager/internal/config"
)

// main 主函数
// 负责启动日志管理器服务
func main() {
	// 加载配置
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 创建应用实例
	application := app.NewApp(cfg)

	// 初始化应用
	if err := application.Init(); err != nil {
		log.Fatalf("初始化应用失败: %v", err)
	}

	// 启动服务器
	if err := application.Start(); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
