package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"simplehpc/backend/internal/service"
)

func (api *API) listAuditLogs(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	query := service.AuditQuery{
		Page: queryInt(c, "page", 1), PageSize: queryInt(c, "pageSize", 20),
		Actor: c.Query("actor"), Action: c.Query("action"), Result: c.Query("result"),
	}
	items, total, err := api.services.AuditLogs(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": query.Page, "pageSize": query.PageSize})
}

func (api *API) getPlatformConfig(c *gin.Context) {
	if _, ok := api.currentUser(c); !ok {
		return
	}
	value, _, err := api.services.GetSystemConfig(c.Request.Context(), "platform")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if value["name"] == nil {
		value["name"] = "simpleHPC"
	}
	if value["language"] == nil {
		value["language"] = "zh-CN"
	}
	c.JSON(http.StatusOK, gin.H{"config": value})
}

func (api *API) getPublicPlatformConfig(c *gin.Context) {
	value, _, err := api.services.GetSystemConfig(c.Request.Context(), "platform")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	name, _ := value["name"].(string)
	if name == "" {
		name = "simpleHPC"
	}
	logo, _ := value["logo"].(string)
	loginImage, _ := value["loginImage"].(string)
	c.JSON(http.StatusOK, gin.H{"config": gin.H{"name": name, "logo": logo, "loginImage": loginImage, "language": "zh-CN"}})
}

func (api *API) savePlatformConfig(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	var value service.PlatformConfig
	if err := c.ShouldBindJSON(&value); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效平台配置"})
		return
	}
	value, err := service.NormalizePlatformConfig(value)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	payload := map[string]any{"name": value.Name, "logo": value.Logo, "loginImage": value.LoginImage, "language": value.Language}
	if err := api.services.SetSystemConfig(c.Request.Context(), "platform", payload); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	_ = api.services.RecordAudit(c.Request.Context(), service.AuditEntry{
		Actor: user.Username, ActorType: user.Type, Action: "config.platform.update",
		TargetType: "config", Target: "platform", Result: "success", IPAddress: c.ClientIP(),
	})
	c.JSON(http.StatusOK, gin.H{"config": payload})
}
