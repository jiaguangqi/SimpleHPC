#!/usr/bin/env bash
set -euo pipefail

: "${DATABASE_URL:?DATABASE_URL is required}"

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
DO $$
BEGIN
  IF (SELECT count(*) FROM roles WHERE is_builtin) <> 5 THEN
    RAISE EXCEPTION 'expected 5 builtin roles';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM user_roles_v2 ur
    JOIN roles r ON r.id=ur.role_id
    WHERE r.code='cluster_admin' AND r.status='active' AND ur.status='active'
  ) THEN
    RAISE EXCEPTION 'no active cluster_admin binding';
  END IF;
  IF EXISTS (
    SELECT 1 FROM user_roles_v2 ur LEFT JOIN roles r ON r.id=ur.role_id
    WHERE r.id IS NULL
  ) THEN
    RAISE EXCEPTION 'orphan role binding found';
  END IF;
END $$;

SELECT 'roles' AS item, count(*) AS count FROM roles
UNION ALL SELECT 'permissions', count(*) FROM permissions
UNION ALL SELECT 'menus', count(*) FROM menus
UNION ALL SELECT 'bindings', count(*) FROM user_roles_v2
UNION ALL SELECT 'role_permissions', count(*) FROM role_permissions
UNION ALL SELECT 'data_scopes', count(*) FROM role_data_scopes
UNION ALL SELECT 'file_policies', count(*) FROM role_file_policies;
SQL
