package playbackmodule

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/playbackmodule"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
	plugins "github.com/mantonx/viewra/sdk"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestData holds common test data
type TestData struct {
	VideoPath        string
	TempDir          string
	TranscodingDir   string
	ExpectedDuration int // in seconds
}

// TestableResponseWriter implements http.CloseNotifier for testing
type TestableResponseWriter struct {
	*httptest.ResponseRecorder
	closeChan chan bool
}

// NewTestableResponseWriter creates a new testable response writer
func NewTestableResponseWriter() *TestableResponseWriter {
	return &TestableResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		closeChan:        make(chan bool, 1),
	}
}

// CloseNotify returns a channel that receives a value when the client connection has gone away
func (w *TestableResponseWriter) CloseNotify() <-chan bool {
	return w.closeChan
}

// Close simulates a client disconnect
func (w *TestableResponseWriter) Close() {
	select {
	case w.closeChan <- true:
	default:
	}
}

// PluginInfo represents basic plugin information for tests
type PluginInfo struct {
	ID          string
	Name        string
	Version     string
	Type        string
	Description string
	Author      string
	Status      string
}

// PluginManagerInterface defines the plugin manager interface for tests
type PluginManagerInterface interface {
	GetRunningPluginInterface(pluginID string) (interface{}, bool)
	ListPlugins() []PluginInfo
	GetRunningPlugins() []PluginInfo
}

// CreateTestModule creates a test playback module using the new architecture
func CreateTestModule(db *gorm.DB, pluginManager playbackmodule.PluginManagerInterface) *playbackmodule.Module {
	return playbackmodule.NewModule(db, nil)
}

// PluginManagerAdapter adapts test PluginManagerInterface to playbackmodule.PluginManagerInterface
type PluginManagerAdapter struct {
	pluginManager PluginManagerInterface
}

func (a *PluginManagerAdapter) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	return a.pluginManager.GetRunningPluginInterface(pluginID)
}

func (a *PluginManagerAdapter) ListPlugins() []playbackmodule.PluginInfo {
	plugins := a.pluginManager.ListPlugins()
	var result []playbackmodule.PluginInfo
	for _, p := range plugins {
		result = append(result, playbackmodule.PluginInfo{
			ID:          p.ID,
			Name:        p.Name,
			Version:     p.Version,
			Type:        p.Type,
			Description: p.Description,
			Author:      p.Author,
			Status:      p.Status,
		})
	}
	return result
}

func (a *PluginManagerAdapter) GetRunningPlugins() []playbackmodule.PluginInfo {
	return a.ListPlugins()
}

// setupTestEnvironment creates a test environment with temporary directories
func setupTestEnvironment(t *testing.T) *TestData {
	t.Helper()

	// Create temp directories for testing
	tempDir, err := os.MkdirTemp("", "viewra_test_")
	require.NoError(t, err)

	transcodingDir := filepath.Join(tempDir, "transcoding")
	err = os.MkdirAll(transcodingDir, 0755)
	require.NoError(t, err)

	// Configure test environment
	os.Setenv("VIEWRA_TRANSCODING_DIR", transcodingDir)
	os.Setenv("VIEWRA_TEMP_DIR", tempDir)

	// Create a test video file (if FFmpeg is available)
	videoPath := filepath.Join(tempDir, "test_video.mp4")
	if err := createTestVideo(t, videoPath); err != nil {
		t.Skipf("Skipping test - FFmpeg not available: %v", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
		os.Unsetenv("VIEWRA_TRANSCODING_DIR")
		os.Unsetenv("VIEWRA_TEMP_DIR")
	})

	return &TestData{
		VideoPath:        videoPath,
		TempDir:          tempDir,
		TranscodingDir:   transcodingDir,
		ExpectedDuration: 10,
	}
}

