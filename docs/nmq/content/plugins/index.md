---
title: "NCP Plugins Overview"
description: "NCP 当前注册的五个核心插件：ncp、dpproxy、dpcore、network、webui。"
---

# Plugins

NCP 当前运行时不是把所有能力塞进一个进程主函数里，而是通过插件式组件模型组织。`cmd/ncp/ncp.go` 里最终注册了 5 个核心插件：

- `ncp`
- `dpproxy`
- `dpcore`
- `network`
- `webui`

## 注册顺序

`RegisterComponents(...)` 的注册顺序如下：

1. `dpproxy`
2. `network`
3. `dpcore`
4. `webui`

`ncp` 本身在 `plugins/ncp.NewNcp(...)` 中会先把自己注册进组件表，所以它是所有其他插件的宿主和调度器。

## 插件模型

所有插件都遵循同一套组件接口：

- `GetInterface(uuid string) any`
- `Init() error`
- `Start() error`
- `Stop() error`
- `Reset() error`
- `GetName() string`
- `GetVersion() string`
- `Notify(event string, data any)`
- `GetStatus() ComponentStatus`

## 运行时关系

| 插件 | 主要职责 | 依赖的上游能力 | 暴露给其他插件的能力 |
| --- | --- | --- | --- |
| `ncp` | 组件容器、CLI、配置、日志、协程池、指标注册 | 无 | `ncp-metrics-registry` 等运行时基础能力 |
| `dpproxy` | HTTP/HTTPS API、Gin 路由、JWT、部署/升级/日志/包管理接口 | `dpcore`、`network`、`ncp` | `dpproxy-router` |
| `dpcore` | SQLite 存储、部署器、Helm client、微平台 client | `ncp`、配置 | `dp-storage`、`dp-deployer`、`dp-helmclient`、`dp-microclient` 等 |
| `network` | HTTP client 封装 | `ncp` logger | `network-http-client` |
| `webui` | 独立 Dashboard、SQLite 浏览、路由只读视图 | `dpproxy-router`、`dpcore` storage | 当前不额外暴露能力 |

## 推荐阅读顺序

- 先看 [ncp](ncp.md)，理解宿主插件如何管理生命周期。
- 再看 [dpcore](dpcore.md)，理解存储和部署能力。
- 然后看 [dpproxy](dpproxy.md)，理解业务 API 是怎么拼起来的。
- 最后看 [network](network.md) 和 [webui](webui.md) 这两个辅助型插件。
