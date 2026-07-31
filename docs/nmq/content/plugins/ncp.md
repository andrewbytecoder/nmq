---
title: "Plugin: ncp"
description: "NCP 宿主插件，负责组件容器、CLI、配置解析、日志、指标和生命周期编排。"
---

# Plugin `ncp`

代码入口：`plugins/ncp/ncp.go`

`ncp` 不是一个普通业务插件，而是整个插件体系的宿主。其他组件都运行在它创建的上下文和生命周期里。

## 核心职责

### 1. 创建组件容器

`NewNcp(...)` 会初始化：

- `components map[string]ncp.Component`
- `logger`
- `context.Context` 与 `cancel`
- Cobra 根命令 `rootCmd`
- `metrics.Registry`
- 协程池 `ants.Pool`

并且在构造阶段就执行：

```go
n.RegisterComponent(n.GetName(), n)
```

这意味着 `ncp` 自己也以组件身份存在于统一注册表中。

### 2. 提供 CLI 根命令

根命令为：

```go
Use:   "ncp"
Short: "NCP is a component manager"
```

并注册了持久化参数：

- `--config.file`
- `--cert.path`
- `--work.dir`
- `--remoter.work.dir`
- `--register.prefix.metrics`
- `--register.metrics`
- `--register.process.metrics`
- `--register.go.metrics`
- `--register.server.metrics`

### 3. 驱动统一生命周期

`PersistentPreRunE` 中：

1. 调用 `Init()`
2. 打日志 `Starting NCP`
3. 调用 `Start()`

`PersistentPostRunE` 中：

1. 调用 `Stop()`
2. 释放协程池
3. 调用 `Reset()`
4. 打日志 `Exit NCP`

### 4. 配置解析与运行时初始化

`Init()` 主要做这些事：

- 绑定 `config.file` 到 viper
- 注入 `work.dir` 和当前目录为配置查找路径
- 设置 `yaml` 配置类型
- 调用 `internal/config.ParseConfig()`
- 初始化 metrics registry
- 逐个初始化其他已注册插件

### 5. 指标注册中心

`SetUpMetricsHandler()` 根据 CLI 参数创建 `internal/metrics.Registry`，控制：

- process metrics
- Go runtime metrics
- server metrics

并通过 `GetInterface(...)` 暴露为：

- `ncp-metrics-registry`

### 6. 组件发现和能力路由

`GetInterface(uuid string)` 的逻辑是：

1. 先处理 `ncp` 自己的内建能力
2. 再遍历所有其他插件
3. 依次调用它们的 `GetInterface(...)`
4. 找到第一个非 `nil` 的能力即返回

这就是 NCP 插件间解耦的关键机制。

## 对外暴露的实际价值

从代码实现看，`ncp` 插件提供的不是业务功能，而是：

- 宿主上下文
- 组件注册表
- 生命周期调度
- 指标总线
- 统一日志
- 协程池提交入口 `Submit(...)`

没有这个插件，其余 4 个插件就只是离散模块，不能形成一个完整运行时。

## 适合怎么理解它

可以把 `ncp` 理解成：

- 一个轻量插件容器
- 一个 CLI 入口
- 一个运行时编排器
- 一个统一 capability router
