---
title: "NCP Certificates"
description: "NCP 当前证书文件布局与管理约定。"
---

# Certificates

NCP 当前实现对证书的要求比较直接：运行时只要能拿到 `server.crt` 和 `server.key` 即可启动 HTTPS。

## 路径解析规则

`GetCertFiles()` 的逻辑如下：

- 如果传入 `--cert.path`，则使用 `<cert.path>/server.crt` 和 `<cert.path>/server.key`
- 否则使用 `<work.dir>/certs/server.crt` 和 `<work.dir>/certs/server.key`

## 仓库内默认样例

仓库已经自带：

- `manifest/dpproxy/certs/server.crt`
- `manifest/dpproxy/certs/server.key`

## 相关数据能力

仓库还存在证书信息存储接口：

- 能力名：`dp-certinfo-storage`
- 数据模型：`interfaces/dpcore/model/certinfo.go`
- WebUI 可通过 SQLite 视图或证书相关 API 间接查看

## 实践建议

- 开发环境可以直接复用 `manifest/dpproxy/certs/`。
- 生产环境应通过配置卷、Secret 或外部证书目录注入。
- 不建议把长期有效证书直接固化在镜像内。
