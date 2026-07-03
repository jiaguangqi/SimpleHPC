package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"simplehpc/backend/internal/integrations/slurm"
)

func acknowledgeAlert(alert DashboardAlert, username string, now time.Time) DashboardAlert {
	alert.Status = "acknowledged"
	alert.AcknowledgedBy = username
	alert.AcknowledgedAt = now.Format(time.RFC3339)
	return alert
}

func nodeAlertCandidates(nodes []slurm.Node) []DashboardAlert {
	items := []DashboardAlert{}
	for _, node := range nodes {
		state := strings.ToLower(strings.TrimSpace(node.State))
		if !strings.Contains(state, "down") && !strings.Contains(state, "drain") && !strings.Contains(state, "fail") && !strings.Contains(state, "unknown") {
			continue
		}
		level := "warning"
		if strings.Contains(state, "down") || strings.Contains(state, "fail") {
			level = "critical"
		}
		items = append(items, DashboardAlert{
			Level:   level,
			Status:  "active",
			Title:   fmt.Sprintf("节点 %s 状态异常", node.Name),
			Message: fmt.Sprintf("Slurm 节点状态为 %s", node.State),
			Source:  "slurm-node:" + node.Name,
		})
	}
	return items
}

func (s *Services) RefreshMonitoringAlerts(ctx context.Context) error {
	nodes, err := s.Slurm.Nodes(ctx)
	if err != nil {
		return err
	}
	candidates := nodeAlertCandidates(nodes)
	activeSources := make([]string, 0, len(candidates))
	for _, item := range candidates {
		activeSources = append(activeSources, item.Source)
		_, err := s.DB.ExecContext(ctx, `
INSERT INTO dashboard_alerts (level,status,title,message,source)
SELECT $1,'active',$2,$3,$4
WHERE NOT EXISTS (
  SELECT 1 FROM dashboard_alerts
  WHERE source=$4 AND status IN ('active','acknowledged')
)`, item.Level, item.Title, item.Message, item.Source)
		if err != nil {
			return err
		}
	}
	query := `UPDATE dashboard_alerts SET status='resolved',resolved_at=now()
WHERE source LIKE 'slurm-node:%' AND status IN ('active','acknowledged')`
	args := []any{}
	if len(activeSources) > 0 {
		placeholders := make([]string, len(activeSources))
		for index, source := range activeSources {
			args = append(args, source)
			placeholders[index] = "$" + strconv.Itoa(index+1)
		}
		query += " AND source NOT IN (" + strings.Join(placeholders, ",") + ")"
	}
	_, err = s.DB.ExecContext(ctx, query, args...)
	return err
}

func (s *Services) MonitoringAlerts(ctx context.Context, status string, limit int) ([]DashboardAlert, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	where := "1=1"
	args := []any{}
	status = strings.TrimSpace(status)
	if status != "" {
		args = append(args, status)
		where = "status = $1"
	}
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, `
SELECT id,level,status,title,message,source,occurred_at,acknowledged_by,acknowledged_at
FROM dashboard_alerts WHERE `+where+`
ORDER BY CASE level WHEN 'critical' THEN 0 WHEN 'warning' THEN 1 ELSE 2 END, occurred_at DESC
LIMIT $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DashboardAlert{}
	for rows.Next() {
		var item DashboardAlert
		var occurred time.Time
		var acknowledged sql.NullTime
		if err := rows.Scan(&item.ID, &item.Level, &item.Status, &item.Title, &item.Message, &item.Source, &occurred, &item.AcknowledgedBy, &acknowledged); err != nil {
			return nil, err
		}
		item.OccurredAt = occurred.Format(time.RFC3339)
		if acknowledged.Valid {
			item.AcknowledgedAt = acknowledged.Time.Format(time.RFC3339)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) AcknowledgeMonitoringAlert(ctx context.Context, id int64, username string) (DashboardAlert, error) {
	var item DashboardAlert
	var occurred time.Time
	var acknowledged sql.NullTime
	err := s.DB.QueryRowContext(ctx, `
UPDATE dashboard_alerts
SET status='acknowledged',acknowledged_by=$2,acknowledged_at=now()
WHERE id=$1 AND status='active'
RETURNING id,level,status,title,message,source,occurred_at,acknowledged_by,acknowledged_at`,
		id, username).Scan(&item.ID, &item.Level, &item.Status, &item.Title, &item.Message, &item.Source, &occurred, &item.AcknowledgedBy, &acknowledged)
	if err != nil {
		return item, err
	}
	item.OccurredAt = occurred.Format(time.RFC3339)
	if acknowledged.Valid {
		item.AcknowledgedAt = acknowledged.Time.Format(time.RFC3339)
	}
	return item, nil
}
