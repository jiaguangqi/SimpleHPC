DELETE FROM role_permissions
WHERE created_by='migration-016-project-slurm-account';

DELETE FROM permissions
WHERE permission_key IN (
  'api.projects.sync',
  'api.projects.set_default_project'
);

DROP INDEX IF EXISTS idx_slurm_jobs_account;
ALTER TABLE slurm_jobs DROP COLUMN IF EXISTS account;

DROP INDEX IF EXISTS idx_job_template_runs_account;
DROP INDEX IF EXISTS idx_job_template_runs_project;
ALTER TABLE job_template_runs DROP COLUMN IF EXISTS slurm_account;
ALTER TABLE job_template_runs DROP COLUMN IF EXISTS project_name;
ALTER TABLE job_template_runs DROP COLUMN IF EXISTS project_code;
ALTER TABLE job_template_runs DROP COLUMN IF EXISTS project_id;

DROP INDEX IF EXISTS idx_project_job_links_account;
ALTER TABLE project_job_links DROP COLUMN IF EXISTS account;

DROP INDEX IF EXISTS idx_project_members_default_project;
ALTER TABLE project_members DROP COLUMN IF EXISTS default_project;

DROP INDEX IF EXISTS idx_projects_slurm_account;
ALTER TABLE projects DROP COLUMN IF EXISTS slurm_synced_at;
ALTER TABLE projects DROP COLUMN IF EXISTS slurm_sync_message;
ALTER TABLE projects DROP COLUMN IF EXISTS slurm_sync_status;
ALTER TABLE projects DROP COLUMN IF EXISTS slurm_sync_enabled;
ALTER TABLE projects DROP COLUMN IF EXISTS slurm_qos;
ALTER TABLE projects DROP COLUMN IF EXISTS slurm_parent_account;
ALTER TABLE projects DROP COLUMN IF EXISTS slurm_account;
