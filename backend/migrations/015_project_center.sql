CREATE TABLE IF NOT EXISTS projects (
  id BIGSERIAL PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  summary TEXT NOT NULL DEFAULT '',
  owner_username TEXT NOT NULL DEFAULT '',
  unit_id BIGINT REFERENCES units(id) ON DELETE SET NULL,
  team_id BIGINT REFERENCES teams(id) ON DELETE SET NULL,
  status TEXT NOT NULL DEFAULT 'planning'
    CHECK (status IN ('planning','active','paused','completed','archived')),
  priority TEXT NOT NULL DEFAULT 'normal'
    CHECK (priority IN ('low','normal','high','critical')),
  start_date DATE,
  end_date DATE,
  storage_quota_gb INTEGER NOT NULL DEFAULT 0,
  compute_quota_hours INTEGER NOT NULL DEFAULT 0,
  license_budget_points INTEGER NOT NULL DEFAULT 0,
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_by TEXT NOT NULL DEFAULT 'system',
  updated_by TEXT NOT NULL DEFAULT 'system',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
CREATE INDEX IF NOT EXISTS idx_projects_owner ON projects(owner_username);
CREATE INDEX IF NOT EXISTS idx_projects_unit_team ON projects(unit_id,team_id);

CREATE TABLE IF NOT EXISTS project_members (
  id BIGSERIAL PRIMARY KEY,
  project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  username TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  role TEXT NOT NULL DEFAULT 'compute_member'
    CHECK (role IN ('owner','manager','compute_member','data_member','viewer','external')),
  permission TEXT NOT NULL DEFAULT 'work'
    CHECK (permission IN ('read','work','manage')),
  status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active','disabled')),
  joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by TEXT NOT NULL DEFAULT 'system',
  UNIQUE(project_id,username)
);
CREATE INDEX IF NOT EXISTS idx_project_members_username ON project_members(username,status);

CREATE TABLE IF NOT EXISTS project_tasks (
  id BIGSERIAL PRIMARY KEY,
  project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  assignee_username TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'todo'
    CHECK (status IN ('todo','running','blocked','done','cancelled')),
  priority TEXT NOT NULL DEFAULT 'normal'
    CHECK (priority IN ('low','normal','high','critical')),
  due_date DATE,
  description TEXT NOT NULL DEFAULT '',
  upstream_task_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_by TEXT NOT NULL DEFAULT 'system',
  updated_by TEXT NOT NULL DEFAULT 'system',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_project_tasks_project_status ON project_tasks(project_id,status);

CREATE TABLE IF NOT EXISTS project_directories (
  id BIGSERIAL PRIMARY KEY,
  project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL DEFAULT '项目空间',
  path TEXT NOT NULL,
  permission TEXT NOT NULL DEFAULT 'rw'
    CHECK (permission IN ('r','rw','rwx','manage')),
  status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active','disabled')),
  created_by TEXT NOT NULL DEFAULT 'system',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(project_id,path)
);

CREATE TABLE IF NOT EXISTS project_job_links (
  id BIGSERIAL PRIMARY KEY,
  project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  job_id TEXT NOT NULL,
  task_id BIGINT REFERENCES project_tasks(id) ON DELETE SET NULL,
  job_name TEXT NOT NULL DEFAULT '',
  username TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL DEFAULT '',
  partition TEXT NOT NULL DEFAULT '',
  linked_by TEXT NOT NULL DEFAULT 'system',
  linked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(project_id,job_id)
);
CREATE INDEX IF NOT EXISTS idx_project_job_links_job ON project_job_links(job_id);

CREATE TABLE IF NOT EXISTS project_activity_logs (
  id BIGSERIAL PRIMARY KEY,
  project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  actor TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL,
  target_type TEXT NOT NULL DEFAULT '',
  target_id TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  detail JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_project_activity_project_time ON project_activity_logs(project_id,created_at DESC);

WITH project_permissions(permission_key,permission_type,module_code,resource_code,action_code,name,sort_order) AS (
  VALUES
    ('menu.projects.overview.view','menu','projects','projects','view','项目中心菜单',300),
    ('route.projects.overview.view','route','projects','projects','view','访问项目中心',301),
    ('api.projects.list','api','projects','projects','list','项目列表',302),
    ('api.projects.view','api','projects','projects','view','项目详情',303),
    ('api.projects.create','api','projects','projects','create','创建项目',304),
    ('api.projects.update','api','projects','projects','update','更新项目',305),
    ('api.projects.delete','api','projects','projects','delete','删除项目',306)
)
INSERT INTO permissions (permission_key,permission_type,module_code,resource_code,action_code,name,sort_order)
SELECT permission_key,permission_type,module_code,resource_code,action_code,name,sort_order
FROM project_permissions
ON CONFLICT(permission_key) DO UPDATE SET
  permission_type=EXCLUDED.permission_type,
  module_code=EXCLUDED.module_code,
  resource_code=EXCLUDED.resource_code,
  action_code=EXCLUDED.action_code,
  name=EXCLUDED.name,
  sort_order=EXCLUDED.sort_order,
  status='active',
  updated_at=now();

INSERT INTO menus(code,parent_id,name,icon,route_path,route_permission_key,menu_permission_key,menu_type,sort_order)
VALUES
('projects',NULL,'项目中心','▣','','','menu.projects.overview.view','group',45)
ON CONFLICT(code) DO UPDATE SET
  parent_id=NULL,
  name=EXCLUDED.name,
  icon=EXCLUDED.icon,
  route_path=EXCLUDED.route_path,
  route_permission_key=EXCLUDED.route_permission_key,
  menu_permission_key=EXCLUDED.menu_permission_key,
  menu_type=EXCLUDED.menu_type,
  sort_order=EXCLUDED.sort_order,
  status='active';

INSERT INTO menus(code,parent_id,name,route_path,route_permission_key,menu_permission_key,menu_type,sort_order)
SELECT 'projects_overview',p.id,'项目总览','projects.html','route.projects.overview.view','menu.projects.overview.view','page',46
FROM menus p WHERE p.code='projects'
ON CONFLICT(code) DO UPDATE SET
  parent_id=EXCLUDED.parent_id,
  name=EXCLUDED.name,
  route_path=EXCLUDED.route_path,
  route_permission_key=EXCLUDED.route_permission_key,
  menu_permission_key=EXCLUDED.menu_permission_key,
  sort_order=EXCLUDED.sort_order,
  status='active';

INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'migration-015-project-center'
FROM roles r
JOIN permissions p ON p.permission_key IN (
  'menu.projects.overview.view','route.projects.overview.view',
  'api.projects.list','api.projects.view','api.projects.create',
  'api.projects.update','api.projects.delete'
)
WHERE r.code IN ('cluster_admin','unit_admin','team_admin')
ON CONFLICT(role_id,permission_id) DO NOTHING;

INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'migration-015-project-center'
FROM roles r
JOIN permissions p ON p.permission_key IN (
  'menu.projects.overview.view','route.projects.overview.view',
  'api.projects.list','api.projects.view','api.projects.create',
  'api.projects.update','api.projects.delete'
)
WHERE r.code='user'
ON CONFLICT(role_id,permission_id) DO NOTHING;

INSERT INTO projects(code,name,summary,owner_username,status,priority,storage_quota_gb,compute_quota_hours,license_budget_points,created_by,updated_by)
VALUES
  ('demo-cfd-airfoil','翼型 CFD 优化项目','围绕网格生成、Fluent 求解、后处理和报告输出组织数据、作业与成员。','admin','active','high',500,1200,300,'migration-015-project-center','migration-015-project-center'),
  ('demo-ai-material','AI4S 材料筛选课题','使用批量作业和 Python 工作流筛选候选材料，并沉淀项目数据空间。','admin','planning','normal',300,800,120,'migration-015-project-center','migration-015-project-center')
ON CONFLICT(code) DO NOTHING;

INSERT INTO project_members(project_id,username,display_name,role,permission,created_by)
SELECT p.id,'admin','平台管理员','owner','manage','migration-015-project-center'
FROM projects p WHERE p.code IN ('demo-cfd-airfoil','demo-ai-material')
ON CONFLICT(project_id,username) DO NOTHING;

INSERT INTO project_tasks(project_id,title,assignee_username,status,priority,due_date,description,created_by,updated_by)
SELECT p.id,'准备输入数据','admin','done','normal',CURRENT_DATE + INTERVAL '3 days','整理几何、材料参数和边界条件。','migration-015-project-center','migration-015-project-center'
FROM projects p WHERE p.code='demo-cfd-airfoil'
ON CONFLICT DO NOTHING;

INSERT INTO project_tasks(project_id,title,assignee_username,status,priority,due_date,description,created_by,updated_by)
SELECT p.id,'提交第一轮求解作业','admin','running','high',CURRENT_DATE + INTERVAL '7 days','使用作业模板提交批量计算，并在项目作业中跟踪状态。','migration-015-project-center','migration-015-project-center'
FROM projects p WHERE p.code='demo-cfd-airfoil'
ON CONFLICT DO NOTHING;

INSERT INTO project_directories(project_id,name,path,permission,created_by)
SELECT p.id,'项目数据空间','/data/projects/' || p.code,'rw','migration-015-project-center'
FROM projects p WHERE p.code IN ('demo-cfd-airfoil','demo-ai-material')
ON CONFLICT(project_id,path) DO NOTHING;
