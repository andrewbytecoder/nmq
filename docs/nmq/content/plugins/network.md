---
title: "Plugin: network"
description: "NCP 的基础网络插件，当前主要暴露统一 HTTP client 能力。"
---

# Plugin `network`

代码入口：`plugins/network/network_component.go`

`network` 是五个插件里最轻量的一个，但它承担着很典型的“基础设施插件”角色。

## 核心职责

### 1. 创建统一 HTTP client

构造时直接初始化：

```go
httpclient.NewHttpClient(ctx.GetLogger())
```

也就是说这个插件本身几乎就是一个对 `pkg/httpclient` 的运行时包装层。

### 2. 暴露网络能力给其他插件

`GetInterface(...)` 只做一件事：

- 当请求 `network-http-client` 时，返回 `httpClient`

这让其他插件不需要直接依赖具体 HTTP client 实现包，只需要通过 capability 名获取即可。

### 3. 保持生命周期简单

它的：

- `Init()`
- `Start()`
- `Stop()`
- `Reset()`

当前都没有额外逻辑，说明这个插件目前是一个纯能力提供者，而不是带有复杂后台任务的运行单元。

## 为什么它依然值得单独成插件

虽然代码量小，但单独抽成插件有几个好处：

- 统一管理 HTTP client 生命周期
- 避免 `dpproxy` / `webui` / `dpcore` 直接依赖具体网络实现
- 后续如果要扩展重试、代理、TLS 策略、Tracing 或 mock client，会更容易演进

## 最重要的能力总结

`network` 目前就是：

- 一个基础网络适配器
- 一个 capability provider
- 一个未来可扩展的统一出站通信层