// createTestVideo creates a simple test video file using FFmpeg
func createTestVideo(t *testing.T, outputPath string) error {
	// Try different encoders in order of preference
	encoders := []string{
		"-vcodec", "mpeg4", // Usually available
		"-vcodec", "libvpx", // WebM codec
		"-vcodec", "mjpeg", // Motion JPEG (fallback)
	}

	// Base FFmpeg command without video codec
	baseArgs := []string{
		"-f", "lavfi",
		"-i", "testsrc=duration=10:size=320x240:rate=25",
		"-f", "lavfi",
		"-i", "sine=frequency=1000:duration=10",
		"-c:a", "aac",
		"-b:a", "64k",
		"-y",
	}

	// Try each encoder
	for i := 0; i < len(encoders); i += 2 {
		args := append([]string{}, baseArgs...)
		args = append(args, encoders[i], encoders[i+1])
		args = append(args, outputPath)

		cmd := exec.Command("ffmpeg", args...)
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Logf("✅ Created test video using %s encoder", encoders[i+1])
			return nil
		}

		// Log the attempt
		t.Logf("❌ Failed with %s encoder: %v", encoders[i+1], err)
		if len(output) > 0 && i == len(encoders)-2 {
			// Only show output for last failure
			return fmt.Errorf("failed to create test video: %v\nOutput: %s", err, output)
		}
	}

	return fmt.Errorf("no suitable video encoder found")
}

// setupTestDatabase creates an in-memory test database
func setupTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   nil, // Disable logging for tests
	})
	require.NoError(t, err)

	// SQLite doesn't support JSONB, so we need to register a custom type
	// This allows GORM to use TEXT for JSON fields in SQLite
	db.Exec("PRAGMA foreign_keys = ON")

	// Register JSON serializer for SQLite
	sqliteDB, err := db.DB()
	require.NoError(t, err)

	// Set connection parameters for SQLite
	sqliteDB.SetMaxOpenConns(1)
	sqliteDB.SetMaxIdleConns(1)

	// Run migrations for the playback module tables
	err = db.AutoMigrate(&database.TranscodeSession{})
	if err != nil {
		// If AutoMigrate fails, try creating the table manually for SQLite
		t.Logf("AutoMigrate failed: %v, creating table manually", err)

		createTableSQL := `
		CREATE TABLE IF NOT EXISTS transcode_sessions (
			id TEXT PRIMARY KEY,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			provider TEXT NOT NULL DEFAULT 'unknown',
			status TEXT NOT NULL,
			request TEXT,
			progress TEXT,
			result TEXT,
			hardware TEXT,
			start_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			end_time DATETIME,
			last_accessed DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			directory_path TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_transcode_sessions_deleted_at ON transcode_sessions(deleted_at);
		CREATE INDEX IF NOT EXISTS idx_transcode_sessions_provider ON transcode_sessions(provider);
		CREATE INDEX IF NOT EXISTS idx_transcode_sessions_status ON transcode_sessions(status);
		CREATE INDEX IF NOT EXISTS idx_transcode_sessions_start_time ON transcode_sessions(start_time);
		CREATE INDEX IF NOT EXISTS idx_transcode_sessions_last_accessed ON transcode_sessions(last_accessed);
		`

		err = db.Exec(createTableSQL).Error
		require.NoError(t, err, "Failed to create transcode_sessions table")
		t.Logf("Created transcode_sessions table manually")
	}

	// Force table creation by running a query
	var count int64
	db.Model(&database.TranscodeSession{}).Count(&count)
	t.Logf("Transcode sessions table initialized with %d rows", count)

	// For SQLite testing, we need to disable certain GORM features
	// that use PostgreSQL-specific syntax
	db = db.Session(&gorm.Session{
		DisableNestedTransaction: true,
		AllowGlobalUpdate:        true,
		SkipDefaultTransaction:   true,
	})

	// Override the SQL dialect for JSON fields in SQLite
	// This is a workaround for SQLite not supporting JSONB
	db.Callback().Create().Before("gorm:create").Register("sqlite_json_fix", func(db *gorm.DB) {
		if db.Statement.Schema != nil {
			for _, field := range db.Statement.Schema.Fields {
				if field.GORMDataType == "jsonb" {
					field.DataType = "TEXT"
				}
			}
		}
	})

	return db
}

