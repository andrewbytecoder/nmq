---
title: "Plugin: webui"
description: "NCP 的独立控制台插件，负责 Dashboard、SQLite 表查看和路由只读视图。"
---

# Plugin `webui`

代码入口：`plugins/webui/web_component.go`

`webui` 与 `dpproxy` 的关系很像“控制台”对“主 API 服务”的关系。它不复用 `dpproxy` 的监听地址，而是单独起一个 Web 管理入口。

## 核心职责

### 1. 获取路由管理器

`Init()` 中它会从 capability 总线取：

- `dpproxy-router`

然后塞进自己的上下文：

```go
c.ctx.SetRouter(r)
```

这说明它并不自己管理主业务路由，而是只读消费 `dpproxy` 暴露出来的路由信息。

### 2. 启动独立 Dashboard 服务

`Start()` 中会异步调用 `run()`，而 `run()` 会：

- 读取 `config.GetWebHttpAddress()`
- 创建 `api.NewServer(...)`
- 独立监听 WebUI 地址

默认配置里这个端口是：

- `server.webui.port = 11092`

### 3. 暴露 Dashboard 与 API

`plugins/webui/api/handler_dashboard.go` 可以看到它注册了：

- `/dashboard`
- `/api/rawdata`
- `/api/overview`
- `/api/entrypoints`
- `/api/fileview/*`
- `/api/handlers/routers`
- `/api/http/*`
- `/api/tcp/*`
- `/api/udp/*`
- `/api/certificates`
- `/api/version`
- `/api/storage/sqlite`

### 4. 提供 SQLite 浏览能力

`api.NewServer(...)` 时注入了：

- `storage.NewSQLiteTableProvider(...)`

这意味着 WebUI 可以直接查看 `dpcore` 暴露出来的 SQLite 表摘要和表内容。

### 5. 提供路由只读能力

同时注入了：

- `routeinfoprovider.New(c.ctx.GetRouter())`

因此 WebUI 可以把 `dpproxy` 的 HTTP 路由能力转成前端可展示的数据。

### 6. 嵌入式前端静态资源

`handler_dashboard.go` 中使用：

- `webui.FS`

说明 Dashboard 前端资源已经作为嵌入式静态文件打进插件里，而不是依赖外部单独部署。

## 这个插件和 dpproxy 的边界

### `dpproxy`

- 负责真正的业务 API
- 负责登录 / JWT / 部署 / 升级 / 日志

### `webui`

- 负责可视化控制台
- 负责 SQLite 浏览
- 负责路由与资源总览
- 负责一部分 mock/只读展示 API

## 最重要的能力总结

可以把 `webui` 看成 NCP 的：

- 独立管理控制台
- 只读运维视图层
- Dashboard API 层
