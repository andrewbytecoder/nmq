package nmq

import (
	"os"
	"runtime"
	"strings"
)

func envEnablePyroscope() bool {
	if strings.ToLower(os.Getenv("DP_PYROSCOPE_ENABLE")) != "true" {
		return false
	}

	return true
}

func envEnableGoPs() bool {

	// windows 上运行肯定是非正式环境，可以直接启动gops
	if runtime.GOOS == "windows" {
		return true
	}

	if strings.ToLower(os.Getenv("DP_GOPROF_ENABLE")) != "true" {
		return false
	}

	return true
}
