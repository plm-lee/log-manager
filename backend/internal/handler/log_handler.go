package handler

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"log-manager/internal/database"
	"log-manager/internal/fulltext"
	"log-manager/internal/models"
	"log-manager/internal/rulecache"
	"log-manager/internal/tagcache"
	"log-manager/internal/taglogcount"
	"log-manager/internal/unmatchedqueue"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// indexedBillingConfig 按 match_type 索引的计费配置，用于分层匹配
type indexedBillingConfig struct {
	byTag             []models.BillingConfig // match_type=tag
	byRuleName        []models.BillingConfig // match_type=rule_name
	byLogLineContains []models.BillingConfig // match_type=log_line_contains
	tagSet            map[string]struct{}    // tag 规则的 match_value 集合
	billingTagSet     map[string]struct{}    // 归属计费项目的 tag，用于优先判断是否参与计费匹配
}

// BillingConfigCache BillingConfig 内存缓存，减少 DB 查询，返回索引化结构
type BillingConfigCache struct {
	mu       sync.RWMutex
	indexed  *indexedBillingConfig
	loadedAt time.Time
	ttl      time.Duration
}

// NewBillingConfigCache 创建计费配置缓存
func NewBillingConfigCache(ttl time.Duration) *BillingConfigCache {
	return &BillingConfigCache{ttl: ttl}
}

// Invalidate 使缓存失效，下次 get 时将重新从 DB 加载（用于 tag 归属或计费配置变更后立即生效）
func (c *BillingConfigCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.indexed = nil
	c.loadedAt = time.Time{}
}

func (c *BillingConfigCache) get(db *gorm.DB) (*indexedBillingConfig, error) {
	c.mu.RLock()
	if time.Since(c.loadedAt) < c.ttl && c.indexed != nil {
		idx := c.indexed
		c.mu.RUnlock()
		return idx, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.loadedAt) < c.ttl && c.indexed != nil {
		return c.indexed, nil
	}
	var configs []models.BillingConfig
	if err := db.Order("id ASC").Find(&configs).Error; err != nil {
		return nil, err
	}
	idx := buildIndexedBillingConfig(configs)
	idx.billingTagSet = loadBillingTagSet(db)
	c.indexed = idx
	c.loadedAt = time.Now()
	return idx, nil
}

// loadBillingTagSet 从 tags 表加载归属计费项目的 tag 集合
func loadBillingTagSet(db *gorm.DB) map[string]struct{} {
	set := make(map[string]struct{})
	var projectID uint
	if err := db.Model(&models.TagProject{}).Where("type = ?", "billing").Limit(1).Pluck("id", &projectID).Error; err != nil || projectID == 0 {
		return set
	}
	var names []string
	if err := db.Model(&models.Tag{}).Where("project_id = ?", projectID).Pluck("name", &names).Error; err != nil {
		return set
	}
	for _, n := range names {
		if n != "" {
			set[strings.TrimSpace(n)] = struct{}{}
		}
	}
	return set
}

func buildIndexedBillingConfig(configs []models.BillingConfig) *indexedBillingConfig {
	idx := &indexedBillingConfig{
		byTag:             make([]models.BillingConfig, 0),
		byRuleName:        make([]models.BillingConfig, 0),
		byLogLineContains: make([]models.BillingConfig, 0),
		tagSet:            make(map[string]struct{}),
		billingTagSet:     make(map[string]struct{}),
	}
	for _, cfg := range configs {
		switch cfg.MatchType {
		case "tag":
			idx.byTag = append(idx.byTag, cfg)
			idx.tagSet[cfg.MatchValue] = struct{}{}
		case "rule_name":
			idx.byRuleName = append(idx.byRuleName, cfg)
		case "log_line_contains":
			idx.byLogLineContains = append(idx.byLogLineContains, cfg)
		}
	}
	return idx
}

// LogHandler 日志处理器
// 负责处理日志相关的 HTTP 请求
type LogHandler struct {
	db            *gorm.DB
	bccache       *BillingConfigCache
	tagCache      *tagcache.Cache
	ruleCache     *rulecache.Cache
	unmatchedQueue *unmatchedqueue.Queue
}

