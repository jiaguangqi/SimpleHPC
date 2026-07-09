# Changelog

本文档记录 SimpleHPC 的主要版本变更。格式参考 [Keep a Changelog](https://keepachangelog.com/)，版本号采用 `MAJOR.MINOR.PATCH` 语义化版本。

## [0.4.0] - 2026-07-09

### Added

- 新增项目中心 / Project Workspace：
  - 项目用于承载课题、成员、任务、数据空间、Slurm 作业和资源账本。
  - 项目与 Slurm `account` 对齐，项目编码默认作为 Slurm Account。
  - 支持项目创建、编辑、删除、搜索、状态筛选、详情查看和项目动态记录。
  - 支持项目成员、项目任务、项目数据空间和项目作业关联管理。
- 新增 Slurm Account 对接能力：
  - 项目保存 Slurm Account、父级 Account、QOS、同步开关、同步状态和同步消息。
  - 支持通过 `sacctmgr` 创建/维护 Account，并将项目成员关联到对应 Account。
  - 支持为成员设置默认项目，后台同步为用户默认 Account。
  - 新增项目同步接口和 RBAC 权限点。
- 作业模板提交支持项目记账：
  - 用户提交模板时可选择自己参与的项目。
  - 后端校验用户是否有权使用所选项目 Account。
  - 生成 Slurm 脚本时自动注入 `#SBATCH --account=<项目 Account>`。
  - 提交成功后将作业记录关联到项目账本。
- 作业列表支持项目视角：
  - Slurm 同步作业记录新增 `account` 字段。
  - 作业列表新增项目筛选和项目/Account 展示列。
  - 作业详情展示项目和 Slurm Account。
  - 项目详情页自动汇总同 Account 的 Slurm 作业。
- 新增数据库迁移：
  - `015_project_center`
  - `016_project_slurm_account`

### Changed

- 导航新增“项目中心”，并纳入顶部导航和 RBAC 菜单权限。
- 作业同步同时采集 `squeue` / `sacct` 中的 Account 字段，便于按项目归集。
- 项目页采用工作台式布局，适配顶部导航后的宽屏和移动端基本阅读。
- 作业模板页从项目入口跳转时可自动选中指定项目。

### Fixed

- 修复同步作业查询中使用 `sj.` 别名但缺少表别名导致的运行时 SQL 问题。
- 修复新增迁移后嵌入迁移测试期望数量未更新的问题。
- 修复项目页确认弹窗中的异步操作失败时缺少中文错误反馈的问题。

### Deployment

- 测试服务器 `/data/simpleHPC` 已部署并验证：
  - 后端服务重启后保持 active。
  - 数据库迁移已应用到 `016_project_slurm_account`。
  - `/projects.html`、`/job-list.html`、`/js/projects.js`、`/js/job-templates.js` 返回 200。
  - `/api/v1/projects`、`/api/v1/slurm/jobs`、`/api/v1/job-templates` smoke test 返回 200。
  - 作业模板预览已验证生成 `#SBATCH --account=<项目 Account>`。
  - 项目 Slurm 同步接口已验证返回 success。

### Security

- Slurm Account、QOS、Linux 用户名、项目编码和作业参数均进行格式校验。
- Slurm Account 同步使用参数化外部命令调用，不拼接 shell 字符串。
- 外部 Slurm 命令失败时返回脱敏中文错误，不暴露服务器敏感配置。
- 本版本不包含真实 `.env`、密钥、数据库 dump、运行日志和后端构建二进制。

## [0.3.0] - 2026-07-08

### Added

- 新增应用软件 License 监控管理：
  - 支持商业软件应用目录管理，内置 ANSYS、Abaqus、COMSOL、MATLAB、Fluent、STAR-CCM+、Gaussian、Materials Studio 等常见软件。
  - 支持应用 Logo 上传，兼容 PNG、SVG、JPG、JPEG、WebP、GIF，License 配置页、监控页、采集日志和详情弹窗统一读取应用图标。
  - 支持 License 管理器目录管理，内置 FlexNet `lmstat`、`lmutil lmstat` 和 RLM `rlmutil` 模板。
  - 支持 License Server 配置、测试连接、立即采集、Feature 解析、原始输出展示、采集日志和错误提示。
  - 新增 License 监控页，展示已接入软件、License 总点数、当前使用中、使用率、高负载软件、异常服务、趋势图、Feature 使用统计、告警摘要和详情弹窗。
  - 新增 License 监控数据库迁移 `014_license_monitoring`，包括许可证配置、Feature、使用会话和趋势采样表。

### Changed

- 优化顶部导航时代的页面布局规范，License 配置和监控页面改为更紧凑、可扫描的工作台式布局。
- 优化 License 配置页的管理应用、License 管理器、许可证管理和采集日志四个子页面。
- 优化 License 监控图表：无采样记录时显示 0 值基线，不再生成模拟数据。
- 优化管理后台图标、卡片、按钮、弹窗和移动端基础适配。
- 登录页、顶部导航 Logo 和 favicon 更新为 SimpleHPC 统一品牌资产。
- 后端 systemd 状态识别优先读取 `systemctl show ActiveState`，降低旧版 systemd 对 forking 服务返回 `unknown` 的误判。

### Fixed

- 修复许可证管理页重复出现“新增许可证”按钮的问题。
- 修复新增许可证后左侧许可证列表退化为纯文本布局的问题。
- 修复应用 Logo 只在应用管理页生效、其他 License 页面不自动加载的问题。
- 修复 License 服务未填写 systemd 服务名时状态显示为 `unmanaged` 但缺少可解释行为的问题。
- 修复团队创建、账号映射、权限矩阵和导航权限的若干边界问题，保持顶部导航与 RBAC 数据一致。

### Deployment

- 测试服务器已完成 ANSYS License 服务托管验证：
  - 新增 `ansys-license.service`，封装 ANSYS 自带 `start_ansyslmd` / `stop_ansyslmd` 脚本。
  - 已验证 `systemctl stop/start/restart ansys-license.service` 可实际停止、启动和重启 ANSYS FlexNet 服务。
  - 已将 ANSYS License 配置的 `service_name` 更新为 `ansys-license.service`。
  - 已验证 `lmstat` 返回 `license server UP` 与 `ansyslmd: UP`。
- 测试服务器已完成 simpleHPC 后端二进制替换和健康接口 smoke test。

### Security

- License 采集命令限制为受控 License 管理器命令，不执行任意 shell。
- License 服务控制仅通过已配置的 systemd 服务名执行，不在前端暴露服务器命令或敏感路径。
- 本版本不包含真实 `.env`、密钥、数据库 dump、运行日志、License 文件和后端构建二进制。

## [0.2.1] - 2026-07-05

### Changed

- 优化 Slurm 配置页面：
  - Controller 地址拆分为主节点和备节点，主节点必填，备节点选填。
  - 新增可选 Slurm MySQL 数据库对接配置，支持数据库 IP、端口、管理员账号和管理员密码。
  - 后端保存配置时增加主节点必填校验、地址格式校验、MySQL 端口校验。
  - MySQL 管理员密码保存后不在配置读取接口中明文回显，留空保存时保持原密码不变。
- 优化 LDAP 配置页面：
  - LDAP Server 地址拆分为主节点和备节点，主节点必填，备节点选填。
  - 后端保存配置时增加 LDAP URL 协议、主机和端口校验。
  - Bind 密码保存后不在配置读取接口中明文回显，留空保存时保持原密码不变。

## [0.2.0] - 2026-07-05

### Added

- 新增 WebSSH「终端中心」：
  - 支持真实 SSH 终端会话创建、WebSocket 连接、xterm.js 交互。
  - 支持终端标签、会话恢复、断开/重连、窗口 resize。
  - 支持终端登录节点配置，登录节点可在系统设置中维护。
  - 支持轮询分配和负载均衡分配策略。
  - 左侧集成轻量文件管理器，支持授权目录浏览、上传、下载、目录打包下载和多选打包下载。
- 新增终端中心相关数据库迁移：
  - `011_terminal_center`
  - `012_terminal_login_nodes`
  - `013_webssh_api_permissions`
- 新增并完善 RBAC 权限体系：
  - 菜单、路由、按钮、API、数据范围和文件目录策略统一管控。
  - 支持内置角色与自定义角色共存。
  - 支持角色权限矩阵、角色复制、角色禁用、角色绑定用户。
  - 支持多角色权限合并。
  - 测试服务器已完成 `RBAC_MODE=enforce` 验证。
- 新增文件目录安全边界：
  - 普通用户访问范围锁定到 `/授权目录/{username}`。
  - 后端覆盖文件列表、上传、下载、删除、复制、移动、重命名、打包下载等权限校验。
  - 增加路径穿越、符号链接逃逸、归档逃逸、TOCTOU 加固。
- 新增 VNC/noVNC 桌面作业能力：
  - 支持通过 Slurm 提交 VNC 桌面作业。
  - 支持 VNC 桌面任务列表和在线访问。
  - 支持默认运行时间、分辨率和桌面环境参数。
- 新增巡检报告产物：
  - HTML 巡检总结报告。
  - 详细巡检日志。
  - 巡检报告列表支持在线预览、下载和飞书通知。
- 新增日志中心：
  - 用户登录日志。
  - 系统日志。
  - 审计日志。
  - 系统日志支持日志级别筛选。
- 新增平台设置能力：
  - 平台名称、主页面 Logo、登录页图片上传。
  - 登录前公共配置接口。
  - 终端登录节点配置。
- 新增仪表盘资源趋势：
  - 集群资源使用趋势。
  - 资源池作业趋势，按 partition 展示运行和排队作业趋势。

### Changed

- 前端整体视觉升级为浅色 Apple-style SaaS 风格。
- 登录页重构为品牌展示区 + 登录操作区的两栏布局。
- 导航从左侧菜单演进为顶部导航 + 菜单总览面板。
- 菜单总览支持一级/二级菜单结构展示、星标常用菜单、拖拽排序、名称编辑和新增菜单入口。
- 普通用户菜单保持扁平化，只显示：
  - 仪表盘
  - 队列状态
  - 数据目录
  - 作业模板
  - 作业列表
  - VNC 桌面
  - 终端中心（有权限时）
- 作业列表、作业详情、模板提交、stdout/stderr 输出读取等页面与 Slurm 真实数据对齐。
- 文件管理器普通用户进入授权目录时自动映射到用户专属目录。
- 管理员/配置管理员账号的 Linux 映射策略调整为 root。
- 用户创建流程增强：
  - 自动构建家目录。
  - 自动生成默认环境文件。
  - 自动准备 SSH 免密基础配置。
  - 取消登录节点 known_hosts 人工确认。

### Fixed

- 修复通过作业模板提交时资源参数未正确传递的问题。
- 修复新提交作业不能及时出现在作业列表的问题。
- 修复作业详情与 Slurm 终端查询信息不一致的问题。
- 修复作业列表状态刷新不及时的问题。
- 修复普通用户可看到其他用户作业摘要的数据范围越权问题。
- 修复普通用户可看到其他用户目录的文件管理器权限问题。
- 修复文件管理器上一级按钮可突破用户初始目录的问题。
- 修复 Slurm 节点增加后部分调度页面 401/403 权限映射异常。
- 修复角色列表/权限矩阵中间管理员只读权限不完整的问题。
- 修复 RBAC shadow 统计口径中 API 层允许但服务层拒绝时的差异误判。
- 修复顶部导航一级菜单 hover/click 后二级菜单被父容器裁剪不可见的问题。
- 修复 WebSSH 会话刷新后标签丢失的问题。
- 修复 WebSSH 终端输入焦点、退格、光标和 vim 交互异常。

### Security

- RBAC 默认白名单模式，未授权即无权限。
- `cluster_admin` 保留最高权限兜底。
- 禁用角色后权限立即失效。
- 文件服务引入 fd-relative / no-follow 风险控制，降低 TOCTOU 与符号链接逃逸风险。
- 普通用户不能访问管理员页面、管理员接口、其他用户作业详情或其他用户目录。
- Slurm 管理类接口对普通用户保持 403，只开放队列状态等允许入口。

### Operational Notes

- 测试服务器已完成 24 小时 shadow 观察和 enforce 切换验证。
- 生产环境尚未切换 enforce，需要单独提交生产切换方案并评审。
- 本版本不包含真实 `.env`、密钥、数据库 dump、运行日志和构建二进制。

## [0.1.0] - 2026-06-24

### Added

- 初始 SimpleHPC 静态管理界面与 Go 后端原型。
- 基础 LDAP、Slurm、存储目录、作业列表、巡检等接口雏形。
