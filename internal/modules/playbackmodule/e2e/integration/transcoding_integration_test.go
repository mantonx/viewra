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
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/playbackmodule"
	plugins "github.com/mantonx/viewra/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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

func createModuleWithPluginManager(db *gorm.DB, pluginManager PluginManagerInterface) *playbackmodule.Module {
	// Create an adapter to convert our test interface to the real plugin manager
	adapter := &PluginManagerAdapter{pluginManager: pluginManager}
	module := playbackmodule.NewModule(db, nil, adapter)
	if err := module.Init(); err != nil {
		panic(fmt.Sprintf("Failed to initialize module: %v", err))
	}
	return module
}

func createSimpleModule(db *gorm.DB) *playbackmodule.Module {
	module := playbackmodule.NewModule(db, nil, nil)
	if err := module.Init(); err != nil {
		panic(fmt.Sprintf("Failed to initialize module: %v", err))
	}
	return module
}

// PluginManagerAdapter adapts our test interface to the real plugin manager interface
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

// TestData holds test video file information
type TestData struct {
	VideoPath        string
	TempDir          string
	TranscodingDir   string
	ExpectedDuration int // in seconds
}

// TestableResponseWriter implements both ResponseWriter and CloseNotifier for tests
type TestableResponseWriter struct {
	*httptest.ResponseRecorder
	closeChan chan bool
}

func NewTestableResponseWriter() *TestableResponseWriter {
	return &TestableResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		closeChan:        make(chan bool, 1),
	}
}

func (w *TestableResponseWriter) CloseNotify() <-chan bool {
	return w.closeChan
}

func (w *TestableResponseWriter) Close() {
	select {
	case w.closeChan <- true:
	default:
	}
}

// setupTestEnvironment creates test video file and directories
func setupTestEnvironment(t *testing.T) *TestData {
	t.Helper()

	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "viewra_e2e_test_")
	require.NoError(t, err)

	// Create transcoding directory
	transcodingDir := filepath.Join(tempDir, "transcoding")
	err = os.MkdirAll(transcodingDir, 0755)
	require.NoError(t, err)

	// Create a test video file using FFmpeg (if available)
	videoPath := filepath.Join(tempDir, "test_video.mp4")
	if err := createTestVideo(t, videoPath); err != nil {
		t.Skipf("Skipping test - FFmpeg not available: %v", err)
	}

	// Set up config for testing
	cfg := config.Get()
	cfg.Transcoding.DataDir = transcodingDir

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return &TestData{
		VideoPath:        videoPath,
		TempDir:          tempDir,
		TranscodingDir:   transcodingDir,
		ExpectedDuration: 10, // 10 second test video
	}
}

// createTestVideo generates a test video file using FFmpeg
func createTestVideo(t *testing.T, outputPath string) error {
	t.Helper()

	// Check if FFmpeg is available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found in PATH")
	}

	// Create a 10-second test video with audio
	cmd := exec.Command("ffmpeg",
		"-f", "lavfi",
		"-i", "testsrc2=duration=10:size=1280x720:rate=30",
		"-f", "lavfi",
		"-i", "sine=frequency=440:duration=10",
		"-c:v", "libx264",
		"-c:a", "aac",
		"-preset", "ultrafast",
		"-y", // Overwrite output file
		outputPath,
	)

	cmd.Stderr = os.Stderr // Show FFmpeg errors if any
	return cmd.Run()
}

// setupTestDatabase creates an in-memory SQLite database for testing
func setupTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate any necessary tables
	err = db.AutoMigrate(&database.TranscodeSession{})
	require.NoError(t, err)

	return db
}

// setupPluginEnabledEnvironment sets up a test environment with mock plugin manager
func setupPluginEnabledEnvironment(t *testing.T, db *gorm.DB) *playbackmodule.Module {
	t.Helper()

	// Create a mock plugin manager adapter
	mockPluginManager := &MockPluginManager{}

	// Create plugin-enabled playback module
	module := createModuleWithPluginManager(db, mockPluginManager)

	return module
}