// setupPluginEnabledEnvironment creates a test environment with mock plugins
func setupPluginEnabledEnvironment(t *testing.T, db *gorm.DB) *playbackmodule.Module {
	t.Helper()

	// Create mock plugin manager
	mockPluginManager := NewMockPluginManager()
	adapter := &PluginManagerAdapter{pluginManager: mockPluginManager}

	// Create module
	module := CreateTestModule(db, adapter)

	// Run migrations before initializing
	err := module.Migrate(db)
	require.NoError(t, err, "Failed to migrate database")

	// Verify the table exists
	var tableExists bool
	err = db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name='transcode_sessions'").Scan(&tableExists).Error
	if err != nil || !tableExists {
		t.Logf("Table doesn't exist after migration, creating manually")
		// Create the table manually if migration didn't work
		createTableSQL := `
		CREATE TABLE IF NOT EXISTS transcode_sessions (
			id TEXT PRIMARY KEY,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			provider TEXT NOT NULL DEFAULT 'unknown',
			status TEXT NOT NULL,
			request TEXT,
			progress TEXT,
			result TEXT,
			hardware TEXT,
			start_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			end_time DATETIME,
			last_accessed DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			directory_path TEXT
		)`
		err = db.Exec(createTableSQL).Error
		require.NoError(t, err, "Failed to create table manually")
	}

	// For SQLite testing, register a JSON serializer hook
	// This automatically converts structs to JSON strings
	db.Callback().Create().Before("gorm:before_create").Register("json_serialize", func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}

		// Handle TranscodeSession JSON fields
		if tx.Statement.Schema.Name == "TranscodeSession" {
			if tx.Statement.Dest != nil {
				// Use reflection to serialize JSON fields manually
				switch v := tx.Statement.Dest.(type) {
				case *database.TranscodeSession:
					if v.Request != nil {
						if data, err := json.Marshal(v.Request); err == nil {
							tx.Statement.SetColumn("request", string(data))
						}
					}
					if v.Progress != nil {
						if data, err := json.Marshal(v.Progress); err == nil {
							tx.Statement.SetColumn("progress", string(data))
						}
					}
					if v.Result != nil {
						if data, err := json.Marshal(v.Result); err == nil {
							tx.Statement.SetColumn("result", string(data))
						}
					}
					if v.Hardware != nil {
						if data, err := json.Marshal(v.Hardware); err == nil {
							tx.Statement.SetColumn("hardware", string(data))
						}
					}
				}
			}
		}
	})

	// Initialize module
	err = module.Init()
	require.NoError(t, err)

	return module
}

// setupRealPluginEnvironment creates a test environment with real plugin manager
func setupRealPluginEnvironment(t *testing.T, db *gorm.DB) *playbackmodule.Module {
	t.Helper()

	// Create real plugin manager
	pluginDir := filepath.Join(os.TempDir(), "test_plugins")
	err := os.MkdirAll(pluginDir, 0755)
	require.NoError(t, err)

	t.Cleanup(func() {
		os.RemoveAll(pluginDir)
	})

	// Create external plugin manager with correct signature
	logger := hclog.NewNullLogger()
	extMgr := pluginmodule.NewExternalPluginManager(db, logger)

	// Initialize plugin manager with required parameters
	ctx := context.Background()
	hostServices := &pluginmodule.HostServices{}
	err = extMgr.Initialize(ctx, pluginDir, hostServices)
	require.NoError(t, err)

	// Create adapter for plugin manager
	adapter := playbackmodule.NewExternalPluginManagerAdapter(extMgr)

	// Create and initialize module
	module := CreateTestModule(db, adapter)
	err = module.Init()
	require.NoError(t, err)

	return module
}

// setupSlowTranscodingEnvironment creates a test environment with slow mock provider
func setupSlowTranscodingEnvironment(t *testing.T, db *gorm.DB) *playbackmodule.Module {
	t.Helper()

	// Create slow mock plugin manager
	slowPluginManager := NewSlowMockPluginManager()
	adapter := &PluginManagerAdapter{pluginManager: slowPluginManager}

	// Create and initialize module
	module := CreateTestModule(db, adapter)
	err := module.Init()
	require.NoError(t, err)

	return module
}

// createTestRouter creates a test router with playback routes
func createTestRouter(t *testing.T, module *playbackmodule.Module) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Register playback module routes
	module.RegisterRoutes(router)

	return router
}

// SlowMockTranscodingProvider implements a slow transcoding provider for timeout testing
type SlowMockTranscodingProvider struct {
	MockTranscodingProvider
	delay time.Duration
}

