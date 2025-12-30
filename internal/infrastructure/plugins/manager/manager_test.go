package manager

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/manifest"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/types"
)

func TestNewManager(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name    string
		cfg     ManagerConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: ManagerConfig{
				PluginDir:   filepath.Join(tmpDir, "plugins"),
				StorageDir:  filepath.Join(tmpDir, "storage"),
				HostVersion: "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "missing plugin dir",
			cfg: ManagerConfig{
				StorageDir:  filepath.Join(tmpDir, "storage"),
				HostVersion: "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "default health check interval",
			cfg: ManagerConfig{
				PluginDir:   filepath.Join(tmpDir, "plugins2"),
				HostVersion: "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "custom health check interval",
			cfg: ManagerConfig{
				PluginDir:           filepath.Join(tmpDir, "plugins3"),
				HostVersion:         "1.0.0",
				HealthCheckInterval: 60 * time.Second,
				MaxRestarts:         5,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewManager(tt.cfg, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if m == nil {
					t.Error("NewManager() returned nil manager")
					return
				}

				// Verify defaults are applied
				if tt.cfg.HealthCheckInterval == 0 && m.healthCheckInterval != 30*time.Second {
					t.Errorf("expected default health check interval 30s, got %v", m.healthCheckInterval)
				}
				if tt.cfg.MaxRestarts == 0 && m.maxRestarts != 3 {
					t.Errorf("expected default max restarts 3, got %d", m.maxRestarts)
				}

				// Verify custom values are applied
				if tt.cfg.HealthCheckInterval != 0 && m.healthCheckInterval != tt.cfg.HealthCheckInterval {
					t.Errorf("expected health check interval %v, got %v", tt.cfg.HealthCheckInterval, m.healthCheckInterval)
				}
				if tt.cfg.MaxRestarts != 0 && m.maxRestarts != tt.cfg.MaxRestarts {
					t.Errorf("expected max restarts %d, got %d", tt.cfg.MaxRestarts, m.maxRestarts)
				}

				// Verify registries are initialized
				if m.routeRegistry == nil {
					t.Error("route registry not initialized")
				}
				if m.capabilityRegistry == nil {
					t.Error("capability registry not initialized")
				}
				if m.providerRegistry == nil {
					t.Error("provider registry not initialized")
				}
				if m.rateLimiter == nil {
					t.Error("rate limiter not initialized")
				}
			}
		})
	}
}

func TestManager_DiscoverPlugins(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create manager
	m, err := NewManager(ManagerConfig{
		PluginDir:   tmpDir,
		HostVersion: "1.0.0",
	}, logger)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Initially empty
	plugins, err := m.DiscoverPlugins()
	if err != nil {
		t.Fatalf("DiscoverPlugins() error = %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}

	// Create a directory-based plugin (plugin-name/plugin-name)
	pluginDir := filepath.Join(tmpDir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}
	pluginBinary := filepath.Join(pluginDir, "test-plugin")
	if err := os.WriteFile(pluginBinary, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("failed to create plugin binary: %v", err)
	}

	// Create a direct binary plugin
	directBinary := filepath.Join(tmpDir, "direct-plugin")
	if err := os.WriteFile(directBinary, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("failed to create direct binary: %v", err)
	}

	// Discover again
	plugins, err = m.DiscoverPlugins()
	if err != nil {
		t.Fatalf("DiscoverPlugins() error = %v", err)
	}
	if len(plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(plugins))
	}
}

func TestManager_GetPlugin(t *testing.T) {
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

	// Register a test plugin
	testInstance := &types.Instance{
		ID: "test-plugin",
		Health: types.Health{
			Status: pluginv1.HealthStatus_HEALTHY,
		},
	}
	m.RegisterPlugin("test-plugin", testInstance)

	// Test GetPlugin
	instance, ok := m.GetPlugin("test-plugin")
	if !ok {
		t.Error("GetPlugin() returned false for existing plugin")
	}
	if instance.ID != "test-plugin" {
		t.Errorf("GetPlugin() returned wrong plugin ID: %s", instance.ID)
	}

	// Test non-existent plugin
	_, ok = m.GetPlugin("non-existent")
	if ok {
		t.Error("GetPlugin() returned true for non-existent plugin")
	}
}

func TestManager_IsPluginEnabled(t *testing.T) {
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

	// Register healthy plugin
	healthyInstance := &types.Instance{
		ID: "healthy-plugin",
		Health: types.Health{
			Status: pluginv1.HealthStatus_HEALTHY,
		},
	}
	m.RegisterPlugin("healthy-plugin", healthyInstance)

	// Register unhealthy plugin
	unhealthyInstance := &types.Instance{
		ID: "unhealthy-plugin",
		Health: types.Health{
			Status: pluginv1.HealthStatus_UNHEALTHY,
		},
	}
	m.RegisterPlugin("unhealthy-plugin", unhealthyInstance)

	tests := []struct {
		pluginID string
		want     bool
	}{
		{"healthy-plugin", true},
		{"unhealthy-plugin", false},
		{"non-existent", false},
	}

	for _, tt := range tests {
		t.Run(tt.pluginID, func(t *testing.T) {
			if got := m.IsPluginEnabled(tt.pluginID); got != tt.want {
				t.Errorf("IsPluginEnabled(%s) = %v, want %v", tt.pluginID, got, tt.want)
			}
		})
	}
}

func TestManager_ListPlugins(t *testing.T) {
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

	// Initially empty
	plugins := m.ListPlugins()
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}

	// Register some plugins
	for i := 0; i < 3; i++ {
		m.RegisterPlugin("plugin-"+string(rune('a'+i)), &types.Instance{
			ID: "plugin-" + string(rune('a'+i)),
		})
	}

	plugins = m.ListPlugins()
	if len(plugins) != 3 {
		t.Errorf("expected 3 plugins, got %d", len(plugins))
	}
}

func TestManager_GetEnrichers(t *testing.T) {
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

	// Register enricher plugin (needs both category and client)
	// Note: We can't easily mock EnricherClient, so we test the category check
	enricherInstance := &types.Instance{
		ID:         "enricher-plugin",
		Categories: []types.Category{types.CategoryEnricher},
		// EnricherClient would be set in real scenarios
	}
	m.RegisterPlugin("enricher-plugin", enricherInstance)

	// Register non-enricher plugin
	otherInstance := &types.Instance{
		ID:         "other-plugin",
		Categories: []types.Category{types.CategoryProvider},
	}
	m.RegisterPlugin("other-plugin", otherInstance)

	enrichers := m.GetEnrichers()
	// Will be 0 because EnricherClient is nil
	if len(enrichers) != 0 {
		t.Errorf("expected 0 enrichers (no client set), got %d", len(enrichers))
	}
}

func TestManager_PrintTable(t *testing.T) {
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

	var buf bytes.Buffer

	// Test empty plugins
	m.PrintTable(&buf, "Test Title")
	if !bytes.Contains(buf.Bytes(), []byte("No plugins loaded")) {
		t.Error("expected 'No plugins loaded' message for empty plugins")
	}

	buf.Reset()

	// Register a plugin with manifest
	m.RegisterPlugin("test-plugin", &types.Instance{
		ID: "test-plugin",
		Manifest: &manifest.Manifest{
			Name:    "Test Plugin",
			Version: "1.0.0",
		},
		Categories: []types.Category{types.CategoryEnricher},
		Health: types.Health{
			Status: pluginv1.HealthStatus_HEALTHY,
		},
		BuildTime: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
	})

	m.PrintTable(&buf, "Loaded Plugins")
	output := buf.String()

	// Check for expected content
	if !bytes.Contains([]byte(output), []byte("Loaded Plugins")) {
		t.Error("expected title in output")
	}
	if !bytes.Contains([]byte(output), []byte("Test Plugin")) {
		t.Error("expected plugin name in output")
	}
	if !bytes.Contains([]byte(output), []byte("1.0.0")) {
		t.Error("expected version in output")
	}
}

func TestManager_RegisterUnregister(t *testing.T) {
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

	// Register
	instance := &types.Instance{ID: "test-plugin"}
	m.RegisterPlugin("test-plugin", instance)

	if _, ok := m.GetPlugin("test-plugin"); !ok {
		t.Error("plugin not found after registration")
	}

	// Unregister
	m.UnregisterPlugin("test-plugin")

	if _, ok := m.GetPlugin("test-plugin"); ok {
		t.Error("plugin still found after unregistration")
	}
}

func TestManager_Getters(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	m, err := NewManager(ManagerConfig{
		PluginDir:           filepath.Join(tmpDir, "plugins"),
		StorageDir:          filepath.Join(tmpDir, "storage"),
		HostVersion:         "1.2.3",
		HealthCheckInterval: 45 * time.Second,
		MaxRestarts:         7,
	}, logger)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Test all getters
	if m.GetPluginDir() != filepath.Join(tmpDir, "plugins") {
		t.Errorf("GetPluginDir() = %s, want %s", m.GetPluginDir(), filepath.Join(tmpDir, "plugins"))
	}
	if m.GetStorageDir() != filepath.Join(tmpDir, "storage") {
		t.Errorf("GetStorageDir() = %s, want %s", m.GetStorageDir(), filepath.Join(tmpDir, "storage"))
	}
	if m.GetHostVersion() != "1.2.3" {
		t.Errorf("GetHostVersion() = %s, want 1.2.3", m.GetHostVersion())
	}
	if m.GetHealthCheckInterval() != 45*time.Second {
		t.Errorf("GetHealthCheckInterval() = %v, want 45s", m.GetHealthCheckInterval())
	}
	if m.GetMaxRestarts() != 7 {
		t.Errorf("GetMaxRestarts() = %d, want 7", m.GetMaxRestarts())
	}
	if m.GetLogger() != logger {
		t.Error("GetLogger() returned different logger")
	}
	if m.GetRouteRegistry() == nil {
		t.Error("GetRouteRegistry() returned nil")
	}
	if m.GetCapabilityRegistry() == nil {
		t.Error("GetCapabilityRegistry() returned nil")
	}
	if m.GetProviderRegistry() == nil {
		t.Error("GetProviderRegistry() returned nil")
	}
	if m.GetRateLimiter() == nil {
		t.Error("GetRateLimiter() returned nil")
	}

	// Test nil getters for unset values
	if m.GetHostDataServer() != nil {
		t.Error("GetHostDataServer() should be nil initially")
	}
	if m.GetHostStorageServer() != nil {
		t.Error("GetHostStorageServer() should be nil initially")
	}
	if m.GetHostWeatherServer() != nil {
		t.Error("GetHostWeatherServer() should be nil initially")
	}
	if m.GetHostPluginsServer() != nil {
		t.Error("GetHostPluginsServer() should be nil initially")
	}
	if m.GetHTTPProxy() != nil {
		t.Error("GetHTTPProxy() should be nil initially")
	}
	if m.GetPublisher() != nil {
		t.Error("GetPublisher() should be nil initially")
	}
	if m.GetSystemInfo() != nil {
		t.Error("GetSystemInfo() should be nil initially")
	}
}

func TestManager_Setters(t *testing.T) {
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

	// Test SetSystemInfo
	sysInfo := &pluginv1.SystemInfo{
		CpuCores: 8,
		RamBytes: 16 * 1024 * 1024 * 1024,
	}
	m.SetSystemInfo(sysInfo)
	if m.GetSystemInfo() != sysInfo {
		t.Error("SetSystemInfo() did not set value correctly")
	}

	// Test SetHostDataServer
	m.SetHostDataServer(nil) // Can't easily mock, just test it doesn't panic

	// Test SetHostPluginsServer
	m.SetHostPluginsServer(nil)

	// Test SetHTTPProxy
	m.SetHTTPProxy(nil)

	// Test SetPluginFactory
	m.SetPluginFactory(nil)
}

func TestManager_ConcurrentAccess(t *testing.T) {
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

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent registrations
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			id := "plugin-" + string(rune('a'+(idx%26)))
			m.RegisterPlugin(id, &types.Instance{ID: id})
		}(i)
	}
	wg.Wait()

	// Concurrent reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			_ = m.ListPlugins()
			_ = m.GetAllPlugins()
			id := "plugin-" + string(rune('a'+(idx%26)))
			_, _ = m.GetPlugin(id)
			_ = m.IsPluginEnabled(id)
		}(i)
	}
	wg.Wait()

	// Concurrent unregistrations
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			id := "plugin-" + string(rune('a'+(idx%26)))
			m.UnregisterPlugin(id)
		}(i)
	}
	wg.Wait()

	// All plugins should be unregistered
	plugins := m.ListPlugins()
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins after unregistration, got %d", len(plugins))
	}
}

