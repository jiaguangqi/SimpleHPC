package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"

	"simplehpc/backend/internal/config"
	"simplehpc/backend/internal/integrations/ldap"
	"simplehpc/backend/internal/integrations/slurm"
	"simplehpc/backend/internal/integrations/storage"
)

type Services struct {
	Config  config.Config
	DB      *sql.DB
	Redis   *redis.Client
	LDAP    *ldap.Client
	Slurm   *slurm.Client
	Storage *storage.Client
}

type SyncedJob struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	User           string `json:"user"`
	Account        string `json:"account"`
	ProjectID      int64  `json:"projectId,omitempty"`
	Project        string `json:"project,omitempty"`
	Partition      string `json:"partition"`
	State          string `json:"state"`
	Nodes          string `json:"nodes"`
	CPUs           string `json:"cpus"`
	GPUs           string `json:"gpus"`
	Time           string `json:"time"`
	ElapsedSeconds int64  `json:"elapsedSeconds,omitempty"`
	CPUTimeSeconds int64  `json:"cpuTimeSeconds,omitempty"`
	AllocTRES      string `json:"allocTres,omitempty"`
	Submit         string `json:"submit,omitempty"`
	NodeList       string `json:"nodeList,omitempty"`
	Source         string `json:"source,omitempty"`
	SyncedAt       string `json:"syncedAt,omitempty"`
}

type JobQuery struct {
	Page      int
	PageSize  int
	Offset    int
	Status    string
	Keyword   string
	Username  string
	Account   string
	ProjectID int64
	Group     string
	Partition string
	UnitIDs   []string
	TeamIDs   []string
	DenyAll   bool
}

