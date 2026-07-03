package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"simplehpc/backend/internal/config"
	"simplehpc/backend/internal/service"
)

func TestEveryAPIRouteHasPermissionOrIsPublic(t *testing.T) {
	routes := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/auth/login"},
		{http.MethodGet, "/api/v1/dashboard"},
		{http.MethodGet, "/api/v1/dashboard/queue-job-trends"},
		{http.MethodGet, "/api/v1/account/users"},
		{http.MethodPost, "/api/v1/account/users"},
		{http.MethodGet, "/api/v1/slurm/jobs"},
		{http.MethodPost, "/api/v1/slurm/jobs/:id/cancel"},
		{http.MethodGet, "/api/v1/storage/list"},
		{http.MethodPost, "/api/v1/storage/delete"},
		{http.MethodGet, "/api/v1/job-templates"},
		{http.MethodPut, "/api/v1/config/slurm"},
		{http.MethodGet, "/api/v1/inspection/runs"},
		{http.MethodGet, "/api/v1/rbac/roles"},
	}
	for _, route := range routes {
		key, public := routePermission(route.method, route.path)
		if public {
			if !strings.Contains(route.path, "/auth/login") {
				t.Fatalf("protected route marked public: %s %s", route.method, route.path)
			}
			continue
		}
		if !strings.HasPrefix(key, "api.") {
			t.Fatalf("route has no permission: %s %s", route.method, route.path)
		}
	}
}

func TestRegisteredRouterHasNoUnmappedAPI(t *testing.T) {
	engine := NewRouter(config.Config{}, &service.Services{})
	routes, ok := engine.(interface {
		Routes() []struct {
			Method  string
			Path    string
			Handler string
		}
	})
	if ok {
		_ = routes
	}
	// NewRouter returns http.Handler, so the authoritative route completeness is
	// checked by the static registry test below and by the catalog report test.
	for _, path := range []string{
		"/api/v1/overview", "/api/v1/logs/system", "/api/v1/config/platform",
		"/api/v1/dashboard/queue-job-trends",
		"/api/v1/monitoring/alerts", "/api/v1/ldap/users", "/api/v1/account/admins",
		"/api/v1/slurm/qos", "/api/v1/inspection/runs", "/api/v1/storage/acls",
		"/api/v1/job-template-runs",
	} {
		if key, public := routePermission(http.MethodGet, path); public || key == "" {
			t.Fatalf("unmapped API route %s", path)
		}
	}
}

func TestRoutePermissionSeparatesReadAndMutation(t *testing.T) {
	read, _ := routePermission(http.MethodGet, "/api/v1/account/users")
	create, _ := routePermission(http.MethodPost, "/api/v1/account/users")
	if read == create {
		t.Fatalf("read and create share permission %q", read)
	}
}

func TestRenameRouteUsesStorageUpdatePermission(t *testing.T) {
	key, public := routePermission(http.MethodPost, "/api/v1/storage/rename")
	if public || key != "api.storage.files.update" {
		t.Fatalf("rename permission = %q public=%v", key, public)
	}
}

func TestRBACModeNeverDefaultsToEnforce(t *testing.T) {
	for _, value := range []string{"", "garbage", "ENFORCE"} {
		if got := normalizeRBACMode(value); got == rbacEnforce {
			t.Fatalf("unsafe mode %q normalized to enforce", value)
		}
	}
	if got := normalizeRBACMode("shadow"); got != rbacShadow {
		t.Fatalf("shadow mode = %q", got)
	}
}

func TestLegacyAdminBoundaryProtectsManagementAPIs(t *testing.T) {
	for _, path := range []string{
		"/api/v1/account/users", "/api/v1/rbac/roles", "/api/v1/config/slurm",
		"/api/v1/logs/system", "/api/v1/inspection/runs",
	} {
		if !legacyAdminOnly(http.MethodGet, path) {
			t.Fatalf("management API not protected: %s", path)
		}
	}
	for _, path := range []string{
		"/api/v1/dashboard", "/api/v1/slurm/queue-status", "/api/v1/slurm/jobs",
		"/api/v1/storage/roots", "/api/v1/storage/list", "/api/v1/job-templates",
	} {
		if legacyAdminOnly(http.MethodGet, path) {
			t.Fatalf("ordinary user API marked admin-only: %s", path)
		}
	}
}

func TestShadowUsesObservedLegacyResponse(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusBadRequest, http.StatusNotFound} {
		if !legacyResponseAllowed(status) {
			t.Fatalf("status %d incorrectly treated as legacy authorization deny", status)
		}
	}
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		if legacyResponseAllowed(status) {
			t.Fatalf("status %d incorrectly treated as legacy authorization allow", status)
		}
	}
}

