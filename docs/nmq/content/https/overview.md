---
title: "NCP HTTPS Overview"
description: "NCP 的 HTTPS 监听方式、证书来源和相关端口。"
---

# HTTPS Overview

NCP 的 HTTPS 入口由 `dpproxy` 组件在启动阶段创建。

## 启动方式

`plugins/dpproxy/dpproxy_component.go` 会：

1. 根据 `GetCertFiles()` 解析证书路径。
2. 在 goroutine 中调用 `ginRouter.RunTLS(...)`。
3. 同步更新运行时地址缓存。

## 默认监听信息

- 配置键：`server.https`
- 默认端口：`11091`
- 默认证书目录：`<work.dir>/certs`

## 证书文件名

- `server.crt`
- `server.key`

## 与 HTTP 的关系

- HTTPS 和 HTTP 同时启动。
- HTTP 默认监听 `11090`。
- WebUI 默认独立监听 `11092`，不复用 HTTPS 监听器。

## 适合的用途

- 对外提供受保护 API。
- 本地验证登录、JWT、升级和部署接口。
- 在需要时配合反向代理或入口网关二次发布。
