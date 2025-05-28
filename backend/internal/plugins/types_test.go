package plugins

import (
	"context"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/plugins/proto"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Mock implementations for testing interfaces

// MockImplementation provides a test implementation of the Implementation interface
type MockImplementation struct {
	InitializeFunc         func(ctx *proto.PluginContext) error
	StartFunc              func() error
	StopFunc               func() error
	InfoFunc               func() (*proto.PluginInfo, error)
	HealthFunc             func() error
	MetadataScraperFunc    func() MetadataScraperService
	ScannerHookFunc        func() ScannerHookService
	DatabaseFunc           func() DatabaseService
	AdminPageFunc          func() AdminPageService
	APIRegistrationFunc    func() APIRegistrationService
	SearchFunc             func() SearchService
}

func (m *MockImplementation) Initialize(ctx *proto.PluginContext) error {
	if m.InitializeFunc != nil {
		return m.InitializeFunc(ctx)
	}
	return nil
}

func (m *MockImplementation) Start() error {
	if m.StartFunc != nil {
		return m.StartFunc()
	}
	return nil
}

func (m *MockImplementation) Stop() error {
	if m.StopFunc != nil {
		return m.StopFunc()
	}
	return nil
}

func (m *MockImplementation) Info() (*proto.PluginInfo, error) {
	if m.InfoFunc != nil {
		return m.InfoFunc()
	}
	return &proto.PluginInfo{
		Id:      "mock_plugin",
		Name:    "Mock Plugin",
		Version: "1.0.0",
	}, nil
}

func (m *MockImplementation) Health() error {
	if m.HealthFunc != nil {
		return m.HealthFunc()
	}
	return nil
}

func (m *MockImplementation) MetadataScraperService() MetadataScraperService {
	if m.MetadataScraperFunc != nil {
		return m.MetadataScraperFunc()
	}
	return nil
}

func (m *MockImplementation) ScannerHookService() ScannerHookService {
	if m.ScannerHookFunc != nil {
		return m.ScannerHookFunc()
	}
	return nil
}

func (m *MockImplementation) DatabaseService() DatabaseService {
	if m.DatabaseFunc != nil {
		return m.DatabaseFunc()
	}
	return nil
}

func (m *MockImplementation) AdminPageService() AdminPageService {
	if m.AdminPageFunc != nil {
		return m.AdminPageFunc()
	}
	return nil
}

func (m *MockImplementation) APIRegistrationService() APIRegistrationService {
	if m.APIRegistrationFunc != nil {
		return m.APIRegistrationFunc()
	}
	return nil
}

func (m *MockImplementation) SearchService() SearchService {
	if m.SearchFunc != nil {
		return m.SearchFunc()
	}
	return nil
}

// MockMetadataScraperService provides a test implementation
type MockMetadataScraperService struct {
	CanHandleFunc        func(filePath, mimeType string) bool
	ExtractMetadataFunc  func(filePath string) (map[string]string, error)
	GetSupportedTypesFunc func() []string
}

func (m *MockMetadataScraperService) CanHandle(filePath, mimeType string) bool {
	if m.CanHandleFunc != nil {
		return m.CanHandleFunc(filePath, mimeType)
	}
	return true
}

func (m *MockMetadataScraperService) ExtractMetadata(filePath string) (map[string]string, error) {
	if m.ExtractMetadataFunc != nil {
		return m.ExtractMetadataFunc(filePath)
	}
	return map[string]string{"test": "metadata"}, nil
}

func (m *MockMetadataScraperService) GetSupportedTypes() []string {
	if m.GetSupportedTypesFunc != nil {
		return m.GetSupportedTypesFunc()
	}
	return []string{"test/type"}
}

// MockScannerHookService provides a test implementation
type MockScannerHookService struct {
	OnMediaFileScannedFunc func(mediaFileID uint32, filePath string, metadata map[string]string) error
	OnScanStartedFunc      func(scanJobID, libraryID uint32, libraryPath string) error
	OnScanCompletedFunc    func(scanJobID, libraryID uint32, stats map[string]string) error
}

func (m *MockScannerHookService) OnMediaFileScanned(mediaFileID uint32, filePath string, metadata map[string]string) error {
	if m.OnMediaFileScannedFunc != nil {
		return m.OnMediaFileScannedFunc(mediaFileID, filePath, metadata)
	}
	return nil
}

func (m *MockScannerHookService) OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error {
	if m.OnScanStartedFunc != nil {
		return m.OnScanStartedFunc(scanJobID, libraryID, libraryPath)
	}
	return nil
}

func (m *MockScannerHookService) OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error {
	if m.OnScanCompletedFunc != nil {
		return m.OnScanCompletedFunc(scanJobID, libraryID, stats)
	}
	return nil
}

// MockDatabaseService provides a test implementation
type MockDatabaseService struct {
	GetModelsFunc func() []string
	MigrateFunc   func(connectionString string) error
	RollbackFunc  func(connectionString string) error
}

func (m *MockDatabaseService) GetModels() []string {
	if m.GetModelsFunc != nil {
		return m.GetModelsFunc()
	}
	return []string{"TestModel"}
}

func (m *MockDatabaseService) Migrate(connectionString string) error {
	if m.MigrateFunc != nil {
		return m.MigrateFunc(connectionString)
	}
	return nil
}

func (m *MockDatabaseService) Rollback(connectionString string) error {
	if m.RollbackFunc != nil {
		return m.RollbackFunc(connectionString)
	}
	return nil
}

// MockAdminPageService provides a test implementation
type MockAdminPageService struct {
	GetAdminPagesFunc  func() []*proto.AdminPageConfig
	RegisterRoutesFunc func(basePath string) error
}

func (m *MockAdminPageService) GetAdminPages() []*proto.AdminPageConfig {
	if m.GetAdminPagesFunc != nil {
		return m.GetAdminPagesFunc()
	}
	return []*proto.AdminPageConfig{
		{
			Path:  "/test",
			Title: "Test Page",
		},
	}
}

func (m *MockAdminPageService) RegisterRoutes(basePath string) error {
	if m.RegisterRoutesFunc != nil {
		return m.RegisterRoutesFunc(basePath)
	}
	return nil
}

// Interface compliance tests

func TestImplementationInterface(t *testing.T) {
	mock := &MockImplementation{}
	
	// Verify it implements the Implementation interface
	var _ Implementation = mock
	
	// Test basic functionality
	ctx := &proto.PluginContext{
		PluginId: "test",
		Config:   make(map[string]string),
	}
	
	err := mock.Initialize(ctx)
	assert.NoError(t, err)
	
	err = mock.Start()
	assert.NoError(t, err)
	
	info, err := mock.Info()
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "mock_plugin", info.Id)
	
	err = mock.Health()
	assert.NoError(t, err)
	
	err = mock.Stop()
	assert.NoError(t, err)
}

