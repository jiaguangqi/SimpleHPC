# Inspection Artifacts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在真实测试集群中执行可追溯巡检，并生成独立 HTML 总结报告与详细日志。

**Architecture:** 后端固定白名单巡检项串行执行并记录原始证据，摘要与产物保存在 PostgreSQL；前端列表只提供两个独立预览入口。HTML 使用 A3 打印 CSS，日志使用纯文本下载。

**Tech Stack:** Go 1.22、Gin、PostgreSQL、Slurm CLI、HTML/CSS/JavaScript。

---

### Task 1: 可追溯巡检执行器

**Files:**
- Modify: `backend/internal/service/inspection.go`
- Modify: `backend/internal/service/inspection_test.go`

- [x] 测试命令结果必须包含命令、stdout、stderr、退出码和耗时。
- [x] 测试 unavailable 工具转换为 skipped，不伪造正常。
- [x] 实现固定白名单检查与日志格式化。
- [x] 运行 `go test ./internal/service -run Inspection`。

### Task 2: 巡检持久化和报告

**Files:**
- Modify: `backend/internal/service/service.go`
- Create: `backend/internal/service/inspection_report.go`
- Modify: `backend/internal/service/inspection_test.go`

- [x] 测试报告包含 A3 print CSS、真实集群名称与巡检摘要。
- [x] 增加巡检产物数据库字段。
- [x] 生成浅色科技风 HTML 与纯文本详细日志。
- [x] 列表查询不返回大字段，详情接口按需读取。

### Task 3: 报告与日志接口

**Files:**
- Modify: `backend/internal/httpapi/router.go`

- [x] 增加 `/inspection/runs/:id/report` HTML 预览。
- [x] 增加 `/inspection/runs/:id/log` 日志预览。
- [x] 增加 `/inspection/runs/:id/log/download` 日志下载。
- [x] 保持管理员执行权限与登录查看权限。

### Task 4: 页面集成

**Files:**
- Modify: `inspection.html`
- Modify: `js/inspection.js`
- Create: `inspection-report.html`
- Create: `inspection-log.html`

- [x] 列表增加真实耗时和两个独立操作按钮。
- [x] 实现 A3 报告独立预览及打印 PDF。
- [x] 实现日志独立预览及下载。
- [x] 对接口错误显示“数据未获取”，不使用模拟回退。

### Task 5: 服务器部署验收

- [x] 同步完整后端源码到 `/data/simpleHPC/backend`。
- [x] 构建并重启 `simplehpc-backend`。
- [x] 在服务器执行一次真实巡检。
- [x] 对照 `sinfo`、`df`、`sacct` 验证报告数据。
- [x] 验证 HTML、日志预览、日志下载与 A3 打印 CSS。
