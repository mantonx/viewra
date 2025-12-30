package pool

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockPlugin implements PluginInstance for testing.
type mockPlugin struct {
	id string
}

func (m *mockPlugin) GetID() string {
	return m.id
}

// mockManager implements Manager for testing.
type mockManager struct {
	mu             sync.RWMutex
	plugins        map[string]*mockPlugin
	discoverPaths  []string
	discoverErr    error
	loadErr        error
	unloadErr      error
	loadCalls      int32
	unloadCalls    int32
	discoverCalls  int32
	loadedPluginID string
}

func newMockManager() *mockManager {
	return &mockManager{
		plugins: make(map[string]*mockPlugin),
	}
}

func (m *mockManager) GetPlugin(id string) (PluginInstance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[id]
	if !ok {
		return nil, false
	}
	return p, true
}

func (m *mockManager) ListPlugins() []PluginInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]PluginInstance, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, p)
	}
	return result
}

func (m *mockManager) LoadPlugin(ctx context.Context, path string) (PluginInstance, error) {
	atomic.AddInt32(&m.loadCalls, 1)
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// Extract plugin ID from path (last segment)
	id := m.loadedPluginID
	if id == "" {
		id = path
	}
	p := &mockPlugin{id: id}
	m.plugins[id] = p
	return p, nil
}

func (m *mockManager) UnloadPlugin(ctx context.Context, id string) error {
	atomic.AddInt32(&m.unloadCalls, 1)
	if m.unloadErr != nil {
		return m.unloadErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.plugins, id)
	return nil
}

func (m *mockManager) DiscoverPlugins() ([]string, error) {
	atomic.AddInt32(&m.discoverCalls, 1)
	if m.discoverErr != nil {
		return nil, m.discoverErr
	}
	return m.discoverPaths, nil
}

func (m *mockManager) addPlugin(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins[id] = &mockPlugin{id: id}
}

func (m *mockManager) getLoadCalls() int {
	return int(atomic.LoadInt32(&m.loadCalls))
}

func (m *mockManager) getUnloadCalls() int {
	return int(atomic.LoadInt32(&m.unloadCalls))
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewWarmPool(t *testing.T) {
	t.Run("creates pool with default values", func(t *testing.T) {
		mgr := newMockManager()
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		if pool == nil {
			t.Fatal("expected non-nil pool")
		}
		if pool.warmTimeout != 30*time.Minute {
			t.Errorf("expected default warmTimeout of 30m, got %v", pool.warmTimeout)
		}
		if pool.checkInterval != 1*time.Minute {
			t.Errorf("expected default checkInterval of 1m, got %v", pool.checkInterval)
		}
		if pool.manager != mgr {
			t.Error("manager not set correctly")
		}
	})

	t.Run("creates pool with custom values", func(t *testing.T) {
		mgr := newMockManager()
		cfg := WarmPoolConfig{
			WarmTimeout:   5 * time.Minute,
			CheckInterval: 30 * time.Second,
			KeepWarm:      []string{"plugin-a", "plugin-b"},
		}
		pool := NewWarmPool(mgr, cfg, testLogger())

		if pool.warmTimeout != 5*time.Minute {
			t.Errorf("expected warmTimeout of 5m, got %v", pool.warmTimeout)
		}
		if pool.checkInterval != 30*time.Second {
			t.Errorf("expected checkInterval of 30s, got %v", pool.checkInterval)
		}
		if !pool.keepWarm["plugin-a"] || !pool.keepWarm["plugin-b"] {
			t.Error("keepWarm plugins not set correctly")
		}
	})

	t.Run("initializes empty maps", func(t *testing.T) {
		mgr := newMockManager()
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		if pool.lastAccess == nil {
			t.Error("lastAccess map not initialized")
		}
		if pool.keepWarm == nil {
			t.Error("keepWarm map not initialized")
		}
		if pool.stopCh == nil {
			t.Error("stopCh channel not initialized")
		}
	})
}

func TestWarmPoolStartStop(t *testing.T) {
	t.Run("starts and stops cleanly", func(t *testing.T) {
		mgr := newMockManager()
		pool := NewWarmPool(mgr, WarmPoolConfig{
			CheckInterval: 10 * time.Millisecond,
		}, testLogger())

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pool.Start(ctx)
		time.Sleep(50 * time.Millisecond) // Let it run a few cycles
		pool.Stop()

		// Should not panic on stop
	})

	t.Run("stops on context cancellation", func(t *testing.T) {
		mgr := newMockManager()
		pool := NewWarmPool(mgr, WarmPoolConfig{
			CheckInterval: 10 * time.Millisecond,
		}, testLogger())

		ctx, cancel := context.WithCancel(context.Background())

		pool.Start(ctx)
		time.Sleep(30 * time.Millisecond)
		cancel()
		time.Sleep(30 * time.Millisecond)

		// Should not hang or panic
	})

	t.Run("checkIdlePlugins is called periodically", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("idle-plugin")

		pool := NewWarmPool(mgr, WarmPoolConfig{
			CheckInterval: 20 * time.Millisecond,
			WarmTimeout:   1 * time.Millisecond, // Very short for testing
		}, testLogger())

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pool.Start(ctx)
		time.Sleep(100 * time.Millisecond)
		pool.Stop()

		// The idle plugin should have been unloaded
		if mgr.getUnloadCalls() == 0 {
			t.Error("expected checkIdlePlugins to unload idle plugins")
		}
	})
}

