package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os/exec"
	osuser "os/user"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	templateVariablePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)
	safeSlurmValuePattern   = regexp.MustCompile(`^[A-Za-z0-9._:/+-]+$`)
	linuxUsernamePattern    = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
)

func isTemplateDisplayField(fieldType string) bool {
	switch fieldType {
	case "section", "divider", "hint":
		return true
	default:
		return false
	}
}

type TemplateField struct {
	ID          string           `json:"id"`
	Type        string           `json:"type"`
	Label       string           `json:"label"`
	Variable    string           `json:"variable"`
	Required    bool             `json:"required,omitempty"`
	Default     any              `json:"default,omitempty"`
	Placeholder string           `json:"placeholder,omitempty"`
	Help        string           `json:"help,omitempty"`
	Min         *float64         `json:"min,omitempty"`
	Max         *float64         `json:"max,omitempty"`
	Options     []TemplateOption `json:"options,omitempty"`
}

type TemplateOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type TemplateRuntime struct {
	Desktop        string `json:"desktop,omitempty"`
	VNCBackend     string `json:"vncBackend,omitempty"`
	Resolution     string `json:"resolution,omitempty"`
	LaunchCommand  string `json:"launchCommand,omitempty"`
	Protocol       string `json:"protocol,omitempty"`
	ReadinessPath  string `json:"readinessPath,omitempty"`
	BuiltinVersion int    `json:"builtinVersion,omitempty"`
}

type JobTemplate struct {
	ID             int64           `json:"id"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Category       string          `json:"category"`
	Kind           string          `json:"kind"`
	Status         string          `json:"status"`
	Version        int             `json:"version"`
	FormSchema     []TemplateField `json:"formSchema"`
	ScriptTemplate string          `json:"scriptTemplate,omitempty"`
	Runtime        TemplateRuntime `json:"runtime"`
	CreatedBy      string          `json:"createdBy"`
	UpdatedBy      string          `json:"updatedBy"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	Authorized     bool            `json:"authorized"`
	RequestStatus  string          `json:"requestStatus,omitempty"`
	Grants         []TemplateGrant `json:"grants,omitempty"`
}

type TemplateGrant struct {
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
}

type TemplateAccessRequest struct {
	ID           int64     `json:"id"`
	TemplateID   int64     `json:"templateId"`
	TemplateName string    `json:"templateName"`
	Username     string    `json:"username"`
	Reason       string    `json:"reason"`
	Status       string    `json:"status"`
	ReviewedBy   string    `json:"reviewedBy,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type TemplateRun struct {
	ID           int64          `json:"id"`
	TemplateID   int64          `json:"templateId"`
	TemplateName string         `json:"templateName"`
	Username     string         `json:"username"`
	ProjectID    int64          `json:"projectId,omitempty"`
	ProjectCode  string         `json:"projectCode,omitempty"`
	ProjectName  string         `json:"projectName,omitempty"`
	SlurmAccount string         `json:"slurmAccount,omitempty"`
	Kind         string         `json:"kind"`
	Status       string         `json:"status"`
	SlurmJobID   string         `json:"slurmJobId,omitempty"`
	Values       map[string]any `json:"values"`
	AccessToken  string         `json:"accessToken,omitempty"`
	TargetNode   string         `json:"targetNode,omitempty"`
	TargetPort   int            `json:"targetPort,omitempty"`
	Protocol     string         `json:"protocol,omitempty"`
	AccessURL    string         `json:"accessUrl,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
}

func ensureTemplateUsable(item JobTemplate, user AuthUser) error {
	if item.Status != "published" && !IsTemplateManager(user) {
		return errors.New("模板维护中，请稍后！")
	}
	return nil
}

func ensureTemplateEditable(status string) error {
	if status == "published" {
		return errors.New("请先取消发布后再编辑模板")
	}
	return nil
}

func (s *Services) templateAuthorized(ctx context.Context, id int64, user AuthUser) bool {
	if IsTemplateManager(user) {
		return true
	}
	for _, candidate := range mustListTemplates(s.ListJobTemplates(ctx, user)) {
		if candidate.ID == id {
			return candidate.Authorized
		}
	}
	return false
}

