# simpleHPC Dynamic RBAC Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在不破坏现有登录、作业和文件安全边界的前提下，将 simpleHPC 升级为支持内置角色、自定义角色、多角色合并和六层统一鉴权的动态 RBAC 系统。

**Architecture:** 以 PostgreSQL 权限目录和角色关系表为事实来源，后端 `PermissionResolver` 在每个请求中解析启用角色并生成统一权限上下文，Gin 中间件负责接口准入，服务层负责资源数据范围，独立文件策略解析器负责路径边界。前端通过 `/api/v1/auth/me` 获取动态菜单和权限键，统一控制菜单、页面路由和按钮展示。

**Tech Stack:** Go、Gin、PostgreSQL、Redis、原生 HTML/CSS/JavaScript、Go `testing`、`sqlmock`、Node.js 前端测试、Playwright 浏览器验收、systemd。

---

## 0. 执行约束

1. 设计基线为 `docs/plans/2026-06-30-rbac-system-design.md`。
2. 开发前创建独立工作树或完整备份；当前工作目录存在大量未跟踪文件，不得覆盖用户现有改动。
3. 所有功能按 TDD 执行：失败测试、最小实现、通过测试、阶段检查点。
4. 测试服务器 `/data/simpleHPC` 是部署目标，不是直接开发目录；先在本地完成测试，再同步到服务器。
5. 数据库迁移必须可重复、可审计、可回滚。
6. 在新旧权限双读审计完成前，不移除 `admin_users.role_name` 和旧会话字段。
7. 每个阶段完成后记录变更文件、测试结果和回滚点；没有通过阶段验收不得进入下一阶段。

## 1. 交付物总览

### 数据库

- `backend/migrations/002_rbac_schema.sql`
- `backend/migrations/003_rbac_seed.sql`
- `backend/migrations/004_rbac_bindings.sql`
- `backend/migrations/002_rbac_schema.down.sql`
- `backend/migrations/003_rbac_seed.down.sql`
- `backend/migrations/004_rbac_bindings.down.sql`
- 数据一致性检查脚本和迁移说明。

### 后端

- 权限模型、权限解析器和缓存失效。
- Gin 权限中间件。
- 数据范围过滤器。
- 文件策略与路径校验。
- 角色、权限、矩阵和绑定管理 API。
- `/api/v1/auth/me`。
- 现有接口逐模块接入 RBAC。

### 前端

- 动态侧栏、路由守卫和按钮权限工具。
- 角色列表和角色编辑弹窗。
- 菜单权限树、按钮权限树、数据范围、文件策略和绑定用户页签。
- 权限矩阵视图。
- 普通用户六项一级菜单。

### 测试与部署

- Go 单元测试与接口测试。
- Node 前端测试。
- 文件越权安全测试。
- Playwright 浏览器验收。
- 测试服务器迁移、发布、验证和回滚手册。

---

## 2. 阶段一：迁移框架和数据库结构

### Task 1：建立迁移执行与版本记录机制

**Files:**

- Create: `backend/internal/service/migrations.go`
- Create: `backend/internal/service/migrations_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/internal/service/service.go`

**Steps:**

1. 写失败测试：迁移只执行一次、失败回滚、checksum 改变时拒绝启动。
2. 运行：

   ```bash
   cd backend && go test ./internal/service -run TestMigration -v
   ```

   预期：因迁移执行器不存在而失败。
3. 创建 `schema_migrations(version, name, checksum, applied_at)`。
4. 使用 PostgreSQL advisory lock 防止多实例同时迁移。
5. 每个迁移在独立事务中执行。
6. 启动时先完成迁移，再启动 HTTP 服务。
7. 重跑测试，预期全部通过。
8. 阶段检查点：记录当前数据库版本和迁移 checksum。

### Task 2：创建 RBAC 核心表

**Files:**

