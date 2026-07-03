package service

import (
	"fmt"
	"sort"
)

const ClusterAdminRole = "cluster_admin"

type RoleStatus string
type DataScope string
type AccessLevel string
type RoleMutation string

const (
	RoleActive   RoleStatus = "active"
	RoleDisabled RoleStatus = "disabled"

	ScopeNone    DataScope = "none"
	ScopeGranted DataScope = "granted"
	ScopeSelf    DataScope = "self"
	ScopeTeam    DataScope = "team"
	ScopeUnit    DataScope = "unit"
	ScopeGlobal  DataScope = "global"

	AccessNone   AccessLevel = "none"
	AccessView   AccessLevel = "view"
	AccessManage AccessLevel = "manage"

	MutationDisable            RoleMutation = "disable"
	MutationDelete             RoleMutation = "delete"
	MutationReplacePermissions RoleMutation = "replace_permissions"
)

type DataScopeGrant struct {
	Resource string      `json:"resource"`
	Scope    DataScope   `json:"scope"`
	Access   AccessLevel `json:"access"`
}

type FilePolicyGrant struct {
	StorageRoot  string      `json:"storageRoot"`
	SubjectScope string      `json:"subjectScope"`
	Access       AccessLevel `json:"access"`
	AllowHidden  bool        `json:"allowHidden"`
}

type RoleGrant struct {
	Code         string
	Status       RoleStatus
	Permissions  []string
	DataScopes   []DataScopeGrant
	FilePolicies []FilePolicyGrant
}

type PermissionContext struct {
	Username       string
	AccountType    string
	RoleCodes      []string
	Permissions    map[string]struct{}
	DataScopes     map[string]map[DataScope]struct{}
	AccessLevels   map[string]AccessLevel
	FilePolicies   []FilePolicyGrant
	UnitIDs        []string
	TeamIDs        []string
	IsClusterAdmin bool
	Version        string
}

func (p PermissionContext) Has(key string) bool {
	if p.IsClusterAdmin {
		return true
	}
	_, ok := p.Permissions[key]
	return ok
}

func (p PermissionContext) HasScope(resource string, scope DataScope) bool {
	if p.IsClusterAdmin {
		return true
	}
	_, ok := p.DataScopes[resource][scope]
	return ok
}

func (p PermissionContext) AccessLevel(resource string) AccessLevel {
	if p.IsClusterAdmin {
		return AccessManage
	}
	return p.AccessLevels[resource]
}

func (p PermissionContext) HighestScope(resource string) DataScope {
	if p.IsClusterAdmin {
		return ScopeGlobal
	}
	scopes := p.DataScopes[resource]
	for _, scope := range []DataScope{ScopeGlobal, ScopeUnit, ScopeTeam, ScopeSelf, ScopeGranted, ScopeNone} {
		if _, ok := scopes[scope]; ok {
			return scope
		}
	}
	return ScopeNone
}

func MergeRoleGrants(username, accountType string, grants []RoleGrant) PermissionContext {
	result := PermissionContext{
		Username: username, AccountType: accountType,
		Permissions:  map[string]struct{}{},
		DataScopes:   map[string]map[DataScope]struct{}{},
		AccessLevels: map[string]AccessLevel{},
	}
	filePolicies := map[string]FilePolicyGrant{}
	for _, grant := range grants {
		if grant.Status != RoleActive {
			continue
		}
		result.RoleCodes = append(result.RoleCodes, grant.Code)
		if grant.Code == ClusterAdminRole {
			result.IsClusterAdmin = true
			result.Permissions["*"] = struct{}{}
		}
		for _, key := range grant.Permissions {
			if key != "" {
				result.Permissions[key] = struct{}{}
			}
		}
		for _, scope := range grant.DataScopes {
			if result.DataScopes[scope.Resource] == nil {
				result.DataScopes[scope.Resource] = map[DataScope]struct{}{}
			}
			result.DataScopes[scope.Resource][scope.Scope] = struct{}{}
			if accessRank(scope.Access) > accessRank(result.AccessLevels[scope.Resource]) {
				result.AccessLevels[scope.Resource] = scope.Access
			}
		}
		for _, policy := range grant.FilePolicies {
			key := policy.StorageRoot + "\x00" + policy.SubjectScope
			current, ok := filePolicies[key]
			if !ok || accessRank(policy.Access) > accessRank(current.Access) {
				filePolicies[key] = policy
			} else if policy.AllowHidden {
				current.AllowHidden = true
				filePolicies[key] = current
			}
		}
	}
	sort.Strings(result.RoleCodes)
	keys := make([]string, 0, len(filePolicies))
	for key := range filePolicies {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result.FilePolicies = append(result.FilePolicies, filePolicies[key])
	}
	return result
}

func accessRank(level AccessLevel) int {
	switch level {
	case AccessManage:
		return 2
	case AccessView:
		return 1
	default:
		return 0
	}
}

type RoleDefinition struct {
	Code                string
	IsBuiltin           bool
	AllowDelete         bool
	AllowPermissionEdit bool
}

func ValidateRoleMutation(role RoleDefinition, mutation RoleMutation) error {
	switch mutation {
	case MutationDisable:
		if role.Code == ClusterAdminRole {
			return fmt.Errorf("内置角色 cluster_admin 不允许禁用")
		}
	case MutationDelete:
		if role.IsBuiltin || !role.AllowDelete {
			return fmt.Errorf("内置角色不允许删除")
		}
	case MutationReplacePermissions:
		if !role.AllowPermissionEdit || role.Code == ClusterAdminRole {
			return fmt.Errorf("角色 %s 的权限不允许修改", role.Code)
		}
	}
	return nil
}

func ValidateClusterAdminBindingRemoval(activeBindings int) error {
	if activeBindings <= 1 {
		return fmt.Errorf("不能移除最后一个有效 cluster_admin 绑定")
	}
	return nil
}
