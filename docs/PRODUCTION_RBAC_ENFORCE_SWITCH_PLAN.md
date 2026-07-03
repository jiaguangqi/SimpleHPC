# 生产 RBAC enforce 切换方案

## 1. 当前状态

测试服务器 `10.10.38.152` 已完成 `RBAC_MODE=enforce` 切换验证，关键回归通过。

生产环境当前不允许自动切换。生产切换必须单独评审、确认窗口、完成备份，并在批准后执行。

## 2. 切换目标

将生产环境 simpleHPC 后端从当前 RBAC 运行模式切换到：

```text
RBAC_MODE=enforce
```

切换后，菜单、路由、按钮、接口、数据范围和文件路径边界均由新 RBAC 权限体系强制生效。

## 3. 切换窗口

| 项目 | 建议 |
|---|---|
| 切换窗口 | 低业务流量时间段 |
| 预计操作时间 | 10-20 分钟 |
| 观察时间 | 切换后至少 30-60 分钟重点观察 |
| 生产执行 | 需要单独确认 |

待确认：

- 生产切换日期：
- 生产切换开始时间：
- 生产切换结束时间：
- 业务通知范围：

## 4. 责任人与确认人

| 角色 | 人员 | 职责 |
|---|---|---|
| 执行人 | 待确认 | 执行备份、切换、验证、回退 |
| 技术确认人 | 待确认 | 确认服务、权限、核心业务验证结果 |
| 业务确认人 | 待确认 | 确认用户登录、作业、文件、VNC 等业务路径 |
| 回退确认人 | 待确认 | 出现异常时批准回退 shadow 或 legacy |

## 5. 生产切换前置条件

切换前必须满足：

1. 测试服务器 `RBAC_MODE=enforce` 持续稳定；
2. 测试服务器无新增误拒绝反馈；
3. 生产环境已完成代码、配置、数据库和二进制备份；
4. 生产后端二进制 SHA256 已记录；
5. 生产 `RBAC_MODE` 当前值已记录；
6. 生产数据库迁移状态已确认；
7. 生产回退 shadow、legacy、旧二进制路径已确认；
8. 生产验证账号已准备；
9. 切换窗口和责任人已确认；
10. 获得明确生产切换批准。

## 6. 生产备份方案

以下命令为模板，需要在生产服务器上按实际容器、数据库和路径确认后执行。

```bash
TS=$(date +%Y%m%d-%H%M%S)
BACKUP_DIR=/data/simpleHPC/backups/prod-rbac-enforce-$TS
mkdir -p "$BACKUP_DIR"

cp -a /etc/simplehpc-backend.env "$BACKUP_DIR/"
cp -a /data/simpleHPC/backend/simplehpc-backend "$BACKUP_DIR/"

sha256sum /data/simpleHPC/backend/simplehpc-backend > "$BACKUP_DIR/simplehpc-backend.sha256"
grep '^RBAC_MODE=' /etc/simplehpc-backend.env > "$BACKUP_DIR/rbac-mode.before.txt" || true
systemctl status simplehpc-backend --no-pager > "$BACKUP_DIR/simplehpc-backend.status.before.txt" || true
```

数据库备份模板：

```bash
TS=$(date +%Y%m%d-%H%M%S)
BACKUP_DIR=/data/simpleHPC/backups/prod-rbac-enforce-$TS

# 根据生产 PostgreSQL 部署方式选择实际命令。
# 示例：Docker 容器方式
docker exec simplehpc-postgres pg_dump -U simplehpc -d simplehpc -Fc \
  > "$BACKUP_DIR/simplehpc-$TS.dump"

# 示例：本机 psql/pg_dump 方式
# PGPASSWORD='<password>' pg_dump -h 127.0.0.1 -U simplehpc -d simplehpc -Fc \
#   > "$BACKUP_DIR/simplehpc-$TS.dump"
```

备份完成后必须确认：

```bash
ls -lh "$BACKUP_DIR"
test -s "$BACKUP_DIR/simplehpc-backend"
test -s "$BACKUP_DIR/simplehpc-backend.sha256"
test -s "$BACKUP_DIR/simplehpc-$TS.dump"
```

## 7. 生产 env 变更内容

仅变更：

```text
RBAC_MODE=<当前值>
```

为：

```text
RBAC_MODE=enforce
```

不在本次切换中扩大修改范围。

## 8. 生产切换命令

```bash
cp -a /etc/simplehpc-backend.env /etc/simplehpc-backend.env.bak-$(date +%Y%m%d-%H%M%S)
sed -i 's/^RBAC_MODE=.*/RBAC_MODE=enforce/' /etc/simplehpc-backend.env
systemctl restart simplehpc-backend
systemctl is-active simplehpc-backend
grep '^RBAC_MODE=' /etc/simplehpc-backend.env
```

健康检查：

```bash
curl -fsS http://127.0.0.1:8080/api/health
journalctl -u simplehpc-backend --since "10 min ago" --no-pager | tail -n 200
```

## 9. 生产切换后验证清单

### 9.1 基础健康

