package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"simplehpc/backend/internal/service"
)

func (api *API) listAuthEvents(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	items, total, err := api.services.AuthEvents(c.Request.Context(), service.AuthEventQuery{
		Page: queryInt(c, "page", 1), PageSize: queryInt(c, "pageSize", 50),
		Username: c.Query("username"), Event: c.Query("event"), Result: c.Query("result"), Key: c.Query("keyword"),
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "source": "postgres-auth-events"})
}

func (api *API) listSystemLogs(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	ctx, cancel := contextWithTimeout(c, 12*time.Second)
	defer cancel()
	items, err := api.services.SystemLogs(ctx, c.DefaultQuery("source", "simplehpc-backend"), c.DefaultQuery("since", "1h"), queryInt(c, "limit", 300), c.Query("keyword"), c.Query("level"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "count": len(items), "source": "journald-or-docker"})
}

func contextWithTimeout(c *gin.Context, duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), duration)
}

func (api *API) auditWriteRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/v1/auth/") {
			c.Next()
			return
		}
		action := service.AuditActionForRequest(c.Request.Method, path)
		if action == "" {
			c.Next()
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		if token == "" {
			token, _ = c.Cookie("simplehpc_session")
		}
		user, authErr := api.services.SessionUser(c.Request.Context(), token)
		c.Next()
		if authErr != nil || user.Username == "" {
			return
		}
		result := "success"
		if c.Writer.Status() >= 400 {
			result = "failed"
		}
		route := c.FullPath()
		if route == "" {
			route = path
		}
		_ = api.services.RecordAudit(c.Request.Context(), service.AuditEntry{
			Actor: user.Username, ActorType: user.Type, Action: service.AuditActionForRequest(c.Request.Method, route),
			TargetType: "http_route", Target: route, Result: result, IPAddress: c.ClientIP(),
			Detail: map[string]any{"method": c.Request.Method, "path": path, "status": c.Writer.Status()},
		})
	}
}