- Create: `backend/migrations/002_rbac_schema.sql`
- Create: `backend/migrations/002_rbac_schema.down.sql`
- Create: `backend/internal/service/rbac_schema_test.go`

**表结构变更清单：**

1. 扩展 `roles`：
   - `description`
   - `status`
   - `is_builtin`
   - `allow_delete`
   - `allow_permission_edit`
   - `version`
   - `created_by`
   - `updated_by`
   - `updated_at`
2. 新建 `permissions`。
3. 新建 `menus`。
4. 新建 `role_permissions`。
5. 新建 `role_data_scopes`。
6. 新建 `role_file_policies`。
7. 新建统一版 `user_roles_v2`，验证后再替换旧表。
8. 新建必要索引：
   - `roles(status)`
   - `permissions(permission_type,module_code)`
   - `user_roles_v2(account_type,username,status)`
   - `role_data_scopes(role_id,resource_code)`
   - `role_file_policies(role_id,storage_root)`

**Steps:**

1. 写 schema 测试，断言所有表、列、约束和索引存在。
2. 运行测试，预期失败。
3. 编写向上迁移，所有约束使用白名单 `CHECK`。
4. 编写向下迁移；只删除本次新增对象，不删除原 `roles/user_roles`。
5. 在临时数据库执行 up → down → up。
6. 运行 schema 测试，预期通过。
7. 导出迁移前后 schema diff 供复核。

### Task 3：写入权限目录、菜单和内置角色种子

**Files:**

- Create: `backend/migrations/003_rbac_seed.sql`
- Create: `backend/migrations/003_rbac_seed.down.sql`
- Create: `backend/internal/service/rbac_seed_test.go`

**Steps:**

1. 写失败测试，验证五个内置角色、全部菜单、按钮、路由和接口权限键。
2. 写入五个内置角色：
   - `cluster_admin`
   - `config_admin`
   - `unit_admin`
   - `team_admin`
   - `user`
3. 写入菜单树及排序。
4. 写入通用、文件、作业和角色管理按钮权限。
5. 写入所有现有 `/api/v1` 路由对应的接口权限键。
6. 写入内置角色默认菜单、按钮、数据范围和文件策略。
7. 对 `cluster_admin` 采用完整权限种子和代码兜底双保险。
8. 迁移必须使用 `ON CONFLICT ... DO UPDATE` 保证可重复。
9. 运行测试，检查普通用户只有六个菜单权限。

### Task 4：迁移现有账号角色绑定

**Files:**

- Create: `backend/migrations/004_rbac_bindings.sql`
- Create: `backend/migrations/004_rbac_bindings.down.sql`
- Create: `backend/scripts/verify-rbac-migration.sh`
- Create: `backend/internal/service/rbac_migration_test.go`

**Steps:**

1. 写失败测试：
   - `admin_users.role_name` 转换为管理员绑定。
   - LDAP 用户获得 `user`。
   - 已有旧 `user_roles` 合法绑定被保留。
   - 至少存在一个有效 `cluster_admin`。
2. 迁移到 `user_roles_v2`，不覆盖原表。
3. 未识别的旧角色写迁移告警表，不静默提升权限。
4. 编写验证脚本，输出角色数、绑定数、孤儿绑定和超级管理员数量。
5. 编写 down 脚本，仅撤销自动生成的 v2 绑定。
6. 在测试数据库演练并保存结果。

**阶段一验收：**

- 所有迁移可重复执行。
- up/down/up 均成功。
- 内置角色和权限数量稳定。
- 至少一个有效 `cluster_admin`。
- 尚未改变线上实际鉴权结果。

---

## 3. 阶段二：权限内核与多角色合并

### Task 5：定义 RBAC 领域模型

**Files:**

- Create: `backend/internal/service/rbac.go`
- Create: `backend/internal/service/rbac_test.go`
- Modify: `backend/internal/service/auth.go`

**核心类型：**

