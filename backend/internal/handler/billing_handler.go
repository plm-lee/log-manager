package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"log-manager/internal/database"
	"log-manager/internal/models"
	"log-manager/internal/unmatchedqueue"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BillingHandler 计费处理器
type BillingHandler struct {
	db             *gorm.DB
	unmatchedQueue *unmatchedqueue.Queue
}

// NewBillingHandler 创建计费处理器实例，unmatchedQueue 可为 nil
func NewBillingHandler(unmatchedQueue *unmatchedqueue.Queue) *BillingHandler {
	return &BillingHandler{
		db:             database.DB,
		unmatchedQueue: unmatchedQueue,
	}
}

// GetUnmatched 获取无匹配规则的计费日志（归属计费项目 tag 但未命中任何规则）
func (h *BillingHandler) GetUnmatched(c *gin.Context) {
	if h.unmatchedQueue == nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	items := h.unmatchedQueue.Snapshot(200)
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// GetConfigs 获取计费配置列表
func (h *BillingHandler) GetConfigs(c *gin.Context) {
	var configs []models.BillingConfig
	if err := h.db.Order("id ASC").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "查询配置失败",
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": configs})
}

// CreateConfigRequest 新增计费配置请求
type CreateConfigRequest struct {
	BillKey     string   `json:"bill_key" binding:"required"`
	BillingTag  string   `json:"billing_tag"`  // 兼容旧格式：单个 tag
	BillingTags []string `json:"billing_tags"` // 多选时传入数组，优先使用
	MatchType   string   `json:"match_type" binding:"required,oneof=tag rule_name log_line_contains"`
	MatchValue  string   `json:"match_value" binding:"required"`
	UnitPrice   float64  `json:"unit_price" binding:"gte=0"` // 允许 0（免费）
	Description string   `json:"description"`
}

func resolveBillingTag(tags []string, single string) string {
	if len(tags) > 0 {
		trimmed := make([]string, 0, len(tags))
		for _, t := range tags {
			s := strings.TrimSpace(t)
			if s != "" {
				trimmed = append(trimmed, s)
			}
		}
		if len(trimmed) > 0 {
			return strings.Join(trimmed, ",")
		}
	}
	return strings.TrimSpace(single)
}

// CreateConfig 新增计费配置
func (h *BillingHandler) CreateConfig(c *gin.Context) {
	var req CreateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}
	billingTag := resolveBillingTag(req.BillingTags, req.BillingTag)
	if billingTag == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": "请选择至少一个计费 Tag",
		})
		return
	}
	config := models.BillingConfig{
		BillKey:     req.BillKey,
		BillingTag:  billingTag,
		MatchType:   req.MatchType,
		MatchValue:  req.MatchValue,
		UnitPrice:   req.UnitPrice,
		Description: req.Description,
	}
	if err := h.db.Create(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "创建配置失败",
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": config})
}

// UpdateConfigRequest 更新计费配置请求
type UpdateConfigRequest struct {
	BillKey     string   `json:"bill_key" binding:"required"`
	BillingTag  string   `json:"billing_tag"`
	BillingTags []string `json:"billing_tags"`
	MatchType   string   `json:"match_type" binding:"required,oneof=tag rule_name log_line_contains"`
	MatchValue  string   `json:"match_value" binding:"required"`
	UnitPrice   float64  `json:"unit_price" binding:"gte=0"`
	Description string   `json:"description"`
}

// UpdateConfig 更新计费配置
func (h *BillingHandler) UpdateConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少配置ID"})
		return
	}
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": err.Error(),
		})
		return
	}
	billingTag := resolveBillingTag(req.BillingTags, req.BillingTag)
	if billingTag == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "请求参数错误",
			"message": "请选择至少一个计费 Tag",
		})
		return
	}
	var config models.BillingConfig
	if err := h.db.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}
	config.BillKey = req.BillKey
	config.BillingTag = billingTag
	config.MatchType = req.MatchType
	config.MatchValue = req.MatchValue
	config.UnitPrice = req.UnitPrice
	config.Description = req.Description
	if err := h.db.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "更新配置失败",
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": config})
}

