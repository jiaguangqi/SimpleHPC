package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type AuditEntry struct {
	ID         int64          `json:"id"`
	Actor      string         `json:"actor"`
	ActorType  string         `json:"actorType"`
	Action     string         `json:"action"`
	TargetType string         `json:"targetType"`
	Target     string         `json:"target"`
	Result     string         `json:"result"`
	Detail     map[string]any `json:"detail"`
	RequestID  string         `json:"requestId"`
	IPAddress  string         `json:"ipAddress"`
	CreatedAt  string         `json:"createdAt"`
}

type AuditQuery struct {
	Page, PageSize, Offset int
	Actor, Action, Result  string
}

type RBACShadowComparison struct {
	Permission    string `json:"permission"`
	Route         string `json:"route"`
	Method        string `json:"method,omitempty"`
	Status        int    `json:"status,omitempty"`
	CreatedAt     string `json:"createdAt,omitempty"`
	Reason        string `json:"reason"`
	LegacyAllowed bool   `json:"legacyAllowed"`
	RBACAllowed   bool   `json:"rbacAllowed"`
	Matched       bool   `json:"matched"`
}

type RBACShadowBucket struct {
	Key        string `json:"key"`
	Total      int    `json:"total"`
	Matched    int    `json:"matched"`
	Mismatched int    `json:"mismatched"`
}

type RBACShadowSummary struct {
	Since          string                 `json:"since"`
	Total          int                    `json:"total"`
	Matched        int                    `json:"matched"`
	Mismatched     int                    `json:"mismatched"`
	ResolverErrors int                    `json:"resolverErrors"`
	MatchRate      float64                `json:"matchRate"`
	ByModule       []RBACShadowBucket     `json:"byModule"`
	ByPermission   []RBACShadowBucket     `json:"byPermission"`
	Differences    []RBACShadowComparison `json:"differences"`
}

type PlatformConfig struct {
	Name       string `json:"name"`
	Logo       string `json:"logo"`
	LoginImage string `json:"loginImage"`
	Language   string `json:"language"`
}

func NormalizePlatformConfig(value PlatformConfig) (PlatformConfig, error) {
	value.Name = strings.TrimSpace(value.Name)
	value.Logo = strings.TrimSpace(value.Logo)
	value.LoginImage = strings.TrimSpace(value.LoginImage)
	value.Language = strings.TrimSpace(value.Language)
	if value.Name == "" {
		return value, errors.New("平台名称不能为空")
	}
	if value.Language == "" {
		value.Language = "zh-CN"
	}
	if value.Language != "zh-CN" {
		return value, errors.New("English 和繁体中文待开发，当前仅支持简体中文")
	}
	return value, nil
}

func normalizeAuditQuery(query AuditQuery) AuditQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	query.Actor = strings.TrimSpace(query.Actor)
	query.Action = strings.TrimSpace(query.Action)
	query.Result = strings.ToLower(strings.TrimSpace(query.Result))
	query.Offset = (query.Page - 1) * query.PageSize
	return query
}