// MockPluginManager implements PluginManagerInterface for testing
type MockPluginManager struct{}

func (m *MockPluginManager) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	// Return a mock plugin implementation that provides a transcoding service
	return &MockPluginImpl{provider: NewMockTranscodingProvider()}, true
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

// createTestRouter sets up a Gin router with playback module routes
func createTestRouter(t *testing.T, module *playbackmodule.Module) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Register playback module routes
	module.RegisterRoutes(router)

	return router
}

// TestE2ETranscodingDASH tests the complete DASH transcoding workflow
func TestE2ETranscodingDASH(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Setup test environment
	testData := setupTestEnvironment(t)
	db := setupTestDatabase(t)

	// Set transcoding directory for testing (multiple approaches for robustness)
	// 1. Set environment variable that our mock service will use
	os.Setenv("VIEWRA_TEST_TRANSCODE_DIR", testData.TranscodingDir)

	// 2. Set config for any components that read from config
	cfg := config.Get()
	if cfg != nil {
		cfg.Transcoding.DataDir = testData.TranscodingDir
	}

	// 3. Ensure the directory exists and is writable
	err := os.MkdirAll(testData.TranscodingDir, 0755)
	require.NoError(t, err, "Should be able to create transcoding directory")

	// Create plugin-enabled playback module for realistic testing
	playbackModule := setupPluginEnabledEnvironment(t, db)

	// Create test router
	router := createTestRouter(t, playbackModule)

	// Test 1: Playback Decision - Should recommend DASH for Chrome
	t.Run("PlaybackDecision_DASH", func(t *testing.T) {
		decisionRequest := map[string]interface{}{
			"media_path": testData.VideoPath,
			"device_profile": map[string]interface{}{
				"user_agent":       "Mozilla/5.0 (Chrome/120.0)",
				"supported_codecs": []string{"h264", "aac"},
				"max_resolution":   "1080p",
				"max_bitrate":      8000,
				"supports_hevc":    false,
				"client_ip":        "127.0.0.1",
			},
		}

		body, _ := json.Marshal(decisionRequest)
		req := httptest.NewRequest("POST", "/api/playback/decide", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Chrome/120.0)")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Should recommend transcoding to DASH
		assert.True(t, response["should_transcode"].(bool))

		transcodeParams := response["transcode_params"].(map[string]interface{})
		assert.Equal(t, "dash", transcodeParams["target_container"])
		assert.Equal(t, "h264", transcodeParams["target_codec"])

		t.Logf("Playback decision: %s", response["reason"])
	})

	// Test 2: Start DASH Transcoding Session
	var sessionID string
	t.Run("StartDASHSession", func(t *testing.T) {
		transcodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
			"target_codec":     "h264",
			"target_container": "dash",
			"resolution":       "720p",
			"bitrate":          3000,
			"audio_codec":      "aac",
			"audio_bitrate":    128,
			"quality":          23,
			"preset":           "fast",
			"priority":         5,
		}

		body, _ := json.Marshal(transcodeRequest)
		req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Log the actual response if we get an error
		if w.Code != http.StatusCreated {
			t.Logf("Expected status 201, got %d", w.Code)
			t.Logf("Response body: %s", w.Body.String())
			t.Logf("Response headers: %v", w.Header())
		}

		require.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		sessionID = response["id"].(string)
		assert.NotEmpty(t, sessionID)
		assert.Equal(t, "running", response["status"])

		t.Logf("Started DASH session: %s", sessionID)
	})

	// Test 3: Wait for transcoding to initialize and check session status
	t.Run("CheckSessionStatus", func(t *testing.T) {
		require.NotEmpty(t, sessionID, "Session ID should be available from previous test")

		// Wait a bit for transcoding to start generating files
		time.Sleep(2 * time.Second)

		req := httptest.NewRequest("GET", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, sessionID, response["id"])
		status := response["status"].(string)
		assert.Contains(t, []string{"running", "complete"}, status)

		t.Logf("Session status: %s", status)
	})

	// Test 4: Verify DASH Manifest Generation
	t.Run("DASHManifestGeneration", func(t *testing.T) {
		require.NotEmpty(t, sessionID, "Session ID should be available from previous test")

		// Wait for manifest generation
		maxWait := 30 * time.Second
		checkInterval := 1 * time.Second
		deadline := time.Now().Add(maxWait)

		var manifestContent string
		for time.Now().Before(deadline) {
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s/manifest.mpd", sessionID), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				manifestContent = w.Body.String()
				break
			}

			time.Sleep(checkInterval)
		}

		require.NotEmpty(t, manifestContent, "DASH manifest should be generated within timeout")

		// Verify manifest content
		assert.Contains(t, manifestContent, "<?xml")
		assert.Contains(t, manifestContent, "<MPD")
		assert.Contains(t, manifestContent, "xmlns=\"urn:mpeg:dash:schema:mpd:2011\"")
		assert.Contains(t, manifestContent, "<Period")
		assert.Contains(t, manifestContent, "<AdaptationSet")

		t.Logf("DASH manifest generated successfully (%d bytes)", len(manifestContent))

		// Verify manifest file exists on disk
		manifestPath := filepath.Join(testData.TranscodingDir, fmt.Sprintf("session_%s", sessionID), "manifest.mpd")
		assert.FileExists(t, manifestPath)
	})

	// Test 5: Verify DASH Segment Generation
	t.Run("DASHSegmentGeneration", func(t *testing.T) {
		require.NotEmpty(t, sessionID, "Session ID should be available from previous test")

		// Wait for segments to be generated
		maxWait := 30 * time.Second
		checkInterval := 1 * time.Second
		deadline := time.Now().Add(maxWait)

		sessionDir := filepath.Join(testData.TranscodingDir, fmt.Sprintf("session_%s", sessionID))
		var segments []string

		for time.Now().Before(deadline) {
			files, err := os.ReadDir(sessionDir)
			if err == nil {
				segments = []string{}
				for _, file := range files {
					if strings.HasSuffix(file.Name(), ".m4s") || strings.HasSuffix(file.Name(), ".mp4") {
						segments = append(segments, file.Name())
					}
				}
				if len(segments) > 0 {
					break
				}
			}
			time.Sleep(checkInterval)
		}

		assert.Greater(t, len(segments), 0, "DASH segments should be generated")
		t.Logf("Generated DASH segments: %v", segments)

		// Test serving a segment
		if len(segments) > 0 {
			segmentName := segments[0]
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s/%s", sessionID, segmentName), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Segment should be servable")
			assert.Greater(t, w.Body.Len(), 0, "Segment should have content")

			contentType := w.Header().Get("Content-Type")
			assert.Contains(t, contentType, "video/", "Segment should have video content type")

			t.Logf("Segment %s served successfully (%d bytes, %s)", segmentName, w.Body.Len(), contentType)
		}
	})

	// Test 6: Session Management and Cleanup
	t.Run("SessionCleanup", func(t *testing.T) {
		require.NotEmpty(t, sessionID, "Session ID should be available from previous test")

		// List active sessions
		req := httptest.NewRequest("GET", "/api/playback/sessions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Extract sessions array from response
		var sessions []map[string]interface{}
		if sessionsData, exists := response["sessions"]; exists {
			if sessionsArray, ok := sessionsData.([]interface{}); ok {
				for _, sessionData := range sessionsArray {
					if sessionMap, ok := sessionData.(map[string]interface{}); ok {
						sessions = append(sessions, sessionMap)
					}
				}
			}
		}

		// Should find our session
		var foundSession map[string]interface{}
		for _, session := range sessions {
			if session["id"].(string) == sessionID {
				foundSession = session
				break
			}
		}
		require.NotNil(t, foundSession, "Session should be found in active sessions list")

		t.Logf("Found session in list: %s (status: %s)", foundSession["id"], foundSession["status"])

		// Stop the session
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Session should be stopped successfully")

		// Verify session is no longer active (after a brief delay)
		time.Sleep(1 * time.Second)

		req = httptest.NewRequest("GET", "/api/playback/sessions", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		// Parse response again
		response = map[string]interface{}{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Extract sessions array from response
		sessions = []map[string]interface{}{}
		if sessionsData, exists := response["sessions"]; exists {
			if sessionsArray, ok := sessionsData.([]interface{}); ok {
				for _, sessionData := range sessionsArray {
					if sessionMap, ok := sessionData.(map[string]interface{}); ok {
						sessions = append(sessions, sessionMap)
					}
				}
			}
		}

		// Session should no longer be in active list
		for _, session := range sessions {
			assert.NotEqual(t, sessionID, session["id"].(string), "Stopped session should not be in active sessions")
		}

		t.Logf("Session cleanup verified - session removed from active list")
	})
}

// TestE2ETranscodingHLS tests the complete HLS transcoding workflow
func TestE2ETranscodingHLS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Setup test environment
	testData := setupTestEnvironment(t)
	db := setupTestDatabase(t)

	// Set transcoding directory for testing (multiple approaches for robustness)
	// 1. Set environment variable that our mock service will use
	os.Setenv("VIEWRA_TEST_TRANSCODE_DIR", testData.TranscodingDir)

	// 2. Set config for any components that read from config
	cfg := config.Get()
	if cfg != nil {
		cfg.Transcoding.DataDir = testData.TranscodingDir
	}

	// 3. Ensure the directory exists and is writable
	err := os.MkdirAll(testData.TranscodingDir, 0755)
	require.NoError(t, err, "Should be able to create transcoding directory")

	// Create plugin-enabled playback module for realistic testing
	playbackModule := setupPluginEnabledEnvironment(t, db)

	// Create test router
	router := createTestRouter(t, playbackModule)

	// Test 1: Playback Decision - Should recommend HLS for Safari
	var sessionID string
	t.Run("PlaybackDecision_HLS", func(t *testing.T) {
		decisionRequest := map[string]interface{}{
			"media_path": testData.VideoPath,
			"device_profile": map[string]interface{}{
				"user_agent":       "Mozilla/5.0 (Safari/17.0)",
				"supported_codecs": []string{"h264", "aac"},
				"max_resolution":   "1080p",
				"max_bitrate":      8000,
				"supports_hevc":    false,
				"client_ip":        "127.0.0.1",
			},
		}

		body, _ := json.Marshal(decisionRequest)
		req := httptest.NewRequest("POST", "/api/playback/decide", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Safari/17.0)")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Should recommend transcoding to HLS
		assert.True(t, response["should_transcode"].(bool))

		transcodeParams := response["transcode_params"].(map[string]interface{})
		assert.Equal(t, "hls", transcodeParams["target_container"])
		assert.Equal(t, "h264", transcodeParams["target_codec"])

		t.Logf("HLS Playback decision: %s", response["reason"])
	})

	// Test 2: Start HLS Transcoding Session
	t.Run("StartHLSSession", func(t *testing.T) {
		transcodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
			"target_codec":     "h264",
			"target_container": "hls",
			"resolution":       "720p",
			"bitrate":          3000,
			"audio_codec":      "aac",
			"audio_bitrate":    128,
			"quality":          23,
			"preset":           "fast",
			"priority":         5,
		}

		body, _ := json.Marshal(transcodeRequest)
		req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Log the actual response if we get an error
		if w.Code != http.StatusCreated {
			t.Logf("Expected status 201, got %d", w.Code)
			t.Logf("Response body: %s", w.Body.String())
			t.Logf("Response headers: %v", w.Header())
		}

		require.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		sessionID = response["id"].(string)
		assert.NotEmpty(t, sessionID)
		assert.Equal(t, "running", response["status"])

		t.Logf("Started HLS session: %s", sessionID)
	})

	// Test 3: Verify HLS Playlist Generation
	t.Run("HLSPlaylistGeneration", func(t *testing.T) {
		require.NotEmpty(t, sessionID, "Session ID should be available from previous test")

		// Wait for playlist generation
		maxWait := 30 * time.Second
		checkInterval := 1 * time.Second
		deadline := time.Now().Add(maxWait)

		var playlistContent string
		for time.Now().Before(deadline) {
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s/playlist.m3u8", sessionID), nil)
			w := NewTestableResponseWriter()
			defer w.Close()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				playlistContent = w.Body.String()
				break
			}

			time.Sleep(checkInterval)
		}

		require.NotEmpty(t, playlistContent, "HLS playlist should be generated within timeout")

		// Verify playlist content
		assert.Contains(t, playlistContent, "#EXTM3U")
		assert.Contains(t, playlistContent, "#EXT-X-VERSION")
		assert.Contains(t, playlistContent, "#EXT-X-TARGETDURATION")
		assert.Contains(t, playlistContent, "#EXTINF")

		t.Logf("HLS playlist generated successfully (%d bytes)", len(playlistContent))

		// Verify playlist file exists on disk
		playlistPath := filepath.Join(testData.TranscodingDir, fmt.Sprintf("session_%s", sessionID), "playlist.m3u8")
		assert.FileExists(t, playlistPath)
	})

	// Test 4: Verify HLS Segment Generation
	t.Run("HLSSegmentGeneration", func(t *testing.T) {
		require.NotEmpty(t, sessionID, "Session ID should be available from previous test")

		// Wait for segments to be generated
		maxWait := 30 * time.Second
		checkInterval := 1 * time.Second
		deadline := time.Now().Add(maxWait)

		sessionDir := filepath.Join(testData.TranscodingDir, fmt.Sprintf("session_%s", sessionID))
		var segments []string

		for time.Now().Before(deadline) {
			files, err := os.ReadDir(sessionDir)
			if err == nil {
				segments = []string{}
				for _, file := range files {
					if strings.HasSuffix(file.Name(), ".ts") {
						segments = append(segments, file.Name())
					}
				}
				if len(segments) > 0 {
					break
				}
			}
			time.Sleep(checkInterval)
		}

		assert.Greater(t, len(segments), 0, "HLS segments should be generated")
		t.Logf("Generated HLS segments: %v", segments)

		// Test serving a segment
		if len(segments) > 0 {
			segmentName := segments[0]
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s/%s", sessionID, segmentName), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Segment should be servable")
			assert.Greater(t, w.Body.Len(), 0, "Segment should have content")

			contentType := w.Header().Get("Content-Type")
			assert.Contains(t, contentType, "video/", "Segment should have video content type")

			t.Logf("HLS segment %s served successfully (%d bytes, %s)", segmentName, w.Body.Len(), contentType)
		}
	})

	// Cleanup
	t.Run("HLSSessionCleanup", func(t *testing.T) {
		require.NotEmpty(t, sessionID, "Session ID should be available from previous test")

		// Stop the session
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "HLS session should be stopped successfully")
		t.Logf("HLS session cleanup completed")
	})
}

