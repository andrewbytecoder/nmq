---
title: "NCP Operations"
description: "从 CLI、包目录和部署流程理解 NCP 的日常运维路径。"
---

# Operations

NCP 的日常运维基本围绕三件事展开：

- 启停和参数控制
- 包与存储资产管理
- 部署、升级与回滚

## 入口

- CLI 入口看 [CLI](cli.md)
- 目录与包看 [Storage & Packages](storage-and-packages.md)
- 交付链路看 [Deployment Workflow](deployment-workflow.md)

## 平台运维特点

这套系统并不是“单二进制 + 无状态配置”模型。它需要同时维护：

- 可执行文件
- YAML 配置
- SQLite 数据
- 证书
- Kubernetes 凭据
- 产品包与 Helm charts
