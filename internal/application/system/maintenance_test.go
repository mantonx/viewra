package system

import (
	"sync"
	"testing"
	"time"
)

func TestMaintenanceManager_InitialState(t *testing.T) {
	mgr := NewMaintenanceManager()
	state := mgr.GetState()

	if state.Enabled {
		t.Error("Expected maintenance mode to be disabled initially")
	}
	if state.Reason != "" {
		t.Error("Expected empty reason initially")
	}
	if state.StartedAt != nil {
		t.Error("Expected nil StartedAt initially")
	}
}

func TestMaintenanceManager_Enable(t *testing.T) {
	mgr := NewMaintenanceManager()

	state := mgr.Enable("Database migration", 30*time.Minute)

	if !state.Enabled {
		t.Error("Expected maintenance mode to be enabled")
	}
	if state.Reason != "Database migration" {
		t.Errorf("Expected reason 'Database migration', got %q", state.Reason)
	}
	if state.StartedAt == nil {
		t.Error("Expected StartedAt to be set")
	}
	if state.EstimatedEnd == nil {
		t.Error("Expected EstimatedEnd to be set")
	}

	// Verify IsEnabled
	if !mgr.IsEnabled() {
		t.Error("Expected IsEnabled to return true")
	}
}

func TestMaintenanceManager_EnableWithoutDuration(t *testing.T) {
	mgr := NewMaintenanceManager()

	state := mgr.Enable("Manual maintenance", 0)

	if !state.Enabled {
		t.Error("Expected maintenance mode to be enabled")
	}
	if state.EstimatedEnd != nil {
		t.Error("Expected EstimatedEnd to be nil when no duration provided")
	}
}

func TestMaintenanceManager_Disable(t *testing.T) {
	mgr := NewMaintenanceManager()

	mgr.Enable("Test", time.Hour)
	state := mgr.Disable()

	if state.Enabled {
		t.Error("Expected maintenance mode to be disabled")
	}
	if state.Reason != "" {
		t.Error("Expected reason to be cleared")
	}
	if state.StartedAt != nil {
		t.Error("Expected StartedAt to be cleared")
	}
}

func TestMaintenanceManager_UpdateReason(t *testing.T) {
	mgr := NewMaintenanceManager()

	mgr.Enable("Initial reason", time.Hour)
	state := mgr.UpdateReason("Updated reason")

	if state.Reason != "Updated reason" {
		t.Errorf("Expected reason 'Updated reason', got %q", state.Reason)
	}
}

func TestMaintenanceManager_UpdateReasonWhenDisabled(t *testing.T) {
	mgr := NewMaintenanceManager()

	// Should have no effect when disabled
	state := mgr.UpdateReason("Should not update")

	if state.Enabled {
		t.Error("Expected maintenance mode to remain disabled")
	}
	if state.Reason != "" {
		t.Error("Expected reason to remain empty")
	}
}

func TestMaintenanceManager_OnChange(t *testing.T) {
	mgr := NewMaintenanceManager()

	var called bool
	var wg sync.WaitGroup
	wg.Add(1)

	mgr.SetOnChange(func(state MaintenanceState) {
		called = true
		wg.Done()
	})

	mgr.Enable("Test", 0)

	// Wait for callback with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if !called {
			t.Error("Expected onChange callback to be called")
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for onChange callback")
	}
}

func TestMaintenanceManager_Concurrency(t *testing.T) {
	mgr := NewMaintenanceManager()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				mgr.Enable("Test", time.Hour)
			} else {
				mgr.Disable()
			}
			_ = mgr.GetState()
			_ = mgr.IsEnabled()
		}(i)
	}
	wg.Wait()
}
