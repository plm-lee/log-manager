package handler

import (
	"net/http"
	"strings"
	"time"

	"log-manager/internal/database"
	"log-manager/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BillingHandler 计费处理器
type BillingHandler struct {
	db *gorm.DB
}

// NewBillingHandler 创建计费处理器实例
func NewBillingHandler() *BillingHandler {
	return &BillingHandler{
		db: database.DB,
	}
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
	BillKey     string  `json:"bill_key" binding:"required"`
	MatchType   string  `json:"match_type" binding:"required,oneof=tag rule_name log_line_contains"`
	MatchValue  string  `json:"match_value" binding:"required"`
	TagScope    string  `json:"tag_scope"` // 可选，逗号分隔的 tag 列表；rule_name/log_line_contains 时仅对 scope 内 tag 生效
	UnitPrice   float64 `json:"unit_price" binding:"gte=0"` // 允许 0（免费），required 对 float64 零值会报错
	Description string  `json:"description"`
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
	config := models.BillingConfig{
		BillKey:     req.BillKey,
		MatchType:   req.MatchType,
		MatchValue:  req.MatchValue,
		TagScope:    strings.TrimSpace(req.TagScope),
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
	BillKey     string  `json:"bill_key" binding:"required"`
	MatchType   string  `json:"match_type" binding:"required,oneof=tag rule_name log_line_contains"`
	MatchValue  string  `json:"match_value" binding:"required"`
	TagScope    string  `json:"tag_scope"` // 可选，逗号分隔的 tag 列表
	UnitPrice   float64 `json:"unit_price" binding:"gte=0"` // 允许 0（免费）
	Description string  `json:"description"`
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
	var config models.BillingConfig
	if err := h.db.First(&config, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
		return
	}
	config.BillKey = req.BillKey
	config.MatchType = req.MatchType
	config.MatchValue = req.MatchValue
	config.TagScope = strings.TrimSpace(req.TagScope)
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

// BillingStatItem 计费统计项
type BillingStatItem struct {
	Date      string  `json:"date"`
	BillKey   string  `json:"bill_key"`
	Tag       string  `json:"tag"`
	Count     int64   `json:"count"`
	UnitPrice float64 `json:"unit_price"`
	Amount    float64 `json:"amount"`
}

// GetStatsResponse 计费统计响应
type GetStatsResponse struct {
	Data        []BillingStatItem `json:"data"`
	TotalAmount float64           `json:"total_amount"`
}

// GetUnmatched 获取无匹配规则的计费日志（按日期范围）
func (h *BillingHandler) GetUnmatched(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "缺少参数",
			"message": "start_date 和 end_date 为必填",
		})
		return
	}
	var list []models.UnmatchedBillingLog
	if err := h.db.Where("date >= ? AND date <= ?", startDate, endDate).
		Order("date DESC, count DESC").
		Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "查询失败",
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// GetStats 计费统计
// 从 billing_entries 读取，参数: start_date, end_date (格式 YYYY-MM-DD)
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

	var entries []models.BillingEntry
	q := h.db.Model(&models.BillingEntry{}).
		Where("date >= ? AND date <= ?", startDate, endDate)
	if len(tagFilter) > 0 {
		q = q.Where("tag IN ?", tagFilter)
	}
	if err := q.Order("date ASC, bill_key ASC").Find(&entries).Error; err != nil {
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

	c.JSON(http.StatusOK, GetStatsResponse{
		Data:        result,
		TotalAmount: totalAmount,
	})
}
