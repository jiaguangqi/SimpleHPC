package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type RBACRole struct {
	Code                string `json:"code"`
	Name                string `json:"name"`
	Description         string `json:"description"`
	ScopeType           string `json:"scopeType"`
	Status              string `json:"status"`
	IsBuiltin           bool   `json:"isBuiltin"`
	AllowDelete         bool   `json:"allowDelete"`
	AllowPermissionEdit bool   `json:"allowPermissionEdit"`
	PermissionSummary   string `json:"permissionSummary"`
	Version             int64  `json:"version"`
	UserCount           int    `json:"userCount"`
	UpdatedAt           string `json:"updatedAt"`
}

type RBACRoleInput struct {
	Code                string `json:"code"`
	Name                string `json:"name"`
	Description         string `json:"description"`
	ScopeType           string `json:"scopeType"`
	PermissionSummary   string `json:"permissionSummary"`
	AllowPermissionEdit bool   `json:"allowPermissionEdit"`
}

type RoleBindingInput struct {
	AccountType string `json:"accountType"`
	Username    string `json:"username"`
	ScopeType   string `json:"scopeType"`
	ScopeID     string `json:"scopeId"`
}

type RoleConfiguration struct {
	Role         RBACRole           `json:"role"`
	Permissions  []string           `json:"permissions"`
	DataScopes   []DataScopeGrant   `json:"dataScopes"`
	FilePolicies []FilePolicyGrant  `json:"filePolicies"`
	Bindings     []RoleBindingInput `json:"bindings"`
}

