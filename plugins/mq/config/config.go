package config

import (
	"github.com/andrewbytecoder/nmq/pkg/options"
)

type Config struct {
	Type   int `json:"type"` // 0. server 1. client
	Client Client
	Server Server
}

func NewConfig(opts ...options.Option) *Config {
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

type Server struct {
	Port int
	Addr string
}

type Client struct {
	Port int
	Addr string
}

func SetClient(port int, addr string) options.Option {
	return func(c any) {
		cfg := c.(*Config)
		cfg.Type = 1
		cfg.Client = Client{
			Port: port,
			Addr: addr,
		}
	}
}
func SetServer(port int, addr string) options.Option {
	return func(c any) {
		cfg := c.(*Config)
		cfg.Type = 0
		cfg.Server = Server{
			Port: port,
			Addr: addr,
		}
	}
}
