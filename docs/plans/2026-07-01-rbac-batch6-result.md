# RBAC 批次六：测试服务器部署与真实集群验证结果

## 1. 结论

- 部署目标仅为测试服务器 `10.10.38.152`，项目目录 `/data/simpleHPC`。
- 当前服务保持 `RBAC_MODE=shadow`，未进入 `enforce`。
- 服务、LDAP、PostgreSQL、Redis、Slurm 健康检查均通过。
- 数据库迁移已完成隔离库 `up/down/up` 演练，实际测试库已应用迁移 002-008。
- 普通用户菜单、管理员路由守卫、作业列表和文件目录边界通过验证。
- 真实流量 shadow 存在 8 条差异，且发现普通用户仪表盘最近作业泄露其他用户摘要。
- TOCTOU 文件操作加固尚未完成。

因此：**当前不满足进入 enforce 的前置条件。**

## 2. 部署前备份

备份目录：

`/data/simpleHPC/backups/20260701-130430-rbac-batch6-predeploy`

备份内容：

- `simpleHPC-code-config.tgz`
- `simplehpc-db.dump`
- systemd unit、drop-in 和环境配置
- `SHA256SUMS`

部署前旧二进制 SHA256：

`b1d2d8131eac52417493efcefcae7cb19a9e22d50dcc9be7f125fedb30e9fced`

部署后二进制 SHA256：

`527739382fe6ec1b5c215b739f1c633ca498d6d13cece99f507ec5e0ff2acf9e`

## 3. 测试服务器部署步骤

1. 检查主机、磁盘、端口、systemd、PostgreSQL、Redis、LDAP 和 Slurm 状态。
2. 备份项目代码、配置、systemd 配置和 PostgreSQL 数据库。
3. 在本地执行 Node、Go 测试并构建 Linux amd64 静态二进制。
4. 上传并校验 67 个发布文件。
5. 在隔离数据库执行迁移演练。
6. 停止后端服务，原子替换二进制并同步前端资源。
7. 设置 `RBAC_MODE=shadow`，启动服务并执行健康检查。
8. 创建仅用于批次六验证的单位、团队、角色绑定和会话。
9. 使用 API 和真实浏览器验证角色、菜单、作业、文件、模板与 VNC。

发布目录：

`/data/simpleHPC/releases/20260701-130623-rbac-batch6`

## 4. 数据库迁移结果

### 隔离库演练

隔离库：`simplehpc_rbac_batch6_rehearsal`

- 迁移 002-008：up 成功。
- 迁移 008：down 成功，权限级别恢复为 `read/manage`；再次 up 成功。
- 迁移 007：down 后路由权限记录清除；再次 up 后恢复。
- 隔离 Redis DB 14 已清理。
- 演练数据库已删除。

### 实际测试库

已应用迁移：

`2, 3, 4, 5, 6, 7, 8`

种子结果：

- 内置角色：5
- 权限点：415
- `cluster_admin` 有效绑定：2

未对生产数据库执行任何操作。

## 5. 服务启动与健康检查

- `simplehpc-backend`: active
- RBAC 模式：shadow
- LDAP：ok
- PostgreSQL：ok
- Redis：ok
- Slurm：ok
- panic/fatal/segmentation 日志：0
- `roles.html`、`job-list.html`、`job-templates.html`、`vnc-desktop.html`、`data.html`：HTTP 200

## 6. 真实角色验证

| 角色 | 验证结果 |
|---|---|
| cluster_admin | 角色管理接口 200，全局兜底有效 |
| config_admin | LDAP 配置接口 200；读取用户文件 403 |
| unit_admin | 本单位共享目录 200；其他单位目录 403 |
| team_admin | 本团队共享目录 200；其他团队目录 403 |
| 自定义观察员 | shadow 下旧逻辑允许角色列表、RBAC 拒绝，形成预期差异 |
| 普通用户 user001 | 六项菜单、本人作业、本人文件、管理员接口拒绝均完成验证 |

团队和单位共享目录按确认规范验证：

- `/授权根/teams/{teamId}`
- `/授权根/units/{unitId}`

这些目录仍由管理员或初始化任务创建，尚未自动创建。

## 7. 普通用户验收

### 通过项

- 左侧仅显示 6 个一级菜单：仪表盘、队列状态、数据目录、作业模板、作业列表、VNC 桌面。
- 不显示管理分组标题。
- 直接访问 `roles.html` 显示 RBAC 403 页面。
- 作业列表仅返回本人作业，`foreign_users=0`。
- `/data/home/user001` 返回 200。
- `/data/home` 返回 403。
- `/data/home/user002` 返回 403。
- 编码后的 `user001/../user002` 返回 403。
- 初始目录返回：
  - `effectivePath=/data/home/user001`
  - `initialPath=/data/home/user001`
  - `canGoParent=false`
