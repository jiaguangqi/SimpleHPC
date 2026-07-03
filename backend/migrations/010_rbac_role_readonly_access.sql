-- Grant read-only role catalog visibility to intermediate administrator roles.
-- Mutating role operations remain restricted because no action.roles.* mutation
-- or api.roles.create/update/delete permissions are granted here.

WITH readonly_permissions(permission_key,permission_type,module_code,resource_code,action_code,name,sort_order) AS (VALUES
  ('menu.account.roles.view','menu','account','roles','view','角色管理',24),
  ('route.account.roles.view','route','account','roles','view','访问角色管理',114),
  ('api.roles.list','api','roles','roles','list','角色：查看列表',400),
  ('action.roles.view','action','account','roles','view','查看角色',200)
)
INSERT INTO permissions(
  permission_key,permission_type,module_code,resource_code,action_code,
  name,description,status,sort_order,is_system
)
SELECT permission_key,permission_type,module_code,resource_code,action_code,
  name,'RBAC 角色体系只读权限','active',sort_order,TRUE
FROM readonly_permissions
ON CONFLICT(permission_key) DO UPDATE SET
  permission_type=EXCLUDED.permission_type,
  module_code=EXCLUDED.module_code,
  resource_code=EXCLUDED.resource_code,
  action_code=EXCLUDED.action_code,
  name=EXCLUDED.name,
  status='active',
  sort_order=EXCLUDED.sort_order,
  updated_at=now();

INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'migration-010-role-readonly'
FROM roles r
JOIN permissions p ON p.permission_key IN (
  'menu.account.roles.view',
  'route.account.roles.view',
  'api.roles.list',
  'action.roles.view'
)
WHERE r.code IN ('config_admin','unit_admin','team_admin')
ON CONFLICT DO NOTHING;
