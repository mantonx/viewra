package playbackmodule

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/playbackmodule"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type TestData struct {
	VideoPath        string
	TempDir          string
	TranscodingDir   string
	ExpectedDuration int
}

type PluginInfo struct {
	ID          string
	Name        string
	Version     string
	Type        string
	Description string
	Author      string
	Status      string
}

type PluginManagerInterface interface {
	GetRunningPluginInterface(pluginID string) (interface{}, bool)
	ListPlugins() []PluginInfo
	GetRunningPlugins() []PluginInfo
}

// Helper functions
func setupTestEnvironment(t *testing.T) *TestData {
	t.Helper()
	tempDir, _ := os.MkdirTemp("", "viewra_docker_test_")
	t.Cleanup(func() { os.RemoveAll(tempDir) })
	return &TestData{VideoPath: filepath.Join(tempDir, "test.mp4"), TempDir: tempDir}
}

func setupTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	// Run migrations
	db.AutoMigrate(&database.TranscodeSession{})
	return db
}

func createTestRouter(t *testing.T, module *playbackmodule.Module) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	module.RegisterRoutes(router)
	return router
}

func createTestVideoFile(path string) error {
	// Create a minimal test file
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString("test video content")
	return err
}

// PluginManagerAdapter adapts our test interface
type PluginManagerAdapter struct {
	pluginManager PluginManagerInterface
}

func (a *PluginManagerAdapter) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	return a.pluginManager.GetRunningPluginInterface(pluginID)
}

func (a *PluginManagerAdapter) ListPlugins() []playbackmodule.PluginInfo {
	testPlugins := a.pluginManager.ListPlugins()
	var result []playbackmodule.PluginInfo
	for _, p := range testPlugins {
		result = append(result, playbackmodule.PluginInfo{
			ID: p.ID, Name: p.Name, Version: p.Version, Type: p.Type,
			Description: p.Description, Author: p.Author, Status: p.Status,
		})
	}
	return result
}

func (a *PluginManagerAdapter) GetRunningPlugins() []playbackmodule.PluginInfo {
	return a.ListPlugins()
}

// MockPluginManager for testing
type MockPluginManager struct{}

func (m *MockPluginManager) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	return &struct{}{}, true
}

func (m *MockPluginManager) ListPlugins() []PluginInfo {
	return []PluginInfo{{ID: "mock_plugin", Name: "Mock Plugin", Version: "1.0.0", Type: "transcoder", Status: "running"}}
}

func (m *MockPluginManager) GetRunningPlugins() []PluginInfo {
	return m.ListPlugins()
}

func setupPluginEnabledEnvironment(t *testing.T, db *gorm.DB) *playbackmodule.Module {
	t.Helper()
	mockPluginManager := &MockPluginManager{}
	adapter := &PluginManagerAdapter{pluginManager: mockPluginManager}
	module := playbackmodule.NewModule(db, nil, adapter)
	if err := module.Init(); err != nil {
		t.Fatalf("Failed to initialize module: %v", err)
	}
	return module
}

func createTestVideo(path string) error {
	return createTestVideoFile(path)
}

// TestableResponseWriter implements ResponseWriter for testing
type TestableResponseWriter struct {
	header     http.Header
	body       []byte
	statusCode int
}

func NewTestableResponseWriter() *TestableResponseWriter {
	return &TestableResponseWriter{
		header:     make(http.Header),
		statusCode: 200,
	}
}

func (w *TestableResponseWriter) Header() http.Header {
	return w.header
}

func (w *TestableResponseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return len(data), nil
}

func (w *TestableResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *TestableResponseWriter) Body() []byte {
	return w.body
}

func (w *TestableResponseWriter) StatusCode() int {
	return w.statusCode
}

func (w *TestableResponseWriter) Close() error {
	return nil
}

func (w *TestableResponseWriter) Code() int {
	return w.statusCode
}

func (w *TestableResponseWriter) BodyString() string {
	return string(w.body)
}

// Existing test functions continue below...
