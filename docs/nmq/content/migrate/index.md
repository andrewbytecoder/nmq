---
title: "NCP Migration"
description: "根据概要设计整理的 NCP 演进方向与兼容性思路。"
---

# Migrate

NCP 的仓库里暂时没有像 Traefik 那样按版本拆分的迁移手册，但 `docs/概要设计/ncp平台概要设计/ncp概要设计.md` 已经明确描述了平台的演进目标。

## 当前可见的演进方向

- 从“单体业务功能堆叠”演进到“组件化平台”。
- 从“直接耦合实现”演进到“通过接口能力调用”。
- 从“单一程序职责”演进到“运行时 + 存储 + API + 部署资产”协同。

## 做迁移时最该关注的点

### 1. 配置路径

迁移环境时，优先检查：

- `--work.dir`
- `--config.file`
- `pkg.*` 目录类配置
- `database.path`

### 2. 证书和端口

`server.http`、`server.https`、`server.webui` 决定三类入口，变更环境时不要只迁移一个端口。

### 3. 部署资产一致性

如果升级部署器或包管理逻辑，通常还需要同步调整 `manifest/dpproxy/packages` 与 `manifest/helm-charts`。

### 4. 存储结构

`dpcore` 启动阶段会执行自动迁移，因此迁移 SQLite 数据时要同时关注：

- 目标二进制版本
- 自动迁移逻辑
- 新增列或表的兼容性
