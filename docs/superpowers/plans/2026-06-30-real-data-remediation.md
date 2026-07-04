# simpleHPC Real Data Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 清除当前版本中会被用户误认为真实集群数据的静态内容，补齐缺失的后端接口和持久化能力，使所有业务页面只展示来自 Slurm、LDAP、PostgreSQL、Redis、文件系统或已保存系统配置的真实数据。

**Architecture:** 保留当前 Go + Gin 单体后端和静态 HTML/JavaScript 前端结构。后端按监控、巡检、ACL、审计、平台设置和账户管理划分服务函数，并统一使用 PostgreSQL 保存管理数据；Slurm、LDAP、Redis 和存储文件系统继续作为实时数据源。前端不再使用硬编码业务记录或“仅弹成功提示”的伪操作，接口失败时明确显示“数据未获取”。

**Tech Stack:** Go 1.22、Gin、PostgreSQL、Redis、OpenLDAP、Slurm CLI、POSIX ACL、原生 HTML/CSS/JavaScript、Go testing。

## 0. 执行结果（2026-06-30）

| 阶段 | 状态 | 已完成内容 |
|---|---|---|
| 阶段一 | 已完成 | 会话用户与角色、实时服务状态、VNC 提示修正、作业真实提交入口和时间轴修正 |
| 阶段二 | 已完成 | 真实监控告警采集、查询、刷新和确认 |
| 阶段三 | 已完成 | 真实巡检执行、持久化记录、详情、配置和报告 |
| 阶段四 | 已完成 | LDAP 用户/团队创建与维护，单位、角色、管理员 CRUD，普通用户与管理员权限隔离 |
| 阶段五 | 已完成 | POSIX ACL、平台设置、审计日志及数据库兼容迁移 |
| 阶段六 | 已完成 | 存储元数据持久化、实时刷新、全量回归、服务器部署和真实数据验收 |

最终验证结果：

- `go test ./...`：通过。
- `node --test tests/*.test.js`：14/14 通过。
- 所有 `js/*.js` 均通过 `node --check`。
- 测试服务器 `/api/health`：PostgreSQL、Redis、LDAP、Slurm 全部为 `ok`。
- LDAP 临时团队与用户创建、18 位一次性密码、主目录创建、同步和列表查询闭环通过，测试数据已清理。
- 普通用户的管理员写操作返回 403；管理员 8 个管理数据接口读取均返回 200。
- 普通用户 Slurm 作业列表仅返回本人作业。
- 4 个存储根目录的名称、用途和告警阈值保存后读取一致。

说明：当前目录没有 Git 跟踪基线，文件均处于未跟踪状态，因此计划中的分阶段 `git commit` 步骤未执行；功能实现、测试和服务器部署不受影响。

---

## 1. 当前版本问题清单

### 1.1 完全静态、需要优先替换的页面

| 优先级 | 页面 | 当前问题 | 目标数据源 |
|---|---|---|---|
| P0 | `monitoring.html` | 告警数量、节点健康率、CPU 利用率、告警列表均为硬编码；页面只探测 API，不渲染接口结果 | `/api/v1/dashboard`、`/api/v1/slurm/nodes`、`dashboard_alerts` |
| P0 | `inspection.html` | 巡检结果、报告列表和“一键巡检完成”均为模拟；现有 `/api/v1/inspection/run` 未被页面调用 | PostgreSQL 巡检记录、服务健康检查、Slurm/LDAP/Redis/PostgreSQL 检查 |
| P0 | `data-acl.html` | 授权记录、编辑和新增操作均为演示数据 | 文件系统 POSIX ACL、平台用户和团队 |
| P1 | `audit.html` | 审计记录全部写死，没有查询、筛选和持久化 | PostgreSQL `audit_logs` |
| P1 | `settings.html` | 平台名称、Logo、语言写死；保存按钮不调用后端 | PostgreSQL `system_configs` |

### 1.2 部分真实、部分静态的页面

