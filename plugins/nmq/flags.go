package nmq

func ParseFlags(n *Nmq) {
	// config --config.file 无论在子命令还是主命令里面都只能使用一次
	n.rootCmd.PersistentFlags().StringVarP(&n.cfg.configFile, "config.file", "f", "nmq.yaml", "input the config file name")
	n.rootCmd.PersistentFlags().StringVarP(&n.cfg.certPath, "cert.path", "c", "", "cert path for https")
	n.rootCmd.PersistentFlags().StringVarP(&n.cfg.workDir, "work.dir", "w", "", "config local work path")
	n.rootCmd.PersistentFlags().StringVar(&n.cfg.metricsConfig.prefix, "register.prefix.metrics", "ncp", "metrics name prefix")
	n.rootCmd.PersistentFlags().BoolVar(&n.cfg.metricsConfig.enableMetrics, "register.metrics", true, "enable metrics")
	n.rootCmd.PersistentFlags().BoolVar(&n.cfg.metricsConfig.registerProcessMetrics, "register.process.metrics", true, "register process metrics")
	n.rootCmd.PersistentFlags().BoolVar(&n.cfg.metricsConfig.registerGoMetrics, "register.go.metrics", true, "register go metrics")
	n.rootCmd.PersistentFlags().BoolVar(&n.cfg.metricsConfig.registerServerMetrics, "register.server.metrics", true, "register server metrics")
	// 注册webui组件开关, 默认情况下使能
	n.rootCmd.PersistentFlags().BoolVar(&n.cfg.componentConfigs.webui.enable, "webui.enable", true, "enable webui component")
}
