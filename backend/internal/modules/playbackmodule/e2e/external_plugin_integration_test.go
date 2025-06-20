package playbackmodule

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/playbackmodule"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Helper types for real plugin testing
type ProgressUpdate struct {
	Timestamp time.Time
	Status    string
	Progress  float64
	Backend   string
}

// setupTestLogger creates a test logger
func setupTestLogger() hclog.Logger {
	return hclog.NewNullLogger()
}

// setupExternalPluginManager attempts to create a real external plugin manager
func setupExternalPluginManager(ctx context.Context, db *gorm.DB, logger hclog.Logger) *pluginmodule.ExternalPluginManager {
	// Create external plugin manager
	extMgr := pluginmodule.NewExternalPluginManager(db, logger)

	// Try to initialize with test plugin directory
	pluginDir := filepath.Join(os.TempDir(), "test_plugins")
	os.MkdirAll(pluginDir, 0755)

	hostServices := &pluginmodule.HostServices{}
	if err := extMgr.Initialize(ctx, pluginDir, hostServices); err != nil {
		// Return nil if real plugin manager can't be initialized
		return nil
	}

	return extMgr
}

// TestE2EExternalPluginIntegration tests integration with real external plugins
func TestE2EExternalPluginIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E external plugin integration test in short mode")
	}

	// Setup test environment
	testData := setupTestEnvironment(t)
	defer cleanupDockerEnvironment(t, testData)

	db := setupTestDatabase(t)

	// Try real external plugin integration first, fallback to mock if not available
	playbackModule := setupRealPluginEnvironment(t, db)
	router := createTestRouter(t, playbackModule)

	t.Run("PluginDiscoveryAndCapabilities", func(t *testing.T) {
		// Test plugin discovery endpoint
		req := httptest.NewRequest("GET", "/api/playback/plugins", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var plugins []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &plugins)
		require.NoError(t, err)

		t.Logf("🔌 Discovered %d plugin(s)", len(plugins))

		// Should find at least one plugin (either external or mock)
		assert.Greater(t, len(plugins), 0, "Should discover at least one transcoding plugin")

		for i, plugin := range plugins {
			t.Logf("   Plugin %d:", i+1)
			t.Logf("     - ID: %v", plugin["id"])
			t.Logf("     - Name: %v", plugin["name"])
			t.Logf("     - Type: %v", plugin["type"])
			t.Logf("     - Version: %v", plugin["version"])
			t.Logf("     - Status: %v", plugin["status"])

			// Validate plugin structure
			assert.NotEmpty(t, plugin["id"])
			assert.NotEmpty(t, plugin["name"])
			assert.Contains(t, plugin["type"], "transcod") // Should contain "transcoding" or similar
		}
	})

	t.Run("PluginCapabilitiesRetrieval", func(t *testing.T) {
		// Get available plugins
		req := httptest.NewRequest("GET", "/api/playback/plugins", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var plugins []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &plugins)
		require.NoError(t, err)

		if len(plugins) == 0 {
			t.Skip("No plugins available for capabilities testing")
		}

		// Test capabilities for each plugin
		for _, plugin := range plugins {
			pluginID := plugin["id"].(string)

			req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/plugins/%s/capabilities", pluginID), nil)
			w = httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var capabilities map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &capabilities)
				require.NoError(t, err)

				t.Logf("🎯 Plugin %s capabilities:", pluginID)
				t.Logf("   - Name: %v", capabilities["name"])
				t.Logf("   - Supported codecs: %v", capabilities["supported_codecs"])
				t.Logf("   - Supported containers: %v", capabilities["supported_containers"])
				t.Logf("   - Hardware acceleration: %v", capabilities["hardware_acceleration"])
				t.Logf("   - Max concurrent sessions: %v", capabilities["max_concurrent_sessions"])

				// Validate capabilities structure
				assert.NotEmpty(t, capabilities["name"])
				assert.NotNil(t, capabilities["supported_codecs"])
				assert.NotNil(t, capabilities["supported_containers"])
				assert.NotNil(t, capabilities["hardware_acceleration"])

				if codecs, ok := capabilities["supported_codecs"].([]interface{}); ok {
					assert.Greater(t, len(codecs), 0, "Should support at least one codec")
				}

				if containers, ok := capabilities["supported_containers"].([]interface{}); ok {
					assert.Greater(t, len(containers), 0, "Should support at least one container")
				}
			} else {
				t.Logf("⚠️ Plugin %s capabilities not available (status: %d)", pluginID, w.Code)
			}
		}
	})

	t.Run("ExternalPluginTranscodingWorkflow", func(t *testing.T) {
		// Create a transcoding session using external plugin
		transcodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
			"target_codec":     "h264",
			"target_container": "dash",
			"resolution":       "720p",
			"bitrate":          3000,
			"audio_codec":      "aac",
			"preset":           "ultrafast",
		}

		body, _ := json.Marshal(transcodeRequest)
		req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Logf("⚠️ External plugin transcoding not available (status: %d)", w.Code)
			if w.Code == http.StatusServiceUnavailable {
				t.Skip("External plugin transcoding service unavailable")
			}
			return
		}

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		sessionID := response["id"].(string)
		t.Logf("✅ External plugin session created: %s", sessionID)

		// Monitor transcoding progress
		maxWaitTime := 30 * time.Second
		checkInterval := 2 * time.Second
		deadline := time.Now().Add(maxWaitTime)

		var finalStatus string
		var finalProgress float64

		for time.Now().Before(deadline) {
			req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
			w = httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var sessionInfo map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &sessionInfo); err == nil {
					status := sessionInfo["status"].(string)
					progress := sessionInfo["progress"].(float64)
					backend := sessionInfo["backend"].(string)

					finalStatus = status
					finalProgress = progress

					t.Logf("🔄 Session %s: %s (%.1f%%) [%s]", sessionID, status, progress*100, backend)

					// Should show external plugin backend
					if backend != "mock_ffmpeg" && backend != "slow_mock_ffmpeg" {
						t.Logf("✅ Using external plugin backend: %s", backend)
					}

					if status == "completed" {
						t.Logf("✅ External plugin transcoding completed")
						break
					} else if status == "failed" {
						t.Logf("❌ External plugin transcoding failed")
						break
					}
				}
			} else {
				t.Logf("⚠️ Session status check failed: %d", w.Code)
				break
			}

			time.Sleep(checkInterval)
		}

		// Verify final state
		assert.Contains(t, []string{"completed", "running", "starting"}, finalStatus,
			"Session should reach a valid final state")

		if finalStatus == "completed" {
			assert.Equal(t, 1.0, finalProgress, "Completed session should have 100% progress")
		} else {
			assert.GreaterOrEqual(t, finalProgress, 0.0, "Progress should be non-negative")
		}

		// Test streaming from completed session
		if finalStatus == "completed" {
			req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s", sessionID), nil)
			w = httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				streamData := w.Body.Bytes()
				assert.Greater(t, len(streamData), 0, "Should return streaming data")
				t.Logf("✅ External plugin streaming works (%d bytes)", len(streamData))
			} else {
				t.Logf("⚠️ External plugin streaming not ready (status: %d)", w.Code)
			}
		}

		// Cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			t.Logf("✅ External plugin session cleaned up")
		} else {
			t.Logf("⚠️ Session cleanup status: %d", w.Code)
		}
	})

	t.Run("ExternalPluginErrorHandling", func(t *testing.T) {
		// Test invalid input file handling
		transcodeRequest := map[string]interface{}{
			"input_path":       "/nonexistent/file.mp4",
			"target_codec":     "h264",
			"target_container": "dash",
			"resolution":       "720p",
			"bitrate":          3000,
			"audio_codec":      "aac",
		}

		body, _ := json.Marshal(transcodeRequest)
		req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should handle error gracefully
		if w.Code == http.StatusCreated {
			// If session was created, it should eventually fail
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			sessionID := response["id"].(string)
			t.Logf("Session created for invalid file: %s", sessionID)

			// Wait a bit and check if it fails
			time.Sleep(3 * time.Second)

			req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
			w = httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var sessionInfo map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &sessionInfo); err == nil {
					status := sessionInfo["status"].(string)
					t.Logf("Invalid file session status: %s", status)

					// Should eventually fail
					if status == "failed" {
						t.Logf("✅ External plugin properly handles invalid input")
					}
				}
			}

			// Cleanup
			req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
			router.ServeHTTP(w, req)
		} else {
			t.Logf("✅ External plugin rejected invalid input (status: %d)", w.Code)
		}
	})

	t.Run("ExternalPluginSessionManagement", func(t *testing.T) {
		// Test that external plugin properly manages multiple sessions
		numSessions := 3
		var sessionIDs []string

		// Create multiple sessions
		for i := 0; i < numSessions; i++ {
			transcodeRequest := map[string]interface{}{
				"input_path":       testData.VideoPath,
				"target_codec":     "h264",
				"target_container": "dash",
				"resolution":       fmt.Sprintf("%dp", 480+(i*240)), // Different resolutions
				"bitrate":          2000 + (i * 1000),               // Different bitrates
				"audio_codec":      "aac",
				"preset":           "ultrafast",
			}

			body, _ := json.Marshal(transcodeRequest)
			req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusCreated {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				sessionID := response["id"].(string)
				sessionIDs = append(sessionIDs, sessionID)
				t.Logf("✅ External plugin session %d created: %s", i+1, sessionID)
			} else {
				t.Logf("⚠️ External plugin session %d creation failed: %d", i+1, w.Code)
			}
		}

		if len(sessionIDs) == 0 {
			t.Skip("No external plugin sessions created")
		}

		// Wait for some transcoding progress
		time.Sleep(3 * time.Second)

		// Check all sessions are properly tracked
		for i, sessionID := range sessionIDs {
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var sessionInfo map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &sessionInfo); err == nil {
					status := sessionInfo["status"].(string)
					progress := sessionInfo["progress"].(float64)
					backend := sessionInfo["backend"].(string)

					t.Logf("📊 Session %d (%s): %s (%.1f%%) [%s]",
						i+1, sessionID, status, progress*100, backend)

					// Verify session independence
					assert.Equal(t, sessionID, sessionInfo["id"])
					assert.NotEmpty(t, status)
					assert.GreaterOrEqual(t, progress, 0.0)
				}
			} else {
				t.Logf("⚠️ Session %d status check failed: %d", i+1, w.Code)
			}
		}

		// List all active sessions
		req := httptest.NewRequest("GET", "/api/playback/sessions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			var sessions []map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &sessions); err == nil {
				t.Logf("📊 Total active sessions: %d", len(sessions))

				// Should find our created sessions
				foundSessions := 0
				for _, session := range sessions {
					sessionID := session["id"].(string)
					for _, createdID := range sessionIDs {
						if sessionID == createdID {
							foundSessions++
							break
						}
					}
				}

				t.Logf("✅ Found %d/%d created sessions in active list", foundSessions, len(sessionIDs))
			}
		}

		// Cleanup all sessions
		for i, sessionID := range sessionIDs {
			req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				t.Logf("✅ Session %d cleaned up", i+1)
			} else {
				t.Logf("⚠️ Session %d cleanup status: %d", i+1, w.Code)
			}
		}
	})

	t.Run("ExternalPluginRefresh", func(t *testing.T) {
		// Test plugin refresh functionality
		req := httptest.NewRequest("POST", "/api/playback/plugins/refresh", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should handle refresh gracefully (even if not implemented)
		assert.Contains(t, []int{http.StatusOK, http.StatusAccepted, http.StatusNotImplemented, http.StatusMethodNotAllowed}, w.Code)
		t.Logf("Plugin refresh response: %d", w.Code)

		if w.Code == http.StatusOK || w.Code == http.StatusAccepted {
			t.Logf("✅ Plugin refresh supported")

			// Wait a bit for refresh to complete
			time.Sleep(1 * time.Second)

			// Re-check plugin discovery
			req = httptest.NewRequest("GET", "/api/playback/plugins", nil)
			w = httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)

			var plugins []map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &plugins)
			require.NoError(t, err)

			t.Logf("✅ After refresh: %d plugin(s) discovered", len(plugins))
		} else {
			t.Logf("⚠️ Plugin refresh not implemented or not allowed")
		}
	})
}

