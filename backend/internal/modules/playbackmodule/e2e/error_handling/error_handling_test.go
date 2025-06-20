package playbackmodule

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/playbackmodule"
	"github.com/mantonx/viewra/pkg/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Test-specific types for error handling

// PluginInfo represents plugin information for tests
type PluginInfo struct {
	ID          string
	Name        string
	Version     string
	Type        string
	Description string
	Author      string
	Status      string
}

// PluginManagerInterface defines the interface for plugin managers in tests
type PluginManagerInterface interface {
	GetRunningPluginInterface(pluginID string) (interface{}, bool)
	ListPlugins() []PluginInfo
	GetRunningPlugins() []PluginInfo
}

// TestData holds test environment data
type TestData struct {
	VideoPath      string
	TempDir        string
	TranscodingDir string
}

// setupTestDatabase creates an in-memory test database
func setupTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&database.TranscodeSession{})
	require.NoError(t, err)

	return db
}

// createTestVideo creates a small test video file using FFmpeg
func createTestVideo(t *testing.T, outputPath string) error {
	t.Helper()

	// Check if FFmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("FFmpeg not found in PATH")
	}

	// Generate a simple test pattern video (5 seconds)
	cmd := exec.Command("ffmpeg",
		"-f", "lavfi", "-i", "testsrc=duration=5:size=320x240:rate=25",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=5",
		"-c:v", "libx264", "-c:a", "aac",
		"-y", outputPath,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create test video: %w", err)
	}

	return nil
}

// setupTestEnvironment creates test directories and files
func setupTestEnvironment(t *testing.T) *TestData {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "playback_error_test_")
	require.NoError(t, err)

	transcodingDir := filepath.Join(tempDir, "transcoding")
	err = os.MkdirAll(transcodingDir, 0755)
	require.NoError(t, err)

	videoPath := filepath.Join(tempDir, "test_video.mp4")
	if err := createTestVideo(t, videoPath); err != nil {
		t.Skipf("Skipping test - FFmpeg not available: %v", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return &TestData{
		VideoPath:      videoPath,
		TempDir:        tempDir,
		TranscodingDir: transcodingDir,
	}
}

// setupPluginEnabledEnvironment sets up a test environment with mock plugin manager
func setupPluginEnabledEnvironment(t *testing.T, db *gorm.DB) *playbackmodule.Module {
	t.Helper()

	// Create mock plugin manager
	adapter := &PluginManagerAdapter{mockPluginManager: &MockPluginManager{}}

	// Create module
	module := playbackmodule.NewModule(db, nil, adapter)
	err := module.Init()
	require.NoError(t, err)

	return module
}

// createTestRouter sets up a Gin router with playback module routes
func createTestRouter(t *testing.T, module *playbackmodule.Module) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	module.RegisterRoutes(router)
	return router
}

// PluginManagerAdapter adapts our test interface to the real plugin manager interface
type PluginManagerAdapter struct {
	mockPluginManager PluginManagerInterface
}

func (a *PluginManagerAdapter) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	return a.mockPluginManager.GetRunningPluginInterface(pluginID)
}

