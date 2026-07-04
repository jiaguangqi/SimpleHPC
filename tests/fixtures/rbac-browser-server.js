const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');

const root = path.resolve(__dirname, '../..');
const port = Number(process.env.PORT || 60849);
const menuPages = [
  ['dashboard','仪表盘','index.html','menu.dashboard.view','route.dashboard.view','dashboard'],
  ['queue','队列状态','queue-status.html','menu.compute.queue.view','route.queue.view','queue'],
  ['files','数据目录','data.html','menu.data.files.view','route.data.files.view','storage_files'],
  ['templates','作业模板','job-templates.html','menu.jobs.templates.view','route.jobs.templates.view','job_templates'],
  ['job_list','作业列表','job-list.html','menu.jobs.list.view','route.jobs.list.view','jobs'],
  ['vnc','VNC 桌面','vnc-desktop.html','menu.jobs.vnc.view','route.jobs.vnc.view','vnc_sessions']
].map((v,i)=>({code:v[0],name:v[1],path:v[2],permission:v[3],routePermission:v[4],resource:v[5],type:'page',sortOrder:10+i}));
const adminMenus = [{code:'account',name:'账户管理',type:'group',sortOrder:20,children:[
  {code:'roles',name:'角色管理',path:'roles.html',permission:'menu.account.roles.view',routePermission:'route.account.roles.view',resource:'roles',type:'page',sortOrder:25}
]}].concat(menuPages);
const permissions = [
  {key:'menu.account.roles.view',type:'menu',module:'account',resource:'roles',action:'view',name:'角色管理'},
  {key:'route.account.roles.view',type:'route',module:'account',resource:'roles',action:'view',name:'访问角色管理'},
  ...['view','create','edit','delete','copy','assign','permissions.manage'].map((action,i)=>({key:'action.roles.'+action,type:'action',module:'account',resource:'roles',action,name:['查看角色','新建角色','编辑角色','删除角色','复制角色','绑定用户','分配权限'][i]}))
];
const roles = [
  {code:'cluster_admin',name:'集群管理员',description:'系统最高管理员',scopeType:'global',status:'active',isBuiltin:true,allowDelete:false,allowPermissionEdit:false,permissionSummary:'全模块管理权限',userCount:1},
  {code:'user',name:'普通用户',description:'个人作业与数据',scopeType:'self',status:'active',isBuiltin:true,allowDelete:false,allowPermissionEdit:true,permissionSummary:'个人与被授权数据',userCount:12},
  {code:'observer',name:'运维观察员',description:'只读监控角色',scopeType:'global',status:'active',isBuiltin:false,allowDelete:true,allowPermissionEdit:true,permissionSummary:'只读巡检与日志',userCount:2}
];

function sendJSON(res, value, status=200) {
  res.writeHead(status, {'content-type':'application/json; charset=utf-8'});
  res.end(JSON.stringify(value));
}
function roleConfig(role) {
  return {role,permissions:role.code==='cluster_admin'?['*']:['menu.account.roles.view','route.account.roles.view','action.roles.view'],
    dataScopes:[{resource:'roles',scope:role.scopeType,access:role.code==='observer'?'view':'manage'}],
    filePolicies:role.code==='user'?[{storageRoot:'/data/home',subjectScope:'self',access:'manage',allowHidden:false}]:[],
    bindings:[{accountType:'ldap',username:role.code==='user'?'user001':'observer01',scopeType:role.scopeType,scopeId:''}]};
}

http.createServer((req,res)=>{
  const url = new URL(req.url, `http://${req.headers.host}`);
  const actor = new URL(req.headers.referer || `http://${req.headers.host}/?actor=admin`).searchParams.get('actor') || 'admin';
  if (url.pathname === '/api/v1/auth/me') {
    const user = actor === 'user';
    const menus = user ? menuPages : adminMenus;
    return sendJSON(res,{user:{username:user?'user001':'root',displayName:user?'普通用户':'集群管理员',type:user?'ldap':'admin'},roles:[user?'user':'cluster_admin'],
      permissions:user?menus.flatMap(x=>[x.permission,x.routePermission]):['*'],dataScopes:{},accessLevels:{},menus,flatMenu:user,version:'browser-fixture'});
  }
  if (url.pathname === '/api/v1/rbac/roles') return sendJSON(res,{items:roles,count:roles.length});
  if (url.pathname === '/api/v1/rbac/menus') return sendJSON(res,{items:[{code:'account',name:'账户管理',type:'group',sortOrder:20},{...adminMenus[0].children[0],parentCode:'account'}].concat(menuPages),count:8});
  if (url.pathname === '/api/v1/rbac/permissions') return sendJSON(res,{items:permissions,count:permissions.length});
  if (url.pathname === '/api/v1/rbac/matrix') return sendJSON(res,{roles:roles.map(roleConfig),menus:[adminMenus[0].children[0]].concat(menuPages)});
  const match=url.pathname.match(/^\/api\/v1\/rbac\/roles\/([^/]+)$/);
  if(match) return sendJSON(res,roleConfig(roles.find(r=>r.code===match[1])||roles[1]));
  if (url.pathname.startsWith('/api/')) return sendJSON(res,{services:{}});
  let file=url.pathname==='/'?'roles.html':decodeURIComponent(url.pathname.slice(1));
  const target=path.resolve(root,file);
  if(!target.startsWith(root)||!fs.existsSync(target)) return res.writeHead(404).end('not found');
  let body=fs.readFileSync(target);
  if(file.endsWith('.html')) body=Buffer.from(String(body).replace('</head>','<script>localStorage.setItem("simplehpc_token","browser-test")</script></head>'));
  const ext=path.extname(file), types={'.html':'text/html; charset=utf-8','.js':'text/javascript','.css':'text/css'};
  res.writeHead(200,{'content-type':types[ext]||'application/octet-stream'});res.end(body);
}).listen(port,'127.0.0.1',()=>console.log(`RBAC browser fixture listening on ${port}`));
