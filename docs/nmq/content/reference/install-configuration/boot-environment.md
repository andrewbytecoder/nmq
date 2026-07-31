---
title: "NCP Boot Environment"
description: "NCP 启动前需要准备的目录、文件和外部依赖。"
---

# Boot Environment

NCP 要成功启动，最重要的是工作目录必须完整。

## 最小工作目录

```text
<work.dir>
├── ncp.yaml
├── certs/
│   ├── server.crt
│   └── server.key
├── data/
│   └── dp.db
└── kube-config/
    └── config.yaml
```

## 可选但常见的目录

- `packages/`
- `tmp_pkg/`
- `tarTemp/`
- `manifest/`
- 业务日志目录

## 外部依赖

- Kubernetes API
- 可能存在的微平台 Service Manager
- 文件系统可写权限
- HTTPS 证书

## 默认样例位置

仓库中最适合做启动模板的是：

- `manifest/dpproxy/ncp.yaml`
- `manifest/dpproxy/certs/`
- `manifest/dpproxy/kube-config/`
- `manifest/dpproxy/data/`