// NewSlowMockTranscodingProvider creates a new slow mock transcoding provider
func NewSlowMockTranscodingProvider() *SlowMockTranscodingProvider {
	return &SlowMockTranscodingProvider{
		MockTranscodingProvider: MockTranscodingProvider{
			sessions: make(map[string]*plugins.TranscodeHandle),
			streams:  make(map[string]*mockStream),
		},
		delay: 2 * time.Second, // Simulate slow operations
	}
}

// StartTranscode starts a slow transcoding operation
func (s *SlowMockTranscodingProvider) StartTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	// Simulate slow startup
	time.Sleep(s.delay)
	return s.MockTranscodingProvider.StartTranscode(ctx, req)
}

// GetProgress returns transcoding progress slowly
func (s *SlowMockTranscodingProvider) GetProgress(handle *plugins.TranscodeHandle) (*plugins.TranscodingProgress, error) {
	// Simulate slow progress check
	time.Sleep(500 * time.Millisecond)
	return s.MockTranscodingProvider.GetProgress(handle)
}

// MockPluginManager implements PluginManagerInterface for testing
type MockPluginManager struct {
	provider *MockTranscodingProvider
}

// NewMockPluginManager creates a new mock plugin manager
func NewMockPluginManager() *MockPluginManager {
	return &MockPluginManager{
		provider: NewMockTranscodingProvider(),
	}
}

func (m *MockPluginManager) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	// Return mock plugin implementation
	return &MockPluginImpl{provider: m.provider}, true
}

func (m *MockPluginManager) ListPlugins() []PluginInfo {
	return []PluginInfo{
		{
			ID:          "mock-ffmpeg",
			Name:        "Mock FFmpeg Plugin",
			Version:     "1.0.0",
			Type:        "transcoder",
			Description: "Mock transcoding plugin for testing",
			Author:      "Test Suite",
			Status:      "running",
		},
	}
}

func (m *MockPluginManager) GetRunningPlugins() []PluginInfo {
	return m.ListPlugins()
}

// SlowMockPluginManager implements a slow mock plugin manager for timeout testing
type SlowMockPluginManager struct {
	provider *SlowMockTranscodingProvider
}

// NewSlowMockPluginManager creates a new slow mock plugin manager
func NewSlowMockPluginManager() *SlowMockPluginManager {
	return &SlowMockPluginManager{
		provider: NewSlowMockTranscodingProvider(),
	}
}

func (m *SlowMockPluginManager) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	return &MockPluginImpl{provider: m.provider}, true
}

func (m *SlowMockPluginManager) ListPlugins() []PluginInfo {
	return []PluginInfo{
		{
			ID:          "slow-mock-ffmpeg",
			Name:        "Slow Mock FFmpeg Plugin",
			Version:     "1.0.0",
			Type:        "transcoder",
			Description: "Slow mock transcoding plugin for timeout testing",
			Author:      "Test Suite",
			Status:      "running",
		},
	}
}

func (m *SlowMockPluginManager) GetRunningPlugins() []PluginInfo {
	return m.ListPlugins()
}

// MockPluginImpl represents a mock plugin implementation
type MockPluginImpl struct {
	provider plugins.TranscodingProvider
}

// Initialize initializes the plugin
func (m *MockPluginImpl) Initialize(ctx *plugins.PluginContext) error {
	return nil
}

// Start starts the plugin
func (m *MockPluginImpl) Start() error {
	return nil
}

// Stop stops the plugin
func (m *MockPluginImpl) Stop() error {
	return nil
}

// Info returns plugin information
func (m *MockPluginImpl) Info() (*plugins.PluginInfo, error) {
	info := m.provider.GetInfo()
	return &plugins.PluginInfo{
		ID:          info.ID,
		Name:        info.Name,
		Version:     info.Version,
		Description: info.Description,
		Author:      info.Author,
		Type:        "transcoder",
	}, nil
}

// Health returns plugin health status
func (m *MockPluginImpl) Health() error {
	return nil
}

// MetadataScraperService not implemented for transcoding plugin
func (m *MockPluginImpl) MetadataScraperService() plugins.MetadataScraperService { return nil }

// ScannerHookService not implemented for transcoding plugin
func (m *MockPluginImpl) ScannerHookService() plugins.ScannerHookService { return nil }