```go
type PermissionContext struct {
    Username     string
    AccountType  string
    RoleCodes    []string
    Permissions  map[string]struct{}
    DataScopes   map[string]ScopeSet
    AccessLevels map[string]AccessLevel
    FilePolicies []ResolvedFilePolicy
    UnitIDs      []string
    TeamIDs      []string
    Version      string
}
```

**Steps:**

1. 写表驱动失败测试：
   - 权限键取并集。
   - `manage > view > none`。
   - `global > unit > team > self/granted > none`。
   - `self + granted` 同时保留。
   - 禁用角色不参与合并。
   - 自定义角色与内置角色规则相同。
2. 实现纯函数合并逻辑。
3. 对集合输出排序，保证缓存和测试稳定。
4. 运行测试，预期通过。

### Task 6：实现权限解析器和缓存失效

**Files:**

- Create: `backend/internal/service/rbac_resolver.go`
- Create: `backend/internal/service/rbac_resolver_test.go`
- Modify: `backend/internal/service/service.go`
- Modify: `backend/internal/service/auth.go`

**Steps:**

1. 写失败测试：
   - 从数据库解析多角色。
   - 过滤禁用角色和失效绑定。
   - 解析单位、团队作用域。
   - `cluster_admin` 完整权限兜底。
   - 修改角色版本后旧缓存不再命中。
2. Redis 缓存键：

   ```text
   authz:{accountType}:{username}:{permissionVersion}
   ```
3. TTL 设置 60 秒。
4. 角色、权限、状态或绑定变化时删除受影响用户缓存。
5. `SessionUser` 保留身份，不再把单个 `Role` 作为授权依据。
6. 实现 `ResolvePermissionContext`。
7. 运行单元测试和 Redis 集成测试。

### Task 7：实现最后一个 cluster_admin 保护

**Files:**

- Modify: `backend/internal/service/rbac.go`
- Modify: `backend/internal/service/accounts.go`
- Test: `backend/internal/service/rbac_test.go`
- Test: `backend/internal/service/accounts_test.go`

**Steps:**

1. 写失败测试：
   - 不能禁用 `cluster_admin` 内置角色。
   - 不能删除最后一个有效 `cluster_admin` 绑定。
   - 不能冻结或删除最后一个有效超级管理员账号。
   - 不能清空 `cluster_admin` 权限。
2. 所有检查放在数据库事务内并锁定相关行。
3. 成功修改后递增角色版本并清理缓存。
4. 运行并通过并发保护测试。

**阶段二验收：**

- 权限合并函数完整覆盖。
- 禁用角色最多一个请求缓存周期内失效，主动失效路径立即生效。
- `cluster_admin` 无法被误删除、禁用或清空。

---

## 4. 阶段三：后端接口、路由与数据范围

### Task 8：实现统一 Gin 鉴权中间件

**Files:**

- Create: `backend/internal/httpapi/rbac.go`
- Create: `backend/internal/httpapi/rbac_test.go`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `backend/internal/httpapi/templates.go`

**Steps:**

1. 写失败测试：401、403、授权成功、禁用角色后立即 403。
2. 实现：
   - `RequirePermission`
   - `RequireAnyPermission`
   - `RequireResourceAccess`
3. 将权限上下文放入 Gin context，单请求只解析一次。
4. 返回统一错误结构和 `requestId`。
5. 暂时保留 `requireAdmin`，标记 deprecated，禁止新代码调用。
6. 运行 HTTP 测试。

### Task 9：建立路由权限注册表并接入全部 API

**Files:**

- Create: `backend/internal/httpapi/permission_registry.go`
- Create: `backend/internal/httpapi/permission_registry_test.go`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `backend/internal/httpapi/*.go`

**步骤：**

1. 测试扫描 Gin 路由，确保除公开接口外每个 `/api/v1` 路由都有权限键。
2. 明确公开白名单：
   - 登录。
   - 密码重置。
   - 登录前平台公共配置。
   - 健康检查。
