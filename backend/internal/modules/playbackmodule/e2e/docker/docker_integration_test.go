package playbackmodule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2EDockerTranscodingIntegration tests the complete transcoding workflow
// with Docker-style directory mounting and configuration
func TestE2EDockerTranscodingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker integration test in short mode")
	}

	// Setup Docker-style environment
	testData := setupDockerStyleEnvironment(t)
	defer cleanupDockerEnvironment(t, testData)

	// Create database
	db := setupTestDatabase(t)

	// CRITICAL: Configure transcoding BEFORE creating playback module
	configureDockerStyleTranscoding(t, testData)

	// Create plugin-enabled playback module AFTER config is set
	playbackModule := setupPluginEnabledEnvironment(t, db)

	// Create test router
	router := createTestRouter(t, playbackModule)

	t.Run("DockerVolumeAccessibility", func(t *testing.T) {
		// Verify we can write to the transcoding directory
		testFile := filepath.Join(testData.TranscodingDir, "test_write.tmp")
		err := os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err, "Should be able to write to Docker-mounted transcoding directory")

		// Verify we can read from it
		data, err := os.ReadFile(testFile)
		require.NoError(t, err, "Should be able to read from Docker-mounted transcoding directory")
		assert.Equal(t, "test", string(data))

		// Cleanup
		os.Remove(testFile)

		t.Logf("✅ Docker volume accessibility verified: %s", testData.TranscodingDir)
	})

	// Test complete DASH transcoding workflow with Docker volumes
	var sessionID string
	t.Run("DockerDASHWorkflow", func(t *testing.T) {
		// 1. Start DASH transcoding session
		transcodeRequest := map[string]interface{}{
			"input_path":    testData.VideoPath,
			"video_codec":   "h264",
			"container":     "dash",
			"resolution":    map[string]int{"width": 1280, "height": 720},
			"bitrate":       3000,
			"audio_codec":   "aac",
			"audio_bitrate": 128,
			"quality":       23,
			"preset":        "fast",
			"priority":      5,
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

		sessionID = response["id"].(string)
		assert.NotEmpty(t, sessionID)

		t.Logf("✅ DASH session created: %s", sessionID)

		// 2. Verify session directory was created in Docker volume
		sessionDir := filepath.Join(testData.TranscodingDir, fmt.Sprintf("session_%s", sessionID))
		assert.DirExists(t, sessionDir, "Session directory should exist in Docker volume")

		// 3. Verify DASH manifest was created
		manifestPath := filepath.Join(sessionDir, "manifest.mpd")
		assert.FileExists(t, manifestPath, "DASH manifest should be created in Docker volume")

		// 4. Verify manifest content
		manifestData, err := os.ReadFile(manifestPath)
		require.NoError(t, err)
		manifestContent := string(manifestData)

		assert.Contains(t, manifestContent, "<?xml")
		assert.Contains(t, manifestContent, "<MPD")
		assert.Contains(t, manifestContent, "xmlns=\"urn:mpeg:dash:schema:mpd:2011\"")

		t.Logf("✅ DASH manifest created successfully (%d bytes)", len(manifestContent))

		// 5. Verify segments were created
		segmentFiles, err := filepath.Glob(filepath.Join(sessionDir, "*.m4s"))
		require.NoError(t, err)
		assert.Greater(t, len(segmentFiles), 0, "DASH segments should be created")

		t.Logf("✅ DASH segments created: %v", segmentFiles)
	})

	// Test manifest serving through API
	t.Run("DockerManifestServing", func(t *testing.T) {
		require.NotEmpty(t, sessionID, "Session ID should be available")

		// Debug: Check what config directory is being used
		cfg := config.Get()
		expectedManifestPath := filepath.Join(cfg.Transcoding.DataDir, fmt.Sprintf("session_%s", sessionID), "manifest.mpd")
		actualManifestPath := filepath.Join(testData.TranscodingDir, fmt.Sprintf("session_%s", sessionID), "manifest.mpd")

		t.Logf("🔍 Config transcoding dir: %s", cfg.Transcoding.DataDir)
		t.Logf("🔍 Test transcoding dir: %s", testData.TranscodingDir)
		t.Logf("🔍 Expected manifest path: %s", expectedManifestPath)
		t.Logf("🔍 Actual manifest path: %s", actualManifestPath)

		// Check if actual file exists
		if _, err := os.Stat(actualManifestPath); err == nil {
			t.Logf("✅ Manifest file exists at actual path")
		} else {
			t.Logf("❌ Manifest file missing at actual path: %v", err)
		}

		// Check if expected file exists
		if _, err := os.Stat(expectedManifestPath); err == nil {
			t.Logf("✅ Manifest file exists at expected path")
		} else {
			t.Logf("❌ Manifest file missing at expected path: %v", err)
		}

		// Test DASH manifest endpoint
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s/manifest.mpd", sessionID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Logf("❌ Manifest request failed with status %d", w.Code)
			t.Logf("❌ Response body: %s", w.Body.String())
		}

		require.Equal(t, http.StatusOK, w.Code)

		manifestContent := w.Body.String()
		assert.Contains(t, manifestContent, "<MPD")
		assert.Contains(t, manifestContent, "xmlns=\"urn:mpeg:dash:schema:mpd:2011\"")

		// Verify content type
		assert.Equal(t, "application/dash+xml", w.Header().Get("Content-Type"))

		t.Logf("✅ DASH manifest served successfully via API (%d bytes)", len(manifestContent))
	})

	// Test segment serving through API
	t.Run("DockerSegmentServing", func(t *testing.T) {
		require.NotEmpty(t, sessionID, "Session ID should be available")

		// Test serving a specific segment
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s/init-stream0.m4s", sessionID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Greater(t, w.Body.Len(), 0, "Segment should have content")

		// Verify content type for DASH segments
		contentType := w.Header().Get("Content-Type")
		assert.Contains(t, contentType, "video/", "Segment should have video content type")

		t.Logf("✅ DASH segment served successfully (%d bytes, %s)", w.Body.Len(), contentType)
	})

	// Test complete HLS workflow
	t.Run("DockerHLSWorkflow", func(t *testing.T) {
		// Start HLS transcoding session
		transcodeRequest := map[string]interface{}{
			"input_path":    testData.VideoPath,
			"video_codec":   "h264",
			"container":     "hls",
			"resolution":    map[string]int{"width": 1280, "height": 720},
			"bitrate":       3000,
			"audio_codec":   "aac",
			"audio_bitrate": 128,
			"quality":       23,
			"preset":        "fast",
			"priority":      5,
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

		hlsSessionID := response["id"].(string)
		assert.NotEmpty(t, hlsSessionID)

		t.Logf("✅ HLS session created: %s", hlsSessionID)

		// Verify HLS playlist was created
		sessionDir := filepath.Join(testData.TranscodingDir, fmt.Sprintf("session_%s", hlsSessionID))
		playlistPath := filepath.Join(sessionDir, "playlist.m3u8")
		assert.FileExists(t, playlistPath, "HLS playlist should be created in Docker volume")

		// Test HLS playlist serving
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s/playlist.m3u8", hlsSessionID), nil)
		hlsW := NewTestableResponseWriter()
		defer hlsW.Close()
		router.ServeHTTP(hlsW, req)

		require.Equal(t, http.StatusOK, hlsW.Code)

		playlistContent := hlsW.BodyString()
		assert.Contains(t, playlistContent, "#EXTM3U")
		assert.Contains(t, playlistContent, "#EXT-X-VERSION")

		t.Logf("✅ HLS playlist served successfully (%d bytes)", len(playlistContent))
	})

	// Test resource cleanup
	t.Run("DockerResourceCleanup", func(t *testing.T) {
		require.NotEmpty(t, sessionID, "Session ID should be available")

		// Stop the session
		req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify session is removed from active list
		time.Sleep(500 * time.Millisecond) // Brief delay for cleanup

		req = httptest.NewRequest("GET", "/api/playback/sessions", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var sessionsResponse map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &sessionsResponse)
		require.NoError(t, err)

		sessions := sessionsResponse["sessions"].([]interface{})
		for _, session := range sessions {
			sessionMap := session.(map[string]interface{})
			assert.NotEqual(t, sessionID, sessionMap["id"], "Stopped session should not be in active list")
		}

		t.Logf("✅ Session cleanup verified")
	})
}

