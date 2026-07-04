DELETE FROM role_permissions
WHERE permission_id IN (
  SELECT id FROM permissions
  WHERE permission_key IN ('api.config.terminal.list','api.config.terminal.update')
);

DELETE FROM permissions
WHERE permission_key IN ('api.config.terminal.list','api.config.terminal.update');

