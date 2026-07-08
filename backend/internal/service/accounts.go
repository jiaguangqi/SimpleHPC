package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	ldapintegration "simplehpc/backend/internal/integrations/ldap"
)

type AccountSyncResult struct {
	Users  int    `json:"users"`
	Groups int    `json:"groups"`
	Units  int    `json:"units"`
	Source string `json:"source"`
}

type AccountUser struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Unit        string `json:"unit"`
	Team        string `json:"team"`
	LeaderName  string `json:"leaderName"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	SyncStatus  string `json:"syncStatus"`
	UIDNumber   string `json:"uidNumber,omitempty"`
	GIDNumber   string `json:"gidNumber,omitempty"`
	HomeDir     string `json:"homeDirectory,omitempty"`
	LDAPDN      string `json:"ldapDn,omitempty"`
	SyncedAt    string `json:"syncedAt,omitempty"`
}

type AccountTeam struct {
	Name           string `json:"name"`
	GroupName      string `json:"groupName"`
	Unit           string `json:"unit"`
	LeaderUsername string `json:"leaderUsername"`
	LeaderName     string `json:"leaderName"`
	Members        int    `json:"members"`
	MemberLimit    int    `json:"memberLimit"`
	ResourcePolicy string `json:"resourcePolicy"`
	Status         string `json:"status"`
	LDAPDN         string `json:"ldapDn,omitempty"`
	SyncedAt       string `json:"syncedAt,omitempty"`
}

type AccountTeamMember struct {
	UIDNumber   string `json:"uidNumber"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	HomeDir     string `json:"homeDirectory,omitempty"`
	Leader      bool   `json:"leader"`
}

type AccountOperationResult struct {
	Username      string `json:"username"`
	Status        string `json:"status"`
	Email         string `json:"email,omitempty"`
	Password      string `json:"password,omitempty"`
	LDAPUpdated   bool   `json:"ldapUpdated"`
	PGUpdated     bool   `json:"pgUpdated"`
	LoginDisabled bool   `json:"loginDisabled,omitempty"`
}

type AccountUnit struct {
	Name    string `json:"name"`
	Code    string `json:"code"`
	Admin   string `json:"admin"`
	Teams   int    `json:"teams"`
	Members int    `json:"members"`
	Status  string `json:"status"`
	Source  string `json:"source"`
}

type AdminUser struct {
	Username  string `json:"username"`
	RoleName  string `json:"roleName"`
	Status    string `json:"status"`
	Email     string `json:"email"`
	CreatedBy string `json:"createdBy"`
	LastLogin string `json:"lastLogin"`
}

type AdminUpdate struct {
	Email    string `json:"email"`
	RoleName string `json:"roleName"`
	Status   string `json:"status"`
}

type AdminPasswordResetResult struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

type AdminDeleteResult struct {
	Username string `json:"username"`
	Deleted  bool   `json:"deleted"`
}

type RoleRecord struct {
	Code              string `json:"code"`
	Name              string `json:"name"`
	ScopeType         string `json:"scopeType"`
	PermissionSummary string `json:"permissionSummary"`
	UserCount         int    `json:"userCount"`
}

type UnitInput struct {
	Name   string `json:"name"`
	Code   string `json:"code"`
	Admin  string `json:"admin"`
	Status string `json:"status"`
}
type RoleInput struct {
	Code              string `json:"code"`
	Name              string `json:"name"`
	ScopeType         string `json:"scopeType"`
	PermissionSummary string `json:"permissionSummary"`
}
type AdminCreate struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	RoleName string `json:"roleName"`
	Password string `json:"password"`
}

type CreateUserInput struct {
	Username, DisplayName, Email, Team, Unit, HomeDirectory string
}
type UpdateUserInput struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}
type CreateTeamInput struct {
	Name           string `json:"name"`
	GroupName      string `json:"groupName"`
	Unit           string `json:"unit"`
	LeaderUsername string `json:"leaderUsername"`
	ResourcePolicy string `json:"resourcePolicy"`
}
type CreateTeamWithLeaderInput struct {
	Team          CreateTeamInput    `json:"team"`
	Leader        CreateUserInput    `json:"leader"`
	StorageGrants []TeamStorageGrant `json:"storageGrants"`
}
type TeamStorageGrant struct {
	Enabled    bool   `json:"enabled"`
	Path       string `json:"path"`
	Permission string `json:"permission"`
}
type CreatedAccount struct {
	User     AccountUser `json:"user"`
	Password string      `json:"password,omitempty"`
}
type CreatedTeamWithLeader struct {
	Team   AccountTeam    `json:"team"`
	Leader CreatedAccount `json:"leader"`
}

func validateCreateUser(input CreateUserInput) error {
	raw := input
	input = normalizeCreateUserInput(input)
	if err := validateLinuxAccountName(raw.Username, "账号"); err != nil {
		return err
	}
	if input.DisplayName == "" {
		return fmt.Errorf("姓名不能为空")
	}
	if err := validateEmailAddress(input.Email); err != nil {
		return err
	}
	if input.Unit == "" {
		return fmt.Errorf("单位不能为空")
	}
	if input.Team == "" {
		return fmt.Errorf("团队不能为空")
	}
	if err := validateHomeDirectoryForUser(raw.HomeDirectory, input.Username); err != nil {
		return err
	}
	return nil
}
func validateCreateTeam(input CreateTeamInput) error {
	raw := input
	input = normalizeCreateTeamInput(input)
	if input.Name == "" {
		return fmt.Errorf("团队名称不能为空")
	}
	if err := validateLinuxAccountName(raw.GroupName, "LDAP/Linux 组名"); err != nil {
		return err
	}
	if input.Unit == "" {
		return fmt.Errorf("单位不能为空")
	}
	if raw.LeaderUsername != "" {
		if err := validateLinuxAccountName(raw.LeaderUsername, "组长账号"); err != nil {
			return err
		}
	}
	return nil
}
func normalizeCreateTeamWithLeader(input CreateTeamWithLeaderInput) CreateTeamWithLeaderInput {
	input.Team = normalizeCreateTeamInput(input.Team)
	input.Leader = normalizeCreateUserInput(input.Leader)
	if input.Team.GroupName == "" {
		input.Team.GroupName = input.Team.Name
	}
	if input.Team.LeaderUsername == "" {
		input.Team.LeaderUsername = input.Leader.Username
	}
	if input.Leader.Team == "" {
		input.Leader.Team = input.Team.GroupName
	}
	if input.Leader.Unit == "" {
		input.Leader.Unit = input.Team.Unit
	}
	return input
}
func validateCreateTeamWithLeader(input CreateTeamWithLeaderInput) (CreateTeamWithLeaderInput, error) {
	raw := input
	input = normalizeCreateTeamWithLeader(input)
	teamForValidation := input.Team
	if raw.Team.GroupName != "" {
		teamForValidation.GroupName = raw.Team.GroupName
	}
	if raw.Team.LeaderUsername != "" {
		teamForValidation.LeaderUsername = raw.Team.LeaderUsername
	}
	if err := validateCreateTeam(teamForValidation); err != nil {
		return input, err
	}
	if input.Leader.Username == "" {
		return input, fmt.Errorf("组长账号不能为空")
	}
	if input.Team.LeaderUsername != input.Leader.Username {
		return input, fmt.Errorf("组长账号必须与首个用户账号一致")
	}
	if input.Leader.Team != input.Team.GroupName && input.Leader.Team != input.Team.Name {
		return input, fmt.Errorf("组长用户组必须绑定到新建用户组")
	}
	if input.Leader.Unit != input.Team.Unit {
		return input, fmt.Errorf("组长单位必须与用户组单位一致")
	}
	leaderForValidation := input.Leader
	leaderForValidation.Username = raw.Leader.Username
	leaderForValidation.HomeDirectory = raw.Leader.HomeDirectory
	if err := validateCreateUser(leaderForValidation); err != nil {
		return input, err
	}
	for i, grant := range input.StorageGrants {
		if !grant.Enabled {
			continue
		}
		if strings.TrimSpace(grant.Path) == "" {
			return input, fmt.Errorf("第 %d 条组存储目录授权路径不能为空", i+1)
		}
		permission := strings.TrimSpace(grant.Permission)
		if permission == "" {
			permission = "rwx"
		}
		if permission != "r" && permission != "rw" && permission != "rwx" {
			return input, fmt.Errorf("第 %d 条组存储目录授权权限只允许 r、rw 或 rwx", i+1)
		}
		input.StorageGrants[i].Permission = permission
		input.StorageGrants[i].Path = strings.TrimSpace(grant.Path)
	}
	return input, nil
}