func TestMetadataScraperInterface(t *testing.T) {
	mock := &MockMetadataScraperService{}
	
	// Verify it implements the MetadataScraperService interface
	var _ MetadataScraperService = mock
	
	// Test functionality
	canHandle := mock.CanHandle("test.mp3", "audio/mpeg")
	assert.True(t, canHandle)
	
	metadata, err := mock.ExtractMetadata("test.mp3")
	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, "metadata", metadata["test"])
	
	types := mock.GetSupportedTypes()
	assert.Len(t, types, 1)
	assert.Equal(t, "test/type", types[0])
}

func TestScannerHookInterface(t *testing.T) {
	mock := &MockScannerHookService{}
	
	// Verify it implements the ScannerHookService interface
	var _ ScannerHookService = mock
	
	// Test functionality
	err := mock.OnScanStarted(1, 1, "/test/path")
	assert.NoError(t, err)
	
	err = mock.OnMediaFileScanned(1, "/test/file.mp3", map[string]string{"title": "test"})
	assert.NoError(t, err)
	
	err = mock.OnScanCompleted(1, 1, map[string]string{"files": "10"})
	assert.NoError(t, err)
}

func TestDatabaseInterface(t *testing.T) {
	mock := &MockDatabaseService{}
	
	// Verify it implements the DatabaseService interface
	var _ DatabaseService = mock
	
	// Test functionality
	models := mock.GetModels()
	assert.Len(t, models, 1)
	assert.Equal(t, "TestModel", models[0])
	
	err := mock.Migrate("sqlite://test.db")
	assert.NoError(t, err)
	
	err = mock.Rollback("sqlite://test.db")
	assert.NoError(t, err)
}

func TestAdminPageInterface(t *testing.T) {
	mock := &MockAdminPageService{}
	
	// Verify it implements the AdminPageService interface
	var _ AdminPageService = mock
	
	// Test functionality
	pages := mock.GetAdminPages()
	assert.Len(t, pages, 1)
	assert.Equal(t, "/test", pages[0].Path)
	assert.Equal(t, "Test Page", pages[0].Title)
	
	err := mock.RegisterRoutes("/admin")
	assert.NoError(t, err)
}

