-- Configuration administrators: platform and cluster configuration without
-- implicit access to user jobs or user file contents.
INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'system' FROM roles r JOIN permissions p ON
  p.permission_key IN (
    'menu.dashboard.view','menu.compute.partitions.view','menu.compute.queue.view',
    'menu.compute.nodes.view','menu.compute.qos.view',
    'menu.operations.monitoring.view','menu.operations.inspection.view','menu.logs.view',
    'route.dashboard.view','route.queue.view'
  )
  OR (p.permission_type='api' AND (
    p.resource_code LIKE 'config.%' OR
    p.resource_code IN ('dashboard','queue','nodes','qos','partitions',
      'monitoring','inspection','logs','storage.roots')
  ))
WHERE r.code='config_admin'
ON CONFLICT DO NOTHING;

-- Unit administrators.
INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'system' FROM roles r JOIN permissions p ON
  p.permission_key IN (
    'menu.dashboard.view','menu.account.teams.view','menu.account.users.view',
    'menu.compute.queue.view','menu.data.files.view','menu.jobs.templates.view',
    'menu.jobs.list.view','menu.jobs.vnc.view',
    'route.dashboard.view','route.queue.view','route.data.files.view',
    'route.jobs.templates.view','route.jobs.list.view','route.jobs.vnc.view',
    'api.storage.roots.list'
  )
  OR (p.permission_type='api' AND p.resource_code IN
    ('auth','dashboard','users','teams','jobs','queue','storage.files','templates'))
WHERE r.code='unit_admin'
ON CONFLICT DO NOTHING;

-- Team administrators.
INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'system' FROM roles r JOIN permissions p ON
  p.permission_key IN (
    'menu.dashboard.view','menu.account.users.view','menu.compute.queue.view',
    'menu.data.files.view','menu.jobs.templates.view','menu.jobs.list.view',
    'menu.jobs.vnc.view','route.dashboard.view','route.queue.view',
    'route.data.files.view','route.jobs.templates.view','route.jobs.list.view',
    'route.jobs.vnc.view','api.storage.roots.list'
  )
  OR (p.permission_type='api' AND p.resource_code IN
    ('auth','dashboard','users','jobs','queue','storage.files','templates'))
WHERE r.code='team_admin'
ON CONFLICT DO NOTHING;

INSERT INTO role_data_scopes(role_id,resource_code,scope_type,access_level)
SELECT r.id,v.resource,'unit',v.access FROM roles r CROSS JOIN (VALUES
  ('users','manage'),('teams','manage'),('jobs','manage'),
  ('job_templates','manage'),('vnc_sessions','manage'),('storage_files','manage')
) v(resource,access) WHERE r.code='unit_admin'
ON CONFLICT(role_id,resource_code,scope_type)
DO UPDATE SET access_level=EXCLUDED.access_level;

INSERT INTO role_data_scopes(role_id,resource_code,scope_type,access_level)
SELECT r.id,v.resource,'team',v.access FROM roles r CROSS JOIN (VALUES
  ('users','view'),('jobs','manage'),('job_templates','manage'),
  ('vnc_sessions','manage'),('storage_files','manage')
) v(resource,access) WHERE r.code='team_admin'
ON CONFLICT(role_id,resource_code,scope_type)
DO UPDATE SET access_level=EXCLUDED.access_level;

-- Shared directories are explicit. Member directories are not granted by
-- default and must be added by an administrator.
INSERT INTO role_file_policies(
  role_id,storage_root,subject_scope,access_level,allow_hidden,created_by
)
SELECT r.id,v.root,'unit_shared','manage',FALSE,'system'
FROM roles r CROSS JOIN (VALUES('/data/share'),('/data/scratch')) v(root)
WHERE r.code='unit_admin'
ON CONFLICT(role_id,storage_root,subject_scope)
DO UPDATE SET access_level=EXCLUDED.access_level;

INSERT INTO role_file_policies(
  role_id,storage_root,subject_scope,access_level,allow_hidden,created_by
)
SELECT r.id,v.root,'team_shared','manage',FALSE,'system'
FROM roles r CROSS JOIN (VALUES('/data/share'),('/data/scratch')) v(root)
WHERE r.code='team_admin'
ON CONFLICT(role_id,storage_root,subject_scope)
DO UPDATE SET access_level=EXCLUDED.access_level;