func TestManager_Shutdown(t *testing.T) {
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

	// Note: Can't fully test shutdown without real plugins, but we can verify it doesn't panic
	ctx := context.Background()
	m.Shutdown(ctx)

	// Verify plugins map is empty
	plugins := m.ListPlugins()
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins after shutdown, got %d", len(plugins))
	}
}

func TestHandshake(t *testing.T) {
	t.Parallel()

	handshake := Handshake()
	if handshake.ProtocolVersion == 0 {
		t.Error("Handshake() returned zero protocol version")
	}
	if handshake.MagicCookieKey == "" {
		t.Error("Handshake() returned empty magic cookie key")
	}
	if handshake.MagicCookieValue == "" {
		t.Error("Handshake() returned empty magic cookie value")
	}
}

func TestIsExecutable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mode     os.FileMode
		expected bool
	}{
		{"no exec", 0644, false},
		{"user exec", 0755, true},
		{"group exec", 0754, true},
		{"other exec", 0745, true},
		{"all exec", 0777, true},
		{"only exec", 0111, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "test")
			if err := os.WriteFile(tmpFile, []byte(""), tt.mode); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}
			info, err := os.Stat(tmpFile)
			if err != nil {
				t.Fatalf("failed to stat test file: %v", err)
			}
			if got := isExecutable(info); got != tt.expected {
				t.Errorf("isExecutable() = %v, want %v for mode %o", got, tt.expected, tt.mode)
			}
		})
	}
}
