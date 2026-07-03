package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simplehpc/backend/internal/service"
)

func (api *API) listStorageACLs(c *gin.Context) {
	if _, ok := api.currentUser(c); !ok {
		return
	}
	items, err := api.services.ACLs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items), "source": "postgres-posix-acl"})
}

func (api *API) createStorageACL(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	var request service.ACLRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 ACL 参数"})
		return
	}
	item, err := api.services.CreateACL(c.Request.Context(), request, user.Username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (api *API) deleteStorageACL(c *gin.Context) {
	user, ok := api.currentUser(c)
	if !ok {
		return
	}
	if user.Type != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 ACL 编号"})
		return
	}
	if err := api.services.DeleteACL(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