func TestWarmPoolTouch(t *testing.T) {
	t.Run("updates last access time", func(t *testing.T) {
		mgr := newMockManager()
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		before := time.Now()
		pool.Touch("test-plugin")
		after := time.Now()

		pool.mu.RLock()
		lastAccess, ok := pool.lastAccess["test-plugin"]
		pool.mu.RUnlock()

		if !ok {
			t.Fatal("lastAccess not set")
		}
		if lastAccess.Before(before) || lastAccess.After(after) {
			t.Error("lastAccess time not in expected range")
		}
	})

	t.Run("overwrites previous access time", func(t *testing.T) {
		mgr := newMockManager()
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		pool.Touch("test-plugin")
		time.Sleep(10 * time.Millisecond)

		before := time.Now()
		pool.Touch("test-plugin")

		pool.mu.RLock()
		lastAccess := pool.lastAccess["test-plugin"]
		pool.mu.RUnlock()

		if lastAccess.Before(before) {
			t.Error("lastAccess not updated on second touch")
		}
	})
}

func TestWarmPoolSetKeepWarm(t *testing.T) {
	t.Run("adds plugin to keepWarm", func(t *testing.T) {
		mgr := newMockManager()
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		pool.SetKeepWarm("my-plugin", true)

		pool.mu.RLock()
		isKept := pool.keepWarm["my-plugin"]
		pool.mu.RUnlock()

		if !isKept {
			t.Error("plugin not marked as keepWarm")
		}
	})

	t.Run("removes plugin from keepWarm", func(t *testing.T) {
		mgr := newMockManager()
		pool := NewWarmPool(mgr, WarmPoolConfig{
			KeepWarm: []string{"my-plugin"},
		}, testLogger())

		pool.SetKeepWarm("my-plugin", false)

		pool.mu.RLock()
		_, exists := pool.keepWarm["my-plugin"]
		pool.mu.RUnlock()

		if exists {
			t.Error("plugin should have been removed from keepWarm")
		}
	})
}

func TestWarmPoolIsWarm(t *testing.T) {
	t.Run("returns true for loaded plugin", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("loaded-plugin")
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		if !pool.IsWarm("loaded-plugin") {
			t.Error("expected IsWarm to return true for loaded plugin")
		}
	})

	t.Run("returns false for unloaded plugin", func(t *testing.T) {
		mgr := newMockManager()
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		if pool.IsWarm("unloaded-plugin") {
			t.Error("expected IsWarm to return false for unloaded plugin")
		}
	})
}

