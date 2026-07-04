package httpapi

import (
	"testing"

	"simplehpc/backend/internal/service"
)

func TestResolveTerminalLinuxUsernameMapsClusterAndConfigAdminsToRoot(t *testing.T) {
	cases := []struct {
		name     string
		user     service.AuthUser
		authz    service.PermissionContext
		expected string
	}{
		{
			name:     "cluster admin role",
			user:     service.AuthUser{Username: "testadmin", Type: "admin", Role: service.ClusterAdminRole},
			expected: "root",
		},
		{
			name:     "config admin role",
			user:     service.AuthUser{Username: "config01", Type: "admin", Role: "config_admin"},
			expected: "root",
		},
		{
			name:     "cluster admin permission context",
			user:     service.AuthUser{Username: "admin01", Type: "admin", Role: "custom_admin"},
			authz:    service.PermissionContext{IsClusterAdmin: true},
			expected: "root",
		},
		{
			name:     "config admin merged role",
			user:     service.AuthUser{Username: "multi01", Type: "admin", Role: "custom_admin"},
			authz:    service.PermissionContext{RoleCodes: []string{"config_admin"}},
			expected: "root",
		},
		{
			name:     "unit admin keeps own linux user",
			user:     service.AuthUser{Username: "unitadmin", Type: "admin", Role: "unit_admin"},
			authz:    service.PermissionContext{RoleCodes: []string{"unit_admin"}},
			expected: "unitadmin",
		},
		{
			name:     "ldap user keeps own linux user",
			user:     service.AuthUser{Username: "user001", Type: "ldap", Role: "user"},
			authz:    service.PermissionContext{RoleCodes: []string{"user"}},
			expected: "user001",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveTerminalLinuxUsername(tc.user, tc.authz); got != tc.expected {
				t.Fatalf("resolveTerminalLinuxUsername() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestSessionOwnerUsernameKeepsPlatformOwnerSeparateFromLinuxUser(t *testing.T) {
	session := &websshSession{OwnerUsername: "testadmin", Username: "root"}
	if got := sessionOwnerUsername(session); got != "testadmin" {
		t.Fatalf("sessionOwnerUsername() = %q, want platform owner", got)
	}

	legacySession := &websshSession{Username: "user001"}
	if got := sessionOwnerUsername(legacySession); got != "user001" {
		t.Fatalf("legacy sessionOwnerUsername() = %q, want username fallback", got)
	}
}
