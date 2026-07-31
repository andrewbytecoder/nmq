---
title: "NCP Routing Overview"
description: "NCP 内部的路由不止是 HTTP 路由，也包括组件 capability 路由。"
---

# Routing

在 NCP 里，“routing” 有两层含义。

## 1. HTTP 路由

由 `dpproxy` 基于 Gin 管理：

- 公开路由
- 私有路由
- metrics 路由
- pprof 路由

## 2. 组件能力路由

由 `plugins/ncp` 负责：

- 组件注册到 `components` map
- `GetInterface(uuid)` 遍历组件并返回 capability
- `Notify(event, data)` 向所有组件广播事件

## 这两层如何配合

- `webui` 通过 `dpproxy-router` capability 读取路由信息
- `dpproxy` 通过 `dp-storage`、`dp-deployer`、`network-http-client` 等 capability 获取后端能力
- `ncp` 本身不处理业务请求，但负责把路由所需能力组织起来

## 结论

NCP 当前的路由模型本质上是“HTTP 路由 + 组件能力路由”双层结构，这也是它能同时充当运行时容器和业务控制面的原因。