3. 为账户、Slurm、配置、巡检、日志、存储、模板、作业和 VNC 路由逐项绑定权限。
4. 首阶段以“双读审计模式”记录新旧判定差异，不改变响应。
5. 差异清零后切换为强制模式。
6. 删除遗漏的裸 `currentUser` 准入逻辑。

### Task 10：实现统一数据范围过滤器

**Files:**

- Create: `backend/internal/service/data_scope.go`
- Create: `backend/internal/service/data_scope_test.go`
- Modify: `backend/internal/service/accounts.go`
- Modify: `backend/internal/service/templates.go`
- Modify: `backend/internal/service/dashboard.go`
- Modify: `backend/internal/httpapi/router.go`

**资源覆盖：**

- 用户、团队。
- 作业、作业详情和输出。
- 作业模板和授权模板。
- VNC 会话。
- 巡检报告、日志和审计日志。

**Steps:**

1. 为每种资源写列表、详情、修改、删除范围测试。
2. 实现资源范围谓词：
   - global：不加限制。
   - unit：匹配允许单位。
   - team：匹配允许团队。
   - self：匹配 owner。
   - granted：连接授权表。
3. `self + granted` 使用 OR 集合，不覆盖。
4. 更新/删除必须先加载目标并校验范围。
5. 批量接口逐项校验。
6. 普通用户查询其他用户资源返回 403 或安全 404。
7. 运行全量 Go 测试。

### Task 11：建立角色管理 API

**Files:**

- Create: `backend/internal/service/rbac_admin.go`
- Create: `backend/internal/service/rbac_admin_test.go`
- Create: `backend/internal/httpapi/rbac_admin.go`
- Create: `backend/internal/httpapi/rbac_admin_test.go`
- Modify: `backend/internal/httpapi/router.go`
- Deprecate: `backend/internal/service/accounts.go` 中旧 `SaveRole/Roles`

**API 清单：**

- `/api/v1/rbac/permissions`
- `/api/v1/rbac/menus`
- `/api/v1/rbac/roles`
- `/api/v1/rbac/roles/:code`
- 复制、启停、删除、权限、数据范围、文件策略和用户绑定。
- `/api/v1/rbac/matrix`

**Steps:**

1. 先写所有 CRUD 和保护规则的失败测试。
2. 角色保存使用事务。
3. 角色编码创建后不可修改。
4. 复制角色不复制用户绑定。
5. 自定义角色删除前检查绑定用户。
6. 操作者只能授予自己拥有且可下放的权限。
7. 变更后写审计日志并失效缓存。
8. 返回前端可直接消费的权限树和矩阵数据。

### Task 12：实现 `/auth/me` 与动态菜单服务

**Files:**

- Modify: `backend/internal/httpapi/auth.go`
- Modify: `backend/internal/httpapi/router.go`
- Create: `backend/internal/service/navigation.go`
- Create: `backend/internal/service/navigation_test.go`

**Steps:**

1. 写失败测试：
   - 普通用户只返回六个菜单。
   - 普通用户返回 `flat=true` 且无 group 节点。
   - 管理员按权限返回树形菜单。
   - 自定义角色返回授权菜单。
2. 返回身份、角色、权限键、数据范围、菜单和版本。
3. 不返回接口内部敏感信息和文件真实策略表达式。
4. 运行测试。

**阶段三验收：**

- 除公开白名单外所有 API 均有权限键。
- 菜单、接口和数据范围使用同一权限上下文。
- 直接访问资源 ID 不能绕过列表过滤。

---

## 5. 阶段四：文件目录权限与安全边界

### Task 13：扩展文件策略解析器

**Files:**

- Modify: `backend/internal/service/storage_access.go`
- Modify: `backend/internal/service/storage_access_test.go`
- Modify: `backend/internal/httpapi/storage_access.go`

**Steps:**