func (s *Services) ListRBACRoles(ctx context.Context) ([]RBACRole, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT r.code,r.name,r.description,r.scope_type,r.status,r.is_builtin,
r.allow_delete,r.allow_permission_edit,r.permission_summary,r.version,
COUNT(ur.id)::int,r.updated_at
FROM roles r LEFT JOIN user_roles_v2 ur ON ur.role_id=r.id AND ur.status='active'
GROUP BY r.id ORDER BY r.is_builtin DESC,r.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []RBACRole
	for rows.Next() {
		var item RBACRole
		var updated time.Time
		if err := rows.Scan(&item.Code, &item.Name, &item.Description, &item.ScopeType,
			&item.Status, &item.IsBuiltin, &item.AllowDelete, &item.AllowPermissionEdit,
			&item.PermissionSummary, &item.Version, &item.UserCount, &updated); err != nil {
			return nil, err
		}
		item.UpdatedAt = updated.Format(time.RFC3339)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Services) SaveRBACRole(ctx context.Context, current string, input RBACRoleInput, actor string) (RBACRole, error) {
	input.Code, input.Name = strings.TrimSpace(input.Code), strings.TrimSpace(input.Name)
	if input.Code == "" || input.Name == "" {
		return RBACRole{}, fmt.Errorf("角色编码和名称不能为空")
	}
	if input.ScopeType != "global" && input.ScopeType != "unit" &&
		input.ScopeType != "team" && input.ScopeType != "self" {
		return RBACRole{}, fmt.Errorf("无效角色作用域")
	}
	if current == "" {
		_, err := s.DB.ExecContext(ctx, `
INSERT INTO roles(code,name,description,scope_type,permission_summary,status,is_builtin,
allow_delete,allow_permission_edit,created_by,updated_by)
VALUES($1,$2,$3,$4,$5,'active',FALSE,TRUE,$6,$7,$7)`,
			input.Code, input.Name, input.Description, input.ScopeType,
			input.PermissionSummary, input.AllowPermissionEdit, actor)
		if err != nil {
			return RBACRole{}, err
		}
	} else {
		if input.Code != current {
			return RBACRole{}, fmt.Errorf("角色编码创建后不允许修改")
		}
		var protected bool
		if err := s.DB.QueryRowContext(ctx, `
SELECT NOT allow_permission_edit FROM roles WHERE code=$1`, current).Scan(&protected); err != nil {
			return RBACRole{}, err
		}
		if protected {
			return RBACRole{}, fmt.Errorf("角色 %s 不允许修改", current)
		}
		_, err := s.DB.ExecContext(ctx, `
UPDATE roles SET name=$2,description=$3,scope_type=$4,permission_summary=$5,
allow_permission_edit=$6,version=version+1,updated_by=$7,updated_at=now()
WHERE code=$1`, current, input.Name, input.Description, input.ScopeType,
			input.PermissionSummary, input.AllowPermissionEdit, actor)
		if err != nil {
			return RBACRole{}, err
		}
		_ = s.invalidateRoleUsers(ctx, current)
	}
	return s.GetRBACRole(ctx, input.Code)
}

func (s *Services) GetRBACRole(ctx context.Context, code string) (RBACRole, error) {
	var item RBACRole
	var updated time.Time
	err := s.DB.QueryRowContext(ctx, `
SELECT r.code,r.name,r.description,r.scope_type,r.status,r.is_builtin,
r.allow_delete,r.allow_permission_edit,r.permission_summary,r.version,
(SELECT count(*) FROM user_roles_v2 ur WHERE ur.role_id=r.id AND ur.status='active')::int,
r.updated_at FROM roles r WHERE r.code=$1`, code).Scan(
		&item.Code, &item.Name, &item.Description, &item.ScopeType, &item.Status,
		&item.IsBuiltin, &item.AllowDelete, &item.AllowPermissionEdit,
		&item.PermissionSummary, &item.Version, &item.UserCount, &updated)
	item.UpdatedAt = updated.Format(time.RFC3339)
	return item, err
}

func (s *Services) GetRoleConfiguration(ctx context.Context, code string) (RoleConfiguration, error) {
	role, err := s.GetRBACRole(ctx, code)
	if err != nil {
		return RoleConfiguration{}, err
	}
	result := RoleConfiguration{Role: role}
	rows, err := s.DB.QueryContext(ctx, `
SELECT p.permission_key FROM role_permissions rp JOIN roles r ON r.id=rp.role_id
JOIN permissions p ON p.id=rp.permission_id WHERE r.code=$1 ORDER BY p.permission_key`, code)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return result, err
		}
		result.Permissions = append(result.Permissions, key)
	}
	rows.Close()
	scopeRows, err := s.DB.QueryContext(ctx, `
SELECT ds.resource_code,ds.scope_type,ds.access_level FROM role_data_scopes ds
JOIN roles r ON r.id=ds.role_id WHERE r.code=$1 ORDER BY ds.resource_code,ds.scope_type`, code)
	if err != nil {
		return result, err
	}
	for scopeRows.Next() {
		var item DataScopeGrant
		if err := scopeRows.Scan(&item.Resource, &item.Scope, &item.Access); err != nil {
			scopeRows.Close()
			return result, err
		}
		result.DataScopes = append(result.DataScopes, item)
	}
	scopeRows.Close()
	fileRows, err := s.DB.QueryContext(ctx, `
SELECT fp.storage_root,fp.subject_scope,fp.access_level,fp.allow_hidden
FROM role_file_policies fp JOIN roles r ON r.id=fp.role_id
WHERE r.code=$1 ORDER BY fp.storage_root,fp.subject_scope`, code)
	if err != nil {
		return result, err
	}
	for fileRows.Next() {
		var item FilePolicyGrant
		if err := fileRows.Scan(&item.StorageRoot, &item.SubjectScope, &item.Access, &item.AllowHidden); err != nil {
			fileRows.Close()
			return result, err
		}
		result.FilePolicies = append(result.FilePolicies, item)
	}
	fileRows.Close()
	bindingRows, err := s.DB.QueryContext(ctx, `
SELECT ur.account_type,ur.username,ur.scope_type,ur.scope_id FROM user_roles_v2 ur
JOIN roles r ON r.id=ur.role_id WHERE r.code=$1 AND ur.status='active'
ORDER BY ur.account_type,ur.username`, code)
	if err != nil {
		return result, err
	}
	defer bindingRows.Close()
	for bindingRows.Next() {
		var item RoleBindingInput
		if err := bindingRows.Scan(&item.AccountType, &item.Username, &item.ScopeType, &item.ScopeID); err != nil {
			return result, err
		}
		result.Bindings = append(result.Bindings, item)
	}
	return result, bindingRows.Err()
}

