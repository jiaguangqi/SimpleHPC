# SimpleHPC

SimpleHPC 是一套面向高校、科研院所、超算/智算中心的小型 HPC 集群管理与作业调度平台。系统围绕 Slurm、LDAP、共享存储、WebSSH、VNC 桌面和 RBAC 权限体系构建，目标是让集群管理员、团队负责人和普通用户可以通过 Web 页面完成日常集群使用与运维管理。

> 当前仓库版本定位：测试环境验证版，已在测试服务器完成 RBAC enforce 模式验证。生产环境切换仍需要单独评审和部署窗口。

## 核心能力

### 1. 集群驾驶舱

- 在线用户、运行作业、排队作业、存储使用等关键指标概览。
- 集群资源使用趋势，展示 CPU/GPU 平均利用率。
- 资源池作业趋势，按 Slurm partition 展示运行作业和排队作业趋势。
- 浅色 Apple-style SaaS UI，适合日常运维和客户展示。

### 2. Slurm 作业管理

- 队列状态、节点状态、QOS 策略、资源队列配置。
- 作业列表、作业详情、stdout/stderr 输出查看。
- 作业模板提交，支持 Shell、VNC/noVNC 等模板。
- VNC 作业可通过 noVNC 在线访问计算节点桌面。
- 作业资源、工作目录、输出文件与 Slurm 实际状态对齐。

### 3. WebSSH 终端中心

- 浏览器内创建真实 SSH 终端会话。
- 使用 xterm.js 支持交互式命令、vim 等全屏终端程序。
- 会话持久化，刷新页面后可恢复已创建终端标签。
- 登录节点由系统设置配置，仅被标记为登录节点的主机可被分配。
- 支持登录节点轮询分配和负载均衡策略。
- 左侧内置轻量文件管理器，支持授权目录浏览、上传、下载和打包下载。

### 4. 数据目录与文件安全

- 支持多个授权存储根目录，例如 `/data/home`、`/data/share`、`/data/recycle`、`/data/scratch`。
- 普通用户访问边界固定为 `/授权目录/{username}`。
- 后端统一校验列表、上传、下载、删除、复制、移动、重命名、打包下载等文件操作。
- 防路径穿越、URL 编码绕过、符号链接逃逸和归档逃逸。
- 管理员可查看授权根目录，普通用户不能看到其他用户目录。

### 5. RBAC 权限体系

- 支持内置角色和自定义角色共存。
- 内置角色包括：
  - `cluster_admin` 集群管理员
  - `config_admin` 配置管理员
  - `unit_admin` 单位管理员
  - `team_admin` 团队管理员
  - `user` 普通用户
- 支持菜单权限、路由权限、按钮权限、接口权限、数据范围权限、文件目录策略。
- 多角色按启用角色权限取并集，数据范围按 `global > unit > team > self/granted > none` 合并。
- 支持权限矩阵、角色复制、角色禁用、用户绑定。
- 当前测试服务器已完成 `RBAC_MODE=enforce` 验证。

### 6. 账户、团队与 LDAP 集成

- 管理员账号、LDAP 用户、单位、团队管理。
- 新建用户组采用向导流程：先创建团队，再创建组长账号。
- 组长默认绑定 `team_admin`，并作为团队第一个成员。
- 用户创建后可自动准备家目录、默认环境文件和 SSH 免密基础配置。

### 7. 巡检、日志与通知

- 一键巡检生成两类产物：
  - 精美 HTML 巡检报告，可在线预览和下载。
  - 详细巡检日志，包含巡检项、命令和输出。
- 支持飞书机器人通知。
- 日志中心包括用户登录日志、系统日志、审计日志。
- 系统日志支持服务来源、时间范围、日志级别和关键字筛选。

### 8. 平台配置

- 平台名称、Logo、登录页图片。
- LDAP、Slurm、存储、通知、终端登录节点配置。
- 登录前公共配置接口仅返回平台名称和图片地址。
- 图片资源保存到服务器资源目录，不使用 Base64 写入数据库。

### 9. 应用软件 License 监控

