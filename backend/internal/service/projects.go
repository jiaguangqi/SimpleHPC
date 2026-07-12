package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	slurmintegration "simplehpc/backend/internal/integrations/slurm"
)

var (
	projectCodePattern         = regexp.MustCompile(`^[a-z][a-z0-9_-]{2,47}$`)
	projectSlurmAccountPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
)

type ProjectRecord struct {
	ID                  int64              `json:"id"`
	Code                string             `json:"code"`
	Name                string             `json:"name"`
	Summary             string             `json:"summary"`
	OwnerUsername       string             `json:"ownerUsername"`
	OwnerDisplayName    string             `json:"ownerDisplayName"`
	UnitID              int64              `json:"unitId,omitempty"`
	UnitName            string             `json:"unitName,omitempty"`
	TeamID              int64              `json:"teamId,omitempty"`
	TeamName            string             `json:"teamName,omitempty"`
	SlurmAccount        string             `json:"slurmAccount"`
	SlurmParentAccount  string             `json:"slurmParentAccount,omitempty"`
	SlurmQOS            string             `json:"slurmQos,omitempty"`
	SlurmSyncEnabled    bool               `json:"slurmSyncEnabled"`
	SlurmSyncStatus     string             `json:"slurmSyncStatus"`
	SlurmSyncMessage    string             `json:"slurmSyncMessage,omitempty"`
	SlurmSyncedAt       string             `json:"slurmSyncedAt,omitempty"`
	Status              string             `json:"status"`
	Priority            string             `json:"priority"`
	StartDate           string             `json:"startDate,omitempty"`
	EndDate             string             `json:"endDate,omitempty"`
	StorageQuotaGB      int                `json:"storageQuotaGb"`
	ComputeQuotaHours   int                `json:"computeQuotaHours"`
	LicenseBudgetPoints int                `json:"licenseBudgetPoints"`
	Tags                []string           `json:"tags"`
	Metrics             ProjectMetrics     `json:"metrics"`
	CurrentUserRole     string             `json:"currentUserRole,omitempty"`
	CurrentUserAccess   string             `json:"currentUserAccess,omitempty"`
	CurrentUserDefault  bool               `json:"currentUserDefaultProject,omitempty"`
	CanManage           bool               `json:"canManage"`
	CreatedBy           string             `json:"createdBy,omitempty"`
	UpdatedBy           string             `json:"updatedBy,omitempty"`
	CreatedAt           string             `json:"createdAt,omitempty"`
	UpdatedAt           string             `json:"updatedAt,omitempty"`
	Members             []ProjectMember    `json:"members,omitempty"`
	Tasks               []ProjectTask      `json:"tasks,omitempty"`
	Directories         []ProjectDirectory `json:"directories,omitempty"`
	JobLinks            []ProjectJobLink   `json:"jobLinks,omitempty"`
	Activities          []ProjectActivity  `json:"activities,omitempty"`
}

type ProjectMetrics struct {
	MemberCount       int     `json:"memberCount"`
	TaskCount         int     `json:"taskCount"`
	DoneTaskCount     int     `json:"doneTaskCount"`
	DirectoryCount    int     `json:"directoryCount"`
	JobCount          int     `json:"jobCount"`
	RunningJobCount   int     `json:"runningJobCount"`
	PendingJobCount   int     `json:"pendingJobCount"`
	CompletedJobCount int     `json:"completedJobCount"`
	FailedJobCount    int     `json:"failedJobCount"`
	ElapsedHours      float64 `json:"elapsedHours"`
	AllocatedCPUHours float64 `json:"allocatedCpuHours"`
	AllocatedGPUHours float64 `json:"allocatedGpuHours"`
	LicenseUsed       int     `json:"licenseUsed"`
	ProgressPercent   float64 `json:"progressPercent"`
}

type ProjectSummary struct {
	Total          int `json:"total"`
	Active         int `json:"active"`
	Paused         int `json:"paused"`
	Completed      int `json:"completed"`
	MyProjects     int `json:"myProjects"`
	RunningJobs    int `json:"runningJobs"`
	OpenTasks      int `json:"openTasks"`
	StorageQuotaGB int `json:"storageQuotaGb"`
}

type ProjectMember struct {
	ID             int64  `json:"id"`
	ProjectID      int64  `json:"projectId"`
	Username       string `json:"username"`
	DisplayName    string `json:"displayName"`
	Role           string `json:"role"`
	Permission     string `json:"permission"`
	Status         string `json:"status"`
	DefaultProject bool   `json:"defaultProject"`
	JoinedAt       string `json:"joinedAt,omitempty"`
	CreatedBy      string `json:"createdBy,omitempty"`
}

type ProjectTask struct {
	ID               int64   `json:"id"`
	ProjectID        int64   `json:"projectId"`
	Title            string  `json:"title"`
	AssigneeUsername string  `json:"assigneeUsername"`
	Status           string  `json:"status"`
	Priority         string  `json:"priority"`
	DueDate          string  `json:"dueDate,omitempty"`
	Description      string  `json:"description"`
	UpstreamTaskIDs  []int64 `json:"upstreamTaskIds"`
	CreatedBy        string  `json:"createdBy,omitempty"`
	UpdatedBy        string  `json:"updatedBy,omitempty"`
	CreatedAt        string  `json:"createdAt,omitempty"`
	UpdatedAt        string  `json:"updatedAt,omitempty"`
}

type ProjectDirectory struct {
	ID         int64  `json:"id"`
	ProjectID  int64  `json:"projectId"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Permission string `json:"permission"`
	Status     string `json:"status"`
	CreatedBy  string `json:"createdBy,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty"`
}

type ProjectJobLink struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"projectId"`
	JobID     string `json:"jobId"`
	TaskID    int64  `json:"taskId,omitempty"`
	JobName   string `json:"jobName"`
	Username  string `json:"username"`
	Account   string `json:"account"`
	State     string `json:"state"`
	Partition string `json:"partition"`
	LinkedBy  string `json:"linkedBy,omitempty"`
	LinkedAt  string `json:"linkedAt,omitempty"`
}

type ProjectActivity struct {
	ID         int64          `json:"id"`
	ProjectID  int64          `json:"projectId"`
	Actor      string         `json:"actor"`
	Action     string         `json:"action"`
	TargetType string         `json:"targetType"`
	TargetID   string         `json:"targetId"`
	Message    string         `json:"message"`
	Detail     map[string]any `json:"detail"`
	CreatedAt  string         `json:"createdAt"`
}

