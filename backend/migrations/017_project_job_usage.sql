ALTER TABLE slurm_jobs ADD COLUMN IF NOT EXISTS elapsed_seconds BIGINT NOT NULL DEFAULT 0;
ALTER TABLE slurm_jobs ADD COLUMN IF NOT EXISTS cpu_time_seconds BIGINT NOT NULL DEFAULT 0;
ALTER TABLE slurm_jobs ADD COLUMN IF NOT EXISTS alloc_tres TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_slurm_jobs_account_usage
  ON slurm_jobs(account, synced_at DESC)
  WHERE account <> '';
