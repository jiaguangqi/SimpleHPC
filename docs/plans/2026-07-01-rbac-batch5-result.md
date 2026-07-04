# RBAC 批次五交付记录：旧权限迁移、双读切换与完整测试

## 1. 执行边界

- 继续保持 `legacy/shadow`，未进入 `enforce`。
- 未部署生产环境或测试服务器业务目录。
- 未修改现有业务数据库。
- PostgreSQL 验证全部使用创建后即销毁的隔离数据库。

## 2. 数据库迁移演练

### 007 up/down/up

隔离库：`simplehpc_rbac_batch5_007`（演练后已删除）

| 阶段 | 新增路由权限数 |
|---|---:|
| 初始 | 0 |
| 第一次 up | 12 |
| down | 0 |
| 第二次 up | 12 |

第二次 up 后：

- 对应角色路由授权关系 21 条；
- 有效 `cluster_admin` 绑定 1 条；
- SQL 使用 `ON_ERROR_STOP=1`，全程无错误。

### 008 up/down/up

完整测试发现数据库文件策略使用 `read`，而 Go 内核和前端使用 `view`。新增 008 迁移统一为 `view/manage`，并增加 shadow 审计部分索引。

演练结果：

- up：约束变为 `view/manage`，索引存在；
- down：约束恢复 `read/manage`，索引删除；
- 再 up：约束再次恢复 `view/manage`；
- 隔离库已删除。

## 3. legacy/shadow 双读审计

改造后 shadow 使用实际 legacy 响应状态作为旧判定：

- HTTP 401/403：legacy 拒绝；
- 其他状态：legacy 已通过授权层；
- 同时记录 RBAC 判定、权限点、路由、响应状态和原因；
- 不记录请求体、密码、令牌或文件内容。

新增：

- `GET /api/v1/rbac/shadow/stats?hours=24`
- 总请求数、匹配数、差异数、匹配率；
- resolver 错误数；
- 按模块统计；
- 按权限点统计。

数据范围双读覆盖：

- 用户列表；
- 团队列表；
- 单位列表；
- 作业模板；
- 作业列表与作业详情。

受控授权模块语料覆盖 17 个模块，结果 `17/17 match`、差异 `0`。差异检测样本同时验证可正确统计 permission mismatch、scope mismatch 和 resolver error。

说明：尚未部署真实业务 shadow 流量，因此这里是受控测试统计，不冒充真实集群线上统计。真实流量差异清零仍是批次六后进入 enforce 的硬前置条件。

## 4. 测试账号与范围

隔离库建立后销毁：

| 账号 | 角色/范围 | 目的 |
|---|---|---|
| `batch5_cluster` | cluster_admin/global | 最高权限兜底 |
| `batch5_config` | config_admin/global | 配置权限且无用户文件策略 |
| `batch5_unit_a` | unit_admin/unit-a | 单位边界 |
| `batch5_team_a` | team_admin/team-a | 团队边界 |
| `batch5_user_a` | user/self | 普通用户 |
| `batch5_observer` | 自定义只读角色 | 自定义角色 |
| `batch5_user_a` + reviewer | 多角色 | 权限并集、范围最大值 |
| `batch5_disabled_user` | disabled role | 禁用角色不生效 |

隔离库验证：

- 测试绑定 7 条；
- 有效绑定角色 6 类；
- config_admin 文件策略 0 条；
- unit_admin 明确 unit_shared 策略 2 条；
- team_admin 明确 team_shared 策略 2 条。

## 5. 鉴权、数据与文件验证

- 未登录访问受保护 API：legacy/shadow/enforce 三种模式均返回 401。
- 普通用户不继承角色管理 API。
- unit_admin 仅匹配本单位资源。
- team_admin 仅匹配本团队资源。
- config_admin 无显式文件策略时解析结果为空。
- 自定义角色权限正常参与合并。
- 多角色权限取并集，数据范围取最大。
- 多角色中的 global 数据范围不会自动产生 global 文件策略。
- 禁用角色权限完全忽略。
- cluster_admin 通配权限与 global 范围兜底正常。
- 权限版本随角色版本或绑定更新时间变化。
- 文件边界继续覆盖授权根、其他用户、`../`、双斜线、`.`、URL 编码、软链接和归档逃逸。

## 6. 角色保存全部配置评估

本批次不增加聚合事务接口，原因：

1. 现有基础信息、权限、数据范围、文件策略、用户绑定接口各自已使用单语句原子操作或数据库事务；
2. 聚合接口需要将现有服务方法重构为共享 `sql.Tx`，并将 Redis 主动失效延后到总事务提交之后；
3. 在切换 enforce 前仓促并行保留两套写路径，会增加缓存失效和回滚复杂度。

当前风险控制：

- 前端任何子步骤失败都会明确提示，不显示虚假成功；
- 后端每个子接口自身保持事务一致性；
- 重试保存会以 replace 语义收敛到目标配置；
- 后续计划在 enforce 稳定后增加带版本号/乐观锁的聚合事务接口。

## 7. 旧鉴权残留

- 扫描基线 59 处；
- 全部位于 12 个已评审文件；
- 由 Node 测试固定文件和出现次数，新增旧式判断会使测试失败；
- 明细见 `2026-07-01-rbac-legacy-residuals.md`。

## 8. 回滚验证

- 007、008 均通过 up/down/up；
- `RBAC_MODE` 非法值和空值始终回退 `legacy`；
- legacy/shadow/enforce 受保护 API 均保持登录边界；
- 应用回滚只需恢复 `RBAC_MODE=legacy`，无需删除 RBAC 数据；
- 旧 `user.Type`、`requireAdmin` 和 legacy 判断仍保留作为对照。

## 9. enforce 前置条件

以下条件全部满足后才允许进入 enforce：

1. 007、008 在测试服务器备份后的测试库再次验证；
2. 测试服务器以 shadow 模式运行真实业务流量；
3. 所有模块 permission mismatch 和 scope mismatch 清零；
4. resolver error 为 0；
5. 五类内置角色、自定义角色、多角色和禁用角色真实账号验收通过；
6. 角色/权限/绑定变更后缓存主动失效通过；
7. config_admin 用户文件访问为 403；
8. unit/team 跨组织访问为 403；
9. 文件越权专项测试全部为 403，合法路径正常；
10. 普通用户六菜单、路由守卫和按钮权限浏览器验收通过；
11. Go、race、Node、浏览器测试全部通过；
12. TOCTOU `openat/O_NOFOLLOW` 加固完成或形成正式风险接受记录；
13. shadow 审计保留周期、告警阈值和回滚负责人明确。

## 10. 风险

1. 当前差异统计来自受控测试，不是测试服务器真实流量。
2. 聚合事务接口暂缓，前端保存存在“前序步骤成功、后序步骤失败”的可恢复中间状态。
3. TOCTOU 加固仍是 enforce 前安全事项。
4. 59 处旧式判断仅用于 shadow/legacy；进入 enforce 后必须按模块逐步清理。
