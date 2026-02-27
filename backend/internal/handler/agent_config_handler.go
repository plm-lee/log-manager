package handler

import (
	"net/http"

	"log-manager/internal/database"
	"log-manager/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AgentConfigHandler Agent 配置下发处理器
type AgentConfigHandler struct{}

// NewAgentConfigHandler 创建 Agent 配置处理器
func NewAgentConfigHandler() *AgentConfigHandler {
	return &AgentConfigHandler{}
}

// GetConfig Agent 拉取配置
// GET /api/v1/agent/config?agent_id=xxx
// 需 API Key 认证
func (h *AgentConfigHandler) GetConfig(c *gin.Context) {
	agentID := c.Query("agent_id")
	if agentID == "" {
		agentID = "default"
	}
	var ac models.AgentConfig
	if err := database.DB.Where("agent_id = ?", agentID).First(&ac).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "未找到配置",
			"message": "该 agent_id 尚无下发配置，请使用本地配置文件",
		})
		return
	}
	// 返回 YAML 文本，Agent 可直接解析
	c.Header("Content-Type", "application/x-yaml")
	c.String(http.StatusOK, ac.ConfigYAML)
}

// SetConfigRequest 设置 Agent 配置请求
type SetConfigRequest struct {
	AgentID    string `json:"agent_id" binding:"required"`
	ConfigYAML string `json:"config_yaml" binding:"required"`
}

// SetConfig Admin 设置 Agent 配置
// POST /api/v1/agent/config
func (h *AgentConfigHandler) SetConfig(c *gin.Context) {
	var req SetConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误", "message": err.Error()})
		return
	}
	var ac models.AgentConfig
	err := database.DB.Where("agent_id = ?", req.AgentID).First(&ac).Error
	if err == gorm.ErrRecordNotFound {
		ac = models.AgentConfig{AgentID: req.AgentID, ConfigYAML: req.ConfigYAML, Version: 1}
		if err := database.DB.Create(&ac).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建失败", "message": err.Error()})
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败", "message": err.Error()})
		return
	} else {
		ac.ConfigYAML = req.ConfigYAML
		ac.Version++
		if err := database.DB.Save(&ac).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败", "message": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "agent_id": req.AgentID, "version": ac.Version})
}