1. 保留并扩展现有普通用户目录边界测试。
2. 新增失败测试：
   - 用户 + 非文件高权限角色仍不能访问他人目录。
   - 用户只有明确文件策略才能扩展路径。
   - 团队共享、团队成员只读、单位共享、单位成员管理。
   - `config_admin` 默认不能读取用户文件。
3. 从 `PermissionContext.FilePolicies` 生成规范允许路径。
4. 权限合并按“存储根 + 主体范围”取最高访问级别。
5. 不允许 `jobs.global`、菜单权限或普通 `storage_files.global` 自动扩展路径。
6. 运行文件策略单元测试。

### Task 14：覆盖所有文件操作和符号链接防护

**Files:**

- Modify: `backend/internal/httpapi/router.go`
- Modify: `backend/internal/integrations/storage/client.go`
- Modify: `backend/internal/integrations/storage/client_test.go`
- Create: `backend/internal/httpapi/storage_security_test.go`

**接口覆盖：**

- 列表、上传、下载。
- 新建、删除、复制、移动、重命名。
- 打包下载、显示隐藏文件。

**Steps:**

1. 对每个操作写源路径、目标路径越权测试。
2. 对 `../`、双斜线、`.`、URL 编码和绝对路径写测试。
3. 对现有目标使用 `EvalSymlinks`。
4. 对新建目标校验真实父目录。
5. 打包时逐项验证归档成员。
6. 后端返回 `effectivePath`、`initialPath`、`canGoParent`。
7. 越权写审计日志。
8. 运行存储集成测试。

**阶段四验收：**

- 普通用户在四个授权根均锁定到 `/授权目录/{username}`。
- 多角色并集不会隐式突破文件路径边界。
- 所有文件操作源、目标均受后端校验。

---

## 6. 阶段五：前端动态权限和角色管理

### Task 15：实现统一前端权限客户端

**Files:**

- Create: `js/rbac.js`
- Create: `tests/rbac-client.test.js`
- Modify: `js/session-ui.js`
- Modify: `js/app.js`

**Steps:**

1. 写失败测试：权限查询、缓存、刷新、按钮隐藏和 403 处理。
2. 页面启动只调用一次 `/api/v1/auth/me`。
3. 提供：

   ```javascript
   App.authz.can(permissionKey)
   App.authz.scope(resourceCode)
   App.authz.refresh()
   ```
4. 扫描 `[data-permission]` 元素。
5. 角色变更后支持刷新上下文。
6. 运行 Node 测试。

### Task 16：动态侧栏和普通用户扁平菜单

**Files:**

- Modify: `js/nav.js`
- Modify: `css/theme.css`
- Create: `tests/rbac-navigation.test.js`

**Steps:**

1. 写失败测试：
   - 普通用户六项一级菜单。
   - 不出现“概览、账户管理、计算管理、数据管理、作业中心、监控运维、日志中心”等分组标题。
   - 管理员保持树形分组。
2. 删除固定 `NAV_TREE` 作为权限事实来源；只保留接口失败时的安全空导航。
3. 根据 `/auth/me.menus` 渲染。
4. 普通用户一级菜单顺序：
   - 仪表盘。
   - 队列状态。
   - 数据目录。
   - 作业模板。
   - 作业列表。
   - VNC 桌面。
5. 无权限页面不渲染链接。
6. 运行前端测试。

### Task 17：实现静态页面路由守卫

**Files:**

- Modify: `js/app.js`
- Create: `tests/rbac-route-guard.test.js`
- Modify: all protected `*.html` to include `js/rbac.js`

**Steps:**

1. 建立页面与 `route.*` 权限映射。
2. 写直接输入管理员页面 URL 的失败测试。
3. 未授权时渲染 403 并跳至第一个可访问菜单。
4. 后端 API 仍保持独立 403。
5. 运行页面守卫测试。

### Task 18：改造角色列表

**Files:**

