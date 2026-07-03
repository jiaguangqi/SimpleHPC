package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"simplehpc/backend/internal/integrations/slurm"
)

type DashboardSnapshot struct {
	Source      string             `json:"source"`
	Generated   string             `json:"generatedAt"`
	TrendRange  string             `json:"trendRange"`
	TrendBucket string             `json:"trendBucket"`
	Users       DashboardUsers     `json:"users"`
	Jobs        DashboardJobs      `json:"jobs"`
	Resources   DashboardResources `json:"resources"`
	Storage     []DashboardStorage `json:"storage"`
	Trends      []DashboardTrend   `json:"trends"`
	Alerts      []DashboardAlert   `json:"alerts"`
	Errors      map[string]string  `json:"errors"`
}

type DashboardUsers struct {
	Total  int  `json:"total"`
	Active int  `json:"active"`
	Online *int `json:"online"`
	Frozen int  `json:"frozen"`
}

type DashboardJobs struct {
	Total   int         `json:"total"`
	Running int         `json:"running"`
	Pending int         `json:"pending"`
	Recent  []SyncedJob `json:"recent"`
}

type DashboardResources struct {
	TotalNodes     int      `json:"totalNodes"`
	TotalCPUs      int      `json:"totalCpus"`
	TotalGPUs      int      `json:"totalGpus"`
	AllocatedCPUs  int      `json:"allocatedCpus"`
	AllocatedGPUs  int      `json:"allocatedGpus"`
	CPUUsage       *float64 `json:"cpuUsagePercent"`
	GPUUsage       *float64 `json:"gpuUsagePercent"`
	IdleNodes      int      `json:"idleNodes"`
	UnavailableGPU string   `json:"gpuUsageNote,omitempty"`
}

type DashboardStorage struct {
	Type           string   `json:"type"`
	Name           string   `json:"name"`
	Path           string   `json:"path"`
	FSType         string   `json:"fsType"`
	Purpose        string   `json:"purpose"`
	TotalBytes     *uint64  `json:"totalBytes,omitempty"`
	UsedBytes      *uint64  `json:"usedBytes,omitempty"`
	AvailableBytes *uint64  `json:"availableBytes,omitempty"`
	UsagePercent   *float64 `json:"usagePercent,omitempty"`
	UsageError     string   `json:"usageError,omitempty"`
}

type DashboardTrend struct {
	SampledAt       string   `json:"sampledAt"`
	CPUUsagePercent *float64 `json:"cpuUsagePercent"`
	GPUUsagePercent *float64 `json:"gpuUsagePercent"`
	RunningJobs     int      `json:"runningJobs"`
	PendingJobs     int      `json:"pendingJobs"`
}

type DashboardAlert struct {
	ID             int64  `json:"id"`
	Level          string `json:"level"`
	Status         string `json:"status"`
	Title          string `json:"title"`
	Message        string `json:"message"`
	Source         string `json:"source"`
	OccurredAt     string `json:"occurredAt"`
	AcknowledgedBy string `json:"acknowledgedBy,omitempty"`
	AcknowledgedAt string `json:"acknowledgedAt,omitempty"`
}

type QueueJobTrendRequest struct {
	Queue string
	Range string
	Query JobQuery
}

type QueueJobTrendResponse struct {
	Queue               string               `json:"queue"`
	Queues              []string             `json:"queues"`
	Range               string               `json:"range"`
	SampleInterval      string               `json:"sampleInterval"`
	SampleIntervalLabel string               `json:"sampleIntervalLabel"`
	TotalPoints         int                  `json:"totalPoints"`
	Points              []QueueJobTrendPoint `json:"points"`
}

type QueueJobTrendPoint struct {
	Time    string `json:"time"`
	Running int    `json:"running"`
	Pending int    `json:"pending"`
}

type dashboardTrendConfig struct {
	Name   string
	Window string
	Bucket string
}

type currentJobSummary struct {
	Running       int
	Pending       int
	AllocatedCPUs int
	AllocatedGPUs int
}