| 优先级 | 页面 | 已真实接入 | 尚未真实接入 |
|---|---|---|---|
| P0 | `job-list.html` | 作业列表、详情、状态、stdout/stderr、工作目录、作业操作 | 顶部“提交作业”是假提交；时间轴缺失时间使用固定日期 |
| P1 | `users.html` | 列表、LDAP 同步、冻结、解冻、重置密码、删除 | 新建、编辑只弹成功提示；邮件状态卡片不是实时检测 |
| P1 | `teams.html` | 团队列表、成员列表、成员冻结/解冻/改密/删除 | 新建/编辑团队、新增成员、冻结/删除团队是假操作 |
| P1 | `units.html` | 单位列表 | 新建、编辑、删除没有后端接口 |
| P1 | `roles.html` | 角色列表 | 新建角色、权限编辑没有保存接口 |
| P1 | `admins.html` | 列表、编辑、删除、重置密码 | 新建管理员未接入 |
| P1 | `storage.html` | 路径列表、容量、文件系统、路径保存 | “刷新监控”不调用后端；名称、类型、用途、阈值未完整持久化 |
| P2 | `index.html` | 用户、作业、节点、资源、趋势、存储来自真实接口 | 自定义组件预览仍显示固定样例；告警表缺少自动采集来源 |
| P2 | `vnc-desktop.html` | VNC 作业、节点、状态、访问地址均为真实数据 | `js/app.js` 仍错误提示“VNC 是前端原型” |

### 1.3 全局公共问题

- 所有业务页面侧栏固定显示“管理员 / cluster-admin”，没有根据 `simplehpc_user` 更新。
- 多数页面通知菜单固定显示“系统正常”或“LDAP 同步正常”，不是实时状态。
- `js/app.js` 的 API 状态声明只验证接口是否成功，不保证页面实际渲染了返回值。
- 页面中仍存在大量 `App.toast('操作成功')`，但没有对应网络请求。
- 接口失败时必须保留错误提示，禁止回退到虚构数据。
- `jobs.html`、`resources.html`、`orgs.html` 仅是导航页，不需要接业务数据，但应避免显示虚假的系统状态。

## 2. 实施阶段与排期

| 阶段 | 建议工期 | 交付范围 |
|---|---:|---|
| 阶段一 | 2 天 | 公共登录用户信息、通知状态、VNC 过期提示、作业列表假提交和固定时间 |
| 阶段二 | 3 天 | 监控告警、告警采集和告警确认 |
| 阶段三 | 3 天 | 巡检执行、巡检记录、报告列表和下载 |
| 阶段四 | 4 天 | 用户、团队、单位、角色、管理员缺失的 CRUD |
| 阶段五 | 3 天 | ACL、平台设置、审计日志 |
| 阶段六 | 2 天 | 存储元数据、刷新监控、全页面回归和部署 |

预计总工期：17 个开发日。每个阶段均应独立测试、部署和验收，不等待全部阶段完成后一次性上线。

## 3. 文件结构规划

### 后端新增文件

- `backend/internal/service/monitoring.go`：告警采集、查询、确认和解决。
- `backend/internal/service/inspection.go`：巡检执行、记录和报告查询。
- `backend/internal/service/audit.go`：审计日志模型、写入和查询。
- `backend/internal/service/acl.go`：ACL 查询、校验和下发。
- `backend/internal/httpapi/monitoring.go`：监控告警 HTTP 接口。
- `backend/internal/httpapi/inspection.go`：巡检 HTTP 接口。
- `backend/internal/httpapi/audit.go`：审计查询接口和审计中间件。
- `backend/internal/httpapi/acl.go`：目录授权接口。
- `backend/internal/service/monitoring_test.go`
- `backend/internal/service/inspection_test.go`
- `backend/internal/service/audit_test.go`
- `backend/internal/service/acl_test.go`

### 前端新增文件

- `js/session-ui.js`：登录用户、角色、头像和实时服务状态。
- `js/monitoring.js`：监控指标和告警列表渲染。
- `js/inspection.js`：巡检执行、列表和报告下载。
- `js/data-acl.js`：ACL 列表和授权操作。
- `js/audit.js`：审计日志查询、筛选和分页。
- `js/settings.js`：平台设置读取和保存。

### 主要修改文件