type ProjectInput struct {
	Code                string   `json:"code"`
	Name                string   `json:"name"`
	Summary             string   `json:"summary"`
	OwnerUsername       string   `json:"ownerUsername"`
	UnitID              int64    `json:"unitId"`
	TeamID              int64    `json:"teamId"`
	SlurmAccount        string   `json:"slurmAccount"`
	SlurmParentAccount  string   `json:"slurmParentAccount"`
	SlurmQOS            string   `json:"slurmQos"`
	SlurmSyncEnabled    bool     `json:"slurmSyncEnabled"`
	Status              string   `json:"status"`
	Priority            string   `json:"priority"`
	StartDate           string   `json:"startDate"`
	EndDate             string   `json:"endDate"`
	StorageQuotaGB      int      `json:"storageQuotaGb"`
	ComputeQuotaHours   int      `json:"computeQuotaHours"`
	LicenseBudgetPoints int      `json:"licenseBudgetPoints"`
	Tags                []string `json:"tags"`
}

type ProjectMemberInput struct {
	Username       string `json:"username"`
	DisplayName    string `json:"displayName"`
	Role           string `json:"role"`
	Permission     string `json:"permission"`
	Status         string `json:"status"`
	DefaultProject bool   `json:"defaultProject"`
}

type ProjectTaskInput struct {
	ID               int64   `json:"id"`
	Title            string  `json:"title"`
	AssigneeUsername string  `json:"assigneeUsername"`
	Status           string  `json:"status"`
	Priority         string  `json:"priority"`
	DueDate          string  `json:"dueDate"`
	Description      string  `json:"description"`
	UpstreamTaskIDs  []int64 `json:"upstreamTaskIds"`
}

type ProjectDirectoryInput struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Permission string `json:"permission"`
	Status     string `json:"status"`
}

type ProjectJobLinkInput struct {
	JobID     string `json:"jobId"`
	TaskID    int64  `json:"taskId"`
	JobName   string `json:"jobName"`
	Username  string `json:"username"`
	Account   string `json:"account"`
	State     string `json:"state"`
	Partition string `json:"partition"`
}

type ProjectQuery struct {
	Keyword      string
	Status       string
	Username     string
	CanManageAll bool
}

type ProjectListResponse struct {
	Items   []ProjectRecord `json:"items"`
	Count   int             `json:"count"`
	Summary ProjectSummary  `json:"summary"`
}

func normalizeProjectInput(input ProjectInput) ProjectInput {
	input.Code = strings.ToLower(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	input.Summary = strings.TrimSpace(input.Summary)
	input.OwnerUsername = strings.TrimSpace(input.OwnerUsername)
	input.SlurmAccount = strings.TrimSpace(input.SlurmAccount)
	input.SlurmParentAccount = strings.TrimSpace(input.SlurmParentAccount)
	input.SlurmQOS = strings.TrimSpace(input.SlurmQOS)
	input.Status = strings.TrimSpace(input.Status)
	input.Priority = strings.TrimSpace(input.Priority)
	input.StartDate = strings.TrimSpace(input.StartDate)
	input.EndDate = strings.TrimSpace(input.EndDate)
	if input.Status == "" {
		input.Status = "planning"
	}
	if input.SlurmAccount == "" {
		input.SlurmAccount = input.Code
	}
	if input.Priority == "" {
		input.Priority = "normal"
	}
	tags := []string{}
	seen := map[string]bool{}
	for _, tag := range input.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	input.Tags = tags
	return input
}

func validateProjectInput(input ProjectInput, creating bool) error {
	if input.Name == "" {
		return errors.New("项目名称不能为空")
	}
	if len([]rune(input.Name)) > 80 {
		return errors.New("项目名称不能超过 80 个字符")
	}
	if input.Code == "" && creating {
		return errors.New("项目编码不能为空")
	}
	if input.Code != "" && !projectCodePattern.MatchString(input.Code) {
		return errors.New("项目编码只能使用小写字母、数字、下划线和中划线，必须以小写字母开头，长度 3-48 位")
	}
	if input.OwnerUsername == "" {
		return errors.New("项目负责人不能为空")
	}
	if err := validateLinuxAccountName(input.OwnerUsername, "项目负责人账号"); err != nil {
		return err
	}
	if input.SlurmAccount == "" {
		return errors.New("Slurm Account 不能为空")
	}
	if !projectSlurmAccountPattern.MatchString(input.SlurmAccount) {
		return errors.New("Slurm Account 只能使用字母、数字、点、下划线和中划线，长度 1-64 位")
	}
	if input.SlurmParentAccount != "" && !projectSlurmAccountPattern.MatchString(input.SlurmParentAccount) {
		return errors.New("父级 Slurm Account 格式无效")
	}
	if input.SlurmQOS != "" && !projectSlurmAccountPattern.MatchString(input.SlurmQOS) {
		return errors.New("Slurm QOS 格式无效")
	}
	if !validProjectStatus(input.Status) {
		return errors.New("项目状态无效")
	}
	if !validProjectPriority(input.Priority) {
		return errors.New("项目优先级无效")
	}
	if input.StorageQuotaGB < 0 || input.ComputeQuotaHours < 0 || input.LicenseBudgetPoints < 0 {
		return errors.New("资源额度不能为负数")
	}
	if _, err := parseOptionalDate(input.StartDate); err != nil {
		return fmt.Errorf("开始日期格式无效")
	}
	if _, err := parseOptionalDate(input.EndDate); err != nil {
		return fmt.Errorf("结束日期格式无效")
	}
	return nil
}

func validProjectStatus(value string) bool {
	switch value {
	case "planning", "active", "paused", "completed", "archived":
		return true
	default:
		return false
	}
}

func validProjectPriority(value string) bool {
	switch value {
	case "low", "normal", "high", "critical":
		return true
	default:
		return false
	}
}

func validProjectRole(value string) bool {
	switch value {
	case "owner", "manager", "compute_member", "data_member", "viewer", "external":
		return true
	default:
		return false
	}
}

func validProjectPermission(value string) bool {
	switch value {
	case "read", "work", "manage":
		return true
	default:
		return false
	}
}

func validTaskStatus(value string) bool {
	switch value {
	case "todo", "running", "blocked", "done", "cancelled":
		return true
	default:
		return false
	}
}

func parseOptionalDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func jsonStringArray(values []string) []byte {
	raw, _ := json.Marshal(values)
	return raw
}

func jsonInt64Array(values []int64) []byte {
	raw, _ := json.Marshal(values)
	return raw
}

func decodeStringArray(raw []byte) []string {
	var values []string
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &values)
	}
	if values == nil {
		return []string{}
	}
	return values
}

func decodeInt64Array(raw []byte) []int64 {
	var values []int64
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &values)
	}
	if values == nil {
		return []int64{}
	}
	return values
}

