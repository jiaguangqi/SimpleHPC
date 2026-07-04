# 批次六问题修复与 enforce 前置加固结果

## 1. 当前结论

- 普通用户仪表盘最近作业越权问题已修复并重新部署测试服务器。
- `user`、`team_admin`、`unit_admin`、`cluster_admin` 的仪表盘作业范围验证通过。
- 原 8 条 shadow 差异已经逐条定位。
- 修复部署后的新增 shadow 判定为 `18/18 match，0 mismatch`。
- 24 小时统计仍保留历史 8 条 mismatch，没有删除或篡改审计记录。
- TOCTOU 第一阶段已落地，但递归删除、复制、移动和归档目录枚举还未全部改造成 fd-relative 操作。

因此：**仍不建议进入 enforce。**

## 2. 仪表盘越权根因

`GET /api/v1/slurm/jobs` 已通过 `scopeJobQuery` 应用作业数据范围，但 `GET /api/v1/dashboard` 直接调用全局 `DashboardSnapshot`。

原 `dashboardJobs` 的以下查询均未接收当前用户的 `JobQuery`：

- 作业总数；
- 运行中作业数；
- 排队作业数；
- 最近 5 条作业。

因此普通用户作业列表正确，但仪表盘聚合接口仍返回全局最近作业。

## 3. 修复方案

1. Dashboard API 从当前认证上下文取得 `PermissionContext`。
2. 使用统一的 `ScopeJobQueryByPermission` 解析作业范围。
3. 将范围条件传入 Dashboard 服务。
4. 总数、运行数、排队数、最近作业使用同一份范围 SQL。
5. shadow 模式也应用该数据安全边界，不再返回全局作业后仅做对比。
6. 增加四类范围单元测试：
   - self；
   - team；
   - unit；
   - global。

## 4. 修改文件

### 仪表盘与 shadow

- `backend/internal/httpapi/router.go`
- `backend/internal/httpapi/rbac.go`
- `backend/internal/httpapi/storage_access.go`
- `backend/internal/httpapi/dashboard_scope_test.go`
- `backend/internal/httpapi/permission_registry_test.go`
- `backend/internal/service/dashboard.go`

### TOCTOU 加固

- `backend/internal/integrations/storage/client.go`
- `backend/internal/integrations/storage/secure_open_linux.go`
- `backend/internal/integrations/storage/secure_open_other.go`
- `backend/internal/integrations/storage/secure_open_test.go`

## 5. 原 8 条 shadow 差异逐条分析

| 序号 | 账号 | 请求路径 | legacy | RBAC | 原因 | 预期差异 | 处理 |
|---:|---|---|---|---|---|---|---|
| 1 | batch6_unit | GET `/api/v1/storage/list` | deny | allow | 单位管理员请求超出文件策略范围，最终 403 被误当作 API 鉴权拒绝 | 是，文件路径策略独立拒绝 | 已修比较器 |
| 2 | batch6_team | GET `/api/v1/storage/list` | deny | allow | 团队管理员请求超出文件策略范围，最终 403 被误当作 API 鉴权拒绝 | 是 | 已修比较器 |
| 3 | batch6_observer | GET `/api/v1/rbac/roles` | allow | deny | 旧逻辑只看 admin 类型，自定义观察员继承了管理员全权限；RBAC 白名单正确拒绝 | 是，新旧模型真实差异 | 不放宽 RBAC；停止用旧 admin 类型代表自定义只读角色 |
| 4 | batch6_unit | GET `/api/v1/storage/list` | deny | allow | 跨单位目录访问被独立路径策略拒绝 | 是 | 已修比较器 |
| 5 | batch6_team | GET `/api/v1/storage/list` | deny | allow | 跨团队目录访问被独立路径策略拒绝 | 是 | 已修比较器 |
| 6 | user001 | GET `/api/v1/storage/list` | deny | allow | 请求授权根 `/data/home`，路径边界返回 403 | 是 | 已修比较器 |
| 7 | user001 | GET `/api/v1/storage/list` | deny | allow | 请求 `/data/home/user002`，路径边界返回 403 | 是 | 已修比较器 |
| 8 | user001 | GET `/api/v1/storage/list` | deny | allow | 使用 `../` 尝试跨目录，路径边界返回 403 | 是 | 已修比较器 |

### 比较器根因

原 shadow 中间件使用最终 HTTP 状态判断 legacy 是否允许：

`403 => legacy denied`

但文件接口有两层独立控制：

1. API 权限；
2. 文件路径策略。

当 API 权限允许、文件路径策略正确返回 403 时，原比较器错误地记录为 `legacy=false, rbac=true`。

