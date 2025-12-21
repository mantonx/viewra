package lifecycle

import (
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/events"
)

// mockPublisher collects published events for testing
type mockPublisher struct {
	mu     sync.Mutex
	events []events.Event
}

func (m *mockPublisher) Publish(event events.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func (m *mockPublisher) getEvents() []events.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]events.Event, len(m.events))
	copy(result, m.events)
	return result
}

func (m *mockPublisher) clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = nil
}

func newTestManager() (*Manager, *mockPublisher) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	publisher := &mockPublisher{}
	manager := NewManager(logger, publisher)
	return manager, publisher
}

func TestNewManager(t *testing.T) {
	manager, _ := newTestManager()

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}

	if manager.HasPendingRestart() {
		t.Error("New manager should not have pending restart")
	}

	status := manager.GetRestartStatus()
	if status.Pending {
		t.Error("New manager status should not show pending")
	}
}

func TestManager_MarkSettingPendingRestart(t *testing.T) {
	manager, publisher := newTestManager()

	// Mark a setting as pending restart
	manager.MarkSettingPendingRestart("database.connection_string")

	if !manager.HasPendingRestart() {
		t.Error("Expected HasPendingRestart to be true after marking setting")
	}

	settings := manager.GetPendingSettings()
	if len(settings) != 1 {
		t.Errorf("Expected 1 pending setting, got %d", len(settings))
	}
	if settings[0] != "database.connection_string" {
		t.Errorf("Expected 'database.connection_string', got %s", settings[0])
	}

	// Check event was published
	evts := publisher.getEvents()
	if len(evts) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(evts))
	}
	if evts[0].Type != events.EventSettingsPendingRestart {
		t.Errorf("Expected EventSettingsPendingRestart, got %s", evts[0].Type)
	}
	if evts[0].Data["setting_key"] != "database.connection_string" {
		t.Errorf("Expected setting_key in event data")
	}
}

func TestManager_MarkMultipleSettings(t *testing.T) {
	manager, _ := newTestManager()

	manager.MarkSettingPendingRestart("setting1")
	manager.MarkSettingPendingRestart("setting2")
	manager.MarkSettingPendingRestart("setting3")

	settings := manager.GetPendingSettings()
	if len(settings) != 3 {
		t.Errorf("Expected 3 pending settings, got %d", len(settings))
	}

	status := manager.GetRestartStatus()
	if len(status.PendingSettings) != 3 {
		t.Errorf("Expected 3 pending settings in status, got %d", len(status.PendingSettings))
	}
}

func TestManager_ClearPendingSettings(t *testing.T) {
	manager, _ := newTestManager()

	manager.MarkSettingPendingRestart("setting1")
	manager.MarkSettingPendingRestart("setting2")

	if !manager.HasPendingRestart() {
		t.Error("Expected pending restart before clear")
	}

	manager.ClearPendingSettings()

	settings := manager.GetPendingSettings()
	if len(settings) != 0 {
		t.Errorf("Expected 0 pending settings after clear, got %d", len(settings))
	}

	// Note: HasPendingRestart may still return true if restartPending is set
	// For this test, we only cleared pendingSettings, not restartPending
}

func TestManager_RequestRestart(t *testing.T) {
	manager, publisher := newTestManager()

	manager.RequestRestart("User requested restart")

	if !manager.HasPendingRestart() {
		t.Error("Expected HasPendingRestart to be true")
	}

	status := manager.GetRestartStatus()
	if !status.Pending {
		t.Error("Expected status.Pending to be true")
	}
	if status.Reason != "User requested restart" {
		t.Errorf("Expected reason 'User requested restart', got %s", status.Reason)
	}
	if status.RequestedAt.IsZero() {
		t.Error("Expected RequestedAt to be set")
	}

	// Check event
	evts := publisher.getEvents()
	found := false
	for _, e := range evts {
		if e.Type == events.EventSystemRestartRequested {
			found = true
			if e.Data["reason"] != "User requested restart" {
				t.Errorf("Expected reason in event data")
			}
		}
	}
	if !found {
		t.Error("Expected EventSystemRestartRequested event")
	}
}

