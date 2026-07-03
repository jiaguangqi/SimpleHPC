DROP INDEX IF EXISTS idx_audit_logs_rbac_shadow_created;

ALTER TABLE role_file_policies
  DROP CONSTRAINT IF EXISTS role_file_policies_access_level_check;

UPDATE role_file_policies SET access_level='read' WHERE access_level='view';

ALTER TABLE role_file_policies
  ADD CONSTRAINT role_file_policies_access_level_check
  CHECK (access_level IN ('read','manage'));
