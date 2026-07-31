---
title: "Getting Started with NCP"
description: "快速理解并启动 NCP。"
---

# Getting Started with NCP

NCP 的启动依赖非常直接：Go 构建产物、工作目录、配置文件、证书目录，以及可选的 kube config。

## 推荐阅读顺序

- [Configuration Introduction](configuration-overview.md)
- [Docker Quick Start](docker.md)
- [Kubernetes Quick Start](kubernetes.md)

## 在开始之前

请确认你至少具备以下条件：

- Go 开发环境，可直接编译 `cmd/ncp`
- 一个可作为工作目录的目录，里面至少有 `ncp.yaml`
- 如果要启用 HTTPS，需要 `certs/server.crt` 和 `certs/server.key`
- 如果要调用 Kubernetes，需要 `kube-config/config.yaml` 或集群内 ServiceAccount

## 最短运行路径

1. 进入仓库根目录。
2. 使用 `go build` 生成 `ncp` 可执行文件。
3. 选择 `manifest/dpproxy` 作为默认工作目录。
4. 启动 `ncp dpproxy`。
5. 访问 HTTP、HTTPS、WebUI 与 metrics 端点进行验证。