func dashboardTrendRangeConfig(value string) dashboardTrendConfig {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "24h":
		return dashboardTrendConfig{Name: "24h", Window: "24 hours", Bucket: "15 minutes"}
	case "30d":
		return dashboardTrendConfig{Name: "30d", Window: "30 days", Bucket: "6 hours"}
	case "90d":
		return dashboardTrendConfig{Name: "90d", Window: "90 days", Bucket: "1 day"}
	case "1y":
		return dashboardTrendConfig{Name: "1y", Window: "1 year", Bucket: "7 days"}
	default:
		return dashboardTrendConfig{Name: "7d", Window: "7 days", Bucket: "1 hour"}
	}
}

func dashboardTrendBucketLabel(bucket string) string {
	switch bucket {
	case "15 minutes":
		return "15 分钟"
	case "1 hour":
		return "1 小时"
	case "6 hours":
		return "6 小时"
	case "1 day":
		return "1 天"
	case "7 days":
		return "7 天"
	default:
		return bucket
	}
}

func (s *Services) StartDashboardSampleSync(ctx context.Context) {
	if s.DB == nil {
		return
	}
	go func() {
		s.recordDashboardSample(ctx)
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.recordDashboardSample(ctx)
			}
		}
	}()
}

func (s *Services) DashboardSnapshot(ctx context.Context, trendRange string, jobQuery JobQuery) (DashboardSnapshot, error) {
	if s.DB == nil {
		return DashboardSnapshot{}, errNotConfigured("postgres")
	}
	_ = s.recordDashboardSample(ctx)

	users, userErr := s.dashboardUsers(ctx)
	jobs, jobErr := s.dashboardJobs(ctx, jobQuery)
	resources, resourceErr := s.dashboardResources(ctx, jobs)
	storageItems := s.dashboardStorage()
	trendConfig := dashboardTrendRangeConfig(trendRange)
	trends, trendErr := s.dashboardTrends(ctx, trendConfig)
	alerts, alertErr := s.dashboardAlerts(ctx)

	errors := map[string]string{}
	if userErr != nil {
		errors["users"] = userErr.Error()
	}
	if jobErr != nil {
		errors["jobs"] = jobErr.Error()
	}
	if resourceErr != nil {
		errors["resources"] = resourceErr.Error()
	}
	if trendErr != nil {
		errors["trends"] = trendErr.Error()
	}
	if alertErr != nil {
		errors["alerts"] = alertErr.Error()
	}

	return DashboardSnapshot{
		Source:      "slurm-live-postgres-history",
		Generated:   time.Now().Format(time.RFC3339),
		TrendRange:  trendConfig.Name,
		TrendBucket: trendConfig.Bucket,
		Users:       users,
		Jobs:        jobs,
		Resources:   resources,
		Storage:     storageItems,
		Trends:      trends,
		Alerts:      alerts,
		Errors:      errors,
	}, nil
}

