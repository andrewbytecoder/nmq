配置和运行资产在当前仓库中分成四层：

1. **命令入口层**: `cmd/ncp` 负责注册组件并启动 CLI。
2. **运行时层**: `plugins/ncp` 负责配置、日志、指标和生命周期。
3. **业务组件层**: `plugins/dpcore`、`plugins/dpproxy`、`plugins/webui`、`plugins/network`。
4. **资产层**: `manifest/`、`docs/概要设计/`、`data/`、`certs/`、`packages/`。
