INSERT INTO roles(
  code,name,description,scope_type,permission_summary,status,is_builtin,
  allow_delete,allow_permission_edit,created_by,updated_by
) VALUES
('cluster_admin','集群管理员','系统最高管理员','global','全模块管理权限','active',TRUE,FALSE,FALSE,'system','system'),
('config_admin','配置管理员','平台基础与运维配置','global','配置与运维管理','active',TRUE,FALSE,TRUE,'system','system'),
('unit_admin','学院/单位管理员','管理本单位用户与资源','unit','本单位数据管理','active',TRUE,FALSE,TRUE,'system','system'),
('team_admin','团队管理员','管理本团队成员与资源','team','本团队数据管理','active',TRUE,FALSE,TRUE,'system','system'),
('user','普通用户','个人作业、数据、模板和 VNC','self','个人与被授权数据','active',TRUE,FALSE,TRUE,'system','system')
ON CONFLICT(code) DO UPDATE SET
  name=EXCLUDED.name,description=EXCLUDED.description,scope_type=EXCLUDED.scope_type,
  permission_summary=EXCLUDED.permission_summary,is_builtin=TRUE,allow_delete=FALSE,
  allow_permission_edit=EXCLUDED.allow_permission_edit,updated_at=now();

INSERT INTO permissions(permission_key,permission_type,module_code,resource_code,action_code,name,sort_order)
VALUES
('menu.dashboard.view','menu','dashboard','','view','仪表盘',10),
('menu.account.units.view','menu','account','units','view','单位管理',20),
('menu.account.teams.view','menu','account','teams','view','团队管理',21),
('menu.account.users.view','menu','account','users','view','用户管理',22),
('menu.account.admins.view','menu','account','admins','view','管理员账号管理',23),
('menu.account.roles.view','menu','account','roles','view','角色管理',24),
('menu.compute.partitions.view','menu','compute','partitions','view','资源队列配置',30),
('menu.compute.queue.view','menu','compute','queue','view','队列状态',31),
('menu.compute.nodes.view','menu','compute','nodes','view','节点状态',32),
('menu.compute.qos.view','menu','compute','qos','view','QOS 策略',33),
('menu.data.files.view','menu','data','storage_files','view','数据目录',40),
('menu.data.acl.view','menu','data','storage_acl','view','访问授权',41),
('menu.jobs.templates.view','menu','jobs','job_templates','view','作业模板',50),
('menu.jobs.list.view','menu','jobs','jobs','view','作业列表',51),
('menu.jobs.vnc.view','menu','jobs','vnc_sessions','view','VNC 桌面',52),
('menu.operations.monitoring.view','menu','operations','monitoring','view','监控告警',60),
('menu.operations.inspection.view','menu','operations','inspection_reports','view','巡检报告',61),
('menu.logs.view','menu','logs','logs','view','日志中心',70),
('route.dashboard.view','route','dashboard','','view','访问仪表盘',100),
('route.queue.view','route','compute','queue','view','访问队列状态',101),
('route.data.files.view','route','data','storage_files','view','访问数据目录',102),
('route.jobs.templates.view','route','jobs','job_templates','view','访问作业模板',103),
('route.jobs.list.view','route','jobs','jobs','view','访问作业列表',104),
('route.jobs.vnc.view','route','jobs','vnc_sessions','view','访问 VNC 桌面',105),
('action.roles.view','action','account','roles','view','查看角色',200),
('action.roles.create','action','account','roles','create','新建角色',201),
('action.roles.edit','action','account','roles','edit','编辑角色',202),
('action.roles.delete','action','account','roles','delete','删除角色',203),
('action.roles.copy','action','account','roles','copy','复制角色',204),
('action.roles.assign','action','account','roles','assign','绑定用户',205),
('action.roles.permissions.manage','action','account','roles','permissions','分配权限',206),
('action.storage.view','action','data','storage_files','view','查看文件',220),
('action.storage.upload','action','data','storage_files','upload','上传文件',221),
('action.storage.download','action','data','storage_files','download','下载文件',222),
('action.storage.create_directory','action','data','storage_files','create_directory','新建目录',223),
('action.storage.delete','action','data','storage_files','delete','删除文件',224),
('action.storage.copy','action','data','storage_files','copy','复制文件',225),
('action.storage.move','action','data','storage_files','move','移动文件',226),
('action.storage.rename','action','data','storage_files','rename','重命名',227),
('action.storage.archive','action','data','storage_files','archive','打包下载',228),
('action.storage.hidden','action','data','storage_files','hidden','显示隐藏文件',229),
('action.jobs.view','action','jobs','jobs','view','查看作业',240),
('action.jobs.submit','action','jobs','jobs','submit','提交作业',241),
('action.jobs.cancel','action','jobs','jobs','cancel','取消作业',242),
('action.jobs.retry','action','jobs','jobs','retry','重试作业',243),
('action.jobs.delete','action','jobs','jobs','delete','删除作业',244),
('action.jobs.logs','action','jobs','jobs','logs','查看日志',245),
('action.jobs.download','action','jobs','jobs','download','下载结果',246),
('api.auth.me','api','auth','','me','读取当前权限',300),
('api.rbac.manage','api','account','roles','manage','管理 RBAC',301),
('api.storage.files','api','data','storage_files','access','访问文件接口',302),
('api.jobs.access','api','jobs','jobs','access','访问作业接口',303),
('api.templates.access','api','jobs','job_templates','access','访问模板接口',304),
('api.vnc.access','api','jobs','vnc_sessions','access','访问 VNC 接口',305)
ON CONFLICT(permission_key) DO UPDATE SET
  permission_type=EXCLUDED.permission_type,module_code=EXCLUDED.module_code,
  resource_code=EXCLUDED.resource_code,action_code=EXCLUDED.action_code,
  name=EXCLUDED.name,sort_order=EXCLUDED.sort_order,status='active',updated_at=now();

