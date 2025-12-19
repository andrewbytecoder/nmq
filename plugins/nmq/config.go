package nmq

type Config struct {
	enableGoPs      bool
	enablePyroscope bool
	poolNumber      int    // 协程池大小
	configFile      string // 配置文件
	certPath        string // 证书路径
	workDir         string // 当前工作目录
}

func DefaultConfig() *Config {
	return &Config{
		enableGoPs:      false,
		enablePyroscope: false,
		poolNumber:      10,
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