- Modify: `roles.html`
- Modify: `js/account-pages.js`
- Modify: `css/theme.css`
- Create: `js/roles.js`
- Create: `tests/roles-page.test.js`

**Steps:**

1. 写列表渲染测试。
2. 展示名称、编码、作用域、内置标识、权限摘要、绑定数、状态和更新时间。
3. 按权限显示新建、编辑、复制、启停、绑定、删除。
4. 内置角色危险操作置灰并说明原因。
5. 删除固定角色数组，角色选项从 API 获取。
6. 运行测试。

### Task 19：实现角色编辑弹窗六个页签

**Files:**

- Modify: `js/roles.js`
- Modify: `css/theme.css`
- Test: `tests/roles-page.test.js`

**页签：**

1. 基础信息。
2. 菜单权限。
3. 操作权限。
4. 数据范围。
5. 文件目录权限。
6. 绑定用户。

**Steps:**

1. 为每个页签写渲染和保存失败测试。
2. 菜单权限采用父子联动树。
3. 按钮按模块分组，可全选/清空。
4. 数据范围支持 `self + granted` 组合。
5. 文件策略只能选择后端返回的授权存储根。
6. 保存前展示权限变化摘要。
7. 一次提交、后端事务保存，失败不局部落库。
8. 复制角色预填权限但清空绑定用户。

### Task 20：实现角色权限矩阵

**Files:**

- Modify: `roles.html`
- Modify: `js/roles.js`
- Modify: `css/theme.css`
- Test: `tests/roles-page.test.js`

**Steps:**

1. 写矩阵测试：角色横向、菜单纵向、文字状态徽标。
2. 支持角色筛选、冻结首列和横向滚动。
3. 单元格显示：
   - 无权限、查看、管理。
   - 个人、被授权数据、本团队、本单位、全局。
4. 点击单元格打开对应角色配置。
5. 增加导出 CSV。
6. 验证 1440px 和普通笔记本宽度。

### Task 21：文件管理器前端边界回归

**Files:**

- Modify: `data.html`
- Modify: `js/app.js` 或数据目录页面脚本
- Create: `tests/storage-boundary-ui.test.js`

**Steps:**

1. 前端只使用后端 `effectivePath/initialPath/canGoParent`。
2. 位于初始目录时上一级置灰。
3. 子目录中上一级可用。
4. 禁止前端自行拼接到授权根。
5. 按按钮权限控制上传、下载、删除、复制、移动等操作。
6. 运行 UI 单元测试。

**阶段五验收：**

- 前端无固定角色和固定权限事实。
- 普通用户只见六项一级菜单。
- 角色编辑、复制、绑定和矩阵完整可用。
- 隐藏按钮与后端接口拒绝结果一致。

---

## 7. 阶段六：旧权限迁移、双读切换与回滚

### Task 22：实现双读审计模式

**Files:**

- Modify: `backend/internal/httpapi/rbac.go`
- Modify: `backend/internal/service/audit.go`
- Create: `backend/internal/service/rbac_shadow_test.go`

**Steps:**

1. 配置项：

   ```text
   RBAC_MODE=legacy|shadow|enforce
   ```
2. `shadow` 模式使用旧逻辑实际放行，同时记录新逻辑判定。
3. 记录接口、用户、旧结果、新结果和原因，不记录敏感请求体。
4. 输出差异汇总查询。
5. 差异清零后才允许 `enforce`。

### Task 23：按模块切换强制鉴权

**切换顺序：**

1. 角色、平台配置、日志。
2. 账户、单位、团队。
3. 队列、节点、QOS、监控、巡检。
4. 作业、模板、VNC。
5. 数据目录和访问授权。

**Steps:**

1. 每个模块先跑 shadow 差异。
2. 为测试账号绑定预期角色。
3. 切换该模块 enforce。
4. 运行模块回归和越权测试。
5. 观察服务日志和 403 比例。
6. 出现异常时退回 shadow，不回滚数据表。

