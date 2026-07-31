---
title: "NCP Governance"
description: "从仓库现状整理出的 NCP 工程治理约定。"
---

# Govern

NCP 当前仓库没有独立的治理制度文档，但从代码布局和概要设计可以提炼出一套非常明确的工程边界。

## 当前治理重心

- 组件之间通过接口协作，而不是直接共享内部结构。
- 基础能力沉淀在 `pkg/`，业务实现放在 `plugins/`。
- 运行时契约集中在 `interfaces/`。
- 平台设计意图在 `docs/概要设计/` 中给出。

## 推荐的治理原则

### 1. 先定义接口，再增加实现

如果一个能力会被多个组件使用，应先补 `interfaces/`，再补 `plugins/` 或 `pkg/`。

### 2. 工作目录是第一等公民

NCP 的配置、证书、SQLite、包目录、kube config 都依赖工作目录解析，所以任何部署方案都应先固定目录契约。

### 3. 部署资产要和运行时一起演进

仓库中的 `manifest/` 不是附属品，而是平台能力的一部分。更新部署逻辑时，通常也要同步检查：

- `manifest/helm-charts`
- `manifest/dpproxy/packages`
- `manifest/dpproxy/ncp.yaml`

### 4. 观测不是事后补丁

`plugins/ncp` 启动时就注册 metrics registry，`dpproxy` 路由默认挂观测中间件，说明仓库已经把可观测性视为基础能力。
