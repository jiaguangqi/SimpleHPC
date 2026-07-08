package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type LicenseConfig struct {
	ID                 int64  `json:"id"`
	AppName            string `json:"appName"`
	AppCode            string `json:"appCode"`
	AppType            string `json:"appType"`
	IconURL            string `json:"iconUrl"`
	Vendor             string `json:"vendor"`
	LicenseType        string `json:"licenseType"`
	ManagerName        string `json:"managerName"`
	ServerHost         string `json:"serverHost"`
	Port               int    `json:"port"`
	CollectMethod      string `json:"collectMethod"`
	CollectCommand     string `json:"collectCommand"`
	ServiceName        string `json:"serviceName"`
	CollectIntervalSec int    `json:"collectIntervalSec"`
	TimeoutSec         int    `json:"timeoutSec"`
	WarningThreshold   int    `json:"warningThreshold"`
	CriticalThreshold  int    `json:"criticalThreshold"`
	ExpireWarningDays  int    `json:"expireWarningDays"`
	Enabled            bool   `json:"enabled"`
	ServiceStatus      string `json:"serviceStatus"`
	LastCollectStatus  string `json:"lastCollectStatus"`
	LastCollectMessage string `json:"lastCollectMessage"`
	LastRawOutput      string `json:"lastRawOutput,omitempty"`
	LastCollectedAt    string `json:"lastCollectedAt,omitempty"`
	CreatedAt          string `json:"createdAt,omitempty"`
	UpdatedAt          string `json:"updatedAt,omitempty"`
}