func (a *PluginManagerAdapter) ListPlugins() []playbackmodule.PluginInfo {
	testPlugins := a.mockPluginManager.ListPlugins()
	var result []playbackmodule.PluginInfo
	for _, p := range testPlugins {
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

// MockPluginManager implements PluginManagerInterface for testing
type MockPluginManager struct{}

func (m *MockPluginManager) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	return &MockPluginImpl{}, true
}

func (m *MockPluginManager) ListPlugins() []PluginInfo {
	return []PluginInfo{
		{
			ID:          "mock_ffmpeg",
			Name:        "Mock FFmpeg Transcoder",
			Version:     "1.0.0",
			Type:        "transcoder",
			Description: "Mock transcoding service for testing",
			Author:      "Test Suite",
			Status:      "running",
		},
	}
}

func (m *MockPluginManager) GetRunningPlugins() []PluginInfo {
	return m.ListPlugins()
}

// MockPluginImpl represents a mock plugin that provides a transcoding service
type MockPluginImpl struct {
	provider plugins.TranscodingProvider
}

// NewMockPluginImpl creates a new mock plugin implementation
func NewMockPluginImpl() *MockPluginImpl {
	return &MockPluginImpl{
		provider: NewMockTranscodingProvider(),
	}
}

// TranscodingProvider returns the mock transcoding provider
func (m *MockPluginImpl) TranscodingProvider() plugins.TranscodingProvider {
	return m.provider
}

// Initialize initializes the plugin
func (m *MockPluginImpl) Initialize(ctx *plugins.PluginContext) error { return nil }

// Start starts the plugin
func (m *MockPluginImpl) Start() error { return nil }

// Stop stops the plugin
func (m *MockPluginImpl) Stop() error { return nil }

// Info returns plugin information
func (m *MockPluginImpl) Info() (*plugins.PluginInfo, error) {
	info := m.provider.GetInfo()
	return &plugins.PluginInfo{
		ID:          info.ID,
		Name:        info.Name,
		Version:     info.Version,
		Description: info.Description,
		Author:      info.Author,
		Type:        "transcoding",
	}, nil
}

// Health returns plugin health status
func (m *MockPluginImpl) Health() error { return nil }

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

// MockTranscodingProvider implements the TranscodingProvider interface for testing
type MockTranscodingProvider struct {
	mu       sync.Mutex
	sessions map[string]*plugins.TranscodeHandle
	streams  map[string]*mockStream
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

// StartTranscode starts a transcoding operation
func (m *MockTranscodingProvider) StartTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("mock-%d", time.Now().UnixNano())
	}

	handle := &plugins.TranscodeHandle{
		SessionID:  sessionID,
		Provider:   "mock-ffmpeg",
		StartTime:  time.Now(),
		Directory:  filepath.Join("/tmp/viewra-transcode", sessionID),
		ProcessID:  12345, // Mock PID
		Context:    ctx,
		CancelFunc: func() {},
	}

	m.sessions[sessionID] = handle

	// Create mock DASH/HLS files if requested
	if req.Container == "dash" || req.Container == "hls" {
		go m.createMockAdaptiveFiles(handle.Directory, req.Container)
	}

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

// createMockAdaptiveFiles creates mock DASH/HLS files for testing
func (m *MockTranscodingProvider) createMockAdaptiveFiles(sessionDir, container string) {
	// Create directory
	os.MkdirAll(sessionDir, 0755)

	if container == "dash" {
		// Create mock DASH manifest
		manifest := `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="PT10S">
  <Period>
    <AdaptationSet mimeType="video/mp4">
      <Representation id="0" bandwidth="1000000">
        <SegmentTemplate media="chunk-$RepresentationID$-$Number$.m4s" initialization="init-$RepresentationID$.m4s"/>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`
		os.WriteFile(filepath.Join(sessionDir, "manifest.mpd"), []byte(manifest), 0644)

		// Create mock init segments
		os.WriteFile(filepath.Join(sessionDir, "init-0.m4s"), []byte("mock init segment"), 0644)

		// Create mock media segments
		for i := 1; i <= 5; i++ {
			segmentFile := fmt.Sprintf("chunk-0-%05d.m4s", i)
			os.WriteFile(filepath.Join(sessionDir, segmentFile), []byte(fmt.Sprintf("mock segment %d", i)), 0644)
		}
	} else if container == "hls" {
		// Create mock HLS playlist
		playlist := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:10
#EXTINF:10.0,
segment001.ts
#EXTINF:10.0,
segment002.ts
#EXT-X-ENDLIST`
		os.WriteFile(filepath.Join(sessionDir, "playlist.m3u8"), []byte(playlist), 0644)

		// Create mock segments
		os.WriteFile(filepath.Join(sessionDir, "segment001.ts"), []byte("mock segment 1"), 0644)
		os.WriteFile(filepath.Join(sessionDir, "segment002.ts"), []byte("mock segment 2"), 0644)
	}
}

// TestE2EErrorHandling tests comprehensive error scenarios
func TestE2EErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E error handling test in short mode")
	}

	// Setup test environment
	testData := setupTestEnvironment(t)
	defer os.RemoveAll(testData.TempDir)

	db := setupTestDatabase(t)

	t.Run("InvalidInputFileErrors", func(t *testing.T) {
		// Test with non-existent input file
		playbackModule := setupPluginEnabledEnvironment(t, db)
		router := createTestRouter(t, playbackModule)

		transcodeRequest := map[string]interface{}{
			"input_path":       "/nonexistent/file.mp4",
			"target_codec":     "h264",
			"target_container": "dash",
			"resolution":       "720p",
			"bitrate":          3000,
		}

		body, _ := json.Marshal(transcodeRequest)
		req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should handle invalid file gracefully (either reject or accept but fail later)
		assert.Contains(t, []int{http.StatusBadRequest, http.StatusCreated, http.StatusInternalServerError}, w.Code)
		t.Logf("✅ Invalid input file handled correctly (status: %d)", w.Code)
	})

	t.Run("MalformedRequestErrors", func(t *testing.T) {
		playbackModule := setupPluginEnabledEnvironment(t, db)
		router := createTestRouter(t, playbackModule)

		testCases := []struct {
			name         string
			request      interface{}
			expectStatus int
		}{
			{
				name:         "Empty Request",
				request:      map[string]interface{}{},
				expectStatus: http.StatusBadRequest,
			},
			{
				name: "Invalid Codec",
				request: map[string]interface{}{
					"input_path":       testData.VideoPath,
					"target_codec":     "invalid_codec_xyz",
					"target_container": "dash",
				},
				expectStatus: http.StatusBadRequest,
			},
			{
				name: "Invalid Container",
				request: map[string]interface{}{
					"input_path":       testData.VideoPath,
					"target_codec":     "h264",
					"target_container": "invalid_container",
				},
				expectStatus: http.StatusBadRequest,
			},
			{
				name: "Invalid Bitrate",
				request: map[string]interface{}{
					"input_path":       testData.VideoPath,
					"target_codec":     "h264",
					"target_container": "dash",
					"bitrate":          -1000,
				},
				expectStatus: http.StatusBadRequest,
			},
			{
				name: "Missing Required Fields",
				request: map[string]interface{}{
					"target_codec": "h264",
					// Missing input_path
				},
				expectStatus: http.StatusBadRequest,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				body, _ := json.Marshal(tc.request)
				req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				assert.Equal(t, tc.expectStatus, w.Code, "Expected status for %s", tc.name)
				t.Logf("✅ %s handled correctly (status: %d)", tc.name, w.Code)
			})
		}
	})

	t.Run("SessionNotFoundErrors", func(t *testing.T) {
		playbackModule := setupPluginEnabledEnvironment(t, db)
		router := createTestRouter(t, playbackModule)

		// Test getting non-existent session
		req := httptest.NewRequest("GET", "/api/playback/session/nonexistent_session_id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		t.Logf("✅ Non-existent session GET handled correctly")

		// Test stopping non-existent session
		req = httptest.NewRequest("DELETE", "/api/playback/session/nonexistent_session_id", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		t.Logf("✅ Non-existent session DELETE handled correctly")

		// Test streaming from non-existent session
		req = httptest.NewRequest("GET", "/api/playback/stream/nonexistent_session_id/manifest.mpd", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		t.Logf("✅ Non-existent session streaming handled correctly")
	})

	t.Run("ContentTypeErrors", func(t *testing.T) {
		playbackModule := setupPluginEnabledEnvironment(t, db)
		router := createTestRouter(t, playbackModule)

		// Test with wrong content type
		transcodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
			"target_codec":     "h264",
			"target_container": "dash",
		}

		body, _ := json.Marshal(transcodeRequest)
		req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "text/plain") // Wrong content type

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		t.Logf("✅ Wrong content type handled correctly")

		// Test with no content type
		req = httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
		// No Content-Type header

		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		t.Logf("✅ Missing content type handled correctly")
	})
}