// NewLogHandler 创建日志处理器实例
// tagCache、ruleCache 可为 nil；unmatchedQueue 可为 nil；bcCache 可为 nil，为 nil 时内部新建（TTL 60s）
func NewLogHandler(tagCache *tagcache.Cache, ruleCache *rulecache.Cache, unmatchedQueue *unmatchedqueue.Queue, bcCache *BillingConfigCache) *LogHandler {
	if bcCache == nil {
		bcCache = &BillingConfigCache{ttl: 60 * time.Second}
	}
	return &LogHandler{
		db:             database.DB,
		bccache:        bcCache,
		tagCache:       tagCache,
		ruleCache:      ruleCache,
		unmatchedQueue: unmatchedQueue,
	}
}

// isBillingTag 判断 tag 是否归属计费项目（仅计费项目 tag 才参与计费规则匹配）
// 支持逗号分隔多 tag：任一 tag 在 billingTagSet 中即返回 true
func isBillingTag(tagStr string, idx *indexedBillingConfig) bool {
	tags := parseLogTags(tagStr)
	if len(tags) == 0 || idx.billingTagSet == nil {
		return false
	}
	for _, t := range tags {
		if _, ok := idx.billingTagSet[t]; ok {
			return true
		}
	}
	return false
}

// billingTagContains 判断 cfg.BillingTag（逗号拼接）是否包含 tag
func billingTagContains(billingTagStr, tag string) bool {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return false
	}
	for _, t := range strings.Split(billingTagStr, ",") {
		if strings.TrimSpace(t) == tag {
			return true
		}
	}
	return false
}

// parseLogTags 解析逗号分隔的 tag 字符串为 tag 列表（log-filter-monitor 可能上报 "tag1,tag2,tag3"）
func parseLogTags(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// anyLogTagInBillingTag 判断 log 的 tag 列表中是否有任一 tag 在 cfg.BillingTag 内
func anyLogTagInBillingTag(logTags []string, billingTagStr string) bool {
	for _, t := range logTags {
		if billingTagContains(billingTagStr, t) {
			return true
		}
	}
	return false
}

// matchBillingConfigs 检查日志是否匹配计费配置，返回匹配的配置列表
// 匹配逻辑：log.tag 支持逗号分隔多 tag，任一 tag 在 cfg.BillingTag 内且满足 match_type 即匹配
// 注意：仅对归属计费项目的 tag 调用此函数
func matchBillingConfigs(req ReceiveLogRequest, idx *indexedBillingConfig) []models.BillingConfig {
	logTags := parseLogTags(req.Tag)
	if len(logTags) == 0 {
		return nil
	}
	var matched []models.BillingConfig
	for _, cfg := range idx.byTag {
		if !anyLogTagInBillingTag(logTags, cfg.BillingTag) {
			continue
		}
		for _, t := range logTags {
			if billingTagContains(cfg.BillingTag, t) && (cfg.MatchValue == t || strings.Contains(t, cfg.MatchValue)) {
				matched = append(matched, cfg)
				break
			}
		}
	}
	for _, cfg := range idx.byRuleName {
		if !anyLogTagInBillingTag(logTags, cfg.BillingTag) {
			continue
		}
		if strings.Contains(req.RuleName, cfg.MatchValue) {
			matched = append(matched, cfg)
		}
	}
	for _, cfg := range idx.byLogLineContains {
		if !anyLogTagInBillingTag(logTags, cfg.BillingTag) {
			continue
		}
		if strings.Contains(req.LogLine, cfg.MatchValue) {
			matched = append(matched, cfg)
		}
	}
	return matched
}

// upsertBillingEntry 按 (date, bill_key, tag) 聚合计费数据，存在则累加
func (h *LogHandler) upsertBillingEntry(date string, billKey string, tag string, addCount int64, addAmount float64) error {
	now := time.Now()
	entry := models.BillingEntry{
		Date:      date,
		BillKey:   billKey,
		Tag:       tag,
		Count:     addCount,
		Amount:    addAmount,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return h.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "date"}, {Name: "bill_key"}, {Name: "tag"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"count":      gorm.Expr("count + ?", addCount),
			"amount":     gorm.Expr("amount + ?", addAmount),
			"updated_at": now,
		}),
	}).Create(&entry).Error
}