type LicenseFeature struct {
	ConfigID    int64  `json:"configId"`
	AppName     string `json:"appName"`
	IconURL     string `json:"iconUrl"`
	FeatureName string `json:"featureName"`
	Total       int    `json:"total"`
	Used        int    `json:"used"`
	Free        int    `json:"free"`
	Queued      int    `json:"queued"`
	UsageRate   string `json:"usageRate"`
	ExpiresAt   string `json:"expiresAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

type LicenseUsageSession struct {
	ConfigID      int64  `json:"configId"`
	AppName       string `json:"appName"`
	FeatureName   string `json:"featureName"`
	Username      string `json:"username"`
	JobID         string `json:"jobId"`
	NodeName      string `json:"nodeName"`
	HostName      string `json:"hostName"`
	ProcessID     string `json:"processId"`
	CheckoutCount int    `json:"checkoutCount"`
	StartedAt     string `json:"startedAt,omitempty"`
	LastSeenAt    string `json:"lastSeenAt,omitempty"`
	Status        string `json:"status"`
}

type LicenseStatusOverview struct {
	AppCount       int    `json:"appCount"`
	TotalLicenses  int    `json:"totalLicenses"`
	UsedLicenses   int    `json:"usedLicenses"`
	FreeLicenses   int    `json:"freeLicenses"`
	QueuedCount    int    `json:"queuedCount"`
	UsageRate      string `json:"usageRate"`
	HighLoadApps   int    `json:"highLoadApps"`
	AbnormalServer int    `json:"abnormalServer"`
	LastUpdated    string `json:"lastUpdated,omitempty"`
}

type LicenseStatusResponse struct {
	Overview LicenseStatusOverview `json:"overview"`
	Configs  []LicenseConfig       `json:"configs"`
	Features []LicenseFeature      `json:"features"`
	Sessions []LicenseUsageSession `json:"sessions"`
	Samples  []LicenseTrendSample  `json:"samples"`
	Alerts   []DashboardAlert      `json:"alerts"`
}

type licenseParsedFeature struct {
	Name      string
	Total     int
	Used      int
	Queued    int
	ExpiresAt *time.Time
}

type licenseParsedSession struct {
	FeatureName   string
	Username      string
	HostName      string
	ProcessID     string
	CheckoutCount int
}

type licenseParsedOutput struct {
	Features []licenseParsedFeature
	Sessions []licenseParsedSession
}

type LicenseTrendSample struct {
	ConfigID    int64  `json:"configId"`
	AppName     string `json:"appName"`
	FeatureName string `json:"featureName"`
	SampleTime  string `json:"sampleTime"`
	Total       int    `json:"total"`
	Used        int    `json:"used"`
	Free        int    `json:"free"`
	Queued      int    `json:"queued"`
	UsageRate   string `json:"usageRate"`
}

func normalizeLicenseConfig(input LicenseConfig) LicenseConfig {
	input.AppName = strings.TrimSpace(input.AppName)
	input.AppCode = strings.TrimSpace(input.AppCode)
	input.AppType = strings.TrimSpace(input.AppType)
	input.IconURL = strings.TrimSpace(input.IconURL)
	input.Vendor = strings.TrimSpace(input.Vendor)
	input.LicenseType = strings.TrimSpace(input.LicenseType)
	input.ManagerName = strings.TrimSpace(input.ManagerName)
	input.ServerHost = strings.TrimSpace(input.ServerHost)
	input.CollectMethod = strings.TrimSpace(input.CollectMethod)
	input.CollectCommand = strings.TrimSpace(input.CollectCommand)
	input.ServiceName = strings.TrimSpace(input.ServiceName)
	if input.AppCode == "" {
		input.AppCode = strings.ToLower(strings.ReplaceAll(input.AppName, " ", "_"))
	}
	if input.AppType == "" {
		input.AppType = "commercial"
	}
	if input.LicenseType == "" {
		input.LicenseType = "FlexNet"
	}
	if input.CollectMethod == "" {
		input.CollectMethod = "lmstat"
	}
	if input.CollectIntervalSec <= 0 {
		input.CollectIntervalSec = 60
	}
	if input.TimeoutSec <= 0 || input.TimeoutSec > 120 {
		input.TimeoutSec = 10
	}
	if input.WarningThreshold <= 0 {
		input.WarningThreshold = 80
	}
	if input.CriticalThreshold <= 0 {
		input.CriticalThreshold = 95
	}
	if input.ExpireWarningDays <= 0 {
		input.ExpireWarningDays = 30
	}
	return input
}

func validateLicenseConfig(input LicenseConfig) error {
	if input.AppName == "" {
		return errors.New("应用名称不能为空")
	}
	if input.AppCode == "" {
		return errors.New("应用编码不能为空")
	}
	if input.ServerHost == "" {
		return errors.New("license server 地址不能为空")
	}
	if input.Port < 0 || input.Port > 65535 {
		return errors.New("端口范围无效")
	}
	if input.WarningThreshold > input.CriticalThreshold {
		return errors.New("预警阈值不能大于严重阈值")
	}
	return nil
}

func scanLicenseConfig(rows interface{ Scan(dest ...any) error }) (LicenseConfig, error) {
	var item LicenseConfig
	var collected, created, updated sql.NullTime
	err := rows.Scan(&item.ID, &item.AppName, &item.AppCode, &item.AppType, &item.IconURL, &item.Vendor,
		&item.LicenseType, &item.ManagerName, &item.ServerHost, &item.Port, &item.CollectMethod,
		&item.CollectCommand, &item.ServiceName, &item.CollectIntervalSec, &item.TimeoutSec,
		&item.WarningThreshold, &item.CriticalThreshold, &item.ExpireWarningDays, &item.Enabled,
		&item.ServiceStatus, &item.LastCollectStatus, &item.LastCollectMessage, &item.LastRawOutput,
		&collected, &created, &updated)
	if err != nil {
		return item, err
	}
	if collected.Valid {
		item.LastCollectedAt = collected.Time.Format(time.RFC3339)
	}
	if created.Valid {
		item.CreatedAt = created.Time.Format(time.RFC3339)
	}
	if updated.Valid {
		item.UpdatedAt = updated.Time.Format(time.RFC3339)
	}
	return item, nil
}

const licenseConfigColumns = `id,app_name,app_code,app_type,icon_url,vendor,license_type,manager_name,server_host,port,collect_method,collect_command,service_name,collect_interval_sec,timeout_sec,warning_threshold,critical_threshold,expire_warning_days,enabled,service_status,last_collect_status,last_collect_message,last_raw_output,last_collected_at,created_at,updated_at`

func (s *Services) ListLicenseConfigs(ctx context.Context, keyword string, enabled string) ([]LicenseConfig, error) {
	if s.DB == nil {
		return nil, errors.New("数据库未配置")
	}
	where := []string{"1=1"}
	args := []any{}
	if strings.TrimSpace(keyword) != "" {
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(keyword))+"%")
		where = append(where, fmt.Sprintf("(lower(app_name) LIKE $%d OR lower(app_code) LIKE $%d OR lower(server_host) LIKE $%d)", len(args), len(args), len(args)))
	}
	if enabled == "true" || enabled == "false" {
		args = append(args, enabled == "true")
		where = append(where, fmt.Sprintf("enabled=$%d", len(args)))
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+licenseConfigColumns+` FROM license_servers WHERE `+strings.Join(where, " AND ")+` ORDER BY app_name,id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []LicenseConfig{}
	for rows.Next() {
		item, err := scanLicenseConfig(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) GetLicenseConfig(ctx context.Context, id int64) (LicenseConfig, error) {
	if s.DB == nil {
		return LicenseConfig{}, errors.New("数据库未配置")
	}
	return scanLicenseConfig(s.DB.QueryRowContext(ctx, `SELECT `+licenseConfigColumns+` FROM license_servers WHERE id=$1`, id))
}

func (s *Services) SaveLicenseConfig(ctx context.Context, input LicenseConfig) (LicenseConfig, error) {
	if s.DB == nil {
		return LicenseConfig{}, errors.New("数据库未配置")
	}
	input = normalizeLicenseConfig(input)
	if err := validateLicenseConfig(input); err != nil {
		return LicenseConfig{}, err
	}
	if input.ID > 0 {
		return scanLicenseConfig(s.DB.QueryRowContext(ctx, `
UPDATE license_servers SET app_name=$2,app_code=$3,app_type=$4,icon_url=$5,vendor=$6,license_type=$7,manager_name=$8,server_host=$9,port=$10,collect_method=$11,collect_command=$12,service_name=$13,collect_interval_sec=$14,timeout_sec=$15,warning_threshold=$16,critical_threshold=$17,expire_warning_days=$18,enabled=$19,updated_at=now()
WHERE id=$1 RETURNING `+licenseConfigColumns,
			input.ID, input.AppName, input.AppCode, input.AppType, input.IconURL, input.Vendor, input.LicenseType, input.ManagerName,
			input.ServerHost, input.Port, input.CollectMethod, input.CollectCommand, input.ServiceName, input.CollectIntervalSec,
			input.TimeoutSec, input.WarningThreshold, input.CriticalThreshold, input.ExpireWarningDays, input.Enabled))
	}
	return scanLicenseConfig(s.DB.QueryRowContext(ctx, `
INSERT INTO license_servers (app_name,app_code,app_type,icon_url,vendor,license_type,manager_name,server_host,port,collect_method,collect_command,service_name,collect_interval_sec,timeout_sec,warning_threshold,critical_threshold,expire_warning_days,enabled)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
RETURNING `+licenseConfigColumns,
		input.AppName, input.AppCode, input.AppType, input.IconURL, input.Vendor, input.LicenseType, input.ManagerName,
		input.ServerHost, input.Port, input.CollectMethod, input.CollectCommand, input.ServiceName, input.CollectIntervalSec,
		input.TimeoutSec, input.WarningThreshold, input.CriticalThreshold, input.ExpireWarningDays, input.Enabled))
}

func (s *Services) DeleteLicenseConfig(ctx context.Context, id int64) error {
	if s.DB == nil {
		return errors.New("数据库未配置")
	}
	result, err := s.DB.ExecContext(ctx, `DELETE FROM license_servers WHERE id=$1`, id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Services) listLicenseFeatures(ctx context.Context, configID int64) ([]LicenseFeature, error) {
	where := ""
	args := []any{}
	if configID > 0 {
		where = " WHERE f.config_id=$1"
		args = append(args, configID)
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT f.config_id,s.app_name,s.icon_url,f.feature_name,f.total_count,f.used_count,f.free_count,f.queued_count,f.expires_at,f.updated_at
FROM license_features f JOIN license_servers s ON s.id=f.config_id`+where+`
ORDER BY s.app_name,f.feature_name`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	features := []LicenseFeature{}
	for rows.Next() {
		var feature LicenseFeature
		var expires sql.NullTime
		var updated time.Time
		if err := rows.Scan(&feature.ConfigID, &feature.AppName, &feature.IconURL, &feature.FeatureName, &feature.Total, &feature.Used, &feature.Free, &feature.Queued, &expires, &updated); err != nil {
			return nil, err
		}
		feature.UsageRate = percent(feature.Used, feature.Total)
		if expires.Valid {
			feature.ExpiresAt = expires.Time.Format(time.RFC3339)
		}
		feature.UpdatedAt = updated.Format(time.RFC3339)
		features = append(features, feature)
	}
	return features, rows.Err()
}

func licenseCommandArgs(cfg LicenseConfig) (string, []string, error) {
	commandText := strings.TrimSpace(cfg.CollectCommand)
	if commandText == "" {
		target := cfg.ServerHost
		if cfg.Port > 0 {
			target = strconv.Itoa(cfg.Port) + "@" + cfg.ServerHost
		}
		commandText = "lmstat -a -c " + target
	}
	parts := strings.Fields(commandText)
	if len(parts) == 0 {
		return "", nil, errors.New("采集命令为空")
	}
	execName := filepath.Base(parts[0])
	allowed := map[string]bool{"lmstat": true, "lmutil": true, "rlmutil": true}
	if !allowed[execName] {
		return "", nil, fmt.Errorf("不允许的采集命令 %s，仅允许 lmstat/lmutil/rlmutil", execName)
	}
	return parts[0], parts[1:], nil
}

func runCommandWithTimeout(ctx context.Context, timeoutSec int, name string, args ...string) (string, error) {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(output), errors.New("命令执行超时")
	}
	if err != nil {
		return string(output), err
	}
	return string(output), nil
}

func truncateLicenseOutput(raw string) string {
	raw = strings.TrimSpace(raw)
	const maxLen = 20000
	if len(raw) <= maxLen {
		return raw
	}
	return raw[:maxLen] + "\n... 输出过长，已截断"
}

func serviceStatus(ctx context.Context, serviceName string, timeoutSec int) string {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return "unmanaged"
	}
	out, err := runCommandWithTimeout(ctx, timeoutSec, "systemctl", "show", serviceName, "-p", "ActiveState", "--value")
	status := strings.TrimSpace(out)
	if err == nil && status != "" {
		return status
	}
	out, err = runCommandWithTimeout(ctx, timeoutSec, "systemctl", "is-active", serviceName)
	status = strings.TrimSpace(out)
	if err != nil && status == "" {
		return "unknown"
	}
	if status == "" {
		return "unknown"
	}
	return status
}

var flexFeatureRe = regexp.MustCompile(`(?i)Users of\s+([^:]+):.*Total of\s+([0-9]+)\s+licenses?\s+issued.*Total of\s+([0-9]+)\s+licenses?\s+in use`)
var flexExpiryRe = regexp.MustCompile(`(?i)(?:license\s+)?expires?:\s*([0-9]{1,2}-[a-z]{3}-[0-9]{4}|permanent)`)

func parseFlexExpiry(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "permanent") {
		return nil
	}
	parts := strings.Split(value, "-")
	if len(parts) != 3 {
		return nil
	}
	month := map[string]string{
		"jan": "Jan", "feb": "Feb", "mar": "Mar", "apr": "Apr",
		"may": "May", "jun": "Jun", "jul": "Jul", "aug": "Aug",
		"sep": "Sep", "oct": "Oct", "nov": "Nov", "dec": "Dec",
	}[strings.ToLower(parts[1])]
	if month == "" {
		return nil
	}
	normalized := strings.TrimLeft(parts[0], "0") + "-" + month + "-" + parts[2]
	parsed, err := time.Parse("2-Jan-2006", normalized)
	if err != nil {
		return nil
	}
	return &parsed
}

func parseFlexLMOutput(raw string) licenseParsedOutput {
	lines := strings.Split(raw, "\n")
	result := licenseParsedOutput{}
	currentFeature := ""
	featureIndex := map[string]int{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if match := flexFeatureRe.FindStringSubmatch(trimmed); len(match) == 4 {
			total, _ := strconv.Atoi(match[2])
			used, _ := strconv.Atoi(match[3])
			currentFeature = strings.TrimSpace(match[1])
			result.Features = append(result.Features, licenseParsedFeature{Name: currentFeature, Total: total, Used: used})
			featureIndex[currentFeature] = len(result.Features) - 1
			continue
		}
		if currentFeature == "" {
			continue
		}
		if match := flexExpiryRe.FindStringSubmatch(trimmed); len(match) == 2 {
			if idx, ok := featureIndex[currentFeature]; ok {
				result.Features[idx].ExpiresAt = parseFlexExpiry(match[1])
			}
			continue
		}
		if strings.HasPrefix(trimmed, "\"") || strings.Contains(trimmed, "start ") || strings.Contains(trimmed, "linger:") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 1 && !strings.HasPrefix(fields[0], "\"") {
				count := 1
				for idx, f := range fields {
					if strings.EqualFold(f, "licenses") && idx > 0 {
						if parsed, err := strconv.Atoi(fields[idx-1]); err == nil {
							count = parsed
						}
					}
				}
				session := licenseParsedSession{FeatureName: currentFeature, Username: fields[0], CheckoutCount: count}
				if len(fields) > 1 {
					session.HostName = fields[1]
				}
				if len(fields) > 2 {
					session.ProcessID = strings.Trim(fields[2], "(),")
				}
				result.Sessions = append(result.Sessions, session)
			}
		}
	}
	return result
}

func (s *Services) CollectLicenseConfig(ctx context.Context, id int64) (LicenseConfig, []LicenseFeature, error) {
	cfg, err := s.GetLicenseConfig(ctx, id)
	if err != nil {
		return cfg, nil, err
	}
	status := serviceStatus(ctx, cfg.ServiceName, cfg.TimeoutSec)
	name, args, err := licenseCommandArgs(cfg)
	raw := ""
	collectStatus := "success"
	message := "采集成功"
	if err == nil {
		raw, err = runCommandWithTimeout(ctx, cfg.TimeoutSec, name, args...)
	}
	if err != nil {
		collectStatus = "failed"
		message = err.Error()
	}
	raw = truncateLicenseOutput(raw)
	if err != nil {
		if _, dbErr := s.DB.ExecContext(ctx, `UPDATE license_servers SET service_status=$2,last_collect_status=$3,last_collect_message=$4,last_raw_output=$5,last_collected_at=now(),updated_at=now() WHERE id=$1`, id, status, collectStatus, message, raw); dbErr != nil {
			return cfg, nil, dbErr
		}
		existing, _ := s.listLicenseFeatures(ctx, id)
		_ = s.refreshLicenseAlerts(ctx, cfg, status, collectStatus, message, existing)
		cfg, _ = s.GetLicenseConfig(ctx, id)
		return cfg, existing, nil
	}
	parsed := parseFlexLMOutput(raw)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return cfg, nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM license_features WHERE config_id=$1`, id); err != nil {
		return cfg, nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM license_usage_sessions WHERE config_id=$1`, id); err != nil {
		return cfg, nil, err
	}
	features := []LicenseFeature{}
	for _, item := range parsed.Features {
		free := item.Total - item.Used
		if free < 0 {
			free = 0
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO license_features(config_id,feature_name,total_count,used_count,free_count,queued_count,expires_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, id, item.Name, item.Total, item.Used, free, item.Queued, item.ExpiresAt); err != nil {
			return cfg, nil, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO license_usage_samples(config_id,feature_name,total_count,used_count,free_count,queued_count) VALUES ($1,$2,$3,$4,$5,$6)`, id, item.Name, item.Total, item.Used, free, item.Queued); err != nil {
			return cfg, nil, err
		}
		feature := LicenseFeature{ConfigID: id, AppName: cfg.AppName, IconURL: cfg.IconURL, FeatureName: item.Name, Total: item.Total, Used: item.Used, Free: free, Queued: item.Queued, UsageRate: percent(item.Used, item.Total)}
		if item.ExpiresAt != nil {
			feature.ExpiresAt = item.ExpiresAt.Format(time.RFC3339)
		}
		features = append(features, feature)
	}
	for _, sess := range parsed.Sessions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO license_usage_sessions(config_id,feature_name,username,host_name,process_id,checkout_count,status) VALUES ($1,$2,$3,$4,$5,$6,'active')`, id, sess.FeatureName, sess.Username, sess.HostName, sess.ProcessID, sess.CheckoutCount); err != nil {
			return cfg, nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE license_servers SET service_status=$2,last_collect_status=$3,last_collect_message=$4,last_raw_output=$5,last_collected_at=now(),updated_at=now() WHERE id=$1`, id, status, collectStatus, message, raw); err != nil {
		return cfg, nil, err
	}
	if err := tx.Commit(); err != nil {
		return cfg, nil, err
	}
	_ = s.refreshLicenseAlerts(ctx, cfg, status, collectStatus, message, features)
	cfg, _ = s.GetLicenseConfig(ctx, id)
	return cfg, features, nil
}

func percent(used, total int) string {
	if total <= 0 {
		return "0.0%"
	}
	return fmt.Sprintf("%.1f%%", float64(used)*100/float64(total))
}

func (s *Services) refreshLicenseAlerts(ctx context.Context, cfg LicenseConfig, serviceState, collectStatus, message string, features []LicenseFeature) error {
	if s.DB == nil {
		return nil
	}
	candidates := []DashboardAlert{}
	if serviceState != "active" && serviceState != "unmanaged" {
		candidates = append(candidates, DashboardAlert{Level: "critical", Status: "active", Title: cfg.AppName + " License 服务异常", Message: "systemd 服务状态：" + serviceState, Source: fmt.Sprintf("license-service:%d", cfg.ID)})
	}
	if collectStatus != "success" {
		candidates = append(candidates, DashboardAlert{Level: "warning", Status: "active", Title: cfg.AppName + " License 采集失败", Message: message, Source: fmt.Sprintf("license-collect:%d", cfg.ID)})
	}
	for _, feature := range features {
		rate := 0
		if feature.Total > 0 {
			rate = feature.Used * 100 / feature.Total
		}
		if rate >= cfg.CriticalThreshold {
			candidates = append(candidates, DashboardAlert{Level: "critical", Status: "active", Title: cfg.AppName + " License 即将耗尽", Message: fmt.Sprintf("%s 使用率 %d%%", feature.FeatureName, rate), Source: fmt.Sprintf("license-usage:%d:%s", cfg.ID, feature.FeatureName)})
		} else if rate >= cfg.WarningThreshold {
			candidates = append(candidates, DashboardAlert{Level: "warning", Status: "active", Title: cfg.AppName + " License 高负载", Message: fmt.Sprintf("%s 使用率 %d%%", feature.FeatureName, rate), Source: fmt.Sprintf("license-usage:%d:%s", cfg.ID, feature.FeatureName)})
		}
		if feature.ExpiresAt != "" {
			if expiresAt, err := time.Parse(time.RFC3339, feature.ExpiresAt); err == nil {
				daysLeft := int(time.Until(expiresAt).Hours() / 24)
				if daysLeft < 0 {
					candidates = append(candidates, DashboardAlert{Level: "critical", Status: "active", Title: cfg.AppName + " License 已过期", Message: fmt.Sprintf("%s 已于 %s 过期", feature.FeatureName, expiresAt.Format("2006-01-02")), Source: fmt.Sprintf("license-expire:%d:%s", cfg.ID, feature.FeatureName)})
				} else if daysLeft <= cfg.ExpireWarningDays {
					candidates = append(candidates, DashboardAlert{Level: "warning", Status: "active", Title: cfg.AppName + " License 即将到期", Message: fmt.Sprintf("%s 预计 %d 天后到期", feature.FeatureName, daysLeft), Source: fmt.Sprintf("license-expire:%d:%s", cfg.ID, feature.FeatureName)})
				}
			}
		}
	}
	activeSources := []string{}
	for _, item := range candidates {
		activeSources = append(activeSources, item.Source)
		if _, err := s.DB.ExecContext(ctx, `
INSERT INTO dashboard_alerts(level,status,title,message,source)
SELECT $1,'active',$2,$3,$4
WHERE NOT EXISTS (SELECT 1 FROM dashboard_alerts WHERE source=$4 AND status IN ('active','acknowledged'))`,
			item.Level, item.Title, item.Message, item.Source); err != nil {
			return err
		}
	}
	query := `UPDATE dashboard_alerts SET status='resolved',resolved_at=now() WHERE source LIKE $1 AND status IN ('active','acknowledged')`
	args := []any{fmt.Sprintf("license-%%:%d%%", cfg.ID)}
	if len(activeSources) > 0 {
		holders := make([]string, len(activeSources))
		for i, source := range activeSources {
			args = append(args, source)
			holders[i] = "$" + strconv.Itoa(i+2)
		}
		query += " AND source NOT IN (" + strings.Join(holders, ",") + ")"
	}
	_, err := s.DB.ExecContext(ctx, query, args...)
	return err
}

func (s *Services) appendLicenseTrendSamples(ctx context.Context, resp *LicenseStatusResponse) error {
	rows, err := s.DB.QueryContext(ctx, `
SELECT sample.config_id,server.app_name,sample.feature_name,sample.sample_time,sample.total_count,sample.used_count,sample.free_count,sample.queued_count
FROM license_usage_samples sample
JOIN license_servers server ON server.id=sample.config_id
WHERE sample.sample_time >= now() - interval '365 days'
ORDER BY sample.sample_time ASC
LIMIT 2000`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item LicenseTrendSample
		var sampledAt time.Time
		if err := rows.Scan(&item.ConfigID, &item.AppName, &item.FeatureName, &sampledAt, &item.Total, &item.Used, &item.Free, &item.Queued); err != nil {
			return err
		}
		item.SampleTime = sampledAt.Format(time.RFC3339)
		item.UsageRate = percent(item.Used, item.Total)
		resp.Samples = append(resp.Samples, item)
	}
	return rows.Err()
}

func (s *Services) LicenseStatus(ctx context.Context) (LicenseStatusResponse, error) {
	resp := LicenseStatusResponse{Configs: []LicenseConfig{}, Features: []LicenseFeature{}, Sessions: []LicenseUsageSession{}, Samples: []LicenseTrendSample{}, Alerts: []DashboardAlert{}}
	configs, err := s.ListLicenseConfigs(ctx, "", "")
	if err != nil {
		return resp, err
	}
	resp.Configs = configs
	for _, cfg := range configs {
		if cfg.Enabled {
			resp.Overview.AppCount++
		}
		if cfg.ServiceStatus != "active" && cfg.ServiceStatus != "unmanaged" {
			resp.Overview.AbnormalServer++
		}
		if cfg.LastCollectedAt > resp.Overview.LastUpdated {
			resp.Overview.LastUpdated = cfg.LastCollectedAt
		}
	}
	features, err := s.listLicenseFeatures(ctx, 0)
	if err != nil {
		return resp, err
	}
	highLoadApps := map[int64]bool{}
	for _, feature := range features {
		resp.Features = append(resp.Features, feature)
		resp.Overview.TotalLicenses += feature.Total
		resp.Overview.UsedLicenses += feature.Used
		resp.Overview.FreeLicenses += feature.Free
		resp.Overview.QueuedCount += feature.Queued
		if feature.Total > 0 && feature.Used*100/feature.Total >= 80 {
			highLoadApps[feature.ConfigID] = true
		}
	}
	resp.Overview.HighLoadApps = len(highLoadApps)
	resp.Overview.UsageRate = percent(resp.Overview.UsedLicenses, resp.Overview.TotalLicenses)
	if err := s.appendLicenseTrendSamples(ctx, &resp); err != nil {
		return resp, err
	}
	sessionRows, err := s.DB.QueryContext(ctx, `
SELECT u.config_id,s.app_name,u.feature_name,u.username,u.job_id,u.node_name,u.host_name,u.process_id,u.checkout_count,u.started_at,u.last_seen_at,u.status
FROM license_usage_sessions u JOIN license_servers s ON s.id=u.config_id
WHERE u.status='active'
ORDER BY u.last_seen_at DESC LIMIT 200`)
	if err != nil {
		return resp, err
	}
	defer sessionRows.Close()
	for sessionRows.Next() {
		var item LicenseUsageSession
		var started sql.NullTime
		var seen time.Time
		if err := sessionRows.Scan(&item.ConfigID, &item.AppName, &item.FeatureName, &item.Username, &item.JobID, &item.NodeName, &item.HostName, &item.ProcessID, &item.CheckoutCount, &started, &seen, &item.Status); err != nil {
			return resp, err
		}
		if started.Valid {
			item.StartedAt = started.Time.Format(time.RFC3339)
		}
		item.LastSeenAt = seen.Format(time.RFC3339)
		resp.Sessions = append(resp.Sessions, item)
	}
	alerts, _ := s.MonitoringAlerts(ctx, "active", 100)
	for _, alert := range alerts {
		if strings.HasPrefix(alert.Source, "license-") {
			resp.Alerts = append(resp.Alerts, alert)
		}
	}
	return resp, sessionRows.Err()
}

func (s *Services) LicenseServiceAction(ctx context.Context, id int64, action string) (LicenseConfig, error) {
	cfg, err := s.GetLicenseConfig(ctx, id)
	if err != nil {
		return cfg, err
	}
	action = strings.TrimSpace(action)
	if action != "start" && action != "stop" && action != "restart" {
		return cfg, errors.New("不支持的服务操作")
	}
	if strings.TrimSpace(cfg.ServiceName) == "" {
		return cfg, errors.New("未配置 systemd 服务名称")
	}
	if _, err := runCommandWithTimeout(ctx, cfg.TimeoutSec, "systemctl", action, cfg.ServiceName); err != nil {
		return cfg, err
	}
	status := serviceStatus(ctx, cfg.ServiceName, cfg.TimeoutSec)
	_, err = s.DB.ExecContext(ctx, `UPDATE license_servers SET service_status=$2,updated_at=now() WHERE id=$1`, id, status)
	if err != nil {
		return cfg, err
	}
	return s.GetLicenseConfig(ctx, id)
}
