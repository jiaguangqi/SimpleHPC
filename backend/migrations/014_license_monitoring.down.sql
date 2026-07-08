DELETE FROM role_permissions
WHERE created_by='migration-014-license-monitoring';

DELETE FROM permissions
WHERE permission_key IN (
  'menu.license.config.view','route.license.config.view',
  'menu.license.status.view','route.license.status.view',
  'api.license.config.list','api.license.config.view','api.license.config.create',
  'api.license.config.update','api.license.config.delete','api.license.config.test',
  'api.license.config.collect','api.license.config.start','api.license.config.stop',
  'api.license.config.restart','api.license.status.list','api.license.status.view'
);

DROP TABLE IF EXISTS license_usage_samples;
DROP TABLE IF EXISTS license_usage_sessions;
DROP TABLE IF EXISTS license_features;
DROP TABLE IF EXISTS license_servers;
