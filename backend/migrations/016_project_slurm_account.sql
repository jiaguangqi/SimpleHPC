ALTER TABLE projects ADD COLUMN IF NOT EXISTS slurm_account TEXT NOT NULL DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS slurm_parent_account TEXT NOT NULL DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS slurm_qos TEXT NOT NULL DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS slurm_sync_enabled BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS slurm_sync_status TEXT NOT NULL DEFAULT 'pending';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS slurm_sync_message TEXT NOT NULL DEFAULT '';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS slurm_synced_at TIMESTAMPTZ;
UPDATE projects SET slurm_account = code WHERE slurm_account = '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_slurm_account ON projects(slurm_account) WHERE slurm_account <> '';

ALTER TABLE project_members ADD COLUMN IF NOT EXISTS default_project BOOLEAN NOT NULL DEFAULT false;
CREATE UNIQUE INDEX IF NOT EXISTS idx_project_members_default_project
  ON project_members(username)
  WHERE default_project AND status='active';

ALTER TABLE project_job_links ADD COLUMN IF NOT EXISTS account TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_project_job_links_account ON project_job_links(account);

ALTER TABLE job_template_runs ADD COLUMN IF NOT EXISTS project_id BIGINT REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE job_template_runs ADD COLUMN IF NOT EXISTS project_code TEXT NOT NULL DEFAULT '';
ALTER TABLE job_template_runs ADD COLUMN IF NOT EXISTS project_name TEXT NOT NULL DEFAULT '';
ALTER TABLE job_template_runs ADD COLUMN IF NOT EXISTS slurm_account TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_job_template_runs_project ON job_template_runs(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_job_template_runs_account ON job_template_runs(slurm_account);

ALTER TABLE slurm_jobs ADD COLUMN IF NOT EXISTS account TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_slurm_jobs_account ON slurm_jobs(account);

WITH project_account_permissions(permission_key,permission_type,module_code,resource_code,action_code,name,sort_order) AS (
  VALUES
    ('api.projects.sync','api','projects','projects','sync','同步项目 Slurm Account',307),
    ('api.projects.set_default_project','api','projects','projects','set_default_project','设置默认项目',308)
)
INSERT INTO permissions (permission_key,permission_type,module_code,resource_code,action_code,name,sort_order)
SELECT permission_key,permission_type,module_code,resource_code,action_code,name,sort_order
FROM project_account_permissions
ON CONFLICT(permission_key) DO UPDATE SET
  permission_type=EXCLUDED.permission_type,
  module_code=EXCLUDED.module_code,
  resource_code=EXCLUDED.resource_code,
  action_code=EXCLUDED.action_code,
  name=EXCLUDED.name,
  sort_order=EXCLUDED.sort_order,
  status='active',
  updated_at=now();

INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'migration-016-project-slurm-account'
FROM roles r
JOIN permissions p ON p.permission_key IN (
  'api.projects.sync',
  'api.projects.set_default_project'
)
WHERE r.code IN ('cluster_admin','unit_admin','team_admin','user')
ON CONFLICT(role_id,permission_id) DO NOTHING;
