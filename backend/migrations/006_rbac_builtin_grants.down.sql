DELETE FROM role_file_policies
WHERE role_id IN (SELECT id FROM roles WHERE code IN ('unit_admin','team_admin'))
  AND created_by='system';
DELETE FROM role_data_scopes
WHERE role_id IN (SELECT id FROM roles WHERE code IN ('unit_admin','team_admin'));
DELETE FROM role_permissions
WHERE role_id IN (
  SELECT id FROM roles WHERE code IN ('config_admin','unit_admin','team_admin')
) AND created_by='system';
