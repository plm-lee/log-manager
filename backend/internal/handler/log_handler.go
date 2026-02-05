package handler

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
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
		Source:    "agent",
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

// BatchReceiveLogRequest 批量接收日志请求结构体
// 支持一次性接收多条日志数据
type BatchReceiveLogRequest struct {
	Logs []ReceiveLogRequest `json:"logs" binding:"required,min=1,max=100"` // 日志列表（最多100条）
}

// BatchReceiveLogResponse 批量接收日志响应结构体
type BatchReceiveLogResponse struct {
	Success int   `json:"success"` // 成功数量
	Failed  int   `json:"failed"`  // 失败数量
	IDs     []uint `json:"ids"`     // 成功创建的ID列表
}

// BatchReceiveLog 批量接收日志数据
// 支持一次性接收多条日志，提高吞吐量
func (h *LogHandler) BatchReceiveLog(c *gin.Context) {
	var req BatchReceiveLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}

	// 限制批量大小
	if len(req.Logs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "批量大小超过限制",
			"message": "最多支持100条日志",
		})
		return
	}

	// 批量创建日志条目
	logEntries := make([]models.LogEntry, 0, len(req.Logs))
	now := time.Now()
	for _, logReq := range req.Logs {
		logEntries = append(logEntries, models.LogEntry{
			Timestamp: logReq.Timestamp,
			RuleName:  logReq.RuleName,
			RuleDesc:  logReq.RuleDesc,
			LogLine:   logReq.LogLine,
			LogFile:   logReq.LogFile,
			Pattern:   logReq.Pattern,
			Tag:       logReq.Tag,
			Source:    "agent",
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	// 批量保存到数据库
	var successIDs []uint
	var successCount, failedCount int

	// 使用事务批量插入
	err := h.db.CreateInBatches(&logEntries, 50).Error
	if err != nil {
		// 如果批量插入失败，尝试逐条插入
		for _, entry := range logEntries {
			if err := h.db.Create(&entry).Error; err != nil {
				failedCount++
			} else {
				successCount++
				successIDs = append(successIDs, entry.ID)
			}
		}
	} else {
		// 批量插入成功
		successCount = len(logEntries)
		for _, entry := range logEntries {
			successIDs = append(successIDs, entry.ID)
		}
	}

	c.JSON(http.StatusOK, BatchReceiveLogResponse{
		Success: successCount,
		Failed:  failedCount,
		IDs:     successIDs,
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

// TagStats 标签统计
type TagStats struct {
	Tag   string `json:"tag"`
	Count int64  `json:"count"`
}

// GetTagStats 获取标签及日志数量统计（用于分类管理）
func (h *LogHandler) GetTagStats(c *gin.Context) {
	var stats []TagStats
	h.db.Model(&models.LogEntry{}).
		Select("tag as tag, count(*) as count").
		Where("tag != ? AND tag IS NOT NULL", "").
		Group("tag").
		Order("count DESC").
		Scan(&stats)

	c.JSON(http.StatusOK, gin.H{
		"tags": stats,
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

// UploadLogResponse 文件上传响应结构体
type UploadLogResponse struct {
	Success int   `json:"success"` // 成功数量
	Failed  int   `json:"failed"`  // 失败数量
	Total   int   `json:"total"`   // 总行数
}

// UploadLog 接收文件或文本上传的日志
// 支持 multipart/form-data: file (文件) 或 logs (粘贴的文本)
// 可选参数: tag (标签)
func (h *LogHandler) UploadLog(c *gin.Context) {
	// 限制请求体大小 10MB
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10*1024*1024)

	tag := c.PostForm("tag")

	var lines []string

	// 1. 尝试从文件获取
	file, err := c.FormFile("file")
	if err == nil {
		// 有文件上传
		f, err := file.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "打开文件失败",
				"message": err.Error(),
			})
			return
		}
		defer f.Close()

		content, err := io.ReadAll(f)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "读取文件失败",
				"message": err.Error(),
			})
			return
		}

		lines = parseLogLines(content)
	} else if logsText := c.PostForm("logs"); logsText != "" {
		// 2. 从粘贴的文本获取
		lines = parseLogLines([]byte(logsText))
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": "请上传文件或粘贴日志内容（form 字段: file 或 logs）",
		})
		return
	}

	// 限制最多 10000 行
	if len(lines) > 10000 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "行数超过限制",
			"message": "最多支持 10000 行日志",
		})
		return
	}

	now := time.Now()
	ts := now.Unix()
	entries := make([]models.LogEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entries = append(entries, models.LogEntry{
			Timestamp: ts,
			LogLine:   line,
			Tag:       tag,
			Source:    "manual",
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	if len(entries) == 0 {
		c.JSON(http.StatusOK, UploadLogResponse{
			Success: 0,
			Failed:  0,
			Total:   len(lines),
		})
		return
	}

	// 分批插入，每批 100 条
	var successCount int
	batchSize := 100
	for i := 0; i < len(entries); i += batchSize {
		end := i + batchSize
		if end > len(entries) {
			end = len(entries)
		}
		batch := entries[i:end]
		if err := h.db.CreateInBatches(&batch, 50).Error; err != nil {
			// 逐条重试
			for _, e := range batch {
				if h.db.Create(&e).Error == nil {
					successCount++
				}
			}
		} else {
			successCount += len(batch)
		}
	}

	c.JSON(http.StatusOK, UploadLogResponse{
		Success: successCount,
		Failed:  len(entries) - successCount,
		Total:   len(lines),
	})
}

// ExportLogsRequest 导出日志请求参数
type ExportLogsRequest struct {
	Tag       string `form:"tag"`
	RuleName  string `form:"rule_name"`
	Keyword   string `form:"keyword"`
	StartTime int64  `form:"start_time"`
	EndTime   int64  `form:"end_time"`
	Format    string `form:"format"` // csv 或 json，默认 csv
}

// ExportLogs 按条件导出日志为 CSV 或 JSON
// 最多导出 10000 条
func (h *LogHandler) ExportLogs(c *gin.Context) {
	var req ExportLogsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}

	format := strings.ToLower(req.Format)
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "json" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "格式不支持",
			"message": "format 只能是 csv 或 json",
		})
		return
	}

	query := h.db.Model(&models.LogEntry{})
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

	// 最多导出 10000 条
	var logs []models.LogEntry
	if err := query.Order("timestamp DESC").Limit(10000).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "导出失败",
			"message": err.Error(),
		})
		return
	}

	filename := "logs_export_" + strconv.FormatInt(time.Now().Unix(), 10)
	if format == "csv" {
		filename += ".csv"
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename="+filename)
		writer := csv.NewWriter(c.Writer)
		writer.Write([]string{"id", "timestamp", "tag", "rule_name", "log_line", "log_file", "created_at"})
		for _, l := range logs {
			writer.Write([]string{
				strconv.FormatUint(uint64(l.ID), 10),
				strconv.FormatInt(l.Timestamp, 10),
				l.Tag,
				l.RuleName,
				l.LogLine,
				l.LogFile,
				l.CreatedAt.Format(time.RFC3339),
			})
		}
		writer.Flush()
		return
	}

	// JSON
	filename += ".json"
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Status(http.StatusOK)
	json.NewEncoder(c.Writer).Encode(logs)
}

// parseLogLines 按行解析内容，支持 \n 和 \r\n
func parseLogLines(content []byte) []string {
	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 增大缓冲以支持长行
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}
