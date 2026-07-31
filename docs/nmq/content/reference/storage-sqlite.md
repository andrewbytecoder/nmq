---
title: "NCP SQLite Storage"
description: "NCP 默认 SQLite 存储的职责边界。"
---

# SQLite Storage

`dpcore` 默认把 SQLite 作为平台状态存储。

## 连接方式

- 默认类型：`sqlite`
- 默认路径：`data/dp.db`
- 创建位置：`plugins/dpcore/storage.NewStorage(...)`

## 自动迁移

启动时会执行两次相关动作：

1. `storage.AutoMigrate(dp.storage)`
2. `dp.storage.AutoMigrate()`

这说明运行时本身会主动维护表结构。

## 当前数据域

根据 storage 子包和接口定义，至少覆盖：

- `idc_info`
- `product_info`
- `deploy_ip_info`
- `cert_info`
- `topo_info`
- `operate_log`
- `service_group_mng`
- `config_data`

## 只读查看方式

`webui` 提供：

- `GET /api/storage/sqlite`
- `GET /api/storage/sqlite/:tableName`

适合做运维排查和表结构确认。
