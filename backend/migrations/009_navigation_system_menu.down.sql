DELETE FROM role_permissions
WHERE permission_id IN (
  SELECT id FROM permissions WHERE permission_key IN (
    'menu.system.platform.view','menu.system.ldap.view','menu.system.slurm.view',
    'menu.system.storage.view','menu.system.notify.view',
    'route.system.platform.view','route.system.ldap.view','route.system.slurm.view',
    'route.system.storage.view','route.system.notify.view'
  )
)
AND created_by='system';

DELETE FROM menus WHERE code IN (
  'settings','ldap_config','slurm_config','storage_config','notify_config','system'
);

UPDATE menus SET name='计算管理' WHERE code='compute';
UPDATE menus SET name='作业中心' WHERE code='jobs';
UPDATE menus SET name='监控运维' WHERE code='operations';
UPDATE menus SET name='日志中心' WHERE code='logs';

DELETE FROM permissions WHERE permission_key IN (
  'menu.system.platform.view','menu.system.ldap.view','menu.system.slurm.view',
  'menu.system.storage.view','menu.system.notify.view',
  'route.system.platform.view','route.system.ldap.view','route.system.slurm.view',
  'route.system.storage.view','route.system.notify.view'
);
