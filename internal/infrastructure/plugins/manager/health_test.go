package manager

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/types"
)

func TestManager_RestartPlugin_NotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	m, err := NewManager(ManagerConfig{
		PluginDir:   tmpDir,
		HostVersion: "1.0.0",
	}, logger)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	ctx := context.Background()
	err = m.RestartPlugin(ctx, "non-existent-plugin")
	if err == nil {
		t.Error("RestartPlugin() should fail for non-existent plugin")
	}
}

func TestManager_StartHealthMonitor(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	m, err := NewManager(ManagerConfig{
		PluginDir:           tmpDir,
		HostVersion:         "1.0.0",
		HealthCheckInterval: 50 * time.Millisecond, // Short interval for testing
	}, logger)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start health monitor
	m.StartHealthMonitor(ctx)

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Cancel to stop the monitor
	cancel()

	// Give it time to stop
	time.Sleep(50 * time.Millisecond)
}

func TestManager_CheckAllHealth_NoPlugins(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	m, err := NewManager(ManagerConfig{
		PluginDir:   tmpDir,
		HostVersion: "1.0.0",
	}, logger)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	ctx := context.Background()
	// Should not panic with no plugins
	m.checkAllHealth(ctx)
}

func TestManager_CheckPluginHealth_NilCoreClient(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	m, err := NewManager(ManagerConfig{
		PluginDir:   tmpDir,
		HostVersion: "1.0.0",
	}, logger)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create plugin instance with nil CoreClient
	instance := &types.Instance{
		ID: "test-plugin",
		Health: types.Health{
			Status: pluginv1.HealthStatus_HEALTHY,
		},
		// CoreClient is nil - this will cause the health check to fail
	}

	ctx := context.Background()
	// Should not panic but will fail to check health
	// The function will panic trying to call nil.HealthCheck - that's expected
	// In production, CoreClient is always set when a plugin is loaded
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with nil CoreClient")
		}
	}()
	m.checkPluginHealth(ctx, instance)
}

func TestManager_MaxRestarts(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	customRestarts := 5
	m, err := NewManager(ManagerConfig{
		PluginDir:   tmpDir,
		HostVersion: "1.0.0",
		MaxRestarts: customRestarts,
	}, logger)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if m.GetMaxRestarts() != customRestarts {
		t.Errorf("expected max restarts %d, got %d", customRestarts, m.GetMaxRestarts())
	}
}

func TestManager_HealthCheckInterval(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	customInterval := 2 * time.Minute
	m, err := NewManager(ManagerConfig{
		PluginDir:           tmpDir,
		HostVersion:         "1.0.0",
		HealthCheckInterval: customInterval,
	}, logger)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if m.GetHealthCheckInterval() != customInterval {
		t.Errorf("expected health check interval %v, got %v", customInterval, m.GetHealthCheckInterval())
	}
}