// AssetService not implemented for transcoding plugin
func (m *MockPluginImpl) AssetService() plugins.AssetService { return nil }

// DatabaseService not implemented for transcoding plugin
func (m *MockPluginImpl) DatabaseService() plugins.DatabaseService { return nil }

// AdminPageService not implemented for transcoding plugin
func (m *MockPluginImpl) AdminPageService() plugins.AdminPageService { return nil }

// APIRegistrationService not implemented for transcoding plugin
func (m *MockPluginImpl) APIRegistrationService() plugins.APIRegistrationService { return nil }

// SearchService not implemented for transcoding plugin
func (m *MockPluginImpl) SearchService() plugins.SearchService { return nil }

// HealthMonitorService not implemented for transcoding plugin
func (m *MockPluginImpl) HealthMonitorService() plugins.HealthMonitorService { return nil }

// ConfigurationService not implemented for transcoding plugin
func (m *MockPluginImpl) ConfigurationService() plugins.ConfigurationService { return nil }

// PerformanceMonitorService not implemented for transcoding plugin
func (m *MockPluginImpl) PerformanceMonitorService() plugins.PerformanceMonitorService { return nil }

// EnhancedAdminPageService not implemented for transcoding plugin
func (m *MockPluginImpl) EnhancedAdminPageService() plugins.EnhancedAdminPageService { return nil }

// TranscodingProvider returns the mock transcoding provider
func (m *MockPluginImpl) TranscodingProvider() plugins.TranscodingProvider {
	return m.provider
}

// ExternalPluginManagerAdapter adapts external plugin manager to PluginManagerInterface
type ExternalPluginManagerAdapter struct {
	manager *pluginmodule.ExternalPluginManager
}

func (a *ExternalPluginManagerAdapter) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	return a.manager.GetRunningPluginInterface(pluginID)
}

func (a *ExternalPluginManagerAdapter) ListPlugins() []PluginInfo {
	plugins := a.manager.ListPlugins()
	var result []PluginInfo
	for _, p := range plugins {
		result = append(result, PluginInfo{
			ID:          p.ID,
			Name:        p.Name,
			Version:     p.Version,
			Type:        p.Type,
			Description: p.Description,
			Author:      "", // pluginmodule.PluginInfo doesn't have Author field
			Status:      "", // pluginmodule.PluginInfo doesn't have Status field
		})
	}
	return result
}

func (a *ExternalPluginManagerAdapter) GetRunningPlugins() []PluginInfo {
	return a.ListPlugins()
}

// All mock implementations using old TranscodingService interface have been removed
// TODO: Implement new mocks using TranscodingProvider interface when needed

/*
// MockPluginImpl represents a mock plugin that provides a transcoding service
type MockPluginImpl struct {
	service *MockTranscodingService
}

// ... rest of mock implementations removed ...
*/

// Docker-specific test helpers

