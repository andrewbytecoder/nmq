package runtimecfg

import "github.com/andrewbytecoder/nmq/internal/config/dynamic"

// UDPRouterInfo holds information about a currently running UDP router.
type UDPRouterInfo struct {
	*dynamic.UDPRouter // dynamic configuration

	Err []string `json:"error,omitempty"` // initialization error
	// Status reports whether the router is disabled, in a warning state, or all good (enabled).
	// If not in "enabled" state, the reason for it should be in the list of Err.
	// It is the caller's responsibility to set the initial status.
	Status string   `json:"status,omitempty"`
	Using  []string `json:"using,omitempty"` // Effective entry points used by that router.
}

// UDPServiceInfo holds information about a currently running UDP service.
// udp 服务器，当routers多的时候，分别分配给不同的udp服务器
type UDPServiceInfo struct {
	*dynamic.UDPService // dynamic configuration

	Err []string `json:"error,omitempty"` // initialization error
	// Status reports whether the service is disabled, in a warning state, or all good (enabled).
	// If not in "enabled" state, the reason for it should be in the list of Err.
	// It is the caller's responsibility to set the initial status.
	Status string   `json:"status,omitempty"`
	UsedBy []string `json:"usedBy,omitempty"` // list of routers using that service
}
