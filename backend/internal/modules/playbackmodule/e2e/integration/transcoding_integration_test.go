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
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/modules/playbackmodule"
	"github.com/mantonx/viewra/pkg/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Type aliases and imports for compatibility with parent package helpers
type PlaybackModule = playbackmodule.PlaybackModule

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

func NewPlaybackModule(logger hclog.Logger, pluginManager PluginManagerInterface) *PlaybackModule {
	// Create an adapter to convert our test interface to the real plugin manager
	adapter := &PluginManagerAdapter{pluginManager: pluginManager}
	return playbackmodule.NewPlaybackModule(logger, adapter)
}

func NewSimplePlaybackModule(logger hclog.Logger, db *gorm.DB) *PlaybackModule {
	return playbackmodule.NewSimplePlaybackModule(logger, db)
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
	// (Add any required table migrations here)

	return db
}

// setupPluginEnabledEnvironment sets up a test environment with mock plugin manager
func setupPluginEnabledEnvironment(t *testing.T, db *gorm.DB) *PlaybackModule {
	t.Helper()

	logger := hclog.NewNullLogger()

	// Create a mock plugin manager adapter
	mockPluginManager := &MockPluginManager{}

	// Create plugin-enabled playback module
	playbackModule := NewPlaybackModule(logger, mockPluginManager)
	require.NoError(t, playbackModule.Initialize())

	return playbackModule
}

// MockPluginManager implements PluginManagerInterface for testing
type MockPluginManager struct{}

func (m *MockPluginManager) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	// Return a mock plugin implementation that provides a transcoding service
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
	service *MockTranscodingService
}

func (m *MockPluginImpl) TranscodingService() plugins.TranscodingService {
	if m.service == nil {
		m.service = &MockTranscodingService{
			sessions: make(map[string]*plugins.TranscodeSession),
		}
	}
	return m.service
}

// MockTranscodingService implements the TranscodingService interface for testing
type MockTranscodingService struct {
	sessions map[string]*plugins.TranscodeSession
	mu       sync.RWMutex
}

func (m *MockTranscodingService) GetCapabilities(ctx context.Context) (*plugins.TranscodingCapabilities, error) {
	return &plugins.TranscodingCapabilities{
		Name:                  "mock_ffmpeg",
		SupportedCodecs:       []string{"h264", "hevc", "vp8", "vp9"},
		SupportedResolutions:  []string{"480p", "720p", "1080p"},
		SupportedContainers:   []string{"mp4", "webm", "mkv", "dash", "hls"},
		HardwareAcceleration:  false,
		MaxConcurrentSessions: 5,
		Features: plugins.TranscodingFeatures{
			SubtitleBurnIn:      true,
			SubtitlePassthrough: true,
			MultiAudioTracks:    true,
			StreamingOutput:     true,
			SegmentedOutput:     true,
		},
		Priority: 50,
	}, nil
}

func (m *MockTranscodingService) StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*plugins.TranscodeSession, error) {
	sessionID := fmt.Sprintf("mock_%d", time.Now().UnixNano())

	// For testing, we'll simulate successful session creation
	session := &plugins.TranscodeSession{
		ID:        sessionID,
		Request:   req,
		Status:    plugins.TranscodeStatusRunning,
		Progress:  0.0,
		StartTime: time.Now(),
		Backend:   "mock_ffmpeg",
	}

	// Store session
	m.mu.Lock()
	if m.sessions == nil {
		m.sessions = make(map[string]*plugins.TranscodeSession)
	}
	m.sessions[sessionID] = session
	m.mu.Unlock()

	// Create files immediately for testing (don't use goroutine to avoid timing issues)
	m.simulateTranscoding(session)

	return session, nil
}

func (m *MockTranscodingService) GetTranscodeSession(ctx context.Context, sessionID string) (*plugins.TranscodeSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if session, exists := m.sessions[sessionID]; exists {
		return session, nil
	}

	return nil, fmt.Errorf("session not found: %s", sessionID)
}

func (m *MockTranscodingService) StopTranscode(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[sessionID]; exists {
		delete(m.sessions, sessionID)
		return nil
	}

	return fmt.Errorf("session not found: %s", sessionID)
}

