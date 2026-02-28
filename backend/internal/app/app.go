package app

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log-manager/internal/config"
	"log-manager/internal/database"
	"log-manager/internal/handler"
	"log-manager/internal/middleware"
	"log-manager/internal/models"
	"log-manager/internal/tagcache"
	"log-manager/internal/tcpserver"
	"log-manager/internal/udpserver"
	"log-manager/internal/unmatchedqueue"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// App 应用结构体
// 负责管理整个应用的初始化和运行
type App struct {
	cfg        *config.Config
	router     *gin.Engine
	logHandler *handler.LogHandler
	udpServer  interface{ Stop() }
	tcpServer  interface{ Stop() }
}

// GetRouter 获取路由引擎
// 返回: Gin 路由引擎
func (a *App) GetRouter() *gin.Engine {
	return a.router
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

	// 初始化 Tag 缓存
	tc := tagcache.New(database.DB)
	if err := tc.LoadFromDB(); err != nil {
		return fmt.Errorf("加载 tag 缓存失败: %w", err)
	}
	if err := tc.BackfillFromLegacyTables(); err != nil {
		log.Printf("[warn] 回填历史 tag 失败: %v", err)
	}

	// 初始化无匹配规则队列
	unmatchedQueue := unmatchedqueue.New(5000)

	// 初始化路由
	a.initRouter(tc, unmatchedQueue)

	// 启动 UDP 日志接收（若配置启用）
	if a.cfg.UDP.Enabled {
		srv, err := udpserver.Start(&a.cfg.UDP, a.logHandler)
		if err != nil {
			return fmt.Errorf("启动UDP日志接收失败: %w", err)
		}
		if srv != nil {
			a.udpServer = srv
		}
	}

	// 启动 TCP 日志接收（若配置启用）
	if a.cfg.TCP.Enabled {
		srv, err := tcpserver.Start(&a.cfg.TCP, a.logHandler)
		if err != nil {
			return fmt.Errorf("启动TCP日志接收失败: %w", err)
		}
		if srv != nil {
			a.tcpServer = srv
		}
	}

	return nil
}

// StopUDPServer 停止 UDP 服务（优雅关闭时调用）
func (a *App) StopUDPServer() {
	if a.udpServer != nil {
		a.udpServer.Stop()
		a.udpServer = nil
	}
}

// StopTCPServer 停止 TCP 服务（优雅关闭时调用）
func (a *App) StopTCPServer() {
	if a.tcpServer != nil {
		a.tcpServer.Stop()
		a.tcpServer = nil
	}
}

