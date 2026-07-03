package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"simplehpc/backend/internal/service"
)

func (api *API) getRBACShadowStats(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	since, err := parseRBACShadowSince(time.Now(), c.Request.URL.Query())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	summary, err := api.services.RBACShadowStats(c.Request.Context(), since)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func parseRBACShadowSince(now time.Time, values url.Values) (time.Time, error) {
	for _, key := range []string{"since", "from", "baselineTime"} {
		if raw := values.Get(key); raw != "" {
			parsed, err := parseShadowTime(raw)
			if err != nil {
				return time.Time{}, err
			}
			return parsed, nil
		}
	}
	hours, _ := strconv.Atoi(values.Get("hours"))
	if hours < 1 {
		hours = 1
	}
	if hours > 24*30 {
		hours = 24 * 30
	}
	return now.Add(-time.Duration(hours) * time.Hour), nil
}

func parseShadowTime(raw string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse("2006-01-02T15:04:05-0700", raw); err == nil {
		return parsed, nil
	}
	if unix, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(unix, 0).UTC(), nil
	}
	return time.Time{}, errors.New("无效 shadow 基线时间，需使用 RFC3339 或 Unix 秒")
}

func (api *API) listRBACRoles(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	items, err := api.services.ListRBACRoles(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items)})
}

func (api *API) getRBACRole(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	item, err := api.services.GetRoleConfiguration(c.Request.Context(), c.Param("code"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (api *API) saveRBACRole(c *gin.Context, current string) {
	user, ok := api.requireAdmin(c)
	if !ok {
		return
	}
	var input service.RBACRoleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效角色数据"})
		return
	}
	item, err := api.services.SaveRBACRole(c.Request.Context(), current, input, user.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (api *API) createRBACRole(c *gin.Context) { api.saveRBACRole(c, "") }
func (api *API) updateRBACRole(c *gin.Context) { api.saveRBACRole(c, c.Param("code")) }

func (api *API) copyRBACRole(c *gin.Context) {
	user, ok := api.requireAdmin(c)
	if !ok {
		return
	}
	var input service.RBACRoleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效角色数据"})
		return
	}
	item, err := api.services.CopyRBACRole(c.Request.Context(), c.Param("code"), input, user.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (api *API) setRBACRoleStatus(c *gin.Context) {
	user, ok := api.requireAdmin(c)
	if !ok {
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效状态"})
		return
	}
	if err := api.services.SetRBACRoleStatus(c.Request.Context(), c.Param("code"), input.Status, user.Username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) deleteRBACRole(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	if err := api.services.DeleteRBACRole(c.Request.Context(), c.Param("code")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) listRBACPermissions(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	items, err := api.services.ListPermissions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items)})
}

func (api *API) listRBACMenus(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	items := service.DefaultMenuCatalog()
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items)})
}

func (api *API) getRBACMatrix(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	roles, err := api.services.ListRBACRoles(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	items := make([]service.RoleConfiguration, 0, len(roles))
	for _, role := range roles {
		item, err := api.services.GetRoleConfiguration(c.Request.Context(), role.Code)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"roles": items, "menus": service.DefaultMenuCatalog()})
}

func (api *API) replaceRBACPermissions(c *gin.Context) {
	user, ok := api.requireAdmin(c)
	if !ok {
		return
	}
	var input struct {
		Permissions []string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效权限数据"})
		return
	}
	if err := api.services.ReplaceRolePermissions(c.Request.Context(), c.Param("code"), input.Permissions, user.Username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) replaceRBACDataScopes(c *gin.Context) {
	user, ok := api.requireAdmin(c)
	if !ok {
		return
	}
	var input struct {
		Items []service.DataScopeGrant `json:"items"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效数据范围"})
		return
	}
	if err := api.services.ReplaceRoleDataScopes(c.Request.Context(), c.Param("code"), input.Items, user.Username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) replaceRBACFilePolicies(c *gin.Context) {
	user, ok := api.requireAdmin(c)
	if !ok {
		return
	}
	var input struct {
		Items []service.FilePolicyGrant `json:"items"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效文件策略"})
		return
	}
	if err := api.services.ReplaceRoleFilePolicies(c.Request.Context(), c.Param("code"), input.Items, user.Username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (api *API) replaceRBACBindings(c *gin.Context) {
	user, ok := api.requireAdmin(c)
	if !ok {
		return
	}
	var input struct {
		Items []service.RoleBindingInput `json:"items"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效用户绑定"})
		return
	}
	if err := api.services.ReplaceRoleBindings(c.Request.Context(), c.Param("code"), input.Items, user.Username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
