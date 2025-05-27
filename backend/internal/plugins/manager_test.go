package plugins

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Test helper to create a temporary directory
func createTempDir(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "viewra-plugin-test-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})
	return tmpDir
}

// Test helper to create a test database
func createTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	
	// Create plugins table for testing
	err = db.Exec(`
		CREATE TABLE plugins (
			id INTEGER PRIMARY KEY,
			plugin_id TEXT UNIQUE NOT NULL,
			status TEXT NOT NULL DEFAULT 'disabled',
			created_at DATETIME,
			updated_at DATETIME
		)
	`).Error
	require.NoError(t, err)
	
	return db
}

// Test helper to create a test logger
func createTestLogger() hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.Error, // Reduce noise during tests
	})
}

// Helper to set private fields for testing
func setManagerPlugins(manager Manager, plugins map[string]*Plugin) {
	mgr := reflect.ValueOf(manager).Elem()
	pluginsField := mgr.FieldByName("plugins")
	if pluginsField.IsValid() && pluginsField.CanSet() {
		pluginsField.Set(reflect.ValueOf(plugins))
	}
}

func TestNewManager(t *testing.T) {
	tmpDir := createTempDir(t)
	db := createTestDB(t)
	logger := createTestLogger()
	
	manager := NewManager(tmpDir, db, logger)
	assert.NotNil(t, manager)
	
	// Verify it implements the Manager interface
	var _ Manager = manager
}

func TestManagerInitialize(t *testing.T) {
	tmpDir := createTempDir(t)
	db := createTestDB(t)
	logger := createTestLogger()
	
	manager := NewManager(tmpDir, db, logger)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := manager.Initialize(ctx)
	assert.NoError(t, err)
	
	// Verify plugin directory was created
	assert.DirExists(t, tmpDir)
	
	// Clean up
	err = manager.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestDiscoverPlugins(t *testing.T) {
	tmpDir := createTempDir(t)
	db := createTestDB(t)
	logger := createTestLogger()
	
	// Create a test plugin directory structure
	pluginDir := filepath.Join(tmpDir, "test_plugin")
	err := os.MkdirAll(pluginDir, 0755)
	require.NoError(t, err)
	
	// Create a test plugin configuration
	configContent := `#Plugin: {
	schema_version: "1.0"
	id:            "test_plugin"
	name:          "Test Plugin"
	version:       "1.0.0"
	description:   "A test plugin"
	author:        "Test Author"
	type:          "metadata_scraper"
	
	entry_points: {
		main: "test_plugin"
	}
	
	permissions: []
	settings: {}
}`
	
	configPath := filepath.Join(pluginDir, "plugin.cue")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)
	
	// Create a dummy binary
	binaryPath := filepath.Join(pluginDir, "test_plugin")
	err = os.WriteFile(binaryPath, []byte("dummy binary"), 0755)
	require.NoError(t, err)
	
	manager := NewManager(tmpDir, db, logger)
	
	err = manager.DiscoverPlugins()
	assert.NoError(t, err)
	
	// Verify plugin was discovered
	plugins := manager.ListPlugins()
	assert.Len(t, plugins, 1)
	assert.Contains(t, plugins, "test_plugin")
	
	plugin := plugins["test_plugin"]
	assert.Equal(t, "test_plugin", plugin.ID)
	assert.Equal(t, "Test Plugin", plugin.Name)
	assert.Equal(t, "1.0.0", plugin.Version)
	assert.Equal(t, "metadata_scraper", plugin.Type)
	assert.Equal(t, binaryPath, plugin.BinaryPath)
	assert.Equal(t, configPath, plugin.ConfigPath)
	assert.False(t, plugin.Running)
}

func TestDiscoverPluginsInvalidConfig(t *testing.T) {
	tmpDir := createTempDir(t)
	db := createTestDB(t)
	logger := createTestLogger()
	
	// Create a test plugin directory with invalid config
	pluginDir := filepath.Join(tmpDir, "invalid_plugin")
	err := os.MkdirAll(pluginDir, 0755)
	require.NoError(t, err)
	
	// Create invalid plugin configuration
	configContent := `invalid cue syntax {`
	configPath := filepath.Join(pluginDir, "plugin.cue")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)
	
	manager := NewManager(tmpDir, db, logger)
	
	// Should not fail, but should log errors and continue
	err = manager.DiscoverPlugins()
	assert.NoError(t, err)
	
	// Verify no plugins were discovered
	plugins := manager.ListPlugins()
	assert.Len(t, plugins, 0)
}

