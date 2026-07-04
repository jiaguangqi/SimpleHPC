# RBAC 批次二 API 路由权限映射

状态：shadow 验证  
统计：135 条 `/api/v1` 路由（GET 49、POST 53、PUT 23、DELETE 9、Any 1）

## 映射规则

每条路由映射为：

```text
api.{resource}.{action}
```

动作根据 HTTP 方法和业务后缀确定：

- GET 列表：`list`
- GET 参数详情：`view`
- POST：`create`
- PUT/PATCH：`update`
- DELETE：`delete`
- `/cancel`：`cancel`
- `/suspend`：`suspend`
- `/resume`：`resume`
- `/publish`、`/unpublish`：`publish`
- `/approve`、`/reject`：`review`
- `/test`：`test`
- `/refresh`：`refresh`

## 路由资源映射

| 路由范围 | 权限资源 |
|---|---|
| `/auth/*` | `api.auth.*` |
| `/overview`、`/dashboard` | `api.dashboard.*` |
| `/account/roles*`、`/rbac/*` | `api.roles.*` |
| `/account/users*`、`/ldap/*` | `api.users.*` |
| `/account/admins*` | `api.admins.*` |
| `/account/teams*` | `api.teams.*` |
| `/account/units*` | `api.units.*` |
| `/account/sync-ldap` | `api.accounts.*` |
| `/slurm/jobs*` | `api.jobs.*` |
| `/slurm/queue-status` | `api.queue.*` |
| `/slurm/nodes` | `api.nodes.*` |
| `/slurm/qos*` | `api.qos.*` |
| `/slurm/partition*` | `api.partitions.*` |
| `/storage/list`、重命名及其他文件操作 | `api.storage.files.*` |
| `/storage/acls*` | `api.storage.acls.*` |
| `/storage/roots*` | `api.storage.roots.*` |
| `/job-template*` | `api.templates.*` |
| `/inspection*` | `api.inspection.*` |
| `/monitoring*` | `api.monitoring.*` |
| `/logs/*`、`/audit/*` | `api.logs.*` |
| `/config/{module}*` | `api.config.{module}.*` |

## 公共白名单

以下路由不要求登录后的 RBAC 权限：

- `/api/health`
- `/api/v1/auth/login`
- `/api/v1/auth/password-reset/request`
- `/api/v1/auth/password-reset/confirm`
- `/api/v1/config/platform/public`
- 作业模板回调注册与 token 网关

## legacy/shadow 管理接口安全边界

在进入 enforce 前，下列管理接口继续使用管理员身份边界，普通用户直接返回 403：

- 账户、角色和 RBAC 管理。
- LDAP 和平台配置。
- 日志、审计、监控和巡检。
- 存储根和访问授权。
- 分区配置及 QOS 修改。

shadow 会记录旧身份边界与新 RBAC 判定的匹配或差异，但不会根据新 RBAC 判定放行业务请求。
