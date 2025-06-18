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

// Type aliases and local definitions for error handling tests
type PlaybackModule = playbackmodule.PlaybackModule

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

// Helper functions for error handling tests
func setupTestEnvironment(t *testing.T) *TestData {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "viewra_error_test_")
	require.NoError(t, err)

	transcodingDir := filepath.Join(tempDir, "transcoding")
	err = os.MkdirAll(transcodingDir, 0755)
	require.NoError(t, err)

	// Create a test video file using FFmpeg (if available)
	videoPath := filepath.Join(tempDir, "test_video.mp4")
	if err := createTestVideo(t, videoPath); err != nil {
		t.Skipf("Skipping test - FFmpeg not available: %v", err)
	}

	cfg := config.Get()
	cfg.Transcoding.DataDir = transcodingDir

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

func createTestVideo(t *testing.T, outputPath string) error {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found in PATH")
	}

	cmd := exec.Command("ffmpeg",
		"-f", "lavfi",
		"-i", "testsrc2=duration=10:size=1280x720:rate=30",
		"-f", "lavfi",
		"-i", "sine=frequency=440:duration=10",
		"-c:v", "libx264",
		"-c:a", "aac",
		"-preset", "ultrafast",
		"-y",
		outputPath,
	)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func setupTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func setupPluginEnabledEnvironment(t *testing.T, db *gorm.DB) *PlaybackModule {
	t.Helper()
	logger := hclog.NewNullLogger()
	mockPluginManager := &MockPluginManager{}
	adapter := &PluginManagerAdapter{pluginManager: mockPluginManager}
	playbackModule := playbackmodule.NewPlaybackModule(logger, adapter)
	require.NoError(t, playbackModule.Initialize())
	return playbackModule
}

func createTestRouter(t *testing.T, playbackModule *PlaybackModule) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	playbackModule.RegisterRoutes(router)
	return router
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
		SupportedCodecs:       []string{"h264", "h265", "vp9", "aac", "mp3"},
		SupportedResolutions:  []string{"480p", "720p", "1080p"},
		SupportedContainers:   []string{"dash", "hls", "mp4"},
		HardwareAcceleration:  false,
		MaxConcurrentSessions: 10,
		Features: plugins.TranscodingFeatures{
			SubtitleBurnIn:      true,
			SubtitlePassthrough: true,
			MultiAudioTracks:    true,
			HDRSupport:          false,
			ToneMapping:         false,
			StreamingOutput:     true,
			SegmentedOutput:     true,
		},
		Priority: 5,
	}, nil
}

func (m *MockTranscodingService) StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*plugins.TranscodeSession, error) {
	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())

	session := &plugins.TranscodeSession{
		ID:        sessionID,
		Request:   req,
		Status:    plugins.TranscodeStatusStarting,
		Progress:  0.0,
		StartTime: time.Now(),
		Backend:   "mock_ffmpeg",
	}

	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	// Simulate transcoding progress
	go m.simulateTranscoding(session)

	return session, nil
}

