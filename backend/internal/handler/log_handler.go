package handler

import (
	"net/http"
	"time"

	"log-manager/internal/database"
	"log-manager/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// LogHandler 日志处理器
// 负责处理日志相关的 HTTP 请求
type LogHandler struct {
	db *gorm.DB
}

// NewLogHandler 创建日志处理器实例
// 返回: LogHandler 实例
func NewLogHandler() *LogHandler {
	return &LogHandler{
		db: database.DB,
	}
}

// ReceiveLogRequest 接收日志请求结构体
// 对应 log-filter-monitor 上报的日志数据格式
type ReceiveLogRequest struct {
	Timestamp int64  `json:"timestamp" binding:"required"` // 时间戳
	RuleName  string `json:"rule_name"`                    // 规则名称
	RuleDesc  string `json:"rule_desc"`                    // 规则描述
	LogLine   string `json:"log_line" binding:"required"`  // 日志行内容
	LogFile   string `json:"log_file"`                     // 日志文件路径
	Pattern   string `json:"pattern"`                      // 匹配模式
	Tag       string `json:"tag"`                          // 标签
}

// ReceiveLog 接收日志数据
// 处理来自 log-filter-monitor 的日志上报请求
func (h *LogHandler) ReceiveLog(c *gin.Context) {
	var req ReceiveLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}

	// 创建日志条目
	logEntry := models.LogEntry{
		Timestamp: req.Timestamp,
		RuleName:  req.RuleName,
		RuleDesc:  req.RuleDesc,
		LogLine:   req.LogLine,
		LogFile:   req.LogFile,
		Pattern:   req.Pattern,
		Tag:       req.Tag,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 保存到数据库
	if err := h.db.Create(&logEntry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "保存日志失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"id":      logEntry.ID,
	})
}

// QueryLogsRequest 查询日志请求结构体
// 定义日志查询的筛选条件
type QueryLogsRequest struct {
	Tag       string `form:"tag"`        // 标签筛选
	RuleName  string `form:"rule_name"`  // 规则名称筛选
	Keyword   string `form:"keyword"`    // 关键词搜索（在日志内容中搜索）
	StartTime int64  `form:"start_time"` // 开始时间戳
	EndTime   int64  `form:"end_time"`   // 结束时间戳
	Page      int    `form:"page"`       // 页码（从1开始）
	PageSize  int    `form:"page_size"`  // 每页数量
}

// QueryLogsResponse 查询日志响应结构体
// 包含日志列表和分页信息
type QueryLogsResponse struct {
	Logs      []models.LogEntry `json:"logs"`       // 日志列表
	Total     int64             `json:"total"`      // 总记录数
	Page      int               `json:"page"`       // 当前页码
	PageSize  int               `json:"page_size"`  // 每页数量
	TotalPage int               `json:"total_page"` // 总页数
}

// QueryLogs 查询日志数据
// 支持按标签、规则名称、关键词、时间范围等条件查询
func (h *LogHandler) QueryLogs(c *gin.Context) {
	var req QueryLogsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100 // 限制最大页面大小
	}

	// 构建查询
	query := h.db.Model(&models.LogEntry{})

	// 应用筛选条件
	if req.Tag != "" {
		query = query.Where("tag = ?", req.Tag)
	}
	if req.RuleName != "" {
		query = query.Where("rule_name = ?", req.RuleName)
	}
	if req.Keyword != "" {
		query = query.Where("log_line LIKE ?", "%"+req.Keyword+"%")
	}
	if req.StartTime > 0 {
		query = query.Where("timestamp >= ?", req.StartTime)
	}
	if req.EndTime > 0 {
		query = query.Where("timestamp <= ?", req.EndTime)
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "查询日志失败",
			"message": err.Error(),
		})
		return
	}

	// 分页查询
	var logs []models.LogEntry
	offset := (req.Page - 1) * req.PageSize
	if err := query.Order("timestamp DESC").Offset(offset).Limit(req.PageSize).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "查询日志失败",
			"message": err.Error(),
		})
		return
	}

	// 计算总页数
	totalPage := int((total + int64(req.PageSize) - 1) / int64(req.PageSize))

	c.JSON(http.StatusOK, QueryLogsResponse{
		Logs:      logs,
		Total:     total,
		Page:      req.Page,
		PageSize:  req.PageSize,
		TotalPage: totalPage,
	})
}

// GetTags 获取所有标签列表
// 返回数据库中所有不同的标签
func (h *LogHandler) GetTags(c *gin.Context) {
	var tags []string
	if err := h.db.Model(&models.LogEntry{}).
		Distinct("tag").
		Where("tag != ?", "").
		Pluck("tag", &tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "查询标签失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tags": tags,
	})
}

// GetRuleNames 获取所有规则名称列表
// 返回数据库中所有不同的规则名称
func (h *LogHandler) GetRuleNames(c *gin.Context) {
	var ruleNames []string
	if err := h.db.Model(&models.LogEntry{}).
		Distinct("rule_name").
		Where("rule_name != ?", "").
		Pluck("rule_name", &ruleNames).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "查询规则名称失败",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rule_names": ruleNames,
	})
}
