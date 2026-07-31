---
title: "NCP Component Interfaces"
description: "NCP 组件接口、能力名和依赖注入方式。"
---

# Component Interfaces

NCP 的核心接口定义在 `interfaces/ncp` 和 `interfaces/componet_name.go`。

## 组件接口

每个组件都需要实现：

```go
type Component interface {
    GetInterface(uuid string) any
    Init() error
    Start() error
    Stop() error
    Reset() error
    GetName() string
    GetVersion() string
    Notify(event string, data any)
    GetStatus() ComponentStatus
}
```

## 上下文接口

组件可通过 `Context` 获取：

- 全局 `context.Context`
- 全局 logger
- `ComponentManager`
- 其他组件暴露的 capability
- 工作目录、证书目录和配置文件名

## 当前能力名

`interfaces/componet_name.go` 中当前最关键的能力名包括：

- `ncp-metrics-registry`
- `network-http-client`
- `dpproxy-router`
- `dp-storage`
- `dp-deployer`
- `dp-helmclient`
- `dp-microclient`

## 使用建议

跨组件协作时优先通过 `GetInterface(...)` 获取能力，而不是直接依赖具体实现包。
