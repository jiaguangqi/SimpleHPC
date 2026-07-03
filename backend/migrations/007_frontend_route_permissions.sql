-- Batch 4: make every dynamic administrative menu assignable together with
-- its route guard. This migration is delivered only; it is not applied to the
-- existing business database during legacy/shadow development.
WITH route_permissions(permission_key,module_code,resource_code,name,sort_order) AS (VALUES
  ('route.account.units.view','account','units','访问单位管理',110),
  ('route.account.teams.view','account','teams','访问团队管理',111),
  ('route.account.users.view','account','users','访问用户管理',112),
  ('route.account.admins.view','account','admins','访问管理员账号管理',113),
  ('route.account.roles.view','account','roles','访问角色管理',114),
  ('route.compute.partitions.view','compute','partitions','访问资源队列配置',120),
  ('route.compute.nodes.view','compute','nodes','访问节点状态',121),
  ('route.compute.qos.view','compute','qos','访问 QOS 策略',122),
  ('route.data.acl.view','data','storage_acl','访问存储授权',130),
  ('route.operations.monitoring.view','operations','monitoring','访问监控告警',140),
  ('route.operations.inspection.view','operations','inspection_reports','访问巡检报告',141),
  ('route.logs.view','logs','logs','访问日志中心',150)
)
INSERT INTO permissions(
  permission_key,permission_type,module_code,resource_code,action_code,
  name,description,status,sort_order,is_system
)
SELECT permission_key,'route',module_code,resource_code,'view',name,
  '前端动态路由守卫权限','active',sort_order,TRUE
FROM route_permissions
ON CONFLICT(permission_key) DO UPDATE SET
  permission_type='route',module_code=EXCLUDED.module_code,
  resource_code=EXCLUDED.resource_code,action_code='view',
  name=EXCLUDED.name,description=EXCLUDED.description,status='active',
  sort_order=EXCLUDED.sort_order,updated_at=now();

-- Existing roles keep access to pages whose menu permission they already own.
WITH pairs(menu_key,route_key) AS (VALUES
  ('menu.account.units.view','route.account.units.view'),
  ('menu.account.teams.view','route.account.teams.view'),
  ('menu.account.users.view','route.account.users.view'),
  ('menu.account.admins.view','route.account.admins.view'),
  ('menu.account.roles.view','route.account.roles.view'),
  ('menu.compute.partitions.view','route.compute.partitions.view'),
  ('menu.compute.nodes.view','route.compute.nodes.view'),
  ('menu.compute.qos.view','route.compute.qos.view'),
  ('menu.data.acl.view','route.data.acl.view'),
  ('menu.operations.monitoring.view','route.operations.monitoring.view'),
  ('menu.operations.inspection.view','route.operations.inspection.view'),
  ('menu.logs.view','route.logs.view')
)
INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT rp.role_id,route_permission.id,'migration-007'
FROM role_permissions rp
JOIN permissions menu_permission ON menu_permission.id=rp.permission_id
JOIN pairs ON pairs.menu_key=menu_permission.permission_key
JOIN permissions route_permission ON route_permission.permission_key=pairs.route_key
ON CONFLICT DO NOTHING;