// initRouter 初始化路由
// 配置所有 API 路由和中间件
func (a *App) initRouter(tagCache *tagcache.Cache, unmatchedQueue *unmatchedqueue.Queue) {
	// 创建 Gin 路由引擎
	if a.cfg.Server.Host == "0.0.0.0" && a.cfg.Server.Port == 8888 {
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

	// 创建处理器实例（共享 billing 缓存，tag 归属变更时立即失效以实时生效）
	billingConfigCache := handler.NewBillingConfigCache(60 * time.Second)
	a.logHandler = handler.NewLogHandler(tagCache, unmatchedQueue, billingConfigCache)
	logHandler := a.logHandler
	metricsHandler := handler.NewMetricsHandler()
	dashboardHandler := handler.NewDashboardHandler()
	billingHandler := handler.NewBillingHandler(unmatchedQueue)
	tagHandler := handler.NewTagHandler(tagCache, func() { billingConfigCache.Invalidate() })
	authHandler := handler.NewAuthHandler(a.cfg)
	agentConfigHandler := handler.NewAgentConfigHandler()

	// 统一前缀 /log/manager
	g := a.router.Group("/log/manager")
	api := g.Group("/api/v1")
	if a.cfg.RateLimit.Enabled {
		api.Use(middleware.DualRateLimitMiddleware(
			a.cfg.RateLimit.Rate,
			a.cfg.RateLimit.Capacity,
			a.cfg.RateLimit.BatchRate,
			a.cfg.RateLimit.BatchCapacity,
		))
	}

	// 认证相关接口
	api.GET("/auth/config", authHandler.Config)   // 无需认证，返回 login_enabled
	api.POST("/auth/login", authHandler.Login)    // 无需认证
	api.POST("/auth/logout", authHandler.Logout)  // 无需认证
	if a.cfg.Auth.LoginEnabled {
		api.GET("/auth/me", middleware.APIKeyOrJWTMiddleware(a.cfg.Auth.APIKey, a.cfg.Auth.JWTSecret, true), authHandler.Me)
	} else {
		api.GET("/auth/me", authHandler.Me)
	}

	// Agent 上报接口：仅 API Key 认证（与 log-filter-monitor 等客户端通信）
	agentAPI := api.Group("")
	if a.cfg.Auth.APIKey != "" {
		agentAPI.Use(middleware.APIKeyMiddleware(a.cfg.Auth.APIKey))
	}
	{
		agentAPI.POST("/logs", logHandler.ReceiveLog)
		agentAPI.POST("/logs/batch", logHandler.BatchReceiveLog)
		agentAPI.POST("/metrics", metricsHandler.ReceiveMetrics)
		agentAPI.POST("/metrics/batch", metricsHandler.BatchReceiveMetrics)
		agentAPI.GET("/agent/config", agentConfigHandler.GetConfig)
	}

	// Admin 接口：Web 管理界面使用，API Key 或 JWT 任一有效
	adminAPI := api.Group("")
	adminAPI.Use(middleware.APIKeyOrJWTMiddleware(a.cfg.Auth.APIKey, a.cfg.Auth.JWTSecret, a.cfg.Auth.LoginEnabled))
	{
		// 日志管理
		adminAPI.GET("/logs", logHandler.QueryLogs)
		adminAPI.POST("/logs/upload", logHandler.UploadLog)
		adminAPI.GET("/logs/export", logHandler.ExportLogs)
		adminAPI.GET("/logs/tags", logHandler.GetTags)
		adminAPI.GET("/logs/tags/stats", logHandler.GetTagStats)
		adminAPI.GET("/logs/tags/managed", tagHandler.GetManagedTags)
		adminAPI.PUT("/logs/tags/:name/project", tagHandler.SetTagProject)
		adminAPI.GET("/tag-projects", tagHandler.ListTagProjects)
		adminAPI.POST("/tag-projects", tagHandler.CreateTagProject)
		adminAPI.PUT("/tag-projects/:id", tagHandler.UpdateTagProject)
		adminAPI.DELETE("/tag-projects/:id", tagHandler.DeleteTagProject)
		adminAPI.GET("/logs/rule-names", logHandler.GetRuleNames)
		// 仪表盘
		adminAPI.GET("/dashboard/stats", dashboardHandler.GetStats)
		// 指标管理
		adminAPI.GET("/metrics", metricsHandler.QueryMetrics)
		adminAPI.GET("/metrics/stats", metricsHandler.QueryMetricsStats)
		// 计费管理
		adminAPI.POST("/agent/config", agentConfigHandler.SetConfig)
		adminAPI.GET("/billing/tags", billingHandler.GetTags)
		adminAPI.GET("/billing/billing-project-tags", tagHandler.GetBillingProjectTags)
		adminAPI.GET("/billing/configs", billingHandler.GetConfigs)
		adminAPI.POST("/billing/configs", billingHandler.CreateConfig)
		adminAPI.PUT("/billing/configs/:id", billingHandler.UpdateConfig)
		adminAPI.DELETE("/billing/configs/:id", billingHandler.DeleteConfig)
		adminAPI.GET("/billing/stats", billingHandler.GetStats)
		adminAPI.GET("/billing/unmatched", billingHandler.GetUnmatched)
	}

	// 健康检查接口
	g.GET("/health", func(c *gin.Context) {
		// 检查数据库连接
		sqlDB, err := database.DB.DB()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "unhealthy",
				"service": "log-manager",
				"error":   "数据库连接失败",
			})
			return
		}

		// 测试数据库连接
		if err := sqlDB.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "unhealthy",
				"service": "log-manager",
				"error":   "数据库连接测试失败",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "log-manager",
			"time":    time.Now().Unix(),
		})
	})

	// 监控指标接口（注意：与 /api/v1/metrics 不同，此为 Prometheus 风格）
	g.GET("/metrics", func(c *gin.Context) {
		// 获取数据库统计信息
		var logCount, metricsCount int64
		database.DB.Model(&models.LogEntry{}).Count(&logCount)
		database.DB.Model(&models.MetricsEntry{}).Count(&metricsCount)

		// 获取数据库连接池信息（仅对非 SQLite 数据库有效）
		var dbStats map[string]interface{}
		sqlDB, err := database.DB.DB()
		if err == nil {
			stats := sqlDB.Stats()
			dbStats = map[string]interface{}{
				"max_open_connections": stats.MaxOpenConnections,
				"open_connections":     stats.OpenConnections,
				"in_use":               stats.InUse,
				"idle":                 stats.Idle,
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"service": "log-manager",
			"stats": gin.H{
				"log_entries":     logCount,
				"metrics_entries": metricsCount,
				"database":        dbStats,
			},
		})
	})

	// 托管前端静态文件（生产部署：仅启动后端，无需 Node.js）
	a.serveWebIfConfigured(g)
}