修复后，文件策略拒绝会设置明确的 downstream policy 标记。shadow 比较器只比较 API 层权限，同时文件越权仍返回 403 并写审计日志。

## 6. 修复后的 shadow 统计

### 部署后增量

统计窗口：修复部署时间 `2026-07-01 13:59:00+08` 之后。

- total：18
- matched：18
- mismatched：0
- match rate：100%

覆盖：

- 登录/auth；
- dashboard；
- 普通用户文件根越权；
- unit_admin 跨单位文件越权；
- team_admin 跨团队文件越权；
- 普通用户本人文件列表。

### 24 小时历史窗口

`/api/v1/rbac/shadow/stats?hours=24` 当前仍包含部署前记录：

- total：56
- matched：48
- mismatched：8
- match rate：85.71%

历史记录按审计要求保留。只有当 24 小时滚动窗口中的历史差异自然退出、且新增流量持续为 0 mismatch 后，才可认为观测接口清零。

## 7. 各角色仪表盘验证

使用四条隔离测试作业验证后已清理测试数据。

| 角色 | 结果 |
|---|---|
| user001 | total=10，最近 5 条全部为 user001，foreign=0 |
| team_admin | total=1，仅包含 team 15 成员作业 |
| unit_admin | total=2，仅包含 unit 1126 用户作业 |
| cluster_admin | 可见四条跨单位、跨团队测试作业及全局作业 |

浏览器使用真实 LDAP 用户登录，仪表盘最近 5 条全部为 `user001`，未出现 `root`、`user002` 或测试账号；控制台错误为 0。

## 8. TOCTOU 加固

### 已实现

Linux 文件服务新增：

- 优先使用 `openat2`；
- `RESOLVE_BENEATH`；
- `RESOLVE_NO_MAGICLINKS`；
- `RESOLVE_NO_SYMLINKS`；
- `O_NOFOLLOW`；
- 以授权根目录 FD 为起点；
- 目录列表、上传创建、下载打开、归档文件读取使用安全打开。

测试服务器内核不支持 `openat2`，返回 `ENOSYS`。因此增加兼容旧内核的安全回退：

- 逐级 `openat`；
- 每一级使用目录 FD；
- 每一级启用 `O_NOFOLLOW`；
- 拒绝 `..`；
- 最终文件操作仍相对于验证过的目录 FD。

真实 Linux 测试通过：

- 符号链接逃逸拒绝；
- 安全创建文件；
- 根目录内文件操作；
- 普通用户本人目录列表。

### 尚未完全覆盖

以下操作仍包含路径型递归调用：

- `RemoveAll` 递归删除；
- 跨文件系统复制；
- 移动失败后的复制与删除；
- 归档目录枚举。

建议下一步实现 fd-relative 目录遍历、`unlinkat`、`renameat2` 和基于目录 FD 的递归复制/归档。在完成前，TOCTOU 仍作为 enforce 阻断项。

## 9. 测试结果

### Go

- `go test ./...`：通过
- `go test -race ./internal/httpapi ./internal/service ./internal/integrations/storage`：通过
- Linux amd64 storage 专项测试：通过

### Node

- 22 项测试全部通过
- 0 failed

### API

- user001 dashboard：通过
- team_admin dashboard：通过
- unit_admin dashboard：通过
- cluster_admin dashboard：通过
- user001 授权根访问：403
- unit_admin 跨单位文件访问：403
- team_admin 跨团队文件访问：403
- user001 本人目录：200

### 浏览器

- 真实 LDAP 登录：通过
- 六项一级菜单：通过
- 仪表盘最近作业仅本人：通过
- 控制台错误：0

## 10. 测试服务器部署

部署前备份：

`/data/simpleHPC/backups/20260701-135510-rbac-batch6-hardening`

部署后二进制 SHA256：

`600c43f453d9b8e1b0cc1361d3cc8cafe8d30edb1c1cb6d189f84eab6414022c`

当前状态：

- `simplehpc-backend`: active
- `RBAC_MODE=shadow`
- LDAP：ok
- PostgreSQL：ok
- Redis：ok
- Slurm：ok

## 11. enforce 判断

当前判断：**不满足进入 enforce。**

已解决：

- 仪表盘作业越权；
- storage shadow 比较器误报；
- 新部署增量 shadow mismatch 已为 0。

仍需满足：

1. 24 小时 shadow 滚动窗口自然清零并持续观察。
2. 完成递归删除、复制、移动和归档目录枚举的 fd-relative TOCTOU 加固。
3. 再执行一次完整角色、文件和浏览器回归。
4. 获得用户明确批准后才能进入 enforce。

