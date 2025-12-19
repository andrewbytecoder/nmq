// config_converter_test.go
package convert

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// TestParseConfig 测试正常情况下的配置解析
func TestParseConfig(t *testing.T) {
	// 创建临时配置文件
	content := `
server:
  port: 8080
  host: localhost
`
	tmpfile, err := os.CreateTemp("", "test-config-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte(content))
	assert.NoError(t, err)
	err = tmpfile.Close()
	assert.NoError(t, err)

	// 设置 viper 配置文件路径
	viper.Set("configFile", tmpfile.Name())

	// 定义用于测试的配置结构体
	type ServerConfig struct {
		Port int    `mapstructure:"port"`
		Host string `mapstructure:"host"`
	}

	type Config struct {
		Server ServerConfig `mapstructure:"server"`
	}

	// 调用 ParseConfig 解析配置
	config, err := ParseConfig[Config]()
	assert.NoError(t, err)
	assert.Equal(t, 8080, config.Server.Port)
	assert.Equal(t, "localhost", config.Server.Host)
}

// TestParseConfig_NoConfigFile 测试没有指定配置文件的情况
func TestParseConfig_NoConfigFile(t *testing.T) {
	viper.Set("configFile", "")

	type TestConfig struct{}

	_, err := ParseConfig[TestConfig]()
	assert.Error(t, err)
	assert.EqualError(t, err, "no config file specified")
}

// TestParseConfig_InvalidConfigFile 测试读取不存在的配置文件
func TestParseConfig_InvalidConfigFile(t *testing.T) {
	viper.Set("configFile", "/path/to/nonexistent/config.yaml")

	type TestConfig struct{}

	_, err := ParseConfig[TestConfig]()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error reading config file")
}