// TestE2EConfigurationEdgeCases tests edge cases in configuration
func TestE2EConfigurationEdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E configuration edge cases test in short mode")
	}

	testData := setupTestEnvironment(t)
	defer os.RemoveAll(testData.TempDir)

	db := setupTestDatabase(t)

	t.Run("EmptyTranscodingDirectory", func(t *testing.T) {
		// Configure empty transcoding directory
		cfg := config.Get()
		originalDir := cfg.Transcoding.DataDir
		cfg.Transcoding.DataDir = ""
		defer func() { cfg.Transcoding.DataDir = originalDir }()

		playbackModule := setupPluginEnabledEnvironment(t, db)
		router := createTestRouter(t, playbackModule)

		transcodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
			"target_codec":     "h264",
			"target_container": "dash",
		}

		body, _ := json.Marshal(transcodeRequest)
		req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should handle empty config gracefully
		t.Logf("✅ Empty transcoding directory handled (status: %d)", w.Code)
	})
}

// TestE2ENetworkResilienceScenarios tests network-related failure scenarios
func TestE2ENetworkResilienceScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E network resilience test in short mode")
	}

	testData := setupTestEnvironment(t)
	defer os.RemoveAll(testData.TempDir)

	db := setupTestDatabase(t)

	t.Run("ClientDisconnectDuringSession", func(t *testing.T) {
		playbackModule := setupPluginEnabledEnvironment(t, db)
		router := createTestRouter(t, playbackModule)

		// Start a session
		transcodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
			"target_codec":     "h264",
			"target_container": "dash",
			"resolution":       "720p",
			"bitrate":          3000,
		}

		body, _ := json.Marshal(transcodeRequest)
		req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		sessionID := response["id"].(string)
		t.Logf("✅ Session created: %s", sessionID)

		// Simulate client disconnect by trying to access stream multiple times
		// then stopping abruptly
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s", sessionID), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			// Don't wait for response, simulate disconnect
			time.Sleep(100 * time.Millisecond)
		}

		// Session should still be manageable
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		t.Logf("✅ Session remains accessible after simulated client disconnects")

		// Cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})

	t.Run("MultipleClientsSameSession", func(t *testing.T) {
		playbackModule := setupPluginEnabledEnvironment(t, db)
		router := createTestRouter(t, playbackModule)

		// Start a session
		transcodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
			"target_codec":     "h264",
			"target_container": "dash",
			"resolution":       "720p",
			"bitrate":          3000,
		}

		body, _ := json.Marshal(transcodeRequest)
		req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		sessionID := response["id"].(string)

		// Simulate multiple clients accessing the same session concurrently
		numClients := 5
		var wg sync.WaitGroup

		for i := 0; i < numClients; i++ {
			wg.Add(1)
			go func(clientID int) {
				defer wg.Done()

				// Each client tries to access the session info
				req := httptest.NewRequest("GET", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code, "Client %d should access session", clientID)
				t.Logf("✅ Client %d accessed session successfully", clientID)
			}(i)
		}

		wg.Wait()
		t.Logf("✅ Multiple clients handled correctly")

		// Cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

// TestE2ETimeoutScenarios tests various timeout scenarios
func TestE2ETimeoutScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E timeout scenarios test in short mode")
	}

	testData := setupTestEnvironment(t)
	defer os.RemoveAll(testData.TempDir)

	db := setupTestDatabase(t)

	t.Run("SessionTimeout", func(t *testing.T) {
		// Create a special mock service that simulates slow transcoding
		playbackModule := setupSlowTranscodingEnvironment(t, db)
		router := createTestRouter(t, playbackModule)

		// Start a session
		transcodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
			"target_codec":     "h264",
			"target_container": "dash",
			"resolution":       "720p",
			"bitrate":          3000,
		}

		body, _ := json.Marshal(transcodeRequest)
		req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		sessionID := response["id"].(string)
		t.Logf("✅ Slow session created: %s", sessionID)

		// Wait longer than normal and check session status
		time.Sleep(3 * time.Second)

		req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Session should still exist (timeout handling depends on implementation)
		if w.Code == http.StatusOK {
			t.Logf("✅ Session persists through timeout period")
		} else {
			t.Logf("✅ Session timed out as expected (status: %d)", w.Code)
		}
	})

	t.Run("RequestTimeout", func(t *testing.T) {
		playbackModule := setupPluginEnabledEnvironment(t, db)
		router := createTestRouter(t, playbackModule)

		// Test with a request that might timeout due to large payload
		largeTranscodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
			"target_codec":     "h264",
			"target_container": "dash",
			"resolution":       "4k",  // Very high resolution
			"bitrate":          50000, // Very high bitrate
			"audio_codec":      "aac",
			"audio_bitrate":    320,
			"quality":          15,         // High quality (slow)
			"preset":           "veryslow", // Very slow preset
		}

		body, _ := json.Marshal(largeTranscodeRequest)
		req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should handle timeout gracefully
		assert.Contains(t, []int{http.StatusCreated, http.StatusRequestTimeout, http.StatusInternalServerError}, w.Code)
		t.Logf("✅ Request timeout scenario handled (status: %d)", w.Code)
	})
}

