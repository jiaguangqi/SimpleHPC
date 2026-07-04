-- WebSSH real API, route, menu, and action permissions.
WITH webssh_permissions(permission_key,permission_type,module_code,resource_code,action_code,name,sort_order) AS (VALUES
  ('menu.webssh.view','menu','job','webssh','view','WebSSH：菜单可见',520),
  ('route.webssh.view','route','job','webssh','view','WebSSH：页面访问',521),
  ('api.webssh.nodes.list','api','job','webssh.nodes','list','WebSSH：查看登录节点',522),
  ('api.webssh.files.tree','api','job','webssh.files','tree','WebSSH：查看目录树',523),
  ('api.webssh.files.list','api','job','webssh.files','list','WebSSH：列出文件',524),
  ('api.webssh.files.upload','api','job','webssh.files','upload','WebSSH：上传文件',525),
  ('api.webssh.files.download','api','job','webssh.files','download','WebSSH：下载文件',526),
  ('api.webssh.files.mkdir','api','job','webssh.files','mkdir','WebSSH：新建目录',527),
  ('api.webssh.files.delete','api','job','webssh.files','delete','WebSSH：删除文件',528),
  ('api.webssh.files.rename','api','job','webssh.files','rename','WebSSH：重命名文件',529),
  ('api.webssh.files.copy','api','job','webssh.files','copy','WebSSH：复制文件',530),
  ('api.webssh.files.move','api','job','webssh.files','move','WebSSH：移动文件',531),
  ('api.webssh.files.archive','api','job','webssh.files','archive','WebSSH：打包下载',532),
  ('api.webssh.sessions.create','api','job','webssh.sessions','create','WebSSH：创建终端会话',533),
  ('api.webssh.sessions.list','api','job','webssh.sessions','list','WebSSH：查看终端会话',534),
  ('api.webssh.sessions.resize','api','job','webssh.sessions','resize','WebSSH：调整终端尺寸',535),
  ('api.webssh.sessions.reconnect','api','job','webssh.sessions','reconnect','WebSSH：重连终端会话',536),
  ('api.webssh.sessions.delete','api','job','webssh.sessions','delete','WebSSH：关闭终端会话',537),
  ('api.webssh.sessions.ws','api','job','webssh.sessions','ws','WebSSH：终端 WebSocket',538),
  ('action.webssh.terminal.create','action','job','webssh.terminal','create','WebSSH：新建终端按钮',539),
  ('action.webssh.terminal.close','action','job','webssh.terminal','close','WebSSH：关闭终端按钮',540),
  ('action.webssh.terminal.reconnect','action','job','webssh.terminal','reconnect','WebSSH：重连终端按钮',541),
  ('action.webssh.terminal.resize','action','job','webssh.terminal','resize','WebSSH：终端 resize',542),
  ('action.webssh.files.upload','action','job','webssh.files','upload','WebSSH：上传按钮',543),
  ('action.webssh.files.download','action','job','webssh.files','download','WebSSH：下载按钮',544),
  ('action.webssh.files.mkdir','action','job','webssh.files','mkdir','WebSSH：新建目录按钮',545),
  ('action.webssh.files.delete','action','job','webssh.files','delete','WebSSH：删除按钮',546),
  ('action.webssh.files.rename','action','job','webssh.files','rename','WebSSH：重命名按钮',547),
  ('action.webssh.files.copy','action','job','webssh.files','copy','WebSSH：复制按钮',548),
  ('action.webssh.files.move','action','job','webssh.files','move','WebSSH：移动按钮',549),
  ('action.webssh.files.archive','action','job','webssh.files','archive','WebSSH：打包下载按钮',550)
)
INSERT INTO permissions(permission_key,permission_type,module_code,resource_code,action_code,name,description,status,sort_order,is_system)
SELECT permission_key,permission_type,module_code,resource_code,action_code,name,'WebSSH 功能权限点','active',sort_order,TRUE
FROM webssh_permissions
ON CONFLICT(permission_key) DO UPDATE SET
  permission_type=EXCLUDED.permission_type,
  module_code=EXCLUDED.module_code,
  resource_code=EXCLUDED.resource_code,
  action_code=EXCLUDED.action_code,
  name=EXCLUDED.name,
  status='active',
  updated_at=now();

INSERT INTO role_permissions(role_id,permission_id,created_by)
SELECT r.id,p.id,'migration-013-webssh-api-permissions'
FROM roles r
JOIN permissions p ON p.permission_key LIKE 'api.webssh.%'
  OR p.permission_key LIKE 'action.webssh.%'
  OR p.permission_key IN ('menu.webssh.view','route.webssh.view')
WHERE r.code IN ('cluster_admin','config_admin','unit_admin','team_admin','user')
ON CONFLICT DO NOTHING;
