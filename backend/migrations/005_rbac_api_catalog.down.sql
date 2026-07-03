DELETE FROM role_permissions
WHERE permission_id IN (
  SELECT id FROM permissions WHERE description='API 路由权限点'
);
DELETE FROM permissions WHERE description='API 路由权限点';