func (m *MockTranscodingService) ListActiveSessions(ctx context.Context) ([]*plugins.TranscodeSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []*plugins.TranscodeSession
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (m *MockTranscodingService) GetTranscodeStream(ctx context.Context, sessionID string) (io.ReadCloser, error) {
	// Use the same directory resolution logic as simulateTranscoding
	transcodingDir := "/tmp/viewra-transcode"

	// Check if we have a test environment variable or config override
	if testDir := os.Getenv("VIEWRA_TEST_TRANSCODE_DIR"); testDir != "" {
		transcodingDir = testDir
	} else if cfg := config.Get(); cfg != nil && cfg.Transcoding.DataDir != "" {
		transcodingDir = cfg.Transcoding.DataDir
	}

	sessionDir := filepath.Join(transcodingDir, fmt.Sprintf("session_%s", sessionID))

	// Check for HLS playlist first
	playlistPath := filepath.Join(sessionDir, "playlist.m3u8")
	if data, err := os.ReadFile(playlistPath); err == nil {
		return io.NopCloser(bytes.NewReader(data)), nil
	}

	// Check for DASH manifest
	manifestPath := filepath.Join(sessionDir, "manifest.mpd")
	if data, err := os.ReadFile(manifestPath); err == nil {
		return io.NopCloser(bytes.NewReader(data)), nil
	}

	// Fallback to mock data
	return io.NopCloser(strings.NewReader("mock transcoded data")), nil
}

// simulateTranscoding simulates the transcoding process for testing
func (m *MockTranscodingService) simulateTranscoding(session *plugins.TranscodeSession) {
	// Create manifest and segment files based on container type
	time.Sleep(100 * time.Millisecond) // Small delay to simulate startup

	// Use test-specific transcoding directory that mimics Docker setup
	transcodingDir := "/tmp/viewra-transcode"

	// Check if we have a test environment variable or config override
	if testDir := os.Getenv("VIEWRA_TEST_TRANSCODE_DIR"); testDir != "" {
		transcodingDir = testDir
	} else if cfg := config.Get(); cfg != nil && cfg.Transcoding.DataDir != "" {
		transcodingDir = cfg.Transcoding.DataDir
	}

	sessionDir := filepath.Join(transcodingDir, fmt.Sprintf("session_%s", session.ID))
	err := os.MkdirAll(sessionDir, 0755)
	if err != nil {
		// If we can't create the directory, log and return - tests will fail appropriately
		return
	}

	// Create mock files based on container type
	switch session.Request.TargetContainer {
	case "dash":
		m.createMockDashFiles(sessionDir)
	case "hls":
		m.createMockHlsFiles(sessionDir)
	}
}

// createMockDashFiles creates mock DASH manifest and segments
func (m *MockTranscodingService) createMockDashFiles(sessionDir string) {
	// Create DASH manifest
	manifestContent := `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="PT10S" profiles="urn:mpeg:dash:profile:isoff-main:2011">
  <Period duration="PT10S">
    <AdaptationSet contentType="video" mimeType="video/mp4">
      <Representation id="video" bandwidth="3000000" width="1280" height="720" frameRate="30">
        <SegmentList>
          <Initialization sourceURL="init-stream0.m4s"/>
          <SegmentURL media="chunk-stream0-00001.m4s"/>
          <SegmentURL media="chunk-stream0-00002.m4s"/>
        </SegmentList>
      </Representation>
    </AdaptationSet>
    <AdaptationSet contentType="audio" mimeType="audio/mp4">
      <Representation id="audio" bandwidth="128000">
        <SegmentList>
          <Initialization sourceURL="init-stream1.m4s"/>
          <SegmentURL media="chunk-stream1-00001.m4s"/>
          <SegmentURL media="chunk-stream1-00002.m4s"/>
        </SegmentList>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`

	manifestPath := filepath.Join(sessionDir, "manifest.mpd")
	os.WriteFile(manifestPath, []byte(manifestContent), 0644)

	// Create mock initialization segments
	mockData := []byte("mock segment data")
	os.WriteFile(filepath.Join(sessionDir, "init-stream0.m4s"), mockData, 0644)
	os.WriteFile(filepath.Join(sessionDir, "init-stream1.m4s"), mockData, 0644)

	// Create mock media segments
	os.WriteFile(filepath.Join(sessionDir, "chunk-stream0-00001.m4s"), mockData, 0644)
	os.WriteFile(filepath.Join(sessionDir, "chunk-stream0-00002.m4s"), mockData, 0644)
	os.WriteFile(filepath.Join(sessionDir, "chunk-stream1-00001.m4s"), mockData, 0644)
	os.WriteFile(filepath.Join(sessionDir, "chunk-stream1-00002.m4s"), mockData, 0644)
}

// createMockHlsFiles creates mock HLS playlist and segments
func (m *MockTranscodingService) createMockHlsFiles(sessionDir string) {
	// Create HLS playlist
	playlistContent := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:4
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:4.0,
segment_00001.ts
#EXTINF:4.0,
segment_00002.ts
#EXT-X-ENDLIST`

	os.WriteFile(filepath.Join(sessionDir, "playlist.m3u8"), []byte(playlistContent), 0644)

	// Create mock segment files
	mockData := []byte("mock HLS segment data")
	os.WriteFile(filepath.Join(sessionDir, "segment_00001.ts"), mockData, 0644)
	os.WriteFile(filepath.Join(sessionDir, "segment_00002.ts"), mockData, 0644)
}

// createTestRouter sets up a Gin router with playback module routes
func createTestRouter(t *testing.T, playbackModule *PlaybackModule) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Register playback module routes
	playbackModule.RegisterRoutes(router)

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

	playbackModule := &PlaybackModule{} // Simplified for benchmark - mock will be created in setupPluginEnabledEnvironment but we'll skip it for benchmark
	// For benchmarks, we'll use the simple version to avoid mock overhead
	logger := hclog.NewNullLogger()
	playbackModule = NewSimplePlaybackModule(logger, db)
	if err := playbackModule.Initialize(); err != nil {
		b.Fatalf("Failed to initialize playback module: %v", err)
	}

	router := gin.New()
	playbackModule.RegisterRoutes(router)

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