func (s *Services) CopyRBACRole(ctx context.Context, source string, input RBACRoleInput, actor string) (RBACRole, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return RBACRole{}, err
	}
	defer tx.Rollback()
	var sourceID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE code=$1`, source).Scan(&sourceID); err != nil {
		return RBACRole{}, err
	}
	var targetID int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO roles(code,name,description,scope_type,permission_summary,status,is_builtin,
allow_delete,allow_permission_edit,created_by,updated_by)
VALUES($1,$2,$3,$4,$5,'active',FALSE,TRUE,TRUE,$6,$6) RETURNING id`,
		input.Code, input.Name, input.Description, input.ScopeType,
		input.PermissionSummary, actor).Scan(&targetID)
	if err != nil {
		return RBACRole{}, err
	}
	for _, statement := range []string{
		`INSERT INTO role_permissions(role_id,permission_id,created_by)
		 SELECT $1,permission_id,$3 FROM role_permissions WHERE role_id=$2`,
		`INSERT INTO role_data_scopes(role_id,resource_code,scope_type,access_level)
		 SELECT $1,resource_code,scope_type,access_level FROM role_data_scopes WHERE role_id=$2`,
		`INSERT INTO role_file_policies(role_id,storage_root,subject_scope,access_level,allow_hidden,created_by)
		 SELECT $1,storage_root,subject_scope,access_level,allow_hidden,$3 FROM role_file_policies WHERE role_id=$2`,
	} {
		if _, err := tx.ExecContext(ctx, statement, targetID, sourceID, actor); err != nil {
			return RBACRole{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return RBACRole{}, err
	}
	return s.GetRBACRole(ctx, input.Code)
}

func (s *Services) SetRBACRoleStatus(ctx context.Context, code, status, actor string) error {
	if status != "active" && status != "disabled" {
		return fmt.Errorf("无效角色状态")
	}
	var role RoleDefinition
	if err := s.DB.QueryRowContext(ctx, `
SELECT code,is_builtin,allow_delete,allow_permission_edit FROM roles WHERE code=$1`,
		code).Scan(&role.Code, &role.IsBuiltin, &role.AllowDelete, &role.AllowPermissionEdit); err != nil {
		return err
	}
	if status == "disabled" {
		if err := ValidateRoleMutation(role, MutationDisable); err != nil {
			return err
		}
	}
	_, err := s.DB.ExecContext(ctx, `
UPDATE roles SET status=$2,version=version+1,updated_by=$3,updated_at=now() WHERE code=$1`,
		code, status, actor)
	if err == nil {
		err = s.invalidateRoleUsers(ctx, code)
	}
	return err
}

func (s *Services) DeleteRBACRole(ctx context.Context, code string) error {
	var role RoleDefinition
	var users int
	if err := s.DB.QueryRowContext(ctx, `
SELECT r.code,r.is_builtin,r.allow_delete,r.allow_permission_edit,
(SELECT count(*) FROM user_roles_v2 ur WHERE ur.role_id=r.id)
FROM roles r WHERE r.code=$1`, code).Scan(
		&role.Code, &role.IsBuiltin, &role.AllowDelete, &role.AllowPermissionEdit, &users); err != nil {
		return err
	}
	if err := ValidateRoleMutation(role, MutationDelete); err != nil {
		return err
	}
	if users > 0 {
		return fmt.Errorf("角色仍绑定 %d 个用户，请先解绑", users)
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM roles WHERE code=$1`, code)
	return err
}

type PermissionRecord struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	Module      string `json:"module"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Services) ListPermissions(ctx context.Context) ([]PermissionRecord, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT permission_key,permission_type,module_code,resource_code,action_code,name,description
FROM permissions WHERE status='active' ORDER BY permission_type,module_code,sort_order,permission_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PermissionRecord
	for rows.Next() {
		var item PermissionRecord
		if err := rows.Scan(&item.Key, &item.Type, &item.Module, &item.Resource,
			&item.Action, &item.Name, &item.Description); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Services) ReplaceRolePermissions(ctx context.Context, code string, keys []string, actor string) error {
	var role RoleDefinition
	if err := s.DB.QueryRowContext(ctx, `
SELECT code,is_builtin,allow_delete,allow_permission_edit FROM roles WHERE code=$1`,
		code).Scan(&role.Code, &role.IsBuiltin, &role.AllowDelete, &role.AllowPermissionEdit); err != nil {
		return err
	}
	if err := ValidateRoleMutation(role, MutationReplacePermissions); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var roleID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE code=$1 FOR UPDATE`, code).Scan(&roleID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, roleID); err != nil {
		return err
	}
	for _, key := range keys {
		result, err := tx.ExecContext(ctx, `
INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT $1,id,$3 FROM permissions WHERE permission_key=$2 AND status='active'`,
			roleID, key, actor)
		if err != nil {
			return err
		}
		if count, _ := result.RowsAffected(); count != 1 {
			return fmt.Errorf("未知权限键 %s", key)
		}
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE roles SET version=version+1,updated_by=$2,updated_at=now() WHERE id=$1`, roleID, actor); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.invalidateRoleUsers(ctx, code)
}

func (s *Services) ReplaceRoleDataScopes(ctx context.Context, code string, scopes []DataScopeGrant, actor string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var roleID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE code=$1 FOR UPDATE`, code).Scan(&roleID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM role_data_scopes WHERE role_id=$1`, roleID); err != nil {
		return err
	}
	for _, item := range scopes {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO role_data_scopes(role_id,resource_code,scope_type,access_level)
VALUES($1,$2,$3,$4)`, roleID, item.Resource, item.Scope, item.Access); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE roles SET version=version+1,updated_by=$2,updated_at=now() WHERE id=$1`, roleID, actor); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.invalidateRoleUsers(ctx, code)
}

func (s *Services) ReplaceRoleFilePolicies(ctx context.Context, code string, policies []FilePolicyGrant, actor string) error {
	if code == ClusterAdminRole {
		return fmt.Errorf("cluster_admin 文件兜底策略不允许清空或修改")
	}
	allowedRoots := map[string]bool{}
	for _, root := range s.Config.StorageRoots {
		allowedRoots[root] = true
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var roleID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE code=$1 FOR UPDATE`, code).Scan(&roleID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM role_file_policies WHERE role_id=$1`, roleID); err != nil {
		return err
	}
	for _, item := range policies {
		if !allowedRoots[item.StorageRoot] {
			return fmt.Errorf("文件策略目录不在授权存储根中")
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO role_file_policies(role_id,storage_root,subject_scope,access_level,allow_hidden,created_by)
VALUES($1,$2,$3,$4,$5,$6)`, roleID, item.StorageRoot, item.SubjectScope,
			item.Access, item.AllowHidden, actor); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE roles SET version=version+1,updated_by=$2,updated_at=now() WHERE id=$1`, roleID, actor); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.invalidateRoleUsers(ctx, code)
}

func (s *Services) ReplaceRoleBindings(ctx context.Context, code string, bindings []RoleBindingInput, actor string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var roleID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE code=$1 FOR UPDATE`, code).Scan(&roleID); err != nil {
		return err
	}
	if code == ClusterAdminRole && len(bindings) == 0 {
		return ValidateClusterAdminBindingRemoval(1)
	}
	type accountKey struct{ accountType, username string }
	affected := map[accountKey]struct{}{}
	oldRows, err := tx.QueryContext(ctx, `
SELECT account_type,username FROM user_roles_v2 WHERE role_id=$1`, roleID)
	if err != nil {
		return err
	}
	for oldRows.Next() {
		var key accountKey
		if err := oldRows.Scan(&key.accountType, &key.username); err != nil {
			oldRows.Close()
			return err
		}
		affected[key] = struct{}{}
	}
	if err := oldRows.Close(); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_roles_v2 WHERE role_id=$1`, roleID); err != nil {
		return err
	}
	for _, binding := range bindings {
		affected[accountKey{binding.AccountType, binding.Username}] = struct{}{}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO user_roles_v2(account_type,username,role_id,scope_type,scope_id,status,created_by)
VALUES($1,$2,$3,$4,$5,'active',$6)`,
			binding.AccountType, binding.Username, roleID, binding.ScopeType, binding.ScopeID, actor); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE roles SET version=version+1,updated_by=$2,updated_at=now() WHERE id=$1`, roleID, actor); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for key := range affected {
		_ = s.InvalidateUserPermissions(ctx, key.accountType, key.username)
	}
	return nil
}

func (s *Services) invalidateRoleUsers(ctx context.Context, code string) error {
	if s.Redis == nil {
		return nil
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT ur.account_type,ur.username FROM user_roles_v2 ur
JOIN roles r ON r.id=ur.role_id WHERE r.code=$1`, code)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var accountType, username string
		if err := rows.Scan(&accountType, &username); err != nil {
			return err
		}
		if err := s.InvalidateUserPermissions(ctx, accountType, username); err != nil {
			return err
		}
	}
	return rows.Err()
}

func isNoRows(err error) bool { return err == sql.ErrNoRows }