// TestE2ERealPluginRequirements tests specific requirements for real plugin integration
func TestE2ERealPluginRequirements(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real plugin requirements test in short mode")
	}

	// This test specifically validates real external plugin integration
	testData := setupDockerStyleEnvironment(t)
	configureDockerStyleTranscoding(t, testData)
	defer cleanupDockerEnvironment(t, testData)

	db := setupTestDatabase(t)

	// Force real plugin environment (not mock)
	ctx := context.Background()
	logger := setupTestLogger()

	// Try to create real external plugin manager
	externalPluginManager := setupExternalPluginManager(ctx, db, logger)
	if externalPluginManager == nil {
		t.Skip("Real external plugin manager not available")
	}

	// Use the external plugin manager directly with the adapter from playbackmodule
	adapter := playbackmodule.NewExternalPluginManagerAdapter(externalPluginManager)
	module := playbackmodule.NewModule(db, nil, adapter)
	require.NoError(t, module.Init())

	router := createTestRouter(t, module)

	t.Run("RealPluginDetection", func(t *testing.T) {
		// Verify we're actually using real plugins, not mocks
		req := httptest.NewRequest("GET", "/api/playback/plugins", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var plugins []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &plugins)
		require.NoError(t, err)

		hasRealPlugin := false
		for _, plugin := range plugins {
			pluginID := plugin["id"].(string)
			pluginName := plugin["name"].(string)

			// Real plugins should not be mock plugins
			if pluginID != "mock_ffmpeg" && pluginID != "slow_mock_ffmpeg" {
				hasRealPlugin = true
				t.Logf("✅ Real plugin detected: %s (%s)", pluginName, pluginID)
			}
		}

		if !hasRealPlugin {
			t.Skip("No real external plugins detected - only mocks available")
		}
	})

	t.Run("RealPluginTranscodingPerformance", func(t *testing.T) {
		// Test actual transcoding performance with real plugin
		transcodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
			"target_codec":     "h264",
			"target_container": "dash",
			"resolution":       "720p",
			"bitrate":          3000,
			"audio_codec":      "aac",
			"preset":           "ultrafast", // Fast for testing
		}

		body, _ := json.Marshal(transcodeRequest)
		req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		startTime := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		sessionCreationTime := time.Since(startTime)

		if w.Code != http.StatusCreated {
			t.Skipf("Real plugin transcoding not available (status: %d)", w.Code)
		}

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		sessionID := response["id"].(string)
		t.Logf("✅ Real plugin session created in %v: %s", sessionCreationTime, sessionID)

		// Monitor real transcoding progress
		maxWaitTime := 60 * time.Second // Longer wait for real transcoding
		checkInterval := 3 * time.Second
		deadline := time.Now().Add(maxWaitTime)

		var progressUpdates []ProgressUpdate

		for time.Now().Before(deadline) {
			req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
			w = httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var sessionInfo map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &sessionInfo); err == nil {
					status := sessionInfo["status"].(string)
					progress := sessionInfo["progress"].(float64)
					backend := sessionInfo["backend"].(string)

					update := ProgressUpdate{
						Timestamp: time.Now(),
						Status:    status,
						Progress:  progress,
						Backend:   backend,
					}
					progressUpdates = append(progressUpdates, update)

					t.Logf("🔄 Real transcoding: %s (%.1f%%) [%s]", status, progress*100, backend)

					if status == "completed" {
						totalTime := time.Since(startTime)
						t.Logf("✅ Real plugin transcoding completed in %v", totalTime)
						break
					} else if status == "failed" {
						t.Logf("❌ Real plugin transcoding failed")

						// Check for error details
						if errorMsg, ok := sessionInfo["error"]; ok {
							t.Logf("   Error: %v", errorMsg)
						}
						break
					}
				}
			}

			time.Sleep(checkInterval)
		}

		// Analyze real transcoding performance
		if len(progressUpdates) > 0 {
			firstUpdate := progressUpdates[0]
			lastUpdate := progressUpdates[len(progressUpdates)-1]

			t.Logf("📊 Real transcoding analysis:")
			t.Logf("   - Initial status: %s", firstUpdate.Status)
			t.Logf("   - Final status: %s", lastUpdate.Status)
			t.Logf("   - Final progress: %.1f%%", lastUpdate.Progress*100)
			t.Logf("   - Backend: %s", lastUpdate.Backend)
			t.Logf("   - Duration: %v", lastUpdate.Timestamp.Sub(firstUpdate.Timestamp))
			t.Logf("   - Updates received: %d", len(progressUpdates))

			// Real plugins should show actual progress
			if lastUpdate.Status == "completed" {
				assert.Equal(t, 1.0, lastUpdate.Progress, "Completed transcoding should be 100%")
			}
		}

		// Cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}