func (s *Services) ListProjects(ctx context.Context, query ProjectQuery) (ProjectListResponse, error) {
	if s.DB == nil {
		return ProjectListResponse{}, errors.New("数据库未配置")
	}
	where := []string{"1=1"}
	args := []any{query.Username}
	if strings.TrimSpace(query.Keyword) != "" {
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(query.Keyword))+"%")
		where = append(where, fmt.Sprintf("(lower(p.name) LIKE $%d OR lower(p.code) LIKE $%d OR lower(p.summary) LIKE $%d)", len(args), len(args), len(args)))
	}
	if strings.TrimSpace(query.Status) != "" {
		args = append(args, strings.TrimSpace(query.Status))
		where = append(where, fmt.Sprintf("p.status=$%d", len(args)))
	}
	if !query.CanManageAll {
		where = append(where, "(p.owner_username=$1 OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id=p.id AND pm.username=$1 AND pm.status='active'))")
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT p.id,p.code,p.name,p.summary,p.owner_username,COALESCE(owner.display_name,''),
COALESCE(p.unit_id,0),COALESCE(u.name,''),COALESCE(p.team_id,0),COALESCE(t.name,''),
COALESCE(NULLIF(p.slurm_account,''),p.code),COALESCE(p.slurm_parent_account,''),COALESCE(p.slurm_qos,''),p.slurm_sync_enabled,COALESCE(p.slurm_sync_status,'pending'),COALESCE(p.slurm_sync_message,''),p.slurm_synced_at,
p.status,p.priority,p.start_date,p.end_date,p.storage_quota_gb,p.compute_quota_hours,p.license_budget_points,
p.tags,p.created_by,p.updated_by,p.created_at,p.updated_at,
COALESCE(m.member_count,0),COALESCE(task.task_count,0),COALESCE(task.done_task_count,0),
COALESCE(dir.directory_count,0),COALESCE(job.job_count,0),COALESCE(job.running_job_count,0),
COALESCE(job.pending_job_count,0),COALESCE(job.completed_job_count,0),COALESCE(job.failed_job_count,0),
COALESCE(job.elapsed_hours,0),COALESCE(job.allocated_cpu_hours,0),COALESCE(job.allocated_gpu_hours,0),
COALESCE(me.role,''),COALESCE(me.permission,''),COALESCE(me.default_project,false)
FROM projects p
LEFT JOIN platform_users owner ON owner.username=p.owner_username
LEFT JOIN units u ON u.id=p.unit_id
LEFT JOIN teams t ON t.id=p.team_id
LEFT JOIN (SELECT project_id,COUNT(*) member_count FROM project_members WHERE status='active' GROUP BY project_id) m ON m.project_id=p.id
LEFT JOIN (SELECT project_id,COUNT(*) task_count,COUNT(*) FILTER (WHERE status='done') done_task_count FROM project_tasks GROUP BY project_id) task ON task.project_id=p.id
LEFT JOIN (SELECT project_id,COUNT(*) directory_count FROM project_directories WHERE status='active' GROUP BY project_id) dir ON dir.project_id=p.id
LEFT JOIN (
  SELECT project_id,
         COUNT(*) job_count,
         COUNT(*) FILTER (WHERE upper(state) IN ('RUNNING','R','运行中')) running_job_count,
         COUNT(*) FILTER (WHERE upper(state) IN ('PENDING','PD','排队中')) pending_job_count,
         COUNT(*) FILTER (WHERE upper(state) IN ('COMPLETED','CD','DONE','完成')) completed_job_count,
         COUNT(*) FILTER (WHERE upper(state) IN ('FAILED','F','CANCELLED','CA','TIMEOUT','TO','NODE_FAIL','NF','OUT_OF_MEMORY','OOM')) failed_job_count,
         ROUND(SUM(elapsed_seconds)::numeric / 3600, 2) elapsed_hours,
         ROUND(SUM(cpu_time_seconds)::numeric / 3600, 2) allocated_cpu_hours,
         ROUND(SUM(gpu_count * elapsed_seconds)::numeric / 3600, 2) allocated_gpu_hours
  FROM (
    SELECT DISTINCT ON (project_id,job_id)
           project_id,job_id,state,elapsed_seconds,cpu_time_seconds,gpu_count
    FROM (
      SELECT p2.id project_id,sj.job_id,sj.state,sj.elapsed_seconds,sj.cpu_time_seconds,sj.gpu_count,1 source_rank
      FROM projects p2
      JOIN slurm_jobs sj ON sj.account=COALESCE(NULLIF(p2.slurm_account,''),p2.code) AND sj.account<>''
      UNION ALL
      SELECT pjl.project_id,pjl.job_id,COALESCE(sj.state,pjl.state),COALESCE(sj.elapsed_seconds,0),COALESCE(sj.cpu_time_seconds,0),COALESCE(sj.gpu_count,0),2 source_rank
      FROM project_job_links pjl
      LEFT JOIN slurm_jobs sj ON sj.job_id=pjl.job_id
    ) project_jobs_raw
    ORDER BY project_id,job_id,source_rank
  ) project_jobs
  GROUP BY project_id
) job ON job.project_id=p.id
LEFT JOIN project_members me ON me.project_id=p.id AND me.username=$1 AND me.status='active'
WHERE `+strings.Join(where, " AND ")+`
ORDER BY CASE p.status WHEN 'active' THEN 1 WHEN 'planning' THEN 2 WHEN 'paused' THEN 3 WHEN 'completed' THEN 4 ELSE 5 END,p.updated_at DESC,p.id DESC`, args...)
	if err != nil {
		return ProjectListResponse{}, err
	}
	defer rows.Close()
	items := []ProjectRecord{}
	summary := ProjectSummary{}
	for rows.Next() {
		item, err := scanProjectRecord(rows)
		if err != nil {
			return ProjectListResponse{}, err
		}
		item.CanManage = query.CanManageAll || item.OwnerUsername == query.Username || item.CurrentUserAccess == "manage"
		items = append(items, item)
		summary.Total++
		if item.Status == "active" {
			summary.Active++
		}
		if item.Status == "paused" {
			summary.Paused++
		}
		if item.Status == "completed" {
			summary.Completed++
		}
		if item.CurrentUserRole != "" || item.OwnerUsername == query.Username {
			summary.MyProjects++
		}
		summary.RunningJobs += item.Metrics.RunningJobCount
		summary.OpenTasks += item.Metrics.TaskCount - item.Metrics.DoneTaskCount
		summary.StorageQuotaGB += item.StorageQuotaGB
	}
	if err := rows.Err(); err != nil {
		return ProjectListResponse{}, err
	}
	return ProjectListResponse{Items: items, Count: len(items), Summary: summary}, nil
}

func scanProjectRecord(row interface{ Scan(dest ...any) error }) (ProjectRecord, error) {
	var item ProjectRecord
	var startDate, endDate sql.NullTime
	var slurmSyncedAt sql.NullTime
	var createdAt, updatedAt time.Time
	var tagsRaw []byte
	err := row.Scan(&item.ID, &item.Code, &item.Name, &item.Summary, &item.OwnerUsername, &item.OwnerDisplayName,
		&item.UnitID, &item.UnitName, &item.TeamID, &item.TeamName,
		&item.SlurmAccount, &item.SlurmParentAccount, &item.SlurmQOS, &item.SlurmSyncEnabled, &item.SlurmSyncStatus, &item.SlurmSyncMessage, &slurmSyncedAt,
		&item.Status, &item.Priority,
		&startDate, &endDate, &item.StorageQuotaGB, &item.ComputeQuotaHours, &item.LicenseBudgetPoints,
		&tagsRaw, &item.CreatedBy, &item.UpdatedBy, &createdAt, &updatedAt,
		&item.Metrics.MemberCount, &item.Metrics.TaskCount, &item.Metrics.DoneTaskCount,
		&item.Metrics.DirectoryCount, &item.Metrics.JobCount, &item.Metrics.RunningJobCount,
		&item.Metrics.PendingJobCount, &item.Metrics.CompletedJobCount, &item.Metrics.FailedJobCount,
		&item.Metrics.ElapsedHours, &item.Metrics.AllocatedCPUHours, &item.Metrics.AllocatedGPUHours,
		&item.CurrentUserRole, &item.CurrentUserAccess, &item.CurrentUserDefault)
	if err != nil {
		return item, err
	}
	if startDate.Valid {
		item.StartDate = startDate.Time.Format("2006-01-02")
	}
	if endDate.Valid {
		item.EndDate = endDate.Time.Format("2006-01-02")
	}
	if slurmSyncedAt.Valid {
		item.SlurmSyncedAt = slurmSyncedAt.Time.Format(time.RFC3339)
	}
	if item.SlurmAccount == "" {
		item.SlurmAccount = item.Code
	}
	item.Tags = decodeStringArray(tagsRaw)
	if item.Metrics.TaskCount > 0 {
		item.Metrics.ProgressPercent = float64(item.Metrics.DoneTaskCount) * 100 / float64(item.Metrics.TaskCount)
	}
	item.CreatedAt = createdAt.Format(time.RFC3339)
	item.UpdatedAt = updatedAt.Format(time.RFC3339)
	return item, nil
}

func (s *Services) GetProject(ctx context.Context, id int64, username string, canManageAll bool) (ProjectRecord, error) {
	if s.DB == nil {
		return ProjectRecord{}, errors.New("数据库未配置")
	}
	resp, err := s.ListProjects(ctx, ProjectQuery{Username: username, CanManageAll: canManageAll})
	if err != nil {
		return ProjectRecord{}, err
	}
	var project ProjectRecord
	for _, item := range resp.Items {
		if item.ID == id {
			project = item
			break
		}
	}
	if project.ID == 0 {
		return ProjectRecord{}, sql.ErrNoRows
	}
	project.Members, _ = s.ListProjectMembers(ctx, id)
	project.Tasks, _ = s.ListProjectTasks(ctx, id)
	project.Directories, _ = s.ListProjectDirectories(ctx, id)
	project.JobLinks, _ = s.ListProjectJobLinks(ctx, id)
	project.Activities, _ = s.ListProjectActivities(ctx, id, 8)
	return project, nil
}

func (s *Services) SaveProject(ctx context.Context, id int64, input ProjectInput, actor string) (ProjectRecord, error) {
	if s.DB == nil {
		return ProjectRecord{}, errors.New("数据库未配置")
	}
	input = normalizeProjectInput(input)
	if err := validateProjectInput(input, id == 0); err != nil {
		return ProjectRecord{}, err
	}
	start, _ := parseOptionalDate(input.StartDate)
	end, _ := parseOptionalDate(input.EndDate)
	var unit any
	if input.UnitID > 0 {
		unit = input.UnitID
	}
	var team any
	if input.TeamID > 0 {
		team = input.TeamID
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ProjectRecord{}, err
	}
	defer tx.Rollback()
	if id > 0 {
		result, err := tx.ExecContext(ctx, `
UPDATE projects SET name=$2,summary=$3,owner_username=$4,unit_id=$5,team_id=$6,
slurm_account=$7,slurm_parent_account=$8,slurm_qos=$9,slurm_sync_enabled=$10,slurm_sync_status='pending',slurm_sync_message='',
status=$11,priority=$12,start_date=$13,end_date=$14,
storage_quota_gb=$15,compute_quota_hours=$16,license_budget_points=$17,tags=$18,updated_by=$19,updated_at=now()
WHERE id=$1`, id, input.Name, input.Summary, input.OwnerUsername, unit, team,
			input.SlurmAccount, input.SlurmParentAccount, input.SlurmQOS, input.SlurmSyncEnabled, input.Status, input.Priority, start, end,
			input.StorageQuotaGB, input.ComputeQuotaHours, input.LicenseBudgetPoints, jsonStringArray(input.Tags), actor)
		if err != nil {
			return ProjectRecord{}, err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return ProjectRecord{}, sql.ErrNoRows
		}
		if err := s.recordProjectActivityTx(ctx, tx, id, actor, "project.update", "project", fmt.Sprint(id), "更新项目基础信息", nil); err != nil {
			return ProjectRecord{}, err
		}
	} else {
		err := tx.QueryRowContext(ctx, `
INSERT INTO projects(code,name,summary,owner_username,unit_id,team_id,slurm_account,slurm_parent_account,slurm_qos,slurm_sync_enabled,status,priority,start_date,end_date,storage_quota_gb,compute_quota_hours,license_budget_points,tags,created_by,updated_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$19)
RETURNING id`, input.Code, input.Name, input.Summary, input.OwnerUsername, unit, team, input.SlurmAccount, input.SlurmParentAccount, input.SlurmQOS, input.SlurmSyncEnabled, input.Status, input.Priority, start, end,
			input.StorageQuotaGB, input.ComputeQuotaHours, input.LicenseBudgetPoints, jsonStringArray(input.Tags), actor).Scan(&id)
		if err != nil {
			return ProjectRecord{}, err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO project_members(project_id,username,display_name,role,permission,default_project,created_by)
VALUES($1,$2,$3,'owner','manage',false,$4)
ON CONFLICT(project_id,username) DO UPDATE SET role='owner',permission='manage',status='active'`,
			id, input.OwnerUsername, input.OwnerUsername, actor); err != nil {
			return ProjectRecord{}, err
		}
		defaultPath := "/data/projects/" + input.Code
		if _, err := tx.ExecContext(ctx, `
INSERT INTO project_directories(project_id,name,path,permission,created_by)
VALUES($1,'项目数据空间',$2,'rw',$3)
ON CONFLICT(project_id,path) DO NOTHING`, id, defaultPath, actor); err != nil {
			return ProjectRecord{}, err
		}
		if err := s.recordProjectActivityTx(ctx, tx, id, actor, "project.create", "project", fmt.Sprint(id), "创建项目", nil); err != nil {
			return ProjectRecord{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ProjectRecord{}, err
	}
	if input.SlurmSyncEnabled {
		_ = s.SyncProjectSlurmAccount(ctx, id, actor)
	}
	return s.GetProject(ctx, id, actor, true)
}