func (s *Services) recordDashboardSample(ctx context.Context) error {
	if s.DB == nil {
		return errNotConfigured("postgres")
	}
	jobs, _ := s.dashboardJobs(ctx, JobQuery{})
	resources, _ := s.dashboardResources(ctx, jobs)
	users, _ := s.dashboardUsers(ctx)
	storageRaw, _ := json.Marshal(s.dashboardStorage())
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO dashboard_resource_samples (
  running_jobs, pending_jobs, total_jobs, total_users, total_nodes, total_cpus, total_gpus,
  allocated_cpus, allocated_gpus, cpu_usage_percent, gpu_usage_percent, storage
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb)
`, jobs.Running, jobs.Pending, jobs.Total, users.Total, resources.TotalNodes, resources.TotalCPUs, resources.TotalGPUs,
		resources.AllocatedCPUs, resources.AllocatedGPUs, resources.CPUUsage, resources.GPUUsage, string(storageRaw))
	if err != nil {
		return err
	}
	return s.recordQueueJobTrendSamples(ctx)
}

func (s *Services) dashboardUsers(ctx context.Context) (DashboardUsers, error) {
	users := DashboardUsers{}
	row := s.DB.QueryRowContext(ctx, `
SELECT count(*),
       count(*) FILTER (WHERE status IN ('active','normal','enabled')),
       count(*) FILTER (WHERE status IN ('frozen','disabled','locked'))
FROM platform_users`)
	if err := row.Scan(&users.Total, &users.Active, &users.Frozen); err != nil {
		return users, err
	}
	if s.Redis != nil {
		online, err := s.onlineUserCount(ctx)
		if err != nil {
			return users, err
		}
		users.Online = &online
	}
	return users, nil
}

func (s *Services) dashboardJobs(ctx context.Context, query JobQuery) (DashboardJobs, error) {
	if count, err := s.slurmJobCount(ctx); err == nil && count == 0 {
		_ = s.SyncSlurmJobs(ctx)
	}
	result := DashboardJobs{Recent: []SyncedJob{}}
	where, args := buildJobWhere(query)
	countSQL := `
SELECT count(*),
       count(*) FILTER (WHERE upper(state) IN ('RUNNING','R')),
       count(*) FILTER (WHERE upper(state) IN ('PENDING','PD'))
FROM slurm_jobs WHERE ` + where
	if err := s.DB.QueryRowContext(ctx, countSQL, args...).Scan(&result.Total, &result.Running, &result.Pending); err != nil {
		return result, err
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT job_id, name, user_name, partition, state, node_count, cpu_count, gpu_count,
       runtime, node_list, submit_time, source, synced_at
FROM slurm_jobs
WHERE `+where+`
ORDER BY synced_at DESC, NULLIF(regexp_replace(job_id, '\D.*$', ''), '')::bigint DESC NULLS LAST, job_id DESC
LIMIT 5`, args...)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		var job SyncedJob
		var nodes, cpus, gpus int
		var synced time.Time
		if err := rows.Scan(&job.ID, &job.Name, &job.User, &job.Partition, &job.State, &nodes, &cpus, &gpus, &job.Time, &job.NodeList, &job.Submit, &job.Source, &synced); err != nil {
			return result, err
		}
		job.Nodes = strconv.Itoa(nodes)
		job.CPUs = strconv.Itoa(cpus)
		job.GPUs = strconv.Itoa(gpus)
		job.SyncedAt = synced.Format(time.RFC3339)
		result.Recent = append(result.Recent, job)
	}
	return result, rows.Err()
}