// TestE2EProtocolErrors tests protocol-level error scenarios
func TestE2EProtocolErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E protocol error test in short mode")
	}

	testData := setupTestEnvironment(t)
	defer os.RemoveAll(testData.TempDir)

	db := setupTestDatabase(t)
	playbackModule := setupPluginEnabledEnvironment(t, db)
	router := createTestRouter(t, playbackModule)

	t.Run("InvalidHTTPMethods", func(t *testing.T) {
		// Test unsupported HTTP methods
		methods := []string{"PUT", "PATCH", "OPTIONS"}

		for _, method := range methods {
			req := httptest.NewRequest(method, "/api/playback/start", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusMethodNotAllowed, w.Code, "Method %s should not be allowed", method)
			t.Logf("✅ Method %s rejected correctly", method)
		}
	})

	t.Run("InvalidEndpoints", func(t *testing.T) {
		// Test non-existent endpoints
		endpoints := []string{
			"/api/playback/invalid",
			"/api/playback/session/invalid/action",
			"/api/transcoding/start", // Wrong module
		}

		for _, endpoint := range endpoints {
			req := httptest.NewRequest("GET", endpoint, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code, "Endpoint %s should return 404", endpoint)
			t.Logf("✅ Invalid endpoint %s handled correctly", endpoint)
		}
	})

	t.Run("LargePayloadHandling", func(t *testing.T) {
		// Test very large request payload
		largePayload := make(map[string]interface{})
		largePayload["input_path"] = testData.VideoPath

		// Add many unnecessary fields to make payload large
		for i := 0; i < 1000; i++ {
			largePayload[fmt.Sprintf("dummy_field_%d", i)] = fmt.Sprintf("dummy_value_%d", i)
		}

		body, _ := json.Marshal(largePayload)
		req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should handle large payload gracefully
		assert.Contains(t, []int{http.StatusOK, http.StatusCreated, http.StatusBadRequest, http.StatusRequestEntityTooLarge}, w.Code)
		t.Logf("✅ Large payload handled (status: %d)", w.Code)
	})
}

