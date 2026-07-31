---
title: "NCP Deprecation Status"
description: "NCP 仓库当前可见的废弃信息状态。"
---

# Deprecation

截至 2026-07-27，仓库里没有发现独立的废弃策略目录、版本化 deprecation 清单或统一迁移公告。

## 这意味着什么

- 当前项目仍处于“以代码和设计文档为主”的演进状态。
- 兼容性边界更多体现在接口常量、配置键和部署资产结构上。
- 做变更时，最好把兼容性说明直接写进提交说明或文档页面。

## 当前最应该谨慎变更的区域

- `interfaces/componet_name.go` 中的 capability 名
- `internal/config.Config` 中的配置键
- `manifest/dpproxy/ncp.yaml` 的目录约定
- `manifest/dpproxy/packages` 与 `manifest/helm-charts` 的资产结构

## 推荐做法

如果后续要正式引入废弃机制，建议至少补三类内容：

1. 废弃开始版本
2. 替代方案
3. 移除时间点
