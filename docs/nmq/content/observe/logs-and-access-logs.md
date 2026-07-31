---
title: "NCP Logs"
description: "NCP 当前日志输出、日志目录与相关接口。"
---

# Logs & Access Logs

NCP 没有单独的 access log 模块命名，但 `dpproxy` 基于 Gin，调试模式下会开启 Gin Logger，同时业务和系统日志统一走 Zap。

## 日志初始化

`cmd/ncp/ncp.go` 中启动时创建了滚动日志器，参数包括：

- 单文件大小：`50MB`
- 备份数：`2`
- 保留天数：`30`
- 压缩：开启

## 典型日志来源

- 根运行时启动/停止日志
- 配置解析与 reload 日志
- 登录、验证码、部署、升级、包管理日志
- 微平台、Kubernetes、Helm、SQLite 操作日志

## 日志相关 API

`plugins/dpproxy/httpapi/handler_log_mng.go` 提供了大量日志管理接口，例如：

- 当前日志读取
- 历史日志下载
- FTP 日志
- HBP 日志
- 其他系统日志

## LLF 配置

`manifest/dpproxy/ncp.yaml` 中 `llf` 段定义了：

- 当前日志目录
- 历史日志目录
- FTP 与 HBP 目录模板
- 文件拷贝、压缩、SCP、tail 命令模板

这说明仓库已经把日志搜集视为平台内建能力，而不是外部脚本。
