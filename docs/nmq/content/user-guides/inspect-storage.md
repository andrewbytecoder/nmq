---
title: "Inspect SQLite via WebUI"
description: "使用 NCP 自带 WebUI 查看 SQLite 表结构和数据。"
---

# Inspect SQLite via WebUI

`plugins/webui` 已经提供了读取 SQLite 表摘要和表内容的接口，这是当前仓库里最直接的在线排障方式之一。

## 前提

- `webui` 组件已启动
- `dpcore` 已成功初始化 SQLite

## 访问入口

- Dashboard: `http://<host>:11092/dashboard/`
- API: `http://<host>:11092/api/storage/sqlite`

## 可以做什么

- 查看所有 SQLite 表
- 查看单表列信息
- 查看分页数据
- 检查 `cert_info`、拓扑、产品包等运行时状态

## 适合的排障场景

- 自动迁移后确认表是否存在
- 登录或部署接口异常时核对底层状态
- 验证产品包导入或证书同步是否成功