func TestManager_RequestRestart_AlreadyPending(t *testing.T) {
	manager, publisher := newTestManager()

	manager.RequestRestart("First reason")
	publisher.clear()

	// Request another restart while one is pending
	manager.RequestRestart("Second reason")

	// Should still have first reason
	status := manager.GetRestartStatus()
	if status.Reason != "First reason" {
		t.Errorf("Expected first reason to be preserved, got %s", status.Reason)
	}

	// No new event should be published
	evts := publisher.getEvents()
	if len(evts) != 0 {
		t.Error("Expected no new events for duplicate restart request")
	}
}

func TestManager_CancelRestart(t *testing.T) {
	manager, publisher := newTestManager()

	// Request then cancel
	manager.RequestRestart("Temporary restart")
	publisher.clear()

	cancelled := manager.CancelRestart()
	if !cancelled {
		t.Error("Expected CancelRestart to return true")
	}

	status := manager.GetRestartStatus()
	// Note: status.Pending may still be true if pendingSettings is non-empty
	if status.Reason != "" {
		t.Error("Expected reason to be cleared after cancel")
	}

	// Check cancel event
	evts := publisher.getEvents()
	if len(evts) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(evts))
	}
	if evts[0].Type != events.EventSystemRestartCancelled {
		t.Errorf("Expected EventSystemRestartCancelled, got %s", evts[0].Type)
	}
}

func TestManager_CancelRestart_NotPending(t *testing.T) {
	manager, _ := newTestManager()

	cancelled := manager.CancelRestart()
	if cancelled {
		t.Error("Expected CancelRestart to return false when no restart is pending")
	}
}

func TestManager_SetShutdownFunc(t *testing.T) {
	manager, _ := newTestManager()

	manager.SetShutdownFunc(func() {
		// Shutdown function set
	})

	// We can't easily test ExecuteRestart since it calls os.Exit
	// But we can verify the shutdown function was set correctly
	// by checking the internal state
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	if manager.shutdownFn == nil {
		t.Error("Expected shutdown function to be set")
	}
}

func TestManager_GetRestartStatus_Combined(t *testing.T) {
	manager, _ := newTestManager()

	// Mark some settings
	manager.MarkSettingPendingRestart("setting1")
	manager.MarkSettingPendingRestart("setting2")

	// Request restart
	manager.RequestRestart("Apply pending settings")

	status := manager.GetRestartStatus()

	if !status.Pending {
		t.Error("Expected Pending to be true")
	}
	if status.Reason != "Apply pending settings" {
		t.Error("Expected reason to be set")
	}
	if len(status.PendingSettings) != 2 {
		t.Errorf("Expected 2 pending settings, got %d", len(status.PendingSettings))
	}
	if status.RequestedAt.IsZero() {
		t.Error("Expected RequestedAt to be set")
	}
}

func TestManager_ShutdownCh(t *testing.T) {
	manager, _ := newTestManager()

	ch := manager.ShutdownCh()
	if ch == nil {
		t.Error("Expected non-nil shutdown channel")
	}

	// Channel should not be closed initially
	select {
	case <-ch:
		t.Error("Shutdown channel should not be closed initially")
	default:
		// Expected
	}
}

func TestManager_Concurrency(t *testing.T) {
	manager, _ := newTestManager()

	// Concurrent operations
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			// Mix of operations
			switch n % 5 {
			case 0:
				manager.MarkSettingPendingRestart("setting")
			case 1:
				manager.GetPendingSettings()
			case 2:
				manager.HasPendingRestart()
			case 3:
				manager.GetRestartStatus()
			case 4:
				if n%10 == 0 {
					manager.RequestRestart("test")
				}
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Error("Concurrency test timed out - possible deadlock")
	}
}

func TestRestartExitCode(t *testing.T) {
	if RestartExitCode != 42 {
		t.Errorf("Expected RestartExitCode to be 42, got %d", RestartExitCode)
	}
}
