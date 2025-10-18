package nmq

type Config struct {
	enableGoPs      bool
	enablePyroscope bool
}

func DefaultConfig() *Config {
	return &Config{
		enableGoPs:      false,
		enablePyroscope: false,
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