- 文件管理器“上一级”按钮在初始目录置灰。
- 模板和 VNC 运行记录接口可访问。
- RBAC 管理接口返回 403。
- 浏览器控制台无错误。

### 未通过项

普通用户仪表盘“最近提交的作业”仍显示 `user002` 和 `root` 的作业摘要。作业列表接口已正确过滤，但仪表盘聚合接口尚未应用相同的数据范围。

该问题属于数据范围泄露，必须在进入 enforce 前修复并回归。

## 8. 真实 shadow 差异

观测接口：

`GET /api/v1/rbac/shadow/stats?hours=24`

最终统计：

- total：39
- matched：31
- mismatched：8
- resolverErrors：0
- matchRate：79.49%

差异明细：

| 权限点 | 差异数 | 原因 |
|---|---:|---|
| `api.roles.list` | 1 | 自定义观察员在旧 admin 判断中被允许，RBAC 白名单拒绝 |
| `api.storage.files.list` | 7 | API 粗粒度权限允许，但文件路径策略最终返回 403；当前 shadow 中间件把最终路径拒绝统计为权限差异 |

结论：

- 差异未清零，不允许进入 enforce。
- storage 差异需要让 shadow 对比器区分“API 权限判定”和“独立文件路径策略判定”。
- roles 差异需要明确自定义角色在旧逻辑和新 RBAC 下的预期行为。

## 9. 回滚演练

### 模式回滚

1. 将测试服务器临时切换到 `RBAC_MODE=legacy`。
2. 重启服务并验证全部健康检查为 ok。
3. 恢复 `RBAC_MODE=shadow`。
4. 再次重启并验证全部健康检查为 ok。

### 二进制回滚

1. 从部署前备份提取旧二进制。
2. 停止服务并临时替换为旧二进制。
3. 旧二进制启动成功，健康检查全部为 ok。
4. 恢复新二进制并重启。
5. 新二进制 SHA256、服务状态、健康检查和 shadow 模式均恢复正确。

### 数据库回滚

007/008 的 down/up 已在测试服务器隔离库完成，未对实际测试库执行 destructive down。

## 10. TOCTOU 加固评估

当前文件服务在执行操作前使用路径清理和 `EvalSymlinks` 校验，随后调用 `os.Open`、`os.OpenFile`、`os.Rename`、`os.RemoveAll` 等路径型 API。

这能防止常规目录穿越和静态符号链接逃逸，但路径校验与实际文件操作之间仍有理论竞态窗口。

建议在 Linux 测试环境增加：

- 目录文件描述符相对操作；
- `openat2(RESOLVE_BENEATH | RESOLVE_NO_SYMLINKS)`，不支持时回退到逐级 `openat`；
- `O_NOFOLLOW`；
- 复制、移动、删除、归档遍历全过程都保持在已验证目录 FD 下；
- 对目标目录和最终文件分别校验；
- 增加符号链接并发替换专项测试。

该加固未在批次六临时实现，避免扩大服务器改造范围；它仍是 enforce 前置事项。

## 11. 测试与文件清单

部署前完成：

- Go 单元测试
- Go race/文件安全回归
- Node 前端测试
- 浏览器真实登录和页面验收
- 旧鉴权残留 59 处基线防回退测试

批次六没有临时修改业务实现代码。新增交付文件：

- `docs/plans/2026-07-01-rbac-batch6-result.md`

测试服务器新增或变更：

- `/data/simpleHPC/backend/simplehpc-backend`
- `/data/simpleHPC/frontend/`
- `/etc/simplehpc-backend.env`（仅增加/保持 shadow 模式）
- RBAC 002-008 数据库结构与种子数据
- 批次六测试单位、团队、角色绑定和会话
- `/data/share/{units,teams}/...`
- `/data/scratch/{units,teams}/...`

## 12. enforce 前置条件

当前状态：**不满足**。

必须完成：

1. 修复普通用户仪表盘最近作业跨用户泄露。
2. 修正 storage shadow 差异统计语义。
3. 明确并消除自定义观察员的 roles 差异。
4. 真实流量 shadow 差异清零并持续观察。
5. 完成 openat/openat2、O_NOFOLLOW 的 TOCTOU 加固或形成经批准的风险豁免。
6. 重新执行普通用户及四类管理员浏览器/API/文件越权回归。
7. 获得用户再次明确确认后，才可讨论 enforce。

## 13. 风险与待确认事项

1. 普通用户仪表盘存在跨用户作业摘要泄露，优先级高。
2. 当前 storage shadow 统计混合了 API 权限与文件路径策略结果，存在误报。
3. TOCTOU 理论风险尚未加固。
4. LDAP 使用持久化配置时曾出现凭据漂移，当前已修复且重启验证通过，但建议后续增加 LDAP 配置备份和启动巡检。
5. SSH 服务当前未协商后量子密钥交换算法，这是基础设施后续加固项，不影响本批次 RBAC 功能结论。
