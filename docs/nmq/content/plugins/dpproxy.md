---
title: "Plugin: dpproxy"
description: "NCP 的主业务 API 插件，负责 HTTP/HTTPS 服务、Gin 路由、JWT 鉴权、部署与运维接口。"
---

# Plugin `dpproxy`

代码入口：`plugins/dpproxy/dpproxy_component.go`

`dpproxy` 是 NCP 当前最重的业务插件，真正对外提供管理 API 的就是它。

## 核心职责

### 1. 注册业务子命令

构造时它会向 `ncp` 根命令追加：

```go
Use:   "dpproxy"
Short: "dpproxy is a web api proxy tools"
RunE:  dpProxy.Run
```

所以最终用户运行的实际命令通常是：

```bash
ncp dpproxy
```

### 2. 初始化业务上下文和 Gin 路由管理器

`Init()` 中完成：

- 创建 `plugins/dpproxy/ctx.Context`
- 创建 IDC 缓存
- 解析环境变量配置
- 加载 cluster config
- 创建 `RouterMng`

并在 `GetInterface(...)` 里暴露：

- `dpproxy-router`

### 3. 装配业务依赖

`Start()` 中它会从 `ncp` 能力总线上获取：

- `dp-deployer`
- `dp-storage`
- `dp-idcinfo-storage`
- `dp-productinfo-storage`
- `dp-deployipinfo-storage`
- `dp-certinfo-storage`
- `dp-topoinfo-manager`
- `dp-operate-log-storage`
- `dp-servicegroupmng-storage`
- `dp-configdata-storage`
- `dp-helmclient`
- `dp-microclient`
- `ncp-metrics-registry`
- `network-http-client`

然后把这些能力装配成自己的：

- storage facade
- data panel
- kube client
- gin API router

### 4. 启动 HTTP / HTTPS 双入口

`Run(...)` 中最核心的动作是：

- 创建 GinRouter
- 读取配置
- 准备证书路径
- goroutine 启动 HTTPS
- 主流程启动 HTTP

对应默认监听：

- HTTP：`server.http`
- HTTPS：`server.https`

### 5. 暴露主业务 API

`plugins/dpproxy/httpapi/routers.go` 里可以看到它组织了大量路由组：

- `/health`
- `/data`
- `/-/reload`
- `/captcha`
- `/login`
- `/dp/api/sys`
- `/dp/api/db`
- `/dp/api/cdwgui`
- `/dp/api/deployMng`
- `/dp/api/init`
- `/dp/api/logMng`
- `/dp/api/pkgMng`
- `/dp/api/roleManage`
- `/dp/api/service`
- `/dp/api/businessConfig`
- `/dp/api/user`
- `/dp/api/top`
- `/dp/api/upgrade`

### 6. 集成观测与调试能力

它会自动接上：

- `pprof`
- `/metrics`
- `/dp/api/v1/metrics`
- Gin metrics middleware

### 7. 承担鉴权入口

私有路由统一挂：

```go
ginRouter.privateRouter.Use(middlejwt.JwtAuth())
```

说明登录、JWT 和权限控制都是这个插件的职责范围。

## 从代码结构看它包含什么

`plugins/dpproxy` 目录下包含：

- `httpapi`: 路由与控制器
- `middleware`: JWT 等中间件
- `cache`: IDC 等缓存
- `datapanel`: 业务数据聚合面板
- `models`: 存储 facade
- `client`: API client
- `components`: 路由管理组件等
- `service`: 服务逻辑
- `util`: 工具逻辑

## 最重要的能力总结

可以把 `dpproxy` 看成 NCP 的：

- API Gateway 风格控制面
- 运维接口入口
- 部署/升级/日志/包管理聚合器
- Gin Web runtime