func TestWarmPoolGetWarmPlugins(t *testing.T) {
	t.Run("returns empty list when no plugins", func(t *testing.T) {
		mgr := newMockManager()
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		plugins := pool.GetWarmPlugins()
		if len(plugins) != 0 {
			t.Errorf("expected empty list, got %d plugins", len(plugins))
		}
	})

	t.Run("returns all warm plugin IDs", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("plugin-a")
		mgr.addPlugin("plugin-b")
		mgr.addPlugin("plugin-c")
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		plugins := pool.GetWarmPlugins()
		if len(plugins) != 3 {
			t.Errorf("expected 3 plugins, got %d", len(plugins))
		}

		// Check all plugins are present
		found := make(map[string]bool)
		for _, id := range plugins {
			found[id] = true
		}
		for _, expected := range []string{"plugin-a", "plugin-b", "plugin-c"} {
			if !found[expected] {
				t.Errorf("expected plugin %s not found", expected)
			}
		}
	})
}

func TestWarmPoolCheckIdlePlugins(t *testing.T) {
	t.Run("unloads idle plugins", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("idle-plugin")
		pool := NewWarmPool(mgr, WarmPoolConfig{
			WarmTimeout: 1 * time.Millisecond,
		}, testLogger())

		// Set old access time
		pool.mu.Lock()
		pool.lastAccess["idle-plugin"] = time.Now().Add(-1 * time.Hour)
		pool.mu.Unlock()

		ctx := context.Background()
		pool.checkIdlePlugins(ctx)

		if mgr.getUnloadCalls() != 1 {
			t.Errorf("expected 1 unload call, got %d", mgr.getUnloadCalls())
		}

		if _, ok := mgr.GetPlugin("idle-plugin"); ok {
			t.Error("idle plugin should have been unloaded")
		}
	})

	t.Run("keeps keepWarm plugins", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("keep-warm-plugin")
		pool := NewWarmPool(mgr, WarmPoolConfig{
			WarmTimeout: 1 * time.Millisecond,
			KeepWarm:    []string{"keep-warm-plugin"},
		}, testLogger())

		// Set old access time
		pool.mu.Lock()
		pool.lastAccess["keep-warm-plugin"] = time.Now().Add(-1 * time.Hour)
		pool.mu.Unlock()

		ctx := context.Background()
		pool.checkIdlePlugins(ctx)

		if mgr.getUnloadCalls() != 0 {
			t.Error("keepWarm plugin should not be unloaded")
		}

		if _, ok := mgr.GetPlugin("keep-warm-plugin"); !ok {
			t.Error("keepWarm plugin should still be loaded")
		}
	})

	t.Run("keeps recently accessed plugins", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("recent-plugin")
		pool := NewWarmPool(mgr, WarmPoolConfig{
			WarmTimeout: 1 * time.Hour,
		}, testLogger())

		pool.Touch("recent-plugin")

		ctx := context.Background()
		pool.checkIdlePlugins(ctx)

		if mgr.getUnloadCalls() != 0 {
			t.Error("recently accessed plugin should not be unloaded")
		}
	})

	t.Run("handles unload errors gracefully", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("error-plugin")
		mgr.unloadErr = errors.New("unload failed")
		pool := NewWarmPool(mgr, WarmPoolConfig{
			WarmTimeout: 1 * time.Millisecond,
		}, testLogger())

		pool.mu.Lock()
		pool.lastAccess["error-plugin"] = time.Now().Add(-1 * time.Hour)
		pool.mu.Unlock()

		ctx := context.Background()
		pool.checkIdlePlugins(ctx) // Should not panic

		// Plugin should still be in lastAccess since unload failed
		pool.mu.RLock()
		_, exists := pool.lastAccess["error-plugin"]
		pool.mu.RUnlock()
		if !exists {
			t.Error("lastAccess should not be cleared on unload error")
		}
	})

	t.Run("sets default access time for new plugins", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("new-plugin")
		pool := NewWarmPool(mgr, WarmPoolConfig{
			WarmTimeout: 1 * time.Hour,
		}, testLogger())

		ctx := context.Background()
		pool.checkIdlePlugins(ctx)

		pool.mu.RLock()
		_, exists := pool.lastAccess["new-plugin"]
		pool.mu.RUnlock()

		if !exists {
			t.Error("expected lastAccess to be set for new plugin")
		}
	})
}

