package config

import (
	"fmt"
	"reflect"

	"github.com/andrewbytecoder/nmq/pkg/convert"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func ParseConfig(log *zap.Logger) error {

	parsedConfig, err := convert.ParseConfig[Config]()
	if err != nil {
		return fmt.Errorf("error parsing config: %v", err)
	}
	// If ReadInConfig succeeded but Unmarshal yielded zero values, it almost always means:
	// - viper read the wrong YAML file (e.g. kubeconfig), or
	// - YAML key layout doesn't match Config's mapstructure tags.
	// Fail fast with actionable diagnostics instead of silently running with defaults.
	if reflect.DeepEqual(parsedConfig, Config{}) {
		used := viper.ConfigFileUsed()
		settings := viper.AllSettings()
		return fmt.Errorf("config is empty after unmarshal (configFileUsed=%q, topLevelKeys=%d): check that --config.file points to the expected YAML and keys match Config mapstructure tags", used, len(settings))
	}

	SetGlobalConfig(parsedConfig)

	return nil
}
