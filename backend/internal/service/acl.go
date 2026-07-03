package service

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var ErrPathOutsideRoots = errors.New("ACL 路径不在已配置存储根目录中")

type ACLRequest struct {
	Path        string `json:"path"`
	SubjectType string `json:"subjectType"`
	Subject     string `json:"subject"`
	Permission  string `json:"permission"`
}

type ACLRecord struct {
	ID          int64  `json:"id"`
	Path        string `json:"path"`
	SubjectType string `json:"subjectType"`
	Subject     string `json:"subject"`
	Permission  string `json:"permission"`
	CreatedBy   string `json:"createdBy"`
	CreatedAt   string `json:"createdAt"`
}

func ValidateACLRequest(request ACLRequest, roots []string) error {
	request.Path = filepath.Clean(strings.TrimSpace(request.Path))
	allowed := false
	for _, root := range roots {
		rel, err := filepath.Rel(filepath.Clean(root), request.Path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			allowed = true
			break
		}
	}
	if !allowed {
		return ErrPathOutsideRoots
	}
	if request.SubjectType != "user" && request.SubjectType != "group" {
		return errors.New("授权对象类型只允许 user 或 group")
	}
	if strings.TrimSpace(request.Subject) == "" {
		return errors.New("授权对象不能为空")
	}
	if request.Permission != "r" && request.Permission != "rw" && request.Permission != "rwx" {
		return errors.New("权限只允许 r、rw 或 rwx")
	}
	return nil
}

func (s *Services) storageRootPaths() []string {
	items := s.Storage.ListRoots()
	roots := make([]string, 0, len(items))
	for _, item := range items {
		roots = append(roots, item.Path)
	}
	return roots
}

func (s *Services) CreateACL(ctx context.Context, request ACLRequest, username string) (ACLRecord, error) {
	request.Path = filepath.Clean(strings.TrimSpace(request.Path))
	request.Subject = strings.TrimSpace(request.Subject)
	if err := ValidateACLRequest(request, s.storageRootPaths()); err != nil {
		return ACLRecord{}, err
	}
	prefix := "u"
	checkSQL := "SELECT EXISTS(SELECT 1 FROM platform_users WHERE username=$1 AND status<>'deleted')"
	if request.SubjectType == "group" {
		prefix = "g"
		checkSQL = "SELECT EXISTS(SELECT 1 FROM teams WHERE group_name=$1 OR name=$1)"
	}
	var exists bool
	if err := s.DB.QueryRowContext(ctx, checkSQL, request.Subject).Scan(&exists); err != nil || !exists {
		return ACLRecord{}, fmt.Errorf("授权对象 %s 不存在", request.Subject)
	}
	if output, err := exec.CommandContext(ctx, "setfacl", "-m", fmt.Sprintf("%s:%s:%s", prefix, request.Subject, request.Permission), request.Path).CombinedOutput(); err != nil {
		return ACLRecord{}, fmt.Errorf("setfacl: %s", strings.TrimSpace(string(output)))
	}
	var item ACLRecord
	var created time.Time
	err := s.DB.QueryRowContext(ctx, `
INSERT INTO storage_acls(subject_type,subject_name,path,permission,created_by)
VALUES($1,$2,$3,$4,$5)
ON CONFLICT(subject_type,subject_name,path) DO UPDATE SET permission=EXCLUDED.permission,created_by=EXCLUDED.created_by,created_at=now()
RETURNING id,subject_type,subject_name,path,permission,created_by,created_at`,
		request.SubjectType, request.Subject, request.Path, request.Permission, username).
		Scan(&item.ID, &item.SubjectType, &item.Subject, &item.Path, &item.Permission, &item.CreatedBy, &created)
	item.CreatedAt = created.Format(time.RFC3339)
	return item, err
}

func (s *Services) ACLs(ctx context.Context) ([]ACLRecord, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,subject_type,subject_name,path,permission,created_by,created_at FROM storage_acls ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ACLRecord{}
	for rows.Next() {
		var item ACLRecord
		var created time.Time
		if err := rows.Scan(&item.ID, &item.SubjectType, &item.Subject, &item.Path, &item.Permission, &item.CreatedBy, &created); err != nil {
			return nil, err
		}
		item.CreatedAt = created.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) DeleteACL(ctx context.Context, id int64) error {
	var item ACLRecord
	if err := s.DB.QueryRowContext(ctx, `SELECT subject_type,subject_name,path FROM storage_acls WHERE id=$1`, id).Scan(&item.SubjectType, &item.Subject, &item.Path); err != nil {
		return err
	}
	prefix := "u"
	if item.SubjectType == "group" {
		prefix = "g"
	}
	if output, err := exec.CommandContext(ctx, "setfacl", "-x", fmt.Sprintf("%s:%s", prefix, item.Subject), item.Path).CombinedOutput(); err != nil {
		return fmt.Errorf("setfacl: %s", strings.TrimSpace(string(output)))
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM storage_acls WHERE id=$1`, id)
	return err
}
