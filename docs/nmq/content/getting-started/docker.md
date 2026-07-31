---
title: "NCP Quick Start on Docker"
description: "结合仓库中的 Dockerfile 和默认工作目录快速启动 NCP。"
---

# Docker Quick Start

仓库已经提供了面向 `ncp` 的容器构建文件：`manifest/docker/ncp/Dockerfile`。

## 适用前提

- 你想把 `cmd/ncp` 编译为容器镜像。
- 你准备挂载 `ncp.yaml`、`certs/`、`data/`、`packages/` 等运行资产。

## 构建镜像

```bash
docker build -t ncp:local -f manifest/docker/ncp/Dockerfile .
```

这个 Dockerfile 会：

- 在构建阶段执行 `go build ./cmd/ncp/ncp.go`
- 向版本包注入 git 分支、tag、commit 和构建时间
- 在运行阶段复制二进制到 Alpine 镜像

## 运行前准备

建议直接把 `manifest/dpproxy` 作为容器内工作目录映射：

- `ncp.yaml`
- `certs/server.crt`
- `certs/server.key`
- `data/dp.db`
- `kube-config/config.yaml`

## 运行命令示例

```bash
docker run --rm -p 11090:11090 -p 11091:11091 -p 11092:11092 \
  -v $(pwd)/manifest/dpproxy:/work \
  ncp:local /home/ysp-user/ncp dpproxy --work.dir /work --config.file ncp.yaml
```

## 启动后验证

- `http://127.0.0.1:11090/health`
- `http://127.0.0.1:11090/metrics`
- `https://127.0.0.1:11091/health`
- `http://127.0.0.1:11092/dashboard/`
