package client

import "github.com/andrewbytecoder/nmq/pkg/options"

// Config holds the configuration for the websocket server
// 包含端口和地址配置项
type Config struct {
	// Port specifies the port number the server will listen on
	// 服务器监听的端口号，默认为8080
	Port int
	// Addr specifies the address the server will bind to
	// 服务器绑定的IP地址，默认为"0.0.0.0"
	Addr string
}

// NewConfig creates a new Config instance with default values and applies provided options
// 使用默认值创建新的Config实例，并应用传入的选项
// 参数opts是可变的选项函数，用于自定义配置
func NewConfig(opts ...options.Option) *Config {
	c := &Config{
		Port: 8080,
		Addr: "0.0.0.0",
	}

	// Apply each option to the config
	// 遍历并应用每个选项函数
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// SetPort returns an Option that sets the Port field of Config
// 返回一个设置Config的Port字段的Option函数
// 参数port是要设置的端口号
func SetPort(port int) options.Option {
	return func(c any) {
		// Type assert to ensure we're working with a Config pointer
		// 类型断言确保我们操作的是Config指针
		if c, ok := c.(*Config); ok {
			c.Port = port
		}
	}
}

// SetAddr returns an Option that sets the Addr field of Config
// 返回一个设置Config的Addr字段的Option函数
// 参数addr是要设置的地址
func SetAddr(addr string) options.Option {
	return func(c any) {
		// Type assert to ensure we're working with a Config pointer
		// 类型断言确保我们操作的是Config指针
		if c, ok := c.(*Config); ok {
			c.Addr = addr
		}
	}
}