func (s *Services) CreatePlatformUser(ctx context.Context, input CreateUserInput) (CreatedAccount, error) {
	if err := validateCreateUser(input); err != nil {
		return CreatedAccount{}, err
	}
	input = normalizeCreateUserInput(input)
	var teamID int64
	var gid int
	err := s.DB.QueryRowContext(ctx, `SELECT id FROM teams WHERE name=$1 OR group_name=$1 LIMIT 1`, input.Team).Scan(&teamID)
	if err != nil {
		return CreatedAccount{}, fmt.Errorf("团队 %s 不存在", input.Team)
	}
	groups, groupErr := s.LDAP.ListGroups()
	if groupErr != nil {
		return CreatedAccount{}, groupErr
	}
	for _, group := range groups {
		if group.CN == input.Team {
			gid, _ = strconv.Atoi(group.GIDNumber)
			break
		}
	}
	if gid == 0 {
		if err := s.DB.QueryRowContext(ctx, `SELECT COALESCE(MAX(gid_number),999)+1 FROM platform_users`).Scan(&gid); err != nil {
			return CreatedAccount{}, err
		}
	}
	var uid int
	if err := s.DB.QueryRowContext(ctx, `SELECT GREATEST(COALESCE(MAX(uid_number),999)+1,1000) FROM platform_users`).Scan(&uid); err != nil {
		return CreatedAccount{}, err
	}
	password, err := randomPassword(18)
	if err != nil {
		return CreatedAccount{}, err
	}
	home := strings.TrimSpace(input.HomeDirectory)
	if home == "" {
		home = "/data/home/" + input.Username
	}
	if err := s.LDAP.CreateUser(ldapintegration.CreateUserRequest{Username: input.Username, DisplayName: input.DisplayName, Email: input.Email, Password: password, UIDNumber: strconv.Itoa(uid), GIDNumber: strconv.Itoa(gid), HomeDirectory: home}); err != nil {
		return CreatedAccount{}, err
	}
	if err := s.LDAP.AddGroupMember(input.Team, input.Username); err != nil {
		_ = s.LDAP.DeleteUser(input.Username)
		return CreatedAccount{}, err
	}
	homeInit, err := ensureLinuxUserHome(ctx, input.Username, home, uid, gid)
	if err != nil {
		_ = s.LDAP.DeleteUser(input.Username)
		return CreatedAccount{}, err
	}
	var unitID any
	if input.Unit != "" {
		var id int64
		if s.DB.QueryRowContext(ctx, `SELECT id FROM units WHERE name=$1 OR code=$1 LIMIT 1`, input.Unit).Scan(&id) == nil {
			unitID = id
		}
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO platform_users(username,display_name,email,unit_id,team_id,uid_number,gid_number,home_directory,status,source,synced_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'active','ldap',now())`, input.Username, input.DisplayName, input.Email, unitID, teamID, uid, gid, home)
	if err != nil {
		_ = s.LDAP.DeleteUser(input.Username)
		cleanupCreatedHome(homeInit, home)
		return CreatedAccount{}, err
	}
	return CreatedAccount{User: AccountUser{Username: input.Username, DisplayName: input.DisplayName, Email: input.Email, Team: input.Team, Unit: input.Unit, Status: "active", UIDNumber: strconv.Itoa(uid), GIDNumber: strconv.Itoa(gid), HomeDir: home}, Password: password}, nil
}

func (s *Services) UpdatePlatformUser(ctx context.Context, username string, input UpdateUserInput) (AccountUser, error) {
	input = normalizeUpdateUserInput(input)
	if input.DisplayName == "" {
		return AccountUser{}, fmt.Errorf("姓名不能为空")
	}
	if err := validateEmailAddress(input.Email); err != nil {
		return AccountUser{}, err
	}
	if err := s.LDAP.UpdateUser(username, input.DisplayName, input.Email); err != nil {
		return AccountUser{}, err
	}
	var item AccountUser
	err := s.DB.QueryRowContext(ctx, `UPDATE platform_users SET display_name=$2,email=$3,updated_at=now() WHERE username=$1 RETURNING username,display_name,email,status,COALESCE(uid_number::text,''),COALESCE(gid_number::text,''),home_directory`, username, input.DisplayName, input.Email).Scan(&item.Username, &item.DisplayName, &item.Email, &item.Status, &item.UIDNumber, &item.GIDNumber, &item.HomeDir)
	return item, err
}

func (s *Services) CreatePlatformTeam(ctx context.Context, input CreateTeamInput) (AccountTeam, error) {
	if err := validateCreateTeam(input); err != nil {
		return AccountTeam{}, err
	}
	input = normalizeCreateTeamInput(input)
	var unitID int64
	if err := s.DB.QueryRowContext(ctx, `SELECT id FROM units WHERE name=$1 OR code=$1 LIMIT 1`, input.Unit).Scan(&unitID); err != nil {
		return AccountTeam{}, fmt.Errorf("单位 %s 不存在", input.Unit)
	}
	gid := 1000
	groups, err := s.LDAP.ListGroups()
	if err != nil {
		return AccountTeam{}, err
	}
	for _, group := range groups {
		if value, parseErr := strconv.Atoi(group.GIDNumber); parseErr == nil && value >= gid {
			gid = value + 1
		}
	}
	if err := s.LDAP.CreateGroup(ldapintegration.CreateGroupRequest{Name: input.GroupName, GIDNumber: strconv.Itoa(gid), Description: input.Name}); err != nil {
		return AccountTeam{}, err
	}
	if input.LeaderUsername != "" {
		if err := s.LDAP.AddGroupMember(input.GroupName, input.LeaderUsername); err != nil {
			_ = s.LDAP.DeleteGroup(input.GroupName)
			return AccountTeam{}, err
		}
	}
	var item AccountTeam
	err = s.DB.QueryRowContext(ctx, `INSERT INTO teams(unit_id,name,group_name,leader_username,resource_policy,status,source,synced_at) VALUES($1,$2,$3,$4,$5,'active','ldap',now()) RETURNING name,group_name,$6,leader_username,resource_policy,status`, unitID, input.Name, input.GroupName, input.LeaderUsername, input.ResourcePolicy, input.Unit).Scan(&item.Name, &item.GroupName, &item.Unit, &item.LeaderUsername, &item.ResourcePolicy, &item.Status)
	if err != nil {
		_ = s.LDAP.DeleteGroup(input.GroupName)
	}
	return item, err
}

func (s *Services) applyTeamStorageGrantsTx(ctx context.Context, tx *sql.Tx, groupName string, grants []TeamStorageGrant) ([]ACLRequest, error) {
	applied := []ACLRequest{}
	for _, grant := range grants {
		if !grant.Enabled {
			continue
		}
		req := ACLRequest{Path: grant.Path, SubjectType: "group", Subject: groupName, Permission: grant.Permission}
		if req.Permission == "" {
			req.Permission = "rwx"
		}
		if err := ValidateACLRequest(req, s.storageRootPaths()); err != nil {
			return applied, err
		}
		if output, err := exec.CommandContext(ctx, "setfacl", "-m", fmt.Sprintf("g:%s:%s", req.Subject, req.Permission), req.Path).CombinedOutput(); err != nil {
			return applied, fmt.Errorf("setfacl: %s", strings.TrimSpace(string(output)))
		}
		applied = append(applied, req)
		if _, err := tx.ExecContext(ctx, `
INSERT INTO storage_acls(subject_type,subject_name,path,permission,created_by)
VALUES('group',$1,$2,$3,'team-create-workflow')
ON CONFLICT(subject_type,subject_name,path)
DO UPDATE SET permission=EXCLUDED.permission,created_by=EXCLUDED.created_by,created_at=now()`,
			req.Subject, req.Path, req.Permission); err != nil {
			return applied, err
		}
	}
	return applied, nil
}

func cleanupAppliedACLs(ctx context.Context, requests []ACLRequest) {
	for _, req := range requests {
		_ = exec.CommandContext(ctx, "setfacl", "-x", fmt.Sprintf("g:%s", req.Subject), req.Path).Run()
	}
}

func (s *Services) CreatePlatformTeamWithLeader(ctx context.Context, input CreateTeamWithLeaderInput) (CreatedTeamWithLeader, error) {
	input, err := validateCreateTeamWithLeader(input)
	if err != nil {
		return CreatedTeamWithLeader{}, err
	}
	var unitID int64
	if err := s.DB.QueryRowContext(ctx, `SELECT id FROM units WHERE name=$1 OR code=$1 LIMIT 1`, input.Team.Unit).Scan(&unitID); err != nil {
		return CreatedTeamWithLeader{}, fmt.Errorf("单位 %s 不存在", input.Team.Unit)
	}
	var exists int
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM teams WHERE name=$1 OR group_name=$2`, input.Team.Name, input.Team.GroupName).Scan(&exists); err != nil {
		return CreatedTeamWithLeader{}, err
	}
	if exists > 0 {
		return CreatedTeamWithLeader{}, fmt.Errorf("团队名称或组名称已存在")
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM platform_users WHERE username=$1`, input.Leader.Username).Scan(&exists); err != nil {
		return CreatedTeamWithLeader{}, err
	}
	if exists > 0 {
		return CreatedTeamWithLeader{}, fmt.Errorf("组长账号已存在")
	}

	gid := 1000
	groups, err := s.LDAP.ListGroups()
	if err != nil {
		return CreatedTeamWithLeader{}, err
	}
	for _, group := range groups {
		if group.CN == input.Team.GroupName {
			return CreatedTeamWithLeader{}, fmt.Errorf("LDAP 组 %s 已存在", input.Team.GroupName)
		}
		if value, parseErr := strconv.Atoi(group.GIDNumber); parseErr == nil && value >= gid {
			gid = value + 1
		}
	}
	var uid int
	if err := s.DB.QueryRowContext(ctx, `SELECT GREATEST(COALESCE(MAX(uid_number),999)+1,1000) FROM platform_users`).Scan(&uid); err != nil {
		return CreatedTeamWithLeader{}, err
	}
	password, err := randomPassword(18)
	if err != nil {
		return CreatedTeamWithLeader{}, err
	}
	home := input.Leader.HomeDirectory
	if home == "" {
		home = "/data/home/" + input.Leader.Username
	}

	if err := s.LDAP.CreateGroup(ldapintegration.CreateGroupRequest{Name: input.Team.GroupName, GIDNumber: strconv.Itoa(gid), Description: input.Team.Name}); err != nil {
		return CreatedTeamWithLeader{}, err
	}
	cleanupLDAP := true
	defer func() {
		if cleanupLDAP {
			_ = s.LDAP.DeleteUser(input.Leader.Username)
			_ = s.LDAP.DeleteGroup(input.Team.GroupName)
		}
	}()
	if err := s.LDAP.CreateUser(ldapintegration.CreateUserRequest{Username: input.Leader.Username, DisplayName: input.Leader.DisplayName, Email: input.Leader.Email, Password: password, UIDNumber: strconv.Itoa(uid), GIDNumber: strconv.Itoa(gid), HomeDirectory: home}); err != nil {
		return CreatedTeamWithLeader{}, err
	}
	if err := s.LDAP.AddGroupMember(input.Team.GroupName, input.Leader.Username); err != nil {
		return CreatedTeamWithLeader{}, err
	}
	homeInit, err := ensureLinuxUserHome(ctx, input.Leader.Username, home, uid, gid)
	if err != nil {
		return CreatedTeamWithLeader{}, err
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		cleanupCreatedHome(homeInit, home)
		return CreatedTeamWithLeader{}, err
	}
	defer tx.Rollback()

	var teamID int64
	var team AccountTeam
	err = tx.QueryRowContext(ctx, `INSERT INTO teams(unit_id,name,group_name,leader_username,member_count,resource_policy,status,source,synced_at) VALUES($1,$2,$3,$4,1,$5,'active','ldap',now()) RETURNING id,name,group_name,$6,leader_username,member_count,resource_policy,status`,
		unitID, input.Team.Name, input.Team.GroupName, input.Leader.Username, input.Team.ResourcePolicy, input.Team.Unit).Scan(&teamID, &team.Name, &team.GroupName, &team.Unit, &team.LeaderUsername, &team.Members, &team.ResourcePolicy, &team.Status)
	if err != nil {
		cleanupCreatedHome(homeInit, home)
		return CreatedTeamWithLeader{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO platform_users(username,display_name,email,unit_id,team_id,uid_number,gid_number,home_directory,status,source,synced_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'active','ldap',now())`,
		input.Leader.Username, input.Leader.DisplayName, input.Leader.Email, unitID, teamID, uid, gid, home)
	if err != nil {
		cleanupCreatedHome(homeInit, home)
		return CreatedTeamWithLeader{}, err
	}
	result, err := tx.ExecContext(ctx, `
INSERT INTO user_roles_v2(account_type,username,role_id,scope_type,scope_id,status,created_by)
SELECT 'ldap',$1,r.id,'self',$1,'active','team-create-workflow'
FROM roles r WHERE r.code='user'
ON CONFLICT(account_type,username,role_id,scope_type,scope_id)
DO UPDATE SET status='active',updated_at=now()`, input.Leader.Username)
	if err != nil {
		cleanupCreatedHome(homeInit, home)
		return CreatedTeamWithLeader{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		cleanupCreatedHome(homeInit, home)
		return CreatedTeamWithLeader{}, fmt.Errorf("RBAC 内置角色 user 不存在")
	}
	if result, err = tx.ExecContext(ctx, `
INSERT INTO user_roles_v2(account_type,username,role_id,scope_type,scope_id,status,created_by)
SELECT 'ldap',$1,r.id,'team',$2,'active','team-create-workflow'
FROM roles r WHERE r.code='team_admin'
ON CONFLICT(account_type,username,role_id,scope_type,scope_id)
DO UPDATE SET status='active',updated_at=now()`, input.Leader.Username, strconv.FormatInt(teamID, 10)); err != nil {
		cleanupCreatedHome(homeInit, home)
		return CreatedTeamWithLeader{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		cleanupCreatedHome(homeInit, home)
		return CreatedTeamWithLeader{}, fmt.Errorf("RBAC 内置角色 team_admin 不存在")
	}
	appliedACLs, err := s.applyTeamStorageGrantsTx(ctx, tx, input.Team.GroupName, input.StorageGrants)
	if err != nil {
		cleanupAppliedACLs(ctx, appliedACLs)
		cleanupCreatedHome(homeInit, home)
		return CreatedTeamWithLeader{}, err
	}
	if err := tx.Commit(); err != nil {
		cleanupAppliedACLs(ctx, appliedACLs)
		cleanupCreatedHome(homeInit, home)
		return CreatedTeamWithLeader{}, err
	}
	cleanupLDAP = false
	return CreatedTeamWithLeader{
		Team: team,
		Leader: CreatedAccount{
			User:     AccountUser{Username: input.Leader.Username, DisplayName: input.Leader.DisplayName, Email: input.Leader.Email, Team: input.Team.GroupName, Unit: input.Team.Unit, Role: "组长", Status: "active", UIDNumber: strconv.Itoa(uid), GIDNumber: strconv.Itoa(gid), HomeDir: home},
			Password: password,
		},
	}, nil
}

func (s *Services) UpdatePlatformTeam(ctx context.Context, current string, input CreateTeamInput) (AccountTeam, error) {
	current = strings.TrimSpace(current)
	if strings.TrimSpace(input.Name) == "" {
		return AccountTeam{}, fmt.Errorf("团队名称不能为空")
	}
	if input.LeaderUsername != "" {
		if err := validateLinuxAccountName(input.LeaderUsername, "组长账号"); err != nil {
			return AccountTeam{}, err
		}
	}
	input = normalizeCreateTeamInput(input)
	var item AccountTeam
	err := s.DB.QueryRowContext(ctx, `UPDATE teams SET name=$2,leader_username=$3,resource_policy=$4,updated_at=now() WHERE name=$1 OR group_name=$1 RETURNING name,group_name,$5,leader_username,resource_policy,status`, current, input.Name, input.LeaderUsername, input.ResourcePolicy, input.Unit).Scan(&item.Name, &item.GroupName, &item.Unit, &item.LeaderUsername, &item.ResourcePolicy, &item.Status)
	return item, err
}

func (s *Services) FreezePlatformTeam(ctx context.Context, name string, frozen bool) error {
	members, err := s.AccountTeamMembers(ctx, name)
	if err != nil {
		return err
	}
	changed := []string{}
	for _, member := range members {
		if err := s.LDAP.SetUserDisabled(member.Username, frozen); err != nil {
			for _, username := range changed {
				_ = s.LDAP.SetUserDisabled(username, !frozen)
			}
			return err
		}
		changed = append(changed, member.Username)
	}
	status := "active"
	if frozen {
		status = "frozen"
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE platform_users SET status=$2,updated_at=now() WHERE team_id IN(SELECT id FROM teams WHERE name=$1 OR group_name=$1)`, name, status); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE teams SET status=$2,updated_at=now() WHERE name=$1 OR group_name=$1`, name, status); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Services) DeletePlatformTeam(ctx context.Context, name string) error {
	var groupName string
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT group_name,(SELECT count(*) FROM platform_users WHERE team_id=teams.id AND status<>'deleted') FROM teams WHERE name=$1 OR group_name=$1`, name).Scan(&groupName, &count)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("团队仍有 %d 个成员，不能删除", count)
	}
	if err := s.LDAP.DeleteGroup(groupName); err != nil {
		return err
	}
	if _, err = s.DB.ExecContext(ctx, `UPDATE platform_users SET team_id=NULL,updated_at=now() WHERE team_id IN(SELECT id FROM teams WHERE name=$1 OR group_name=$1) AND status='deleted'`, name); err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `DELETE FROM teams WHERE name=$1 OR group_name=$1`, name)
	return err
}

func (s *Services) SaveUnit(ctx context.Context, current string, input UnitInput) (AccountUnit, error) {
	current = strings.TrimSpace(current)
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Code) == "" {
		return AccountUnit{}, fmt.Errorf("单位名称和编码不能为空")
	}
	if err := validateUnitCode(input.Code); err != nil {
		return AccountUnit{}, err
	}
	if input.Admin != "" {
		if err := validateLinuxAccountName(input.Admin, "单位管理员账号"); err != nil {
			return AccountUnit{}, err
		}
	}
	input = normalizeUnitInput(input)
	if input.Status == "" {
		input.Status = "active"
	}
	var item AccountUnit
	if current == "" {
		err := s.DB.QueryRowContext(ctx, `INSERT INTO units(name,code,admin_username,status,source) VALUES($1,$2,$3,$4,'project') RETURNING name,code,admin_username,status,source`, input.Name, input.Code, input.Admin, input.Status).Scan(&item.Name, &item.Code, &item.Admin, &item.Status, &item.Source)
		return item, err
	}
	err := s.DB.QueryRowContext(ctx, `UPDATE units SET name=$2,code=$3,admin_username=$4,status=$5,updated_at=now() WHERE code=$1 RETURNING name,code,admin_username,status,source`, current, input.Name, input.Code, input.Admin, input.Status).Scan(&item.Name, &item.Code, &item.Admin, &item.Status, &item.Source)
	return item, err
}
func (s *Services) DeleteUnit(ctx context.Context, code string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM units WHERE code=$1 AND NOT EXISTS(SELECT 1 FROM teams WHERE unit_id=units.id) AND NOT EXISTS(SELECT 1 FROM platform_users WHERE unit_id=units.id)`, code)
	return err
}
func (s *Services) SaveRole(ctx context.Context, current string, input RoleInput) (RoleRecord, error) {
	if strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.Name) == "" {
		return RoleRecord{}, fmt.Errorf("角色编码和名称不能为空")
	}
	if input.ScopeType == "" {
		input.ScopeType = "global"
	}
	var item RoleRecord
	if current == "" {
		err := s.DB.QueryRowContext(ctx, `INSERT INTO roles(code,name,scope_type,permission_summary) VALUES($1,$2,$3,$4) RETURNING code,name,scope_type,permission_summary,0`, input.Code, input.Name, input.ScopeType, input.PermissionSummary).Scan(&item.Code, &item.Name, &item.ScopeType, &item.PermissionSummary, &item.UserCount)
		return item, err
	}
	err := s.DB.QueryRowContext(ctx, `UPDATE roles SET name=$2,scope_type=$3,permission_summary=$4 WHERE code=$1 RETURNING code,name,scope_type,permission_summary,0`, current, input.Name, input.ScopeType, input.PermissionSummary).Scan(&item.Code, &item.Name, &item.ScopeType, &item.PermissionSummary, &item.UserCount)
	return item, err
}
func (s *Services) CreateAdminUser(ctx context.Context, input AdminCreate, createdBy string) (AdminUser, error) {
	if err := validateLinuxAccountName(input.Username, "管理员账号"); err != nil {
		return AdminUser{}, err
	}
	if err := validateEmailAddress(input.Email); err != nil {
		return AdminUser{}, err
	}
	input = normalizeAdminCreateInput(input)
	if input.RoleName == "" {
		return AdminUser{}, fmt.Errorf("管理员角色不能为空")
	}
	if len(input.Password) < 8 {
		return AdminUser{}, fmt.Errorf("管理员密码至少 8 位")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return AdminUser{}, err
	}
	var item AdminUser
	err = s.DB.QueryRowContext(ctx, `INSERT INTO admin_users(username,email,role_name,password_hash,created_by,status) VALUES($1,$2,$3,$4,$5,'active') RETURNING username,role_name,status,email,created_by,last_login`, input.Username, input.Email, input.RoleName, string(hash), createdBy).Scan(&item.Username, &item.RoleName, &item.Status, &item.Email, &item.CreatedBy, &item.LastLogin)
	return item, err
}

