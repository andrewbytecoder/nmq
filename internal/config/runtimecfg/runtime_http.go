package runtimecfg

import (
	"maps"
	"slices"
	"sync"

	"github.com/andrewbytecoder/nmq/internal/config/dynamic"
)

// Status of the router/service.
const (
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"
	StatusWarning  = "warning"
)

// ServiceInfo holds information about a currently running service.
type ServiceInfo struct {
	*dynamic.Service // dynamic configuration

	// Err contains all the errors that occurred during service creation.
	Err []string `json:"error,omitempty"`
	// Status reports whether the service is disabled, in a warning state, or all good (enabled).
	// If not in "enabled" state, the reason for it should be in the list of Err.
	// It is the caller's responsibility to set the initial status.
	Status string   `json:"status,omitempty"`
	UsedBy []string `json:"usedBy,omitempty"` // list of routers using that service

	serverStatusMu sync.RWMutex
	serverStatus   map[string]string // keyed by server URL
}

// AddError adds err to s.Err, if it does not already exist.
// If critical is set, s is marked as disabled.
func (s *ServiceInfo) AddError(err error, critical bool) {
	if slices.Contains(s.Err, err.Error()) {
		return
	}

	s.Err = append(s.Err, err.Error())
	if critical {
		s.Status = StatusDisabled
		return
	}

	// only set it to "warning" if not already in a worse state
	if s.Status != StatusDisabled {
		s.Status = StatusWarning
	}
}

// UpdateServerStatus sets the status of the server in the ServiceInfo.
// It is the responsibility of the caller to check that s is not nil.
func (s *ServiceInfo) UpdateServerStatus(server, status string) {
	s.serverStatusMu.Lock()
	defer s.serverStatusMu.Unlock()

	if s.serverStatus == nil {
		s.serverStatus = make(map[string]string)
	}
	s.serverStatus[server] = status
}

// GetAllStatus returns all the statuses of all the servers in ServiceInfo.
// It is the responsibility of the caller to check that s is not nil.
func (s *ServiceInfo) GetAllStatus() map[string]string {
	s.serverStatusMu.RLock()
	defer s.serverStatusMu.RUnlock()

	if len(s.serverStatus) == 0 {
		return nil
	}

	return maps.Clone(s.serverStatus)
}