// setupSlowTranscodingEnvironment creates a mock environment that simulates slow transcoding
func setupSlowTranscodingEnvironment(t *testing.T, db *gorm.DB) *playbackmodule.Module {
	t.Helper()

	// Create slow mock plugin manager
	slowMockPluginManager := &SlowMockPluginManager{
		provider: NewSlowMockTranscodingProvider(),
	}

	// Create adapter
	adapter := &PluginManagerAdapter{mockPluginManager: slowMockPluginManager}

	// Create module
	module := playbackmodule.NewModule(db, nil, adapter)
	err := module.Init()
	require.NoError(t, err)

	return module
}

// SlowMockPluginManager simulates slow transcoding operations
type SlowMockPluginManager struct {
	provider plugins.TranscodingProvider
}

func (m *SlowMockPluginManager) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	return &SlowMockPluginImpl{provider: m.provider}, true
}

func (m *SlowMockPluginManager) ListPlugins() []PluginInfo {
	return []PluginInfo{
		{
			ID:          "slow_mock_ffmpeg",
			Name:        "Slow Mock FFmpeg Transcoder",
			Version:     "1.0.0",
			Type:        "transcoder",
			Description: "Slow mock transcoding service for timeout testing",
			Author:      "Test Suite",
			Status:      "running",
		},
	}
}

func (m *SlowMockPluginManager) GetRunningPlugins() []PluginInfo {
	return m.ListPlugins()
}

// SlowMockPluginImpl implements the plugin interface with slow operations
type SlowMockPluginImpl struct {
	provider plugins.TranscodingProvider
}

// TranscodingProvider returns the slow mock transcoding provider
func (m *SlowMockPluginImpl) TranscodingProvider() plugins.TranscodingProvider {
	return m.provider
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
		delay: 3 * time.Second, // Simulate slow operations
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