// setupDockerStyleEnvironment creates an environment that mimics Docker volume mounting
func setupDockerStyleEnvironment(t *testing.T) *TestData {
	t.Helper()

	// Create Docker-style directory structure
	tempDir, err := os.MkdirTemp("", "viewra_docker_test_")
	require.NoError(t, err)

	// Mimic the Docker volume path structure
	dockerDataDir := filepath.Join(tempDir, "viewra-data")
	transcodingDir := filepath.Join(dockerDataDir, "transcoding")
	err = os.MkdirAll(transcodingDir, 0755)
	require.NoError(t, err)

	// Create a test video file
	videoPath := filepath.Join(tempDir, "test_video.mp4")
	if err := createTestVideo(t, videoPath); err != nil {
		t.Skipf("Skipping Docker test - FFmpeg not available: %v", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return &TestData{
		VideoPath:        videoPath,
		TempDir:          tempDir,
		TranscodingDir:   transcodingDir,
		ExpectedDuration: 10,
	}
}

// configureDockerStyleTranscoding sets up transcoding configuration for Docker-style testing
func configureDockerStyleTranscoding(t *testing.T, testData *TestData) {
	t.Helper()

	// Set environment variables as they would be in Docker
	os.Setenv("VIEWRA_TEST_TRANSCODE_DIR", testData.TranscodingDir)
	os.Setenv("VIEWRA_TRANSCODING_DIR", testData.TranscodingDir)

	// Configure the config system BEFORE it's used
	cfg := config.Get()
	if cfg != nil {
		cfg.Transcoding.DataDir = testData.TranscodingDir
	}

	t.Logf("Docker-style transcoding configured: %s", testData.TranscodingDir)
}

// cleanupDockerEnvironment cleans up Docker-style test environment
func cleanupDockerEnvironment(t *testing.T, testData *TestData) {
	t.Helper()

	// Clean up environment variables
	os.Unsetenv("VIEWRA_TEST_TRANSCODE_DIR")

	// No need to clean up directories - handled by t.Cleanup in setupDockerStyleEnvironment
	t.Logf("Docker-style environment cleaned up")
}

// MockTranscodingPlugin is commented out - uses old TranscodingService interface
/*
type MockTranscodingPlugin struct {
	// Implementation removed - awaiting TranscodingProvider test implementation
}
*/

// MockPluginManagerInterface is a mock implementation of PluginManagerInterface for testing
type MockPluginManagerInterface struct {
	mockPlugins map[string]interface{}
}

// NewMockPluginManagerInterface creates a new mock plugin manager interface
func NewMockPluginManagerInterface() *MockPluginManagerInterface {
	return &MockPluginManagerInterface{
		mockPlugins: make(map[string]interface{}),
	}
}

// GetPlugin returns a mock plugin if it exists
func (m *MockPluginManagerInterface) GetPlugin(pluginType, pluginID string) (interface{}, error) {
	// TODO: Update to return TranscodingProvider mocks once available
	return nil, fmt.Errorf("mock transcoding plugins not yet implemented for TranscodingProvider")
}

// AddMockPlugin adds a mock plugin
func (m *MockPluginManagerInterface) AddMockPlugin(pluginType, pluginID string, plugin interface{}) {
	m.mockPlugins[fmt.Sprintf("%s:%s", pluginType, pluginID)] = plugin
}

// All old TranscodingService mocks have been removed
// TODO: Implement new mocks for TranscodingProvider interface

// MockTranscodingProvider implements the TranscodingProvider interface for testing
type MockTranscodingProvider struct {
	mu       sync.Mutex
	sessions map[string]*plugins.TranscodeHandle
	streams  map[string]*mockStream
	// Add mock session store
	mockSessions map[string]*MockSession
}

// MockSession represents a simplified session for testing
type MockSession struct {
	ID            string
	Provider      string
	Status        string
	InputPath     string
	Container     string
	DirectoryPath string
	StartTime     time.Time
}

// mockStream represents a mock streaming session
type mockStream struct {
	data   []byte
	reader io.ReadCloser
}

// NewMockTranscodingProvider creates a new mock transcoding provider
func NewMockTranscodingProvider() *MockTranscodingProvider {
	return &MockTranscodingProvider{
		sessions: make(map[string]*plugins.TranscodeHandle),
		streams:  make(map[string]*mockStream),
	}
}

// GetInfo returns provider information
func (m *MockTranscodingProvider) GetInfo() plugins.ProviderInfo {
	return plugins.ProviderInfo{
		ID:          "mock-ffmpeg",
		Name:        "Mock FFmpeg Provider",
		Description: "Mock transcoding provider for testing",
		Version:     "1.0.0",
		Author:      "Test Suite",
		Priority:    100,
	}
}

// GetSupportedFormats returns supported container formats
func (m *MockTranscodingProvider) GetSupportedFormats() []plugins.ContainerFormat {
	return []plugins.ContainerFormat{
		{
			Format:      "mp4",
			MimeType:    "video/mp4",
			Extensions:  []string{".mp4"},
			Description: "MPEG-4 Part 14",
			Adaptive:    false,
		},
		{
			Format:      "dash",
			MimeType:    "application/dash+xml",
			Extensions:  []string{".mpd", ".m4s"},
			Description: "Dynamic Adaptive Streaming over HTTP",
			Adaptive:    true,
		},
		{
			Format:      "hls",
			MimeType:    "application/vnd.apple.mpegurl",
			Extensions:  []string{".m3u8", ".ts"},
			Description: "HTTP Live Streaming",
			Adaptive:    true,
		},
	}
}

// GetHardwareAccelerators returns available hardware accelerators
func (m *MockTranscodingProvider) GetHardwareAccelerators() []plugins.HardwareAccelerator {
	return []plugins.HardwareAccelerator{
		{
			Type:        "none",
			ID:          "software",
			Name:        "Software Encoding",
			Available:   true,
			DeviceCount: 1,
		},
	}
}

// GetQualityPresets returns quality presets
func (m *MockTranscodingProvider) GetQualityPresets() []plugins.QualityPreset {
	return []plugins.QualityPreset{
		{
			ID:          "low",
			Name:        "Low Quality",
			Description: "Fast encoding, smaller file size",
			Quality:     30,
			SpeedRating: 10,
			SizeRating:  3,
		},
		{
			ID:          "medium",
			Name:        "Medium Quality",
			Description: "Balanced quality and speed",
			Quality:     50,
			SpeedRating: 5,
			SizeRating:  5,
		},
		{
			ID:          "high",
			Name:        "High Quality",
			Description: "Better quality, slower encoding",
			Quality:     80,
			SpeedRating: 2,
			SizeRating:  8,
		},
	}
}

// StartTranscode starts a mock transcoding operation
func (m *MockTranscodingProvider) StartTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate unique session ID
	sessionID := fmt.Sprintf("mock_%d", time.Now().UnixNano())

	// Get transcoding directory from environment or use default
	transcodingDir := os.Getenv("VIEWRA_TEST_TRANSCODE_DIR")
	if transcodingDir == "" {
		transcodingDir = "/tmp/viewra/transcoding"
	}

	// Create session directory
	sessionDir := filepath.Join(transcodingDir, fmt.Sprintf("session_%s", sessionID))
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Create mock transcoding handle
	handle := &plugins.TranscodeHandle{
		SessionID: sessionID,
		Provider:  "mock-ffmpeg",
		StartTime: time.Now(),
		Directory: sessionDir,
		ProcessID: 12345, // Mock PID
	}

	// For adaptive formats, create mock files
	if req.Container == "dash" || req.Container == "hls" {
		// Create mock adaptive streaming files
		m.createMockAdaptiveFiles(sessionDir, req.Container)
	}

	m.sessions[sessionID] = handle

	return handle, nil
}

// GetProgress returns transcoding progress
func (m *MockTranscodingProvider) GetProgress(handle *plugins.TranscodeHandle) (*plugins.TranscodingProgress, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[handle.SessionID]; !exists {
		return nil, fmt.Errorf("session not found: %s", handle.SessionID)
	}

	// Simulate progress based on time elapsed
	elapsed := time.Since(handle.StartTime)
	percentComplete := float64(elapsed.Seconds()) * 10.0 // 10% per second
	if percentComplete > 100 {
		percentComplete = 100
	}

	return &plugins.TranscodingProgress{
		PercentComplete: percentComplete,
		TimeElapsed:     elapsed,
		TimeRemaining:   time.Duration((100-percentComplete)/10) * time.Second,
		BytesRead:       int64(percentComplete * 1000000), // 100MB total
		BytesWritten:    int64(percentComplete * 500000),  // 50MB output
		CurrentSpeed:    1.0,
		AverageSpeed:    1.0,
	}, nil
}

// StopTranscode stops a transcoding operation
func (m *MockTranscodingProvider) StopTranscode(handle *plugins.TranscodeHandle) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, handle.SessionID)
	return nil
}

