CREATE TABLE IF NOT EXISTS license_servers (
  id BIGSERIAL PRIMARY KEY,
  app_name TEXT NOT NULL DEFAULT '',
  app_code TEXT NOT NULL DEFAULT '',
  app_type TEXT NOT NULL DEFAULT '',
  icon_url TEXT NOT NULL DEFAULT '',
  vendor TEXT NOT NULL DEFAULT '',
  license_type TEXT NOT NULL DEFAULT 'FlexNet',
  manager_name TEXT NOT NULL DEFAULT '',
  server_host TEXT NOT NULL DEFAULT '',
  port INTEGER NOT NULL DEFAULT 0,
  collect_method TEXT NOT NULL DEFAULT 'lmstat',
  collect_command TEXT NOT NULL DEFAULT '',
  service_name TEXT NOT NULL DEFAULT '',
  collect_interval_sec INTEGER NOT NULL DEFAULT 60,
  timeout_sec INTEGER NOT NULL DEFAULT 10,
  warning_threshold INTEGER NOT NULL DEFAULT 80,
  critical_threshold INTEGER NOT NULL DEFAULT 95,
  expire_warning_days INTEGER NOT NULL DEFAULT 30,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  service_status TEXT NOT NULL DEFAULT 'unknown',
  last_collect_status TEXT NOT NULL DEFAULT 'never',
  last_collect_message TEXT NOT NULL DEFAULT '',
  last_raw_output TEXT NOT NULL DEFAULT '',
  last_collected_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(app_code)
);

CREATE TABLE IF NOT EXISTS license_features (
  id BIGSERIAL PRIMARY KEY,
  config_id BIGINT NOT NULL REFERENCES license_servers(id) ON DELETE CASCADE,
  feature_name TEXT NOT NULL,
  total_count INTEGER NOT NULL DEFAULT 0,
  used_count INTEGER NOT NULL DEFAULT 0,
  free_count INTEGER NOT NULL DEFAULT 0,
  queued_count INTEGER NOT NULL DEFAULT 0,
  expires_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(config_id, feature_name)
);

CREATE TABLE IF NOT EXISTS license_usage_sessions (
  id BIGSERIAL PRIMARY KEY,
  config_id BIGINT NOT NULL REFERENCES license_servers(id) ON DELETE CASCADE,
  feature_name TEXT NOT NULL DEFAULT '',
  username TEXT NOT NULL DEFAULT '',
  job_id TEXT NOT NULL DEFAULT '',
  node_name TEXT NOT NULL DEFAULT '',
  host_name TEXT NOT NULL DEFAULT '',
  process_id TEXT NOT NULL DEFAULT '',
  checkout_count INTEGER NOT NULL DEFAULT 1,
  started_at TIMESTAMPTZ,
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  source TEXT NOT NULL DEFAULT 'collector',
  status TEXT NOT NULL DEFAULT 'active'
);

CREATE INDEX IF NOT EXISTS idx_license_usage_config ON license_usage_sessions(config_id, status);
CREATE INDEX IF NOT EXISTS idx_license_usage_user ON license_usage_sessions(username);
CREATE INDEX IF NOT EXISTS idx_license_usage_job ON license_usage_sessions(job_id);

CREATE TABLE IF NOT EXISTS license_usage_samples (
  id BIGSERIAL PRIMARY KEY,
  config_id BIGINT NOT NULL REFERENCES license_servers(id) ON DELETE CASCADE,
  feature_name TEXT NOT NULL DEFAULT '',
  sample_time TIMESTAMPTZ NOT NULL DEFAULT now(),
  total_count INTEGER NOT NULL DEFAULT 0,
  used_count INTEGER NOT NULL DEFAULT 0,
  free_count INTEGER NOT NULL DEFAULT 0,
  queued_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_license_samples_config_time ON license_usage_samples(config_id, sample_time DESC);

WITH license_permissions(permission_key,permission_type,module_code,resource_code,action_code,name,sort_order) AS (
  VALUES
    ('menu.license.config.view','menu','license','license.config','view','应用许可配置菜单',860),
    ('route.license.config.view','route','license','license.config','view','应用许可配置路由',861),
    ('menu.license.status.view','menu','license','license.status','view','应用许可状态菜单',862),
    ('route.license.status.view','route','license','license.status','view','应用许可状态路由',863),
    ('api.license.config.list','api','license','license.config','list','License 配置列表',864),
    ('api.license.config.view','api','license','license.config','view','License 配置详情',865),
    ('api.license.config.create','api','license','license.config','create','新增 License 配置',866),
    ('api.license.config.update','api','license','license.config','update','更新 License 配置',867),
    ('api.license.config.delete','api','license','license.config','delete','删除 License 配置',868),
    ('api.license.config.test','api','license','license.config','test','测试 License 采集',869),
    ('api.license.config.collect','api','license','license.config','collect','立即采集 License',870),
    ('api.license.config.start','api','license','license.config','start','启动 License 服务',871),
    ('api.license.config.stop','api','license','license.config','stop','停止 License 服务',872),
    ('api.license.config.restart','api','license','license.config','restart','重启 License 服务',873),
    ('api.license.status.list','api','license','license.status','list','License 状态聚合',874),
    ('api.license.status.view','api','license','license.status','view','License 状态详情',875)
)
INSERT INTO permissions (permission_key,permission_type,module_code,resource_code,action_code,name,sort_order)
SELECT permission_key,permission_type,module_code,resource_code,action_code,name,sort_order
FROM license_permissions
ON CONFLICT(permission_key) DO UPDATE SET
  permission_type=EXCLUDED.permission_type,
  module_code=EXCLUDED.module_code,
  resource_code=EXCLUDED.resource_code,
  action_code=EXCLUDED.action_code,
  name=EXCLUDED.name,
  sort_order=EXCLUDED.sort_order;

INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'migration-014-license-monitoring'
FROM roles r
JOIN permissions p ON p.permission_key IN (
  'menu.license.config.view','route.license.config.view',
  'menu.license.status.view','route.license.status.view',
  'api.license.config.list','api.license.config.view','api.license.config.create',
  'api.license.config.update','api.license.config.delete','api.license.config.test',
  'api.license.config.collect','api.license.config.start','api.license.config.stop',
  'api.license.config.restart','api.license.status.list','api.license.status.view'
)
WHERE r.code IN ('cluster_admin','config_admin')
ON CONFLICT(role_id,permission_id) DO NOTHING;

INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'migration-014-license-monitoring'
FROM roles r
JOIN permissions p ON p.permission_key IN (
  'menu.license.status.view','route.license.status.view',
  'api.license.status.list','api.license.status.view'
)
WHERE r.code IN ('unit_admin','team_admin','user')
ON CONFLICT(role_id,permission_id) DO NOTHING;