## 12. 2026-07-02 追加修复：Slurm 读接口 shadow 差异

### 背景

在修复前端 Slurm 页面认证头后，使用 `user001` 直接验证 `/api/v1/slurm/nodes`、`/api/v1/slurm/partitions`、`/api/v1/slurm/qos` 时产生了新的 shadow mismatch：legacy 实际返回 200，但 RBAC 判定不允许普通用户访问这些管理员/配置类接口。

### 根因

legacy/shadow 安全边界只保护了 Slurm 写接口、`partition-configs` 和 `partition-description`，没有保护 Slurm 节点、分区、QOS 的读接口。普通用户可通过直接调用 API 绕过页面菜单限制读取管理员侧调度配置数据。

### 修复

- 将以下接口纳入 legacy/shadow 管理员安全边界：
  - `GET /api/v1/slurm/nodes`
  - `GET /api/v1/slurm/partitions`
  - `GET /api/v1/slurm/qos`
- 保持 `GET /api/v1/slurm/queue-status` 对普通用户开放。
- 保持 `GET /api/v1/slurm/jobs` 走数据范围过滤，不做管理员-only。

### 修改文件

- `backend/internal/httpapi/rbac.go`
- `backend/internal/httpapi/permission_registry_test.go`

### 验证结果

- `go test ./...`：通过
- `node --test tests/*.test.js`：26/26 通过
- 测试服务器 `simplehpc-backend`：active
- 测试服务器 `RBAC_MODE=shadow`
- 普通用户 `user001`：
  - `/api/v1/slurm/nodes`：403
  - `/api/v1/slurm/partitions`：403
  - `/api/v1/slurm/qos`：403
  - `/api/v1/slurm/queue-status`：200
  - `/api/v1/dashboard?range=24h` 最近作业只包含 `user001`
- 修复验证窗口 shadow 统计：5 total / 5 match / 0 mismatch

### 部署信息

- 备份目录：`/data/simpleHPC/backups/20260702-111905-slurm-rbac-boundary`
- 后端二进制 SHA256：`de5f348465a3b0dff88f41be0b16da7a3f726a0aca962eddd4cc7d66d056247b`

### enforce 判断更新

当前仍不建议进入 enforce：

1. 24 小时 shadow 窗口仍包含历史 mismatch，需要自然滚动清零或引入基线时间统计口径后继续观察。
2. 文件服务递归删除、复制、移动和归档目录枚举的 fd-relative TOCTOU 加固仍未完全落地。
3. 进入 enforce 前仍需再次执行完整角色、文件、浏览器回归，并由用户确认。

## 13. 2026-07-02 追加修复：shadow 基线窗口与文件 TOCTOU 覆盖

### shadow stats 基线能力

`GET /api/v1/rbac/shadow/stats` 已支持以下观察窗口参数：

- `since`
- `from`
- `baselineTime`
- `hours`

时间格式支持：

- RFC3339，例如 `2026-07-02T11:42:07+08:00`
- 测试机 `date -Iseconds` 输出的紧凑时区格式，例如 `2026-07-02T11:42:07+0800`
- Unix 秒

返回内容包含：

- `total`
- `matched`
- `mismatched`
- `resolverErrors`
- `matchRate`
- `byModule`
- `byPermission`
- `differences`

### shadow 对比修正

修复了 storage 下游文件策略 403 导致的 shadow 假 mismatch：

- 当 RBAC 允许文件 API、但文件路径策略拒绝时，统计为 API 权限匹配，拒绝由文件边界负责；
- 当 RBAC 本身不允许文件 API，且下游也返回 403 时，统计为 legacy/RBAC 同为拒绝，不再误记为 mismatch。

### 文件 TOCTOU 加固覆盖

文件服务已补充安全遍历和拒绝符号链接策略，覆盖：

- 递归删除；
- 跨文件系统复制；
- 移动失败后的复制删除 fallback；
- 归档目录枚举；
- 复制/移动源路径和目标路径校验；
- 归档目录中的符号链接逃逸。

当前实现以 `secureOpenWithinRoot`、路径规范化、逐级目录读取、符号链接拒绝为核心。Linux 路径打开已使用 fd-relative/openat 思路；递归删除、复制、移动、归档枚举已避免直接信任普通 `Walk` 结果。

### 修改文件