type JobPage struct {
	Items      []SyncedJob `json:"items"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	TotalPages int         `json:"totalPages"`
	Source     string      `json:"source"`
}

func New(cfg config.Config) (*Services, error) {
	var db *sql.DB
	if cfg.DatabaseURL != "" {
		var err error
		db, err = sql.Open("pgx", cfg.DatabaseURL)
		if err != nil {
			return nil, err
		}
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(30 * time.Minute)
		if err := ensureSchema(context.Background(), db); err != nil {
			return nil, err
		}
		migrations, err := embeddedMigrations()
		if err != nil {
			return nil, err
		}
		if err := runMigrations(context.Background(), db, migrations); err != nil {
			return nil, err
		}
	}

	var redisClient *redis.Client
	if cfg.RedisURL != "" {
		opts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			return nil, err
		}
		redisClient = redis.NewClient(opts)
	}

	services := &Services{
		Config:  cfg,
		DB:      db,
		Redis:   redisClient,
		LDAP:    ldap.New(cfg.LDAPURL, cfg.LDAPBaseDN, cfg.LDAPAdminDN, cfg.LDAPAdminPassword),
		Slurm:   slurm.New(cfg.SlurmBinDir, cfg.SlurmConfigPath, cfg.SlurmDefaultAccount, cfg.SlurmDefaultPartition),
		Storage: storage.New(cfg.StorageRoots),
	}
	if services.DB != nil {
		if err := services.syncBuiltinNoVNCTemplate(context.Background()); err != nil {
			return nil, err
		}
	}
	services.ApplySavedRuntimeConfig(context.Background())
	return services, nil
}

func ensureSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS slurm_jobs (
  job_id TEXT PRIMARY KEY,
  name TEXT NOT NULL DEFAULT '',
  user_name TEXT NOT NULL DEFAULT '',
  account TEXT NOT NULL DEFAULT '',
  partition TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL DEFAULT '',
  node_count INTEGER NOT NULL DEFAULT 0,
  cpu_count INTEGER NOT NULL DEFAULT 0,
  gpu_count INTEGER NOT NULL DEFAULT 0,
  runtime TEXT NOT NULL DEFAULT '',
  node_list TEXT NOT NULL DEFAULT '',
  submit_time TEXT NOT NULL DEFAULT '',
  start_time TEXT NOT NULL DEFAULT '',
  end_time TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT 'slurm',
  raw JSONB NOT NULL DEFAULT '{}'::jsonb,
  synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE slurm_jobs ADD COLUMN IF NOT EXISTS account TEXT NOT NULL DEFAULT '';
ALTER TABLE slurm_jobs ADD COLUMN IF NOT EXISTS elapsed_seconds BIGINT NOT NULL DEFAULT 0;
ALTER TABLE slurm_jobs ADD COLUMN IF NOT EXISTS cpu_time_seconds BIGINT NOT NULL DEFAULT 0;
ALTER TABLE slurm_jobs ADD COLUMN IF NOT EXISTS alloc_tres TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_slurm_jobs_synced_at ON slurm_jobs (synced_at DESC);
CREATE INDEX IF NOT EXISTS idx_slurm_jobs_state ON slurm_jobs (state);
CREATE INDEX IF NOT EXISTS idx_slurm_jobs_account ON slurm_jobs (account);
CREATE TABLE IF NOT EXISTS dashboard_resource_samples (
  id BIGSERIAL PRIMARY KEY,
  sampled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  running_jobs INTEGER NOT NULL DEFAULT 0,
  pending_jobs INTEGER NOT NULL DEFAULT 0,
  total_jobs INTEGER NOT NULL DEFAULT 0,
  total_users INTEGER NOT NULL DEFAULT 0,
  total_nodes INTEGER NOT NULL DEFAULT 0,
  total_cpus INTEGER NOT NULL DEFAULT 0,
  total_gpus INTEGER NOT NULL DEFAULT 0,
  allocated_cpus INTEGER NOT NULL DEFAULT 0,
  allocated_gpus INTEGER NOT NULL DEFAULT 0,
  cpu_usage_percent NUMERIC,
  gpu_usage_percent NUMERIC,
  storage JSONB NOT NULL DEFAULT '[]'::jsonb
);
CREATE INDEX IF NOT EXISTS idx_dashboard_resource_samples_sampled_at ON dashboard_resource_samples (sampled_at DESC);
CREATE TABLE IF NOT EXISTS queue_job_trend_samples (
  id BIGSERIAL PRIMARY KEY,
  queue_name TEXT NOT NULL DEFAULT '',
  sample_time TIMESTAMPTZ NOT NULL DEFAULT now(),
  running_count INTEGER NOT NULL DEFAULT 0,
  pending_count INTEGER NOT NULL DEFAULT 0,
  total_count INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_queue_job_trend_samples_queue_time ON queue_job_trend_samples (queue_name, sample_time DESC);
CREATE TABLE IF NOT EXISTS dashboard_alerts (
  id BIGSERIAL PRIMARY KEY,
  level TEXT NOT NULL DEFAULT 'info',
  status TEXT NOT NULL DEFAULT 'active',
  title TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT '',
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ
);
ALTER TABLE dashboard_alerts ADD COLUMN IF NOT EXISTS acknowledged_by TEXT NOT NULL DEFAULT '';
ALTER TABLE dashboard_alerts ADD COLUMN IF NOT EXISTS acknowledged_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_dashboard_alerts_occurred_at ON dashboard_alerts (occurred_at DESC);
CREATE TABLE IF NOT EXISTS inspection_runs (
  id BIGSERIAL PRIMARY KEY,
  run_id TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL,
  result JSONB NOT NULL DEFAULT '{}'::jsonb,
  checks JSONB NOT NULL DEFAULT '[]'::jsonb,
  problem_count INTEGER NOT NULL DEFAULT 0,
  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE inspection_runs ADD COLUMN IF NOT EXISTS result JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE inspection_runs ALTER COLUMN result SET DEFAULT '{}'::jsonb;
ALTER TABLE inspection_runs ADD COLUMN IF NOT EXISTS checks JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE inspection_runs ADD COLUMN IF NOT EXISTS problem_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE inspection_runs ADD COLUMN IF NOT EXISTS created_by TEXT NOT NULL DEFAULT 'system';
ALTER TABLE inspection_runs ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ;
ALTER TABLE inspection_runs ADD COLUMN IF NOT EXISTS finished_at TIMESTAMPTZ;
ALTER TABLE inspection_runs ADD COLUMN IF NOT EXISTS duration_ms BIGINT NOT NULL DEFAULT 0;
ALTER TABLE inspection_runs ADD COLUMN IF NOT EXISTS cluster_name TEXT NOT NULL DEFAULT '';
ALTER TABLE inspection_runs ADD COLUMN IF NOT EXISTS summary JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE inspection_runs ADD COLUMN IF NOT EXISTS report_html TEXT NOT NULL DEFAULT '';
ALTER TABLE inspection_runs ADD COLUMN IF NOT EXISTS detail_log TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_inspection_runs_created_at ON inspection_runs (created_at DESC);
CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGSERIAL PRIMARY KEY,
  actor TEXT NOT NULL,
  actor_type TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL,
  target_type TEXT NOT NULL DEFAULT '',
  target TEXT NOT NULL DEFAULT '',
  target_id TEXT NOT NULL DEFAULT '',
  result TEXT NOT NULL,
  detail JSONB NOT NULL DEFAULT '{}'::jsonb,
  request_id TEXT NOT NULL DEFAULT '',
  ip_address TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS actor_type TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS target TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS target_id TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS ip_address TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_logs ALTER COLUMN target_type SET DEFAULT '';
ALTER TABLE audit_logs ALTER COLUMN target_id SET DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs (created_at DESC);
CREATE TABLE IF NOT EXISTS auth_events (
  id BIGSERIAL PRIMARY KEY,
  username TEXT NOT NULL DEFAULT '',
  display_name TEXT NOT NULL DEFAULT '',
  account_type TEXT NOT NULL DEFAULT '',
  event TEXT NOT NULL,
  result TEXT NOT NULL,
  ip_address TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_auth_events_created_at ON auth_events (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_auth_events_username ON auth_events (username, created_at DESC);
CREATE TABLE IF NOT EXISTS storage_acls (
  id BIGSERIAL PRIMARY KEY,
  subject_type TEXT NOT NULL,
  subject_name TEXT NOT NULL,
  path TEXT NOT NULL,
  permission TEXT NOT NULL,
  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(subject_type,subject_name,path)
);
CREATE TABLE IF NOT EXISTS system_configs (
  key TEXT PRIMARY KEY,
  value JSONB NOT NULL DEFAULT '{}'::jsonb,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS job_templates (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL DEFAULT 'batch',
  status TEXT NOT NULL DEFAULT 'draft',
  version INTEGER NOT NULL DEFAULT 1,
  form_schema JSONB NOT NULL DEFAULT '[]'::jsonb,
  script_template TEXT NOT NULL DEFAULT '',
  runtime_config JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_by TEXT NOT NULL DEFAULT 'system',
  updated_by TEXT NOT NULL DEFAULT 'system',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT '';
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'batch';
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'draft';
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS form_schema JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS runtime_config JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS created_by TEXT NOT NULL DEFAULT 'system';
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS updated_by TEXT NOT NULL DEFAULT 'system';
ALTER TABLE job_templates ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE TABLE IF NOT EXISTS job_template_grants (
  id BIGSERIAL PRIMARY KEY,
  template_id BIGINT NOT NULL REFERENCES job_templates(id) ON DELETE CASCADE,
  target_type TEXT NOT NULL,
  target_id TEXT NOT NULL DEFAULT '*',
  granted_by TEXT NOT NULL DEFAULT 'system',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(template_id,target_type,target_id)
);
CREATE TABLE IF NOT EXISTS job_template_access_requests (
  id BIGSERIAL PRIMARY KEY,
  template_id BIGINT NOT NULL REFERENCES job_templates(id) ON DELETE CASCADE,
  username TEXT NOT NULL,
  reason TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  reviewed_by TEXT NOT NULL DEFAULT '',
  reviewed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS job_template_runs (
  id BIGSERIAL PRIMARY KEY,
  template_id BIGINT NOT NULL REFERENCES job_templates(id),
  template_version INTEGER NOT NULL,
  template_name TEXT NOT NULL,
  username TEXT NOT NULL,
  kind TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'submitting',
  slurm_job_id TEXT NOT NULL DEFAULT '',
  submitted_values JSONB NOT NULL DEFAULT '{}'::jsonb,
  rendered_script TEXT NOT NULL,
  access_token TEXT NOT NULL DEFAULT '',
  target_node TEXT NOT NULL DEFAULT '',
  target_port INTEGER NOT NULL DEFAULT 0,
  protocol TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO job_templates(name,description,category,kind,status,form_schema,script_template,runtime_config,created_by,updated_by)
VALUES
('Shell 非交互式作业','提交普通 Shell、Python、MPI 等非交互式计算任务','通用计算','batch','published',
'[{"id":"command","type":"textarea","label":"执行命令","variable":"JOB_COMMAND","required":true,"placeholder":"python train.py --input data.csv"}]'::jsonb,
'bash -lc "$JOB_COMMAND"','{}'::jsonb,'system','system'),
('noVNC Linux 桌面','在计算节点启动 TigerVNC 或 TurboVNC 桌面会话','交互式桌面','novnc','draft',
'[{"id":"desktop","type":"select","label":"桌面环境","variable":"DESKTOP","required":true,"default":"gnome","options":[{"label":"GNOME","value":"gnome"},{"label":"KDE","value":"kde"}]},{"id":"resolution","type":"select","label":"分辨率","variable":"RESOLUTION","default":"1920x1080","options":[{"label":"1920 x 1080","value":"1920x1080"},{"label":"2560 x 1440","value":"2560x1440"}]}]'::jsonb,
'command -v websockify >/dev/null || { echo "websockify 未安装" >&2; exit 127; }
VNC_BIN="$(command -v vncserver || true)"
[ -n "$VNC_BIN" ] || VNC_BIN="/opt/TurboVNC/bin/vncserver"
[ -n "$VNC_BIN" ] || { echo "TigerVNC/TurboVNC 未安装" >&2; exit 127; }
VNC_PASSWD_BIN="$(command -v vncpasswd || true)"
[ -n "$VNC_PASSWD_BIN" ] || VNC_PASSWD_BIN="/opt/TurboVNC/bin/vncpasswd"
[ -x "$VNC_BIN" ] && [ -x "$VNC_PASSWD_BIN" ] || { echo "VNC 服务端组件不完整" >&2; exit 127; }
DISPLAY_NUM=$((SLURM_JOB_ID % 500 + 100))
VNC_PORT=$((5900 + DISPLAY_NUM))
WS_PORT=$((6100 + DISPLAY_NUM))
mkdir -p "$HOME/.vnc"
printf "%s\n" "$SIMPLEHPC_ACCESS_TOKEN" | "$VNC_PASSWD_BIN" -f > "$HOME/.vnc/passwd"
chmod 600 "$HOME/.vnc/passwd"
if [ "$DESKTOP" = "kde" ]; then SESSION=startplasma-x11; else SESSION=gnome-session; fi
printf "#!/bin/sh\nexec %s\n" "$SESSION" > "$HOME/.vnc/xstartup"
chmod 700 "$HOME/.vnc/xstartup"
cleanup() { "$VNC_BIN" -kill ":$DISPLAY_NUM" >/dev/null 2>&1 || true; }
trap cleanup EXIT TERM INT
"$VNC_BIN" ":$DISPLAY_NUM" -geometry "$RESOLUTION" -SecurityTypes VncAuth
websockify --web=/opt/noVNC "$WS_PORT" "127.0.0.1:$VNC_PORT" &
curl -fsS -X POST -H "Content-Type: application/json" \
  -d "{\"node\":\"$(hostname -f)\",\"port\":$WS_PORT,\"protocol\":\"vnc\"}" "$SIMPLEHPC_CALLBACK_URL"
wait','{"desktop":"gnome","vncBackend":"turbovnc","resolution":"1920x1080","protocol":"vnc"}'::jsonb,'system','system'),
('Jupyter Web 应用','在计算节点启动 JupyterLab 并通过 simpleHPC 网关访问','Web 应用','webapp','draft',
'[{"id":"notebook_dir","type":"directory","label":"工作目录","variable":"NOTEBOOK_DIR","required":true}]'::jsonb,
'command -v jupyter >/dev/null || { echo "JupyterLab 未安装" >&2; exit 127; }
APP_PORT=$((20000 + SLURM_JOB_ID % 20000))
jupyter lab --no-browser --ip=0.0.0.0 --port="$APP_PORT" --ServerApp.token="$SIMPLEHPC_ACCESS_TOKEN" --notebook-dir="$NOTEBOOK_DIR" &
APP_PID=$!
for attempt in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:$APP_PORT/api" >/dev/null 2>&1; then break; fi
  sleep 1
done
kill -0 "$APP_PID"
curl -fsS -X POST -H "Content-Type: application/json" \
  -d "{\"node\":\"$(hostname -f)\",\"port\":$APP_PORT,\"protocol\":\"http\"}" "$SIMPLEHPC_CALLBACK_URL"
wait "$APP_PID"',
'{"launchCommand":"jupyter lab","protocol":"http","readinessPath":"/lab"}'::jsonb,'system','system')
,
('VS Code Web 应用','在计算节点启动 Code Server 并通过 simpleHPC 网关访问','Web 应用','webapp','draft',
'[{"id":"workspace_dir","type":"directory","label":"工作目录","variable":"WORKSPACE_DIR","required":true}]'::jsonb,
'command -v code-server >/dev/null || { echo "Code Server 未安装" >&2; exit 127; }
APP_PORT=$((30000 + SLURM_JOB_ID % 20000))
export PASSWORD="$SIMPLEHPC_ACCESS_TOKEN"
code-server --bind-addr "0.0.0.0:$APP_PORT" --auth password "$WORKSPACE_DIR" &
APP_PID=$!
for attempt in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:$APP_PORT/healthz" >/dev/null 2>&1; then break; fi
  sleep 1
done
kill -0 "$APP_PID"
curl -fsS -X POST -H "Content-Type: application/json" \
  -d "{\"node\":\"$(hostname -f)\",\"port\":$APP_PORT,\"protocol\":\"http\"}" "$SIMPLEHPC_CALLBACK_URL"
wait "$APP_PID"',
'{"launchCommand":"code-server","protocol":"http","readinessPath":"/healthz"}'::jsonb,'system','system')
ON CONFLICT(name) DO NOTHING;
INSERT INTO job_template_grants(template_id,target_type,target_id,granted_by)
SELECT id,'all','*','system' FROM job_templates WHERE name='Shell 非交互式作业'
ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS units (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  code TEXT UNIQUE,
  admin_username TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  source TEXT NOT NULL DEFAULT 'project',
  ldap_dn TEXT NOT NULL DEFAULT '',
  synced_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE units ADD COLUMN IF NOT EXISTS admin_username TEXT NOT NULL DEFAULT '';
ALTER TABLE units ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE units ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'project';
ALTER TABLE units ADD COLUMN IF NOT EXISTS ldap_dn TEXT NOT NULL DEFAULT '';
ALTER TABLE units ADD COLUMN IF NOT EXISTS synced_at TIMESTAMPTZ;
ALTER TABLE units ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE units ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE TABLE IF NOT EXISTS teams (
  id BIGSERIAL PRIMARY KEY,
  unit_id BIGINT REFERENCES units(id),
  name TEXT NOT NULL,
  group_name TEXT NOT NULL DEFAULT '',
  leader_username TEXT NOT NULL DEFAULT '',
  member_count INTEGER NOT NULL DEFAULT 0,
  resource_policy TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  source TEXT NOT NULL DEFAULT 'ldap',
  ldap_dn TEXT NOT NULL DEFAULT '',
  synced_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(unit_id, name)
);
ALTER TABLE teams ADD COLUMN IF NOT EXISTS group_name TEXT NOT NULL DEFAULT '';
ALTER TABLE teams ADD COLUMN IF NOT EXISTS member_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE teams ADD COLUMN IF NOT EXISTS resource_policy TEXT NOT NULL DEFAULT '';
ALTER TABLE teams ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE teams ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'ldap';
ALTER TABLE teams ADD COLUMN IF NOT EXISTS ldap_dn TEXT NOT NULL DEFAULT '';
ALTER TABLE teams ADD COLUMN IF NOT EXISTS synced_at TIMESTAMPTZ;
ALTER TABLE teams ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE teams ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE TABLE IF NOT EXISTS platform_users (
  id BIGSERIAL PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL DEFAULT '',
  email TEXT NOT NULL DEFAULT '',
  phone TEXT NOT NULL DEFAULT '',
  unit_id BIGINT REFERENCES units(id),
  team_id BIGINT REFERENCES teams(id),
  ldap_dn TEXT NOT NULL DEFAULT '',
  uid_number INTEGER,
  gid_number INTEGER,
  home_directory TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  source TEXT NOT NULL DEFAULT 'ldap',
  synced_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS phone TEXT NOT NULL DEFAULT '';
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'ldap';
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS synced_at TIMESTAMPTZ;
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE platform_users ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS roles (
  id BIGSERIAL PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  scope_type TEXT NOT NULL DEFAULT 'global',
  permission_summary TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE roles ADD COLUMN IF NOT EXISTS permission_summary TEXT NOT NULL DEFAULT '';
ALTER TABLE roles ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE TABLE IF NOT EXISTS user_roles (
  user_id BIGINT NOT NULL REFERENCES platform_users(id) ON DELETE CASCADE,
  role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  scope_type TEXT NOT NULL DEFAULT 'global',
  scope_id TEXT NOT NULL DEFAULT '*',
  PRIMARY KEY(user_id, role_id, scope_type, scope_id)
);

CREATE TABLE IF NOT EXISTS admin_users (
  id BIGSERIAL PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  role_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  email TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL DEFAULT 'system',
  last_login TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE admin_users ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE admin_users ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ;

INSERT INTO roles(code, name, scope_type, permission_summary) VALUES
  ('cluster_admin', '集群管理员', 'global', '全模块管理权限'),
  ('config_admin', '配置管理员', 'global', 'LDAP、Slurm、存储、通知配置'),
  ('unit_admin', '学院单位管理员', 'unit', '本单位用户、团队和资源查看'),
  ('team_admin', '团队管理员', 'team', '团队成员、模板授权和团队作业'),
  ('user', '用户', 'self', '个人作业、个人数据和模板提交')
ON CONFLICT (code) DO UPDATE SET
  name = EXCLUDED.name,
  scope_type = EXCLUDED.scope_type,
  permission_summary = EXCLUDED.permission_summary;
`)
	return err
}

