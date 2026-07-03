DELETE FROM role_permissions
WHERE created_by='migration-010-role-readonly';

DELETE FROM permissions
WHERE permission_key='action.roles.view'
  AND description='RBAC 角色体系只读权限'
  AND NOT EXISTS (
    SELECT 1 FROM role_permissions rp WHERE rp.permission_id=permissions.id
  );