func TestWarmPoolGetOrLoadEnricher(t *testing.T) {
	t.Run("returns already loaded plugin", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("existing-plugin")
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		ctx := context.Background()
		instance, err := pool.GetOrLoadEnricher(ctx, "existing-plugin")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if instance == nil {
			t.Fatal("expected non-nil instance")
		}
		if instance.GetID() != "existing-plugin" {
			t.Errorf("expected ID 'existing-plugin', got %s", instance.GetID())
		}
		if mgr.getLoadCalls() != 0 {
			t.Error("should not have called LoadPlugin for existing plugin")
		}
	})

	t.Run("loads plugin from discovered paths", func(t *testing.T) {
		mgr := newMockManager()
		mgr.discoverPaths = []string{"/plugins/my-enricher"}
		mgr.loadedPluginID = "my-enricher"
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		ctx := context.Background()
		instance, err := pool.GetOrLoadEnricher(ctx, "my-enricher")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if instance == nil {
			t.Fatal("expected non-nil instance")
		}
		if mgr.getLoadCalls() != 1 {
			t.Errorf("expected 1 load call, got %d", mgr.getLoadCalls())
		}
	})

	t.Run("touches plugin after load", func(t *testing.T) {
		mgr := newMockManager()
		mgr.discoverPaths = []string{"/plugins/touch-test"}
		mgr.loadedPluginID = "touch-test"
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		ctx := context.Background()
		before := time.Now()
		_, err := pool.GetOrLoadEnricher(ctx, "touch-test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		pool.mu.RLock()
		lastAccess, ok := pool.lastAccess["touch-test"]
		pool.mu.RUnlock()

		if !ok {
			t.Error("expected lastAccess to be set")
		}
		if lastAccess.Before(before) {
			t.Error("lastAccess should be updated after load")
		}
	})

	t.Run("returns error when plugin not found", func(t *testing.T) {
		mgr := newMockManager()
		mgr.discoverPaths = []string{"/plugins/other-plugin"}
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		ctx := context.Background()
		_, err := pool.GetOrLoadEnricher(ctx, "nonexistent")

		if err == nil {
			t.Fatal("expected error for nonexistent plugin")
		}

		var notFoundErr *PluginNotFoundError
		if !errors.As(err, &notFoundErr) {
			t.Errorf("expected PluginNotFoundError, got %T", err)
		}
		if notFoundErr.PluginID != "nonexistent" {
			t.Errorf("expected plugin ID 'nonexistent', got %s", notFoundErr.PluginID)
		}
	})

	t.Run("returns error when discover fails", func(t *testing.T) {
		mgr := newMockManager()
		mgr.discoverErr = errors.New("discover failed")
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		ctx := context.Background()
		_, err := pool.GetOrLoadEnricher(ctx, "any-plugin")

		if err == nil {
			t.Fatal("expected error when discover fails")
		}
		if err.Error() != "discover failed" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("returns error when load fails", func(t *testing.T) {
		mgr := newMockManager()
		mgr.discoverPaths = []string{"/plugins/fail-plugin"}
		mgr.loadErr = errors.New("load failed")
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		ctx := context.Background()
		_, err := pool.GetOrLoadEnricher(ctx, "fail-plugin")

		if err == nil {
			t.Fatal("expected error when load fails")
		}
		if err.Error() != "load failed" {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestWarmPoolPrewarm(t *testing.T) {
	t.Run("prewarns multiple plugins", func(t *testing.T) {
		mgr := newMockManager()
		mgr.discoverPaths = []string{"/plugins/plugin-a", "/plugins/plugin-b"}
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		ctx := context.Background()
		err := pool.Prewarm(ctx, []string{"plugin-a", "plugin-b"})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("skips already loaded plugins", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("existing")
		mgr.discoverPaths = []string{"/plugins/new-plugin"}
		mgr.loadedPluginID = "new-plugin"
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		ctx := context.Background()
		err := pool.Prewarm(ctx, []string{"existing", "new-plugin"})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should only load new-plugin, not existing
		if mgr.getLoadCalls() != 1 {
			t.Errorf("expected 1 load call, got %d", mgr.getLoadCalls())
		}
	})

	t.Run("touches prewarmed plugins", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("existing")
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		before := time.Now()
		ctx := context.Background()
		_ = pool.Prewarm(ctx, []string{"existing"})

		pool.mu.RLock()
		lastAccess, ok := pool.lastAccess["existing"]
		pool.mu.RUnlock()

		if !ok {
			t.Error("expected lastAccess to be set")
		}
		if lastAccess.Before(before) {
			t.Error("lastAccess should be updated")
		}
	})
}

func TestWarmPoolStats(t *testing.T) {
	t.Run("returns correct warm count", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("plugin-a")
		mgr.addPlugin("plugin-b")
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		stats := pool.Stats()
		if stats.WarmCount != 2 {
			t.Errorf("expected WarmCount 2, got %d", stats.WarmCount)
		}
	})

	t.Run("returns correct keepWarm count", func(t *testing.T) {
		mgr := newMockManager()
		pool := NewWarmPool(mgr, WarmPoolConfig{
			KeepWarm: []string{"a", "b", "c"},
		}, testLogger())

		stats := pool.Stats()
		if stats.KeepWarmCount != 3 {
			t.Errorf("expected KeepWarmCount 3, got %d", stats.KeepWarmCount)
		}
	})

	t.Run("returns correct idle count", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("active")
		mgr.addPlugin("idle")
		pool := NewWarmPool(mgr, WarmPoolConfig{
			WarmTimeout: 10 * time.Second,
		}, testLogger())

		// Mark one as active
		pool.Touch("active")

		// Mark one as idle (old access time)
		pool.mu.Lock()
		pool.lastAccess["idle"] = time.Now().Add(-1 * time.Hour)
		pool.mu.Unlock()

		stats := pool.Stats()
		if stats.IdleCount != 1 {
			t.Errorf("expected IdleCount 1, got %d", stats.IdleCount)
		}
	})

	t.Run("excludes keepWarm from idle count", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("keep-warm")
		pool := NewWarmPool(mgr, WarmPoolConfig{
			WarmTimeout: 10 * time.Second,
			KeepWarm:    []string{"keep-warm"},
		}, testLogger())

		// Set old access time
		pool.mu.Lock()
		pool.lastAccess["keep-warm"] = time.Now().Add(-1 * time.Hour)
		pool.mu.Unlock()

		stats := pool.Stats()
		if stats.IdleCount != 0 {
			t.Errorf("expected IdleCount 0 (keepWarm excluded), got %d", stats.IdleCount)
		}
	})
}

func TestPluginNotFoundError(t *testing.T) {
	t.Run("returns correct error message", func(t *testing.T) {
		err := &PluginNotFoundError{PluginID: "my-plugin"}
		expected := "plugin not found: my-plugin"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})
}

func TestContainsPluginID(t *testing.T) {
	tests := []struct {
		path     string
		pluginID string
		expected bool
	}{
		{"/plugins/tmdb", "tmdb", true},
		{"/plugins/tmdb/tmdb", "tmdb", true},
		{"tmdb", "tmdb", true},
		{"/plugins/other", "tmdb", false},
		{"/plugins/tmdb-extra", "tmdb", false},
		{"", "tmdb", false},
		{"/plugins/my-plugin", "my-plugin", true},
	}

	for _, tt := range tests {
		t.Run(tt.path+"_"+tt.pluginID, func(t *testing.T) {
			result := containsPluginID(tt.path, tt.pluginID)
			if result != tt.expected {
				t.Errorf("containsPluginID(%q, %q) = %v, expected %v",
					tt.path, tt.pluginID, result, tt.expected)
			}
		})
	}
}

func TestWarmPoolConcurrentAccess(t *testing.T) {
	t.Run("handles concurrent Touch calls", func(t *testing.T) {
		mgr := newMockManager()
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				pool.Touch("plugin-concurrent")
			}(i)
		}
		wg.Wait()

		pool.mu.RLock()
		_, ok := pool.lastAccess["plugin-concurrent"]
		pool.mu.RUnlock()
		if !ok {
			t.Error("expected lastAccess to be set")
		}
	})

	t.Run("handles concurrent SetKeepWarm calls", func(t *testing.T) {
		mgr := newMockManager()
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(2)
			go func() {
				defer wg.Done()
				pool.SetKeepWarm("plugin", true)
			}()
			go func() {
				defer wg.Done()
				pool.SetKeepWarm("plugin", false)
			}()
		}
		wg.Wait()
		// Should not panic
	})

	t.Run("handles concurrent Stats calls", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("plugin-a")
		mgr.addPlugin("plugin-b")
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = pool.Stats()
			}()
		}
		wg.Wait()
		// Should not panic
	})

	t.Run("handles concurrent GetOrLoadEnricher calls", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("concurrent-plugin")
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		var wg sync.WaitGroup
		errCh := make(chan error, 100)

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx := context.Background()
				_, err := pool.GetOrLoadEnricher(ctx, "concurrent-plugin")
				if err != nil {
					errCh <- err
				}
			}()
		}
		wg.Wait()
		close(errCh)

		for err := range errCh {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handles concurrent Touch and checkIdlePlugins", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("concurrent-plugin")
		pool := NewWarmPool(mgr, WarmPoolConfig{
			WarmTimeout: 1 * time.Hour, // Long timeout to avoid unloading
		}, testLogger())

		ctx := context.Background()
		var wg sync.WaitGroup

		// Concurrent touches
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				pool.Touch("concurrent-plugin")
			}()
		}

		// Concurrent checkIdlePlugins
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				pool.checkIdlePlugins(ctx)
			}()
		}

		wg.Wait()
		// Should not panic or deadlock
	})
}

