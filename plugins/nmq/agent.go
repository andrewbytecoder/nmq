package nmq

import (
	"github.com/google/gops/agent"
)

func loadAgentByConfig(cfg *Config) error {
	// 启动gops agent
	if cfg.enableGoPs && envEnableGoPs() {
		if err := agent.Listen(agent.Options{}); err != nil {
			return err
		}
	}

	if cfg.enablePyroscope && envEnablePyroscope() {
		err := startPyroscope()
		if err != nil {
			return err
		}
	}

	return nil
}
