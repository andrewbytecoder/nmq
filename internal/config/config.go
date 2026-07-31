package config

// NmqConfigFile 配置文件名称（编译时常量）
const NmqConfigFile = "nmq.yaml"

// globalConfig 全局配置实例（包级私有，外部不可直接修改）
// 虽然是 var，但通过不导出 + 只提供 Getter 的方式达到"只读全局变量"的效果
var globalConfig = Config{}

// GetGlobalConfig 获取全局配置（只读访问）
func GetGlobalConfig() *Config {
	return &globalConfig
}

// SetGlobalConfig 设置全局配置（仅在初始化阶段调用）
func SetGlobalConfig(cfg Config) {
	globalConfig = cfg
}

type Config struct {
	Network NetworkConfig `yaml:"network"`
}
