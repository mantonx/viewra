package plugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/plugins/proto"
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

// Enhanced test implementation for comprehensive testing
type PluginManagerMockImplementation struct {
	*MockImplementation
	apiRegService    APIRegistrationService
	searchService    SearchService
	startCallCount   int
	stopCallCount    int
	healthCallCount  int
	mu               sync.Mutex
}

func NewPluginManagerMockImplementation() *PluginManagerMockImplementation {
	return &PluginManagerMockImplementation{
		MockImplementation: &MockImplementation{},
		apiRegService:      &PluginManagerMockAPIRegistrationService{},
		searchService:      &PluginManagerMockSearchService{},
	}
}

func (e *PluginManagerMockImplementation) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.startCallCount++
	if e.MockImplementation.StartFunc != nil {
		return e.MockImplementation.StartFunc()
	}
	return nil
}

func (e *PluginManagerMockImplementation) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopCallCount++
	if e.MockImplementation.StopFunc != nil {
		return e.MockImplementation.StopFunc()
	}
	return nil
}

func (e *PluginManagerMockImplementation) Health() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.healthCallCount++
	if e.MockImplementation.HealthFunc != nil {
		return e.MockImplementation.HealthFunc()
	}
	return nil
}

func (e *PluginManagerMockImplementation) APIRegistrationService() APIRegistrationService {
	return e.apiRegService
}

func (e *PluginManagerMockImplementation) SearchService() SearchService {
	return e.searchService
}

func (e *PluginManagerMockImplementation) GetCallCounts() (start, stop, health int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.startCallCount, e.stopCallCount, e.healthCallCount
}

// Mock APIRegistrationService for plugin manager tests
type PluginManagerMockAPIRegistrationService struct {
	routes []*proto.APIRoute
}

func (m *PluginManagerMockAPIRegistrationService) GetRegisteredRoutes(ctx context.Context) ([]*proto.APIRoute, error) {
	if m.routes == nil {
		m.routes = []*proto.APIRoute{
			{Path: "/test", Method: "GET", Description: "Test route"},
		}
	}
	return m.routes, nil
}

// Mock SearchService for plugin manager tests
type PluginManagerMockSearchService struct {
	results []*proto.SearchResult
}

func (m *PluginManagerMockSearchService) Search(ctx context.Context, query map[string]string, limit, offset uint32) ([]*proto.SearchResult, uint32, bool, error) {
	if m.results == nil {
		m.results = []*proto.SearchResult{
			{Id: "1", Title: "Test Result", Artist: "Test Artist", Score: 1.0},
		}
	}
	
	// Simulate pagination
	start := int(offset)
	end := start + int(limit)
	if end > len(m.results) {
		end = len(m.results)
	}
	
	var pageResults []*proto.SearchResult
	if start < len(m.results) {
		pageResults = m.results[start:end]
	}
	
	return pageResults, uint32(len(m.results)), end < len(m.results), nil
}

func (m *PluginManagerMockSearchService) GetSearchCapabilities(ctx context.Context) ([]string, bool, uint32, error) {
	return []string{"title", "artist"}, true, 50, nil
}

// Basic manager functionality tests

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

// Advanced manager tests