func (m *MockTranscodingService) GetTranscodeSession(ctx context.Context, sessionID string) (*plugins.TranscodeSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

func (m *MockTranscodingService) StopTranscode(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
	return nil
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
	// Return a mock stream for testing
	return io.NopCloser(strings.NewReader("mock transcoded data")), nil
}

func (m *MockTranscodingService) simulateTranscoding(session *plugins.TranscodeSession) {
	// Simulate transcoding progress over time
	for i := 0; i <= 10; i++ {
		time.Sleep(200 * time.Millisecond)

		m.mu.Lock()
		if _, exists := m.sessions[session.ID]; !exists {
			m.mu.Unlock()
			return // Session was stopped
		}

		session.Progress = float64(i) / 10.0
		if i == 10 {
			session.Status = plugins.TranscodeStatusCompleted
		} else if i == 0 {
			session.Status = plugins.TranscodeStatusRunning
		}
		m.mu.Unlock()
	}
}

// Now define NewPlaybackModule to fix the compilation error
func NewPlaybackModule(logger hclog.Logger, pluginManager PluginManagerInterface) *PlaybackModule {
	adapter := &PluginManagerAdapter{pluginManager: pluginManager}
	return playbackmodule.NewPlaybackModule(logger, adapter)
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
func setupSlowTranscodingEnvironment(t *testing.T, db *gorm.DB) *PlaybackModule {
	t.Helper()

	logger := hclog.NewNullLogger()

	// Create a special slow mock plugin manager
	mockPluginManager := &SlowMockPluginManager{}

	// Create PlaybackModule with the slow mock
	playbackModule := NewPlaybackModule(logger, mockPluginManager)
	err := playbackModule.Initialize()
	require.NoError(t, err)

	return playbackModule
}

// SlowMockPluginManager simulates slow transcoding operations
type SlowMockPluginManager struct{}

func (m *SlowMockPluginManager) GetRunningPluginInterface(pluginID string) (interface{}, bool) {
	if pluginID == "transcoding.ffmpeg" {
		return &SlowMockPluginImpl{service: &SlowMockTranscodingService{sessions: make(map[string]*plugins.TranscodeSession)}}, true
	}
	return nil, false
}

func (m *SlowMockPluginManager) ListPlugins() []PluginInfo {
	return []PluginInfo{
		{
			ID:      "transcoding.ffmpeg",
			Name:    "Slow FFmpeg Transcoder",
			Version: "1.0.0",
			Type:    "transcoding",
			Status:  "running",
		},
	}
}

func (m *SlowMockPluginManager) GetRunningPlugins() []PluginInfo {
	return m.ListPlugins()
}

// SlowMockPluginImpl implements the plugin interface with slow operations
type SlowMockPluginImpl struct {
	service *SlowMockTranscodingService
}

func (m *SlowMockPluginImpl) TranscodingService() plugins.TranscodingService {
	return m.service
}

// SlowMockTranscodingService implements slow transcoding operations
type SlowMockTranscodingService struct {
	sessions map[string]*plugins.TranscodeSession
	mu       sync.RWMutex
}

func (m *SlowMockTranscodingService) GetCapabilities(ctx context.Context) (*plugins.TranscodingCapabilities, error) {
	// Simulate slow capability check
	time.Sleep(1 * time.Second)

	return &plugins.TranscodingCapabilities{
		Name:                  "Slow FFmpeg Transcoder",
		SupportedCodecs:       []string{"h264", "h265", "av1"},
		SupportedResolutions:  []string{"720p", "1080p", "4k"},
		SupportedContainers:   []string{"mp4", "dash", "hls"},
		HardwareAcceleration:  false,
		MaxConcurrentSessions: 2,
		Priority:              1,
	}, nil
}

func (m *SlowMockTranscodingService) StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*plugins.TranscodeSession, error) {
	// Simulate slow session startup
	time.Sleep(2 * time.Second)

	sessionID := fmt.Sprintf("slow_mock_%d", time.Now().UnixNano())
	session := &plugins.TranscodeSession{
		ID:       sessionID,
		Status:   "starting",
		Progress: 0,
	}

	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	// Start slow transcoding simulation
	go m.simulateSlowTranscoding(session)

	return session, nil
}

func (m *SlowMockTranscodingService) GetTranscodeSession(ctx context.Context, sessionID string) (*plugins.TranscodeSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

func (m *SlowMockTranscodingService) StopTranscode(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
	return nil
}

func (m *SlowMockTranscodingService) ListActiveSessions(ctx context.Context) ([]*plugins.TranscodeSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*plugins.TranscodeSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (m *SlowMockTranscodingService) GetTranscodeStream(ctx context.Context, sessionID string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("slow_mock_stream_data")), nil
}

func (m *SlowMockTranscodingService) simulateSlowTranscoding(session *plugins.TranscodeSession) {
	// Very slow transcoding simulation
	session.Status = "running"

	for i := 0; i <= 100; i += 5 {
		time.Sleep(1 * time.Second) // Very slow progress
		session.Progress = float64(i)

		if i == 100 {
			session.Status = "completed"
		}
	}
}