func (s *Services) StartLDAPAccountSync(ctx context.Context) {
	if s.DB == nil || s.LDAP == nil {
		return
	}
	go func() {
		s.syncLDAPOnce(ctx)
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.syncLDAPOnce(ctx)
			}
		}
	}()
}

func (s *Services) syncLDAPOnce(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	if _, err := s.SyncLDAPAccounts(ctx); err != nil {
		log.Printf("sync ldap accounts: %v", err)
	}
}

func (s *Services) SyncLDAPAccounts(ctx context.Context) (AccountSyncResult, error) {
	if s.DB == nil {
		return AccountSyncResult{}, errNotConfigured("postgres")
	}
	users, err := s.LDAP.ListUsers()
	if err != nil {
		return AccountSyncResult{}, err
	}
	groups, err := s.LDAP.ListGroups()
	if err != nil {
		return AccountSyncResult{}, err
	}
	units, err := s.LDAP.ListOrganizationalUnits()
	if err != nil {
		return AccountSyncResult{}, err
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return AccountSyncResult{}, err
	}
	defer tx.Rollback()

	now := time.Now()
	unitCount := 0
	codeToUnitID := map[string]int64{}
	for _, unit := range units {
		code := strings.TrimSpace(unit.OU)
		if code == "" {
			continue
		}
		var id int64
		err := tx.QueryRowContext(ctx, `
SELECT id
FROM units
WHERE code = $1 OR name = $1
ORDER BY CASE WHEN source = 'project' THEN 0 ELSE 1 END, id
LIMIT 1
`, code).Scan(&id)
		if err == nil {
			if _, err := tx.ExecContext(ctx, `
UPDATE units
SET code = $2,
    source = CASE WHEN source IN ('project', 'project+ldap') THEN 'project+ldap' ELSE 'ldap' END,
    ldap_dn = $3,
    synced_at = $4,
    updated_at = now()
WHERE id = $1
`, id, code, unit.DN, now); err != nil {
				return AccountSyncResult{}, err
			}
		} else if err == sql.ErrNoRows {
			err = tx.QueryRowContext(ctx, `
INSERT INTO units (name, code, status, source, ldap_dn, synced_at, updated_at)
VALUES ($1, $1, 'active', 'ldap', $2, $3, now())
RETURNING id
`, code, unit.DN, now).Scan(&id)
			if err != nil {
				return AccountSyncResult{}, err
			}
		} else {
			return AccountSyncResult{}, err
		}
		codeToUnitID[code] = id
		unitCount++
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM units u
WHERE u.source = 'ldap'
  AND COALESCE(u.code, '') = ''
  AND NOT EXISTS (SELECT 1 FROM teams t WHERE t.unit_id = u.id)
  AND NOT EXISTS (SELECT 1 FROM platform_users p WHERE p.unit_id = u.id)
`); err != nil {
		return AccountSyncResult{}, err
	}

	gidToTeam := map[string]int64{}
	for _, group := range groups {
		name := strings.TrimSpace(group.CN)
		if name == "" {
			continue
		}
		var unitID any
		if id, ok := codeToUnitID[unitCodeFromGroup(name)]; ok {
			unitID = id
		}
		var id int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM teams WHERE group_name = $1 OR name = $1 ORDER BY id LIMIT 1`, name).Scan(&id)
		if err == nil {
			if _, err := tx.ExecContext(ctx, `
UPDATE teams
SET name = $1,
    group_name = $1,
    unit_id = COALESCE($2::bigint, unit_id),
    member_count = $3,
    source = 'ldap',
    ldap_dn = $4,
    synced_at = $5,
    updated_at = now()
WHERE id = $6
`, name, unitID, len(group.MemberUIDs), group.DN, now, id); err != nil {
				return AccountSyncResult{}, err
			}
		} else if err == sql.ErrNoRows {
			err = tx.QueryRowContext(ctx, `
INSERT INTO teams (unit_id, name, group_name, leader_username, member_count, resource_policy, status, source, ldap_dn, synced_at, updated_at)
VALUES ($1, $2, $2, '', $3, '', 'active', 'ldap', $4, $5, now())
RETURNING id
`, unitID, name, len(group.MemberUIDs), group.DN, now).Scan(&id)
			if err != nil {
				return AccountSyncResult{}, err
			}
		} else {
			return AccountSyncResult{}, err
		}
		if strings.TrimSpace(group.GIDNumber) != "" {
			gidToTeam[group.GIDNumber] = id
		}
	}

	for _, user := range users {
		username := strings.TrimSpace(user.UID)
		if username == "" {
			continue
		}
		displayName := firstNonEmpty(user.DisplayName, user.CN, username)
		var teamID any
		if id, ok := gidToTeam[user.GIDNumber]; ok {
			teamID = id
		}
		status := ldapUserStatus(user.LoginShell)
		if _, err := tx.ExecContext(ctx, `
INSERT INTO platform_users (username, display_name, email, team_id, ldap_dn, uid_number, gid_number, home_directory, status, source, synced_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ldap',$10,now())
ON CONFLICT (username) DO UPDATE SET
  display_name = EXCLUDED.display_name,
  email = EXCLUDED.email,
  team_id = EXCLUDED.team_id,
  ldap_dn = EXCLUDED.ldap_dn,
  uid_number = EXCLUDED.uid_number,
  gid_number = EXCLUDED.gid_number,
  home_directory = EXCLUDED.home_directory,
  status = CASE
    WHEN platform_users.status = 'deleted' THEN platform_users.status
    WHEN platform_users.status IN ('frozen','disabled','locked') THEN platform_users.status
    ELSE EXCLUDED.status
  END,
  source = 'ldap',
  synced_at = EXCLUDED.synced_at,
  updated_at = now()
`, username, displayName, user.Mail, teamID, user.DN, atoiNull(user.UIDNumber), atoiNull(user.GIDNumber), user.HomeDir, status, now); err != nil {
			return AccountSyncResult{}, err
		}
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE teams t
SET member_count = counts.member_count,
    updated_at = now()
FROM (
  SELECT t2.id, COUNT(pu.id)::int AS member_count
  FROM teams t2
  LEFT JOIN platform_users pu ON pu.team_id = t2.id AND pu.status <> 'deleted'
  GROUP BY t2.id
) counts
WHERE counts.id = t.id
`); err != nil {
		return AccountSyncResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE teams t
SET leader_username = first_user.username,
    updated_at = now()
FROM (
  SELECT DISTINCT ON (team_id) team_id, username
  FROM platform_users
  WHERE team_id IS NOT NULL AND status <> 'deleted'
  ORDER BY team_id, username
) first_user
WHERE first_user.team_id = t.id
  AND COALESCE(t.leader_username, '') = ''
`); err != nil {
		return AccountSyncResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return AccountSyncResult{}, err
	}
	return AccountSyncResult{Users: len(users), Groups: len(groups), Units: unitCount, Source: "ldap-to-postgres"}, nil
}