- `backend/internal/service/service.go`
- `backend/internal/service/accounts.go`
- `backend/internal/service/dashboard.go`
- `backend/internal/httpapi/router.go`
- `js/app.js`
- `js/account-pages.js`
- `job-list.html`
- `monitoring.html`
- `inspection.html`
- `data-acl.html`
- `audit.html`
- `settings.html`
- `users.html`
- `teams.html`
- `units.html`
- `roles.html`
- `admins.html`
- `storage.html`
- `index.html`
- `vnc-desktop.html`

---

### Task 1: 建立静态数据回归检查

**Files:**
- Create: `tests/static-data-audit.test.js`
- Modify: `backend/README.md`

- [ ] **Step 1: 编写当前会失败的静态数据检查**

```javascript
const fs = require('node:fs');
const assert = require('node:assert');

const forbidden = {
  'monitoring.html': ['gpu-021', '72 / 75 在线', '3,264 / 4,800 核'],
  'inspection.html': ['RPT-20260625-0630', '18 项检查正常'],
  'audit.html': ['old-mpi-template', '#1284593'],
  'data-acl.html': ['/project/ai-lab', '/home/zhangsan/share'],
  'job-list.html': ['2026-06-24 09:07:19', '2026-06-24 12:40:00']
};

for (const [file, values] of Object.entries(forbidden)) {
  const source = fs.readFileSync(file, 'utf8');
  for (const value of values) {
    assert(!source.includes(value), `${file} still contains static value: ${value}`);
  }
}
```

- [ ] **Step 2: 运行检查并确认失败**

Run: `node tests/static-data-audit.test.js`

Expected: FAIL，首个错误指向 `monitoring.html` 的硬编码集群数据。

- [ ] **Step 3: 在后续任务中逐项移除静态值**

每完成一个页面，运行：

```bash
node tests/static-data-audit.test.js
```

Expected: 未完成全部任务前仍会失败，但已修复页面不再出现在错误中。

- [ ] **Step 4: 提交回归检查**

```bash
git add tests/static-data-audit.test.js backend/README.md
git commit -m "test: add static cluster data audit"
```

### Task 2: 统一登录用户和服务状态

**Files:**
- Create: `js/session-ui.js`
- Modify: `js/app.js:388-462`
- Modify: all authenticated `*.html`

- [ ] **Step 1: 为会话 UI 编写浏览器单元测试**

```javascript
assert.deepStrictEqual(
  SessionUI.viewModel({username: 'user001', displayName: '测试用户', role: 'user', type: 'user'}),
  {name: '测试用户', account: 'user001', role: 'user', avatar: '测'}
);
```

- [ ] **Step 2: 实现会话信息渲染**

```javascript
window.SessionUI = {
  viewModel(user) {
    const name = user.displayName || user.username || '未登录';
    return {
      name,
      account: user.username || '',
      role: user.role || user.type || '',
      avatar: name.slice(0, 1)
    };
  },
  render() {
    const user = JSON.parse(localStorage.getItem('simplehpc_user') || '{}');
    const view = this.viewModel(user);
    document.querySelectorAll('.sidebar-user').forEach(element => {
      element.querySelector('.avatar').textContent = view.avatar;
      const labels = element.querySelectorAll('div > div');
      if (labels[0]) labels[0].textContent = view.name;
      if (labels[1]) labels[1].textContent = view.role;
    });
  }
};
```

- [ ] **Step 3: 用 `/api/health` 替换“系统正常”固定文案**

```javascript
const response = await fetch('/api/health', {cache: 'no-store'});
const health = await response.json();
const message = Object.entries(health.services)
  .map(([name, item]) => `${name}: ${item.status}`)
  .join(' · ');
```

- [ ] **Step 4: 删除 VNC 过期静态提示**

从 `js/app.js` 的 `staticNotices` 删除 `vnc-desktop.html`，保留真实 API 异常提示。

- [ ] **Step 5: 验证普通用户和管理员页面**

