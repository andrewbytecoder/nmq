---
title: "Expose NCP Services"
description: "NCP 当前有哪些对外可访问入口，以及它们各自承担什么职责。"
---

# Expose

NCP 默认同时暴露三类入口：HTTP API、HTTPS API 和独立 WebUI。

## 入口一览

| 入口 | 默认端口 | 说明 |
| --- | --- | --- |
| HTTP API | `11090` | `dpproxy` 主服务，含公开与私有 API |
| HTTPS API | `11091` | 与 HTTP API 同能力，通过 `RunTLS` 启动 |
| WebUI | `11092` | `plugins/webui` 的控制台与只读管理接口 |

## 公开路由

在 `plugins/dpproxy/httpapi/routers.go` 中，默认公开的能力包括：

- `GET /health`
- `GET /data`
- `POST /-/reload`
- `GET /captcha`
- `POST /login`

## 受保护路由

私有路由会挂载 `middlejwt.JwtAuth()` 中间件，典型前缀包括：

- `/dp/api/deployMng`
- `/dp/api/pkgMng`
- `/dp/api/roleManage`
- `/dp/api/businessConfig`
- `/dp/api/top`
- `/dp/api/upgrade`

## WebUI 入口

`plugins/webui` 默认提供：

- `GET /dashboard/`
- `GET /api/overview`
- `GET /api/handlers/routers`
- `GET /api/storage/sqlite`

因此，NCP 暴露的不只是后端 API，也包含一个适合运维与排障的轻量控制台。
