---
title: "NCP Deployment Workflow"
description: "NCP 当前仓库里的部署、升级与回滚流程概览。"
---

# Deployment Workflow

NCP 的部署链路由 `dpcore` 和 `dpproxy` 协同完成。

## 关键角色

- `dpcore`: 提供 Helm client、部署器、SQLite 存储和微平台客户端
- `dpproxy`: 暴露部署、升级、包管理和拓扑 API
- `manifest/*`: 提供 chart、包和默认工作目录

## 典型流程

1. 上传或识别产品包。
2. 读取包中的 `DPConfig`、`manifest` 和 `charts`。
3. 生成或同步拓扑信息。
4. 读取节点、K8s 组和服务信息。
5. 调用 Helm/Kubernetes 完成部署。
6. 通过进度接口查看执行状态。
7. 通过升级接口执行服务升级、预升级或回滚。

## 相关 API 组

- `/dp/api/deployMng`
- `/dp/api/pkgMng`
- `/dp/api/top`
- `/dp/api/upgrade`

## 当前仓库的特点

- 既支持平台级 Helm chart，也支持产品包内自带 chart。
- 包管理支持分片上传、合并和进度查询。
- 升级接口已经区分普通升级、预升级、K8s 升级和在线服务升级。
