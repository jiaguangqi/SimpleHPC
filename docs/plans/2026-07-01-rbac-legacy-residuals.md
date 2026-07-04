# RBAC 旧式鉴权残留基线

扫描规则：

```text
user.Type == / !=
requireAdmin(
user.Role ==
```

当前共 61 处，已由 `tests/legacy-auth-guard.test.js` 固化。出现新增、删除或移动时测试会失败，必须重新分类和评审。

| 文件 | 数量 | 保留原因 |
|---|---:|---|
| `httpapi/router.go` | 25 | 旧账户、作业、巡检和配置接口实际判定，shadow 期间作为对照结果 |
| `httpapi/rbac_admin.go` | 14 | RBAC 管理接口的 legacy 安全兜底；enforce 前防止旧模式放开角色管理 |
| `httpapi/rbac.go` | 2 | `legacySafetyBoundary` 对照层与角色变更 cluster_admin 硬边界 |
| `httpapi/terminal_config.go` | 1 | WebSSH 登录节点配置写入的 cluster_admin/config_admin 硬边界 |
| `httpapi/templates.go` | 2 | `requireAdmin` 兼容入口及管理员身份分类 |
| `httpapi/storage_access.go` | 3 | cluster_admin 旧兜底及普通用户文件边界兼容 |
| `httpapi/monitoring.go` | 2 | 监控模块 legacy 管理员判定 |
| `httpapi/logcenter.go` | 2 | 日志模块 legacy 管理员判定 |
| `httpapi/audit.go` | 2 | 审计与平台设置 legacy 管理员判定 |
| `httpapi/acl.go` | 2 | 存储授权 legacy 管理员判定 |
| `httpapi/platform_assets.go` | 1 | 平台资源上传 legacy 管理员判定 |
| `service/service.go` | 1 | 身份类型分类，不直接授予 RBAC 权限 |
| `service/templates.go` | 4 | 旧模板管理规则，shadow 数据范围对照 |

处理原则：

1. 批次五不删除这些判断，继续作为 shadow 的实际旧结果。
2. 新代码原则上不得增加旧式鉴权；确需作为安全硬边界保留时，必须在本文件分类说明并同步更新测试基线。
3. 进入 enforce 后按模块逐项移除业务处理器内的旧判断。
4. `user.Type` 仅作为账号来源分类时可以长期保留，但必须与授权判断分开。