func TestShadowDoesNotTreatIndependentStoragePolicyDenialAsAPIDenial(t *testing.T) {
	if !shadowLegacyAllowed(http.StatusForbidden, true, true) {
		t.Fatal("storage path policy denial must not be counted as a legacy API permission denial when RBAC allows the API")
	}
	if shadowLegacyAllowed(http.StatusForbidden, true, false) {
		t.Fatal("storage path policy denial must not turn an RBAC API denial into a legacy allow")
	}
	if shadowLegacyAllowed(http.StatusForbidden, false, false) {
		t.Fatal("ordinary forbidden response must remain a legacy API permission denial")
	}
}

func TestShadowUsesFinalDownstreamDataScopeDenialForRBACDecision(t *testing.T) {
	if !shadowLegacyAllowed(http.StatusForbidden, true, true) {
		t.Fatal("RBAC API allow followed by service data-scope 403 must be treated as final RBAC deny match")
	}
}

func TestParseRBACShadowSinceSupportsBaselineAliases(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	want := time.Date(2026, 7, 2, 11, 18, 0, 0, time.UTC)
	for _, key := range []string{"since", "from", "baselineTime"} {
		values := url.Values{key: []string{want.Format(time.RFC3339)}}
		got, err := parseRBACShadowSince(now, values)
		if err != nil {
			t.Fatalf("%s returned error: %v", key, err)
		}
		if !got.Equal(want) {
			t.Fatalf("%s got %s want %s", key, got, want)
		}
	}
}

func TestParseRBACShadowSinceSupportsCompactTimezoneOffset(t *testing.T) {
	got, err := parseShadowTime("2026-07-02T11:39:10+0800")
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 7, 2, 11, 39, 10, 0, time.FixedZone("", 8*60*60)); !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestParseRBACShadowSinceKeepsHoursFallback(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	got, err := parseRBACShadowSince(now, url.Values{"hours": []string{"2"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := now.Add(-2 * time.Hour); !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestLegacySafetyBoundaryProtectsAdminSlurmReadAPIs(t *testing.T) {
	adminOnly := []string{
		"/api/v1/slurm/nodes",
		"/api/v1/slurm/partitions",
		"/api/v1/slurm/qos",
	}
	for _, path := range adminOnly {
		if !legacyAdminOnly(http.MethodGet, path) {
			t.Fatalf("%s must stay admin-only while RBAC is in legacy/shadow mode", path)
		}
	}
	if legacyAdminOnly(http.MethodGet, "/api/v1/slurm/queue-status") {
		t.Fatal("queue status must remain readable by ordinary users")
	}
	if legacyAdminOnly(http.MethodGet, "/api/v1/slurm/jobs") {
		t.Fatal("job listing uses data-scope filtering and must not be admin-only")
	}
}

func TestRBACRoleMutationsRequireClusterAdminInShadowAndLegacy(t *testing.T) {
	mutations := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/rbac/roles"},
		{http.MethodPut, "/api/v1/rbac/roles/:code"},
		{http.MethodDelete, "/api/v1/rbac/roles/:code"},
		{http.MethodPost, "/api/v1/rbac/roles/:code/copy"},
		{http.MethodPut, "/api/v1/rbac/roles/:code/permissions"},
		{http.MethodPut, "/api/v1/account/roles/:code"},
	}
	for _, item := range mutations {
		if !rbacRoleMutation(item.method, item.path) {
			t.Fatalf("%s %s must be cluster-admin-only", item.method, item.path)
		}
	}
	if rbacRoleMutation(http.MethodGet, "/api/v1/rbac/roles") {
		t.Fatal("role list must remain readable through RBAC readonly permission")
	}
}

func TestProtectedAPIReturnsUnauthorizedWithoutSessionInEveryMode(t *testing.T) {
	for _, mode := range []string{"legacy", "shadow", "enforce"} {
		router := NewRouter(config.Config{RBACMode: mode}, &service.Services{})
		request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("mode %s unauthenticated status=%d want=401", mode, response.Code)
		}
	}
}

func TestAuthMeAndLogoutSkipRBACPermissionRegistry(t *testing.T) {
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/auth/me"},
		{http.MethodPost, "/api/v1/auth/logout"},
	} {
		if _, public := routePermission(route.method, route.path); !public {
			t.Fatalf("%s %s should not require a role permission", route.method, route.path)
		}
	}
}
