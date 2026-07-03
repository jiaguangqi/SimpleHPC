package httpapi

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"simplehpc/backend/internal/service"
)

func (api *API) currentUser(c *gin.Context) (service.AuthUser, bool) {
	token := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	if token == "" {
		token, _ = c.Cookie("simplehpc_session")
	}
	user, err := api.services.SessionUser(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "登录已失效，请重新登录"})
		return service.AuthUser{}, false
	}
	return user, true
}

func (api *API) requireAdmin(c *gin.Context) (service.AuthUser, bool) {
	user, ok := api.currentUser(c)
	if !ok {
		return service.AuthUser{}, false
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return service.AuthUser{}, false
	}
	return user, true
}

func (api *API) templateManager(c *gin.Context) (service.AuthUser, bool) {
	user, ok := api.currentUser(c)
	if !ok {
		return user, false
	}
	if !service.IsTemplateManager(user) {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要集群管理员或配置管理员权限"})
		return user, false
	}
	return user, true
}

func templateID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效模板编号"})
		return 0, false
	}
	return id, true
}

func (api *API) listJobTemplates(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	items, err := api.services.ListJobTemplates(c.Request.Context(), user)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if authz, exists := permissionContext(c); exists {
		scoped, scopeErr := api.services.FilterJobTemplatesByScope(c.Request.Context(), authz, items)
		if scopeErr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": scopeErr.Error()})
			return
		}
		if normalizeRBACMode(api.cfg.RBACMode) == rbacShadow {
			api.recordRBACScopeShadow(c, user, "data.templates.list", len(items), len(scoped))
		}
		if normalizeRBACMode(api.cfg.RBACMode) == rbacEnforce {
			items = scoped
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items), "canManage": service.IsTemplateManager(user)})
}

func (api *API) getJobTemplate(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	id, ok := templateID(c)
	if !ok {
		return
	}
	item, err := api.services.GetJobTemplate(c.Request.Context(), id, user)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "模板不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (api *API) createJobTemplate(c *gin.Context) {
	user, ok := api.templateManager(c)
	if !ok {
		return
	}
	var item service.JobTemplate
	if c.ShouldBindJSON(&item) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "模板数据格式错误"})
		return
	}
	item.ID = 0
	saved, err := api.services.SaveJobTemplate(c.Request.Context(), item, user.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, saved)
}
func (api *API) updateJobTemplate(c *gin.Context) {
	user, ok := api.templateManager(c)
	if !ok {
		return
	}
	id, ok := templateID(c)
	if !ok {
		return
	}
	var item service.JobTemplate
	if c.ShouldBindJSON(&item) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "模板数据格式错误"})
		return
	}
	item.ID = id
	saved, err := api.services.SaveJobTemplate(c.Request.Context(), item, user.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, saved)
}
func (api *API) deleteJobTemplate(c *gin.Context) {
	_, ok := api.templateManager(c)
	if !ok {
		return
	}
	id, ok := templateID(c)
	if !ok {
		return
	}
	if err := api.services.DeleteJobTemplate(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
func (api *API) setJobTemplateStatus(status string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := api.templateManager(c)
		if !ok {
			return
		}
		id, ok := templateID(c)
		if !ok {
			return
		}
		if err := api.services.SetTemplateStatus(c.Request.Context(), id, status, user.Username); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "status": status})
	}
}
func (api *API) setTemplateGrants(c *gin.Context) {
	user, ok := api.templateManager(c)
	if !ok {
		return
	}
	id, ok := templateID(c)
	if !ok {
		return
	}
	var body struct {
		Grants []service.TemplateGrant `json:"grants"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "授权数据格式错误"})
		return
	}
	if err := api.services.SetTemplateGrants(c.Request.Context(), id, body.Grants, user.Username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
func (api *API) requestTemplateAccess(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	id, ok := templateID(c)
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	if err := api.services.RequestTemplateAccess(c.Request.Context(), id, user.Username, body.Reason); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true})
}
func (api *API) listTemplateRequests(c *gin.Context) {
	if _, ok := api.templateManager(c); !ok {
		return
	}
	items, err := api.services.ListTemplateRequests(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
func (api *API) reviewTemplateRequest(approve bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := api.templateManager(c)
		if !ok {
			return
		}
		id, err := strconv.ParseInt(c.Param("requestId"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效申请编号"})
			return
		}
		if err := api.services.ReviewTemplateRequest(c.Request.Context(), id, approve, user.Username); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}
func (api *API) previewTemplate(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	id, ok := templateID(c)
	if !ok {
		return
	}
	var values map[string]any
	if c.ShouldBindJSON(&values) != nil {
		values = map[string]any{}
	}
	script, err := api.services.PreviewTemplate(c.Request.Context(), id, user, values)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"script": script})
}
func (api *API) submitTemplate(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	id, ok := templateID(c)
	if !ok {
		return
	}
	var values map[string]any
	if c.ShouldBindJSON(&values) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "提交参数格式错误"})
		return
	}
	run, err := api.services.SubmitTemplate(c.Request.Context(), id, user, values)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, run)
}
func (api *API) exportTemplate(c *gin.Context) {
	user, ok := api.templateManager(c)
	if !ok {
		return
	}
	id, ok := templateID(c)
	if !ok {
		return
	}
	item, err := api.services.GetJobTemplate(c.Request.Context(), id, user)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "模板不存在"})
		return
	}
	c.Header("Content-Disposition", `attachment; filename="simplehpc-template-`+strconv.FormatInt(id, 10)+`.json"`)
	c.JSON(http.StatusOK, item)
}
func (api *API) importTemplate(c *gin.Context) {
	user, ok := api.templateManager(c)
	if !ok {
		return
	}
	var item service.JobTemplate
	if c.ShouldBindJSON(&item) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "导入文件不是有效的模板 JSON"})
		return
	}
	item.ID = 0
	item.Status = "draft"
	saved, err := api.services.SaveJobTemplate(c.Request.Context(), item, user.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, saved)
}

func (api *API) listTemplateRuns(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	runs, err := api.services.ListTemplateRuns(c.Request.Context(), user)
	if authz, exists := permissionContext(c); exists &&
		normalizeRBACMode(api.cfg.RBACMode) == rbacEnforce {
		runs, err = api.services.ListTemplateRunsByPermission(c.Request.Context(), authz)
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": runs, "count": len(runs)})
}