// setupDockerStyleEnvironment creates a test environment that mimics Docker setup
func setupDockerStyleEnvironment(t *testing.T) *TestData {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "viewra_docker_test_")
	require.NoError(t, err)

	// Create Docker-style directory structure
	transcodingDir := filepath.Join(tempDir, "viewra-data", "transcoding")
	err = os.MkdirAll(transcodingDir, 0755)
	require.NoError(t, err)

	// Create test video file
	videoPath := filepath.Join(tempDir, "test_video.mp4")
	err = createTestVideo(videoPath)
	if err != nil {
		t.Skipf("Skipping Docker integration test - FFmpeg not available: %v", err)
	}

	return &TestData{
		VideoPath:        videoPath,
		TempDir:          tempDir,
		TranscodingDir:   transcodingDir,
		ExpectedDuration: 10,
	}
}

// configureDockerStyleTranscoding sets up transcoding configuration for Docker environment
func configureDockerStyleTranscoding(t *testing.T, testData *TestData) {
	t.Helper()

	// Set environment variable (primary method for our mock service)
	os.Setenv("VIEWRA_TEST_TRANSCODE_DIR", testData.TranscodingDir)

	// Also set Docker-style environment variable
	os.Setenv("VIEWRA_TRANSCODING_DIR", testData.TranscodingDir)

	// CRITICAL: Force config reload to pick up new environment variables
	// This ensures the config system recognizes our test directory
	err := config.Load("")
	if err != nil {
		t.Logf("⚠️ Config reload failed: %v (continuing with direct config modification)", err)
	}

	// Verify and configure via config system
	cfg := config.Get()
	if cfg != nil {
		// Force set the transcoding directory directly
		cfg.Transcoding.DataDir = testData.TranscodingDir
		t.Logf("🐳 Config updated - Transcoding.DataDir: %s", cfg.Transcoding.DataDir)
	} else {
		t.Fatal("Config is nil - cannot configure transcoding directory")
	}

	t.Logf("🐳 Docker-style transcoding configured: %s", testData.TranscodingDir)
}