// ReceiveLogRequest 接收日志请求结构体
// 对应 log-filter-monitor 上报的日志数据格式（HTTP 与 UDP 共用）
type ReceiveLogRequest struct {
	Timestamp int64  `json:"timestamp" binding:"required"` // 时间戳
	RuleName  string `json:"rule_name"`                    // 规则名称
	RuleDesc  string `json:"rule_desc"`                    // 规则描述
	LogLine   string `json:"log_line" binding:"required"`  // 日志行内容
	LogFile   string `json:"log_file"`                     // 日志文件路径
	Pattern   string `json:"pattern"`                      // 匹配模式
	Tag       string `json:"tag"`                          // 标签
	Secret    string `json:"secret"`                       // UDP 认证密钥（可选，与 udp.secret 一致时校验）
	APIKey    string `json:"api_key"`                      // 同 secret，兼容两种字段名
	Transport string `json:"-"`                            // 来源：http / udp，内部标记，不入库
}

// ProcessLogBatch 批量处理日志（计费分流 + 入库），供 HTTP 与 UDP 共用
// 返回：成功数、失败数、非计费日志的 ID 列表、错误
func (h *LogHandler) ProcessLogBatch(logs []ReceiveLogRequest) (successCount, failedCount int, ids []uint, err error) {
	if len(logs) == 0 {
		return 0, 0, nil, nil
	}
	transport := "http"
	if len(logs) > 0 && logs[0].Transport != "" {
		transport = logs[0].Transport
	}
	log.Printf("[log] 收到 %d 条日志，来源: %s", len(logs), transport)
	if len(logs) > 100 {
		logs = logs[:100] // 单次最多处理 100 条
	}

	idx, err := h.bccache.get(h.db)
	if err != nil {
		return 0, 0, nil, err
	}

	agg := make(map[string]*billingAggregate)
	logEntries := make([]models.LogEntry, 0, len(logs))
	now := time.Now()

	for _, logReq := range logs {
		if h.tagCache != nil && logReq.Tag != "" {
			for _, t := range parseLogTags(logReq.Tag) {
				_ = h.tagCache.EnsureTag(t)
			}
		}
		if h.ruleCache != nil && logReq.RuleName != "" {
			_ = h.ruleCache.EnsureRuleName(logReq.RuleName)
		}
		tag := strings.TrimSpace(logReq.Tag)
		if !isBillingTag(logReq.Tag, idx) {
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
			continue
		}
		matched := matchBillingConfigs(logReq, idx)
		if len(matched) > 0 {
			date := time.Unix(logReq.Timestamp, 0).Format("2006-01-02")
			for _, cfg := range matched {
				key := date + "|" + cfg.BillKey + "|" + tag
				if agg[key] == nil {
					agg[key] = &billingAggregate{}
				}
				agg[key].count++
				agg[key].amount += cfg.UnitPrice
			}
		} else {
			if h.unmatchedQueue != nil {
				h.unmatchedQueue.Add(tag, logReq.RuleName, logReq.LogLine)
			}
		}
	}

	err = h.db.Transaction(func(tx *gorm.DB) error {
		if len(agg) > 0 {
			entries := make([]models.BillingEntry, 0, len(agg))
			for key, v := range agg {
				// key: date|bill_key|tag
				parts := strings.SplitN(key, "|", 3)
				if len(parts) < 2 {
					continue
				}
				tag := ""
				if len(parts) == 3 {
					tag = parts[2]
				}
				entries = append(entries, models.BillingEntry{
					Date:      parts[0],
					BillKey:   parts[1],
					Tag:       tag,
					Count:     v.count,
					Amount:    v.amount,
					CreatedAt: now,
					UpdatedAt: now,
				})
			}
			if len(entries) > 0 {
				if err := tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "date"}, {Name: "bill_key"}, {Name: "tag"}},
					DoUpdates: clause.Assignments(map[string]interface{}{
						"count":      gorm.Expr("count + VALUES(`count`)"),
						"amount":     gorm.Expr("amount + VALUES(`amount`)"),
						"updated_at": now,
					}),
				}).CreateInBatches(&entries, 100).Error; err != nil {
					return err
				}
				for _, v := range agg {
					successCount += int(v.count)
				}
			}
		}
		if len(logEntries) > 0 {
			if err := tx.CreateInBatches(&logEntries, 50).Error; err != nil {
				return err
			}
			successCount += len(logEntries)
			for i := range logEntries {
				ids = append(ids, logEntries[i].ID)
			}
			deltas := make(map[string]int64)
			for _, e := range logEntries {
				for _, t := range parseLogTags(e.Tag) {
					deltas[t]++
				}
			}
			if err := taglogcount.IncrByTagDeltas(tx, deltas); err != nil {
				return err
			}
		}
		return nil
	})
	return successCount, failedCount, ids, err
}