### Task 24：回滚方案演练

**应用回滚：**

- 将 `RBAC_MODE` 改回 `legacy`。
- 恢复上一版后端二进制和前端静态文件。
- 重启 `simplehpc-backend`。

**数据库回滚：**

- enforce 期间不立即删除旧字段和旧表。
- 应用回滚不需要数据库 down。
- 只有完全废弃 RBAC 时才按 `004 → 003 → 002` 执行 down。
- down 前导出角色、权限和绑定表。

**Steps:**

1. 在测试数据库演练应用回滚。
2. 验证旧管理员仍能登录。
3. 验证旧作业和文件功能可用。
4. 再切回 enforce，确认权限数据未丢失。

**阶段六验收：**

- shadow 差异有可查询证据。
- 每个模块可独立退回旧逻辑。
- 无需删除 RBAC 数据即可应用回滚。

---

## 8. 阶段七：完整测试清单

### Task 25：Go 单元测试

运行：

```bash
cd backend
go test ./... -count=1
go test -race ./... -count=1
```

必须覆盖：

- 权限并集。
- 操作级别最大值。
- 所有数据范围组合。
- `self + granted`。
- 禁用角色即时失效。
- 自定义角色复制。
- 内置角色保护。
- 最后一个 `cluster_admin`。
- 缓存版本和主动失效。
- 路径规范化和符号链接。

预期：全部 PASS，无 race。

### Task 26：接口鉴权测试

必须为每个受保护接口验证：

1. 未登录 401。
2. 已登录未授权 403。
3. 授权角色成功。
4. 禁用角色后失败。
5. 多角色合并后成功。
6. 直接按 ID 访问越权资源失败。
7. 批量操作不能夹带越权资源。

额外验证：

- 普通用户不能调用角色管理 API。
- `config_admin` 默认不能读取用户文件。
- team/unit 数据不会跨组织。

### Task 27：文件越权测试

自动化测试矩阵：

```text
/data/home
/data/home/user002
/data/home/user001/../user002
/data/home/user001/../../
/data/home//user001
/data/home/user001/./projectA
URL 编码 ../
目录内指向外部的软链接
归档中含 ../ 的成员
```

每个路径分别测试：

- 列表、上传、下载。
- 删除、复制、移动。
- 新建、重命名。
- 打包、隐藏文件。

预期：合法路径成功，越权路径全部 403。

### Task 28：前端自动化与浏览器验收

运行：

```bash
node --test tests/*.test.js
```

Playwright 账号矩阵：

- `cluster_admin`
- `config_admin`
- `unit_admin`
- `team_admin`
- `user`
- 至少两个自定义角色。
- 一个多角色用户。

浏览器验收：

1. 普通用户六项一级菜单。
2. 无分组标题。
3. 直接打开管理员 URL 被拒绝。
4. 未授权按钮不可见。
5. 角色新建、编辑、复制、启停、删除。
6. 用户绑定立即生效。
7. 权限矩阵准确。
8. 文件上一级边界正确。
9. 多角色权限和范围符合预期。
10. 禁用角色后不重新登录也失效。

### Task 29：安全回归审计

搜索不得残留的权限判断：

```bash
rg -n 'user\\.Type\\s*[!=]=|requireAdmin\\(|Role\\s*==' backend/internal
```

允许保留的位置必须逐条注释说明是身份分类而非授权。

检查：

- SQL 参数化。
- 权限提升。
- IDOR。
- 路径穿越。
- 软链接逃逸。
- 批量接口越权。
- 缓存陈旧权限。
- 审计日志完整性。

---

## 9. 阶段八：测试服务器部署和验证

目标：

```text
服务器：10.10.38.152
项目：/data/simpleHPC
服务：simplehpc-backend
端口：8080
```

### Task 30：部署前只读检查

