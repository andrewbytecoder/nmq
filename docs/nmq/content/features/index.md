---
title: "NCP Features"
description: "基于仓库当前实现整理的 NCP 功能矩阵。"
---

# NCP Features

NCP 的能力主要来自五个已注册组件和若干基础包。下表按仓库当前实现归纳平台功能。

## Feature Matrix

| 能力域 | 当前实现 | 代码来源 |
| --- | --- | --- |
| 组件生命周期 | `Init` / `Start` / `Stop` / `Reset` / `Notify` | `interfaces/ncp`, `plugins/ncp` |
| CLI | 基于 Cobra 的根命令与子命令体系 | `plugins/ncp/ncp.go`, `plugins/dpproxy/dpproxy_component.go` |
| 配置装载 | Viper + YAML + 环境变量补全 | `internal/config`, `internal/env` |
| 日志 | Zap + Lumberjack 风格滚动日志 | `pkg/utils`, `cmd/ncp/ncp.go` |
| HTTP API | Gin 路由、公开与私有路由分组 | `plugins/dpproxy/httpapi` |
| HTTPS | 通过 `server.crt/server.key` 启动 TLS 监听 | `plugins/dpproxy/dpproxy_component.go` |
| 指标 | Prometheus Registry、Gin Middleware、`/metrics` | `internal/metrics` |
| Profiling | pprof 自动注册 | `plugins/dpproxy/httpapi/routers.go` |
| 鉴权 | 验证码、JWT、登录态 Cookie | `plugins/dpproxy/httpapi/handler_login.go`, `plugins/dpproxy/middleware/middlejwt` |
| WebUI | `/dashboard` 和 `/api` 控制面 | `plugins/webui` |
| 存储 | 默认 SQLite，自动迁移 | `plugins/dpcore/storage` |
| 部署 | 部署器、拓扑、升级、包管理 | `plugins/dpproxy/httpapi`, `plugins/dpcore/deploy` |
| Helm | 封装 Helm v2/v3，当前 `dpcore` 默认使用 v3 | `pkg/helm`, `plugins/dpcore/helmclient` |
| Kubernetes | kube client、节点/拓扑/部署查询 | `pkg/kube`, `plugins/dpproxy/httpapi/deploy` |
| 微平台对接 | Service Manager Token 与资源接口 | `plugins/dpcore/microclient` |
| 包管理 | `packages/`、`tmp_pkg/`、分片上传与合并 | `plugins/dpproxy/httpapi/handler_pkgmng.go` |

## 仓库里最突出的几个特点

- 平台不是单纯的网关，而是“组件容器 + 部署控制面 + 运维 API”。
- `dpproxy` 是业务暴露最完整的组件，覆盖登录、部署、拓扑、升级、日志、角色与配置管理。
- `webui` 并不复用 `dpproxy` 监听端口，而是单独通过 `server.webui` 暴露控制台。
- `dpcore` 负责把 SQLite、Helm 和微平台客户端能力统一挂接到组件接口层。

## 适用场景

- 本地或实验环境下的控制面原型。
- 面向内部产品包、拓扑和部署流程的管理工具。
- 需要通过 Helm/Kubernetes 交付业务组件的微服务平台。