// StartStream starts a streaming operation
func (m *MockTranscodingProvider) StartStream(ctx context.Context, req plugins.TranscodeRequest) (*plugins.StreamHandle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("stream-%d", time.Now().UnixNano())
	}

	handle := &plugins.StreamHandle{
		SessionID:  sessionID,
		Provider:   "mock-ffmpeg",
		StartTime:  time.Now(),
		ProcessID:  12346, // Mock PID
		Context:    ctx,
		CancelFunc: func() {},
	}

	// Create mock stream data
	mockData := []byte("mock transcoded streaming data")
	m.streams[sessionID] = &mockStream{
		data:   mockData,
		reader: io.NopCloser(bytes.NewReader(mockData)),
	}

	return handle, nil
}

// GetStream returns the stream reader
func (m *MockTranscodingProvider) GetStream(handle *plugins.StreamHandle) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, exists := m.streams[handle.SessionID]
	if !exists {
		return nil, fmt.Errorf("stream not found: %s", handle.SessionID)
	}

	return stream.reader, nil
}

// StopStream stops a streaming operation
func (m *MockTranscodingProvider) StopStream(handle *plugins.StreamHandle) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if stream, exists := m.streams[handle.SessionID]; exists {
		stream.reader.Close()
		delete(m.streams, handle.SessionID)
	}
	return nil
}