func (s *Services) DeleteProject(ctx context.Context, id int64, actor string) error {
	if s.DB == nil {
		return errors.New("数据库未配置")
	}
	result, err := s.DB.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Services) ProjectAccess(ctx context.Context, projectID int64, username string, canManageAll bool) (string, string, error) {
	if canManageAll {
		return "manager", "manage", nil
	}
	var owner string
	if err := s.DB.QueryRowContext(ctx, `SELECT owner_username FROM projects WHERE id=$1`, projectID).Scan(&owner); err != nil {
		return "", "", err
	}
	if owner == username {
		return "owner", "manage", nil
	}
	var role, permission string
	err := s.DB.QueryRowContext(ctx, `SELECT role,permission FROM project_members WHERE project_id=$1 AND username=$2 AND status='active'`, projectID, username).Scan(&role, &permission)
	if err != nil {
		return "", "", err
	}
	return role, permission, nil
}

func (s *Services) ProjectJobAccount(ctx context.Context, projectID int64, username string, canManageAll bool) (string, error) {
	if _, _, err := s.ProjectAccess(ctx, projectID, username, canManageAll); err != nil {
		return "", err
	}
	var account string
	if err := s.DB.QueryRowContext(ctx, `SELECT COALESCE(NULLIF(slurm_account,''),code) FROM projects WHERE id=$1`, projectID).Scan(&account); err != nil {
		return "", err
	}
	return strings.TrimSpace(account), nil
}

