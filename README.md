# SimpleHPC

SimpleHPC 是一套面向 HPC / 智算集群的轻量级集群管理、作业调度、数据目录和运维管理平台。当前版本已经在测试服务器完成 RBAC enforce 模式验证，覆盖 Slurm、LDAP、PostgreSQL、Redis、文件目录边界、VNC 桌面作业和巡检报告等核心能力。

> 当前仓库是清理后的源码仓库，不包含服务器运行数据、数据库 dump、备份目录、上传文件、二进制构建产物和生产密钥。

## 主要能力

### 1. 集群仪表盘

- 集群资源概览；
- 在线用户、作业、队列、存储等关键指标展示；
- CPU / GPU 资源使用趋势；
- 资源池作业趋势，支持按 Slurm partition 和时间范围查看运行 / 排队作业趋势；
- 普通用户视角下只展示本人可见的数据摘要。

### 2. Slurm 作业管理

- 作业列表、状态、详情和输出查看；
- 作业模板提交；
- Shell 非交互式作业模板；
- noVNC Linux 桌面作业模板；
- 作业 stdout / stderr 在线查看与刷新；
- Slurm 队列状态、节点状态、分区配置、QOS 策略展示与管理；
- 普通用户只能查看本人作业，管理员按 RBAC 数据范围查看全局 / 单位 / 团队作业。

### 3. noVNC 桌面作业

- 通过 Slurm 在计算节点启动 VNC 桌面；
- 支持设置桌面运行时间、分辨率和桌面环境；
- 平台中查看 VNC 桌面任务并在线访问；
- VNC 作业要求提交账号与 Linux / LDAP 用户映射一致；
- 支持 VNC 作业访问网关和令牌化访问。

### 4. 账户、单位、团队管理

- 单位管理；
- 团队 / 用户组管理；
- 用户管理；
- 管理员账号管理；
- 新建用户组向导支持创建团队与组长首用户；
- 用户创建时联动 LDAP、Linux 用户、家目录、默认环境和 SSH 互信初始化；
- 支持团队默认资源策略和存储目录授权。

### 5. RBAC 权限体系

当前版本支持内置角色与自定义角色共存：

- `cluster_admin`：集群管理员，全局最高权限；
- `config_admin`：配置管理员，负责平台和运维配置；
- `unit_admin`：单位管理员，管理本单位范围数据；
- `team_admin`：团队管理员，管理本团队范围数据；
- `user`：普通用户，仅访问本人数据。

RBAC 覆盖：

- 菜单权限；
- 路由权限；
- 按钮 / 操作权限；
- API 权限；
- 数据范围权限；
- 文件目录访问策略。

多角色用户按启用角色权限取并集，数据范围按以下优先级合并：

```text
global > unit > team > self/granted > none
```

文件目录权限是独立安全边界，不会因为菜单权限或普通数据范围扩大而自动突破。

### 6. 文件管理器

- 支持多个授权存储入口；
- 普通用户授权入口自动映射到 `/授权目录/{username}`；
- 后端统一校验所有文件操作路径；
- 支持上传、下载、删除、复制、移动、重命名、打包下载、显示隐藏文件；
- 防止 `../`、双斜线、URL 编码、绝对路径、软链接逃逸等越权方式；
- 上一级按钮由后端返回 `effectivePath`、`initialPath`、`canGoParent`、`parentPath` 控制。

### 7. 运维与巡检

- 一键巡检；
- 输出 HTML 巡检总结报告；
- 输出详细巡检日志；
- 报告和日志可在线预览、下载；
- 支持将巡检报告转换为飞书富文本通知；
- 监控告警入口；
- 系统日志、审计日志、用户登录日志。

### 8. 平台设置

- 平台名称；
- 主页面 Logo 上传；
- 登录页背景图上传；
- 登录前公共配置接口；
- LDAP 配置；
- Slurm 配置；
- 存储目录配置；
- 通知配置。

## 技术栈

### 前端

- 原生 HTML / CSS / JavaScript；
- 轻量级多页面管理后台；
- Apple-style 浅色 Web UI；
- 动态菜单与按钮权限来自 `/api/v1/auth/me`；
- 登录页、仪表盘、角色管理、文件管理、作业中心等独立页面。

### 后端

- Go；
- Gin HTTP API；
- PostgreSQL；
- Redis；
- OpenLDAP；
- Slurm CLI / SlurmDB；
- Linux 文件系统访问控制；
- noVNC / websockify / VNC server 集成。

## 目录结构

