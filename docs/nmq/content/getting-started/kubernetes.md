---
title: "NCP Quick Start on Kubernetes"
description: "理解 NCP 与 Kubernetes/Helm 资产的最短路径。"
---

# Kubernetes Quick Start

NCP 仓库不是只有一个服务镜像，它同时包含了自身运行时和大量部署资产。Kubernetes 相关内容主要分布在以下位置：

- `pkg/kube`: Kubernetes 客户端封装
- `plugins/dpcore/helmclient`: Helm 客户端装配
- `manifest/helm-charts`: 平台级 Helm chart
- `manifest/dpproxy/packages/*/DPConfig/charts`: 产品包级 charts

## 启动思路

1. 先把 `ncp` 作为控制面服务启动。
2. 通过 `kube-config/config.yaml` 或集群内凭据让 `dpcore` 能访问集群。
3. 通过 `dpproxy` 的部署管理 API 调用 Helm/拓扑/包管理能力。

## 关键代码路径

- `plugins/dpcore/dp_component.go` 在初始化阶段创建 Helm v3 客户端。
- `plugins/dpproxy/httpapi/deploy/kubernetes.go` 暴露节点、拓扑、产品与部署相关 API。
- `pkg/kube/kube_client.go` 负责 kube config 或集群内 token 探测。

## 建议准备

- 在工作目录下提供 `kube-config/config.yaml`
- 确保 `pkg.manifest_dir`、`pkg.helm_char_dir`、`pkg.pkg_dir` 与真实目录一致
- 预先准备好 `manifest/dpproxy/packages` 下的产品包

## 常见验证点

- 节点信息查询接口可返回 K8s 节点列表
- 包信息接口可识别 `packages/` 下产品版本
- 部署接口能找到 chart 和 values 资产
