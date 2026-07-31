---
title: "NCP CLI"
description: "NCP 的命令行入口、全局参数和常见运行方式。"
---

# CLI

NCP 的 CLI 基于 Cobra，根命令由 `plugins/ncp` 初始化，业务子命令由各组件补充。

## 当前已知入口

- 根命令：`ncp`
- 组件子命令：`dpproxy`

## 全局参数

`plugins/ncp/ncp.go` 当前注册了以下持久化参数：

- `--config.file`, `-f`
- `--cert.path`, `-c`
- `--work.dir`, `-w`
- `--remoter.work.dir`, `-r`
- `--register.prefix.metrics`
- `--register.metrics`
- `--register.process.metrics`
- `--register.go.metrics`
- `--register.server.metrics`

## 典型运行方式

```bash
ncp dpproxy --work.dir ./manifest/dpproxy --config.file ncp.yaml
```

## 生命周期行为

- 执行命令前：解析配置、初始化组件、注册 metrics、启动组件
- 执行命令后：停止组件、释放协程池、执行 reset

## 运维建议

- 本地开发明确传入 `--work.dir`
- 多套环境时固定 `--config.file`
- 观测压测时可以按需关闭部分 metrics