type templateProjectSelection struct {
	ID      int64
	Code    string
	Name    string
	Account string
}

func (s *Services) templateRenderValuesWithProject(ctx context.Context, user AuthUser, values map[string]any) (map[string]any, templateProjectSelection, error) {
	out := make(map[string]any, len(values)+3)
	for key, value := range values {
		out[key] = value
	}
	if s.DB == nil {
		return out, templateProjectSelection{}, nil
	}
	projectID := anyInt64(values["projectId"])
	account := strings.TrimSpace(valueString(values["account"], ""))
	canManageAll := user.Type == "admin" || IsTemplateManager(user)
	if projectID > 0 {
		project, err := s.GetProject(ctx, projectID, user.Username, canManageAll)
		if err != nil {
			return nil, templateProjectSelection{}, errors.New("当前账号无权使用所选项目")
		}
		if strings.TrimSpace(project.SlurmAccount) == "" {
			return nil, templateProjectSelection{}, errors.New("所选项目尚未配置 Slurm Account")
		}
		projectAccount := strings.TrimSpace(project.SlurmAccount)
		if account != "" && account != projectAccount {
			return nil, templateProjectSelection{}, errors.New("提交参数中的 Slurm Account 与所选项目不一致")
		}
		out["projectId"] = project.ID
		out["projectCode"] = project.Code
		out["projectName"] = project.Name
		out["account"] = projectAccount
		return out, templateProjectSelection{ID: project.ID, Code: project.Code, Name: project.Name, Account: projectAccount}, nil
	}
	projects, err := s.ListProjects(ctx, ProjectQuery{Username: user.Username, CanManageAll: canManageAll})
	if err != nil {
		return nil, templateProjectSelection{}, err
	}
	for _, project := range projects.Items {
		projectAccount := strings.TrimSpace(project.SlurmAccount)
		if projectAccount == "" {
			continue
		}
		if account != "" && projectAccount == account || account == "" && project.CurrentUserDefault {
			out["projectId"] = project.ID
			out["projectCode"] = project.Code
			out["projectName"] = project.Name
			out["account"] = projectAccount
			return out, templateProjectSelection{ID: project.ID, Code: project.Code, Name: project.Name, Account: projectAccount}, nil
		}
	}
	if account != "" {
		return nil, templateProjectSelection{}, errors.New("当前账号无权使用该 Slurm Account")
	}
	return out, templateProjectSelection{}, nil
}

func anyInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n
	default:
		return 0
	}
}

func nullInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func (s *Services) PreviewTemplate(ctx context.Context, id int64, user AuthUser, values map[string]any) (string, error) {
	item, err := s.GetJobTemplate(ctx, id, AuthUser{Username: user.Username, Type: "admin", Role: "cluster_admin"})
	if err != nil {
		return "", err
	}
	if err := ensureTemplateUsable(item, user); err != nil {
		return "", err
	}
	if !s.templateAuthorized(ctx, id, user) {
		return "", errors.New("当前账号未获得模板使用授权")
	}
	renderValues, _, err := s.templateRenderValuesWithProject(ctx, user, values)
	if err != nil {
		return "", err
	}
	return RenderTemplateScript(item, renderValues)
}