// GetDashboardSections returns dashboard sections
func (m *MockTranscodingProvider) GetDashboardSections() []plugins.DashboardSection {
	return []plugins.DashboardSection{
		{
			ID:          "mock-transcoder",
			PluginID:    "mock-ffmpeg",
			Type:        "transcoder",
			Title:       "Mock Transcoder",
			Description: "Mock transcoding statistics",
			Icon:        "film",
			Priority:    100,
		},
	}
}

// GetDashboardData returns dashboard data
func (m *MockTranscodingProvider) GetDashboardData(sectionID string) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return map[string]interface{}{
		"active_sessions": len(m.sessions),
		"total_processed": 42,
		"status":          "healthy",
	}, nil
}

// ExecuteDashboardAction executes a dashboard action
func (m *MockTranscodingProvider) ExecuteDashboardAction(actionID string, params map[string]interface{}) error {
	// Mock implementation - just return success
	return nil
}

// createMockAdaptiveFiles creates mock adaptive streaming files
func (m *MockTranscodingProvider) createMockAdaptiveFiles(sessionDir, container string) {
	if container == "dash" {
		// Create a mock DASH manifest
		manifest := `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" minBufferTime="PT1.5S" type="static" mediaPresentationDuration="PT10S">
  <Period duration="PT10S">
    <AdaptationSet mimeType="video/mp4" codecs="avc1.4d401f">
      <Representation id="1" bandwidth="3000000" width="1280" height="720">
        <BaseURL>video.mp4</BaseURL>
        <SegmentBase indexRangeExact="true" indexRange="0-1000">
          <Initialization range="0-1000"/>
        </SegmentBase>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`
		manifestPath := filepath.Join(sessionDir, "manifest.mpd")
		os.WriteFile(manifestPath, []byte(manifest), 0644)

		// Create mock segments
		for i := 0; i < 3; i++ {
			segmentPath := filepath.Join(sessionDir, fmt.Sprintf("segment%d.m4s", i))
			os.WriteFile(segmentPath, []byte(fmt.Sprintf("mock segment %d data", i)), 0644)
		}

		// Create init segment
		initPath := filepath.Join(sessionDir, "init-stream0.m4s")
		os.WriteFile(initPath, []byte("mock init segment"), 0644)
	} else if container == "hls" {
		// Create a mock HLS playlist
		playlist := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:10.0,
segment0.ts
#EXTINF:10.0,
segment1.ts
#EXTINF:10.0,
segment2.ts
#EXT-X-ENDLIST`
		playlistPath := filepath.Join(sessionDir, "playlist.m3u8")
		os.WriteFile(playlistPath, []byte(playlist), 0644)

		// Create mock segments
		for i := 0; i < 3; i++ {
			segmentPath := filepath.Join(sessionDir, fmt.Sprintf("segment%d.ts", i))
			os.WriteFile(segmentPath, []byte(fmt.Sprintf("mock segment %d data", i)), 0644)
		}
	}
}

// ConvertResolution converts resolution strings (e.g., "720p") to VideoResolution struct
func ConvertResolution(resolution string) map[string]int {
	switch resolution {
	case "480p":
		return map[string]int{"width": 854, "height": 480}
	case "720p":
		return map[string]int{"width": 1280, "height": 720}
	case "1080p":
		return map[string]int{"width": 1920, "height": 1080}
	case "1440p", "2K":
		return map[string]int{"width": 2560, "height": 1440}
	case "2160p", "4K", "UHD":
		return map[string]int{"width": 3840, "height": 2160}
	default:
		// Default to 720p if unknown
		return map[string]int{"width": 1280, "height": 720}
	}
}