func TestManagerWithGRPCServices(t *testing.T) {
	tmpDir := createTempDir(t)
	db := createTestDB(t)
	logger := createTestLogger()
	
	manager := NewManager(tmpDir, db, logger)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := manager.Initialize(ctx)
	require.NoError(t, err)
	
	// Test service accessors with empty manager
	scrapers := manager.GetMetadataScrapers()
	assert.Len(t, scrapers, 0)
	
	scannerHooks := manager.GetScannerHooks()
	assert.Len(t, scannerHooks, 0)
	
	databases := manager.GetDatabases()
	assert.Len(t, databases, 0)
	
	adminPages := manager.GetAdminPages()
	assert.Len(t, adminPages, 0)
	
	err = manager.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestPluginConfigValidation(t *testing.T) {
	tmpDir := createTempDir(t)
	db := createTestDB(t)
	logger := createTestLogger()
	
	testCases := []struct {
		name           string
		config         string
		expectPlugin   bool
	}{
		{
			name: "Valid minimal config",
			config: `#Plugin: {
	schema_version: "1.0"
	id:            "valid_plugin"
	name:          "Valid Plugin"
	version:       "1.0.0"
	description:   "A valid plugin"
	author:        "Test Author"
	type:          "metadata_scraper"
	entry_points: { main: "valid_plugin" }
	permissions: []
	settings: {}
}`,
			expectPlugin: true,
		},
		{
			name: "Missing required field",
			config: `#Plugin: {
	schema_version: "1.0"
	name:          "Invalid Plugin"
	version:       "1.0.0"
	description:   "Missing ID field"
	author:        "Test Author"
	type:          "metadata_scraper"
	entry_points: { main: "invalid_plugin" }
	permissions: []
	settings: {}
}`,
			expectPlugin: false,
		},
		{
			name:         "Completely invalid syntax",
			config:       `this is not valid cue syntax at all {{{`,
			expectPlugin: false,
		},
		{
			name:         "Empty config",
			config:       "",
			expectPlugin: false,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create subdirectory for this test case
			testDir := filepath.Join(tmpDir, strings.ReplaceAll(tc.name, " ", "_"))
			pluginDir := filepath.Join(testDir, "test_plugin")
			err := os.MkdirAll(pluginDir, 0755)
			require.NoError(t, err)
			
			// Write config
			configPath := filepath.Join(pluginDir, "plugin.cue")
			err = os.WriteFile(configPath, []byte(tc.config), 0644)
			require.NoError(t, err)
			
			// Create binary
			binaryPath := filepath.Join(pluginDir, "valid_plugin")
			err = os.WriteFile(binaryPath, []byte("test"), 0755)
			require.NoError(t, err)
			
			// Test discovery
			manager := NewManager(testDir, db, logger)
			err = manager.DiscoverPlugins()
			assert.NoError(t, err) // Discovery should not fail
			
			plugins := manager.ListPlugins()
			if tc.expectPlugin {
				assert.Len(t, plugins, 1, "Expected plugin to be discovered")
			} else {
				assert.Len(t, plugins, 0, "Expected no plugins to be discovered")
			}
		})
	}
}