func (s *Services) slurmJobCount(ctx context.Context) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM slurm_jobs`).Scan(&count)
	return count, err
}

func (s *Services) dashboardResources(ctx context.Context, jobs DashboardJobs) (DashboardResources, error) {
	nodes, err := s.Slurm.Nodes(ctx)
	if err != nil {
		return DashboardResources{}, err
	}
	resources := summarizeNodes(nodes)
	current, err := s.Slurm.Jobs(ctx)
	if err != nil {
		return resources, err
	}
	currentSummary := summarizeCurrentJobs(current)
	resources.AllocatedGPUs = currentSummary.AllocatedGPUs
	if resources.TotalGPUs > 0 {
		resources.GPUUsage = roundedPercent(resources.AllocatedGPUs, resources.TotalGPUs)
	}
	return resources, nil
}

func summarizeCurrentJobs(jobs []slurm.Job) currentJobSummary {
	var result currentJobSummary
	for _, job := range jobs {
		state := strings.ToUpper(strings.TrimSpace(job.State))
		switch {
		case state == "RUNNING" || state == "R" || strings.HasPrefix(state, "RUNNING"):
			result.Running++
			result.AllocatedCPUs += atoiDefault(job.CPUs)
			result.AllocatedGPUs += atoiDefault(job.GPUs)
		case state == "PENDING" || state == "PD" || strings.HasPrefix(state, "PENDING"):
			result.Pending++
		}
	}
	return result
}

func summarizeNodes(nodes []slurm.Node) DashboardResources {
	resources := DashboardResources{}
	seen := map[string]bool{}
	for _, node := range nodes {
		if seen[node.Name] {
			continue
		}
		seen[node.Name] = true
		resources.TotalNodes++
		resources.TotalCPUs += atoiDefault(node.CPUTotal)
		resources.AllocatedCPUs += atoiDefault(node.CPUAllocated)
		resources.TotalGPUs += gresCountLocal(node.GRES)
		if strings.Contains(strings.ToLower(node.State), "idle") {
			resources.IdleNodes++
		}
	}
	if resources.TotalCPUs > 0 {
		resources.CPUUsage = roundedPercent(resources.AllocatedCPUs, resources.TotalCPUs)
	}
	if resources.TotalGPUs == 0 {
		resources.UnavailableGPU = "当前 Slurm 集群未配置 GPU 资源"
	}
	return resources
}

func (s *Services) onlineUserCount(ctx context.Context) (int, error) {
	values := make([][]byte, 0)
	var cursor uint64
	for {
		keys, next, err := s.Redis.Scan(ctx, cursor, "auth:session:*", 100).Result()
		if err != nil {
			return 0, err
		}
		for _, key := range keys {
			value, err := s.Redis.Get(ctx, key).Bytes()
			if err == nil {
				values = append(values, value)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return uniqueAuthUsers(values), nil
}

func uniqueAuthUsers(values [][]byte) int {
	users := map[string]bool{}
	for _, value := range values {
		var user AuthUser
		if json.Unmarshal(value, &user) == nil && user.Username != "" {
			users[user.Type+":"+user.Username] = true
		}
	}
	return len(users)
}

func (s *Services) dashboardStorage() []DashboardStorage {
	roots := s.Storage.ListRoots()
	items := make([]DashboardStorage, 0, len(roots))
	for _, root := range roots {
		items = append(items, DashboardStorage{
			Type:           root.Type,
			Name:           root.Name,
			Path:           root.Path,
			FSType:         root.FSType,
			Purpose:        root.Purpose,
			TotalBytes:     root.TotalBytes,
			UsedBytes:      root.UsedBytes,
			AvailableBytes: root.AvailableBytes,
			UsagePercent:   root.UsagePercent,
			UsageError:     root.UsageError,
		})
	}
	return items
}

func (s *Services) dashboardTrends(ctx context.Context, config dashboardTrendConfig) ([]DashboardTrend, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT
	date_bin($1::interval, sampled_at, TIMESTAMPTZ '1970-01-01 00:00:00+00') AS bucket_at,
	AVG(cpu_usage_percent),
	AVG(gpu_usage_percent),
	ROUND(AVG(running_jobs))::integer,
	ROUND(AVG(pending_jobs))::integer
FROM dashboard_resource_samples
WHERE sampled_at >= now() - $2::interval
GROUP BY bucket_at
ORDER BY bucket_at ASC
LIMIT 500`, config.Bucket, config.Window)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DashboardTrend{}
	for rows.Next() {
		var sampled time.Time
		var cpu, gpu sql.NullFloat64
		var item DashboardTrend
		if err := rows.Scan(&sampled, &cpu, &gpu, &item.RunningJobs, &item.PendingJobs); err != nil {
			return items, err
		}
		item.SampledAt = sampled.Format(time.RFC3339)
		if cpu.Valid {
			item.CPUUsagePercent = &cpu.Float64
		}
		if gpu.Valid {
			item.GPUUsagePercent = &gpu.Float64
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) QueueJobTrends(ctx context.Context, request QueueJobTrendRequest) (QueueJobTrendResponse, error) {
	if s.DB == nil {
		return QueueJobTrendResponse{}, errNotConfigured("postgres")
	}
	if count, err := s.slurmJobCount(ctx); err == nil && count == 0 {
		_ = s.SyncSlurmJobs(ctx)
	}
	_ = s.recordQueueJobTrendSamples(ctx)
	config := dashboardTrendRangeConfig(request.Range)
	queues, err := s.visibleJobQueues(ctx, request.Query)
	if err != nil {
		return QueueJobTrendResponse{}, err
	}
	queue := chooseVisibleQueue(strings.TrimSpace(request.Queue), queues)
	response := QueueJobTrendResponse{
		Queue:               queue,
		Queues:              queues,
		Range:               config.Name,
		SampleInterval:      config.Bucket,
		SampleIntervalLabel: dashboardTrendBucketLabel(config.Bucket),
		Points:              []QueueJobTrendPoint{},
	}
	if queue == "" {
		return response, nil
	}
	if isGlobalJobQuery(request.Query) {
		points, err := s.globalQueueJobTrendPoints(ctx, queue, config)
		if err != nil {
			return QueueJobTrendResponse{}, err
		}
		response.Points = points
		response.TotalPoints = len(points)
		return response, nil
	}

	where, args := buildJobWhere(request.Query)
	args = append(args, queue)
	queueArg := len(args)
	args = append(args, config.Bucket)
	bucketArg := len(args)
	args = append(args, config.Window)
	windowArg := len(args)

	eventTime := `CASE
  WHEN submit_time ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}' THEN submit_time::timestamptz
  ELSE synced_at
END`
	query := fmt.Sprintf(`
WITH scoped AS (
  SELECT partition, state, %s AS event_time
  FROM slurm_jobs
  WHERE %s AND partition = $%d
), bucketed AS (
  SELECT
    date_bin($%d::interval, event_time, TIMESTAMPTZ '1970-01-01 00:00:00+00') AS bucket_at,
    count(*) FILTER (WHERE upper(state) IN ('RUNNING','R') OR upper(state) LIKE 'RUNNING%%')::integer AS running,
    count(*) FILTER (WHERE upper(state) IN ('PENDING','PD') OR upper(state) LIKE 'PENDING%%')::integer AS pending
  FROM scoped
  WHERE event_time >= now() - $%d::interval
  GROUP BY bucket_at
)
SELECT bucket_at, running, pending
FROM bucketed
WHERE running > 0 OR pending > 0
ORDER BY bucket_at ASC
LIMIT 500`, eventTime, where, queueArg, bucketArg, windowArg)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return QueueJobTrendResponse{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var point QueueJobTrendPoint
		var bucket time.Time
		if err := rows.Scan(&bucket, &point.Running, &point.Pending); err != nil {
			return response, err
		}
		point.Time = bucket.Format(time.RFC3339)
		response.Points = append(response.Points, point)
	}
	if err := rows.Err(); err != nil {
		return response, err
	}
	response.TotalPoints = len(response.Points)
	return response, nil
}

func (s *Services) visibleJobQueues(ctx context.Context, jobQuery JobQuery) ([]string, error) {
	partitions, err := s.Slurm.Partitions(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	queues := make([]string, 0, len(partitions))
	for _, partition := range partitions {
		queue := strings.TrimSpace(partition.Name)
		if queue != "" {
			if _, ok := seen[queue]; !ok {
				seen[queue] = struct{}{}
				queues = append(queues, queue)
			}
		}
	}
	if len(queues) > 0 || isGlobalJobQuery(jobQuery) {
		return queues, nil
	}
	return s.scopedJobQueuesFromHistory(ctx, jobQuery)
}

func (s *Services) scopedJobQueuesFromHistory(ctx context.Context, jobQuery JobQuery) ([]string, error) {
	where, args := buildJobWhere(jobQuery)
	rows, err := s.DB.QueryContext(ctx, `
SELECT DISTINCT partition
FROM slurm_jobs
WHERE `+where+` AND partition <> ''
ORDER BY partition ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	queues := []string{}
	for rows.Next() {
		var queue string
		if err := rows.Scan(&queue); err != nil {
			return queues, err
		}
		if strings.TrimSpace(queue) != "" {
			queues = append(queues, strings.TrimSpace(queue))
		}
	}
	return queues, rows.Err()
}

func chooseVisibleQueue(requested string, queues []string) string {
	if len(queues) == 0 {
		return ""
	}
	for _, queue := range queues {
		if queue == requested {
			return queue
		}
	}
	for _, queue := range queues {
		if queue == "debug" {
			return queue
		}
	}
	return queues[0]
}

func isGlobalJobQuery(query JobQuery) bool {
	return !query.DenyAll && query.Username == "" && query.Group == "" && query.Partition == "" && len(query.UnitIDs) == 0 && len(query.TeamIDs) == 0
}

func (s *Services) recordQueueJobTrendSamples(ctx context.Context) error {
	if s.DB == nil || s.Slurm == nil {
		return nil
	}
	partitions, partitionErr := s.Slurm.Partitions(ctx)
	jobs, jobErr := s.Slurm.Jobs(ctx)
	if partitionErr != nil && jobErr != nil {
		return partitionErr
	}
	counts := map[string]currentJobSummary{}
	for _, job := range jobs {
		queue := strings.TrimSpace(job.Partition)
		if queue == "" {
			continue
		}
		summary := counts[queue]
		switch normalizeSlurmState(job.State) {
		case "running":
			summary.Running++
		case "pending":
			summary.Pending++
		}
		counts[queue] = summary
	}
	seen := map[string]struct{}{}
	queues := make([]string, 0, len(partitions)+len(counts))
	for _, partition := range partitions {
		queue := strings.TrimSpace(partition.Name)
		if queue == "" {
			continue
		}
		if _, ok := seen[queue]; ok {
			continue
		}
		seen[queue] = struct{}{}
		queues = append(queues, queue)
	}
	for queue := range counts {
		if _, ok := seen[queue]; !ok {
			seen[queue] = struct{}{}
			queues = append(queues, queue)
		}
	}
	for _, queue := range queues {
		summary := counts[queue]
		if _, err := s.DB.ExecContext(ctx, `
INSERT INTO queue_job_trend_samples (queue_name, running_count, pending_count, total_count)
VALUES ($1,$2,$3,$4)
`, queue, summary.Running, summary.Pending, summary.Running+summary.Pending); err != nil {
			return err
		}
	}
	return nil
}

func normalizeSlurmState(value string) string {
	state := strings.ToUpper(strings.TrimSpace(value))
	switch {
	case state == "RUNNING" || state == "R" || strings.HasPrefix(state, "RUNNING"):
		return "running"
	case state == "PENDING" || state == "PD" || strings.HasPrefix(state, "PENDING"):
		return "pending"
	default:
		return ""
	}
}

func (s *Services) globalQueueJobTrendPoints(ctx context.Context, queue string, config dashboardTrendConfig) ([]QueueJobTrendPoint, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT
  date_bin($1::interval, sample_time, TIMESTAMPTZ '1970-01-01 00:00:00+00') AS bucket_at,
  ROUND(AVG(running_count))::integer AS running,
  ROUND(AVG(pending_count))::integer AS pending
FROM queue_job_trend_samples
WHERE queue_name = $2
  AND sample_time >= now() - $3::interval
GROUP BY bucket_at
ORDER BY bucket_at ASC
LIMIT 500`, config.Bucket, queue, config.Window)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	points := []QueueJobTrendPoint{}
	for rows.Next() {
		var point QueueJobTrendPoint
		var bucket time.Time
		if err := rows.Scan(&bucket, &point.Running, &point.Pending); err != nil {
			return points, err
		}
		point.Time = bucket.Format(time.RFC3339)
		points = append(points, point)
	}
	return points, rows.Err()
}

func (s *Services) dashboardAlerts(ctx context.Context) ([]DashboardAlert, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT id, level, status, title, message, source, occurred_at,
       acknowledged_by, acknowledged_at
FROM dashboard_alerts
ORDER BY occurred_at DESC
LIMIT 10`)
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
			return items, err
		}
		item.OccurredAt = occurred.Format(time.RFC3339)
		if acknowledged.Valid {
			item.AcknowledgedAt = acknowledged.Time.Format(time.RFC3339)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func roundedPercent(used, total int) *float64 {
	if total <= 0 {
		return nil
	}
	value := math.Round(float64(used)/float64(total)*1000) / 10
	return &value
}

func countNodeExpressionLocal(value string) int {
	value = strings.TrimSpace(strings.TrimSuffix(value, "*"))
	if value == "" {
		return 0
	}
	total := 0
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "[") && strings.Contains(part, "]") {
			prefix := part[:strings.Index(part, "[")]
			body := part[strings.Index(part, "[")+1 : strings.LastIndex(part, "]")]
			for _, segment := range strings.Split(body, ",") {
				segment = strings.TrimSpace(segment)
				if strings.Contains(segment, "-") {
					bounds := strings.SplitN(segment, "-", 2)
					start, _ := strconv.Atoi(strings.TrimLeft(bounds[0], "0"))
					end, _ := strconv.Atoi(strings.TrimLeft(bounds[1], "0"))
					if end >= start {
						total += end - start + 1
					}
					continue
				}
				if prefix != "" || segment != "" {
					total++
				}
			}
			continue
		}
		total++
	}
	return total
}

func gresCountLocal(value string) int {
	total := 0
	for _, match := range regexp.MustCompile(`(?i)gpu(?::[^:,]+)?:(\d+)`).FindAllStringSubmatch(value, -1) {
		if len(match) > 1 {
			total += atoiDefault(match[1])
		}
	}
	return total
}

func expandNodeExpressionApprox(value string) []string {
	count := countNodeExpressionLocal(value)
	if count == 0 {
		return nil
	}
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, value+"#"+strconv.Itoa(i))
	}
	return out
}
