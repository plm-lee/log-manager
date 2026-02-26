package handler

import (
	"net/http"

	"log-manager/internal/config"
	"log-manager/internal/middleware"

	"github.com/gin-gonic/gin"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	cfg *config.Config
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token  string `json:"token"`
	User   string `json:"user"`
	Expire int    `json:"expire_hours"`
}

// Login 登录
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	if !h.cfg.Auth.LoginEnabled {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "登录已禁用",
			"message": "当前未启用 Web 登录",
		})
		return
	}
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "参数错误",
			"message": err.Error(),
		})
		return
	}
	if req.Username != h.cfg.Auth.AdminUsername || req.Password != h.cfg.Auth.AdminPassword {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "登录失败",
			"message": "用户名或密码错误",
		})
		return
	}
	token, err := middleware.GenerateToken(h.cfg.Auth.JWTSecret, req.Username, h.cfg.Auth.JWTExpireHours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "生成 Token 失败",
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, LoginResponse{
		Token:  token,
		User:   req.Username,
		Expire: h.cfg.Auth.JWTExpireHours,
	})
}

// Me 当前用户信息
// GET /api/v1/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	if !h.cfg.Auth.LoginEnabled {
		c.JSON(http.StatusOK, gin.H{
			"user":     "",
			"enabled":  false,
		})
		return
	}
	user, exists := c.Get("user")
	if !exists || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "未授权",
			"message": "请先登录",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user":    user.(string),
		"enabled": true,
	})
}

// Logout 登出（客户端清除 token 即可，服务端无状态）
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Config 认证配置（无需登录，供前端判断是否需要登录页）
// GET /api/v1/auth/config
func (h *AuthHandler) Config(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"login_enabled": h.cfg.Auth.LoginEnabled,
	})
}
