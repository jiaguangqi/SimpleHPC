# SimpleHPC 发版与代码管理流程

本文档用于把 SimpleHPC 从“测试服务器快速迭代”逐步规范成可追溯、可回滚、可协作的产品项目。

## 版本号规则

采用语义化版本：

```text
MAJOR.MINOR.PATCH
```

- `MAJOR`：不兼容的大架构变化或破坏性升级。
- `MINOR`：新增功能模块或重要能力，例如 WebSSH、RBAC enforce。
- `PATCH`：Bug 修复、UI 小优化、兼容性修复。

示例：

- `v0.2.0`：新增 WebSSH、RBAC enforce、顶部导航、巡检报告等重要能力。
- `v0.2.1`：修复 WebSSH 输入焦点、导航下拉、上传弹窗样式等问题。
- `v0.3.0`：新增 Web IDE 或 AI4S 工具模块。

## 每次发版前检查清单

### 1. 工作区检查

```bash
git status --short
git diff --stat
```

确认没有误提交：

- `.env`
- 密码、token、webhook
- SSH key
- 数据库 dump
- 运行日志
- 构建二进制
- `.DS_Store`
- 临时截图和 Playwright 缓存

### 2. 测试

前端：

```bash
node --test tests/*.test.js
```

后端：

```bash
cd backend
go test ./...
```

关键浏览器验收：

- 登录页。
- 仪表盘。
- 队列状态。
- 数据目录。
- 作业模板。
- 作业列表。
- VNC 桌面。
- 终端中心。
- 角色管理。
- 系统设置。

权限验收：

- 普通用户菜单和数据范围。
- 普通用户管理员接口 403。
- 文件目录越权 403。
- WebSSH 登录节点限制。
- 中间管理员只读角色权限。
- `cluster_admin` 管理权限。

### 3. 更新文档

每次版本提交前至少更新：

- `CHANGELOG.md`
- `README.md`（如新增功能、部署方式或依赖变化）
- 相关 `docs/` 设计或部署文档

## 推荐 Git 工作流

### 创建功能分支

```bash
git switch -c codex/feature-name
```

### 提交

```bash
git add .
git commit -m "feat: add webssh terminal center"
```

提交信息建议：

- `feat:` 新功能
- `fix:` 修复
- `docs:` 文档
- `test:` 测试
- `refactor:` 重构
- `chore:` 工程化、依赖、脚本

### 推送

```bash
git push -u origin codex/feature-name
```

### 合并到主分支

确认测试通过后合并到 `main` 或 `master`：

```bash
git switch main
git merge --no-ff codex/feature-name
git push origin main
```

## 打标签

确认版本可发布后：

```bash
git tag -a v0.2.0 -m "SimpleHPC v0.2.0"
git push origin v0.2.0
```

## 测试服务器发布流程

1. 备份测试服务器代码、配置和数据库。
2. 上传新构建或同步静态资源。
3. 执行数据库迁移。
4. 重启后端服务。
5. 检查服务状态。
6. 执行关键回归。
7. 记录后端二进制 SHA256、env 备份路径、回滚命令。

## 生产发布原则

生产发布必须单独评审，不直接沿用测试服务器操作。

生产切换方案至少包含：

- 切换窗口。
- 备份路径。
- env 变更。
- 数据库迁移 up/down。
- 切换命令。
- 回归清单。
- 回退 shadow 步骤。
- 回退 legacy 步骤。
- 旧二进制回滚方案。
- 责任人和确认人。
- 用户影响说明。

## 回滚原则

优先应用级回滚：

```bash
sed -i 's/^RBAC_MODE=.*/RBAC_MODE=shadow/' /etc/simplehpc-backend.env
systemctl restart simplehpc-backend
```

如仍异常，再回退：

```bash
sed -i 's/^RBAC_MODE=.*/RBAC_MODE=legacy/' /etc/simplehpc-backend.env
systemctl restart simplehpc-backend
```

必要时再恢复旧二进制和旧静态文件。

数据库 down 迁移只作为最后手段，执行前必须确认数据影响。