func (s *Services) RecordAudit(ctx context.Context, entry AuditEntry) error {
	raw, _ := json.Marshal(entry.Detail)
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO audit_logs(actor,actor_type,action,target_type,target,target_id,result,detail,request_id,ip_address)
VALUES($1,$2,$3,$4,$5,$5,$6,$7::jsonb,$8,$9)`,
		entry.Actor, entry.ActorType, entry.Action, entry.TargetType, entry.Target,
		entry.Result, string(raw), entry.RequestID, entry.IPAddress)
	return err
}

func (s *Services) AuditLogs(ctx context.Context, query AuditQuery) ([]AuditEntry, int, error) {
	query = normalizeAuditQuery(query)
	where := []string{"1=1"}
	args := []any{}
	add := func(column, value string) {
		if value != "" {
			args = append(args, value)
			where = append(where, fmt.Sprintf("%s = $%d", column, len(args)))
		}
	}
	add("actor", query.Actor)
	add("action", query.Action)
	add("result", query.Result)
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM audit_logs WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, query.PageSize, query.Offset)
	rows, err := s.DB.QueryContext(ctx, fmt.Sprintf(`
SELECT id,actor,actor_type,action,target_type,target,result,detail,request_id,ip_address,created_at
FROM audit_logs WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		whereSQL, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []AuditEntry{}
	for rows.Next() {
		var item AuditEntry
		var raw []byte
		var created time.Time
		if err := rows.Scan(&item.ID, &item.Actor, &item.ActorType, &item.Action, &item.TargetType, &item.Target, &item.Result, &raw, &item.RequestID, &item.IPAddress, &created); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(raw, &item.Detail)
		item.CreatedAt = created.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func SummarizeRBACShadow(items []RBACShadowComparison, since time.Time) RBACShadowSummary {
	summary := RBACShadowSummary{Since: since.Format(time.RFC3339)}
	modules := map[string]*RBACShadowBucket{}
	permissions := map[string]*RBACShadowBucket{}
	add := func(bucket map[string]*RBACShadowBucket, key string, matched bool) {
		if key == "" {
			key = "unknown"
		}
		item := bucket[key]
		if item == nil {
			item = &RBACShadowBucket{Key: key}
			bucket[key] = item
		}
		item.Total++
		if matched {
			item.Matched++
		} else {
			item.Mismatched++
		}
	}
	for _, item := range items {
		summary.Total++
		if item.Matched {
			summary.Matched++
		} else {
			summary.Mismatched++
			summary.Differences = append(summary.Differences, item)
		}
		if item.Reason == "resolver_error" {
			summary.ResolverErrors++
		}
		parts := strings.Split(item.Permission, ".")
		module := "unknown"
		if len(parts) > 1 {
			module = parts[1]
		}
		add(modules, module, item.Matched)
		add(permissions, item.Permission, item.Matched)
	}
	if summary.Total > 0 {
		summary.MatchRate = float64(summary.Matched) / float64(summary.Total)
	}
	for _, item := range modules {
		summary.ByModule = append(summary.ByModule, *item)
	}
	for _, item := range permissions {
		summary.ByPermission = append(summary.ByPermission, *item)
	}
	sort.Slice(summary.ByModule, func(i, j int) bool { return summary.ByModule[i].Key < summary.ByModule[j].Key })
	sort.Slice(summary.ByPermission, func(i, j int) bool { return summary.ByPermission[i].Key < summary.ByPermission[j].Key })
	return summary
}

func (s *Services) RBACShadowStats(ctx context.Context, since time.Time) (RBACShadowSummary, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT target,detail,created_at
FROM audit_logs
WHERE action='rbac.shadow.compare' AND created_at >= $1
ORDER BY created_at`, since)
	if err != nil {
		return RBACShadowSummary{}, err
	}
	defer rows.Close()
	items := []RBACShadowComparison{}
	for rows.Next() {
		var item RBACShadowComparison
		var raw []byte
		var created time.Time
		if err := rows.Scan(&item.Permission, &raw, &created); err != nil {
			return RBACShadowSummary{}, err
		}
		var detail struct {
			Route         string `json:"route"`
			Method        string `json:"method"`
			Status        int    `json:"status"`
			Reason        string `json:"reason"`
			LegacyAllowed bool   `json:"legacyAllowed"`
			RBACAllowed   bool   `json:"rbacAllowed"`
			Matched       bool   `json:"matched"`
		}
		if err := json.Unmarshal(raw, &detail); err != nil {
			return RBACShadowSummary{}, err
		}
		item.Route, item.Method, item.Status, item.Reason = detail.Route, detail.Method, detail.Status, detail.Reason
		item.CreatedAt = created.Format(time.RFC3339)
		item.LegacyAllowed, item.RBACAllowed, item.Matched =
			detail.LegacyAllowed, detail.RBACAllowed, detail.Matched
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return RBACShadowSummary{}, err
	}
	return SummarizeRBACShadow(items, since), nil
}