func (s *Services) ProjectJobAccessByAccount(ctx context.Context, account string, username string, canManageAll bool) (string, string, error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return "", "", sql.ErrNoRows
	}
	var projectID int64
	if err := s.DB.QueryRowContext(ctx, `SELECT id FROM projects WHERE COALESCE(NULLIF(slurm_account,''),code)=$1`, account).Scan(&projectID); err != nil {
		return "", "", err
	}
	return s.ProjectAccess(ctx, projectID, username, canManageAll)
}

func (s *Services) SyncProjectSlurmAccount(ctx context.Context, projectID int64, actor string) error {
	if s.DB == nil {
		return errors.New("数据库未配置")
	}
	if s.Slurm == nil {
		return s.updateProjectSlurmStatus(ctx, projectID, "disabled", "Slurm 集成未配置")
	}
	project, err := s.projectSlurmConfig(ctx, projectID)
	if err != nil {
		return err
	}
	if !project.SlurmSyncEnabled {
		return s.updateProjectSlurmStatus(ctx, projectID, "disabled", "项目未启用 Slurm Account 同步")
	}
	account := strings.TrimSpace(project.SlurmAccount)
	if account == "" {
		account = strings.TrimSpace(project.Code)
	}
	if !projectSlurmAccountPattern.MatchString(account) {
		err := errors.New("Slurm Account 格式无效")
		_ = s.updateProjectSlurmStatus(ctx, projectID, "error", err.Error())
		return err
	}
	members, err := s.ListProjectMembers(ctx, projectID)
	if err != nil {
		_ = s.updateProjectSlurmStatus(ctx, projectID, "error", "读取项目成员失败")
		return err
	}
	if err := s.Slurm.EnsureProjectAccount(ctx, slurmintegration.ProjectAccountRequest{
		Name:         account,
		Parent:       project.SlurmParentAccount,
		Description:  project.Name,
		Organization: project.TeamName,
		QOS:          project.SlurmQOS,
	}); err != nil {
		message := safeProjectSyncMessage(err)
		_ = s.updateProjectSlurmStatus(ctx, projectID, "error", message)
		_ = s.recordProjectActivity(ctx, projectID, actor, "slurm.sync.failed", "slurm_account", account, "Slurm Account 同步失败", map[string]any{"error": message})
		return errors.New(message)
	}
	applied := 0
	for _, member := range members {
		if member.Status != "active" {
			continue
		}
		if err := s.Slurm.EnsureUserAccountAssociation(ctx, account, member.Username, project.SlurmQOS, member.DefaultProject); err != nil {
			message := safeProjectSyncMessage(err)
			_ = s.updateProjectSlurmStatus(ctx, projectID, "error", message)
			_ = s.recordProjectActivity(ctx, projectID, actor, "slurm.association.failed", "member", member.Username, "Slurm 用户关联同步失败", map[string]any{"error": message})
			return errors.New(message)
		}
		applied++
	}
	associations, err := s.Slurm.Associations(ctx)
	if err != nil {
		message := safeProjectSyncMessage(fmt.Errorf("读取 Slurm 用户关联失败: %w", err))
		_ = s.updateProjectSlurmStatus(ctx, projectID, "error", message)
		return errors.New(message)
	}
	revoked := 0
	for _, association := range staleProjectAssociations(account, members, associations) {
		if association.DefaultAccount == account {
			fallback := strings.TrimSpace(s.Slurm.DefaultAccount)
			if fallback == "" || fallback == account {
				message := fmt.Sprintf("用户 %s 的默认 Account 仍为 %s，无法安全撤销关联", association.User, account)
				_ = s.updateProjectSlurmStatus(ctx, projectID, "error", message)
				return errors.New(message)
			}
			if err := s.Slurm.EnsureUserAccountAssociation(ctx, fallback, association.User, "", true); err != nil {
				message := safeProjectSyncMessage(fmt.Errorf("恢复用户 %s 的默认 Account 失败: %w", association.User, err))
				_ = s.updateProjectSlurmStatus(ctx, projectID, "error", message)
				return errors.New(message)
			}
		}
		if err := s.Slurm.RemoveUserAccountAssociation(ctx, account, association.User); err != nil {
			message := safeProjectSyncMessage(err)
			_ = s.updateProjectSlurmStatus(ctx, projectID, "error", message)
			return errors.New(message)
		}
		revoked++
	}
	message := fmt.Sprintf("已同步 Slurm Account %s，关联 %d 个成员，回收 %d 个过期关联", account, applied, revoked)
	if err := s.updateProjectSlurmStatus(ctx, projectID, "success", message); err != nil {
		return err
	}
	_ = s.recordProjectActivity(ctx, projectID, actor, "slurm.sync", "slurm_account", account, message, map[string]any{"members": applied})
	return nil
}

