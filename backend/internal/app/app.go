package app

import (
	"fmt"
	"log-manager/internal/config"
	"log-manager/internal/database"
	"log-manager/internal/handler"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// App 应用结构体
// 负责管理整个应用的初始化和运行
type App struct {
	cfg    *config.Config
	router *gin.Engine
}

// NewApp 创建应用实例
// cfg: 应用配置
// 返回: App 实例
func NewApp(cfg *config.Config) *App {
	return &App{
		cfg: cfg,
	}
}

// Init 初始化应用
// 包括数据库连接、路由配置等
// 返回: 错误信息
func (a *App) Init() error {
	// 初始化数据库
	if err := database.Init(&a.cfg.Database); err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}

	// 初始化路由
	a.initRouter()

	return nil
}

// initRouter 初始化路由
// 配置所有 API 路由和中间件
func (a *App) initRouter() {
	// 创建 Gin 路由引擎
	if a.cfg.Server.Host == "0.0.0.0" && a.cfg.Server.Port == 8080 {
		gin.SetMode(gin.ReleaseMode)
	}
	a.router = gin.Default()

	// 配置 CORS
	if a.cfg.CORS.Enabled {
		corsConfig := cors.Config{
			AllowOrigins:     a.cfg.CORS.AllowOrigins,
			AllowMethods:     a.cfg.CORS.AllowMethods,
			AllowHeaders:     a.cfg.CORS.AllowHeaders,
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
		}
		a.router.Use(cors.New(corsConfig))
	}

	// 创建处理器实例
	logHandler := handler.NewLogHandler()
	metricsHandler := handler.NewMetricsHandler()

	// API 路由组
	api := a.router.Group("/api/v1")
	{
		// 日志相关接口
		logs := api.Group("/logs")
		{
			logs.POST("", logHandler.ReceiveLog)       // 接收日志
			logs.GET("", logHandler.QueryLogs)         // 查询日志
			logs.GET("/tags", logHandler.GetTags)      // 获取标签列表
			logs.GET("/rule-names", logHandler.GetRuleNames) // 获取规则名称列表
		}

		// 指标相关接口
		metrics := api.Group("/metrics")
		{
			metrics.POST("", metricsHandler.ReceiveMetrics) // 接收指标
			metrics.GET("", metricsHandler.QueryMetrics)    // 查询指标
		}
	}

	// 健康检查接口
	a.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "log-manager",
		})
	})
}

// Start 启动服务器
// 返回: 错误信息
func (a *App) Start() error {
	addr := fmt.Sprintf("%s:%d", a.cfg.Server.Host, a.cfg.Server.Port)
	fmt.Printf("服务器启动在 %s\n", addr)
	return a.router.Run(addr)
}