func TestDiscoverPluginsMissingBinary(t *testing.T) {
	tmpDir := createTempDir(t)
	db := createTestDB(t)
	logger := createTestLogger()
	
	// Create a test plugin directory without binary
	pluginDir := filepath.Join(tmpDir, "no_binary_plugin")
	err := os.MkdirAll(pluginDir, 0755)
	require.NoError(t, err)
	
	// Create valid plugin configuration
	configContent := `#Plugin: {
	schema_version: "1.0"
	id:            "no_binary_plugin"
	name:          "No Binary Plugin"
	version:       "1.0.0"
	description:   "A plugin without binary"
	author:        "Test Author"
	type:          "metadata_scraper"
	
	entry_points: {
		main: "missing_binary"
	}
	
	permissions: []
	settings: {}
}`
	
	configPath := filepath.Join(pluginDir, "plugin.cue")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)
	
	manager := NewManager(tmpDir, db, logger)
	
	// Should not fail, but should log errors and continue
	err = manager.DiscoverPlugins()
	assert.NoError(t, err)
	
	// Verify no plugins were discovered
	plugins := manager.ListPlugins()
	assert.Len(t, plugins, 0)
}

func TestGetPlugin(t *testing.T) {
	tmpDir := createTempDir(t)
	db := createTestDB(t)
	logger := createTestLogger()
	
	manager := NewManager(tmpDir, db, logger)
	
	// Test getting non-existent plugin
	plugin, exists := manager.GetPlugin("non_existent")
	assert.False(t, exists)
	assert.Nil(t, plugin)
	
	// For remaining tests, we'll use the public interface only
	// to avoid accessing private fields
}

func TestListPlugins(t *testing.T) {
	tmpDir := createTempDir(t)
	db := createTestDB(t)
	logger := createTestLogger()
	
	manager := NewManager(tmpDir, db, logger)
	
	// Test empty list
	plugins := manager.ListPlugins()
	assert.Len(t, plugins, 0)
}

func TestServiceAccessors(t *testing.T) {
	tmpDir := createTempDir(t)
	db := createTestDB(t)
	logger := createTestLogger()
	
	manager := NewManager(tmpDir, db, logger)
	
	// Test empty accessors
	scrapers := manager.GetMetadataScrapers()
	assert.Len(t, scrapers, 0)
	
	scannerHooks := manager.GetScannerHooks()
	assert.Len(t, scannerHooks, 0)
	
	databases := manager.GetDatabases()
	assert.Len(t, databases, 0)
	
	adminPages := manager.GetAdminPages()
	assert.Len(t, adminPages, 0)
}

func TestManagerShutdown(t *testing.T) {
	tmpDir := createTempDir(t)
	db := createTestDB(t)
	logger := createTestLogger()
	
	manager := NewManager(tmpDir, db, logger)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Initialize manager
	err := manager.Initialize(ctx)
	require.NoError(t, err)
	
	// Shutdown should work without errors
	err = manager.Shutdown(ctx)
	assert.NoError(t, err)
	
	// Multiple shutdowns should be safe
	err = manager.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestConcurrentAccess(t *testing.T) {
	tmpDir := createTempDir(t)
	db := createTestDB(t)
	logger := createTestLogger()
	
	manager := NewManager(tmpDir, db, logger)
	
	// Test concurrent plugin list access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				plugins := manager.ListPlugins()
				_ = plugins // Use the result to avoid optimization
			}
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// Should complete without data races
	assert.True(t, true) // Test passed if we reach here without race conditions
}

// Benchmark tests
func BenchmarkListPlugins(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "viewra-plugin-bench-*")
	defer os.RemoveAll(tmpDir)
	
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	logger := createTestLogger()
	
	manager := NewManager(tmpDir, db, logger)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugins := manager.ListPlugins()
		_ = plugins
	}
}

func BenchmarkGetPlugin(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "viewra-plugin-bench-*")
	defer os.RemoveAll(tmpDir)
	
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	logger := createTestLogger()
	
	manager := NewManager(tmpDir, db, logger)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin, exists := manager.GetPlugin("test_plugin")
		_ = plugin
		_ = exists
	}
} 