INSERT INTO menus(code,parent_id,name,icon,route_path,route_permission_key,menu_permission_key,menu_type,sort_order)
VALUES
('dashboard',NULL,'仪表盘','▦','index.html','route.dashboard.view','menu.dashboard.view','page',10),
('account',NULL,'账户管理','☷','','','menu.account.users.view','group',20),
('compute',NULL,'计算管理','◇','','','menu.compute.queue.view','group',30),
('data',NULL,'数据管理','□','','','menu.data.files.view','group',40),
('jobs',NULL,'作业中心','▶','','','menu.jobs.list.view','group',50),
('operations',NULL,'监控运维','◎','','','menu.operations.monitoring.view','group',60),
('logs',NULL,'日志中心','≡','','','menu.logs.view','group',70)
ON CONFLICT(code) DO UPDATE SET name=EXCLUDED.name,route_path=EXCLUDED.route_path,
  route_permission_key=EXCLUDED.route_permission_key,menu_permission_key=EXCLUDED.menu_permission_key,
  menu_type=EXCLUDED.menu_type,sort_order=EXCLUDED.sort_order,status='active';

INSERT INTO menus(code,parent_id,name,route_path,route_permission_key,menu_permission_key,menu_type,sort_order)
SELECT v.code,p.id,v.name,v.path,v.route_key,v.menu_key,'page',v.sort_order
FROM (VALUES
('units','account','单位管理','units.html','route.account.units.view','menu.account.units.view',21),
('teams','account','团队管理','teams.html','route.account.teams.view','menu.account.teams.view',22),
('users','account','用户管理','users.html','route.account.users.view','menu.account.users.view',23),
('admins','account','管理员账号管理','admins.html','route.account.admins.view','menu.account.admins.view',24),
('roles','account','角色管理','roles.html','route.account.roles.view','menu.account.roles.view',25),
('partitions','compute','资源队列配置','partitions.html','route.compute.partitions.view','menu.compute.partitions.view',31),
('queue','compute','队列状态','queue-status.html','route.queue.view','menu.compute.queue.view',32),
('nodes','compute','节点状态','nodes.html','route.compute.nodes.view','menu.compute.nodes.view',33),
('qos','compute','QOS 策略','qos.html','route.compute.qos.view','menu.compute.qos.view',34),
('files','data','数据目录','data.html','route.data.files.view','menu.data.files.view',41),
('data_acl','data','访问授权','data-acl.html','route.data.acl.view','menu.data.acl.view',42),
('templates','jobs','作业模板','job-templates.html','route.jobs.templates.view','menu.jobs.templates.view',51),
('job_list','jobs','作业列表','job-list.html','route.jobs.list.view','menu.jobs.list.view',52),
('vnc','jobs','VNC 桌面','vnc-desktop.html','route.jobs.vnc.view','menu.jobs.vnc.view',53),
('monitoring','operations','监控告警','monitoring.html','route.operations.monitoring.view','menu.operations.monitoring.view',61),
('inspection','operations','巡检报告','inspection.html','route.operations.inspection.view','menu.operations.inspection.view',62)
) AS v(code,parent_code,name,path,route_key,menu_key,sort_order)
JOIN menus p ON p.code=v.parent_code
ON CONFLICT(code) DO UPDATE SET parent_id=EXCLUDED.parent_id,name=EXCLUDED.name,
  route_path=EXCLUDED.route_path,route_permission_key=EXCLUDED.route_permission_key,
  menu_permission_key=EXCLUDED.menu_permission_key,sort_order=EXCLUDED.sort_order,status='active';

INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'system' FROM roles r CROSS JOIN permissions p
WHERE r.code='cluster_admin'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'system' FROM roles r JOIN permissions p ON p.permission_key IN (
'menu.dashboard.view','menu.compute.queue.view','menu.data.files.view',
'menu.jobs.templates.view','menu.jobs.list.view','menu.jobs.vnc.view',
'route.dashboard.view','route.queue.view','route.data.files.view',
'route.jobs.templates.view','route.jobs.list.view','route.jobs.vnc.view',
'action.storage.view','action.storage.upload','action.storage.download',
'action.storage.create_directory','action.storage.delete','action.storage.copy',
'action.storage.move','action.storage.rename','action.storage.archive',
'action.jobs.view','action.jobs.submit','action.jobs.cancel','action.jobs.logs',
'action.jobs.download','api.auth.me','api.storage.files','api.jobs.access',
'api.templates.access','api.vnc.access')
WHERE r.code='user'
ON CONFLICT DO NOTHING;

INSERT INTO role_data_scopes(role_id,resource_code,scope_type,access_level)
SELECT r.id,v.resource,v.scope,v.access FROM roles r CROSS JOIN (VALUES
('jobs','self','manage'),('vnc_sessions','self','manage'),
('storage_files','self','manage'),('job_templates','self','manage'),
('job_templates','granted','view')
) v(resource,scope,access) WHERE r.code='user'
ON CONFLICT(role_id,resource_code,scope_type)
DO UPDATE SET access_level=EXCLUDED.access_level;

INSERT INTO role_data_scopes(role_id,resource_code,scope_type,access_level)
SELECT r.id,v.resource,'global','manage' FROM roles r CROSS JOIN (VALUES
('users'),('teams'),('jobs'),('job_templates'),('vnc_sessions'),
('storage_files'),('inspection_reports'),('audit_logs')
) v(resource) WHERE r.code='cluster_admin'
ON CONFLICT(role_id,resource_code,scope_type)
DO UPDATE SET access_level=EXCLUDED.access_level;

INSERT INTO role_file_policies(role_id,storage_root,subject_scope,access_level,created_by)
SELECT r.id,v.root,'self','manage','system' FROM roles r CROSS JOIN (VALUES
('/data/home'),('/data/share'),('/data/recycle'),('/data/scratch')
) v(root) WHERE r.code='user'
ON CONFLICT(role_id,storage_root,subject_scope)
DO UPDATE SET access_level=EXCLUDED.access_level;

INSERT INTO role_file_policies(role_id,storage_root,subject_scope,access_level,allow_hidden,created_by)
SELECT r.id,v.root,'global','manage',TRUE,'system' FROM roles r CROSS JOIN (VALUES
('/data/home'),('/data/share'),('/data/recycle'),('/data/scratch')
) v(root) WHERE r.code='cluster_admin'
ON CONFLICT(role_id,storage_root,subject_scope)
DO UPDATE SET access_level=EXCLUDED.access_level,allow_hidden=TRUE;
