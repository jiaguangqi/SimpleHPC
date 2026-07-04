# Platform Branding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让平台名称和上传图片配置真实作用于全部页面。

**Architecture:** 后端提供安全公共配置和受控图片上传；前端公共脚本统一应用品牌配置。

**Tech Stack:** Go、Gin、PostgreSQL、PNG/JPEG、原生 JavaScript。

---

- [x] 测试语言固定和图片尺寸规则。
- [x] 实现公开配置与管理员上传接口。
- [x] 实现 `/uploads` 静态资源服务。
- [x] 改造设置页为文件上传和预览。
- [x] 全页面应用名称、Logo、标题、页脚和登录背景。
- [x] 服务器上传与持久化验收。