func (s *Services) SubmitTemplate(ctx context.Context, id int64, user AuthUser, values map[string]any) (TemplateRun, error) {
	item, err := s.GetJobTemplate(ctx, id, AuthUser{Username: user.Username, Type: "admin", Role: "cluster_admin"})
	if err != nil {
		return TemplateRun{}, err
	}
	if err := ensureTemplateUsable(item, user); err != nil {
		return TemplateRun{}, err
	}
	if !s.templateAuthorized(ctx, id, user) {
		return TemplateRun{}, errors.New("当前账号未获得模板使用授权")
	}
	submitValues, project, err := s.templateRenderValuesWithProject(ctx, user, values)
	if err != nil {
		return TemplateRun{}, err
	}
	submitUser, err := resolveTemplateSubmitUser(user, item.Kind, func(username string) error {
		return lookupLinuxSubmitUser(ctx, username)
	})
	if err != nil {
		return TemplateRun{}, err
	}
	token, err := randomHex(24)
	if err != nil {
		return TemplateRun{}, err
	}
	renderValues := make(map[string]any, len(submitValues)+3)
	for key, value := range submitValues {
		renderValues[key] = value
	}
	renderValues["SIMPLEHPC_RUN_TOKEN"] = token
	renderValues["SIMPLEHPC_ACCESS_TOKEN"] = token[:16]
	renderValues["SIMPLEHPC_CALLBACK_URL"] = s.Config.PublicURL + "/api/v1/job-template-runs/" + token + "/register"
	renderValues["SIMPLEHPC_SUBMIT_USER"] = submitUser
	script, err := RenderTemplateScript(item, renderValues)
	if err != nil {
		return TemplateRun{}, err
	}
	raw, _ := json.Marshal(submitValues)
	run := TemplateRun{TemplateID: id, TemplateName: item.Name, Username: user.Username, ProjectID: project.ID, ProjectCode: project.Code, ProjectName: project.Name, SlurmAccount: project.Account, Kind: item.Kind, Status: "submitting", Values: submitValues, AccessToken: token}
	err = s.DB.QueryRowContext(ctx, `INSERT INTO job_template_runs(template_id,template_version,template_name,username,project_id,project_code,project_name,slurm_account,kind,status,submitted_values,rendered_script,access_token)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'submitting',$10,$11,$12) RETURNING id,created_at`,
		id, item.Version, item.Name, user.Username, nullInt64(project.ID), project.Code, project.Name, project.Account, item.Kind, raw, script, token).Scan(&run.ID, &run.CreatedAt)
	if err != nil {
		return TemplateRun{}, err
	}
	jobID, err := s.Slurm.SubmitScript(ctx, script, submitUser)
	if err != nil {
		_, _ = s.DB.ExecContext(ctx, `UPDATE job_template_runs SET status='submit_failed',error_message=$2,updated_at=now() WHERE id=$1`, run.ID, err.Error())
		return TemplateRun{}, err
	}
	run.SlurmJobID, run.Status = jobID, "submitted"
	_, err = s.DB.ExecContext(ctx, `UPDATE job_template_runs SET status='submitted',slurm_job_id=$2,updated_at=now() WHERE id=$1`, run.ID, jobID)
	if err == nil {
		if project.ID > 0 {
			_, _ = s.DB.ExecContext(ctx, `
INSERT INTO project_job_links(project_id,job_id,job_name,username,account,state,partition,linked_by)
VALUES($1,$2,$3,$4,$5,'SUBMITTED',$6,$7)
ON CONFLICT(project_id,job_id) DO UPDATE SET job_name=EXCLUDED.job_name,username=EXCLUDED.username,account=EXCLUDED.account,state=EXCLUDED.state,partition=EXCLUDED.partition,linked_by=EXCLUDED.linked_by,linked_at=now()`,
				project.ID, jobID, valueString(renderValues["jobName"], item.Name), user.Username, project.Account, valueString(renderValues["partition"], ""), user.Username)
			_ = s.recordProjectActivity(ctx, project.ID, user.Username, "job.submit", "job", jobID, "通过作业模板提交项目作业", map[string]any{"template": item.Name, "account": project.Account})
		}
		if syncErr := s.SyncCurrentSlurmJobs(ctx); syncErr != nil {
			log.Printf("sync submitted template job %s: %v", jobID, syncErr)
		}
	}
	return run, err
}

func resolveTemplateSubmitUser(user AuthUser, kind string, lookup func(string) error) (string, error) {
	username := strings.TrimSpace(user.Username)
	if kind == "novnc" {
		if !linuxUsernamePattern.MatchString(username) || lookup(username) != nil {
			return "", errors.New("VNC 桌面必须使用同名 Linux/LDAP 用户提交；当前平台账号未映射 Linux 用户")
		}
		return username, nil
	}
	if user.Type == "admin" {
		return "root", nil
	}
	return username, nil
}

