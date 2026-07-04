-- Terminal login node configuration permissions.
WITH terminal_config_permissions(permission_key,permission_type,module_code,resource_code,action_code,name,sort_order) AS (VALUES
  ('api.config.terminal.list','api','system','config.terminal','list','终端登录节点配置：查看',465),
  ('api.config.terminal.update','api','system','config.terminal','update','终端登录节点配置：保存',466)
)
INSERT INTO permissions(permission_key,permission_type,module_code,resource_code,action_code,name,description,status,sort_order,is_system)
SELECT permission_key,permission_type,module_code,resource_code,action_code,name,'API 路由权限点','active',sort_order,TRUE
FROM terminal_config_permissions
ON CONFLICT(permission_key) DO UPDATE SET
  permission_type=EXCLUDED.permission_type,
  module_code=EXCLUDED.module_code,
  resource_code=EXCLUDED.resource_code,
  action_code=EXCLUDED.action_code,
  name=EXCLUDED.name,
  status='active',
  updated_at=now();

INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'migration-012-terminal-login-nodes'
FROM roles r
JOIN permissions p ON p.permission_key IN ('api.config.terminal.list','api.config.terminal.update')
WHERE r.code IN ('cluster_admin','config_admin')
ON CONFLICT DO NOTHING;

