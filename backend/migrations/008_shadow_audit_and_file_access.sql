-- Align file policy access levels with the Go RBAC model and optimize shadow
-- comparison reporting before enforce is considered.
ALTER TABLE role_file_policies
  DROP CONSTRAINT IF EXISTS role_file_policies_access_level_check;

UPDATE role_file_policies SET access_level='view' WHERE access_level='read';

ALTER TABLE role_file_policies
  ADD CONSTRAINT role_file_policies_access_level_check
  CHECK (access_level IN ('view','manage'));

CREATE INDEX IF NOT EXISTS idx_audit_logs_rbac_shadow_created
  ON audit_logs(created_at DESC)
  WHERE action='rbac.shadow.compare';
