package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"simplehpc/backend/internal/service"
)

const permissionContextKey = "simplehpc.permission-context"

func (api *API) rbacAccessControl() gin.HandlerFunc {
	return func(c *gin.Context) {
		mode := normalizeRBACMode(api.cfg.RBACMode)
		key, public := routePermission(c.Request.Method, c.FullPath())
		if public {
			c.Next()
			return
		}
		user, ok := api.sessionUserSilent(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "请先登录"})
			return
		}
		if mode == rbacLegacy {
			c.Next()
			return
		}
		authz, err := api.services.ResolvePermissionContext(c.Request.Context(), user)
		if err != nil {
			c.Next()
			api.recordRBACShadow(c, user, key, legacyResponseAllowed(c.Writer.Status()), false, "resolver_error")
			return
		}
		c.Set(permissionContextKey, authz)
		allowed := authz.Has(key)
		if mode == rbacEnforce && !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "forbidden", "message": "当前账号无权执行此操作",
			})
			return
		}
		c.Next()
		if mode == rbacShadow {
			_, downstreamPolicyDenied := c.Get(downstreamPolicyDeniedKey)
			legacyAllowed := shadowLegacyAllowed(c.Writer.Status(), downstreamPolicyDenied, allowed)
			reason := "legacy_rbac_match"
			if allowed != legacyAllowed {
				reason = "legacy_rbac_mismatch"
			} else if downstreamPolicyDenied {
				reason = "legacy_rbac_match_downstream_policy_denied"
			}
			api.recordRBACShadow(c, user, key, legacyAllowed, allowed, reason)
		}
	}
}

func legacyResponseAllowed(status int) bool {
	return status != http.StatusUnauthorized && status != http.StatusForbidden
}

func shadowLegacyAllowed(status int, downstreamPolicyDenied bool, rbacAllowed bool) bool {
	if downstreamPolicyDenied {
		return rbacAllowed
	}
	return legacyResponseAllowed(status)
}

func (api *API) legacySafetyBoundary() gin.HandlerFunc {
	return func(c *gin.Context) {
		if rbacRoleMutation(c.Request.Method, c.FullPath()) {
			user, ok := api.sessionUserSilent(c)
			if !ok {
				c.Next()
				return
			}
			if user.Type != "admin" || user.Role != service.ClusterAdminRole {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "角色变更仅允许集群管理员执行"})
				return
			}
			c.Next()
			return
		}
		if normalizeRBACMode(api.cfg.RBACMode) == rbacEnforce {
			c.Next()
			return
		}
		if !legacyAdminOnly(c.Request.Method, c.FullPath()) {
			c.Next()
			return
		}
		user, ok := api.sessionUserSilent(c)
		if !ok {
			c.Next()
			return
		}
		if user.Type != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			return
		}
		c.Next()
	}
}

func rbacRoleMutation(method, path string) bool {
	if method == http.MethodGet || method == http.MethodHead {
		return false
	}
	return strings.HasPrefix(path, "/api/v1/rbac/roles") ||
		strings.HasPrefix(path, "/api/v1/account/roles")
}

func legacyAdminOnly(method, path string) bool {
	switch {
	case strings.HasPrefix(path, "/api/v1/account/"),
		strings.HasPrefix(path, "/api/v1/rbac/"),
		strings.HasPrefix(path, "/api/v1/ldap/"),
		strings.HasPrefix(path, "/api/v1/config/") && path != "/api/v1/config/platform/public",
		strings.HasPrefix(path, "/api/v1/logs/"),
		strings.HasPrefix(path, "/api/v1/audit/"),
		strings.HasPrefix(path, "/api/v1/monitoring/"),
		strings.HasPrefix(path, "/api/v1/inspection/"),
		strings.HasPrefix(path, "/api/v1/storage/acls"):
		return true
	case strings.HasPrefix(path, "/api/v1/storage/roots") && method != http.MethodGet:
		return true
	case strings.HasPrefix(path, "/api/v1/slurm/partition-configs"),
		path == "/api/v1/slurm/partition-description",
		path == "/api/v1/slurm/nodes",
		path == "/api/v1/slurm/partitions",
		path == "/api/v1/slurm/qos":
		return true
	case strings.HasPrefix(path, "/api/v1/slurm/qos") && method != http.MethodGet:
		return true
	default:
		return false
	}
}

func (api *API) sessionUserSilent(c *gin.Context) (service.AuthUser, bool) {
	token := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	if token == "" {
		token, _ = c.Cookie("simplehpc_session")
	}
	if token == "" {
		return service.AuthUser{}, false
	}
	user, err := api.services.SessionUser(c.Request.Context(), token)
	return user, err == nil
}

func permissionContext(c *gin.Context) (service.PermissionContext, bool) {
	value, ok := c.Get(permissionContextKey)
	if !ok {
		return service.PermissionContext{}, false
	}
	authz, ok := value.(service.PermissionContext)
	return authz, ok
}

func (api *API) recordRBACShadow(c *gin.Context, user service.AuthUser, permission string, legacyAllowed, rbacAllowed bool, reason string) {
	matched := legacyAllowed == rbacAllowed && !strings.Contains(reason, "mismatch") &&
		!strings.HasPrefix(reason, "resolver_error")
	_ = api.services.RecordAudit(c.Request.Context(), service.AuditEntry{
		Actor: user.Username, ActorType: user.Type, Action: "rbac.shadow.compare",
		TargetType: "api_permission", Target: permission,
		Result: map[bool]string{true: "match", false: "mismatch"}[matched],
		Detail: map[string]any{
			"method": c.Request.Method, "route": c.FullPath(), "reason": reason,
			"status": c.Writer.Status(), "legacyAllowed": legacyAllowed,
			"rbacAllowed": rbacAllowed, "matched": matched,
		},
		IPAddress: c.ClientIP(),
	})
}

func (api *API) recordRBACScopeShadow(c *gin.Context, user service.AuthUser, permission string, legacyCount, rbacCount int) {
	matched := legacyCount == rbacCount
	reason := "legacy_rbac_scope_match"
	if !matched {
		reason = "legacy_rbac_scope_mismatch"
	}
	_ = api.services.RecordAudit(c.Request.Context(), service.AuditEntry{
		Actor: user.Username, ActorType: user.Type, Action: "rbac.shadow.compare",
		TargetType: "data_scope", Target: permission,
		Result: map[bool]string{true: "match", false: "mismatch"}[matched],
		Detail: map[string]any{
			"method": c.Request.Method, "route": c.FullPath(), "reason": reason,
			"status": c.Writer.Status(), "legacyAllowed": legacyCount > 0,
			"rbacAllowed": rbacCount > 0, "matched": matched,
			"legacyCount": legacyCount, "rbacCount": rbacCount,
		},
		IPAddress: c.ClientIP(),
	})
}
