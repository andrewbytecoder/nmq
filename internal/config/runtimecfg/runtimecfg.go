package runtimecfg

var (
	globalConfig = RuntimeCfg{}
)

// RuntimeCfg 运行时生成的配置
type RuntimeCfg struct {
	Server  Server
	DirInfo DirInfo
}

// Status of the router/service.
const (
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"
	StatusWarning  = "warning"
)

// Status of the servers.
const (
	StatusUp   = "UP"
	StatusDown = "DOWN"
)
