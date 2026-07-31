---
title: "NCP Documentation"
description: "NCP 是一个基于 Go 的组件化微服务运行与部署平台，内置组件管理、API 代理、WebUI、存储、Helm 与观测能力。"
---

# What is NCP?

NCP 是平台组自研的 Go 微服务管理平台。它把原先部署管理系统拆成 `ncp`、`dpcore`、`dpproxy`、`network`、`webui` 等组件，由 `cmd/ncp` 统一注册并启动，功能包括配置解析、组件生命周期编排、HTTP/HTTPS 服务暴露、SQLite 存储、Helm chart 渲染、服务部署以及 Web 管理控制台。

- `ncp`: 组件注册管理组件。
- `dpcore`: 存储、部署器、Helm、微平台客户端。
- `dpproxy`: API 入口、业务控制面、登录与升级流程。
- `network`: HTTP 客户端等基础网络能力。
- `webui`: Dashboard、路由视图、SQLite 只读接口。

## 平台定位

- `plugins/ncp` 提供组件容器、CLI 根命令、配置装载、日志与指标注册。
- `plugins/dpcore` 提供存储、部署器、Helm 客户端和微平台客户端。
- `plugins/dpproxy` 提供实际的业务 API、登录认证、部署管理、拓扑管理、升级与日志能力。
- `plugins/webui` 暴露独立的 Dashboard 与 SQLite/路由查看接口。
- `plugins/network` 为其他组件提供统一的 HTTP 客户端能力。

## 你可以在这里找到什么

- 如果你要快速跑起来，从 [Getting Started](getting-started/index.md) 开始。
- 如果你要理解配置模型，看 [Configuration Introduction](getting-started/configuration-overview.md)。
- 如果你要接手部署链路，看 [Deployment Workflow](operations/deployment-workflow.md)。
- 如果你要扩展平台能力，看 [Extend NCP](extend/extend-ncp.md) 和 [Register a Component](user-guides/register-component.md)。

## 运行时概览

配置和运行资产在当前仓库中分成四层：

1. **命令入口层**: `cmd/ncp` 负责注册组件并启动 CLI。
2. **运行时层**: `plugins/ncp` 负责配置、日志、指标和生命周期。
3. **业务组件层**: `plugins/dpcore`、`plugins/dpproxy`、`plugins/webui`、`plugins/network`。
4. **资产层**: `manifest/`、`docs/概要设计/`、`data/`、`certs/`、`packages/`。

1. 进程入口在 `cmd/ncp/ncp.go`。
2. `ncp.NewNcp(...)` 创建根组件并注册 CLI 持久化参数。
3. `RegisterComponents(...)` 注入 `dpproxy`、`network`、`dpcore`、`webui`。
4. `PersistentPreRunE` 中完成配置解析、指标注册和组件初始化。
5. `dpproxy` 启动 HTTP/HTTPS API，`webui` 启动独立控制台，`dpcore` 接管 SQLite/Helm/部署逻辑。

## 当前仓库默认工作模式

仓库自带的 `manifest/dpproxy/ncp.yaml` 显示，当前默认样例更偏向单实例或小型化环境：

- `runtime_env: docker`
- `server.http.port: 11090`
- `server.https.port: 11091`
- `server.webui.port: 11092`
- `database.type: sqlite`
- `database.path: data/dp.db`

这意味着 NCP 既可以作为本地开发态控制面运行，也可以作为管理打包、部署和升级流程的长期服务运行。