```text
.
├── *.html                         # 前端页面
├── assets/                        # 图片、图标、Logo
├── css/                           # 全局样式与视觉效果
├── js/                            # 前端交互逻辑
├── tests/                         # Node.js 前端轻量测试
├── docs/                          # 部署、设计、验收与切换文档
└── backend/
    ├── cmd/server/                # 后端入口
    ├── internal/config/           # 配置加载
    ├── internal/httpapi/          # HTTP 路由与接口层
    ├── internal/integrations/     # LDAP / Slurm / Storage 集成
    ├── internal/service/          # 业务服务与 RBAC 核心逻辑
    ├── migrations/                # PostgreSQL 迁移脚本
    └── scripts/                   # 后端辅助脚本
```

## 快速启动

### 1. 准备依赖

需要可用的：

- Go 1.22+；
- PostgreSQL；
- Redis；
- OpenLDAP；
- Slurm 命令行工具；
- 可选：VNC server、websockify、noVNC。

### 2. 配置环境变量

```bash
cd backend
cp .env.example .env
```

编辑 `.env`，至少配置：

```bash
SIMPLEHPC_ADDR=:8080
SIMPLEHPC_PUBLIC_URL=http://127.0.0.1:8080
SIMPLEHPC_FRONTEND_DIR=..

DATABASE_URL=postgres://simplehpc:CHANGE_ME@127.0.0.1:5432/simplehpc?sslmode=disable
REDIS_URL=redis://:CHANGE_ME@127.0.0.1:6379/0

LDAP_URL=ldap://127.0.0.1:389
LDAP_BASE_DN=dc=simplehpc,dc=local
LDAP_ADMIN_DN=cn=admin,dc=simplehpc,dc=local
LDAP_ADMIN_PASSWORD=CHANGE_ME

SLURM_BIN_DIR=/opt/slurm/current/bin
SLURM_CONFIG_PATH=/etc/slurm/slurm.conf
SLURM_DEFAULT_ACCOUNT=simplehpc
SLURM_DEFAULT_PARTITION=debug

STORAGE_ROOTS=/data/home,/data/share,/data/recycle,/data/scratch
RBAC_MODE=shadow
```

RBAC 模式说明：

| 模式 | 用途 |
|---|---|
| `legacy` | 旧权限模式 |
| `shadow` | 新旧权限双读对比，不强制拒绝 |
| `enforce` | 新 RBAC 强制生效 |

生产环境切换到 `enforce` 前应先完成 shadow 观察和回归验证。

### 3. 启动后端

```bash
cd backend
set -a
. ./.env
set +a
go run ./cmd/server
```

访问：

```text
http://127.0.0.1:8080/login.html
```

健康检查：

```bash
curl http://127.0.0.1:8080/api/health
```

## 测试

### 后端测试

```bash
cd backend
go test ./...
```

### 前端轻量测试

```bash
node --test tests/*.test.js
```

## 部署说明

部署文档参考：

- [后端服务部署说明](docs/BACKEND_SERVICES_DEPLOYMENT.md)
- [RBAC 测试服务器 enforce 验收报告](docs/RBAC_TEST_ENFORCE_ACCEPTANCE_REPORT.md)
- [生产 RBAC enforce 切换方案](docs/PRODUCTION_RBAC_ENFORCE_SWITCH_PLAN.md)

### 测试服务器当前验证状态

测试服务器已完成 `RBAC_MODE=enforce` 验证：

- 服务状态：active；
- PostgreSQL / Redis / LDAP / Slurm 健康检查正常；
- 关键回归 `failures=0`；
- 普通用户菜单、文件目录边界、Slurm 管理接口、作业列表、VNC、中间管理员只读角色权限、`cluster_admin` 管理权限均验证通过。

生产环境尚未切换到 enforce。生产切换需要单独评审和批准。

## 安全说明

仓库不应提交以下内容：

- `.env`；
- 数据库 dump；
- 运行日志；
- 用户上传文件；
- 服务器备份目录；
- 编译后二进制；
- LDAP / 数据库 / Redis / 飞书 Webhook 等真实密钥；
- SSH 私钥、证书、token。

相关规则已写入 `.gitignore`。

## 当前版本状态

当前版本定位为测试服务器已验证的功能版本，适合继续做：

1. 生产切换评审；
2. 安装部署文档完善；
3. 前端工程化改造；
4. API 文档与 OpenAPI 补充；
5. Slurm 多集群适配；
6. 更完整的自动化测试和 CI。

## License

本项目采用 Apache License 2.0，详见 [LICENSE](LICENSE)。
