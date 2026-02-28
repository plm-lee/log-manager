package handler

import (
	"net/http"
	"sort"
	"strings"

	"log-manager/internal/database"
	"log-manager/internal/models"
	"log-manager/internal/tagcache"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TagHandler 标签与大项目管理
type TagHandler struct {
	db                   *gorm.DB
	tagCache             *tagcache.Cache
	invalidateBillingCache func()
}

// NewTagHandler 创建 TagHandler
// invalidateBillingCache 可选，在 SetTagProject 成功后调用以使计费相关缓存立即生效
func NewTagHandler(tagCache *tagcache.Cache, invalidateBillingCache func()) *TagHandler {
	return &TagHandler{
		db:                   database.DB,
		tagCache:             tagCache,
		invalidateBillingCache: invalidateBillingCache,
	}
}

// ManagedTag 带项目信息的 tag（用于分类管理）
type ManagedTag struct {
	Tag         string  `json:"tag"`
	Count       int64   `json:"count"`
	ProjectID   *uint   `json:"project_id"`
	ProjectName string  `json:"project_name"`
	ProjectType string  `json:"project_type"`
}

// parseTagNames 解析逗号分隔的 tag 字符串为 tag 列表（与 log_handler.parseLogTags 逻辑一致）
func parseTagNames(s string) []string {
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

// GetManagedTags 获取 tag 列表（含项目信息、日志数）
// log_entries.tag 可能为逗号分隔（如 tag1,tag2），会拆分为独立 tag 分别统计
func (h *TagHandler) GetManagedTags(c *gin.Context) {
	var stats []struct {
		Tag   string
		Count int64
	}
	if err := h.db.Model(&models.LogEntry{}).
		Select("tag as tag, count(*) as count").
		Where("tag != '' AND tag IS NOT NULL").
		Group("tag").
		Order("count DESC").
		Scan(&stats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 按逗号拆分后聚合：每个独立 tag 的 count 累加
	agg := make(map[string]int64)
	for _, s := range stats {
		for _, name := range parseTagNames(s.Tag) {
			agg[name] += s.Count
		}
	}
	var tags []models.Tag
	if err := h.db.Preload("Project").Find(&tags).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tagByKey := make(map[string]*models.Tag)
	for i := range tags {
		tagByKey[tags[i].Name] = &tags[i]
	}
	// 确保所有在 log_entries 中出现的 tag 都有记录（可能尚未在 tags 表中）
	for name := range agg {
		if tagByKey[name] == nil {
			t := models.Tag{Name: name}
			_ = h.db.Where("name = ?", name).FirstOrCreate(&t).Error
			_ = h.db.Preload("Project").First(&t, t.ID).Error
			tagByKey[name] = &t
		}
	}
	// 按 count 降序构建结果
	type pair struct {
		name  string
		count int64
	}
	pairs := make([]pair, 0, len(agg))
	for name, count := range agg {
		pairs = append(pairs, pair{name: name, count: count})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].count > pairs[j].count })
	result := make([]ManagedTag, 0, len(pairs))
	for _, p := range pairs {
		t := tagByKey[p.name]
		mt := ManagedTag{Tag: p.name, Count: p.count}
		if t != nil {
			mt.ProjectID = t.ProjectID
			if t.Project != nil {
				mt.ProjectName = t.Project.Name
				mt.ProjectType = t.Project.Type
			}
		}
		result = append(result, mt)
	}
	c.JSON(http.StatusOK, gin.H{"tags": result})
}

// SetTagProjectReq 设置 tag 所属项目
type SetTagProjectReq struct {
	ProjectID *uint `json:"project_id"`
}

// SetTagProject 设置 tag 所属项目
func (h *TagHandler) SetTagProject(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag 名称不能为空"})
		return
	}
	var req SetTagProjectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var tag models.Tag
	if err := h.db.Where("name = ?", name).FirstOrCreate(&tag, models.Tag{Name: name}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tag.ProjectID = req.ProjectID
	if err := h.db.Save(&tag).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.tagCache != nil {
		_ = h.tagCache.Reload()
	}
	if h.invalidateBillingCache != nil {
		h.invalidateBillingCache()
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListTagProjects 大项目列表
func (h *TagHandler) ListTagProjects(c *gin.Context) {
	var list []models.TagProject
	if err := h.db.Order("type DESC, id ASC").Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// CreateTagProjectReq 创建大项目
type CreateTagProjectReq struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// CreateTagProject 创建大项目
func (h *TagHandler) CreateTagProject(c *gin.Context) {
	var req CreateTagProjectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p := models.TagProject{
		Name:        strings.TrimSpace(req.Name),
		Type:        "normal",
		Description: strings.TrimSpace(req.Description),
	}
	if err := h.db.Create(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": p})
}

// UpdateTagProjectReq 更新大项目
type UpdateTagProjectReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UpdateTagProject 更新大项目
func (h *TagHandler) UpdateTagProject(c *gin.Context) {
	id := c.Param("id")
	var p models.TagProject
	if err := h.db.First(&p, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}
	var req UpdateTagProjectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Name != "" {
		p.Name = strings.TrimSpace(req.Name)
	}
	if req.Description != "" || c.Request.ContentLength > 0 {
		p.Description = strings.TrimSpace(req.Description)
	}
	if err := h.db.Save(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": p})
}

// GetBillingProjectTags 获取归属计费项目的 tag 列表（供计费配置新增时选择）
func (h *TagHandler) GetBillingProjectTags(c *gin.Context) {
	var billingProject models.TagProject
	if err := h.db.Where("type = ?", "billing").First(&billingProject).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}
	var names []string
	if err := h.db.Model(&models.Tag{}).Where("project_id = ?", billingProject.ID).Pluck("name", &names).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if names == nil {
		names = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"data": names})
}

// DeleteTagProject 删除大项目（计费项目不可删）
func (h *TagHandler) DeleteTagProject(c *gin.Context) {
	id := c.Param("id")
	var p models.TagProject
	if err := h.db.First(&p, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}
	if p.Type == "billing" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "计费项目不可删除"})
		return
	}
	if err := h.db.Model(&models.Tag{}).Where("project_id = ?", p.ID).Update("project_id", nil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Delete(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.tagCache != nil {
		_ = h.tagCache.Reload()
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
