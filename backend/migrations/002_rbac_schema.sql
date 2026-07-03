ALTER TABLE roles ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE roles ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE roles ADD COLUMN IF NOT EXISTS is_builtin BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE roles ADD COLUMN IF NOT EXISTS allow_delete BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE roles ADD COLUMN IF NOT EXISTS allow_permission_edit BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE roles ADD COLUMN IF NOT EXISTS version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE roles ADD COLUMN IF NOT EXISTS created_by TEXT NOT NULL DEFAULT 'system';
ALTER TABLE roles ADD COLUMN IF NOT EXISTS updated_by TEXT NOT NULL DEFAULT 'system';
ALTER TABLE roles ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

DO $$ BEGIN
  ALTER TABLE roles ADD CONSTRAINT roles_scope_type_check
    CHECK (scope_type IN ('global','unit','team','self'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN
  ALTER TABLE roles ADD CONSTRAINT roles_status_check
    CHECK (status IN ('active','disabled'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
CREATE INDEX IF NOT EXISTS idx_roles_status ON roles(status);

CREATE TABLE IF NOT EXISTS permissions (
  id BIGSERIAL PRIMARY KEY,
  permission_key TEXT NOT NULL UNIQUE,
  permission_type TEXT NOT NULL CHECK (permission_type IN ('menu','action','route','api')),
  module_code TEXT NOT NULL,
  resource_code TEXT NOT NULL DEFAULT '',
  action_code TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  sort_order INTEGER NOT NULL DEFAULT 0,
  is_system BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_permissions_type_module
  ON permissions(permission_type,module_code);

CREATE TABLE IF NOT EXISTS menus (
  id BIGSERIAL PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  parent_id BIGINT REFERENCES menus(id) ON DELETE RESTRICT,
  name TEXT NOT NULL,
  icon TEXT NOT NULL DEFAULT '',
  route_path TEXT NOT NULL DEFAULT '',
  route_permission_key TEXT NOT NULL DEFAULT '',
  menu_permission_key TEXT NOT NULL,
  menu_type TEXT NOT NULL CHECK (menu_type IN ('group','page')),
  sort_order INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS role_permissions (
  role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
  created_by TEXT NOT NULL DEFAULT 'system',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY(role_id,permission_id)
);

CREATE TABLE IF NOT EXISTS role_data_scopes (
  role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  resource_code TEXT NOT NULL,
  scope_type TEXT NOT NULL CHECK
    (scope_type IN ('global','unit','team','self','granted','none')),
  access_level TEXT NOT NULL DEFAULT 'view' CHECK
    (access_level IN ('none','view','manage')),
  PRIMARY KEY(role_id,resource_code,scope_type)
);
CREATE INDEX IF NOT EXISTS idx_role_data_scopes_resource
  ON role_data_scopes(role_id,resource_code);

CREATE TABLE IF NOT EXISTS role_file_policies (
  id BIGSERIAL PRIMARY KEY,
  role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  storage_root TEXT NOT NULL,
  subject_scope TEXT NOT NULL CHECK
    (subject_scope IN ('self','team_shared','team_members','unit_shared','unit_members','global')),
  access_level TEXT NOT NULL CHECK (access_level IN ('read','manage')),
  allow_hidden BOOLEAN NOT NULL DEFAULT FALSE,
  created_by TEXT NOT NULL DEFAULT 'system',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(role_id,storage_root,subject_scope)
);
CREATE INDEX IF NOT EXISTS idx_role_file_policies_root
  ON role_file_policies(role_id,storage_root);

CREATE TABLE IF NOT EXISTS user_roles_v2 (
  id BIGSERIAL PRIMARY KEY,
  account_type TEXT NOT NULL CHECK (account_type IN ('admin','ldap')),
  username TEXT NOT NULL,
  role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
  scope_type TEXT NOT NULL CHECK (scope_type IN ('global','unit','team','self')),
  scope_id TEXT NOT NULL DEFAULT '*',
  status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  valid_from TIMESTAMPTZ,
  valid_until TIMESTAMPTZ,
  created_by TEXT NOT NULL DEFAULT 'system',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(account_type,username,role_id,scope_type,scope_id)
);
CREATE INDEX IF NOT EXISTS idx_user_roles_v2_account
  ON user_roles_v2(account_type,username,status);