func (s *Services) AccountUsers(ctx context.Context) ([]AccountUser, error) {
	if s.DB == nil {
		return nil, errNotConfigured("postgres")
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT u.username, u.display_name, u.email, COALESCE(un.name, tun.name, ''), COALESCE(t.name,''), COALESCE(leader.display_name, t.leader_username, ''), u.status,
		       COALESCE(u.uid_number::text,''), COALESCE(u.gid_number::text,''), COALESCE(u.home_directory,''), COALESCE(u.ldap_dn,''),
       COALESCE(u.synced_at::text,'')
FROM platform_users u
LEFT JOIN units un ON un.id = u.unit_id
LEFT JOIN teams t ON t.id = u.team_id
LEFT JOIN units tun ON tun.id = t.unit_id
LEFT JOIN platform_users leader ON leader.username = t.leader_username
ORDER BY u.username
LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AccountUser{}
	for rows.Next() {
		var item AccountUser
		if err := rows.Scan(&item.Username, &item.DisplayName, &item.Email, &item.Unit, &item.Team, &item.LeaderName, &item.Status, &item.UIDNumber, &item.GIDNumber, &item.HomeDir, &item.LDAPDN, &item.SyncedAt); err != nil {
			return nil, err
		}
		item.Role = "普通用户"
		if item.Status == "" {
			item.Status = "active"
		}
		item.SyncStatus = "LDAP/项目库一致"
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) CurrentUserProfile(ctx context.Context, user AuthUser) (AuthProfile, error) {
	if s.DB == nil {
		return AuthProfile{}, errNotConfigured("postgres")
	}
	profile := AuthProfile{
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		AccountType: user.Type,
		Role:        user.Role,
	}
	if strings.EqualFold(user.Type, "admin") {
		err := s.DB.QueryRowContext(ctx, `
SELECT username, role_name, status, email, created_by,
       COALESCE(last_login, ''),
       COALESCE(created_at::text, ''),
       COALESCE(updated_at::text, '')
FROM admin_users
WHERE username = $1
`, user.Username).Scan(
			&profile.Username,
			&profile.Role,
			&profile.Status,
			&profile.Email,
			&profile.CreatedBy,
			&profile.LastLogin,
			&profile.CreatedAt,
			&profile.UpdatedAt,
		)
		if err != nil {
			return profile, err
		}
		if profile.DisplayName == "" {
			profile.DisplayName = profile.Username
		}
		profile.AccountType = "admin"
		return profile, nil
	}

	err := s.DB.QueryRowContext(ctx, `
SELECT u.username,
       COALESCE(u.display_name, ''),
       COALESCE(u.email, ''),
       COALESCE(u.phone, ''),
       COALESCE(un.name, tun.name, ''),
       COALESCE(t.name, ''),
       COALESCE(leader.display_name, t.leader_username, ''),
       COALESCE(u.status, ''),
       COALESCE(u.uid_number::text, ''),
       COALESCE(u.gid_number::text, ''),
       COALESCE(u.home_directory, ''),
       COALESCE(u.ldap_dn, ''),
       COALESCE(u.synced_at::text, ''),
       COALESCE(u.created_at::text, ''),
       COALESCE(u.updated_at::text, '')
FROM platform_users u
LEFT JOIN units un ON un.id = u.unit_id
LEFT JOIN teams t ON t.id = u.team_id
LEFT JOIN units tun ON tun.id = t.unit_id
LEFT JOIN platform_users leader ON leader.username = t.leader_username
WHERE u.username = $1 AND u.status <> 'deleted'
`, user.Username).Scan(
		&profile.Username,
		&profile.DisplayName,
		&profile.Email,
		&profile.Phone,
		&profile.Unit,
		&profile.Team,
		&profile.LeaderName,
		&profile.Status,
		&profile.UIDNumber,
		&profile.GIDNumber,
		&profile.HomeDir,
		&profile.LDAPDN,
		&profile.SyncedAt,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if err != nil {
		return profile, err
	}
	if profile.DisplayName == "" {
		profile.DisplayName = profile.Username
	}
	if strings.TrimSpace(profile.Role) == "" {
		profile.Role = user.Role
	}
	if strings.TrimSpace(profile.Role) == "" {
		profile.Role = "user"
	}
	if profile.AccountType == "" {
		profile.AccountType = user.Type
	}
	return profile, nil
}

func (s *Services) AccountTeams(ctx context.Context) ([]AccountTeam, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT t.name, t.group_name, COALESCE(u.name,''), t.leader_username, COALESCE(p.display_name,''), COALESCE(mc.members, t.member_count), t.resource_policy, t.status, t.ldap_dn, COALESCE(t.synced_at::text,'')
FROM teams t
LEFT JOIN units u ON u.id = t.unit_id
LEFT JOIN platform_users p ON p.username = t.leader_username
LEFT JOIN (
  SELECT team_id, COUNT(*)::int AS members
  FROM platform_users
  WHERE team_id IS NOT NULL AND status <> 'deleted'
  GROUP BY team_id
) mc ON mc.team_id = t.id
ORDER BY t.name
LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AccountTeam{}
	for rows.Next() {
		var item AccountTeam
		if err := rows.Scan(&item.Name, &item.GroupName, &item.Unit, &item.LeaderUsername, &item.LeaderName, &item.Members, &item.ResourcePolicy, &item.Status, &item.LDAPDN, &item.SyncedAt); err != nil {
			return nil, err
		}
		item.MemberLimit = 50
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) AccountTeamMembers(ctx context.Context, teamName string) ([]AccountTeamMember, error) {
	if s.DB == nil {
		return nil, errNotConfigured("postgres")
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT COALESCE(u.uid_number::text,''), u.username, u.display_name, u.email, u.home_directory, u.status,
       CASE WHEN u.username = t.leader_username THEN '组长' ELSE '组员' END AS role,
       u.username = t.leader_username AS is_leader
FROM teams t
JOIN platform_users u ON u.team_id = t.id
WHERE (t.name = $1 OR t.group_name = $1)
  AND u.status <> 'deleted'
ORDER BY CASE WHEN u.username = t.leader_username THEN 0 ELSE 1 END, u.username
LIMIT 1000`, strings.TrimSpace(teamName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AccountTeamMember{}
	for rows.Next() {
		var item AccountTeamMember
		if err := rows.Scan(&item.UIDNumber, &item.Username, &item.DisplayName, &item.Email, &item.HomeDir, &item.Status, &item.Role, &item.Leader); err != nil {
			return nil, err
		}
		if item.Status == "" {
			item.Status = "active"
		}
		if item.DisplayName == "" {
			item.DisplayName = item.Username
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) FreezeAccount(ctx context.Context, username string) (AccountOperationResult, error) {
	return s.setAccountFrozen(ctx, username, true)
}

func (s *Services) UnfreezeAccount(ctx context.Context, username string) (AccountOperationResult, error) {
	return s.setAccountFrozen(ctx, username, false)
}

func (s *Services) setAccountFrozen(ctx context.Context, username string, frozen bool) (AccountOperationResult, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return AccountOperationResult{}, fmt.Errorf("username is required")
	}
	if s.DB == nil {
		return AccountOperationResult{}, errNotConfigured("postgres")
	}
	status := "active"
	if frozen {
		status = "frozen"
	}
	if err := s.LDAP.SetUserDisabled(username, frozen); err != nil {
		return AccountOperationResult{}, err
	}
	email, err := s.updateAccountStatus(ctx, username, status)
	if err != nil {
		return AccountOperationResult{}, err
	}
	return AccountOperationResult{Username: username, Status: status, Email: email, LDAPUpdated: true, PGUpdated: true, LoginDisabled: frozen}, nil
}

func (s *Services) ResetAccountPassword(ctx context.Context, username string) (AccountOperationResult, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return AccountOperationResult{}, fmt.Errorf("username is required")
	}
	password, err := randomPassword(18)
	if err != nil {
		return AccountOperationResult{}, err
	}
	if err := s.LDAP.SetUserPassword(username, password); err != nil {
		return AccountOperationResult{}, err
	}
	email, err := s.accountEmail(ctx, username)
	if err != nil {
		return AccountOperationResult{}, err
	}
	return AccountOperationResult{Username: username, Status: "password_reset", Email: email, Password: password, LDAPUpdated: true, PGUpdated: false}, nil
}

func (s *Services) DeleteAccount(ctx context.Context, username string) (AccountOperationResult, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return AccountOperationResult{}, fmt.Errorf("username is required")
	}
	if s.DB == nil {
		return AccountOperationResult{}, errNotConfigured("postgres")
	}
	if err := s.LDAP.DeleteUser(username); err != nil {
		return AccountOperationResult{}, err
	}
	email, err := s.updateAccountStatus(ctx, username, "deleted")
	if err != nil {
		return AccountOperationResult{}, err
	}
	return AccountOperationResult{Username: username, Status: "deleted", Email: email, LDAPUpdated: true, PGUpdated: true, LoginDisabled: true}, nil
}

func (s *Services) AccountUnits(ctx context.Context) ([]AccountUnit, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT u.name, COALESCE(u.code,''), u.admin_username, u.status, u.source,
       COUNT(DISTINCT t.id)::int AS team_count,
       COUNT(DISTINCT COALESCE(p.id, tp.id))::int AS member_count
FROM units u
LEFT JOIN teams t ON t.unit_id = u.id
LEFT JOIN platform_users p ON p.unit_id = u.id
LEFT JOIN platform_users tp ON tp.team_id = t.id
GROUP BY u.id
ORDER BY u.name
LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AccountUnit{}
	for rows.Next() {
		var item AccountUnit
		if err := rows.Scan(&item.Name, &item.Code, &item.Admin, &item.Status, &item.Source, &item.Teams, &item.Members); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) AdminUsers(ctx context.Context) ([]AdminUser, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT username, role_name, status, email, created_by, last_login
FROM admin_users
ORDER BY username
LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AdminUser{}
	for rows.Next() {
		var item AdminUser
		if err := rows.Scan(&item.Username, &item.RoleName, &item.Status, &item.Email, &item.CreatedBy, &item.LastLogin); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) UpdateAdminUser(ctx context.Context, username string, update AdminUpdate) (AdminUser, error) {
	username = strings.TrimSpace(username)
	update.Email = strings.TrimSpace(update.Email)
	update.RoleName = strings.TrimSpace(update.RoleName)
	update.Status = strings.ToLower(strings.TrimSpace(update.Status))
	if username == "" {
		return AdminUser{}, fmt.Errorf("管理员账号不能为空")
	}
	if update.Email == "" || !strings.Contains(update.Email, "@") {
		return AdminUser{}, fmt.Errorf("管理员邮箱格式不正确")
	}
	if update.RoleName == "" {
		return AdminUser{}, fmt.Errorf("管理员角色不能为空")
	}
	if update.Status == "" {
		update.Status = "active"
	}
	if update.Status != "active" && update.Status != "frozen" {
		return AdminUser{}, fmt.Errorf("管理员状态只允许 active 或 frozen")
	}
	var item AdminUser
	err := s.DB.QueryRowContext(ctx, `
UPDATE admin_users
SET email = $2, role_name = $3, status = $4, updated_at = now()
WHERE username = $1
RETURNING username, role_name, status, email, created_by, last_login
`, username, update.Email, update.RoleName, update.Status).Scan(
		&item.Username, &item.RoleName, &item.Status, &item.Email, &item.CreatedBy, &item.LastLogin,
	)
	if err == sql.ErrNoRows {
		return AdminUser{}, fmt.Errorf("管理员账号 %s 不存在", username)
	}
	return item, err
}

func (s *Services) ResetAdminPassword(
	ctx context.Context,
	username string,
	deliver func(email, password string) error,
) (AdminPasswordResetResult, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return AdminPasswordResetResult{}, fmt.Errorf("管理员账号不能为空")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return AdminPasswordResetResult{}, err
	}
	defer tx.Rollback()

	var email string
	if err := tx.QueryRowContext(ctx, `
SELECT email FROM admin_users WHERE username = $1 FOR UPDATE
`, username).Scan(&email); err != nil {
		if err == sql.ErrNoRows {
			return AdminPasswordResetResult{}, fmt.Errorf("管理员账号 %s 不存在", username)
		}
		return AdminPasswordResetResult{}, err
	}
	email = strings.TrimSpace(email)
	if email == "" || !strings.Contains(email, "@") {
		return AdminPasswordResetResult{}, fmt.Errorf("管理员账号未绑定有效邮箱")
	}
	password, err := randomPassword(18)
	if err != nil {
		return AdminPasswordResetResult{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AdminPasswordResetResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE admin_users
SET password_hash = $2, password_changed_at = now(), updated_at = now()
WHERE username = $1
`, username, string(hash)); err != nil {
		return AdminPasswordResetResult{}, err
	}
	if deliver == nil {
		return AdminPasswordResetResult{}, fmt.Errorf("邮件发送服务未配置")
	}
	if err := deliver(email, password); err != nil {
		return AdminPasswordResetResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminPasswordResetResult{}, err
	}
	return AdminPasswordResetResult{Username: username, Email: email}, nil
}

func (s *Services) DeleteAdminUser(ctx context.Context, username string) (AdminDeleteResult, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return AdminDeleteResult{}, fmt.Errorf("管理员账号不能为空")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return AdminDeleteResult{}, err
	}
	defer tx.Rollback()

	var status, roleName string
	if err := tx.QueryRowContext(ctx, `
SELECT status, role_name FROM admin_users WHERE username = $1 FOR UPDATE
`, username).Scan(&status, &roleName); err != nil {
		if err == sql.ErrNoRows {
			return AdminDeleteResult{}, fmt.Errorf("管理员账号 %s 不存在", username)
		}
		return AdminDeleteResult{}, err
	}
	var activeCount int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM admin_users WHERE status = 'active'
`).Scan(&activeCount); err != nil {
		return AdminDeleteResult{}, err
	}
	var activeClusterAdmins int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM admin_users
WHERE status = 'active' AND role_name = 'cluster_admin'
`).Scan(&activeClusterAdmins); err != nil {
		return AdminDeleteResult{}, err
	}
	if err := validateAdminDeletion(username, status, roleName, activeCount, activeClusterAdmins); err != nil {
		return AdminDeleteResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM admin_users WHERE username = $1`, username); err != nil {
		return AdminDeleteResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return AdminDeleteResult{}, err
	}
	return AdminDeleteResult{Username: username, Deleted: true}, nil
}

func validateAdminDeletion(username, status, roleName string, activeCount, activeClusterAdmins int) error {
	if strings.EqualFold(strings.TrimSpace(username), "admin") {
		return fmt.Errorf("内置管理员 admin 不允许删除")
	}
	if strings.EqualFold(strings.TrimSpace(status), "active") && activeCount <= 1 {
		return fmt.Errorf("不能删除系统中最后一个正常管理员")
	}
	if strings.EqualFold(strings.TrimSpace(status), "active") &&
		strings.EqualFold(strings.TrimSpace(roleName), ClusterAdminRole) &&
		activeClusterAdmins <= 1 {
		return fmt.Errorf("不能删除系统中最后一个有效 cluster_admin")
	}
	return nil
}

func (s *Services) Roles(ctx context.Context) ([]RoleRecord, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT r.code, r.name, r.scope_type, r.permission_summary, COUNT(ur.user_id)::int
FROM roles r
LEFT JOIN user_roles ur ON ur.role_id = r.id
GROUP BY r.id
ORDER BY r.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RoleRecord{}
	for rows.Next() {
		var item RoleRecord
		if err := rows.Scan(&item.Code, &item.Name, &item.ScopeType, &item.PermissionSummary, &item.UserCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func atoiNull(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return nil
	}
	return n
}

func ldapUserStatus(loginShell string) string {
	shell := strings.ToLower(strings.TrimSpace(loginShell))
	if strings.Contains(shell, "nologin") || strings.Contains(shell, "false") {
		return "frozen"
	}
	return "active"
}

func (s *Services) updateAccountStatus(ctx context.Context, username, status string) (string, error) {
	var email string
	err := s.DB.QueryRowContext(ctx, `
UPDATE platform_users
SET status = $2, updated_at = now()
WHERE username = $1
RETURNING email
`, username, status).Scan(&email)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("project user %s not found", username)
	}
	return email, err
}

func (s *Services) accountEmail(ctx context.Context, username string) (string, error) {
	if s.DB == nil {
		return "", nil
	}
	var email string
	err := s.DB.QueryRowContext(ctx, `SELECT email FROM platform_users WHERE username = $1`, username).Scan(&email)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return email, err
}

func randomPassword(length int) (string, error) {
	const lower = "abcdefghijklmnopqrstuvwxyz"
	const upper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const digits = "0123456789"
	const alphabet = lower + upper + digits
	if length < 3 {
		length = 18
	}
	out := make([]byte, length)
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out[0] = lower[int(buf[0])%len(lower)]
	out[1] = upper[int(buf[1])%len(upper)]
	out[2] = digits[int(buf[2])%len(digits)]
	for i, b := range buf[3:] {
		index := i + 3
		out[index] = alphabet[int(b)%len(alphabet)]
	}
	for i := len(out) - 1; i > 0; i-- {
		j := int(buf[i]) % (i + 1)
		out[i], out[j] = out[j], out[i]
	}
	for i, b := range out {
		if b == 0 {
			out[i] = alphabet[int(b)%len(alphabet)]
		}
	}
	return string(out), nil
}

func unitCodeFromGroup(name string) string {
	name = strings.TrimSpace(name)
	lower := strings.ToLower(name)
	for _, suffix := range []string{"_group", "-group", "_team", "-team"} {
		if strings.HasSuffix(lower, suffix) {
			return strings.TrimSpace(name[:len(name)-len(suffix)])
		}
	}
	return name
}
