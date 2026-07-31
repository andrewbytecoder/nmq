---
title: "Run NCP Locally"
description: "基于仓库现有样例在本地直接启动 NCP。"
---

# Run Locally

最省事的本地运行方式，是直接复用 `manifest/dpproxy` 作为工作目录。

## 1. 编译

```bash
go build -o ncp ./cmd/ncp
```

## 2. 准备工作目录

确认以下内容存在：

- `manifest/dpproxy/ncp.yaml`
- `manifest/dpproxy/certs/server.crt`
- `manifest/dpproxy/certs/server.key`
- `manifest/dpproxy/data/dp.db`
- `manifest/dpproxy/kube-config/config.yaml`

## 3. 启动

```bash
./ncp dpproxy --work.dir ./manifest/dpproxy --config.file ncp.yaml
```

## 4. 验证

- `http://127.0.0.1:11090/health`
- `http://127.0.0.1:11090/metrics`
- `http://127.0.0.1:11092/dashboard/`

## 5. 常见问题

- 如果 SQLite 打不开，先检查 `database.path` 是否相对 `work.dir` 正确。
- 如果 HTTPS 启动失败，先检查 `certs/` 是否存在。
- 如果 K8s 相关接口失败，优先检查 `kube-config/config.yaml`。
