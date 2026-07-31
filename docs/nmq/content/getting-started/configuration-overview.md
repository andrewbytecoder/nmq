---
title: "NCP Configuration Overview"
description: "NCP 的配置来源、覆盖顺序和核心配置段。"
---

# Configuration Introduction

NCP 的配置模型集中在 `internal/config`，默认样例位于 `manifest/dpproxy/ncp.yaml`。

## 配置覆盖关系

源码注释给出的目标优先级是：

```text
config file > command > env > default
```

在当前实现中，实际行为可以理解为：

1. `plugins/ncp` 先注册持久化参数，例如 `--config.file`、`--work.dir`、`--cert.path`。
2. `internal/config.ParseConfig()` 从 YAML 解析结构体。
3. 缺失的端口信息会尝试从 `internal/env` 中补齐。
4. `internal/config.init()` 为部分字段设置默认值。
5. Windows 下会把 `server.http.port` 和 `server.https.port` 强制调整为 `11090`/`11091`。

## 核心配置段

```yaml
deploy_mode: 1
runtime_env: "docker"
address: "10.168.8.95"

server:
  http:
    port: 11090
  https:
    port: 11091
  webui:
    port: 11092

jwt:
  signing_key: "new-dp-signature"
  expires_time: "24h"
  buffer_time: "10m"
  issuer: "ysp-auth-server"

database:
  type: "sqlite"
  path: "data/dp.db"
```

## 配置段职责

- `server`: 控制 HTTP、HTTPS、WebUI 三个监听入口。
- `dp` / `inspect` / `etcd`: 后端服务与外部依赖地址。
- `captcha`: 登录验证码策略。
- `jwt`: 令牌签发和刷新窗口。
- `database`: 当前默认使用 SQLite。
- `pkg`: 包目录、清单目录、Helm charts 目录等部署资产路径。
- `gofd_client`: 文件传输重试和超时策略。
- `micro_plat`: 微平台 Service Manager 端口。
- `llf`: 日志搜集目录和命令模板。

## 建议做法

- 开发环境直接从 `manifest/dpproxy/ncp.yaml` 派生自己的配置文件。
- 把相对路径和实际工作目录保持一致，避免 SQLite、证书和包目录解析错误。
- 若需要调试 reload 行为，可使用 `POST /-/reload`。
