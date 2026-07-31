---
title: "Contributing to NCP"
description: "根据当前仓库结构整理的 NCP 贡献建议。"
---

# Contributing

NCP 当前还没有完整的外部贡献流程文档，但仓库本身已经清晰表达了协作方式。

## 建议的贡献路径

1. 先阅读 `docs/概要设计/`，理解平台目标。
2. 再看 `cmd/`、`plugins/`、`interfaces/` 的运行关系。
3. 修改前先判断是接口层、平台层还是基础包层问题。
4. 为新增或修复的关键逻辑补充测试。

## 开发约定

- 公共约定放在 `interfaces/`
- 可复用能力放在 `pkg/`
- 组件实现放在 `plugins/`
- 内部配置和运行时细节放在 `internal/`

## 基本检查

```bash
go test ./...
```

## 文档同步

如果变更会影响工作目录、配置键、API 分组或部署资产，建议同步更新 `docs/ncp`。
