# Log Center Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 增加真实的用户登录日志、系统日志和页面操作审计中心。

**Architecture:** 登录事件和审计记录持久化到 PostgreSQL；系统日志按固定白名单实时读取 journald 或容器日志；公共前端脚本统一注入日志中心导航。

**Tech Stack:** Go、Gin、PostgreSQL、systemd journal、Docker logs、原生 JavaScript。

---

### Task 1: 用户登录事件

- [x] 编写登录事件规范化与查询测试。
- [x] 创建 `auth_events` 表和服务方法。
- [x] 登录成功、失败和退出时记录事件。
- [x] 增加管理员查询接口。

### Task 2: 系统日志

- [x] 编写来源白名单和日志解析测试。
- [x] 实现 journalctl/docker logs 固定命令。
- [x] 增加管理员查询接口和限制。

### Task 3: 通用审计

- [x] 编写写请求审计动作命名测试。
- [x] 实现 Gin 写操作审计中间件。
- [x] 排除登录、退出和只读请求。

### Task 4: 日志页面与导航

- [x] 新增用户登录日志页面。
- [x] 新增系统日志页面。
- [x] 复用审计日志筛选和真实数据列表。
- [x] 所有页面动态注入日志中心菜单。

### Task 5: 部署验收

- [x] 运行 Go 与 JavaScript 测试。
- [x] 同步完整源码和前端文件。
- [x] 验证登录、退出、系统日志和写操作审计。
- [x] 验证普通用户访问返回 403。