// serveWebIfConfigured 若配置了 web_dist 且目录存在，则托管静态文件并支持 SPA 回退
func (a *App) serveWebIfConfigured(g *gin.RouterGroup) {
	webDist := strings.TrimSpace(a.cfg.Server.WebDist)
	if webDist == "" {
		return
	}
	// 相对路径相对于 backend 工作目录
	if !filepath.IsAbs(webDist) {
		webDist = filepath.Join(".", webDist)
	}
	info, err := os.Stat(webDist)
	if err != nil || !info.IsDir() {
		log.Printf("[web] 跳过静态托管: web_dist=%q 不存在或非目录", a.cfg.Server.WebDist)
		return
	}
	log.Printf("[web] 托管前端: %s", webDist)
	// 静态资源（CRA 输出在 build/static）
	g.Static("/static", filepath.Join(webDist, "static"))
	if _, err := os.Stat(filepath.Join(webDist, "favicon.ico")); err == nil {
		g.StaticFile("/favicon.ico", filepath.Join(webDist, "favicon.ico"))
	}
	if _, err := os.Stat(filepath.Join(webDist, "manifest.json")); err == nil {
		g.StaticFile("/manifest.json", filepath.Join(webDist, "manifest.json"))
	}
	if _, err := os.Stat(filepath.Join(webDist, "robots.txt")); err == nil {
		g.StaticFile("/robots.txt", filepath.Join(webDist, "robots.txt"))
	}
	// 根路径
	indexPath := filepath.Join(webDist, "index.html")
	g.GET("/", func(c *gin.Context) { c.File(indexPath) })

	// SPA 回退：使用 NoRoute 避免与 /api 等路径冲突（Gin 中同组内 catch-all 与字面路径冲突）
	a.router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/log/manager") &&
			!strings.HasPrefix(path, "/log/manager/api/") &&
			path != "/log/manager/health" &&
			path != "/log/manager/metrics" &&
			!strings.HasPrefix(path, "/log/manager/static/") {
			c.File(indexPath)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})
}

// Start 启动服务器（已废弃，使用 main.go 中的优雅关闭方式）
// 返回: 错误信息
// 注意：此方法保留用于向后兼容，但建议使用 main.go 中的优雅关闭方式
func (a *App) Start() error {
	addr := fmt.Sprintf("%s:%d", a.cfg.Server.Host, a.cfg.Server.Port)
	fmt.Printf("服务器启动在 %s\n", addr)
	return a.router.Run(addr)
}
