package tls

import (
	"sync"
)

// Manager is the TLS option/store/configuration factory
type Manager struct {
	lock         sync.RWMutex
	storesConfig map[string]Store
}
