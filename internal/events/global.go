package events

import (
	"sync"
)

var (
	globalBus     EventBus
	globalBusLock sync.RWMutex
)

// SetGlobalEventBus sets the global event bus instance
func SetGlobalEventBus(bus EventBus) {
	globalBusLock.Lock()
	defer globalBusLock.Unlock()
	globalBus = bus
}

// GetGlobalEventBus returns the global event bus instance
func GetGlobalEventBus() EventBus {
	globalBusLock.RLock()
	defer globalBusLock.RUnlock()
	return globalBus
}
