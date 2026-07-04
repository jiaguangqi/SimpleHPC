-- Terminal Center / WebSSH MVP.
-- Grants default self-terminal access while keeping cluster_admin as the only
-- role eligible for future target-user expansion.

WITH terminal_permissions(permission_key,permission_type,module_code,resource_code,action_code,name,sort_order) AS (VALUES
  ('menu.terminal.view','menu','terminal','terminal','view','终端中心',90),
  ('route.terminal.view','route','terminal','terminal','view','访问终端中心',190),
  ('action.terminal.connect','action','terminal','terminal','connect','连接终端',260),
  ('api.terminal.connect','api','terminal','terminal','connect','终端：建立会话',430)
)
INSERT INTO permissions(
  permission_key,permission_type,module_code,resource_code,action_code,
  name,description,status,sort_order,is_system
)
SELECT permission_key,permission_type,module_code,resource_code,action_code,
  name,'WebSSH 终端中心权限','active',sort_order,TRUE
FROM terminal_permissions
ON CONFLICT(permission_key) DO UPDATE SET
  permission_type=EXCLUDED.permission_type,
  module_code=EXCLUDED.module_code,
  resource_code=EXCLUDED.resource_code,
  action_code=EXCLUDED.action_code,
  name=EXCLUDED.name,
  description=EXCLUDED.description,
  status='active',
  sort_order=EXCLUDED.sort_order,
  updated_at=now();

INSERT INTO menus(
  code,parent_id,name,icon,route_path,route_permission_key,
  menu_permission_key,menu_type,sort_order,status
)
VALUES (
  'terminal',NULL,'终端中心','terminal','terminal.html',
  'route.terminal.view','menu.terminal.view','page',55,'active'
)
ON CONFLICT(code) DO UPDATE SET
  parent_id=NULL,
  name=EXCLUDED.name,
  icon=EXCLUDED.icon,
  route_path=EXCLUDED.route_path,
  route_permission_key=EXCLUDED.route_permission_key,
  menu_permission_key=EXCLUDED.menu_permission_key,
  menu_type=EXCLUDED.menu_type,
  sort_order=EXCLUDED.sort_order,
  status='active';

INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'migration-011-terminal-center'
FROM roles r
JOIN permissions p ON p.permission_key IN (
  'menu.terminal.view',
  'route.terminal.view',
  'action.terminal.connect',
  'api.terminal.connect'
)
WHERE r.code IN ('cluster_admin','config_admin','unit_admin','team_admin','user')
ON CONFLICT DO NOTHING;
