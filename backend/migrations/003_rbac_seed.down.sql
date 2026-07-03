DELETE FROM role_file_policies WHERE created_by='system';
DELETE FROM role_data_scopes WHERE role_id IN (
  SELECT id FROM roles WHERE code IN ('cluster_admin','config_admin','unit_admin','team_admin','user')
);
DELETE FROM role_permissions WHERE created_by='system';
DELETE FROM menus;
DELETE FROM permissions WHERE is_system=TRUE;
UPDATE roles SET is_builtin=FALSE,allow_delete=TRUE,allow_permission_edit=TRUE
WHERE code IN ('cluster_admin','config_admin','unit_admin','team_admin','user');