func staleProjectAssociations(account string, members []ProjectMember, associations []slurmintegration.Association) []slurmintegration.Association {
	active := make(map[string]struct{}, len(members))
	for _, member := range members {
		if member.Status == "active" {
			active[member.Username] = struct{}{}
		}
	}
	seen := map[string]struct{}{}
	stale := []slurmintegration.Association{}
	for _, association := range associations {
		if strings.TrimSpace(association.Account) != account || strings.TrimSpace(association.User) == "" {
			continue
		}
		if _, ok := active[association.User]; ok {
			continue
		}
		if _, ok := seen[association.User]; ok {
			continue
		}
		seen[association.User] = struct{}{}
		stale = append(stale, association)
	}
	return stale
}

func (s *Services) projectSlurmConfig(ctx context.Context, projectID int64) (ProjectRecord, error) {
	var project ProjectRecord
	err := s.DB.QueryRowContext(ctx, `
SELECT projects.id,projects.code,projects.name,COALESCE(projects.slurm_account,''),COALESCE(projects.slurm_parent_account,''),COALESCE(projects.slurm_qos,''),projects.slurm_sync_enabled,COALESCE(team.name,'')
FROM projects
LEFT JOIN teams team ON team.id=projects.team_id
WHERE projects.id=$1`, projectID).Scan(&project.ID, &project.Code, &project.Name, &project.SlurmAccount, &project.SlurmParentAccount, &project.SlurmQOS, &project.SlurmSyncEnabled, &project.TeamName)
	if err != nil {
		return ProjectRecord{}, err
	}
	if project.SlurmAccount == "" {
		project.SlurmAccount = project.Code
	}
	return project, nil
}

func (s *Services) updateProjectSlurmStatus(ctx context.Context, projectID int64, status string, message string) error {
	message = safeProjectSyncMessage(errors.New(message))
	_, err := s.DB.ExecContext(ctx, `
UPDATE projects
SET slurm_sync_status=$2,slurm_sync_message=$3,slurm_synced_at=CASE WHEN $2='success' THEN now() ELSE slurm_synced_at END,updated_at=now()
WHERE id=$1`, projectID, status, message)
	return err
}