// cleanupDockerEnvironment cleans up Docker-style test environment
func cleanupDockerEnvironment(t *testing.T, testData *TestData) {
	t.Helper()

	// Clean up environment variables
	os.Unsetenv("VIEWRA_TEST_TRANSCODE_DIR")
	os.Unsetenv("VIEWRA_TRANSCODING_DIR")

	// Clean up temp directory
	if testData != nil && testData.TempDir != "" {
		os.RemoveAll(testData.TempDir)
	}

	t.Logf("🧹 Docker environment cleanup completed")
}

// TestE2EDockerVolumeStress tests transcoding under Docker volume stress conditions
func TestE2EDockerVolumeStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker volume stress test in short mode")
	}

	testData := setupDockerStyleEnvironment(t)
	defer cleanupDockerEnvironment(t, testData)

	db := setupTestDatabase(t)
	configureDockerStyleTranscoding(t, testData)
	playbackModule := setupPluginEnabledEnvironment(t, db)
	router := createTestRouter(t, playbackModule)

	// Test multiple concurrent sessions to stress the Docker volume
	const numSessions = 5
	sessionIDs := make([]string, numSessions)

	t.Run("CreateMultipleSessions", func(t *testing.T) {
		for i := 0; i < numSessions; i++ {
			transcodeRequest := map[string]interface{}{
				"input_path":  testData.VideoPath,
				"video_codec": "h264",
				"container":   "dash",
				"resolution":  map[string]int{"width": 1280, "height": 720},
				"bitrate":     3000,
				"audio_codec": "aac",
				"priority":    5,
			}

			body, _ := json.Marshal(transcodeRequest)
			req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusCreated, w.Code, "Session %d should be created", i)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			sessionIDs[i] = response["id"].(string)
			t.Logf("✅ Session %d created: %s", i, sessionIDs[i])
		}
	})

	t.Run("VerifyVolumeStructure", func(t *testing.T) {
		// Verify all session directories were created
		for i, sessionID := range sessionIDs {
			sessionDir := filepath.Join(testData.TranscodingDir, fmt.Sprintf("session_%s", sessionID))
			assert.DirExists(t, sessionDir, "Session %d directory should exist", i)

			// Verify manifest files
			manifestPath := filepath.Join(sessionDir, "manifest.mpd")
			assert.FileExists(t, manifestPath, "Session %d manifest should exist", i)
		}

		// Check total number of session directories
		sessionDirs, err := filepath.Glob(filepath.Join(testData.TranscodingDir, "session_*"))
		require.NoError(t, err)
		assert.Equal(t, numSessions, len(sessionDirs), "Should have exactly %d session directories", numSessions)

		t.Logf("✅ All %d sessions created successfully in Docker volume", numSessions)
	})

	t.Run("CleanupAllSessions", func(t *testing.T) {
		// Stop all sessions
		for i, sessionID := range sessionIDs {
			req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Session %d should be stopped successfully", i)
		}

		// Brief delay for cleanup
		time.Sleep(1 * time.Second)

		// Verify all sessions are cleaned up
		req := httptest.NewRequest("GET", "/api/playback/sessions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Safely handle sessions field (it might be nil or empty after cleanup)
		sessionsData, exists := response["sessions"]
		if !exists || sessionsData == nil {
			t.Logf("✅ All sessions cleaned up - sessions field is nil/missing")
		} else if sessionsArray, ok := sessionsData.([]interface{}); ok {
			assert.Equal(t, 0, len(sessionsArray), "All sessions should be cleaned up")
		} else {
			t.Errorf("Unexpected sessions field type: %T", sessionsData)
		}

		t.Logf("✅ All %d sessions cleaned up successfully", numSessions)
	})
}