1. SSH 登录并确认主机身份。
2. 检查：

   ```bash
   systemctl status simplehpc-backend --no-pager
   ss -lntp
   df -h
   psql -c 'select version();'
   redis-cli ping
   ```
3. 记录当前二进制 checksum、静态文件版本、服务环境和数据库 schema。
4. 导出：
   - `roles`
   - `user_roles`
   - `admin_users`
   - `platform_users`
5. 备份 `/data/simpleHPC` 中将被替换的文件。

### Task 31：构建发布包

本地运行：

```bash
cd backend
go test ./... -count=1
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
  -o dist/simplehpc-backend-linux-amd64 ./cmd/server
sha256sum dist/simplehpc-backend-linux-amd64
```

发布包包含：

- 新后端二进制。
- 前端 HTML/CSS/JS。
- 迁移 SQL。
- 验证脚本。
- 发布清单和 checksum。

### Task 32：部署 shadow 模式

1. 上传到时间戳 release 目录，不直接覆盖。
2. 备份数据库。
3. 维护窗口内执行迁移。
4. 运行 `verify-rbac-migration.sh`。
5. 设置 `RBAC_MODE=shadow`。
6. 原子切换二进制和静态资源。
7. 重启服务。
8. 检查 health、日志和迁移版本。

### Task 33：真实集群 shadow 验证

1. 使用五类内置角色测试账号登录。
2. 执行菜单、作业、模板、VNC、日志和文件操作。
3. 汇总新旧判定差异。
4. 修正权限种子后重新验证。
5. shadow 差异未清零不得 enforce。

### Task 34：切换 enforce 并验收

1. 先启用角色和配置模块。
2. 按阶段六顺序逐模块 enforce。
3. 每次切换执行接口和浏览器回归。
4. 观察：
   - 401/403 比例。
   - 后端错误。
   - Redis 权限缓存。
   - 数据库慢查询。
   - 文件越权审计。
5. 完成用户验收清单。

### Task 35：部署回滚演练

1. 切换 `RBAC_MODE=legacy`。
2. 恢复上一 release。
3. 验证管理员、普通用户、作业和文件功能。
4. 再恢复 RBAC release。
5. 将演练时间、命令和结果写入部署记录。

**服务器最终验收标准：**

- 页面和 API 来自测试服务器真实部署。
- 普通用户只见六项一级菜单。
- 自定义角色创建、复制、绑定和禁用即时生效。
- 多角色权限按并集，数据范围按确认规则。
- `cluster_admin` 兜底有效。
- 文件路径边界全部通过。
- 旧权限体系可在配置切换后恢复。

---

## 10. 建议执行顺序与检查点

| 检查点 | 完成范围 | 是否影响实际权限 | 可回滚方式 |
|---|---|---:|---|
| CP1 | 表结构与种子 | 否 | SQL down |
| CP2 | 权限内核 | 否 | 回退二进制 |
| CP3 | shadow 鉴权 | 否 | `RBAC_MODE=legacy` |
| CP4 | 后端 enforce | 是 | 模块退回 shadow |
| CP5 | 动态前端 | 是 | 恢复静态文件 |
| CP6 | 文件策略 | 是，高风险 | 恢复后端并保留旧硬边界 |
| CP7 | 测试服务器发布 | 是 | release 原子回切 |

推荐严格按 CP1 → CP7 顺序执行，不并行修改数据库、权限中间件和文件安全逻辑。

## 11. 预计实施批次

1. 批次一：Task 1–7，数据库和权限内核。
2. 批次二：Task 8–12，后端接口和数据范围。
3. 批次三：Task 13–14，文件安全。
4. 批次四：Task 15–21，前端。
5. 批次五：Task 22–29，迁移切换与完整测试。
6. 批次六：Task 30–35，测试服务器部署和验收。

每个批次结束时向用户报告：

- 已完成任务。
- 测试结果。
- 发现的问题。
- 下一批次内容。
- 是否具备继续条件。