func (s *Services) GetSystemConfig(ctx context.Context, key string) (map[string]any, bool, error) {
	if s.DB == nil {
		return map[string]any{}, false, nil
	}
	var raw []byte
	err := s.DB.QueryRowContext(ctx, `SELECT value FROM system_configs WHERE key = $1`, key).Scan(&raw)
	if err == sql.ErrNoRows {
		return map[string]any{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	value := map[string]any{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func (s *Services) SetSystemConfig(ctx context.Context, key string, value map[string]any) error {
	if s.DB == nil {
		return errNotConfigured("postgres")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `
INSERT INTO system_configs (key, value, updated_at)
VALUES ($1, $2::jsonb, now())
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()
`, key, string(raw))
	return err
}

func (s *Services) ApplySavedRuntimeConfig(ctx context.Context) {
	if s.DB == nil {
		return
	}
	if cfg, ok, err := s.GetSystemConfig(ctx, "slurm"); err == nil && ok {
		if value, _ := cfg["binDir"].(string); strings.TrimSpace(value) != "" {
			s.Slurm.BinDir = strings.TrimSpace(value)
		}
		if value, _ := cfg["clusterName"].(string); strings.TrimSpace(value) != "" {
			s.Slurm.DefaultAccount = strings.TrimSpace(value)
		}
	}
	if cfg, ok, err := s.GetSystemConfig(ctx, "ldap"); err == nil && ok {
		if value, _ := cfg["url"].(string); strings.TrimSpace(value) != "" {
			s.LDAP.URL = strings.TrimSpace(value)
		}
		if value, _ := cfg["baseDN"].(string); strings.TrimSpace(value) != "" {
			s.LDAP.BaseDN = strings.TrimSpace(value)
		}
		if value, _ := cfg["bindDN"].(string); strings.TrimSpace(value) != "" {
			s.LDAP.AdminDN = strings.TrimSpace(value)
		}
		if value, _ := cfg["bindPassword"].(string); strings.TrimSpace(value) != "" {
			s.LDAP.AdminPassword = value
		}
	}
	if cfg, ok, err := s.GetSystemConfig(ctx, "storage"); err == nil && ok {
		if values, ok := cfg["roots"].([]any); ok {
			roots := make([]string, 0, len(values))
			for _, item := range values {
				if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
					roots = append(roots, strings.TrimSpace(value))
				}
			}
			if len(roots) > 0 {
				s.Storage.Roots = roots
			}
		}
	}
}

func (s *Services) StartSlurmJobSync(ctx context.Context) {
	if s.DB == nil || s.Slurm == nil {
		return
	}
	go func() {
		s.syncOnce(ctx)
		recentTicker := time.NewTicker(10 * time.Second)
		fullTicker := time.NewTicker(5 * time.Minute)
		defer recentTicker.Stop()
		defer fullTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-recentTicker.C:
				recentCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
				if err := s.SyncRecentSlurmJobs(recentCtx); err != nil {
					log.Printf("sync recent slurm jobs: %v", err)
				}
				cancel()
			case <-fullTicker.C:
				s.syncOnce(ctx)
			}
		}
	}()
}

func (s *Services) syncOnce(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()
	if err := s.SyncSlurmJobs(ctx); err != nil {
		log.Printf("sync slurm jobs: %v", err)
	}
}

func (s *Services) SyncSlurmJobs(ctx context.Context) error {
	return s.syncSlurmJobsSince(ctx, time.Now().AddDate(0, 0, -90).Format("2006-01-02"))
}

func (s *Services) SyncRecentSlurmJobs(ctx context.Context) error {
	return s.syncSlurmJobsSince(ctx, "today")
}

func (s *Services) syncSlurmJobsSince(ctx context.Context, historySince string) error {
	current, currentErr := s.Slurm.Jobs(ctx)
	history, historyErr := s.Slurm.History(ctx, historySince)
	if currentErr != nil && historyErr != nil {
		return currentErr
	}
	for _, job := range history {
		raw, _ := json.Marshal(job)
		if err := s.upsertJob(ctx, SyncedJob{
			ID: job.ID, Name: job.Name, User: job.User, Account: job.Account, Partition: job.Partition, State: job.State,
			Nodes: job.Nodes, CPUs: job.CPUs, GPUs: job.GPUs, Time: job.Elapsed,
			ElapsedSeconds: job.ElapsedSeconds, CPUTimeSeconds: job.CPUTimeSeconds, AllocTRES: job.AllocTRES,
			Submit: job.Submit, NodeList: job.NodeList, Source: "slurmdbd",
		}, string(raw), job.Start, job.End); err != nil {
			return err
		}
	}
	for _, job := range current {
		elapsedSeconds := parseSlurmRuntimeSeconds(job.Time)
		raw, _ := json.Marshal(job)
		if err := s.upsertJob(ctx, SyncedJob{
			ID: job.ID, Name: job.Name, User: job.User, Account: job.Account, Partition: job.Partition, State: job.State,
			Nodes: job.Nodes, CPUs: job.CPUs, GPUs: job.GPUs, Time: job.Time,
			ElapsedSeconds: elapsedSeconds, CPUTimeSeconds: elapsedSeconds * int64(atoiDefault(job.CPUs)),
			Submit: job.Submit, NodeList: job.NodeList, Source: "squeue",
		}, string(raw), "", ""); err != nil {
			return err
		}
	}
	if currentErr == nil && historyErr == nil {
		removed, err := s.deleteStaleQueueSnapshots(ctx, current)
		if err != nil {
			return err
		}
		if removed > 0 {
			log.Printf("removed %d stale squeue job snapshots", removed)
		}
	}
	if currentErr != nil {
		return currentErr
	}
	return historyErr
}

func (s *Services) SyncCurrentSlurmJobs(ctx context.Context) error {
	current, err := s.Slurm.Jobs(ctx)
	if err != nil {
		return err
	}
	for _, job := range current {
		elapsedSeconds := parseSlurmRuntimeSeconds(job.Time)
		raw, _ := json.Marshal(job)
		if err := s.upsertJob(ctx, SyncedJob{
			ID: job.ID, Name: job.Name, User: job.User, Account: job.Account, Partition: job.Partition, State: job.State,
			Nodes: job.Nodes, CPUs: job.CPUs, GPUs: job.GPUs, Time: job.Time,
			ElapsedSeconds: elapsedSeconds, CPUTimeSeconds: elapsedSeconds * int64(atoiDefault(job.CPUs)),
			Submit: job.Submit, NodeList: job.NodeList, Source: "squeue",
		}, string(raw), "", ""); err != nil {
			return err
		}
	}
	return nil
}

func staleQueueSnapshotDeleteStatement(current []slurm.Job) (string, []any) {
	query := `
DELETE FROM slurm_jobs
WHERE source = 'squeue'
  AND (
    upper(state) IN ('PENDING','PD','RUNNING','R','CONFIGURING','CF','COMPLETING','CG','SUSPENDED','S')
    OR upper(state) LIKE 'PENDING%'
    OR upper(state) LIKE 'RUNNING%'
  )`
	seen := make(map[string]struct{}, len(current))
	args := make([]any, 0, len(current))
	placeholders := make([]string, 0, len(current))
	for _, job := range current {
		id := strings.TrimSpace(job.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		args = append(args, id)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}
	if len(placeholders) > 0 {
		query += "\n  AND job_id NOT IN (" + strings.Join(placeholders, ",") + ")"
	}
	return query, args
}

func (s *Services) deleteStaleQueueSnapshots(ctx context.Context, current []slurm.Job) (int64, error) {
	query, args := staleQueueSnapshotDeleteStatement(current)
	result, err := s.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Services) upsertJob(ctx context.Context, job SyncedJob, raw, start, end string) error {
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO slurm_jobs (
  job_id, name, user_name, account, partition, state, node_count, cpu_count, gpu_count,
  runtime, elapsed_seconds, cpu_time_seconds, alloc_tres, node_list, submit_time, start_time, end_time, source, raw, synced_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19::jsonb,now(),now())
ON CONFLICT (job_id) DO UPDATE SET
  name = EXCLUDED.name,
  user_name = EXCLUDED.user_name,
  account = EXCLUDED.account,
  partition = EXCLUDED.partition,
  state = EXCLUDED.state,
  node_count = EXCLUDED.node_count,
  cpu_count = EXCLUDED.cpu_count,
  gpu_count = EXCLUDED.gpu_count,
  runtime = EXCLUDED.runtime,
  elapsed_seconds = EXCLUDED.elapsed_seconds,
  cpu_time_seconds = EXCLUDED.cpu_time_seconds,
  alloc_tres = CASE WHEN EXCLUDED.alloc_tres <> '' THEN EXCLUDED.alloc_tres ELSE slurm_jobs.alloc_tres END,
  node_list = EXCLUDED.node_list,
  submit_time = EXCLUDED.submit_time,
  start_time = EXCLUDED.start_time,
  end_time = EXCLUDED.end_time,
  source = EXCLUDED.source,
  raw = EXCLUDED.raw,
  synced_at = now(),
  updated_at = now()
`, job.ID, job.Name, job.User, job.Account, job.Partition, job.State, atoiDefault(job.Nodes), atoiDefault(job.CPUs), atoiDefault(job.GPUs),
		job.Time, job.ElapsedSeconds, job.CPUTimeSeconds, job.AllocTRES, job.NodeList, job.Submit, start, end, job.Source, raw)
	return err
}

func (s *Services) MarkSlurmJobState(ctx context.Context, jobID, state string) error {
	if s.DB == nil {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `
UPDATE slurm_jobs
SET state = $2, synced_at = now(), updated_at = now()
WHERE job_id = $1`, jobID, state)
	return err
}

func (s *Services) SyncedSlurmJobs(ctx context.Context) ([]SyncedJob, error) {
	rows, err := s.DB.QueryContext(ctx, `
SELECT sj.job_id, sj.name, sj.user_name, sj.account, COALESCE(p.id,0), COALESCE(p.name,''), sj.partition, sj.state, sj.node_count, sj.cpu_count, sj.gpu_count,
       sj.runtime, sj.node_list, sj.submit_time, sj.source, sj.synced_at
FROM slurm_jobs sj
LEFT JOIN projects p ON p.slurm_account=sj.account AND sj.account <> ''
ORDER BY sj.synced_at DESC, NULLIF(regexp_replace(sj.job_id, '\D.*$', ''), '')::bigint DESC NULLS LAST, sj.job_id DESC
LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []SyncedJob{}
	for rows.Next() {
		var job SyncedJob
		var nodes, cpus, gpus int
		var synced time.Time
		if err := rows.Scan(&job.ID, &job.Name, &job.User, &job.Account, &job.ProjectID, &job.Project, &job.Partition, &job.State, &nodes, &cpus, &gpus, &job.Time, &job.NodeList, &job.Submit, &job.Source, &synced); err != nil {
			return nil, err
		}
		job.Nodes = strconv.Itoa(nodes)
		job.CPUs = strconv.Itoa(cpus)
		job.GPUs = strconv.Itoa(gpus)
		job.SyncedAt = synced.Format(time.RFC3339)
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func normalizeJobQuery(query JobQuery) JobQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 15
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	query.Keyword = strings.TrimSpace(query.Keyword)
	query.Username = strings.TrimSpace(query.Username)
	query.Account = strings.TrimSpace(query.Account)
	query.Group = strings.TrimSpace(query.Group)
	query.Partition = strings.TrimSpace(query.Partition)
	switch strings.ToUpper(strings.TrimSpace(query.Status)) {
	case "运行中", "RUNNING", "R":
		query.Status = "RUNNING"
	case "排队中", "PENDING", "PD":
		query.Status = "PENDING"
	case "完成", "COMPLETED", "CD":
		query.Status = "COMPLETED"
	case "失败", "FAILED", "F":
		query.Status = "FAILED"
	case "挂起", "SUSPENDED", "S":
		query.Status = "SUSPENDED"
	default:
		query.Status = ""
	}
	query.Offset = (query.Page - 1) * query.PageSize
	return query
}

func ScopeJobQuery(user AuthUser, query JobQuery) JobQuery {
	if user.Type != "admin" {
		query.Username = user.Username
		query.Group = ""
	}
	return query
}

func buildJobWhere(query JobQuery) (string, []any) {
	where := []string{"1=1"}
	args := []any{}
	if query.DenyAll {
		return "1=0", args
	}
	orgWhere := []string{}
	if len(query.UnitIDs) > 0 {
		args = append(args, query.UnitIDs)
		orgWhere = append(orgWhere, fmt.Sprintf(`EXISTS(
SELECT 1 FROM platform_users scope_user
WHERE scope_user.username=sj.user_name
AND scope_user.unit_id::text = ANY($%d::text[])
)`, len(args)))
	}
	if len(query.TeamIDs) > 0 {
		args = append(args, query.TeamIDs)
		orgWhere = append(orgWhere, fmt.Sprintf(`EXISTS(
SELECT 1 FROM platform_users scope_user
WHERE scope_user.username=sj.user_name
AND scope_user.team_id::text = ANY($%d::text[])
)`, len(args)))
	}
	if len(orgWhere) > 0 {
		where = append(where, "("+strings.Join(orgWhere, " OR ")+")")
	}
	if query.Username != "" {
		args = append(args, query.Username)
		where = append(where, fmt.Sprintf("sj.user_name = $%d", len(args)))
	}
	if query.Account != "" {
		args = append(args, query.Account)
		where = append(where, fmt.Sprintf("sj.account = $%d", len(args)))
	}
	if query.ProjectID > 0 {
		args = append(args, query.ProjectID)
		where = append(where, fmt.Sprintf(`sj.account = (
			SELECT slurm_account FROM projects WHERE id = $%d AND slurm_account <> ''
		)`, len(args)))
	}
	if query.Partition != "" {
		args = append(args, query.Partition)
		where = append(where, fmt.Sprintf("sj.partition = $%d", len(args)))
	}
	if query.Group != "" {
		args = append(args, query.Group)
		n := len(args)
		where = append(where, fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM platform_users pu
			JOIN teams t ON t.id = pu.team_id
			WHERE pu.username = sj.user_name
			  AND (t.group_name = $%d OR t.name = $%d)
		)`, n, n))
	}
	if query.Keyword != "" {
		args = append(args, "%"+query.Keyword+"%")
		n := len(args)
		where = append(where, fmt.Sprintf(`(sj.job_id ILIKE $%d OR sj.name ILIKE $%d OR sj.user_name ILIKE $%d OR sj.account ILIKE $%d OR sj.partition ILIKE $%d OR sj.node_list ILIKE $%d)`, n, n, n, n, n, n))
	}
	if query.Status != "" {
		switch query.Status {
		case "RUNNING":
			where = append(where, `(upper(sj.state) IN ('RUNNING','R') OR upper(sj.state) LIKE 'RUNNING%')`)
		case "PENDING":
			where = append(where, `(upper(sj.state) IN ('PENDING','PD') OR upper(sj.state) LIKE 'PENDING%')`)
		case "COMPLETED":
			where = append(where, `(upper(sj.state) IN ('COMPLETED','CD','COMPLETING','CG') OR upper(sj.state) LIKE 'COMPLETED%')`)
		case "SUSPENDED":
			where = append(where, `(upper(sj.state) IN ('SUSPENDED','S') OR upper(sj.state) LIKE 'SUSPENDED%')`)
		case "FAILED":
			where = append(where, `(upper(sj.state) IN ('FAILED','F','CANCELLED','CA','TIMEOUT','TO') OR upper(sj.state) LIKE 'FAILED%' OR upper(sj.state) LIKE 'CANCELLED%' OR upper(sj.state) LIKE 'TIMEOUT%')`)
		}
	}
	return strings.Join(where, " AND "), args
}

func (s *Services) QuerySlurmJobs(ctx context.Context, query JobQuery) (JobPage, error) {
	query = normalizeJobQuery(query)
	whereSQL, args := buildJobWhere(query)
	page := JobPage{Items: []SyncedJob{}, Page: query.Page, PageSize: query.PageSize, Source: "postgres-slurm-sync"}
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM slurm_jobs sj WHERE `+whereSQL, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	page.TotalPages = int(math.Ceil(float64(page.Total) / float64(query.PageSize)))
	if page.TotalPages < 1 {
		page.TotalPages = 1
	}
	if page.Page > page.TotalPages {
		page.Page = page.TotalPages
		query.Offset = (page.Page - 1) * query.PageSize
	}
	args = append(args, query.PageSize, query.Offset)
	limitArg, offsetArg := len(args)-1, len(args)
	rows, err := s.DB.QueryContext(ctx, fmt.Sprintf(`
SELECT sj.job_id, sj.name, sj.user_name, sj.account, COALESCE(p.id,0), COALESCE(p.name,''), sj.partition, sj.state, sj.node_count, sj.cpu_count, sj.gpu_count,
       sj.runtime, sj.node_list, sj.submit_time, sj.source, sj.synced_at
FROM slurm_jobs sj
LEFT JOIN projects p ON p.slurm_account=sj.account AND sj.account <> ''
WHERE %s
ORDER BY NULLIF(regexp_replace(sj.job_id, '\D.*$', ''), '')::bigint DESC NULLS LAST, sj.job_id DESC
LIMIT $%d OFFSET $%d`, whereSQL, limitArg, offsetArg), args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var job SyncedJob
		var nodes, cpus, gpus int
		var synced time.Time
		if err := rows.Scan(&job.ID, &job.Name, &job.User, &job.Account, &job.ProjectID, &job.Project, &job.Partition, &job.State, &nodes, &cpus, &gpus, &job.Time, &job.NodeList, &job.Submit, &job.Source, &synced); err != nil {
			return page, err
		}
		job.Nodes, job.CPUs, job.GPUs = strconv.Itoa(nodes), strconv.Itoa(cpus), strconv.Itoa(gpus)
		job.SyncedAt = synced.Format(time.RFC3339)
		page.Items = append(page.Items, job)
	}
	return page, rows.Err()
}

func atoiDefault(value string) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}

func parseSlurmRuntimeSeconds(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "unknown") || strings.EqualFold(value, "unlimited") {
		return 0
	}
	days := int64(0)
	if parts := strings.SplitN(value, "-", 2); len(parts) == 2 {
		days, _ = strconv.ParseInt(parts[0], 10, 64)
		value = parts[1]
	}
	parts := strings.Split(value, ":")
	values := make([]int64, len(parts))
	for i, part := range parts {
		parsed, err := strconv.ParseInt(part, 10, 64)
		if err != nil || parsed < 0 {
			return 0
		}
		values[i] = parsed
	}
	seconds := days * 24 * 60 * 60
	switch len(values) {
	case 3:
		return seconds + values[0]*3600 + values[1]*60 + values[2]
	case 2:
		return seconds + values[0]*60 + values[1]
	case 1:
		return seconds + values[0]
	default:
		return 0
	}
}

func (s *Services) Close() {
	if s.DB != nil {
		_ = s.DB.Close()
	}
	if s.Redis != nil {
		_ = s.Redis.Close()
	}
}

func (s *Services) CheckPostgres(ctx context.Context) error {
	if s.DB == nil {
		return errNotConfigured("postgres")
	}
	return s.DB.PingContext(ctx)
}

func (s *Services) CheckRedis(ctx context.Context) error {
	if s.Redis == nil {
		return errNotConfigured("redis")
	}
	return s.Redis.Ping(ctx).Err()
}

type errNotConfigured string

func (e errNotConfigured) Error() string {
	return string(e) + " is not configured"
}