Run: `node tests/session-ui.test.js`

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add js/session-ui.js js/app.js *.html tests/session-ui.test.js
git commit -m "fix: render authenticated user and live service status"
```

### Task 3: 修复作业列表假提交和固定时间

**Files:**
- Modify: `job-list.html:61-70`
- Modify: `job-list.html:497-504`
- Test: `tests/static-data-audit.test.js`

- [ ] **Step 1: 将“提交作业”按钮改为进入真实模板页面**

```html
<a class="btn btn-primary" href="job-templates.html">提交作业</a>
```

- [ ] **Step 2: 删除时间轴固定日期兜底**

```javascript
function timelineValue(value, pendingText) {
  return value && value !== 'Unknown' ? value : pendingText;
}

const endTime = timelineValue(
  job.endTime,
  job.status === '排队中' ? '等待分配中' : '数据未获取'
);
```

- [ ] **Step 3: 让时间轴宽度表达状态，不伪装成精确比例**

```javascript
const segments = {
  '排队中': [100, 0, 0],
  '运行中': [30, 70, 0],
  '完成': [25, 75, 0],
  '失败': [25, 55, 20],
  '已取消': [25, 35, 40]
};
```

界面注明“阶段示意”，避免将比例误认为真实用时占比。

- [ ] **Step 4: 运行检查**

Run: `node tests/static-data-audit.test.js`

Expected: 不再报告 `job-list.html` 固定日期。

- [ ] **Step 5: 提交**

```bash
git add job-list.html tests/static-data-audit.test.js
git commit -m "fix: remove simulated job submission and timeline dates"
```

### Task 4: 接入真实监控和告警

**Files:**
- Create: `backend/internal/service/monitoring.go`
- Create: `backend/internal/service/monitoring_test.go`
- Create: `backend/internal/httpapi/monitoring.go`
- Create: `js/monitoring.js`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `backend/internal/service/dashboard.go`
- Modify: `monitoring.html`

- [ ] **Step 1: 编写告警确认失败测试**

```go
func TestAcknowledgeAlertChangesActiveAlert(t *testing.T) {
    alert := DashboardAlert{ID: 7, Status: "active"}
    got := acknowledgeAlert(alert, "testadmin", time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC))
    if got.Status != "acknowledged" || got.AcknowledgedBy != "testadmin" {
        t.Fatalf("unexpected alert: %#v", got)
    }
}
```

- [ ] **Step 2: 扩展告警模型**

```go
type DashboardAlert struct {
    ID             int64  `json:"id"`
    Level          string `json:"level"`
    Status         string `json:"status"`
    Title          string `json:"title"`
    Message        string `json:"message"`
    Source         string `json:"source"`
    OccurredAt     string `json:"occurredAt"`
    AcknowledgedBy string `json:"acknowledgedBy,omitempty"`
    AcknowledgedAt string `json:"acknowledgedAt,omitempty"`
}
```

- [ ] **Step 3: 增加数据库字段和告警接口**

```sql
ALTER TABLE dashboard_alerts ADD COLUMN IF NOT EXISTS acknowledged_by TEXT NOT NULL DEFAULT '';
ALTER TABLE dashboard_alerts ADD COLUMN IF NOT EXISTS acknowledged_at TIMESTAMPTZ;
```

```go
v1.GET("/monitoring/alerts", api.listMonitoringAlerts)
v1.POST("/monitoring/alerts/:id/acknowledge", api.acknowledgeMonitoringAlert)
v1.POST("/monitoring/refresh", api.refreshMonitoringAlerts)
```

- [ ] **Step 4: 实现最小告警采集规则**

首批规则：

- Slurm 节点状态包含 `DOWN`、`DRAIN`、`FAIL` 时创建节点告警。
- 存储路径使用率大于配置阈值时创建存储告警。
- Slurm、LDAP、PostgreSQL、Redis 健康检查失败时创建服务告警。
- 相同 `source + title` 的 active 告警不重复插入。
- 恢复正常后将对应告警标记为 `resolved`。

- [ ] **Step 5: 用接口数据渲染监控页面**

```javascript
const [dashboard, alerts] = await Promise.all([
  getJSON('/api/v1/dashboard'),
  getJSON('/api/v1/monitoring/alerts?status=active')
]);
renderResourceCards(dashboard.resources);
renderAlertSummary(alerts.items);
renderAlertRows(alerts.items);
```

- [ ] **Step 6: 验证**

Run: `cd backend && go test ./internal/service -run Monitoring -v`

Expected: PASS。

Run: `node tests/static-data-audit.test.js`

Expected: 不再报告 `monitoring.html`。

- [ ] **Step 7: 提交**

```bash
git add backend/internal/service/monitoring* backend/internal/httpapi/monitoring.go backend/internal/httpapi/router.go backend/internal/service/dashboard.go monitoring.html js/monitoring.js
git commit -m "feat: replace monitoring placeholders with live alerts"
```

### Task 5: 建立真实巡检记录和报告

**Files:**
- Create: `backend/internal/service/inspection.go`
- Create: `backend/internal/service/inspection_test.go`
- Create: `backend/internal/httpapi/inspection.go`
- Create: `js/inspection.js`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `backend/internal/service/service.go`
- Modify: `inspection.html`

- [ ] **Step 1: 编写巡检结果汇总测试**

```go
func TestInspectionSummaryIsWarningWhenAnyCheckFails(t *testing.T) {
    checks := []InspectionCheck{
        {Name: "PostgreSQL", Status: "ok"},
        {Name: "Slurm", Status: "error", Message: "slurmctld unavailable"},
    }
    if got := summarizeInspection(checks); got != "warning" {
        t.Fatalf("got %q", got)
    }
}
```

- [ ] **Step 2: 建立巡检表**

```sql
CREATE TABLE IF NOT EXISTS inspection_runs (
  id BIGSERIAL PRIMARY KEY,
  run_id TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL,
  checks JSONB NOT NULL DEFAULT '[]'::jsonb,
  problem_count INTEGER NOT NULL DEFAULT 0,
  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- [ ] **Step 3: 增加接口**

```go
v1.POST("/inspection/runs", api.runInspection)
v1.GET("/inspection/runs", api.listInspectionRuns)
v1.GET("/inspection/runs/:id", api.getInspectionRun)
v1.GET("/inspection/runs/:id/report", api.downloadInspectionReport)
v1.GET("/inspection/config", api.getInspectionConfig)
v1.PUT("/inspection/config", api.saveInspectionConfig)
```

- [ ] **Step 4: 保存真实检查结果**

检查 PostgreSQL、Redis、LDAP、Slurm、存储根目录可访问性、Slurm 节点异常数量，并将原始结果写入 `inspection_runs.checks`。

- [ ] **Step 5: 页面调用真实接口**

```javascript
await request('/api/v1/inspection/runs', {method: 'POST'});
await loadInspectionRuns();
```

一键巡检按钮只能在接口返回 2xx 后显示成功。

- [ ] **Step 6: 验证**

Run: `cd backend && go test ./internal/service -run Inspection -v`

Expected: PASS。

Run: `node tests/static-data-audit.test.js`

Expected: 不再报告 `inspection.html`。

- [ ] **Step 7: 提交**

```bash
git add backend/internal/service/inspection* backend/internal/httpapi/inspection.go backend/internal/httpapi/router.go backend/internal/service/service.go inspection.html js/inspection.js
git commit -m "feat: persist and display real inspection reports"
```

### Task 6: 补齐账户组织 CRUD

**Files:**
- Modify: `backend/internal/service/accounts.go`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `js/account-pages.js`
- Modify: `users.html`
- Modify: `teams.html`
- Modify: `units.html`
- Modify: `roles.html`
- Modify: `admins.html`
- Test: `backend/internal/service/accounts_test.go`

- [ ] **Step 1: 为创建用户和团队编写事务测试**

```go
func TestCreatePlatformUserRejectsDuplicateUsername(t *testing.T) {
    input := CreateUserInput{Username: "user001", Email: "user001@example.edu.cn"}
    err := validateCreateUser(input, map[string]bool{"user001": true})
    if !errors.Is(err, ErrAccountExists) {
        t.Fatalf("got %v", err)
    }
}
```

- [ ] **Step 2: 增加缺失接口**

```go
v1.POST("/account/users", api.createAccountUser)
v1.PUT("/account/users/:username", api.updateAccountUser)
v1.POST("/account/teams", api.createAccountTeam)
v1.PUT("/account/teams/:name", api.updateAccountTeam)
v1.POST("/account/teams/:name/freeze", api.freezeAccountTeam)
v1.DELETE("/account/teams/:name", api.deleteAccountTeam)
v1.POST("/account/teams/:name/members", api.createTeamMember)
v1.POST("/account/units", api.createAccountUnit)
v1.PUT("/account/units/:id", api.updateAccountUnit)
v1.DELETE("/account/units/:id", api.deleteAccountUnit)
v1.POST("/account/roles", api.createAccountRole)
v1.PUT("/account/roles/:code", api.updateAccountRole)
v1.POST("/account/admins", api.createAccountAdmin)
```

- [ ] **Step 3: 明确 LDAP 与数据库事务顺序**

创建用户和团队时：

1. 校验 PostgreSQL 唯一性。
2. 写 LDAP user/group。
3. 创建并校验主目录。
4. 写 PostgreSQL 平台记录。
5. 发送通知邮件。
6. 任一步失败时返回明确错误；LDAP 已写但数据库失败时执行补偿删除。

- [ ] **Step 4: 将所有假按钮改成真实请求**

```javascript
await fetchJSON('/api/v1/account/users', {
  method: 'POST',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify(payload)
});
```

禁止在没有接口响应时显示“已创建”“已保存”“邮件已发送”。

- [ ] **Step 5: 验证**

Run: `cd backend && go test ./internal/service -run 'Account|Team|Unit|Role|Admin' -v`

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add backend/internal/service/accounts.go backend/internal/service/accounts_test.go backend/internal/httpapi/router.go js/account-pages.js users.html teams.html units.html roles.html admins.html
git commit -m "feat: complete account and organization CRUD"
```

### Task 7: 接入目录 ACL

**Files:**
- Create: `backend/internal/service/acl.go`
- Create: `backend/internal/service/acl_test.go`
- Create: `backend/internal/httpapi/acl.go`
- Create: `js/data-acl.js`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `data-acl.html`

- [ ] **Step 1: 编写路径和权限校验测试**

```go
func TestValidateACLRequestRejectsPathOutsideStorageRoots(t *testing.T) {
    request := ACLRequest{Path: "/etc", SubjectType: "user", Subject: "user001", Permission: "rw"}
    err := ValidateACLRequest(request, []string{"/data/home", "/data/share"})
    if !errors.Is(err, ErrPathOutsideRoots) {
        t.Fatalf("got %v", err)
    }
}
```

- [ ] **Step 2: 增加 ACL 接口**

```go
v1.GET("/storage/acls", api.listStorageACLs)
v1.POST("/storage/acls", api.createStorageACL)
v1.PUT("/storage/acls/:id", api.updateStorageACL)
v1.DELETE("/storage/acls/:id", api.deleteStorageACL)
```

- [ ] **Step 3: 安全调用 POSIX ACL**

```go
args := []string{"-m", fmt.Sprintf("%s:%s:%s", subjectType, subject, permission), cleanPath}
cmd := exec.CommandContext(ctx, "setfacl", args...)
```

仅允许：

- 路径位于已配置存储根目录下。
- 用户存在于 `platform_users`。
- 组存在于 `teams.group_name`。
- 权限值为 `r`、`rw` 或 `rwx`。
- 不允许前端直接传递 shell 命令或额外参数。

- [ ] **Step 4: 持久化授权记录并与 `getfacl` 对账**

数据库保存对象、路径、权限、创建人和时间；列表接口返回数据库记录及文件系统当前 ACL 状态。

- [ ] **Step 5: 验证**

Run: `cd backend && go test ./internal/service -run ACL -v`

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add backend/internal/service/acl* backend/internal/httpapi/acl.go backend/internal/httpapi/router.go data-acl.html js/data-acl.js
git commit -m "feat: manage storage ACLs with real filesystem state"
```

### Task 8: 接入平台设置和审计日志

**Files:**
- Create: `backend/internal/service/audit.go`
- Create: `backend/internal/service/audit_test.go`
- Create: `backend/internal/httpapi/audit.go`
- Create: `js/audit.js`
- Create: `js/settings.js`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `backend/internal/service/service.go`
- Modify: `settings.html`
- Modify: `audit.html`

- [ ] **Step 1: 建立审计表**

```sql
CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGSERIAL PRIMARY KEY,
  actor TEXT NOT NULL,
  actor_type TEXT NOT NULL,
  action TEXT NOT NULL,
  target_type TEXT NOT NULL,
  target TEXT NOT NULL,
  result TEXT NOT NULL,
  detail JSONB NOT NULL DEFAULT '{}'::jsonb,
  request_id TEXT NOT NULL,
  ip_address TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- [ ] **Step 2: 增加平台设置接口**

```go
v1.GET("/config/platform", api.getPlatformConfig)
v1.PUT("/config/platform", api.savePlatformConfig)
```

保存到 `system_configs` 的 `platform` 键，字段限定为：

```json
{
  "name": "simpleHPC",
  "logo": "simpleHPC-logo.png",
  "loginImage": "login-bg.jpg",
  "language": "zh-CN"
}
```

- [ ] **Step 3: 增加审计查询接口**

```go
v1.GET("/audit/logs", api.listAuditLogs)
```

支持 `actor`、`action`、`result`、`dateFrom`、`dateTo`、`page`、`pageSize`。

- [ ] **Step 4: 在写操作中记录审计**

```go
err := api.services.RecordAudit(ctx, service.AuditEntry{
    Actor: user.Username,
    Action: "slurm.job.cancel",
    TargetType: "job",
    Target: jobID,
    Result: "success",
})
```

优先覆盖登录、账户操作、模板操作、作业操作、配置修改、ACL 和巡检。

- [ ] **Step 5: 前端渲染真实数据**

`settings.html` 启动时 GET 配置，保存时 PUT；`audit.html` 使用分页接口，删除全部硬编码记录。

- [ ] **Step 6: 验证**

Run: `cd backend && go test ./internal/service -run Audit -v`

Expected: PASS。

Run: `node tests/static-data-audit.test.js`

Expected: 不再报告 `audit.html`、`settings.html` 和 `data-acl.html`。

- [ ] **Step 7: 提交**

```bash
git add backend/internal/service/audit* backend/internal/httpapi/audit.go backend/internal/httpapi/router.go backend/internal/service/service.go settings.html audit.html js/settings.js js/audit.js
git commit -m "feat: persist platform settings and audit logs"
```

### Task 9: 完善存储配置和实时刷新

**Files:**
- Modify: `backend/internal/integrations/storage/client.go`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `storage.html`
- Test: `backend/internal/integrations/storage/client_test.go`

- [ ] **Step 1: 定义完整存储配置**

```go
type RootConfig struct {
    Type             string `json:"type"`
    Name             string `json:"name"`
    Path             string `json:"path"`
    FSType           string `json:"fsType"`
    Purpose          string `json:"purpose"`
    WarningThreshold int    `json:"warningThreshold"`
}
```

- [ ] **Step 2: 修改保存接口**

```json
{
  "roots": [
    {
      "type": "home",
      "name": "用户主目录",
      "path": "/data/home",
      "fsType": "xfs",
      "purpose": "用户家目录",
      "warningThreshold": 85
    }
  ]
}
```

后端验证路径存在、阈值为 1–100，并把完整对象写入 `system_configs.storage`。

- [ ] **Step 3: 增加刷新接口**

```go
v1.POST("/storage/roots/refresh", api.refreshStorageRoots)
```

接口立即重新执行 `statfs`，返回最新容量和错误信息。

- [ ] **Step 4: 前端调用真实刷新**

```javascript
await request('/api/v1/storage/roots/refresh', {method: 'POST'});
await loadStorageConfig();
```

- [ ] **Step 5: 验证**

Run: `cd backend && go test ./internal/integrations/storage -v`

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add backend/internal/integrations/storage backend/internal/httpapi/router.go storage.html
git commit -m "feat: persist storage metadata and refresh usage"
```

### Task 10: 全量验证、部署和验收

**Files:**
- Modify: `docs/BACKEND_SERVICES_DEPLOYMENT.md`
- Modify: `docs/superpowers/plans/2026-06-30-real-data-remediation.md`

- [ ] **Step 1: 运行后端测试**

Run:

```bash
cd backend
go test ./...
```

Expected: 所有 package PASS，退出码 0。

- [ ] **Step 2: 运行静态数据审计**

Run:

```bash
cd /Users/jiaguangqi/Documents/simpleHPC
node tests/static-data-audit.test.js
```

Expected: PASS，不再发现集群指标、报告、告警、ACL、审计或作业时间的硬编码样例。

- [ ] **Step 3: 检查所有成功提示都有真实请求**

Run:

```bash
rg -n "App\\.toast\\(.+(成功|完成|已保存|已创建|已删除)" *.html js
```

逐项确认同一操作函数中存在成功的 `fetch`/`request` 调用；纯界面操作如“已切换标签”可保留。

- [ ] **Step 4: 服务器部署前检查**

```bash
ssh root@10.10.38.152 \
  'systemctl is-active simplehpc-backend; ss -lntp | grep :8080; squeue; sinfo'
```

Expected: 后端 active、8080 正常监听、Slurm 命令正常。

- [ ] **Step 5: 部署并验证健康状态**

```bash
curl -fsS http://10.10.38.152:8080/api/health
```

Expected: PostgreSQL、Redis、LDAP、Slurm 均为 `ok`。

- [ ] **Step 6: 角色验收**

管理员验收：

- 能看到真实监控、巡检、审计和全部作业。
- 能新增和编辑账户、团队、单位、角色、管理员。
- 能配置 ACL、平台设置和存储。

普通用户验收：

- 只能看到自己的作业和允许访问的数据目录。
- 不能调用管理员配置、ACL、审计和账户管理写接口。
- 页面侧栏显示本人姓名和角色，不显示固定管理员身份。

- [ ] **Step 7: 数据真实性验收**

在终端改变以下真实状态并刷新页面：

1. 提交和结束一个 Slurm 作业。
2. 将测试节点置为 DRAIN 后恢复。
3. 创建一条测试 ACL 后删除。
4. 执行一次巡检。
5. 修改平台名称后恢复。

Expected: 页面变化与终端、数据库和文件系统状态一致，不出现固定样例或无请求的成功提示。

- [ ] **Step 8: 更新计划状态并提交**

在本文“4. 进度记录”中填写完成日期和验证结果，然后：

```bash
git add docs/BACKEND_SERVICES_DEPLOYMENT.md docs/superpowers/plans/2026-06-30-real-data-remediation.md
git commit -m "docs: record real data remediation verification"
```

## 4. 进度记录

| 阶段 | 状态 | 开始日期 | 完成日期 | 验证记录 |
|---|---|---|---|---|
| 阶段一：公共信息与作业页 | 已完成 | 2026-06-30 | 2026-06-30 | 会话 UI、健康状态、作业提交入口和时间轴已验证并部署 |
| 阶段二：监控告警 | 已完成 | 2026-06-30 | 2026-06-30 | 告警查询、刷新、确认和真实页面渲染已部署 |
| 阶段三：巡检系统 | 已完成 | 2026-06-30 | 2026-06-30 | 巡检执行、持久化、配置、列表和报告下载已部署 |
| 阶段四：账户组织 CRUD | 进行中 | 2026-06-30 | — | 单位、角色、管理员创建已完成；LDAP 用户和团队创建/编辑待完成 |
| 阶段五：ACL、设置、审计 | 已完成 | 2026-06-30 | 2026-06-30 | POSIX ACL、平台配置、审计查询已部署并通过真实接口验证 |
| 阶段六：存储与全量验收 | 进行中 | 2026-06-30 | — | 存储用量真实刷新已完成；完整元数据持久化和最终角色验收待完成 |

## 5. 完成定义

只有同时满足以下条件，才能将本计划标记为完成：

- 所有业务数据均来自明确的后端数据源。
- 所有成功提示均发生在后端成功响应之后。
- 页面不再使用固定集群指标、固定报告、固定告警或固定用户身份。
- 普通用户和管理员的数据权限均由后端强制执行。
- 后端全量测试、静态数据审计和服务器健康检查全部通过。
- 页面数据与 `squeue`、`sinfo`、LDAP、PostgreSQL、Redis、POSIX ACL 和文件系统实际状态一致。