func lookupLinuxSubmitUser(ctx context.Context, username string) error {
	username = strings.TrimSpace(username)
	if !linuxUsernamePattern.MatchString(username) {
		return fmt.Errorf("invalid linux username %q", username)
	}
	if _, err := exec.CommandContext(ctx, "getent", "passwd", username).Output(); err == nil {
		return nil
	}
	_, err := osuser.Lookup(username)
	return err
}

func (s *Services) TemplateRunScript(ctx context.Context, jobID string) (string, error) {
	var script string
	err := s.DB.QueryRowContext(ctx, `
SELECT rendered_script
FROM job_template_runs
WHERE slurm_job_id = $1
ORDER BY id DESC
LIMIT 1`, strings.TrimSpace(jobID)).Scan(&script)
	return script, err
}

func mustListTemplates(items []JobTemplate, err error) []JobTemplate {
	if err != nil {
		return nil
	}
	return items
}

func ValidateTemplate(t JobTemplate) error {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" || len(t.Name) > 120 {
		return errors.New("模板名称不能为空且不能超过 120 个字符")
	}
	switch t.Kind {
	case "batch", "novnc", "webapp":
	default:
		return errors.New("模板类型必须是 batch、novnc 或 webapp")
	}
	if strings.TrimSpace(t.ScriptTemplate) == "" {
		return errors.New("后端执行脚本不能为空")
	}
	seenID, seenVariable := map[string]bool{}, map[string]bool{}
	for _, field := range t.FormSchema {
		if strings.TrimSpace(field.ID) == "" || seenID[field.ID] {
			return errors.New("组件 ID 不能为空且必须唯一")
		}
		seenID[field.ID] = true
		if isTemplateDisplayField(field.Type) {
			continue
		}
		if !templateVariablePattern.MatchString(field.Variable) || seenVariable[field.Variable] {
			return fmt.Errorf("环境变量 %q 不合法或重复", field.Variable)
		}
		if field.Min != nil && field.Max != nil && *field.Min > *field.Max {
			return fmt.Errorf("字段 %s 的最小值不能大于最大值", field.Label)
		}
		seenVariable[field.Variable] = true
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func RenderTemplateScript(t JobTemplate, values map[string]any) (string, error) {
	if err := ValidateTemplate(t); err != nil {
		return "", err
	}
	var body strings.Builder
	body.WriteString("#!/bin/bash\n")
	defaultJobName := "simplehpc-job"
	if t.ID > 0 {
		defaultJobName = "simplehpc-template-" + strconv.FormatInt(t.ID, 10)
	}
	directives := []struct{ key, fallback string }{
		{"jobName", defaultJobName}, {"partition", ""}, {"account", ""}, {"nodes", "1"},
		{"cpus", "1"}, {"gpus", "0"}, {"walltime", defaultWalltime(t.Kind)},
	}
	flags := map[string]string{"jobName": "--job-name", "partition": "--partition", "account": "--account", "nodes": "--nodes", "cpus": "--cpus-per-task", "gpus": "--gpus", "walltime": "--time"}
	for _, item := range directives {
		value := valueString(values[item.key], item.fallback)
		if value == "" || (item.key == "gpus" && value == "0") {
			continue
		}
		if !safeSlurmValuePattern.MatchString(value) {
			return "", fmt.Errorf("%s 包含不安全字符", item.key)
		}
		body.WriteString("#SBATCH " + flags[item.key] + "=" + value + "\n")
	}
	body.WriteString("#SBATCH --output=slurm-%j.out\n")
	body.WriteString("#SBATCH --error=slurm-%j.err\n")
	if workdir := valueString(values["workdir"], ""); workdir != "" {
		if !strings.HasPrefix(workdir, "/") || strings.Contains(workdir, "\n") {
			return "", errors.New("工作目录必须是绝对路径")
		}
		body.WriteString("#SBATCH --chdir=" + workdir + "\n")
	}
	body.WriteString("\nset -euo pipefail\n")
	for _, key := range []string{"SIMPLEHPC_RUN_TOKEN", "SIMPLEHPC_ACCESS_TOKEN", "SIMPLEHPC_CALLBACK_URL", "SIMPLEHPC_SUBMIT_USER"} {
		if value := valueString(values[key], ""); value != "" {
			body.WriteString("export " + key + "=" + shellQuote(value) + "\n")
		}
	}
	for _, field := range t.FormSchema {
		if isTemplateDisplayField(field.Type) {
			continue
		}
		value, ok := values[field.ID]
		if !ok || value == nil || valueString(value, "") == "" {
			value = field.Default
		}
		text := valueString(value, "")
		if field.Required && text == "" {
			return "", fmt.Errorf("%s 为必填项", field.Label)
		}
		if (field.Type == "number" || field.Type == "cpu" || field.Type == "gpu" || field.Type == "slider") && text != "" {
			number, err := strconv.ParseFloat(text, 64)
			if err != nil {
				return "", fmt.Errorf("%s 必须是数字", field.Label)
			}
			if field.Min != nil && number < *field.Min || field.Max != nil && number > *field.Max {
				return "", fmt.Errorf("%s 超出允许范围", field.Label)
			}
		}
		body.WriteString("export " + field.Variable + "=" + shellQuote(text) + "\n")
	}
	body.WriteString("\n" + strings.TrimSpace(t.ScriptTemplate) + "\n")
	return body.String(), nil
}

func defaultWalltime(kind string) string {
	if kind == "novnc" {
		return "24:00:00"
	}
	return ""
}

func valueString(value any, fallback string) string {
	if value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return fallback
		}
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(typed)
	}
}

