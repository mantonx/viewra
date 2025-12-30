package manager

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func TestManager_LoadPlugin_NoFactory(t *testing.T) {
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

	// Attempt to load without setting plugin factory
	ctx := context.Background()
	_, err = m.LoadPlugin(ctx, "/some/path")
	if err == nil {
		t.Error("LoadPlugin() should fail when plugin factory is not set")
	}
	if err.Error() != "plugin factory not set" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestManager_UnloadPlugin_NotFound(t *testing.T) {
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
	err = m.UnloadPlugin(ctx, "non-existent-plugin")
	if err == nil {
		t.Error("UnloadPlugin() should fail for non-existent plugin")
	}
}
