DELETE FROM role_permissions rp
USING permissions p
WHERE rp.permission_id=p.id
  AND p.permission_key IN (
    'menu.terminal.view',
    'route.terminal.view',
    'action.terminal.connect',
    'api.terminal.connect'
  );

DELETE FROM menus WHERE code='terminal';

DELETE FROM permissions
WHERE permission_key IN (
  'menu.terminal.view',
  'route.terminal.view',
  'action.terminal.connect',
  'api.terminal.connect'
);
