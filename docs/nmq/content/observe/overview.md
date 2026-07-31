---
title: "Observe NCP"
description: "NCP 已内置的日志、指标和 profiling 能力概览。"
---

# Observe

NCP 当前的可观测性集中在两块：统一日志和内置 Prometheus 指标。

## 已有能力

- 启动日志、错误日志、组件日志
- Gin 请求指标
- Backend 请求、重试、健康状态指标
- `/metrics` 与 `/dp/api/v1/metrics`
- pprof profiling 路由

## 实现位置

- 日志：`cmd/ncp/ncp.go`, `pkg/utils`
- 指标：`internal/metrics`
- 指标挂载：`plugins/ncp/ncp.go`, `plugins/dpproxy/httpapi/routers.go`
- profiling：`plugins/dpproxy/httpapi/routers.go`

## 为什么这很重要

NCP 同时承担 API 入口、部署流程和存储管理。任何一个环节的阻塞都会影响控制面，所以最基础的日志与指标已经被做成平台默认能力，而不是某个业务组件的可选项。