| 验证项 | 预期 |
|---|---|
| `systemctl is-active simplehpc-backend` | `active` |
| `grep '^RBAC_MODE=' /etc/simplehpc-backend.env` | `RBAC_MODE=enforce` |
| `/api/health` | `status=ok` |
| PostgreSQL | ok |
| Redis | ok |
| LDAP | ok |
| Slurm | ok |

### 9.2 普通用户验证

普通用户登录后只显示 6 个一级菜单：

- 仪表盘；
- 队列状态；
- 数据目录；
- 作业模板；
- 作业列表；
- VNC 桌面。

普通用户不得显示：

- 账户管理；
- 资源管理；
- 数据管理；
- 运维管理；
- 日志管理；
- 系统配置；
- 单位管理；
- 团队管理；
- 用户管理；
- 角色管理；
- 节点状态；
- QOS 策略；
- 访问授权。

### 9.3 普通用户接口越权验证

| 接口 | 预期 |
|---|---|
| `/api/v1/rbac/roles` | 403 |
| `/api/v1/slurm/nodes` | 403 |
| `/api/v1/slurm/partitions` | 403 |
| `/api/v1/slurm/qos` | 403 |
| `/api/v1/slurm/queue-status` | 200 |

### 9.4 文件目录边界验证

以 `user001` 为例：

| 路径/操作 | 预期 |
|---|---|
| `/data/home` | 实际映射到 `/data/home/user001` |
| `/data/home/user001` | 200 |
| `/data/home/user001` | `canGoParent=false` |
| `/data/home/user002` | 403 |
| `/data/home/user001/../user002` | 403 |
| 上一级按钮 | 不能突破用户初始目录 |
| 上传/下载/删除/复制/移动/打包下载 | 不能越权 |

### 9.5 作业与 VNC 验证

| 验证项 | 预期 |
|---|---|
| 普通用户作业列表 | 只显示本人作业 |
| 仪表盘最近作业 | 不出现其他用户作业摘要 |
| 作业详情 | 不能访问其他用户作业 |
| 作业模板 | 正常访问 |
| 被授权模板 | 正常展示 |
| VNC 模板提交 | 正常 |
| VNC 测试作业清理 | 无残留 |

### 9.6 中间管理员与集群管理员验证

| 角色 | 预期 |
|---|---|
| `config_admin` | 配置类数据正常；角色页面只读；默认不能读取用户文件 |
| `unit_admin` | 只能访问本单位数据；角色页面只读 |
| `team_admin` | 只能访问本团队数据；角色页面只读 |
| 多角色用户 | 权限按启用角色取并集，数据范围按最大范围 |
| `cluster_admin` | 全局权限正常；角色创建、编辑、删除、分配权限正常 |

## 10. 生产回退到 shadow

当 enforce 后出现误拒绝、核心路径异常或用户反馈无法快速修复时，优先回退到 shadow。

```bash
sed -i 's/^RBAC_MODE=.*/RBAC_MODE=shadow/' /etc/simplehpc-backend.env
systemctl restart simplehpc-backend
systemctl is-active simplehpc-backend
grep '^RBAC_MODE=' /etc/simplehpc-backend.env
curl -fsS http://127.0.0.1:8080/api/health
```

回退后需保留日志，并记录：

- 请求路径；
- 请求用户；
- 用户角色；
- 原始响应码；
- 触发时间；
- 是否为误拒绝；
- 是否需要补权限或修数据范围。

## 11. 生产回退到 legacy

如 shadow 模式仍异常，可继续回退 legacy。

```bash
sed -i 's/^RBAC_MODE=.*/RBAC_MODE=legacy/' /etc/simplehpc-backend.env
systemctl restart simplehpc-backend
systemctl is-active simplehpc-backend
grep '^RBAC_MODE=' /etc/simplehpc-backend.env
curl -fsS http://127.0.0.1:8080/api/health
```

## 12. 旧二进制回滚方案

如应用版本本身异常，可恢复备份二进制。

```bash
BACKUP_DIR=/data/simpleHPC/backups/prod-rbac-enforce-<切换时间>

systemctl stop simplehpc-backend
install -o root -g root -m 0755 \
  "$BACKUP_DIR/simplehpc-backend" \
  /data/simpleHPC/backend/simplehpc-backend
systemctl start simplehpc-backend
systemctl is-active simplehpc-backend
sha256sum /data/simpleHPC/backend/simplehpc-backend
curl -fsS http://127.0.0.1:8080/api/health
```

## 13. 用户影响说明

生产切换到 enforce 后，历史 legacy 模式下被宽松放行的未授权访问会被正式拒绝。

预期影响：

1. 普通用户不能访问管理员页面和管理员接口；
2. 普通用户只能访问本人作业和本人文件目录；
3. 中间管理员的角色管理页面只读，不能创建、编辑、删除角色；
4. Slurm 管理类接口对普通用户返回 403；
5. 文件管理器严格限制在授权目录边界内。

如果用户反馈误拒绝，需要收集请求路径、用户、角色、时间和响应码。无法快速修复时，按本方案回退 shadow。

## 14. 是否建议生产切换

当前建议：

- 可以进入生产切换评审；
- 不建议未经评审直接执行生产切换；
- 生产切换前必须确认切换窗口、备份、责任人和回退路径；
- 获得明确批准后再执行 `RBAC_MODE=enforce` 生产切换。