func IsTemplateManager(user AuthUser) bool {
	return user.Type == "admin" && (user.Role == "cluster_admin" || user.Role == "config_admin")
}

func (s *Services) ListJobTemplates(ctx context.Context, user AuthUser) ([]JobTemplate, error) {
	if s.DB == nil {
		return nil, errNotConfigured("postgres")
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT t.id,t.name,t.description,t.category,t.kind,t.status,t.version,t.form_schema,t.script_template,t.runtime_config,
 t.created_by,t.updated_by,t.created_at,t.updated_at,
 EXISTS(SELECT 1 FROM job_template_grants g WHERE g.template_id=t.id AND (
   g.target_type='all' OR (g.target_type='user' AND g.target_id=$1) OR
   (g.target_type='team' AND g.target_id IN (SELECT COALESCE(tm.group_name,'') FROM platform_users u LEFT JOIN teams tm ON tm.id=u.team_id WHERE u.username=$1))
 )) AS authorized,
 COALESCE((SELECT r.status FROM job_template_access_requests r WHERE r.template_id=t.id AND r.username=$1 ORDER BY r.created_at DESC LIMIT 1),'')
FROM job_templates t
WHERE ($2 OR t.status='published')
ORDER BY t.updated_at DESC,t.id DESC`, user.Username, IsTemplateManager(user))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []JobTemplate{}
	for rows.Next() {
		item, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		if IsTemplateManager(user) {
			item.Authorized = true
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) GetJobTemplate(ctx context.Context, id int64, user AuthUser) (JobTemplate, error) {
	row := s.DB.QueryRowContext(ctx, `
SELECT t.id,t.name,t.description,t.category,t.kind,t.status,t.version,t.form_schema,t.script_template,t.runtime_config,
 t.created_by,t.updated_by,t.created_at,t.updated_at,true,'' FROM job_templates t WHERE t.id=$1`, id)
	item, err := scanTemplate(row)
	if err != nil {
		return JobTemplate{}, err
	}
	if item.Status != "published" && !IsTemplateManager(user) {
		return JobTemplate{}, sql.ErrNoRows
	}
	grants, _ := s.TemplateGrants(ctx, id)
	item.Grants = grants
	if !IsTemplateManager(user) {
		item.ScriptTemplate = ""
	}
	return item, nil
}

type rowScanner interface{ Scan(...any) error }

func scanTemplate(row rowScanner) (JobTemplate, error) {
	var item JobTemplate
	var schemaRaw, runtimeRaw []byte
	err := row.Scan(&item.ID, &item.Name, &item.Description, &item.Category, &item.Kind, &item.Status, &item.Version,
		&schemaRaw, &item.ScriptTemplate, &runtimeRaw, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt, &item.Authorized, &item.RequestStatus)
	if err != nil {
		return item, err
	}
	_ = json.Unmarshal(schemaRaw, &item.FormSchema)
	_ = json.Unmarshal(runtimeRaw, &item.Runtime)
	return item, nil
}

func (s *Services) SaveJobTemplate(ctx context.Context, item JobTemplate, username string) (JobTemplate, error) {
	if err := ValidateTemplate(item); err != nil {
		return JobTemplate{}, err
	}
	schema, _ := json.Marshal(item.FormSchema)
	runtime, _ := json.Marshal(item.Runtime)
	if item.Status == "" {
		item.Status = "draft"
	}
	if item.ID == 0 {
		err := s.DB.QueryRowContext(ctx, `INSERT INTO job_templates(name,description,category,kind,status,form_schema,script_template,runtime_config,created_by,updated_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$9) RETURNING id`, item.Name, item.Description, item.Category, item.Kind, item.Status, schema, item.ScriptTemplate, runtime, username).Scan(&item.ID)
		if err != nil {
			return JobTemplate{}, err
		}
	} else {
		var currentStatus string
		if err := s.DB.QueryRowContext(ctx, `SELECT status FROM job_templates WHERE id=$1`, item.ID).Scan(&currentStatus); err != nil {
			return JobTemplate{}, err
		}
		if err := ensureTemplateEditable(currentStatus); err != nil {
			return JobTemplate{}, err
		}
		_, err := s.DB.ExecContext(ctx, `UPDATE job_templates SET name=$2,description=$3,category=$4,kind=$5,status=$6,form_schema=$7,script_template=$8,runtime_config=$9,updated_by=$10,version=version+1,updated_at=now() WHERE id=$1`,
			item.ID, item.Name, item.Description, item.Category, item.Kind, item.Status, schema, item.ScriptTemplate, runtime, username)
		if err != nil {
			return JobTemplate{}, err
		}
	}
	return s.GetJobTemplate(ctx, item.ID, AuthUser{Username: username, Type: "admin", Role: "cluster_admin"})
}

func (s *Services) DeleteJobTemplate(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM job_templates WHERE id=$1`, id)
	return err
}
func (s *Services) SetTemplateStatus(ctx context.Context, id int64, status, username string) error {
	if status != "draft" && status != "published" {
		return errors.New("无效模板状态")
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE job_templates SET status=$2,updated_by=$3,version=version+1,updated_at=now() WHERE id=$1`, id, status, username)
	return err
}
func (s *Services) TemplateGrants(ctx context.Context, id int64) ([]TemplateGrant, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT target_type,target_id FROM job_template_grants WHERE template_id=$1 ORDER BY target_type,target_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TemplateGrant{}
	for rows.Next() {
		var g TemplateGrant
		if err := rows.Scan(&g.TargetType, &g.TargetID); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
func (s *Services) SetTemplateGrants(ctx context.Context, id int64, grants []TemplateGrant, by string) error {
	var status string
	if err := s.DB.QueryRowContext(ctx, `SELECT status FROM job_templates WHERE id=$1`, id).Scan(&status); err != nil {
		return err
	}
	if err := ensureTemplateEditable(status); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM job_template_grants WHERE template_id=$1`, id); err != nil {
		return err
	}
	for _, g := range grants {
		if g.TargetType != "all" && g.TargetType != "user" && g.TargetType != "team" {
			return errors.New("无效授权类型")
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO job_template_grants(template_id,target_type,target_id,granted_by) VALUES($1,$2,$3,$4)`, id, g.TargetType, g.TargetID, by); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Services) RequestTemplateAccess(ctx context.Context, id int64, user, reason string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO job_template_access_requests(template_id,username,reason,status) VALUES($1,$2,$3,'pending')`, id, user, strings.TrimSpace(reason))
	return err
}
func (s *Services) ListTemplateRequests(ctx context.Context) ([]TemplateAccessRequest, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT r.id,r.template_id,t.name,r.username,r.reason,r.status,r.reviewed_by,r.created_at FROM job_template_access_requests r JOIN job_templates t ON t.id=r.template_id ORDER BY r.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TemplateAccessRequest{}
	for rows.Next() {
		var r TemplateAccessRequest
		if err := rows.Scan(&r.ID, &r.TemplateID, &r.TemplateName, &r.Username, &r.Reason, &r.Status, &r.ReviewedBy, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Services) ReviewTemplateRequest(ctx context.Context, id int64, approve bool, reviewer string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	status := "rejected"
	if approve {
		status = "approved"
	}
	var templateID int64
	var username string
	if err = tx.QueryRowContext(ctx, `UPDATE job_template_access_requests SET status=$2,reviewed_by=$3,reviewed_at=now() WHERE id=$1 AND status='pending' RETURNING template_id,username`, id, status, reviewer).Scan(&templateID, &username); err != nil {
		return err
	}
	if approve {
		_, err = tx.ExecContext(ctx, `INSERT INTO job_template_grants(template_id,target_type,target_id,granted_by) VALUES($1,'user',$2,$3) ON CONFLICT DO NOTHING`, templateID, username, reviewer)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Services) RegisterTemplateEndpoint(ctx context.Context, token, node string, port int, protocol string) (TemplateRun, error) {
	token = strings.TrimSpace(token)
	node = strings.TrimSpace(node)
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if len(token) < 32 {
		return TemplateRun{}, errors.New("无效运行令牌")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,252}$`).MatchString(node) {
		return TemplateRun{}, errors.New("无效计算节点地址")
	}
	if port < 1024 || port > 65535 {
		return TemplateRun{}, errors.New("无效服务端口")
	}
	if protocol != "http" && protocol != "vnc" {
		return TemplateRun{}, errors.New("不支持的交互协议")
	}
	var run TemplateRun
	err := s.DB.QueryRowContext(ctx, `
UPDATE job_template_runs
SET target_node=$2,target_port=$3,protocol=$4,status='ready',updated_at=now()
WHERE access_token=$1
RETURNING id,template_id,template_name,username,COALESCE(project_id,0),COALESCE(project_code,''),COALESCE(project_name,''),COALESCE(slurm_account,''),kind,status,COALESCE(slurm_job_id,''),target_node,target_port,protocol,created_at`,
		token, node, port, protocol).Scan(&run.ID, &run.TemplateID, &run.TemplateName, &run.Username,
		&run.ProjectID, &run.ProjectCode, &run.ProjectName, &run.SlurmAccount, &run.Kind,
		&run.Status, &run.SlurmJobID, &run.TargetNode, &run.TargetPort, &run.Protocol, &run.CreatedAt)
	if err != nil {
		return TemplateRun{}, err
	}
	run.AccessToken = token
	run.AccessURL = templateRunAccessURL(run)
	return run, nil
}

func (s *Services) TemplateRunByToken(ctx context.Context, token string) (TemplateRun, error) {
	var run TemplateRun
	err := s.DB.QueryRowContext(ctx, `
SELECT id,template_id,template_name,username,kind,status,COALESCE(slurm_job_id,''),
 COALESCE(project_id,0),COALESCE(project_code,''),COALESCE(project_name,''),COALESCE(slurm_account,''),
 COALESCE(target_node,''),COALESCE(target_port,0),COALESCE(protocol,''),created_at
FROM job_template_runs WHERE access_token=$1`, strings.TrimSpace(token)).
		Scan(&run.ID, &run.TemplateID, &run.TemplateName, &run.Username, &run.Kind, &run.Status,
			&run.SlurmJobID, &run.ProjectID, &run.ProjectCode, &run.ProjectName, &run.SlurmAccount, &run.TargetNode, &run.TargetPort, &run.Protocol, &run.CreatedAt)
	if err != nil {
		return TemplateRun{}, err
	}
	run.AccessToken = strings.TrimSpace(token)
	run.AccessURL = templateRunAccessURL(run)
	return run, nil
}

func (s *Services) ListTemplateRuns(ctx context.Context, user AuthUser) ([]TemplateRun, error) {
	query := `
SELECT id,template_id,template_name,username,kind,status,COALESCE(slurm_job_id,''),
 COALESCE(project_id,0),COALESCE(project_code,''),COALESCE(project_name,''),COALESCE(slurm_account,''),
 COALESCE(target_node,''),COALESCE(target_port,0),COALESCE(protocol,''),access_token,submitted_values,created_at
FROM job_template_runs`
	args := []any{}
	if !IsTemplateManager(user) {
		query += ` WHERE username=$1`
		args = append(args, user.Username)
	}
	query += ` ORDER BY created_at DESC LIMIT 500`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []TemplateRun{}
	for rows.Next() {
		var run TemplateRun
		var valuesRaw []byte
		if err := rows.Scan(&run.ID, &run.TemplateID, &run.TemplateName, &run.Username, &run.Kind,
			&run.Status, &run.SlurmJobID, &run.ProjectID, &run.ProjectCode, &run.ProjectName, &run.SlurmAccount, &run.TargetNode, &run.TargetPort, &run.Protocol,
			&run.AccessToken, &valuesRaw, &run.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(valuesRaw, &run.Values)
		if run.Status == "ready" {
			run.AccessURL = templateRunAccessURL(run)
		}
		run.AccessToken = ""
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *Services) ListTemplateRunsByPermission(ctx context.Context, authz PermissionContext) ([]TemplateRun, error) {
	query := `
SELECT id,template_id,template_name,username,kind,status,COALESCE(slurm_job_id,''),
 COALESCE(project_id,0),COALESCE(project_code,''),COALESCE(project_name,''),COALESCE(slurm_account,''),
 COALESCE(target_node,''),COALESCE(target_port,0),COALESCE(protocol,''),access_token,submitted_values,created_at
FROM job_template_runs`
	args := []any{}
	switch {
	case authz.IsClusterAdmin || authz.HasScope("vnc_sessions", ScopeGlobal):
	case authz.HasScope("vnc_sessions", ScopeUnit) && len(authz.UnitIDs) > 0:
		query += ` WHERE username IN (
SELECT username FROM platform_users WHERE unit_id::text=ANY($1::text[]))`
		args = append(args, authz.UnitIDs)
	case authz.HasScope("vnc_sessions", ScopeTeam) && len(authz.TeamIDs) > 0:
		query += ` WHERE username IN (
SELECT username FROM platform_users WHERE team_id::text=ANY($1::text[]))`
		args = append(args, authz.TeamIDs)
	case authz.HasScope("vnc_sessions", ScopeSelf):
		query += ` WHERE username=$1`
		args = append(args, authz.Username)
	default:
		query += ` WHERE 1=0`
	}
	query += ` ORDER BY created_at DESC LIMIT 500`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []TemplateRun{}
	for rows.Next() {
		var run TemplateRun
		var valuesRaw []byte
		if err := rows.Scan(&run.ID, &run.TemplateID, &run.TemplateName, &run.Username, &run.Kind,
			&run.Status, &run.SlurmJobID, &run.ProjectID, &run.ProjectCode, &run.ProjectName, &run.SlurmAccount, &run.TargetNode, &run.TargetPort, &run.Protocol,
			&run.AccessToken, &valuesRaw, &run.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(valuesRaw, &run.Values)
		if run.Status == "ready" {
			run.AccessURL = templateRunAccessURL(run)
		}
		run.AccessToken = ""
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func templateRunAccessURL(run TemplateRun) string {
	token := strings.TrimSpace(run.AccessToken)
	base := "/api/v1/job-template-gateway/" + token + "/"
	if run.Protocol != "vnc" {
		return base
	}
	password := token
	if len(password) > 8 {
		password = password[:8]
	}
	query := url.Values{}
	query.Set("autoconnect", "1")
	query.Set("resize", "remote")
	query.Set("path", "api/v1/job-template-gateway/"+token+"/websockify")
	query.Set("password", password)
	return base + "vnc.html?" + query.Encode()
}

func SortTemplateFields(fields []TemplateField) {
	sort.SliceStable(fields, func(i, j int) bool { return fields[i].ID < fields[j].ID })
}
