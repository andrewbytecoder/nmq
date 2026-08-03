package runtimecfg

import "github.com/andrewbytecoder/nmq/internal/config/dynamic"

// Configuration holds the information about the currently running traefik instance.
type Configuration struct {
	Routers        map[string]*RouterInfo        `json:"routers,omitempty"`
	Middlewares    map[string]*MiddlewareInfo    `json:"middlewares,omitempty"`
	Services       map[string]*ServiceInfo       `json:"services,omitempty"`
	Models         map[string]*dynamic.Model     `json:"-"`
	TCPRouters     map[string]*TCPRouterInfo     `json:"tcpRouters,omitempty"`
	TCPMiddlewares map[string]*TCPMiddlewareInfo `json:"tcpMiddlewares,omitempty"`
	TCPServices    map[string]*TCPServiceInfo    `json:"tcpServices,omitempty"`
	UDPRouters     map[string]*UDPRouterInfo     `json:"udpRouters,omitempty"`
	UDPServices    map[string]*UDPServiceInfo    `json:"udpServices,omitempty"`
}

// RouterInfo holds information about a currently running HTTP router.
type RouterInfo struct {
	*dynamic.Router // dynamic configuration

	// Err contains all the errors that occurred during router's creation.
	Err []string `json:"error,omitempty"`
	// Status reports whether the router is disabled, in a warning state, or all good (enabled).
	// If not in "enabled" state, the reason for it should be in the list of Err.
	// It is the caller's responsibility to set the initial status.
	Status string   `json:"status,omitempty"`
	Using  []string `json:"using,omitempty"` // Effective entry points used by that router.

	// ChildRefs contains the names of child routers.
	// This field is only filled during multi-layer routing computation of parentRefs,
	// and used when building the runtime configuration.
	ChildRefs []string `json:"-"`
}
