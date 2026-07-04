# simpleHPC 动态 RBAC 权限体系设计

日期：2026-06-30  
状态：待评审  
范围：角色管理、权限控制、菜单与按钮、路由与接口、数据范围、文件目录访问

## 1. 建设目标

将现有基于 `admin/user` 和少量角色名称判断的权限逻辑，升级为统一的白名单 RBAC：

- 内置角色与自定义角色共存。
- 一个用户可以绑定多个启用角色。
- 多角色权限取并集；同一资源取最高操作级别与最大数据范围。
- 未授权即无权限，第一版不实现显式拒绝。
- 菜单、按钮、前端路由、后端接口、数据查询和文件路径使用同一套权限定义。
- `cluster_admin` 保留最高权限兜底，但所有高权限操作仍记录审计日志。
- 前端只负责展示和交互，后端负责最终安全判定。

## 2. 当前系统差距

当前实现存在以下结构性问题：

1. `roles` 表仅保存角色编码、名称、作用域和文字摘要，没有机器可执行的权限。
2. `admin_users.role_name` 是单角色字段；LDAP 用户虽已有 `user_roles`，但未形成统一绑定模型。
3. `AuthUser` 只携带单个 `Role`，接口大量使用 `user.Type == "admin"`。
4. 菜单树固定写在前端，角色变化不能动态生效。
5. 按钮、路由和接口没有统一权限键。
6. 作业、用户、团队等数据范围由各模块零散判断。
7. 文件管理已有用户目录边界保护，但尚未和角色文件策略统一。

因此不能只扩展角色管理页面，必须同步改造权限解析与后端校验链路。

## 3. 核心概念

### 3.1 权限键

权限键使用稳定的点分命名，不使用页面中文名称做判断：

```text
menu.dashboard.view
menu.account.users.view
action.account.users.create
action.jobs.cancel
api.account.users.list
api.storage.files.download
route.roles.view
```

菜单、操作、路由和接口是不同权限类型，但可以通过同一权限键目录管理。

### 3.2 操作级别

第一版操作级别定义为：

```text
none < view < manage
```

具体按钮仍以独立权限键表达。`manage` 只用于权限矩阵摘要，不代替具体按钮授权。

### 3.3 数据范围

规范值：

```text
global | unit | team | self | granted | none
```

按已确认规则，合并优先级为：

```text
global > unit > team > self/granted > none
```

`self` 与 `granted` 不是严格包含关系。实现时以范围集合合并：

- `self`：本人创建或本人拥有的数据。
- `granted`：通过授权表明确授权的数据。
- 同时拥有二者时结果为 `self + granted`。
- 矩阵中可显示“个人 + 被授权数据”。

### 3.4 文件访问级别

```text
none | read | manage
```

- `read`：列出、查看、下载、打包下载。
- `manage`：在 `read` 基础上允许上传、新建、删除、复制、移动、重命名。

文件权限还必须包含“主体范围”，不能仅保存目录：

```text
self | team_shared | team_members | unit_shared | unit_members | global
```

## 4. 数据库模型

### 4.1 `roles` 角色表

