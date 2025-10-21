package nmq

import (
	"fmt"
	"os"

	"github.com/andrewbytecoder/nmq/pkg/check"
	"github.com/grafana/pyroscope-go"
)

func startPyroscope() error {
	// 只有配置了DP_PYROSCOPE_ENABLE为true时，才启动pyroscope
	address := os.Getenv("DP_PYROSCOPE_SERVER_ADDRESS")
	if address == "" {
		return fmt.Errorf("DP_PYROSCOPE_SERVER_ADDRESS is empty")
	}

	if !check.IsValidPyroscopeAddress(address) {
		return fmt.Errorf("DP_PYROSCOPE_SERVER_ADDRESS is invalid, address: %s\n", address)
	}

	pyroscope.Start(pyroscope.Config{
		ApplicationName: "dp.ncp.service",
		// http://pyroscope-server:4040 指定为pyroscope服务地址
		ServerAddress: address,
		Logger:        pyroscope.StandardLogger,

		ProfileTypes: []pyroscope.ProfileType{
			// these profile types are enabled by default:
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,

			// these profile types are optional:
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})

	return nil
}
