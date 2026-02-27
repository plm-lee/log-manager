package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log-manager/internal/app"
	"log-manager/internal/cleanup"
	"log-manager/internal/config"
	"log-manager/internal/database"
)

// main 主函数
// 负责启动日志管理器服务，支持优雅关闭
func main() {
	// 加载配置（可通过 CONFIG 环境变量指定配置文件）
	configFile := "config.yaml"
	if v := os.Getenv("CONFIG"); v != "" {
		configFile = v
	}
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 创建应用实例
	application := app.NewApp(cfg)

	// 初始化应用
	if err := application.Init(); err != nil {
		log.Fatalf("初始化应用失败: %v", err)
	}

	// 创建 HTTP 服务器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: application.GetRouter(),
	}

	// 启动数据保留定时任务
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cleanup.StartRetentionJob(ctx, cfg)

	// 在 goroutine 中启动服务器
	go func() {
		fmt.Printf("服务器启动在 %s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("启动服务器失败: %v", err)
		}
	}()

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel() // 停止数据保留任务

	log.Println("\n正在关闭服务器...")

	// 创建超时上下文，给服务器5秒时间完成当前请求
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// 优雅关闭服务器
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("服务器强制关闭: %v", err)
	}

	// 停止 UDP 和 TCP 日志接收
	application.StopUDPServer()
	application.StopTCPServer()

	// 关闭数据库连接
	if err := database.Close(); err != nil {
		log.Printf("关闭数据库连接失败: %v", err)
	}

	log.Println("服务器已优雅关闭")
}
