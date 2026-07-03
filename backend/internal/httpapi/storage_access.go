package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	storageintegration "simplehpc/backend/internal/integrations/storage"
	"simplehpc/backend/internal/service"
)

func (api *API) scopedStorage(c *gin.Context) (*storageintegration.Client, []string, service.AuthUser, bool) {
	user, ok := api.currentUser(c)
	if !ok {
		return nil, nil, service.AuthUser{}, false
	}
	authz, exists := permissionContext(c)
	if !exists {
		var err error
		authz, err = api.services.ResolvePermissionContext(c.Request.Context(), user)
		if err != nil {
			if user.Type == "admin" && user.Role == service.ClusterAdminRole {
				authz = service.PermissionContext{Username: user.Username, AccountType: "admin", IsClusterAdmin: true}
			} else if user.Type != "admin" {
				authz = legacyUserFileContext(user, api.services.Storage.Roots)
			} else {
				c.JSON(http.StatusForbidden, gin.H{"error": "当前角色没有用户文件访问权限"})
				return nil, nil, user, false
			}
		}
	}
	access, err := api.services.ResolveStorageAuthorization(c.Request.Context(), user, authz)
	if err != nil {
		api.recordStorageDenied(c, user, "storage.access", err.Error())
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return nil, nil, user, false
	}
	roots := access.RootPaths()
	if len(roots) == 0 {
		api.recordStorageDenied(c, user, "storage.access", "当前角色没有文件目录访问策略")
		c.JSON(http.StatusForbidden, gin.H{"error": "当前角色没有文件目录访问权限"})
		return nil, nil, user, false
	}
	c.Set(storageAuthorizationKey, access)
	return storageintegration.New(roots), roots, user, true
}

const (
	storageAuthorizationKey   = "simplehpc.storage-authorization"
	downstreamPolicyDeniedKey = "simplehpc.downstream-policy-denied"
)

func legacyUserFileContext(user service.AuthUser, roots []string) service.PermissionContext {
	policies := make([]service.FilePolicyGrant, 0, len(roots))
	for _, root := range roots {
		policies = append(policies, service.FilePolicyGrant{
			StorageRoot: root, SubjectScope: "self", Access: service.AccessManage,
		})
	}
	return service.PermissionContext{
		Username: user.Username, AccountType: user.Type, RoleCodes: []string{"user"},
		FilePolicies: policies,
	}
}

func storageAuthorization(c *gin.Context) (service.StorageAuthorization, bool) {
	value, ok := c.Get(storageAuthorizationKey)
	if !ok {
		return service.StorageAuthorization{}, false
	}
	authz, ok := value.(service.StorageAuthorization)
	return authz, ok
}

func (api *API) requireStorageAccess(
	c *gin.Context,
	user service.AuthUser,
	operation string,
	required service.AccessLevel,
	paths ...string,
) bool {
	authz, ok := storageAuthorization(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "文件授权上下文不可用"})
		return false
	}
	for _, path := range paths {
		if !authz.Allows(path, required) {
			api.recordStorageDenied(c, user, operation, "路径超出当前角色文件授权范围")
			c.JSON(http.StatusForbidden, gin.H{"error": "路径超出当前角色文件授权范围"})
			return false
		}
	}
	return true
}

func storageErrorStatus(err error) int {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "outside configured storage roots") ||
		strings.Contains(message, "escapes configured storage root") ||
		strings.Contains(message, "configured storage root cannot") ||
		strings.Contains(message, "symbolic link") {
		return http.StatusForbidden
	}
	return http.StatusBadRequest
}

func (api *API) storageError(c *gin.Context, user service.AuthUser, operation string, err error) {
	status := storageErrorStatus(err)
	if status == http.StatusForbidden {
		api.recordStorageDenied(c, user, operation, err.Error())
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

func (api *API) recordStorageDenied(c *gin.Context, user service.AuthUser, operation, reason string) {
	c.Set(downstreamPolicyDeniedKey, true)
	if api.services.DB == nil {
		return
	}
	_ = api.services.RecordAudit(c.Request.Context(), service.AuditEntry{
		Actor: user.Username, ActorType: user.Type, Action: "storage.access.denied",
		TargetType: "storage_path", Target: operation, Result: "denied",
		Detail: map[string]any{
			"operation": operation, "reason": reason, "route": c.FullPath(),
		},
		IPAddress: c.ClientIP(),
	})
}
