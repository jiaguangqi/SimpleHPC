package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var storageUsernamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,63}$`)

type ResolvedFileRoot struct {
	Path        string      `json:"path"`
	Access      AccessLevel `json:"access"`
	AllowHidden bool        `json:"allowHidden"`
}

type StorageAuthorization struct {
	Roots []ResolvedFileRoot
}

func (a StorageAuthorization) RootPaths() []string {
	result := make([]string, 0, len(a.Roots))
	for _, root := range a.Roots {
		result = append(result, root.Path)
	}
	return result
}

func (a StorageAuthorization) Allows(path string, required AccessLevel) bool {
	clean := filepath.Clean(path)
	for _, root := range a.Roots {
		rootPath := filepath.Clean(root.Path)
		if clean != rootPath && !strings.HasPrefix(clean, rootPath+string(os.PathSeparator)) {
			continue
		}
		return accessRank(root.Access) >= accessRank(required)
	}
	return false
}

func (a StorageAuthorization) AllowsHidden(path string) bool {
	clean := filepath.Clean(path)
	for _, root := range a.Roots {
		rootPath := filepath.Clean(root.Path)
		if (clean == rootPath || strings.HasPrefix(clean, rootPath+string(os.PathSeparator))) && root.AllowHidden {
			return true
		}
	}
	return false
}

func ExpandFilePolicies(
	configuredRoots []string,
	username string,
	unitIDs, teamIDs []string,
	policies []FilePolicyGrant,
	members map[string][]string,
) ([]ResolvedFileRoot, error) {
	configured := map[string]bool{}
	for _, root := range configuredRoots {
		clean := filepath.Clean(root)
		if !filepath.IsAbs(clean) {
			return nil, fmt.Errorf("存储根目录必须是绝对路径")
		}
		configured[clean] = true
	}
	resolved := map[string]ResolvedFileRoot{}
	add := func(path string, policy FilePolicyGrant) {
		clean := filepath.Clean(path)
		current, ok := resolved[clean]
		if !ok || accessRank(policy.Access) > accessRank(current.Access) {
			current = ResolvedFileRoot{Path: clean, Access: policy.Access, AllowHidden: policy.AllowHidden}
		} else if policy.AllowHidden {
			current.AllowHidden = true
		}
		resolved[clean] = current
	}
	for _, policy := range policies {
		root := filepath.Clean(policy.StorageRoot)
		if !configured[root] {
			return nil, fmt.Errorf("文件策略目录不在授权存储根中: %s", root)
		}
		switch policy.SubjectScope {
		case "self":
			if !storageUsernamePattern.MatchString(username) {
				return nil, fmt.Errorf("无效的存储用户名")
			}
			add(filepath.Join(root, username), policy)
		case "global":
			add(root, policy)
		case "team_shared":
			for _, id := range teamIDs {
				add(filepath.Join(root, "teams", id), policy)
			}
		case "unit_shared":
			for _, id := range unitIDs {
				add(filepath.Join(root, "units", id), policy)
			}
		case "team_members":
			for _, id := range teamIDs {
				for _, member := range members["team:"+id] {
					if storageUsernamePattern.MatchString(member) {
						add(filepath.Join(root, member), policy)
					}
				}
			}
		case "unit_members":
			for _, id := range unitIDs {
				for _, member := range members["unit:"+id] {
					if storageUsernamePattern.MatchString(member) {
						add(filepath.Join(root, member), policy)
					}
				}
			}
		default:
			return nil, fmt.Errorf("未知文件主体范围: %s", policy.SubjectScope)
		}
	}
	keys := make([]string, 0, len(resolved))
	for path := range resolved {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	result := make([]ResolvedFileRoot, 0, len(keys))
	for _, path := range keys {
		result = append(result, resolved[path])
	}
	return result, nil
}

func UserStorageRoots(configuredRoots []string, username string) ([]string, error) {
	username = strings.TrimSpace(username)
	if !storageUsernamePattern.MatchString(username) {
		return nil, fmt.Errorf("无效的存储用户名")
	}
	roots := make([]string, 0, len(configuredRoots))
	for _, configured := range configuredRoots {
		root := filepath.Clean(configured)
		if !filepath.IsAbs(root) {
			return nil, fmt.Errorf("存储根目录必须是绝对路径")
		}
		roots = append(roots, filepath.Join(root, username))
	}
	return roots, nil
}

func EnsureUserStorageDirectories(roots []string, uid, gid int) error {
	for _, root := range roots {
		parent := filepath.Dir(root)
		parentInfo, err := os.Stat(parent)
		if err != nil || !parentInfo.IsDir() {
			return fmt.Errorf("授权存储根目录不可用: %s", parent)
		}
		info, err := os.Lstat(root)
		if os.IsNotExist(err) {
			if err := os.Mkdir(root, 0o700); err != nil {
				return fmt.Errorf("创建用户目录 %s: %w", root, err)
			}
			info, err = os.Lstat(root)
		}
		if err != nil {
			return fmt.Errorf("检查用户目录 %s: %w", root, err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("用户存储路径不是安全目录: %s", root)
		}
		if err := os.Chown(root, uid, gid); err != nil {
			return fmt.Errorf("修正用户目录属主 %s: %w", root, err)
		}
		if err := os.Chmod(root, 0o700); err != nil {
			return fmt.Errorf("修正用户目录权限 %s: %w", root, err)
		}
	}
	return nil
}

func (s *Services) EnsureUserStorageAccess(ctx context.Context, username string) ([]string, error) {
	var uid, gid int
	var status string
	if err := s.DB.QueryRowContext(ctx, `SELECT uid_number,gid_number,status FROM platform_users WHERE username=$1`, username).Scan(&uid, &gid, &status); err != nil {
		return nil, fmt.Errorf("未找到用户 %s 的 UID/GID: %w", username, err)
	}
	if status != "active" {
		return nil, fmt.Errorf("用户账号不可用")
	}
	roots, err := UserStorageRoots(s.Storage.Roots, username)
	if err != nil {
		return nil, err
	}
	if err := EnsureUserStorageDirectories(roots, uid, gid); err != nil {
		return nil, err
	}
	return roots, nil
}

func (s *Services) ResolveStorageAuthorization(
	ctx context.Context,
	user AuthUser,
	authz PermissionContext,
) (StorageAuthorization, error) {
	if authz.IsClusterAdmin {
		roots := make([]ResolvedFileRoot, 0, len(s.Storage.Roots))
		for _, root := range s.Storage.Roots {
			roots = append(roots, ResolvedFileRoot{
				Path: filepath.Clean(root), Access: AccessManage, AllowHidden: true,
			})
		}
		return StorageAuthorization{Roots: roots}, nil
	}
	hasSelf := false
	for _, policy := range authz.FilePolicies {
		if policy.SubjectScope == "self" {
			hasSelf = true
			break
		}
	}
	if hasSelf {
		if _, err := s.EnsureUserStorageAccess(ctx, user.Username); err != nil {
			return StorageAuthorization{}, err
		}
	}
	members, err := s.filePolicyMembers(ctx, authz, authz.FilePolicies)
	if err != nil {
		return StorageAuthorization{}, err
	}
	roots, err := ExpandFilePolicies(
		s.Storage.Roots, user.Username, authz.UnitIDs, authz.TeamIDs, authz.FilePolicies, members,
	)
	if err != nil {
		return StorageAuthorization{}, err
	}
	return StorageAuthorization{Roots: roots}, nil
}

func (s *Services) filePolicyMembers(
	ctx context.Context,
	authz PermissionContext,
	policies []FilePolicyGrant,
) (map[string][]string, error) {
	result := map[string][]string{}
	needTeam, needUnit := false, false
	for _, policy := range policies {
		needTeam = needTeam || policy.SubjectScope == "team_members"
		needUnit = needUnit || policy.SubjectScope == "unit_members"
	}
	load := func(kind string, ids []string, column string) error {
		for _, id := range ids {
			rows, err := s.DB.QueryContext(ctx,
				`SELECT username FROM platform_users WHERE `+column+`::text=$1 AND status='active' ORDER BY username`, id)
			if err != nil {
				return err
			}
			for rows.Next() {
				var username string
				if err := rows.Scan(&username); err != nil {
					rows.Close()
					return err
				}
				result[kind+":"+id] = append(result[kind+":"+id], username)
			}
			if err := rows.Close(); err != nil {
				return err
			}
		}
		return nil
	}
	if needTeam {
		if err := load("team", authz.TeamIDs, "team_id"); err != nil {
			return nil, err
		}
	}
	if needUnit {
		if err := load("unit", authz.UnitIDs, "unit_id"); err != nil {
			return nil, err
		}
	}
	return result, nil
}
