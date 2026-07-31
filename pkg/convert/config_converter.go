package convert

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

func getConfigName(configFile string) string {
	ext := filepath.Ext(configFile)
	if ext != "" {
		return configFile[:len(configFile)-len(ext)]
	}
	return configFile
}

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

	// 如果configFile是相对路径，需要结合工作目录
	// 检查是否已经是绝对路径
	if !filepath.IsAbs(configFile) {
		// Backward compatible: allow passing "config" without extension.
		// viper will search config.{yaml,yml,json,...} under AddConfigPath.
		viper.SetConfigName(getConfigName(configFile))
		viper.SetConfigType(strings.TrimPrefix(filepath.Ext(configFile), "."))
	} else {
		// 如果是绝对路径，直接使用
		viper.SetConfigFile(configFile)
	}
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