- `backend/internal/httpapi/rbac_admin.go`
- `backend/internal/httpapi/rbac.go`
- `backend/internal/httpapi/permission_registry.go`
- `backend/internal/httpapi/permission_registry_test.go`
- `backend/internal/service/audit.go`
- `backend/internal/service/rbac_shadow_test.go`
- `backend/internal/integrations/storage/client.go`
- `backend/internal/integrations/storage/client_test.go`
- `backend/internal/integrations/storage/secure_open_linux.go`
- `backend/internal/integrations/storage/secure_open_other.go`
- `backend/internal/integrations/storage/secure_open_test.go`

### 本地测试结果

- `cd backend && go test ./internal/httpapi ./internal/service ./internal/integrations/storage`：通过
- `cd backend && go test ./...`：通过
- `cd backend && go test -race ./internal/httpapi ./internal/service ./internal/integrations/storage`：通过
- `node --test tests/*.test.js`：26/26 通过

### 测试服务器部署

- 备份目录：
  - `/data/simpleHPC/backups/20260702-113546-shadow-baseline-toctou-2`
  - `/data/simpleHPC/backups/20260702-114151-shadow-baseline-parse-fix`
- 后端运行路径：`/data/simpleHPC/backend/simplehpc-backend`
- 后端二进制 SHA256：`a355406c50f4e692c00abc726e5936c6a6a977ddb3ca22ac1720ca6c109cf541`
- `RBAC_MODE=shadow`
- 健康检查：LDAP、PostgreSQL、Redis、Slurm 均为 `ok`

### 测试服务器文件安全专项

`/data/simpleHPC/storage.test` 专项测试通过：

- `TestArchiveRejectsNestedSymlinkEscape`
- `TestDeleteRejectsSymlinkInsideTree`
- `TestCopyAndMoveRejectSymlinkInsideTree`
- `TestCopyAndMoveRejectSymlinkDestinationEscape`
- `TestSecureOpenWithinRootRejectsSymbolicLink`
- `TestSecureOpenWithinRootCreatesRegularFile`

### 修复后 shadow 观察窗口

基线时间：`2026-07-02T11:42:07+0800`

最终统计：

- total：40
- matched：40
- mismatched：0
- resolverErrors：0
- matchRate：1.0
- differences：null

覆盖流量：

- 普通用户 `user001`
- `cluster_admin`
- `config_admin`
- `unit_admin`
- `team_admin`
- 多角色用户 `batch6_multi`
- 浏览器登录后的普通用户页面和接口请求

### 角色与接口验证结果

- `user001` 仪表盘最近作业：5 条，用户集合仅 `user001`，`foreign_count=0`
- `user001` 普通用户菜单：仅显示 6 个一级菜单
- `user001` `/api/v1/slurm/nodes`：403
- `user001` `/api/v1/slurm/partitions`：403
- `user001` `/api/v1/slurm/qos`：403
- `user001` `/api/v1/slurm/queue-status`：200
- `user001` `/api/v1/storage/list?path=/data/home/user001`：200，`canGoParent=false`
- `user001` `/api/v1/storage/list?path=/data/home/user002`：403
- `cluster_admin` Slurm 管理读接口：200
- `cluster_admin` `/api/v1/storage/list?path=/data/home`：200
- `config_admin` Slurm 管理读接口：200
- `config_admin` `/api/v1/storage/list?path=/data/home`：403

### 浏览器验收

使用 Playwright + 本机 Chrome 无头模式完成普通用户验收：

- 登录页正常；
- LDAP 用户 `user001` 可登录；
- 页面跳转到 `/index.html`；
- 菜单显示：仪表盘、队列状态、数据目录、作业模板、作业列表、VNC 桌面；
- 未出现：账户管理、用户管理、角色管理、资源队列配置、节点状态、QOS 策略、巡检报告、日志中心；
- 仪表盘最近作业用户集合仅 `user001`；
- storage roots 返回 `/data/home` 的 `effectivePath=/data/home/user001`；
- 个人目录列表返回 `canGoParent=false`；
- 访问其他用户目录返回 403；
- 普通用户直接调用 Slurm 管理读接口返回 403。

### enforce 判断更新

当前仍保持 **不进入 enforce**。

本轮已经满足：

- 可按基线时间过滤 shadow 观察窗口；
- 修复后观察窗口 mismatch 为 0；
- 文件服务递归删除、复制、移动 fallback、归档枚举已补充 TOCTOU 加固；
- API、文件、浏览器回归均通过。

进入 enforce 前仍建议保留：

1. 继续观察一段真实业务流量，避免单次窗口覆盖不足；
2. 对目录并发替换做更长时间压力测试；
3. 由管理员确认测试账号、临时多角色账号是否保留；
4. 由用户再次明确批准后再切换 `RBAC_MODE=enforce`。
