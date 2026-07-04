# User Storage Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 普通用户所有文件操作只能发生在每个授权根目录的 `{root}/{username}` 下，管理员保留根目录管理能力。

**Architecture:** 后端根据认证用户构造独立 storage client；普通用户 client 的根目录为四个用户专属目录，管理员 client 使用配置根。所有接口统一从同一 helper 获取 client，前端只消费后端返回的 effectivePath。

**Security:** 路径清理、符号链接检查、跨根目录拒绝、文件名校验和服务端授权同时生效。

---

- [x] 测试用户根映射、用户名校验、目录权限和属主修复。
- [x] 测试根目录、其他用户目录、`..` 和符号链接越权被拒绝。
- [x] 所有文件接口接入认证用户作用域。
- [x] 普通用户专属目录自动创建为 0700 并设置 UID/GID。
- [x] 存储入口返回逻辑路径与 effectivePath。
- [x] 前端按 effectivePath 进入并限制上一级按钮。
- [x] 用 user001 验证四个根目录和所有操作。
- [x] 用管理员验证根目录管理不受影响。