- 管理商业软件应用目录、应用 Logo、License 管理器和 License Server 配置。
- 支持 FlexNet `lmstat`、`lmutil lmstat` 和 RLM `rlmutil` 等采集方式。
- 支持测试连接、立即采集、原始输出查看、采集日志和错误提示。
- 监控 License 服务状态、总点数、使用中、空闲、排队、使用率、高负载、过期提醒和异常服务。
- 支持 License 使用趋势、Feature 使用统计、用户占用会话和告警摘要。
- License 服务可配置 systemd 服务名，支持在测试环境中通过页面触发启动、停止和重启。

## 技术架构

```text
Browser
  ├─ HTML/CSS/JavaScript
  ├─ xterm.js WebSSH
  └─ noVNC Desktop

Go Backend
  ├─ Gin HTTP API
  ├─ RBAC Resolver
  ├─ Slurm Integration
  ├─ LDAP Integration
  ├─ Storage Security Layer
  ├─ WebSSH Session Manager
  ├─ Inspection / Log / Notification Services
  └─ Commercial Software License Monitor

Infrastructure
  ├─ PostgreSQL
  ├─ Redis
  ├─ OpenLDAP
  ├─ Slurm / SlurmDBD
  └─ Shared Storage
```

## 目录结构

```text
.
├── *.html                         # 前端页面
├── css/                            # 全局主题与视觉样式
├── js/                             # 前端交互逻辑
├── assets/                         # 图标、插图、xterm/noVNC 相关静态资源
├── backend/
│   ├── cmd/server/                 # Go 后端入口
│   ├── internal/config/            # 配置加载
│   ├── internal/httpapi/           # HTTP API 与路由鉴权
│   ├── internal/integrations/      # LDAP / Slurm / Storage 集成
│   ├── internal/service/           # 业务服务与 RBAC 内核
│   ├── migrations/                 # PostgreSQL 迁移脚本
│   └── scripts/                    # 运维辅助脚本
├── docs/                           # 设计、部署、RBAC、发版文档
├── deploy/                         # 部署配置示例
└── tests/                          # 前端 Node 测试
```

## 快速启动

### 1. 后端配置

复制环境变量模板：

```bash
cd backend
cp .env.example .env
```

至少需要配置：

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
STORAGE_ROOTS=/data/home,/data/share,/data/recycle,/data/scratch
RBAC_MODE=shadow
```

> `RBAC_MODE` 支持 `legacy`、`shadow`、`enforce`。新环境建议先使用 `shadow` 完成观察，再切换 `enforce`。

### 2. 数据库迁移

迁移脚本位于 `backend/migrations/`，按编号顺序执行：

```bash
psql "$DATABASE_URL" -f backend/migrations/001_init.sql
psql "$DATABASE_URL" -f backend/migrations/002_rbac_schema.sql
psql "$DATABASE_URL" -f backend/migrations/003_rbac_seed.sql
# 按编号继续执行后续迁移
```

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

不要直接用 `file://.../login.html` 方式登录；登录、权限和页面数据都需要后端 API。

## 测试

### 前端 Node 测试

```bash
node tests/rbac-frontend.test.js
node tests/terminal-page.test.js
node tests/session-ui.test.js
node tests/storage-boundary-ui.test.js
```

也可以运行全部前端测试：

```bash
node --test tests/*.test.js
```

### 后端 Go 测试

```bash
cd backend
go test ./...
```

## 安全与版本管理约定

- 不提交 `.env`、真实密码、密钥、数据库 dump、运行日志和构建二进制。
- 所有新增接口必须登记 RBAC 权限点。
- 文件操作必须走后端路径校验，前端不能自行拼接越权路径。
- RBAC 从 `shadow` 切换到 `enforce` 前必须完成差异清零和回归验证。
- 生产切换必须单独提交切换方案，不直接复用测试环境操作。

详细发版流程见：[docs/RELEASE_PROCESS.md](docs/RELEASE_PROCESS.md)。

## 当前版本

当前版本：`v0.4.0`。

当前版本变更见：[CHANGELOG.md](CHANGELOG.md)。
