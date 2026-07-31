---
title: "Set Up NCP with Docker"
description: "把 NCP 作为容器运行时需要准备的文件与端口。"
---

# Docker Setup

如果你希望长期用 Docker 运行 NCP，重点不在镜像本身，而在工作目录约定。

## 建议目录

```text
/work
├── ncp.yaml
├── certs/
│   ├── server.crt
│   └── server.key
├── data/
│   └── dp.db
├── kube-config/
│   └── config.yaml
├── packages/
├── tmp_pkg/
└── tarTemp/
```

## 端口

默认样例使用以下端口：

- `11090`: HTTP API
- `11091`: HTTPS API
- `11092`: WebUI

## 关键参数

- `--work.dir`: 指向容器内工作目录
- `--config.file`: 指定 YAML 文件名
- `--cert.path`: 当证书不在 `work.dir/certs` 时显式覆盖

## 需要特别注意

- `manifest/docker/ncp/Dockerfile` 中运行命令路径看起来仍需结合你的镜像目录结构确认。
- 如果你只挂可执行文件，不挂工作目录，`dpproxy` 初始化阶段很快就会因为配置、证书或 kube config 缺失而失败。
