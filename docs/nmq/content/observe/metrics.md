---
title: "NCP Metrics"
description: "NCP 基于 internal/metrics 提供的 Prometheus 指标能力。"
---

# Metrics

`internal/metrics` 是仓库里已经沉淀出来的一套可复用 Prometheus 注册中心。

## 默认端点

- `GET /metrics`
- `GET /dp/api/v1/metrics`

## 默认指标能力

- HTTP 请求总数
- 请求时延
- 请求/响应字节数
- 后端请求总数与时延
- 重试次数
- 后端健康状态
- TLS 相关指标

## 启动过程

1. `plugins/ncp` 根据命令行参数创建 registry。
2. registry 通过组件接口暴露为 `ncp-metrics-registry`。
3. `dpproxy` 在 Gin 中间件中挂接请求观测。
4. 所有路由注册完成后执行 `SyncGinRoutes(...)`。

## 可配置开关

根命令支持以下参数：

- `--register.metrics`
- `--register.process.metrics`
- `--register.go.metrics`
- `--register.server.metrics`
- `--register.prefix.metrics`

## 额外能力

除了内置指标，这套 registry 还支持在运行时注册自定义 collector，因此如果后续新增组件，也可以沿用同一 registry 输出平台级指标。
