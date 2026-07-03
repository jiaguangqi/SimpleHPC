INSERT INTO user_roles_v2(account_type,username,role_id,scope_type,scope_id,status,created_by)
SELECT 'admin',a.username,r.id,r.scope_type,'*','active','migration'
FROM admin_users a
JOIN roles r ON r.code=a.role_name
WHERE a.status='active'
ON CONFLICT(account_type,username,role_id,scope_type,scope_id)
DO UPDATE SET status='active',updated_at=now();

INSERT INTO user_roles_v2(account_type,username,role_id,scope_type,scope_id,status,created_by)
SELECT 'ldap',u.username,r.id,'self',u.username,'active','migration'
FROM platform_users u
JOIN roles r ON r.code='user'
WHERE u.status='active'
ON CONFLICT(account_type,username,role_id,scope_type,scope_id)
DO UPDATE SET status='active',updated_at=now();

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM user_roles_v2 ur
    JOIN roles r ON r.id=ur.role_id
    JOIN admin_users a ON a.username=ur.username AND ur.account_type='admin'
    WHERE r.code='cluster_admin' AND r.status='active'
      AND ur.status='active' AND a.status='active'
      AND (ur.valid_from IS NULL OR ur.valid_from<=now())
      AND (ur.valid_until IS NULL OR ur.valid_until>now())
  ) THEN
    RAISE EXCEPTION 'RBAC migration requires at least one active cluster_admin binding';
  END IF;
END $$;