func TestManagerInterface(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.Error,
	})
	
	manager := NewManager("/tmp", db, logger)
	
	// Verify it implements the Manager interface
	var _ Manager = manager
	
	// Test that all methods exist and can be called
	ctx := context.Background()
	
	// These shouldn't panic
	plugins := manager.ListPlugins()
	assert.NotNil(t, plugins)
	
	_, exists := manager.GetPlugin("non_existent")
	assert.False(t, exists)
	
	scrapers := manager.GetMetadataScrapers()
	assert.NotNil(t, scrapers)
	
	hooks := manager.GetScannerHooks()
	assert.NotNil(t, hooks)
	
	databases := manager.GetDatabases()
	assert.NotNil(t, databases)
	
	adminPages := manager.GetAdminPages()
	assert.NotNil(t, adminPages)
	
	// These will fail but shouldn't panic
	_ = manager.LoadPlugin(ctx, "non_existent")
	_ = manager.UnloadPlugin(ctx, "non_existent")
	_ = manager.RestartPlugin(ctx, "non_existent")
}

func TestPluginStruct(t *testing.T) {
	plugin := &Plugin{
		ID:          "test_plugin",
		Name:        "Test Plugin",
		Version:     "1.0.0",
		Type:        "metadata_scraper",
		Description: "A test plugin",
		Author:      "Test Author",
		BinaryPath:  "/path/to/binary",
		ConfigPath:  "/path/to/config.cue",
		BasePath:    "/path/to/plugin",
		Running:     false,
	}
	
	// Verify all fields are accessible
	assert.Equal(t, "test_plugin", plugin.ID)
	assert.Equal(t, "Test Plugin", plugin.Name)
	assert.Equal(t, "1.0.0", plugin.Version)
	assert.Equal(t, "metadata_scraper", plugin.Type)
	assert.Equal(t, "A test plugin", plugin.Description)
	assert.Equal(t, "Test Author", plugin.Author)
	assert.Equal(t, "/path/to/binary", plugin.BinaryPath)
	assert.Equal(t, "/path/to/config.cue", plugin.ConfigPath)
	assert.Equal(t, "/path/to/plugin", plugin.BasePath)
	assert.False(t, plugin.Running)
}

func TestConfigStruct(t *testing.T) {
	config := &Config{
		SchemaVersion: "1.0",
		ID:            "test_plugin",
		Name:          "Test Plugin",
		Version:       "1.0.0",
		Description:   "A test plugin",
		Author:        "Test Author",
		Website:       "https://example.com",
		Repository:    "https://github.com/example/plugin",
		License:       "MIT",
		Type:          "metadata_scraper",
		Tags:          []string{"test", "metadata"},
		EntryPoints:   PluginEntryPoints{Main: "test_plugin_entrypoint"},
		Permissions:   []string{"database:read", "network:external"},
		Settings:      map[string]interface{}{"enabled": true},
	}
	
	config.Capabilities.MetadataExtraction = true
	config.Capabilities.DatabaseAccess = true
	
	// Verify all fields are accessible
	assert.Equal(t, "1.0", config.SchemaVersion)
	assert.Equal(t, "test_plugin", config.ID)
	assert.Equal(t, "Test Plugin", config.Name)
	assert.Equal(t, "1.0.0", config.Version)
	assert.True(t, config.Capabilities.MetadataExtraction)
	assert.True(t, config.Capabilities.DatabaseAccess)
	assert.Equal(t, "test_plugin_entrypoint", config.EntryPoints.Main)
	assert.Len(t, config.Tags, 2)
	assert.Len(t, config.Permissions, 2)
	assert.True(t, config.Settings["enabled"].(bool))
}

func TestRegistryStruct(t *testing.T) {
	registry := &Registry{
		MetadataScrapers: []string{"scraper1", "scraper2"},
		ScannerHooks:     []string{"hook1"},
		Databases:        []string{"db1"},
		AdminPages:       []string{"admin1", "admin2"},
	}
	
	// Verify all fields are accessible
	assert.Len(t, registry.MetadataScrapers, 2)
	assert.Len(t, registry.ScannerHooks, 1)
	assert.Len(t, registry.Databases, 1)
	assert.Len(t, registry.AdminPages, 2)
	
	assert.Contains(t, registry.MetadataScrapers, "scraper1")
	assert.Contains(t, registry.ScannerHooks, "hook1")
	assert.Contains(t, registry.Databases, "db1")
	assert.Contains(t, registry.AdminPages, "admin1")
} 