func (s *Services) SetDefaultProjectForUser(ctx context.Context, projectID int64, username string, actor string) error {
	username = strings.TrimSpace(username)
	if err := validateLinuxAccountName(username, "成员账号"); err != nil {
		return err
	}
	if err := withTx(ctx, s.DB, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE project_members SET default_project=false WHERE username=$1`, username); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `UPDATE project_members SET default_project=true,status='active' WHERE project_id=$1 AND username=$2`, projectID, username)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return sql.ErrNoRows
		}
		return s.recordProjectActivityTx(ctx, tx, projectID, actor, "member.default", "member", username, "设置默认项目", nil)
	}); err != nil {
		return err
	}
	return s.SyncProjectSlurmAccount(ctx, projectID, actor)
}

func safeProjectSyncMessage(err error) string {
	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = "Slurm 同步失败"
	}
	message = strings.ReplaceAll(message, "\n", " ")
	message = strings.ReplaceAll(message, "\r", " ")
	if len([]rune(message)) > 220 {
		message = string([]rune(message)[:220]) + "..."
	}
	return message
}

func (s *Services) ListProjectMembers(ctx context.Context, projectID int64) ([]ProjectMember, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,project_id,username,display_name,role,permission,status,default_project,joined_at,created_by FROM project_members WHERE project_id=$1 ORDER BY default_project DESC,CASE role WHEN 'owner' THEN 1 WHEN 'manager' THEN 2 ELSE 3 END,username`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProjectMember{}
	for rows.Next() {
		var item ProjectMember
		var joined time.Time
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Username, &item.DisplayName, &item.Role, &item.Permission, &item.Status, &item.DefaultProject, &joined, &item.CreatedBy); err != nil {
			return nil, err
		}
		item.JoinedAt = joined.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) SaveProjectMember(ctx context.Context, projectID int64, input ProjectMemberInput, actor string) (ProjectMember, error) {
	input.Username = strings.TrimSpace(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Role = strings.TrimSpace(input.Role)
	input.Permission = strings.TrimSpace(input.Permission)
	input.Status = strings.TrimSpace(input.Status)
	if len(input.Role) == 0 {
		input.Role = "compute_member"
	}
	if input.Permission == "" {
		input.Permission = "work"
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if err := validateLinuxAccountName(input.Username, "成员账号"); err != nil {
		return ProjectMember{}, err
	}
	if !validProjectRole(input.Role) {
		return ProjectMember{}, errors.New("项目角色无效")
	}
	if !validProjectPermission(input.Permission) {
		return ProjectMember{}, errors.New("项目权限无效")
	}
	if input.Status != "active" && input.Status != "disabled" {
		return ProjectMember{}, errors.New("成员状态无效")
	}
	if input.DisplayName == "" {
		input.DisplayName = input.Username
	}
	err := withTx(ctx, s.DB, func(tx *sql.Tx) error {
		if input.DefaultProject && input.Status == "active" {
			if _, err := tx.ExecContext(ctx, `UPDATE project_members SET default_project=false WHERE username=$1`, input.Username); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `
INSERT INTO project_members(project_id,username,display_name,role,permission,status,default_project,created_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT(project_id,username) DO UPDATE SET display_name=EXCLUDED.display_name,role=EXCLUDED.role,permission=EXCLUDED.permission,status=EXCLUDED.status,default_project=EXCLUDED.default_project`,
			projectID, input.Username, input.DisplayName, input.Role, input.Permission, input.Status, input.DefaultProject && input.Status == "active", actor)
		return err
	})
	if err != nil {
		return ProjectMember{}, err
	}
	_ = s.recordProjectActivity(ctx, projectID, actor, "member.upsert", "member", input.Username, "维护项目成员", map[string]any{"role": input.Role, "permission": input.Permission})
	_ = s.SyncProjectSlurmAccount(ctx, projectID, actor)
	members, err := s.ListProjectMembers(ctx, projectID)
	if err != nil {
		return ProjectMember{}, err
	}
	for _, item := range members {
		if item.Username == input.Username {
			return item, nil
		}
	}
	return ProjectMember{}, sql.ErrNoRows
}

func (s *Services) DeleteProjectMember(ctx context.Context, projectID int64, username string, actor string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("成员账号不能为空")
	}
	var role string
	if err := s.DB.QueryRowContext(ctx, `SELECT role FROM project_members WHERE project_id=$1 AND username=$2`, projectID, username).Scan(&role); err != nil {
		return err
	}
	if role == "owner" {
		return errors.New("项目负责人不能直接移除")
	}
	result, err := s.DB.ExecContext(ctx, `DELETE FROM project_members WHERE project_id=$1 AND username=$2`, projectID, username)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	_ = s.recordProjectActivity(ctx, projectID, actor, "member.delete", "member", username, "移除项目成员", nil)
	if err := s.SyncProjectSlurmAccount(ctx, projectID, actor); err != nil {
		return fmt.Errorf("成员已移除，但 Slurm Account 授权回收失败，请重新同步项目: %w", err)
	}
	return nil
}

func (s *Services) ListProjectTasks(ctx context.Context, projectID int64) ([]ProjectTask, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,project_id,title,assignee_username,status,priority,due_date,description,upstream_task_ids,created_by,updated_by,created_at,updated_at FROM project_tasks WHERE project_id=$1 ORDER BY CASE status WHEN 'running' THEN 1 WHEN 'blocked' THEN 2 WHEN 'todo' THEN 3 WHEN 'done' THEN 4 ELSE 5 END,due_date NULLS LAST,id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProjectTask{}
	for rows.Next() {
		var item ProjectTask
		var due sql.NullTime
		var upstream []byte
		var created, updated time.Time
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Title, &item.AssigneeUsername, &item.Status, &item.Priority, &due, &item.Description, &upstream, &item.CreatedBy, &item.UpdatedBy, &created, &updated); err != nil {
			return nil, err
		}
		if due.Valid {
			item.DueDate = due.Time.Format("2006-01-02")
		}
		item.UpstreamTaskIDs = decodeInt64Array(upstream)
		item.CreatedAt = created.Format(time.RFC3339)
		item.UpdatedAt = updated.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) SaveProjectTask(ctx context.Context, projectID int64, input ProjectTaskInput, actor string) (ProjectTask, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.AssigneeUsername = strings.TrimSpace(input.AssigneeUsername)
	input.Status = strings.TrimSpace(input.Status)
	input.Priority = strings.TrimSpace(input.Priority)
	input.DueDate = strings.TrimSpace(input.DueDate)
	input.Description = strings.TrimSpace(input.Description)
	if input.Title == "" {
		return ProjectTask{}, errors.New("任务名称不能为空")
	}
	if input.Status == "" {
		input.Status = "todo"
	}
	if input.Priority == "" {
		input.Priority = "normal"
	}
	if !validTaskStatus(input.Status) {
		return ProjectTask{}, errors.New("任务状态无效")
	}
	if !validProjectPriority(input.Priority) {
		return ProjectTask{}, errors.New("任务优先级无效")
	}
	due, err := parseOptionalDate(input.DueDate)
	if err != nil {
		return ProjectTask{}, errors.New("截止日期格式无效")
	}
	if input.ID > 0 {
		result, err := s.DB.ExecContext(ctx, `
UPDATE project_tasks SET title=$3,assignee_username=$4,status=$5,priority=$6,due_date=$7,description=$8,upstream_task_ids=$9,updated_by=$10,updated_at=now()
WHERE id=$1 AND project_id=$2`, input.ID, projectID, input.Title, input.AssigneeUsername, input.Status, input.Priority, due, input.Description, jsonInt64Array(input.UpstreamTaskIDs), actor)
		if err != nil {
			return ProjectTask{}, err
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			return ProjectTask{}, sql.ErrNoRows
		}
	} else {
		err = s.DB.QueryRowContext(ctx, `
INSERT INTO project_tasks(project_id,title,assignee_username,status,priority,due_date,description,upstream_task_ids,created_by,updated_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$9) RETURNING id`,
			projectID, input.Title, input.AssigneeUsername, input.Status, input.Priority, due, input.Description, jsonInt64Array(input.UpstreamTaskIDs), actor).Scan(&input.ID)
		if err != nil {
			return ProjectTask{}, err
		}
	}
	_ = s.recordProjectActivity(ctx, projectID, actor, "task.save", "task", fmt.Sprint(input.ID), "维护项目任务", map[string]any{"status": input.Status})
	tasks, err := s.ListProjectTasks(ctx, projectID)
	if err != nil {
		return ProjectTask{}, err
	}
	for _, item := range tasks {
		if item.ID == input.ID {
			return item, nil
		}
	}
	return ProjectTask{}, sql.ErrNoRows
}

func (s *Services) DeleteProjectTask(ctx context.Context, projectID, taskID int64, actor string) error {
	result, err := s.DB.ExecContext(ctx, `DELETE FROM project_tasks WHERE id=$1 AND project_id=$2`, taskID, projectID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	_ = s.recordProjectActivity(ctx, projectID, actor, "task.delete", "task", fmt.Sprint(taskID), "删除项目任务", nil)
	return nil
}

func validateProjectDirectoryPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("项目数据目录不能为空")
	}
	if strings.ContainsRune(path, 0) {
		return errors.New("项目数据目录包含非法字符")
	}
	if !filepath.IsAbs(path) {
		return errors.New("项目数据目录必须是绝对路径")
	}
	clean := filepath.Clean(path)
	if clean != path || clean == "/" {
		return errors.New("项目数据目录必须是规范绝对路径")
	}
	if !strings.HasPrefix(clean, "/data/projects/") && !strings.HasPrefix(clean, "/data/project/") {
		return errors.New("项目数据目录必须位于 /data/projects 或 /data/project 下")
	}
	return nil
}

func (s *Services) ListProjectDirectories(ctx context.Context, projectID int64) ([]ProjectDirectory, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,project_id,name,path,permission,status,created_by,created_at FROM project_directories WHERE project_id=$1 ORDER BY status,name,path`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProjectDirectory{}
	for rows.Next() {
		var item ProjectDirectory
		var created time.Time
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Name, &item.Path, &item.Permission, &item.Status, &item.CreatedBy, &created); err != nil {
			return nil, err
		}
		item.CreatedAt = created.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) SaveProjectDirectory(ctx context.Context, projectID int64, input ProjectDirectoryInput, actor string) (ProjectDirectory, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Path = strings.TrimSpace(input.Path)
	input.Permission = strings.TrimSpace(input.Permission)
	input.Status = strings.TrimSpace(input.Status)
	if input.Name == "" {
		input.Name = "项目数据空间"
	}
	if input.Permission == "" {
		input.Permission = "rw"
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if err := validateProjectDirectoryPath(input.Path); err != nil {
		return ProjectDirectory{}, err
	}
	if input.Permission != "r" && input.Permission != "rw" && input.Permission != "rwx" && input.Permission != "manage" {
		return ProjectDirectory{}, errors.New("目录权限无效")
	}
	if input.Status != "active" && input.Status != "disabled" {
		return ProjectDirectory{}, errors.New("目录状态无效")
	}
	var id int64
	err := s.DB.QueryRowContext(ctx, `
INSERT INTO project_directories(project_id,name,path,permission,status,created_by)
VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT(project_id,path) DO UPDATE SET name=EXCLUDED.name,permission=EXCLUDED.permission,status=EXCLUDED.status
RETURNING id`, projectID, input.Name, input.Path, input.Permission, input.Status, actor).Scan(&id)
	if err != nil {
		return ProjectDirectory{}, err
	}
	_ = s.recordProjectActivity(ctx, projectID, actor, "directory.save", "directory", input.Path, "维护项目数据空间", nil)
	items, err := s.ListProjectDirectories(ctx, projectID)
	if err != nil {
		return ProjectDirectory{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return ProjectDirectory{}, sql.ErrNoRows
}

func (s *Services) DeleteProjectDirectory(ctx context.Context, projectID, dirID int64, actor string) error {
	result, err := s.DB.ExecContext(ctx, `DELETE FROM project_directories WHERE id=$1 AND project_id=$2`, dirID, projectID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	_ = s.recordProjectActivity(ctx, projectID, actor, "directory.delete", "directory", fmt.Sprint(dirID), "删除项目数据空间", nil)
	return nil
}

func (s *Services) ListProjectJobLinks(ctx context.Context, projectID int64) ([]ProjectJobLink, error) {
	rows, err := s.DB.QueryContext(ctx, `
WITH project_account AS (
  SELECT COALESCE(NULLIF(slurm_account,''),code) account FROM projects WHERE id=$1
),
linked AS (
  SELECT id,project_id,job_id,COALESCE(task_id,0) task_id,job_name,username,account,state,partition,linked_by,linked_at
  FROM project_job_links WHERE project_id=$1
),
account_jobs AS (
  SELECT 0::bigint id,$1::bigint project_id,sj.job_id,0::bigint task_id,sj.name job_name,sj.user_name username,sj.account,sj.state,sj.partition,'slurm-account' linked_by,sj.synced_at linked_at
  FROM slurm_jobs sj, project_account pa
  WHERE pa.account <> '' AND sj.account=pa.account
)
SELECT id,project_id,job_id,task_id,job_name,username,account,state,partition,linked_by,linked_at
FROM (
  SELECT * FROM linked
  UNION ALL
  SELECT * FROM account_jobs aj WHERE NOT EXISTS (SELECT 1 FROM linked l WHERE l.job_id=aj.job_id)
) merged
ORDER BY linked_at DESC,job_id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProjectJobLink{}
	for rows.Next() {
		var item ProjectJobLink
		var linked time.Time
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.JobID, &item.TaskID, &item.JobName, &item.Username, &item.Account, &item.State, &item.Partition, &item.LinkedBy, &linked); err != nil {
			return nil, err
		}
		item.LinkedAt = linked.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) SaveProjectJobLink(ctx context.Context, projectID int64, input ProjectJobLinkInput, actor string) (ProjectJobLink, error) {
	input.JobID = strings.TrimSpace(input.JobID)
	input.JobName = strings.TrimSpace(input.JobName)
	input.Username = strings.TrimSpace(input.Username)
	input.Account = strings.TrimSpace(input.Account)
	input.State = strings.TrimSpace(input.State)
	input.Partition = strings.TrimSpace(input.Partition)
	if input.JobID == "" {
		return ProjectJobLink{}, errors.New("作业 ID 不能为空")
	}
	var task any
	if input.TaskID > 0 {
		task = input.TaskID
	}
	var id int64
	err := s.DB.QueryRowContext(ctx, `
INSERT INTO project_job_links(project_id,job_id,task_id,job_name,username,account,state,partition,linked_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT(project_id,job_id) DO UPDATE SET task_id=EXCLUDED.task_id,job_name=EXCLUDED.job_name,username=EXCLUDED.username,account=EXCLUDED.account,state=EXCLUDED.state,partition=EXCLUDED.partition,linked_by=EXCLUDED.linked_by,linked_at=now()
RETURNING id`, projectID, input.JobID, task, input.JobName, input.Username, input.Account, input.State, input.Partition, actor).Scan(&id)
	if err != nil {
		return ProjectJobLink{}, err
	}
	_ = s.recordProjectActivity(ctx, projectID, actor, "job.link", "job", input.JobID, "关联项目作业", map[string]any{"state": input.State})
	items, err := s.ListProjectJobLinks(ctx, projectID)
	if err != nil {
		return ProjectJobLink{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return ProjectJobLink{}, sql.ErrNoRows
}

func (s *Services) DeleteProjectJobLink(ctx context.Context, projectID, linkID int64, actor string) error {
	result, err := s.DB.ExecContext(ctx, `DELETE FROM project_job_links WHERE id=$1 AND project_id=$2`, linkID, projectID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	_ = s.recordProjectActivity(ctx, projectID, actor, "job.unlink", "job", fmt.Sprint(linkID), "取消项目作业关联", nil)
	return nil
}

func (s *Services) ListProjectActivities(ctx context.Context, projectID int64, limit int) ([]ProjectActivity, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id,project_id,actor,action,target_type,target_id,message,detail,created_at FROM project_activity_logs WHERE project_id=$1 ORDER BY created_at DESC LIMIT $2`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ProjectActivity{}
	for rows.Next() {
		var item ProjectActivity
		var detail []byte
		var created time.Time
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Actor, &item.Action, &item.TargetType, &item.TargetID, &item.Message, &detail, &created); err != nil {
			return nil, err
		}
		item.Detail = map[string]any{}
		if len(detail) > 0 {
			_ = json.Unmarshal(detail, &item.Detail)
		}
		item.CreatedAt = created.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) recordProjectActivity(ctx context.Context, projectID int64, actor, action, targetType, targetID, message string, detail map[string]any) error {
	return withTx(ctx, s.DB, func(tx *sql.Tx) error {
		return s.recordProjectActivityTx(ctx, tx, projectID, actor, action, targetType, targetID, message, detail)
	})
}

func (s *Services) recordProjectActivityTx(ctx context.Context, tx *sql.Tx, projectID int64, actor, action, targetType, targetID, message string, detail map[string]any) error {
	if detail == nil {
		detail = map[string]any{}
	}
	raw, _ := json.Marshal(detail)
	_, err := tx.ExecContext(ctx, `
INSERT INTO project_activity_logs(project_id,actor,action,target_type,target_id,message,detail)
VALUES($1,$2,$3,$4,$5,$6,$7)`, projectID, strings.TrimSpace(actor), action, targetType, targetID, message, raw)
	return err
}

func withTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
