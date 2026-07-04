DELETE FROM role_permissions
WHERE permission_id IN (
  SELECT id FROM permissions
  WHERE permission_key LIKE 'api.webssh.%'
     OR permission_key LIKE 'action.webssh.%'
     OR permission_key IN ('menu.webssh.view','route.webssh.view')
);

DELETE FROM permissions
WHERE permission_key LIKE 'api.webssh.%'
   OR permission_key LIKE 'action.webssh.%'
   OR permission_key IN ('menu.webssh.view','route.webssh.view');
