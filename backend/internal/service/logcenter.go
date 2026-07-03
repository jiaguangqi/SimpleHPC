package service

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type AuthEvent struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	AccountType string `json:"accountType"`
	Event       string `json:"event"`
	Result      string `json:"result"`
	IPAddress   string `json:"ipAddress"`
	UserAgent   string `json:"userAgent"`
	SessionID   string `json:"sessionId,omitempty"`
	Message     string `json:"message"`
	CreatedAt   string `json:"createdAt"`
}

type AuthEventQuery struct {
	Page, PageSize, Offset       int
	Username, Event, Result, Key string
}

type SystemLogItem struct {
	Source    string `json:"source"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type systemLogSource struct {
	Kind, Name string
}

var systemLogSources = map[string]systemLogSource{
	"simplehpc-backend": {Kind: "journal", Name: "simplehpc-backend"},
	"slurmctld":         {Kind: "journal", Name: "slurmctld"},
	"slurmd":            {Kind: "journal", Name: "slurmd"},
	"postgres":          {Kind: "docker", Name: "simplehpc-postgres"},
	"redis":             {Kind: "docker", Name: "simplehpc-redis"},
	"ldap":              {Kind: "docker", Name: "simplehpc-openldap"},
}

func normalizeAuthEventQuery(query AuthEventQuery) AuthEventQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	query.Username = strings.TrimSpace(query.Username)
	query.Event = strings.ToLower(strings.TrimSpace(query.Event))
	query.Result = strings.ToLower(strings.TrimSpace(query.Result))
	query.Key = strings.TrimSpace(query.Key)
	query.Offset = (query.Page - 1) * query.PageSize
	return query
}

func (s *Services) RecordAuthEvent(ctx context.Context, item AuthEvent) error {
	if s.DB == nil {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO auth_events(username,display_name,account_type,event,result,ip_address,user_agent,session_id,message) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		item.Username, item.DisplayName, item.AccountType, item.Event, item.Result, item.IPAddress, item.UserAgent, item.SessionID, item.Message)
	return err
}

func (s *Services) AuthEvents(ctx context.Context, query AuthEventQuery) ([]AuthEvent, int, error) {
	query = normalizeAuthEventQuery(query)
	where, args := []string{"1=1"}, []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, strings.ReplaceAll(clause, "%d", strconv.Itoa(len(args))))
	}
	if query.Username != "" {
		add("username=$%d", query.Username)
	}
	if query.Event != "" {
		add("event=$%d", query.Event)
	}
	if query.Result != "" {
		add("result=$%d", query.Result)
	}
	if query.Key != "" {
		add("(username ILIKE '%%'||$%d||'%%' OR display_name ILIKE '%%'||$%d||'%%' OR ip_address ILIKE '%%'||$%d||'%%')", query.Key)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM auth_events WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, query.PageSize, query.Offset)
	rows, err := s.DB.QueryContext(ctx, fmt.Sprintf(`SELECT id,username,display_name,account_type,event,result,ip_address,user_agent,session_id,message,created_at FROM auth_events WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, whereSQL, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []AuthEvent{}
	for rows.Next() {
		var item AuthEvent
		var created time.Time
		if err := rows.Scan(&item.ID, &item.Username, &item.DisplayName, &item.AccountType, &item.Event, &item.Result, &item.IPAddress, &item.UserAgent, &item.SessionID, &item.Message, &created); err != nil {
			return nil, 0, err
		}
		item.CreatedAt = created.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func parseSystemLogLine(source, line string) SystemLogItem {
	item := SystemLogItem{Source: source, Level: "info", Message: strings.TrimSpace(line)}
	fields := strings.Fields(line)
	if len(fields) > 0 && strings.Contains(fields[0], "T") {
		item.Timestamp = fields[0]
	}
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "fatal"), strings.Contains(lower, "panic"), strings.Contains(lower, "error"), strings.Contains(lower, "failed"):
		item.Level = "error"
	case strings.Contains(lower, "warn"):
		item.Level = "warning"
	case strings.Contains(lower, "debug"):
		item.Level = "debug"
	}
	return item
}

func isSystemLogMetadata(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "-- Logs begin at") || line == "-- No entries --"
}

func (s *Services) SystemLogs(ctx context.Context, source, since string, limit int, keyword, level string) ([]SystemLogItem, error) {
	spec, ok := systemLogSources[source]
	if !ok {
		return nil, fmt.Errorf("不允许的系统日志来源")
	}
	durations := map[string]string{"1h": "1 hour ago", "6h": "6 hours ago", "24h": "24 hours ago", "7d": "7 days ago"}
	journalSince, ok := durations[since]
	if !ok {
		since, journalSince = "1h", "1 hour ago"
	}
	if limit < 1 || limit > 1000 {
		limit = 300
	}
	var command *exec.Cmd
	if spec.Kind == "journal" {
		command = exec.CommandContext(ctx, "journalctl", "-u", spec.Name, "--since", journalSince, "-n", strconv.Itoa(limit), "--no-pager", "-o", "short-iso")
	} else {
		command = exec.CommandContext(ctx, "docker", "logs", "--since", since, "--tail", strconv.Itoa(limit), "--timestamps", spec.Name)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, strings.TrimSpace(string(output)))
	}
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	level = strings.ToLower(strings.TrimSpace(level))
	allowedLevels := map[string]bool{"": true, "all": true, "info": true, "warning": true, "error": true, "debug": true}
	if !allowedLevels[level] {
		level = ""
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	items := make([]SystemLogItem, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || isSystemLogMetadata(line) || keyword != "" && !strings.Contains(strings.ToLower(line), keyword) {
			continue
		}
		item := parseSystemLogLine(source, line)
		if level != "" && level != "all" && item.Level != level {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

var auditPathCleanup = regexp.MustCompile(`[:{}]?[A-Za-z0-9_-]+`)

func AuditActionForRequest(method, route string) string {
	method = strings.ToUpper(method)
	if method != "POST" && method != "PUT" && method != "PATCH" && method != "DELETE" {
		return ""
	}
	route = strings.TrimPrefix(route, "/api/v1/")
	parts := []string{}
	for _, part := range strings.Split(route, "/") {
		if part == "" || strings.HasPrefix(part, ":") {
			continue
		}
		parts = append(parts, auditPathCleanup.ReplaceAllString(part, part))
	}
	return "http." + strings.ToLower(method) + "." + strings.Join(parts, ".")
}
