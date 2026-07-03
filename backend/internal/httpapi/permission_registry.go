package httpapi

import (
	"net/http"
	"strings"
)

type rbacMode string

const (
	rbacLegacy  rbacMode = "legacy"
	rbacShadow  rbacMode = "shadow"
	rbacEnforce rbacMode = "enforce"
)

func normalizeRBACMode(value string) rbacMode {
	switch strings.TrimSpace(value) {
	case string(rbacShadow):
		return rbacShadow
	case string(rbacEnforce):
		return rbacEnforce
	default:
		return rbacLegacy
	}
}

func routePermission(method, path string) (string, bool) {
	if path == "/api/health" ||
		path == "/api/v1/auth/login" ||
		path == "/api/v1/auth/me" ||
		path == "/api/v1/auth/logout" ||
		path == "/api/v1/auth/password-reset/request" ||
		path == "/api/v1/auth/password-reset/confirm" ||
		path == "/api/v1/config/platform/public" ||
		strings.HasPrefix(path, "/api/v1/job-template-gateway/") ||
		strings.HasSuffix(path, "/register") {
		return "", true
	}
	action := apiAction(method, path)
	resource := apiResource(path)
	return "api." + resource + "." + action, false
}

func apiAction(method, path string) string {
	if strings.HasSuffix(path, "/cancel") {
		return "cancel"
	}
	if strings.HasSuffix(path, "/suspend") {
		return "suspend"
	}
	if strings.HasSuffix(path, "/resume") {
		return "resume"
	}
	if strings.HasSuffix(path, "/publish") || strings.HasSuffix(path, "/unpublish") {
		return "publish"
	}
	if strings.HasSuffix(path, "/approve") || strings.HasSuffix(path, "/reject") {
		return "review"
	}
	if strings.HasSuffix(path, "/test") {
		return "test"
	}
	if strings.HasSuffix(path, "/refresh") {
		return "refresh"
	}
	if strings.HasSuffix(path, "/rename") {
		return "update"
	}
	switch method {
	case http.MethodGet, http.MethodHead:
		if strings.Contains(path, "/:") {
			return "view"
		}
		return "list"
	case http.MethodPost:
		return "create"
	case http.MethodPut, http.MethodPatch:
		return "update"
	case http.MethodDelete:
		return "delete"
	default:
		return "access"
	}
}

func apiResource(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/v1/")
	switch {
	case strings.HasPrefix(trimmed, "auth/"):
		return "auth"
	case trimmed == "overview" || trimmed == "dashboard" || strings.HasPrefix(trimmed, "dashboard/"):
		return "dashboard"
	case strings.HasPrefix(trimmed, "account/roles"), strings.HasPrefix(trimmed, "rbac/"):
		return "roles"
	case strings.HasPrefix(trimmed, "account/users"), strings.HasPrefix(trimmed, "ldap/"):
		return "users"
	case strings.HasPrefix(trimmed, "account/admins"):
		return "admins"
	case strings.HasPrefix(trimmed, "account/teams"):
		return "teams"
	case strings.HasPrefix(trimmed, "account/units"):
		return "units"
	case strings.HasPrefix(trimmed, "account/"):
		return "accounts"
	case strings.HasPrefix(trimmed, "slurm/jobs"):
		return "jobs"
	case strings.HasPrefix(trimmed, "slurm/queue"):
		return "queue"
	case strings.HasPrefix(trimmed, "slurm/nodes"):
		return "nodes"
	case strings.HasPrefix(trimmed, "slurm/qos"):
		return "qos"
	case strings.HasPrefix(trimmed, "slurm/partition"):
		return "partitions"
	case strings.HasPrefix(trimmed, "storage/list"), strings.HasPrefix(trimmed, "storage/directory"),
		strings.HasPrefix(trimmed, "storage/upload"), strings.HasPrefix(trimmed, "storage/copy"),
		strings.HasPrefix(trimmed, "storage/move"), strings.HasPrefix(trimmed, "storage/delete"),
		strings.HasPrefix(trimmed, "storage/download"), strings.HasPrefix(trimmed, "storage/archive"),
		strings.HasPrefix(trimmed, "storage/rename"):
		return "storage.files"
	case strings.HasPrefix(trimmed, "storage/acls"):
		return "storage.acls"
	case strings.HasPrefix(trimmed, "storage/roots"):
		return "storage.roots"
	case strings.HasPrefix(trimmed, "job-template"):
		return "templates"
	case strings.HasPrefix(trimmed, "inspection"):
		return "inspection"
	case strings.HasPrefix(trimmed, "monitoring"):
		return "monitoring"
	case strings.HasPrefix(trimmed, "logs/"), strings.HasPrefix(trimmed, "audit/"):
		return "logs"
	case strings.HasPrefix(trimmed, "config/"):
		return "config." + strings.ReplaceAll(strings.TrimPrefix(trimmed, "config/"), "/", ".")
	default:
		segment := strings.Split(trimmed, "/")[0]
		return strings.ReplaceAll(segment, "-", ".")
	}
}
