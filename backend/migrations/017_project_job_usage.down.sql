DROP INDEX IF EXISTS idx_slurm_jobs_account_usage;
ALTER TABLE slurm_jobs DROP COLUMN IF EXISTS alloc_tres;
ALTER TABLE slurm_jobs DROP COLUMN IF EXISTS cpu_time_seconds;
ALTER TABLE slurm_jobs DROP COLUMN IF EXISTS elapsed_seconds;
