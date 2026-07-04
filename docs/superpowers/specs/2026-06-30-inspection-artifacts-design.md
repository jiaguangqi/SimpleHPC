# simpleHPC 巡检报告与日志设计

## 目标

在测试服务器真实执行集群巡检。每次“一键巡检”产生两份独立产物：

1. A3 竖版、浅色科技风的 HTML 总结报告，可在线预览并通过浏览器打印为 A3 PDF。
2. 详细纯文本日志，记录每个巡检项、执行节点、命令、开始结束时间、耗时、退出码、stdout、stderr 和判定结果，可在线预览和下载。

## 数据真实性

巡检只使用测试集群实际存在的能力。当前环境为单节点 `simplehpc-dev`，20 CPU、30000 MB 内存、无 GPU GRES、未安装 NVIDIA 与 IB 工具。因此 GPU 和 IB 项显示“未配置/跳过”，不得生成模拟数量或健康值。

真实检查覆盖：

- 主机、负载、内存和运行时间。
- `slurmctld`、`slurmd`、`simplehpc-backend` 服务。
- `scontrol ping`、`sinfo`、`squeue`、`sdiag`、近 7 天 `sacct`。
- `/data/home`、`/data/share`、`/data/recycle`、`/data/scratch` 容量与 inode。
- PostgreSQL `SELECT 1`、Redis `PING`、LDAP bind。
- NVIDIA GPU 与 InfiniBand 工具存在性；不存在时明确记录 skipped。

## 持久化

`inspection_runs` 新增：

- `started_at`、`finished_at`、`duration_ms`
- `report_html`
- `detail_log`
- `summary` JSONB

列表读取摘要，不重复返回大体积报告和日志。独立接口读取 HTML、日志及日志下载。

## 页面交互

巡检列表操作列显示两个按钮：

- “总结报告”：打开独立报告预览页，页面右上角“导出 A3 PDF”调用浏览器打印。
- “详细日志”：打开独立日志预览页，页面右上角“下载日志”下载 `.log`。

巡检运行期间按钮显示正在执行；完成后显示真实耗时、通过/警告/失败数量。

## 安全

- 仅管理员可执行巡检。
- 报告和日志接口要求有效会话。
- 命令由后端固定白名单定义，不接收用户命令参数。
- 日志不记录数据库、Redis 或 LDAP 密码。
- stdout/stderr 单项限制长度，防止数据库无限增长。

## 验收

- 服务器一键巡检耗时和每项命令均可追溯。
- HTML 中的节点、CPU、内存、存储、作业数据与服务器命令一致。
- GPU/IB 未配置时显示 skipped。
- 报告、日志在线预览和日志下载均可用。
- A3 打印样式为 `@page { size: A3 portrait; }`。