// GetTags 从 billing_entries 获取实际产生计费记录的标签列表，供前端下拉选择
func (h *BillingHandler) GetTags(c *gin.Context) {
	var values []string
	if err := h.db.Model(&models.BillingEntry{}).
		Where("tag != ''").
		Distinct("tag").
		Pluck("tag", &values).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "查询标签失败",
			"message": err.Error(),
		})
		return
	}
	if values == nil {
		values = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"data": values})
}

// DeleteConfig 删除计费配置
func (h *BillingHandler) DeleteConfig(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少配置ID"})
		return
	}
	if err := h.db.Delete(&models.BillingConfig{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "删除配置失败",
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// BillingStatItem 计费统计项（明细）
type BillingStatItem struct {
	Date      string  `json:"date"`
	BillKey   string  `json:"bill_key"`
	Tag       string  `json:"tag"`
	Count     int64   `json:"count"`
	UnitPrice float64 `json:"unit_price"`
	Amount    float64 `json:"amount"`
}

// DailyStatItem 按日汇总项
type DailyStatItem struct {
	Date        string  `json:"date"`
	TotalCount  int64   `json:"total_count"`
	TotalAmount float64 `json:"total_amount"`
}

// GetStatsResponse 计费统计响应（明细模式）
type GetStatsResponse struct {
	Data        []BillingStatItem `json:"data"`
	Total       int64             `json:"total,omitempty"`
	TotalAmount float64           `json:"total_amount"`
}

// GetStatsSummaryResponse 按日汇总响应
type GetStatsSummaryResponse struct {
	Data        []DailyStatItem `json:"data"`
	Total       int64           `json:"total"`
	TotalAmount float64         `json:"total_amount"`
}

// GetStats 计费统计
// 从 billing_entries 读取
// 参数: start_date, end_date (格式 YYYY-MM-DD)
// group_by=day: 按日汇总，返回 DailyStatItem，支持 page/page_size，按日期倒序
// group_by=detail 或 传 date: 返回指定日期的明细 BillingStatItem，支持 page/page_size
func (h *BillingHandler) GetStats(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "缺少参数",
			"message": "start_date 和 end_date 为必填",
		})
		return
	}
	if _, err := time.ParseInLocation("2006-01-02", startDate, time.Local); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "start_date 格式错误",
			"message": "应为 YYYY-MM-DD",
		})
		return
	}
	if _, err := time.ParseInLocation("2006-01-02", endDate, time.Local); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "end_date 格式错误",
			"message": "应为 YYYY-MM-DD",
		})
		return
	}

	// 可选 tags 过滤：直接按 billing_entries.tag 过滤
	tags := c.QueryArray("tags")
	var tagFilter []string
	if len(tags) > 0 {
		for _, t := range tags {
			t = strings.TrimSpace(t)
			if t != "" {
				tagFilter = append(tagFilter, t)
			}
		}
	}

	targetDate := c.Query("date")

	// 日明细模式：传 date 时返回该日明细
	if targetDate != "" {
		if _, err := time.ParseInLocation("2006-01-02", targetDate, time.Local); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "date 格式错误",
				"message": "应为 YYYY-MM-DD",
			})
			return
		}
		h.getStatsDetail(c, targetDate, tagFilter)
		return
	}

	// 按日汇总模式（默认）
	h.getStatsSummary(c, startDate, endDate, tagFilter)
}

