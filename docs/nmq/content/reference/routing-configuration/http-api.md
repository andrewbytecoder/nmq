---
title: "NCP HTTP API Surface"
description: "NCP 当前仓库里可见的 HTTP API 分组。"
---

# HTTP API Surface

`plugins/dpproxy/httpapi/routers.go` 定义了当前最主要的 API 面。

## 公开接口

- `GET /health`
- `GET /data`
- `POST /-/reload`
- `GET /captcha`
- `POST /login`

## 私有接口分组

| 分组前缀 | 主要职责 |
| --- | --- |
| `/dp/api/sys` | 公共系统信息 |
| `/dp/api/db` | 数据查询 |
| `/dp/api/cdwgui` | 服务管理相关接口 |
| `/dp/api/deployMng` | 部署管理 |
| `/dp/api/init` | 初始化流程 |
| `/dp/api/logMng` | 日志管理 |
| `/dp/api/pkgMng` | 包管理 |
| `/dp/api/roleManage` | 角色管理 |
| `/dp/api/service` | 服务查询 |
| `/dp/api/businessConfig` | 配置管理 |
| `/dp/api/user` | 用户管理 |
| `/dp/api/top` | 拓扑管理 |
| `/dp/api/upgrade` | 升级与回滚 |

## 指标与调试接口

- `GET /metrics`
- `GET /dp/api/v1/metrics`
- pprof 默认路由

## WebUI API

`plugins/webui` 还提供了单独的 API 集合，例如：

- `/api/overview`
- `/api/handlers/routers`
- `/api/storage/sqlite`
- `/api/fileview/*`
