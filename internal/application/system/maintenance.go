package system

import (
	"sync"
	"time"
)

// MaintenanceState represents the current maintenance mode status.
type MaintenanceState struct {
	Enabled      bool       `json:"enabled"`
	Reason       string     `json:"reason,omitempty"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	EstimatedEnd *time.Time `json:"estimatedEnd,omitempty"`
}

// MaintenanceManager manages the maintenance mode state.
type MaintenanceManager struct {
	mu       sync.RWMutex
	state    MaintenanceState
	onChange func(state MaintenanceState) // Optional callback when state changes
}

// NewMaintenanceManager creates a new maintenance manager.
func NewMaintenanceManager() *MaintenanceManager {
	return &MaintenanceManager{
		state: MaintenanceState{Enabled: false},
	}
}

// SetOnChange sets a callback to be called when maintenance state changes.
// This can be used to broadcast SSE events.
func (m *MaintenanceManager) SetOnChange(fn func(state MaintenanceState)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = fn
}

// GetState returns the current maintenance state.
func (m *MaintenanceManager) GetState() MaintenanceState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// IsEnabled returns whether maintenance mode is currently enabled.
func (m *MaintenanceManager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.Enabled
}

// Enable enables maintenance mode with a reason.
func (m *MaintenanceManager) Enable(reason string, estimatedDuration time.Duration) MaintenanceState {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	m.state = MaintenanceState{
		Enabled:   true,
		Reason:    reason,
		StartedAt: &now,
	}

	if estimatedDuration > 0 {
		end := now.Add(estimatedDuration)
		m.state.EstimatedEnd = &end
	}

	// Notify listeners
	if m.onChange != nil {
		go m.onChange(m.state)
	}

	return m.state
}

// Disable disables maintenance mode.
func (m *MaintenanceManager) Disable() MaintenanceState {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state = MaintenanceState{
		Enabled: false,
	}

	// Notify listeners
	if m.onChange != nil {
		go m.onChange(m.state)
	}

	return m.state
}

// UpdateReason updates the maintenance reason without changing other state.
func (m *MaintenanceManager) UpdateReason(reason string) MaintenanceState {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.state.Enabled {
		return m.state
	}

	m.state.Reason = reason

	// Notify listeners
	if m.onChange != nil {
		go m.onChange(m.state)
	}

	return m.state
}

// UpdateEstimatedEnd updates the estimated end time.
func (m *MaintenanceManager) UpdateEstimatedEnd(end time.Time) MaintenanceState {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.state.Enabled {
		return m.state
	}

	m.state.EstimatedEnd = &end

	// Notify listeners
	if m.onChange != nil {
		go m.onChange(m.state)
	}

	return m.state
}
