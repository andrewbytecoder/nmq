---
title: "NCP Storage and Packages"
description: "NCP 的 SQLite、产品包目录和部署资产目录。"
---

# Storage & Packages

NCP 的状态和部署资产并不都在数据库里。当前仓库同时依赖 SQLite 和文件目录。

## SQLite

默认数据库配置来自 `database` 段：

- 类型：`sqlite`
- 路径：`data/dp.db`

`plugins/dpcore/storage` 负责连接数据库并执行自动迁移。

## 主要表能力

当前 storage 代码至少覆盖以下数据域：

- IDC 信息
- 产品信息
- 部署 IP/端口
- 证书信息
- 拓扑信息
- 操作日志
- 服务组管理
- 配置数据

## 包目录

`pkg` 配置段定义了包与部署资产的主要目录：

- `tmp_dir`
- `tar_temp`
- `pkg_dir`
- `dp_config_dir`
- `business_dir`
- `manifest_dir`
- `helm_char_dir`

## 仓库内默认资产

- `manifest/dpproxy/packages`: 产品包
- `manifest/helm-charts`: 平台级 chart
- `manifest/ncp`: 环境样例配置

## WebUI 支持

`plugins/webui` 已提供 SQLite 表查看接口，可以在控制台里快速核对当前库状态。
