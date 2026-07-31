package nmq

type Config struct {
	enableGoPs       bool
	enablePyroscope  bool
	poolNumber       int    // 协程池大小
	configFile       string // 配置文件
	certPath         string // 证书路径
	workDir          string // 当前工作目录
	debugPort        int    // 调试端口，0 表示不启动
	metricsConfig    MetricsConfig
	componentConfigs ComponentConfigs
}

type ComponentConfigs struct {
	webui        ComponentConfig
	proxy        ComponentConfig
	messageQueue ComponentConfig
}

type ComponentConfig struct {
	name    string
	enable  bool
	version string
	config  any
}

type MetricsConfig struct {
	prefix                 string
	enableMetrics          bool
	registerProcessMetrics bool
	registerGoMetrics      bool
	registerServerMetrics  bool
}

func DefaultConfig() *Config {
	return &Config{
		enableGoPs:      false,
		enablePyroscope: false,
		poolNumber:      10,
		debugPort:       0,
	}
}

func (c *Config) setGoPs(enableGoPs bool) *Config {
	c.enableGoPs = enableGoPs
	return c
}

func (c *Config) setPyroscope(enablePyroscope bool) *Config {
	c.enablePyroscope = enablePyroscope
	return c
}

func (c *Config) setPoolNumber(poolNumber int) *Config {
	c.poolNumber = poolNumber
	return c
}

func (c *Config) setDebugPort(port int) *Config {
	c.debugPort = port
	return c
}
