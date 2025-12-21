// Package lifecycle provides server lifecycle management including graceful restart.
package lifecycle

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/domain/events"
)

// RestartExitCode is the exit code used to signal a restart request.
// Process managers (systemd, Docker, etc.) should restart the process on this code.
const RestartExitCode = 42

// Publisher is the interface for publishing events.
type Publisher interface {
	Publish(event events.Event)
}

// Manager coordinates server lifecycle operations including graceful restart.
type Manager struct {
	mu        sync.RWMutex
	logger    *slog.Logger
	publisher Publisher

	// Restart state
	restartPending   bool
	restartReason    string
	restartRequestAt time.Time

	// Pending settings that require restart
	pendingSettings map[string]bool

	// Shutdown signal
	shutdownCh chan struct{}
	shutdownFn func()
}

// NewManager creates a new lifecycle manager.
func NewManager(logger *slog.Logger, publisher Publisher) *Manager {
	return &Manager{
		logger:          logger,
		publisher:       publisher,
		pendingSettings: make(map[string]bool),
		shutdownCh:      make(chan struct{}),
	}
}

// SetShutdownFunc sets the function to call when triggering shutdown.
// This is typically bound to a context cancel function or signal handler.
func (m *Manager) SetShutdownFunc(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutdownFn = fn
}

// MarkSettingPendingRestart marks a setting as requiring a restart to take effect.
func (m *Manager) MarkSettingPendingRestart(settingKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pendingSettings[settingKey] = true

	m.logger.Info("Setting marked as pending restart",
		"setting_key", settingKey,
		"pending_count", len(m.pendingSettings))

	if m.publisher != nil {
		event := events.NewEvent(events.EventSettingsPendingRestart, "lifecycle").
			WithSettingKey(settingKey).
			WithData("pending_count", len(m.pendingSettings)).
			Build()
		m.publisher.Publish(event)
	}
}

// ClearPendingSettings clears all pending restart settings (called after restart).
func (m *Manager) ClearPendingSettings() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingSettings = make(map[string]bool)
}

// GetPendingSettings returns the list of settings that require restart.
func (m *Manager) GetPendingSettings() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	settings := make([]string, 0, len(m.pendingSettings))
	for k := range m.pendingSettings {
		settings = append(settings, k)
	}
	return settings
}

// HasPendingRestart returns true if any settings require a restart.
func (m *Manager) HasPendingRestart() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pendingSettings) > 0 || m.restartPending
}

// RequestRestart requests a server restart with the given reason.
func (m *Manager) RequestRestart(reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.restartPending {
		m.logger.Warn("Restart already pending, ignoring new request",
			"existing_reason", m.restartReason,
			"new_reason", reason)
		return
	}

	m.restartPending = true
	m.restartReason = reason
	m.restartRequestAt = time.Now()

	m.logger.Info("Server restart requested",
		"reason", reason,
		"pending_settings", len(m.pendingSettings))

	if m.publisher != nil {
		event := events.NewEvent(events.EventSystemRestartRequested, "lifecycle").
			WithReason(reason).
			WithData("pending_settings", len(m.pendingSettings)).
			Build()
		m.publisher.Publish(event)
	}
}

// CancelRestart cancels a pending restart request.
func (m *Manager) CancelRestart() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.restartPending {
		return false
	}

	m.logger.Info("Server restart cancelled",
		"original_reason", m.restartReason)

	if m.publisher != nil {
		event := events.NewEvent(events.EventSystemRestartCancelled, "lifecycle").
			WithReason(m.restartReason).
			Build()
		m.publisher.Publish(event)
	}

	m.restartPending = false
	m.restartReason = ""
	m.restartRequestAt = time.Time{}

	return true
}

// ExecuteRestart triggers the actual restart by exiting with RestartExitCode.
// This should be called after graceful shutdown is complete.
func (m *Manager) ExecuteRestart(ctx context.Context) {
	m.mu.Lock()
	reason := m.restartReason
	shutdownFn := m.shutdownFn
	m.mu.Unlock()

	m.logger.Info("Executing server restart",
		"reason", reason,
		"exit_code", RestartExitCode)

	if m.publisher != nil {
		event := events.NewEvent(events.EventSystemShuttingDown, "lifecycle").
			WithReason(reason).
			WithData("restart", true).
			Build()
		m.publisher.Publish(event)
	}

	// Signal shutdown
	if shutdownFn != nil {
		shutdownFn()
	}

	// Give time for graceful shutdown
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
	}

	// Exit with restart code
	os.Exit(RestartExitCode)
}

// RestartStatus returns the current restart status.
type RestartStatus struct {
	Pending         bool      `json:"pending"`
	Reason          string    `json:"reason,omitempty"`
	RequestedAt     time.Time `json:"requested_at,omitempty"`
	PendingSettings []string  `json:"pending_settings,omitempty"`
}

// GetRestartStatus returns the current restart status.
func (m *Manager) GetRestartStatus() RestartStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	settings := make([]string, 0, len(m.pendingSettings))
	for k := range m.pendingSettings {
		settings = append(settings, k)
	}

	return RestartStatus{
		Pending:         m.restartPending || len(m.pendingSettings) > 0,
		Reason:          m.restartReason,
		RequestedAt:     m.restartRequestAt,
		PendingSettings: settings,
	}
}

// ShutdownCh returns a channel that is closed when shutdown is initiated.
func (m *Manager) ShutdownCh() <-chan struct{} {
	return m.shutdownCh
}
