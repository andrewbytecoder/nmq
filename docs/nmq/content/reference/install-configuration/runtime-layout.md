---
title: "NCP Runtime Layout"
description: "NCP 仓库结构与运行时职责分层。"
---

# Runtime Layout

## 顶层目录职责

| 目录 | 作用 |
| --- | --- |
| `cmd/` | 程序入口 |
| `plugins/` | 平台组件实现 |
| `interfaces/` | 运行时契约与能力标识 |
| `internal/` | 内部配置、指标、环境信息 |
| `pkg/` | 复用型基础库 |
| `manifest/` | 运行资产、chart、包、样例配置 |
| `docs/概要设计/` | 架构设计文档 |
| `version/` | 版本信息 |

## 组件运行关系

- `cmd/ncp` 调用 `plugins/ncp`
- `plugins/ncp` 注册并启动其他组件
- `plugins/dpcore` 向其他组件暴露存储与部署能力
- `plugins/dpproxy` 通过接口获取部署、存储、网络和指标能力
- `plugins/webui` 通过 `dpproxy` 路由管理器和 SQLite Provider 构造控制台

## 为什么这套布局重要

它决定了你在改代码时应该把内容放在哪里：

- 公共抽象进 `interfaces/`
- 复用工具进 `pkg/`
- 平台实现进 `plugins/`
- 仅内部使用的配置和观测实现进 `internal/`
