-- Restore the System Configuration menu as a first-class top-level group and
-- align administrator navigation labels with the agreed RBAC menu taxonomy.

INSERT INTO permissions(permission_key,permission_type,module_code,resource_code,action_code,name,sort_order,is_system)
VALUES
('menu.system.platform.view','menu','system','config.platform','view','平台设置',80,TRUE),
('menu.system.ldap.view','menu','system','config.ldap','view','LDAP 配置',81,TRUE),
('menu.system.slurm.view','menu','system','config.slurm','view','Slurm 配置',82,TRUE),
('menu.system.storage.view','menu','system','storage.roots','view','存储配置',83,TRUE),
('menu.system.notify.view','menu','system','config.notify','view','通知配置',84,TRUE),
('route.system.platform.view','route','system','config.platform','view','访问平台设置',106,TRUE),
('route.system.ldap.view','route','system','config.ldap','view','访问 LDAP 配置',107,TRUE),
('route.system.slurm.view','route','system','config.slurm','view','访问 Slurm 配置',108,TRUE),
('route.system.storage.view','route','system','storage.roots','view','访问存储配置',109,TRUE),
('route.system.notify.view','route','system','config.notify','view','访问通知配置',110,TRUE)
ON CONFLICT(permission_key) DO UPDATE SET
  permission_type=EXCLUDED.permission_type,
  module_code=EXCLUDED.module_code,
  resource_code=EXCLUDED.resource_code,
  action_code=EXCLUDED.action_code,
  name=EXCLUDED.name,
  sort_order=EXCLUDED.sort_order,
  status='active',
  updated_at=now();

UPDATE menus SET name='资源管理' WHERE code='compute';
UPDATE menus SET name='作业管理' WHERE code='jobs';
UPDATE menus SET name='运维管理' WHERE code='operations';
UPDATE menus SET name='日志管理' WHERE code='logs';

INSERT INTO menus(code,parent_id,name,icon,route_path,route_permission_key,menu_permission_key,menu_type,sort_order)
VALUES
('system',NULL,'系统配置','⚙','','','menu.system.platform.view','group',80)
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

INSERT INTO menus(code,parent_id,name,route_path,route_permission_key,menu_permission_key,menu_type,sort_order)
SELECT v.code,p.id,v.name,v.path,v.route_key,v.menu_key,'page',v.sort_order
FROM (VALUES
('settings','system','平台设置','settings.html','route.system.platform.view','menu.system.platform.view',81),
('ldap_config','system','LDAP 配置','ldap.html','route.system.ldap.view','menu.system.ldap.view',82),
('slurm_config','system','Slurm 配置','slurm.html','route.system.slurm.view','menu.system.slurm.view',83),
('storage_config','system','存储配置','storage.html','route.system.storage.view','menu.system.storage.view',84),
('notify_config','system','通知配置','notify.html','route.system.notify.view','menu.system.notify.view',85)
) AS v(code,parent_code,name,path,route_key,menu_key,sort_order)
JOIN menus p ON p.code=v.parent_code
ON CONFLICT(code) DO UPDATE SET
  parent_id=EXCLUDED.parent_id,
  name=EXCLUDED.name,
  route_path=EXCLUDED.route_path,
  route_permission_key=EXCLUDED.route_permission_key,
  menu_permission_key=EXCLUDED.menu_permission_key,
  sort_order=EXCLUDED.sort_order,
  status='active';

INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'system'
FROM roles r CROSS JOIN permissions p
WHERE r.code='cluster_admin'
  AND p.permission_key IN (
    'menu.system.platform.view','menu.system.ldap.view','menu.system.slurm.view',
    'menu.system.storage.view','menu.system.notify.view',
    'route.system.platform.view','route.system.ldap.view','route.system.slurm.view',
    'route.system.storage.view','route.system.notify.view'
  )
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'system'
FROM roles r JOIN permissions p ON p.permission_key IN (
  'menu.system.platform.view','menu.system.ldap.view','menu.system.slurm.view',
  'menu.system.storage.view','menu.system.notify.view',
  'route.system.platform.view','route.system.ldap.view','route.system.slurm.view',
  'route.system.storage.view','route.system.notify.view'
)
WHERE r.code='config_admin'
ON CONFLICT DO NOTHING;
