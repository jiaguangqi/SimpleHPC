package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simplehpc/backend/internal/service"
)

func parseLicenseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "License 配置 ID 无效"})
		return 0, false
	}
	return id, true
}

func writeLicenseError(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "License 配置不存在"})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func (api *API) listLicenseConfigs(c *gin.Context) {
	items, err := api.services.ListLicenseConfigs(c.Request.Context(), c.Query("q"), c.Query("enabled"))
	if err != nil {
		writeLicenseError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items)})
}

func (api *API) getLicenseConfig(c *gin.Context) {
	id, ok := parseLicenseID(c)
	if !ok {
		return
	}
	item, err := api.services.GetLicenseConfig(c.Request.Context(), id)
	if err != nil {
		writeLicenseError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (api *API) createLicenseConfig(c *gin.Context) {
	var input service.LicenseConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	item, err := api.services.SaveLicenseConfig(c.Request.Context(), input)
	if err != nil {
		writeLicenseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (api *API) updateLicenseConfig(c *gin.Context) {
	id, ok := parseLicenseID(c)
	if !ok {
		return
	}
	var input service.LicenseConfig
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}
	input.ID = id
	item, err := api.services.SaveLicenseConfig(c.Request.Context(), input)
	if err != nil {
		writeLicenseError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (api *API) deleteLicenseConfig(c *gin.Context) {
	id, ok := parseLicenseID(c)
	if !ok {
		return
	}
	if err := api.services.DeleteLicenseConfig(c.Request.Context(), id); err != nil {
		writeLicenseError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (api *API) testLicenseConfig(c *gin.Context) {
	id, ok := parseLicenseID(c)
	if !ok {
		return
	}
	cfg, features, err := api.services.CollectLicenseConfig(c.Request.Context(), id)
	if err != nil {
		writeLicenseError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"config": cfg, "features": features, "count": len(features)})
}

func (api *API) collectLicenseConfig(c *gin.Context) {
	api.testLicenseConfig(c)
}

func (api *API) licenseServiceAction(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseLicenseID(c)
		if !ok {
			return
		}
		cfg, err := api.services.LicenseServiceAction(c.Request.Context(), id, action)
		if err != nil {
			writeLicenseError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"config": cfg, "action": action})
	}
}

func (api *API) licenseStatus(c *gin.Context) {
	resp, err := api.services.LicenseStatus(c.Request.Context())
	if err != nil {
		writeLicenseError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
