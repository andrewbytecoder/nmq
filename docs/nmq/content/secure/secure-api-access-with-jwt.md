---
title: "Secure API Access with JWT"
description: "NCP 当前登录、验证码和 JWT 鉴权链路概览。"
---

# Secure Access with JWT

NCP 的私有 API 默认依赖 JWT 中间件保护。

## 当前鉴权链路

1. 公开接口提供验证码与登录入口。
2. 登录控制器校验验证码策略。
3. 登录成功后生成 JWT，并通过响应体和 Cookie 返回。
4. 私有路由组统一挂载 `middlejwt.JwtAuth()`。

## 相关接口

- `GET /captcha`
- `POST /login`
- `POST /mylogout`
- `GET /getSuperToken`

## 相关配置

```yaml
captcha:
  open_captcha: 3
  open_captcha_timeout: 300

jwt:
  signing_key: "new-dp-signature"
  expires_time: "24h"
  buffer_time: "10m"
  issuer: "ysp-auth-server"
```

## 代码来源

- 登录与验证码：`plugins/dpproxy/httpapi/handler_login.go`
- 中间件：`plugins/dpproxy/middleware/middlejwt`
- JWT 基础库：`pkg/jwt`

## 运维建议

- 不要在正式环境复用示例配置里的签名密钥。
- 公开登录入口时优先只暴露 HTTPS 端口。
- 对 `/dp/api/*` 入口做额外网关或网络层限制会更稳妥。