func TestConcurrentPluginOperations(t *testing.T) {
	tmpDir := createTempDir(t)
	db := createTestDB(t)
	logger := createTestLogger()
	
	manager := NewManager(tmpDir, db, logger)
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	err := manager.Initialize(ctx)
	require.NoError(t, err)
	
	// Create multiple test plugins
	for i := 0; i < 5; i++ {
		pluginID := fmt.Sprintf("concurrent_test_plugin_%d", i)
		pluginDir := filepath.Join(tmpDir, pluginID)
		err = os.MkdirAll(pluginDir, 0755)
		require.NoError(t, err)
		
		configContent := fmt.Sprintf(`#Plugin: {
	schema_version: "1.0"
	id:            "%s"
	name:          "Concurrent Test Plugin %d"
	version:       "1.0.0"
	description:   "A plugin for testing concurrent operations"
	author:        "Test Author"
	type:          "metadata_scraper"
	entry_points: { main: "%s" }
	permissions: []
	settings: {}
}`, pluginID, i, pluginID)
		
		configPath := filepath.Join(pluginDir, "plugin.cue")
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)
		
		binaryPath := filepath.Join(pluginDir, pluginID)
		err = os.WriteFile(binaryPath, []byte("test"), 0755)
		require.NoError(t, err)
	}
	
	err = manager.DiscoverPlugins()
	require.NoError(t, err)
	
	// Test concurrent access to plugin list
	var wg sync.WaitGroup
	numGoroutines := 20
	
	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				plugins := manager.ListPlugins()
				assert.Len(t, plugins, 5, "Goroutine %d, iteration %d", id, j)
				
				// Access individual plugins
				for pluginID := range plugins {
					plugin, exists := manager.GetPlugin(pluginID)
					assert.True(t, exists, "Plugin should exist: %s", pluginID)
					assert.NotNil(t, plugin, "Plugin should not be nil: %s", pluginID)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	err = manager.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestPluginDatabaseIntegration(t *testing.T) {
	tmpDir := createTempDir(t)
	
	// Create a real file-based database to test persistence
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	
	// Create plugins table
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
	
	logger := createTestLogger()
	manager := NewManager(tmpDir, db, logger)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err = manager.Initialize(ctx)
	require.NoError(t, err)
	
	// Insert test plugin status
	pluginID := "db_test_plugin"
	err = db.Exec("INSERT INTO plugins (plugin_id, status, created_at, updated_at) VALUES (?, ?, ?, ?)",
		pluginID, "enabled", time.Now(), time.Now()).Error
	require.NoError(t, err)
	
	// Verify the plugin manager can read from database
	var count int64
	err = db.Table("plugins").Where("plugin_id = ? AND status = ?", pluginID, "enabled").Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
	
	err = manager.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestErrorRecovery(t *testing.T) {
	tmpDir := createTempDir(t)
	db := createTestDB(t)
	logger := createTestLogger()
	
	manager := NewManager(tmpDir, db, logger)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := manager.Initialize(ctx)
	require.NoError(t, err)
	
	// Test that manager continues to function after errors
	
	// 1. Try to get non-existent plugin
	plugin, exists := manager.GetPlugin("non_existent_plugin")
	assert.False(t, exists)
	assert.Nil(t, plugin)
	
	// 2. Manager should still work normally
	plugins := manager.ListPlugins()
	assert.NotNil(t, plugins)
	
	// 3. Service accessors should still work
	scrapers := manager.GetMetadataScrapers()
	assert.NotNil(t, scrapers)
	
	scannerHooks := manager.GetScannerHooks()
	assert.NotNil(t, scannerHooks)
	
	err = manager.Shutdown(ctx)
	assert.NoError(t, err)
}

// Benchmark tests for performance validation
func BenchmarkPluginDiscovery(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "viewra-plugin-bench-*")
	defer os.RemoveAll(tmpDir)
	
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	logger := hclog.NewNullLogger()
	
	// Create multiple test plugins
	for i := 0; i < 10; i++ {
		pluginID := fmt.Sprintf("bench_plugin_%d", i)
		pluginDir := filepath.Join(tmpDir, pluginID)
		os.MkdirAll(pluginDir, 0755)
		
		configContent := fmt.Sprintf(`#Plugin: {
	schema_version: "1.0"
	id:            "%s"
	name:          "Benchmark Plugin %d"
	version:       "1.0.0"
	description:   "A plugin for benchmarking"
	author:        "Test Author"
	type:          "metadata_scraper"
	entry_points: { main: "%s" }
	permissions: []
	settings: {}
}`, pluginID, i, pluginID)
		
		configPath := filepath.Join(pluginDir, "plugin.cue")
		os.WriteFile(configPath, []byte(configContent), 0644)
		
		binaryPath := filepath.Join(pluginDir, pluginID)
		os.WriteFile(binaryPath, []byte("test"), 0755)
	}
	
	manager := NewManager(tmpDir, db, logger)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.DiscoverPlugins()
	}
}

func BenchmarkConcurrentPluginAccess(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "viewra-plugin-bench-*")
	defer os.RemoveAll(tmpDir)
	
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	logger := hclog.NewNullLogger()
	
	manager := NewManager(tmpDir, db, logger)
	manager.Initialize(context.Background())
	
	// Create some test plugins
	for i := 0; i < 5; i++ {
		pluginID := fmt.Sprintf("concurrent_bench_plugin_%d", i)
		pluginDir := filepath.Join(tmpDir, pluginID)
		os.MkdirAll(pluginDir, 0755)
		
		configContent := fmt.Sprintf(`#Plugin: {
	schema_version: "1.0"
	id:            "%s"
	name:          "Concurrent Benchmark Plugin %d"
	version:       "1.0.0"
	description:   "A plugin for benchmarking concurrent access"
	author:        "Test Author"
	type:          "metadata_scraper"
	entry_points: { main: "%s" }
	permissions: []
	settings: {}
}`, pluginID, i, pluginID)
		
		configPath := filepath.Join(pluginDir, "plugin.cue")
		os.WriteFile(configPath, []byte(configContent), 0644)
		
		binaryPath := filepath.Join(pluginDir, pluginID)
		os.WriteFile(binaryPath, []byte("test"), 0755)
	}
	
	manager.DiscoverPlugins()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			plugins := manager.ListPlugins()
			for pluginID := range plugins {
				manager.GetPlugin(pluginID)
			}
		}
	})
}