```sql
CREATE TABLE roles (
  id                  BIGSERIAL PRIMARY KEY,
  code                TEXT NOT NULL UNIQUE,
  name                TEXT NOT NULL,
  description         TEXT NOT NULL DEFAULT '',
  scope_type          TEXT NOT NULL CHECK
                      (scope_type IN ('global','unit','team','self')),
  status              TEXT NOT NULL DEFAULT 'active' CHECK
                      (status IN ('active','disabled')),
  is_builtin          BOOLEAN NOT NULL DEFAULT FALSE,
  allow_delete        BOOLEAN NOT NULL DEFAULT TRUE,
  allow_permission_edit BOOLEAN NOT NULL DEFAULT TRUE,
  permission_summary  TEXT NOT NULL DEFAULT '',
  version             BIGINT NOT NULL DEFAULT 1,
  created_by          TEXT NOT NULL DEFAULT 'system',
  updated_by          TEXT NOT NULL DEFAULT 'system',
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

规则：

- 五个内置角色 `is_builtin=true`、`allow_delete=false`。
- `cluster_admin.allow_permission_edit=false`，防止误清空最高权限。
- 其他内置角色可作为模板；是否允许修改由字段控制。
- 自定义角色编码创建后不可修改，避免历史审计和引用失效。
- 每次权限或状态变化递增 `version`。

### 4.2 `permissions` 权限目录表

```sql
CREATE TABLE permissions (
  id            BIGSERIAL PRIMARY KEY,
  permission_key TEXT NOT NULL UNIQUE,
  permission_type TEXT NOT NULL CHECK
                  (permission_type IN ('menu','action','route','api')),
  module_code   TEXT NOT NULL,
  resource_code TEXT NOT NULL DEFAULT '',
  action_code   TEXT NOT NULL DEFAULT '',
  name          TEXT NOT NULL,
  description   TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'active',
  sort_order    INTEGER NOT NULL DEFAULT 0,
  is_system     BOOLEAN NOT NULL DEFAULT TRUE,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

权限目录由系统迁移脚本维护。角色可以选择权限，但普通管理员不能随意创建新的接口权限键，避免配置出不存在或危险的权限。

### 4.3 `menus` 菜单表

```sql
CREATE TABLE menus (
  id              BIGSERIAL PRIMARY KEY,
  code            TEXT NOT NULL UNIQUE,
  parent_id       BIGINT REFERENCES menus(id),
  name            TEXT NOT NULL,
  icon            TEXT NOT NULL DEFAULT '',
  route_path      TEXT NOT NULL DEFAULT '',
  route_permission_key TEXT NOT NULL DEFAULT '',
  menu_permission_key  TEXT NOT NULL,
  menu_type       TEXT NOT NULL CHECK
                  (menu_type IN ('group','page')),
  sort_order      INTEGER NOT NULL DEFAULT 0,
  status          TEXT NOT NULL DEFAULT 'active',
  metadata        JSONB NOT NULL DEFAULT '{}'::jsonb
);
```

说明：

- 菜单本身存储在数据库，可按权限动态返回。
- `group` 是管理员侧栏分组，`page` 是实际页面。
- 普通用户导航通过接口返回扁平化 `page` 列表，不返回分组节点。
- 菜单是否显示由 `menu_permission_key` 判断；访问页面还需 `route_permission_key`。

### 4.4 `role_permissions` 角色权限关系表

```sql
CREATE TABLE role_permissions (
  role_id       BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
  created_by    TEXT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (role_id, permission_id)
);
```

第一版仅保存允许项，不保存 deny。

### 4.5 `user_roles` 用户角色关系表

现有表升级为统一账号绑定表：

```sql
CREATE TABLE user_roles (
  id            BIGSERIAL PRIMARY KEY,
  account_type  TEXT NOT NULL CHECK (account_type IN ('admin','ldap')),
  username      TEXT NOT NULL,
  role_id       BIGINT NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
  scope_type    TEXT NOT NULL CHECK
                (scope_type IN ('global','unit','team','self')),
  scope_id      TEXT NOT NULL DEFAULT '*',
  status        TEXT NOT NULL DEFAULT 'active',
  valid_from    TIMESTAMPTZ,
  valid_until   TIMESTAMPTZ,
  created_by    TEXT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(account_type, username, role_id, scope_type, scope_id)
);
```

说明：

- 管理员与 LDAP 用户统一支持多角色。
- `scope_id` 保存单位 ID、团队 ID 或 `*`。
- 用户组织归属仍来自 `platform_users.unit_id/team_id`。
- 角色声明的 `scope_type` 是角色最大作用域；绑定时不能配置比角色更大的作用域。
- `admin_users.role_name` 在兼容期保留，只用于迁移和回滚，不再作为权限事实来源。

### 4.6 `role_data_scopes` 角色数据范围表

```sql
CREATE TABLE role_data_scopes (
  role_id       BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  resource_code TEXT NOT NULL,
  scope_type    TEXT NOT NULL CHECK
                (scope_type IN ('global','unit','team','self','granted','none')),
  access_level  TEXT NOT NULL DEFAULT 'view' CHECK
                (access_level IN ('none','view','manage')),
  PRIMARY KEY (role_id, resource_code, scope_type)
);
```

典型 `resource_code`：

```text
users
teams
jobs
job_templates
vnc_sessions
storage_files
inspection_reports
audit_logs
```

同一角色可同时拥有 `self` 与 `granted` 两行。

### 4.7 `role_file_policies` 文件目录策略表

```sql
CREATE TABLE role_file_policies (
  id             BIGSERIAL PRIMARY KEY,
  role_id        BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  storage_root   TEXT NOT NULL,
  subject_scope  TEXT NOT NULL CHECK
                 (subject_scope IN
                  ('self','team_shared','team_members',
                   'unit_shared','unit_members','global')),
  access_level   TEXT NOT NULL CHECK
                 (access_level IN ('read','manage')),
  allow_hidden   BOOLEAN NOT NULL DEFAULT FALSE,
  created_by     TEXT NOT NULL,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(role_id, storage_root, subject_scope)
);
```

`storage_root` 必须引用已配置的存储根目录，不允许角色页面直接录入任意系统路径。

### 4.8 权限变更审计

所有角色、绑定和权限修改写入现有审计系统，至少记录：

- 操作人、时间、IP。
- 角色编码与版本。
- 修改前后差异。
- 绑定或解绑的用户。
- 权限键、数据范围和文件策略变化。

## 5. 内置角色默认策略

### 5.1 `cluster_admin`

- 作用域：`global`
- 拥有全部系统权限。
- 所有数据资源为 `global/manage`。
- 文件策略为授权存储根目录 `global/manage`。
- 不允许删除、禁用或清空权限。

### 5.2 `config_admin`

- 平台、LDAP、Slurm、存储根、通知、队列、节点、QOS、监控、巡检和系统日志配置权限。
- 默认不授予用户数据目录、用户作业内容和用户文件读取权限。
- 如需查看用户数据，必须额外绑定自定义角色。

### 5.3 `unit_admin`

- 作用域：`unit`
- 用户、团队、作业、模板、资源统计默认 `unit`。
- 文件默认仅本单位共享目录；单位成员目录访问需单独选择 `read/manage`。

### 5.4 `team_admin`

- 作用域：`team`
- 团队成员、团队作业、团队模板、团队资源统计默认 `team`。
- 文件默认团队共享目录；团队成员目录访问需单独选择 `read/manage`。

### 5.5 `user`

- 作用域：`self`
- 固定默认菜单：仪表盘、队列状态、数据目录、作业模板、作业列表、VNC 桌面。
- 作业、VNC 和个人数据为 `self`。
- 作业模板为 `self + granted`。
- 文件策略为四个授权根的 `self/manage`。

## 6. 权限解析与合并

### 6.1 权限上下文

每次认证后端解析为：

```json
{
  "username": "user001",
  "accountType": "ldap",
  "roles": ["user", "job_reviewer"],
  "permissions": ["menu.dashboard.view", "action.jobs.view"],
  "dataScopes": {
    "jobs": ["self", "team"],
    "job_templates": ["self", "granted"]
  },
  "accessLevels": {
    "jobs": "manage"
  },
  "unitIds": ["12"],
  "teamIds": ["31"],
  "roleVersion": "hash"
}
```

### 6.2 合并算法

1. 读取当前账号所有绑定关系。
2. 丢弃禁用角色、禁用绑定和不在有效期内的绑定。
3. 如果含启用的 `cluster_admin`，产生完整权限上下文。
4. 普通权限键做集合并集。
5. 同一资源的操作级别取 `manage > view > none`。
6. 数据范围做集合并集，并计算用于矩阵显示的最高范围。
7. `global` 覆盖其他查询限制；否则保留 `unit/team/self/granted` 的并集条件。
8. 文件策略按“存储根 + 主体范围”合并，访问级别取最高值。
9. 角色或绑定变更后递增版本并主动失效权限缓存。

### 6.3 `self/granted` 特殊处理

虽然展示优先级写作 `self/granted`，查询不能简单二选一：

```sql
WHERE owner_username = :username
   OR resource_id IN (
      SELECT resource_id FROM resource_grants WHERE grantee = :username
   )
```

### 6.4 禁用立即生效

会话中不永久保存完整权限快照。推荐：

- Redis 缓存权限上下文，TTL 60 秒。
- 缓存键包含用户权限版本。
- 修改角色、权限或绑定后，删除受影响用户缓存。
- 每个请求都通过 `PermissionResolver` 获取当前上下文。

因此角色禁用后无需用户重新登录即可失效。

## 7. 后端鉴权设计

### 7.1 统一中间件

新增：

```go
RequirePermission("api.account.users.list")
RequireAnyPermission(...)
RequireDataAccess("jobs", "view")
RequireFileAccess("manage")
```

接口路由显式绑定权限键：

```go
v1.GET(
  "/account/users",
  api.RequirePermission("api.account.users.list"),
  api.accountUsers,
)
```

禁止继续新增 `user.Type == "admin"` 形式的业务鉴权。

### 7.2 服务层二次校验

中间件只回答“能否调用该类接口”，服务层必须回答“能访问哪些记录”：

```go
scope := authz.DataScope("jobs")
query = ScopeJobs(query, scope, authz.Username, authz.UnitIDs, authz.TeamIDs)
```

所有读取、更新、删除都应用相同范围。不能只过滤列表而允许通过 ID 访问详情。

### 7.3 资源所有权校验

更新、删除、取消作业等操作：

1. 查询目标资源。
2. 根据资源 owner/unit/team/grants 与权限上下文判断。
3. 不满足时统一返回 `403`，不泄露其他用户资源细节。
4. 对不存在和无权限资源可按敏感程度统一返回 `404`。

### 7.4 权限不足响应

```json
{
  "error": "forbidden",
  "message": "当前账号无权执行此操作",
  "permission": "action.jobs.cancel",
  "requestId": "..."
}
```

生产环境可不向普通用户返回权限键，但日志中必须保留。

## 8. 文件路径权限设计

文件访问不使用普通数据范围的“最大范围”直接放行，必须经过独立路径解析器。

### 8.1 普通用户硬边界

即使普通用户还拥有其他不含文件策略的角色，默认仍只能访问：

```text
/data/home/{username}
/data/share/{username}
/data/recycle/{username}
/data/scratch/{username}
```

角色并集只能增加“明确配置的文件策略”，不能因为获得 `jobs.global` 或普通 `data.global` 自动扩展文件路径。

### 8.2 路径解析步骤

1. 从配置表读取授权存储根，拒绝任意根路径。
2. 根据角色文件策略和当前用户组织关系生成允许的规范路径集合。
3. 对请求路径执行 `filepath.Clean`。
4. 对现有目标执行符号链接解析；对新建目标解析父目录真实路径。
5. 使用路径分段比较，禁止字符串前缀误判。
6. 校验源路径和目标路径，复制、移动、打包均不能漏检。
7. 用户位于其初始目录时后端返回 `canGoParent=false`。
8. 越界访问返回 `403` 并写审计日志。

### 8.3 管理员文件范围

- `cluster_admin`：全部授权存储根。
- `config_admin`：默认只能配置根目录元数据，不能读取用户文件。
- `team_admin/unit_admin`：只有角色明确配置相应文件策略才生成团队或单位路径。

## 9. 前端权限控制

### 9.1 当前用户权限接口

```http
GET /api/v1/auth/me
```

响应包含用户信息、角色摘要、权限键、数据范围和动态菜单。前端不自行推导角色权限。

### 9.2 菜单渲染

- 管理员：按数据库菜单树和菜单权限渲染分组。
- 普通用户：接口返回扁平模式，只展示六个一级菜单，不显示任何分组标题。
- 自定义角色：按实际授权菜单返回，可配置为树形或扁平展示；默认按账号类型和菜单层级渲染。
- 无菜单权限的页面不生成链接。

### 9.3 路由守卫

静态 HTML 当前没有真正前端路由器，因此每个页面加载时：

1. 调用 `/auth/me`。
2. 检查当前页面对应 `route.*` 权限。
3. 无权限则替换页面内容为 403，并跳转到第一个可访问菜单。
4. 后端对应接口仍独立拒绝，不能依赖此步骤保障安全。

### 9.4 按钮控制

页面元素使用：

```html
<button data-permission="action.jobs.cancel">取消</button>
```

统一权限组件隐藏或禁用无权按钮。动态菜单和按钮权限变更在刷新权限上下文后立即反映。

## 10. 角色管理页面

### 10.1 角色列表

字段：

- 名称、编码、作用域。
- 内置/自定义标识。
- 权限摘要。
- 绑定用户数。
- 启用状态。
- 更新时间。

操作：

- 新建、编辑、复制、启用/禁用、绑定用户、权限配置、删除。
- 内置角色根据保护字段禁用危险操作。

### 10.2 角色编辑弹窗

沿用项目统一的居中弹窗，使用页签：

1. 基础信息。
2. 菜单权限。
3. 操作权限。
4. 数据范围。
5. 文件目录权限。
6. 绑定用户。

保存采用事务：任一部分失败则整体回滚。

### 10.3 复制角色

- 复制基础信息之外的菜单、操作、数据范围、文件策略。
- 不复制绑定用户。
- 新角色始终为自定义角色、可编辑、可删除。
- 新编码必须唯一。

### 10.4 权限矩阵

- 横向：所有启用角色，可筛选内置/自定义。
- 纵向：菜单或资源。
- 单元格显示“无权限、查看、管理、个人、本团队、本单位、全局、被授权数据”等文字徽标。
- 点击单元格进入该角色对应权限页签。
- 支持冻结首列、横向滚动和导出。

## 11. 管理 API

建议新增：

```text
GET    /api/v1/auth/me
GET    /api/v1/rbac/permissions
GET    /api/v1/rbac/menus
GET    /api/v1/rbac/roles
POST   /api/v1/rbac/roles
GET    /api/v1/rbac/roles/:code
PUT    /api/v1/rbac/roles/:code
DELETE /api/v1/rbac/roles/:code
POST   /api/v1/rbac/roles/:code/copy
PUT    /api/v1/rbac/roles/:code/status
PUT    /api/v1/rbac/roles/:code/permissions
PUT    /api/v1/rbac/roles/:code/data-scopes
PUT    /api/v1/rbac/roles/:code/file-policies
GET    /api/v1/rbac/roles/:code/users
PUT    /api/v1/rbac/roles/:code/users
GET    /api/v1/rbac/matrix
```

接口自身也受 RBAC 管理，例如：

```text
action.roles.view
action.roles.create
action.roles.edit
action.roles.delete
action.roles.copy
action.roles.assign
action.roles.permissions.manage
```

## 12. 安全约束

1. 权限默认关闭，新增接口未登记权限键时启动检查失败或仅 `cluster_admin` 可访问。
2. 禁止删除或禁用最后一个拥有有效 `cluster_admin` 的账号绑定。
3. 禁止普通角色授予超过其操作者自身权限的权限，防止权限提升。
4. 分配角色时，操作者不能创建超出其可管理组织范围的绑定。
5. 所有角色和绑定变更写审计日志。
6. 文件路径必须防目录穿越、符号链接逃逸、源目标路径越权和压缩包路径逃逸。
7. 批量接口逐条校验资源范围，不能只校验请求入口。
8. 后端不信任前端传入的 username、unit_id、team_id，均从会话和数据库关系解析。

## 13. 迁移与兼容方案

### 阶段 A：建表与种子

- 扩展 `roles`。
- 创建权限目录、菜单、关系、数据范围和文件策略表。
- 写入五个内置角色及默认权限。
- 建立唯一索引和约束。

### 阶段 B：账号绑定迁移

- `admin_users.role_name` 转换为 `user_roles(account_type='admin')`。
- 所有 LDAP 用户至少绑定 `user`。
- 校验至少存在一个有效 `cluster_admin`。

### 阶段 C：双读验证

- 新权限解析器运行在审计模式。
- 旧逻辑继续实际放行，新逻辑只记录差异。
- 对关键接口比较旧、新判定，修正种子权限。

### 阶段 D：后端切换

- 先切换角色管理、配置和日志等管理员接口。
- 再切换作业、模板、VNC、账户数据范围。
- 最后接入文件策略，保留现有硬边界测试。

### 阶段 E：前端切换

- `/auth/me` 驱动导航、路由和按钮。
- 上线角色编辑和矩阵。
- 移除固定角色下拉和固定导航树。

### 阶段 F：清理

- 清除业务代码里的 `user.Type == admin` 权限判断。
- 兼容期结束后停止读取 `admin_users.role_name`。

## 14. 测试方案

### 14.1 单元测试

- 多角色权限并集。
- 操作级别最大值。
- `self + granted` 联合范围。
- 禁用角色立即失效。
- 最后一个 `cluster_admin` 保护。
- 文件策略合并不受普通数据范围误扩展。

### 14.2 接口测试

- 每个路由：未登录 401、未授权 403、授权成功。
- 列表、详情、修改、删除均验证数据范围。
- 直接输入 URL 和直接调用 API 都不能越权。
- 自定义角色新增、复制、禁用、删除和绑定即时生效。

### 14.3 文件安全回归

- 用户根目录上一级置灰。
- `../`、双斜杠、`.`、绝对路径、URL 编码绕过。
- 软链接逃逸。
- 复制、移动、打包、上传、隐藏文件操作越权。
- 多角色合并后仍不能无策略访问其他用户目录。

### 14.4 浏览器验收

- 普通用户仅显示六个一级菜单，无分组标题。
- 管理员看到完整菜单。
- 按钮随权限变化。
- 权限矩阵内容和后端实际判定一致。

## 15. 实施边界与评审结论

本设计第一版明确不包含：

- 显式 deny。
- 条件表达式策略语言。
- 跨集群联合权限。
- 用户自行申请角色的审批流。

建议评审确认以下结论后进入实施：

1. 采用权限目录表 + 关系表，不把整套权限塞入角色 JSON。
2. `self` 和 `granted` 查询按集合并集，不简单用单一优先级覆盖。
3. 文件权限独立于普通数据范围，必须存在明确文件策略才能扩展路径。
4. `cluster_admin` 不允许禁用、删除或清空权限。
5. 迁移采用“建表、双读审计、分模块切换”，避免一次性替换导致管理员被锁在系统外。