// TestE2ESystemIntegration tests system-level integration features
func TestE2ESystemIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E integration test in short mode")
	}

	// Setup test environment (though we don't need the video for system tests)
	_ = setupTestEnvironment(t)
	db := setupTestDatabase(t)

	// Create plugin-enabled playback module for realistic testing
	playbackModule := setupPluginEnabledEnvironment(t, db)

	// Create test router
	router := createTestRouter(t, playbackModule)

	t.Run("SystemHealth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/playback/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "healthy", response["status"])
		t.Logf("System health check passed")
	})

	t.Run("SystemStats", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/playback/stats", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Should have basic stats structure
		assert.Contains(t, response, "active_sessions")
		assert.Contains(t, response, "total_sessions")

		t.Logf("System stats: %+v", response)
	})

	t.Run("SessionsList", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/playback/sessions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Should have sessions field
		assert.Contains(t, response, "sessions")

		t.Logf("Sessions response: %+v", response)
	})
}

// BenchmarkTranscodingPerformance runs performance benchmarks
func BenchmarkTranscodingPerformance(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	// Setup test environment (only once)
	tempDir, err := os.MkdirTemp("", "viewra_benchmark_")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	videoPath := filepath.Join(tempDir, "benchmark_video.mp4")
	if err := createTestVideo(nil, videoPath); err != nil {
		b.Skipf("Skipping benchmark - FFmpeg not available: %v", err)
	}

	transcodingDir := filepath.Join(tempDir, "transcoding")
	os.MkdirAll(transcodingDir, 0755)

	cfg := config.Get()
	cfg.Transcoding.DataDir = transcodingDir

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}

	// For benchmarks, we'll use the simple version to avoid mock overhead
	module := createSimpleModule(db)

	router := gin.New()
	module.RegisterRoutes(router)

	b.ResetTimer()

	b.Run("DASHSessionCreation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			transcodeRequest := map[string]interface{}{
				"input_path":       videoPath,
				"target_codec":     "h264",
				"target_container": "dash",
				"resolution":       "720p",
				"bitrate":          3000,
				"audio_codec":      "aac",
				"preset":           "ultrafast", // Use fastest preset for benchmarking
			}

			body, _ := json.Marshal(transcodeRequest)
			req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusCreated { // Changed from StatusOK to StatusCreated
				b.Fatalf("Failed to start session: %d", w.Code)
			}

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)
			sessionID := response["id"].(string)

			// Clean up session
			req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
			w = httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}