// ReceiveLog 接收日志数据
// 处理来自 log-filter-monitor 的日志上报请求
// 匹配计费配置的日志写入 billing_entries，不写入 log_entries
func (h *LogHandler) ReceiveLog(c *gin.Context) {
	var req ReceiveLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}
	req.Transport = "http"
	successCount, _, ids, err := h.ProcessLogBatch([]ReceiveLogRequest{req})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "保存日志失败",
			"message": err.Error(),
		})
		return
	}
	var respID interface{}
	if len(ids) > 0 {
		respID = ids[0]
	}
	c.JSON(http.StatusOK, gin.H{
		"success": successCount > 0,
		"id":      respID,
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

// billingAggregate 按 (date, bill_key) 聚合计费数据
type billingAggregate struct {
	count  int64
	amount float64
}

// BatchReceiveLog 批量接收日志数据
// 支持一次性接收多条日志，提高吞吐量
// 匹配计费配置的日志写入 billing_entries，不写入 log_entries
func (h *LogHandler) BatchReceiveLog(c *gin.Context) {
	var req BatchReceiveLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}

	if len(req.Logs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "批量大小超过限制",
			"message": "最多支持100条日志",
		})
		return
	}
	for i := range req.Logs {
		req.Logs[i].Transport = "http"
	}
	successCount, failedCount, successIDs, err := h.ProcessLogBatch(req.Logs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "保存日志失败",
			"message": err.Error(),
		})
		return
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

	// 应用筛选条件（tag 支持逗号分隔，按单个 tag 匹配包含该 tag 的日志）
	if req.Tag != "" {
		t := req.Tag
		query = query.Where("tag = ? OR tag LIKE ? OR tag LIKE ? OR tag LIKE ?", t, t+",%", "%,"+t, "%,"+t+",%")
	}
	if req.RuleName != "" {
		query = query.Where("rule_name = ?", req.RuleName)
	}
	query = fulltext.ApplyLogLineKeyword(query, req.Keyword)
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
// 从 tags 表读取（写入时维护），避免 log_entries 全表 Distinct 慢查询
func (h *LogHandler) GetTags(c *gin.Context) {
	var tags []string
	if err := h.db.Model(&models.Tag{}).
		Where("name != ? AND name IS NOT NULL", "").
		Pluck("name", &tags).Error; err != nil {
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
// 从 tag_log_counts 读取（写入时更新），避免 log_entries 全表 Group 慢查询
func (h *LogHandler) GetTagStats(c *gin.Context) {
	var stats []TagStats
	if err := h.db.Model(&models.TagLogCount{}).
		Select("tag as tag, count as count").
		Where("tag != ? AND tag IS NOT NULL", "").
		Order("count DESC").
		Scan(&stats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "查询标签统计失败",
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"tags": stats,
	})
}

// GetRuleNames 获取所有规则名称列表
// 从 rule_names 表读取（写入时维护），避免 log_entries 全表 Distinct 慢查询
func (h *LogHandler) GetRuleNames(c *gin.Context) {
	var ruleNames []string
	if err := h.db.Model(&models.RuleName{}).
		Where("name != ? AND name IS NOT NULL", "").
		Pluck("name", &ruleNames).Error; err != nil {
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

	tag := strings.TrimSpace(c.PostForm("tag"))
	if h.tagCache != nil && tag != "" {
		_ = h.tagCache.EnsureTag(tag)
	}

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
			// 逐条重试（此分支不更新 tag_log_counts，避免统计偏差）
			for _, e := range batch {
				if h.db.Create(&e).Error == nil {
					successCount++
				}
			}
		} else {
			successCount += len(batch)
			deltas := make(map[string]int64)
			for _, e := range batch {
				for _, t := range parseLogTags(e.Tag) {
					deltas[t]++
				}
			}
			_ = taglogcount.IncrByTagDeltas(h.db, deltas)
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
		t := req.Tag
		query = query.Where("tag = ? OR tag LIKE ? OR tag LIKE ? OR tag LIKE ?", t, t+",%", "%,"+t, "%,"+t+",%")
	}
	if req.RuleName != "" {
		query = query.Where("rule_name = ?", req.RuleName)
	}
	query = fulltext.ApplyLogLineKeyword(query, req.Keyword)
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
