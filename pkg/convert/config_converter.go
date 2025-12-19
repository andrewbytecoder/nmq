package convert

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"
)

// ParseConfig parses the configuration from a YAML file
// @Description Parses the configuration from a YAML file
// @Return error 如果解析失败，返回错误信息
func ParseConfig[T any]() (T, error) {
	var config T
	// configFile 在ncp中已经绑定，需要使用的地方直接使用即可
	configFile := viper.GetString("configFile")
	if configFile == "" {
		return config, errors.New("no config file specified")
	}
	viper.AddConfigPath(".") // add the current directory as a search path
	viper.SetConfigFile(configFile)

	// load the yaml config file info
	err := viper.ReadInConfig()
	if err != nil {
		return config, fmt.Errorf("error reading config file: %v", err)
	}

	// 将主进程中绑定的配置绑定到结构体中
	if err := viper.Unmarshal(&config); err != nil {
		return config, err
	}

	return config, nil
}
