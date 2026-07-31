---
title: "NCP Configuration Options"
description: "NCP 当前配置结构体中的关键选项。"
---

# Configuration Options

下面列出当前 `internal/config.Config` 中最关键的一组配置项。

| 配置段 | 关键字段 | 说明 |
| --- | --- | --- |
| `deploy_mode` | `0/1/...` | 部署模式 |
| `runtime_env` | `docker` / `k8s` | 运行环境 |
| `address` | IP/hostname | 当前节点地址 |
| `server.http` | `address`, `port` | HTTP 监听 |
| `server.https` | `address`, `port` | HTTPS 监听 |
| `server.webui` | `address`, `port` | WebUI 监听 |
| `dp` | `address`, `port` | DP 后端地址 |
| `inspect` | `address`, `port` | Inspect 地址 |
| `etcd` | `address`, `port` | etcd 地址 |
| `captcha` | 长度、图片尺寸、开启策略 | 登录验证码策略 |
| `jwt` | `signing_key`, `expires_time`, `buffer_time`, `issuer` | JWT 行为 |
| `database` | `type`, `path`, `host`, `port`, `name` | 数据库配置 |
| `pkg` | 目录类参数 | 包、清单、chart 路径 |
| `gofd_client` | 重试、超时、分片 | 文件传输策略 |
| `micro_plat` | HTTP/HTTPS 端口 | 微平台服务管理器 |
| `llf` | 日志目录和命令模板 | 运维日志能力 |

## 相关命令行覆盖

以下内容可由 CLI 覆盖或影响：

- 配置文件名
- 工作目录
- 证书目录
- metrics 开关和前缀

## 相关环境补全

`internal/env` 目前至少补齐以下值：

- `dp.proxy.http.listen.port`
- `dp.proxy.https.listen.port`
- `external.k8s.type`
