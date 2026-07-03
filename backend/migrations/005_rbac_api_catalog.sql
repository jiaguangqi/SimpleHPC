WITH resources(code,name) AS (VALUES
('auth','认证'),
('dashboard','仪表盘'),
('roles','角色'),
('users','用户'),
('admins','管理员账号'),
('teams','团队'),
('units','单位'),
('accounts','账户同步'),
('jobs','作业'),
('queue','队列状态'),
('nodes','节点'),
('qos','QOS'),
('partitions','资源队列'),
('storage.files','数据目录'),
('storage.acls','存储授权'),
('storage.roots','存储根'),
('templates','作业模板'),
('inspection','巡检'),
('monitoring','监控'),
('logs','日志'),
('config.platform','平台配置'),
('config.platform.assets.:kind','平台资源'),
('config.slurm','Slurm 配置'),
('config.ldap','LDAP 配置'),
('config.notify','通知配置'),
('config.notify.email.test','邮件测试'),
('config.notify.feishu.test','飞书测试')
), actions(code,name) AS (VALUES
('list','查看列表'),('view','查看详情'),('create','新增'),('update','编辑'),
('delete','删除'),('cancel','取消'),('suspend','挂起'),('resume','恢复'),
('publish','发布'),('review','审核'),('test','测试'),('refresh','刷新'),
('access','访问')
)
INSERT INTO permissions(
  permission_key,permission_type,module_code,resource_code,action_code,name,
  description,status,sort_order,is_system
)
SELECT 'api.'||r.code||'.'||a.code,'api',split_part(r.code,'.',1),r.code,a.code,
  r.name||'：'||a.name,'API 路由权限点','active',400,TRUE
FROM resources r CROSS JOIN actions a
ON CONFLICT(permission_key) DO UPDATE SET
  permission_type='api',resource_code=EXCLUDED.resource_code,
  action_code=EXCLUDED.action_code,name=EXCLUDED.name,status='active',updated_at=now();

INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'system' FROM roles r CROSS JOIN permissions p
WHERE r.code='cluster_admin' AND p.permission_type='api'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'system' FROM roles r JOIN permissions p ON p.permission_key IN (
  'api.auth.create','api.auth.view','api.auth.list',
  'api.dashboard.list','api.queue.list',
  'api.storage.roots.list',
  'api.jobs.list','api.jobs.view','api.jobs.cancel',
  'api.storage.files.list','api.storage.files.view','api.storage.files.create',
  'api.storage.files.update','api.storage.files.delete',
  'api.templates.list','api.templates.view','api.templates.create','api.templates.update'
)
WHERE r.code='user'
ON CONFLICT DO NOTHING;
