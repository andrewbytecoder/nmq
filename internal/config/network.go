package config

import (
	"fmt"
	"strings"
)

type NetworkConfig struct {
	Webui ServiceConfig `yaml:"webui"`
}

type ServiceConfig struct {
	Name    string `yaml:"name"`
	Address string `yaml:"address"` // ip:port or domain:port or domain
	Port    int    `yaml:"port"`
}

func GetNetworkConfig() *NetworkConfig {
	cfg := GetGlobalConfig()
	return &cfg.Network
}

func (c *NetworkConfig) GetWebuiConfig() *ServiceConfig {
	return &c.Webui
}

func (c *ServiceConfig) GetAddress() string {
	if strings.Contains(c.Address, ":") {
		return c.Address
	}

	// construct address
	c.Address = fmt.Sprintf("%s:%d", c.Address, c.Port)

	return c.Address
}

func (c *ServiceConfig) GetPort() int {
	return c.Port
}

func (c *ServiceConfig) GetName() string {
	return c.Name
}
