package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (api *API) listMonitoringAlerts(c *gin.Context) {
	if _, ok := api.currentUser(c); !ok {
		return
	}
	items, err := api.services.MonitoringAlerts(c.Request.Context(), c.Query("status"), queryInt(c, "limit", 100))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items), "source": "postgres-dashboard-alerts"})
}

func (api *API) acknowledgeMonitoringAlert(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效告警编号"})
		return
	}
	item, err := api.services.AcknowledgeMonitoringAlert(c.Request.Context(), id, user.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (api *API) refreshMonitoringAlerts(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	if err := api.services.RefreshMonitoringAlerts(c.Request.Context()); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	items, err := api.services.MonitoringAlerts(c.Request.Context(), "active", 100)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items), "source": "slurm-live-postgres-alerts"})
}
