package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const permissionCacheTTL = 60 * time.Second

type permissionVersionPart struct {
	RoleCode         string
	RoleVersion      int64
	BindingUpdatedAt time.Time
}

func permissionVersion(parts []permissionVersionPart) string {
	sorted := append([]permissionVersionPart(nil), parts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].RoleCode < sorted[j].RoleCode })
	hash := sha256.New()
	for _, part := range sorted {
		fmt.Fprintf(hash, "%s:%d:%d\n", part.RoleCode, part.RoleVersion, part.BindingUpdatedAt.UnixNano())
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func permissionCacheKey(accountType, username string) string {
	return "authz:" + strings.TrimSpace(accountType) + ":" + strings.TrimSpace(username)
}

func (s *Services) InvalidateUserPermissions(ctx context.Context, accountType, username string) error {
	if s.Redis == nil {
		return nil
	}
	return s.Redis.Del(ctx, permissionCacheKey(accountType, username)).Err()
}

func (s *Services) ResolvePermissionContext(ctx context.Context, user AuthUser) (PermissionContext, error) {
	cacheKey := permissionCacheKey(user.Type, user.Username)
	if s.Redis != nil {
		if raw, err := s.Redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var cached PermissionContext
			if json.Unmarshal(raw, &cached) == nil && cached.Username == user.Username {
				return cached, nil
			}
		}
	}
	if s.DB == nil {
		return PermissionContext{}, errNotConfigured("postgres")
	}

	rows, err := s.DB.QueryContext(ctx, `
SELECT r.id,r.code,r.status,r.version,ur.updated_at,ur.scope_type,ur.scope_id
FROM user_roles_v2 ur
JOIN roles r ON r.id=ur.role_id
WHERE ur.account_type=$1 AND ur.username=$2 AND ur.status='active'
  AND r.status='active'
  AND (ur.valid_from IS NULL OR ur.valid_from<=now())
  AND (ur.valid_until IS NULL OR ur.valid_until>now())
ORDER BY r.code`, normalizeRBACAccountType(user.Type), user.Username)
	if err != nil {
		return PermissionContext{}, err
	}
	type roleRow struct {
		id        int64
		grant     RoleGrant
		version   permissionVersionPart
		scopeType string
		scopeID   string
	}
	var roles []roleRow
	for rows.Next() {
		var item roleRow
		if err := rows.Scan(&item.id, &item.grant.Code, &item.grant.Status,
			&item.version.RoleVersion, &item.version.BindingUpdatedAt,
			&item.scopeType, &item.scopeID); err != nil {
			rows.Close()
			return PermissionContext{}, err
		}
		item.version.RoleCode = item.grant.Code
		roles = append(roles, item)
	}
	if err := rows.Close(); err != nil {
		return PermissionContext{}, err
	}

	for index := range roles {
		if err := s.loadRoleGrant(ctx, roles[index].id, &roles[index].grant); err != nil {
			return PermissionContext{}, err
		}
	}
	grants := make([]RoleGrant, 0, len(roles))
	versions := make([]permissionVersionPart, 0, len(roles))
	for _, item := range roles {
		grants = append(grants, item.grant)
		versions = append(versions, item.version)
	}
	resolved := MergeRoleGrants(user.Username, normalizeRBACAccountType(user.Type), grants)
	for _, item := range roles {
		switch item.scopeType {
		case string(ScopeUnit):
			resolved.UnitIDs = appendUniqueString(resolved.UnitIDs, item.scopeID)
		case string(ScopeTeam):
			resolved.TeamIDs = appendUniqueString(resolved.TeamIDs, item.scopeID)
		}
	}
	resolved.Version = permissionVersion(versions)

	if s.Redis != nil {
		if raw, err := json.Marshal(resolved); err == nil {
			_ = s.Redis.Set(ctx, cacheKey, raw, permissionCacheTTL).Err()
		}
	}
	return resolved, nil
}

func appendUniqueString(values []string, value string) []string {
	if value == "" || value == "*" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func (s *Services) loadRoleGrant(ctx context.Context, roleID int64, grant *RoleGrant) error {
	permissionRows, err := s.DB.QueryContext(ctx, `
SELECT p.permission_key FROM role_permissions rp
JOIN permissions p ON p.id=rp.permission_id
WHERE rp.role_id=$1 AND p.status='active'`, roleID)
	if err != nil {
		return err
	}
	for permissionRows.Next() {
		var key string
		if err := permissionRows.Scan(&key); err != nil {
			permissionRows.Close()
			return err
		}
		grant.Permissions = append(grant.Permissions, key)
	}
	if err := permissionRows.Close(); err != nil {
		return err
	}

	scopeRows, err := s.DB.QueryContext(ctx, `
SELECT resource_code,scope_type,access_level
FROM role_data_scopes WHERE role_id=$1`, roleID)
	if err != nil {
		return err
	}
	for scopeRows.Next() {
		var item DataScopeGrant
		if err := scopeRows.Scan(&item.Resource, &item.Scope, &item.Access); err != nil {
			scopeRows.Close()
			return err
		}
		grant.DataScopes = append(grant.DataScopes, item)
	}
	if err := scopeRows.Close(); err != nil {
		return err
	}

	fileRows, err := s.DB.QueryContext(ctx, `
SELECT storage_root,subject_scope,access_level,allow_hidden
FROM role_file_policies WHERE role_id=$1`, roleID)
	if err != nil {
		return err
	}
	for fileRows.Next() {
		var item FilePolicyGrant
		if err := fileRows.Scan(&item.StorageRoot, &item.SubjectScope, &item.Access, &item.AllowHidden); err != nil {
			fileRows.Close()
			return err
		}
		grant.FilePolicies = append(grant.FilePolicies, item)
	}
	return fileRows.Close()
}

func normalizeRBACAccountType(value string) string {
	if value == "admin" {
		return "admin"
	}
	return "ldap"
}