func TestWarmPoolPoolSizeLimits(t *testing.T) {
	t.Run("tracks all loaded plugins", func(t *testing.T) {
		mgr := newMockManager()
		for i := 0; i < 10; i++ {
			mgr.addPlugin("plugin-" + string(rune('a'+i)))
		}
		pool := NewWarmPool(mgr, WarmPoolConfig{}, testLogger())

		plugins := pool.GetWarmPlugins()
		if len(plugins) != 10 {
			t.Errorf("expected 10 warm plugins, got %d", len(plugins))
		}
	})

	t.Run("unloads plugins when idle beyond timeout", func(t *testing.T) {
		mgr := newMockManager()
		// Add many plugins
		for i := 0; i < 5; i++ {
			mgr.addPlugin("plugin-" + string(rune('a'+i)))
		}

		pool := NewWarmPool(mgr, WarmPoolConfig{
			WarmTimeout: 1 * time.Millisecond,
		}, testLogger())

		// Mark all as idle
		pool.mu.Lock()
		for _, p := range mgr.plugins {
			pool.lastAccess[p.id] = time.Now().Add(-1 * time.Hour)
		}
		pool.mu.Unlock()

		ctx := context.Background()
		pool.checkIdlePlugins(ctx)

		// All should be unloaded
		plugins := pool.GetWarmPlugins()
		if len(plugins) != 0 {
			t.Errorf("expected 0 warm plugins after unloading idle, got %d", len(plugins))
		}
	})

	t.Run("respects keepWarm limit", func(t *testing.T) {
		mgr := newMockManager()
		mgr.addPlugin("keep-a")
		mgr.addPlugin("keep-b")
		mgr.addPlugin("idle-c")

		pool := NewWarmPool(mgr, WarmPoolConfig{
			WarmTimeout: 1 * time.Millisecond,
			KeepWarm:    []string{"keep-a", "keep-b"},
		}, testLogger())

		// Mark all as idle
		pool.mu.Lock()
		for _, p := range mgr.plugins {
			pool.lastAccess[p.id] = time.Now().Add(-1 * time.Hour)
		}
		pool.mu.Unlock()

		ctx := context.Background()
		pool.checkIdlePlugins(ctx)

		// Only keepWarm plugins should remain
		plugins := pool.GetWarmPlugins()
		if len(plugins) != 2 {
			t.Errorf("expected 2 keepWarm plugins, got %d", len(plugins))
		}
	})
}