func (h *BillingHandler) getStatsSummary(c *gin.Context, startDate, endDate string, tagFilter []string) {
	page := 1
	if p := c.Query("page"); p != "" {
		if v, err := parseIntDefault(p, 1); err == nil && v >= 1 {
			page = v
		}
	}
	pageSize := 20
	if ps := c.Query("page_size"); ps != "" {
		if v, err := parseIntDefault(ps, 20); err == nil && v >= 1 && v <= 100 {
			pageSize = v
		}
	}
	offset := (page - 1) * pageSize

	baseQ := h.db.Model(&models.BillingEntry{}).
		Where("date >= ? AND date <= ?", startDate, endDate)
	if len(tagFilter) > 0 {
		baseQ = baseQ.Where("tag IN ?", tagFilter)
	}

	type dailyRow struct {
		Date        string
		TotalCount  int64
		TotalAmount float64
	}

	var totalDays int64
	if err := baseQ.Distinct("date").Count(&totalDays).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "统计失败",
			"message": err.Error(),
		})
		return
	}

	var rows []dailyRow
	// 按 date DESC 排序，分页
	subQ := baseQ.Select("date, SUM(count) as total_count, SUM(amount) as total_amount").
		Group("date").
		Order("date DESC").
		Limit(pageSize).
		Offset(offset)
	if err := subQ.Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "统计失败",
			"message": err.Error(),
		})
		return
	}

	// 计算全量 total_amount（汇总金额，使用 Row().Scan 确保正确读取聚合结果）
	var totalAmount float64
	sumQ := h.db.Model(&models.BillingEntry{}).
		Where("date >= ? AND date <= ?", startDate, endDate)
	if len(tagFilter) > 0 {
		sumQ = sumQ.Where("tag IN ?", tagFilter)
	}
	row := sumQ.Select("COALESCE(SUM(amount), 0)").Row()
	if err := row.Scan(&totalAmount); err != nil {
		totalAmount = 0
	}

	result := make([]DailyStatItem, 0, len(rows))
	for _, r := range rows {
		result = append(result, DailyStatItem{
			Date:        r.Date,
			TotalCount:  r.TotalCount,
			TotalAmount: r.TotalAmount,
		})
	}

	c.JSON(http.StatusOK, GetStatsSummaryResponse{
		Data:        result,
		Total:       totalDays,
		TotalAmount: totalAmount,
	})
}

func (h *BillingHandler) getStatsDetail(c *gin.Context, date string, tagFilter []string) {
	page := 1
	if p := c.Query("page"); p != "" {
		if v, err := parseIntDefault(p, 1); err == nil && v >= 1 {
			page = v
		}
	}
	pageSize := 20
	if ps := c.Query("page_size"); ps != "" {
		if v, err := parseIntDefault(ps, 20); err == nil && v >= 1 && v <= 100 {
			pageSize = v
		}
	}
	offset := (page - 1) * pageSize

	q := h.db.Model(&models.BillingEntry{}).Where("date = ?", date)
	if len(tagFilter) > 0 {
		q = q.Where("tag IN ?", tagFilter)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "统计失败",
			"message": err.Error(),
		})
		return
	}

	var entries []models.BillingEntry
	if err := q.Order("bill_key ASC, tag ASC").
		Limit(pageSize).
		Offset(offset).
		Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "统计失败",
			"message": err.Error(),
		})
		return
	}

	var result []BillingStatItem
	var totalAmount float64
	for _, e := range entries {
		unitPrice := 0.0
		if e.Count > 0 {
			unitPrice = e.Amount / float64(e.Count)
		}
		result = append(result, BillingStatItem{
			Date:      e.Date,
			BillKey:   e.BillKey,
			Tag:       e.Tag,
			Count:     e.Count,
			UnitPrice: unitPrice,
			Amount:    e.Amount,
		})
		totalAmount += e.Amount
	}

	// 详情页的总金额应为该日全量（含 tag 筛选）
	var sumRes struct {
		S float64 `gorm:"column:s"`
	}
	sumQ := h.db.Model(&models.BillingEntry{}).Where("date = ?", date)
	if len(tagFilter) > 0 {
		sumQ = sumQ.Where("tag IN ?", tagFilter)
	}
	sumQ.Select("COALESCE(SUM(amount), 0) as s").Scan(&sumRes)
	dayTotal := sumRes.S

	c.JSON(http.StatusOK, GetStatsResponse{
		Data:        result,
		Total:       total,
		TotalAmount: dayTotal,
	})
}

func parseIntDefault(s string, defaultVal int) (int, error) {
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal, err
	}
	return v, nil
}
