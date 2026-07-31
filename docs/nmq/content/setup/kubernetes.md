---
title: "Set Up NCP with Kubernetes"
description: "准备 K8s 环境、凭据和部署资产以供 NCP 调用。"
---

# Kubernetes Setup

NCP 在 Kubernetes 相关能力上更像“部署控制器”而不是“只消费 ingress 的业务服务”。

## 需要准备的内容

- 一个可访问的 Kubernetes 集群
- `kube-config/config.yaml`，或者在 Pod 内挂载 ServiceAccount 凭据
- `manifest/helm-charts` 与 `manifest/dpproxy/packages` 下的 chart/manifest 资产
- 可写的工作目录，用来保存 SQLite、包缓存和日志

## 代码级行为

- `pkg/kube/kube_client.go` 会优先尝试集群内 token，也支持外部 kube config。
- `plugins/dpcore/helmclient` 当前默认走 Helm v3。
- `plugins/dpproxy/httpapi/deploy` 暴露部署、拓扑、进度和升级接口。

## 推荐部署方式

- 把 `ncp.yaml`、证书和 `kube-config` 作为配置卷挂载。
- 把 `data/`、`packages/`、`tmp_pkg/` 作为持久卷挂载。
- 通过 Service 暴露 11090、11091、11092，或者只暴露 WebUI/HTTPS 入口。