// TestConcurrentTranscoding tests concurrent transcoding sessions
func TestConcurrentTranscoding(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	// Limit concurrent sessions for testing
	maxConcurrentSessions := 3
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		maxConcurrentSessions = 2 // Be more conservative on macOS/Windows
	}

	testData := setupTestEnvironment(t)
	db := setupTestDatabase(t)

	// Create plugin-enabled playback module for realistic testing
	playbackModule := setupPluginEnabledEnvironment(t, db)

	router := createTestRouter(t, playbackModule)

	// Start multiple concurrent sessions
	sessionIDs := make([]string, maxConcurrentSessions)
	for i := 0; i < maxConcurrentSessions; i++ {
		t.Run(fmt.Sprintf("StartConcurrentSession_%d", i), func(t *testing.T) {
			transcodeRequest := map[string]interface{}{
				"input_path":       testData.VideoPath,
				"target_codec":     "h264",
				"target_container": "dash",
				"resolution":       "720p",
				"bitrate":          2000,
				"audio_codec":      "aac",
				"preset":           "ultrafast",
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

			sessionIDs[i] = response["id"].(string)
			t.Logf("Started concurrent session %d: %s", i, sessionIDs[i])
		})
	}

	// Verify all sessions are running
	t.Run("VerifyConcurrentSessions", func(t *testing.T) {
		time.Sleep(2 * time.Second) // Allow time for sessions to initialize

		req := httptest.NewRequest("GET", "/api/playback/sessions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var sessions []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &sessions)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, len(sessions), maxConcurrentSessions, "Should have concurrent sessions running")
		t.Logf("Concurrent sessions verified: %d active", len(sessions))
	})

	// Clean up all sessions
	t.Run("CleanupConcurrentSessions", func(t *testing.T) {
		for i, sessionID := range sessionIDs {
			if sessionID != "" {
				req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code, "Session %d should be cleaned up", i)
			}
		}
		t.Logf("All concurrent sessions cleaned up")
	})
}
