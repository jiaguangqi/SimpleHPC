# RBAC 测试服务器 enforce 验收报告

## 1. 验收结论

测试服务器 `10.10.38.152` 已完成 `RBAC_MODE=enforce` 切换验证，关键回归通过。

当前结论：

- 测试服务器继续保持 `RBAC_MODE=enforce` 运行观察。
- 当前不切换生产环境。
- 生产环境切换必须单独提交切换方案，并经确认后执行。

## 2. enforce 切换信息

| 项目 | 结果 |
|---|---|
| 测试服务器 | `10.10.38.152` |
| 切换时间 | `2026-07-03 20:36:46 +08:00` |
| 当前运行模式 | `RBAC_MODE=enforce` |
| 最近只读复核时间 | `2026-07-03 23:27:44 +08:00` |
| 服务状态 | `active` |
| 健康检查 | `ok` |
| PostgreSQL | `ok` |
| Redis | `ok` |
| LDAP | `ok` |
| Slurm | `ok` |
| 近 30 分钟错误/403 类日志 | `0` |

## 3. 后端版本与备份

### 后端 SHA256

```text
109ee49ab4e572bc5ccf9113ebc7a4b6a3508083386a6adfed73acb3625f498b  /data/simpleHPC/backend/simplehpc-backend
```

### env 备份

```text
/etc/simplehpc-backend.env.bak-20260703-203646
```

### 二进制与迁移备份

```text
/data/simpleHPC/backups/rbac-mismatch-fix-20260703-195359
```

## 4. enforce 切换前置结果

切换前已完成并确认：

1. 24 小时真实业务 shadow 观察窗口曾达到 `1837 total / 1837 matched / 0 mismatched`。
2. 最后一轮主动回归发现 5 条 mismatch 后，已完成修复：
   - 作业详情 shadow 统计口径修正；
   - 中间管理员角色体系只读权限补齐；
   - 角色变更接口增加 `cluster_admin` 硬边界。
3. 修复后短窗口 shadow 验证达到：

```text
54 total / 54 matched / 0 mismatched
matchRate=1
differences=null
```

## 5. enforce 后关键回归清单

| 验证项 | 预期 | 结果 |
|---|---|---|
| 服务健康检查 | `/api/health` 返回 ok | 通过 |
| `/api/v1/auth/me` | 正常返回当前用户与权限 | 通过 |
| 普通用户菜单 | 只显示 6 个一级菜单 | 通过 |
| 普通用户访问角色管理接口 | 403 | 通过 |
| 普通用户访问 `/api/v1/slurm/nodes` | 403 | 通过 |
| 普通用户访问 `/api/v1/slurm/partitions` | 403 | 通过 |
| 普通用户访问 `/api/v1/slurm/qos` | 403 | 通过 |
| 普通用户访问 `/api/v1/slurm/queue-status` | 200 | 通过 |
| `/data/home/user001` | 200 | 通过 |
| `/data/home/user001` 上一级 | `canGoParent=false` | 通过 |
| `/data/home/user002` | 403 | 通过 |
| `/data/home/user001/../user002` | 403 | 通过 |
| 作业列表 | 普通用户只看到本人作业 | 通过 |
| 仪表盘 | 无其他用户作业摘要 | 通过 |
| 作业模板 | 正常访问 | 通过 |
| VNC 模板提交 | 正常提交并可清理测试作业 | 通过 |
| 中间管理员角色页面 | 只读正常 | 通过 |
| 中间管理员角色创建接口 | 403 | 通过 |
| `cluster_admin` 角色创建/删除 | 正常 | 通过 |

最终回归结果：

```text
failures=0
```

## 6. VNC 回归清理

VNC 回归测试作业已提交验证并清理。

```text
测试作业：1754
状态：已取消
残留作业：无
```

## 7. 回滚方案

### 7.1 enforce 回退到 shadow

```bash
sed -i 's/^RBAC_MODE=.*/RBAC_MODE=shadow/' /etc/simplehpc-backend.env
systemctl restart simplehpc-backend
systemctl is-active simplehpc-backend
grep '^RBAC_MODE=' /etc/simplehpc-backend.env
```

### 7.2 shadow 回退到 legacy

```bash
sed -i 's/^RBAC_MODE=.*/RBAC_MODE=legacy/' /etc/simplehpc-backend.env
systemctl restart simplehpc-backend
systemctl is-active simplehpc-backend
grep '^RBAC_MODE=' /etc/simplehpc-backend.env
```

### 7.3 旧二进制回滚

```bash
systemctl stop simplehpc-backend
install -o root -g root -m 0755 \
  /data/simpleHPC/backups/rbac-mismatch-fix-20260703-195359/simplehpc-backend \
  /data/simpleHPC/backend/simplehpc-backend
systemctl start simplehpc-backend
systemctl is-active simplehpc-backend
sha256sum /data/simpleHPC/backend/simplehpc-backend
```

说明：

- 当前数据库无需执行 down 迁移即可完成应用级回滚。
- env、二进制和迁移备份均已保留。

## 8. 当前风险

| 风险 | 说明 | 处理建议 |
|---|---|---|
| 误拒绝风险 | enforce 会真正拒绝未授权请求，个别历史宽松路径可能暴露为 403 | 继续观察后端错误日志、403 比例和用户反馈 |
| 生产差异风险 | 测试服务器已验证，生产数据量、用户习惯和流量路径可能不同 | 生产切换前必须单独评审和备份 |
| 历史迁移兼容风险 | 隔离新库 up/down/up 曾发现旧迁移对 `roles.permission_summary` 有历史依赖 | 当前测试库不受影响；后续应补齐 fresh install 迁移兼容 |
| 运维响应风险 | enforce 后如果出现误拒绝，需要快速回退 shadow | 保留并演练回退命令，明确值班责任人 |

## 9. 后续观察项

测试服务器保持 `RBAC_MODE=enforce` 继续观察以下内容：

1. 后端错误日志；
2. 403 请求比例；
3. 普通用户登录与菜单展示；
4. 作业提交、查看、取消；
5. 作业模板和 VNC；
6. 文件管理器上传、下载、删除、复制、移动；
7. 中间管理员只读角色页面；
8. `cluster_admin` 角色管理能力；
9. 是否出现用户反馈的误拒绝问题。

## 10. 是否建议进入生产切换评审

建议进入生产切换评审，但不建议直接切换生产。

进入生产切换评审的前提：

1. 测试服务器 enforce 继续稳定运行；
2. 无新增误拒绝反馈；
3. 生产切换窗口、备份方案、回退路径和责任人确认；
4. 生产切换方案经确认后执行。

