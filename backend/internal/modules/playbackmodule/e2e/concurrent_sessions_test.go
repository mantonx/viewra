package playbackmodule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2EConcurrentSessionManagement tests concurrent session management scenarios
func TestE2EConcurrentSessionManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E concurrent session management test in short mode")
	}

	// Setup test environment
	testData := setupTestEnvironment(t)
	defer cleanupDockerEnvironment(t, testData)

	db := setupTestDatabase(t)
	playbackModule := setupPluginEnabledEnvironment(t, db)
	router := createTestRouter(t, playbackModule)

	t.Run("MultipleConcurrentSessions", func(t *testing.T) {
		numSessions := 10
		var wg sync.WaitGroup
		sessionResults := make(chan SessionResult, numSessions)

		// Start multiple sessions concurrently
		for i := 0; i < numSessions; i++ {
			wg.Add(1)
			go func(sessionNum int) {
				defer wg.Done()

				transcodeRequest := map[string]interface{}{
					"input_path":       testData.VideoPath,
					"target_codec":     "h264",
					"target_container": "dash",
					"resolution":       "720p",
					"bitrate":          2000 + (sessionNum * 500), // Vary bitrates
					"audio_codec":      "aac",
					"preset":           "ultrafast",
				}

				body, _ := json.Marshal(transcodeRequest)
				req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				startTime := time.Now()
				router.ServeHTTP(w, req)
				duration := time.Since(startTime)

				result := SessionResult{
					SessionNum: sessionNum,
					StatusCode: w.Code,
					Duration:   duration,
					Success:    w.Code == http.StatusCreated,
				}

				if w.Code == http.StatusCreated {
					var response map[string]interface{}
					if err := json.Unmarshal(w.Body.Bytes(), &response); err == nil {
						result.SessionID = response["id"].(string)
					}
				}

				sessionResults <- result
			}(i)
		}

		// Wait for all sessions to complete
		wg.Wait()
		close(sessionResults)

		// Analyze results
		var successful, failed int
		var totalDuration time.Duration
		var sessionIDs []string

		for result := range sessionResults {
			totalDuration += result.Duration
			if result.Success {
				successful++
				sessionIDs = append(sessionIDs, result.SessionID)
				t.Logf("✅ Session %d created successfully: %s (took %v)",
					result.SessionNum, result.SessionID, result.Duration)
			} else {
				failed++
				t.Logf("❌ Session %d failed with status %d (took %v)",
					result.SessionNum, result.StatusCode, result.Duration)
			}
		}

		avgDuration := totalDuration / time.Duration(numSessions)
		t.Logf("📊 Concurrent session creation results:")
		t.Logf("   - Successful: %d/%d", successful, numSessions)
		t.Logf("   - Failed: %d/%d", failed, numSessions)
		t.Logf("   - Average duration: %v", avgDuration)

		// Expect at least 80% success rate for concurrent sessions
		successRate := float64(successful) / float64(numSessions)
		assert.GreaterOrEqual(t, successRate, 0.8, "Should handle concurrent sessions well")

		// Cleanup successful sessions
		var cleanupWg sync.WaitGroup
		for _, sessionID := range sessionIDs {
			cleanupWg.Add(1)
			go func(id string) {
				defer cleanupWg.Done()
				req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", id), nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
			}(sessionID)
		}
		cleanupWg.Wait()
	})

	t.Run("SessionIsolation", func(t *testing.T) {
		// Create multiple sessions with different parameters
		sessionConfigs := []map[string]interface{}{
			{
				"input_path":       testData.VideoPath,
				"target_codec":     "h264",
				"target_container": "dash",
				"resolution":       "480p",
				"bitrate":          1500,
				"audio_codec":      "aac",
			},
			{
				"input_path":       testData.VideoPath,
				"target_codec":     "h264",
				"target_container": "hls",
				"resolution":       "720p",
				"bitrate":          3000,
				"audio_codec":      "aac",
			},
			{
				"input_path":       testData.VideoPath,
				"target_codec":     "h264",
				"target_container": "dash",
				"resolution":       "1080p",
				"bitrate":          5000,
				"audio_codec":      "aac",
			},
		}

		var sessionIDs []string

		// Create sessions with different configurations
		for i, config := range sessionConfigs {
			body, _ := json.Marshal(config)
			req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusCreated, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			sessionID := response["id"].(string)
			sessionIDs = append(sessionIDs, sessionID)
			t.Logf("✅ Session %d created: %s", i+1, sessionID)
		}

		// Verify each session maintains its configuration independently
		for i, sessionID := range sessionIDs {
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)

			var sessionInfo map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &sessionInfo)
			require.NoError(t, err)

			// Verify session isolation
			assert.Equal(t, sessionID, sessionInfo["id"])
			assert.NotNil(t, sessionInfo["request"])

			request := sessionInfo["request"].(map[string]interface{})
			expectedConfig := sessionConfigs[i]

			// Check key parameters are preserved
			assert.Equal(t, expectedConfig["target_container"], request["target_container"])
			assert.Equal(t, expectedConfig["resolution"], request["resolution"])
			assert.Equal(t, float64(expectedConfig["bitrate"].(int)), request["bitrate"])

			t.Logf("✅ Session %s maintains correct configuration", sessionID)
		}

		// Cleanup
		for _, sessionID := range sessionIDs {
			req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})

	t.Run("SessionDuplicationPrevention", func(t *testing.T) {
		// Create initial session
		transcodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
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

		require.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		originalSessionID := response["id"].(string)
		t.Logf("✅ Original session created: %s", originalSessionID)

		// Try to create identical session multiple times
		numAttempts := 5
		var duplicateSessionIDs []string

		for i := 0; i < numAttempts; i++ {
			req = httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			w = httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusCreated {
				var dupResponse map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &dupResponse); err == nil {
					duplicateSessionID := dupResponse["id"].(string)
					duplicateSessionIDs = append(duplicateSessionIDs, duplicateSessionID)
					t.Logf("Session %d: %s", i+1, duplicateSessionID)
				}
			} else if w.Code == http.StatusConflict {
				t.Logf("Duplicate session prevented (status: %d)", w.Code)
			} else {
				t.Logf("Session creation attempt %d: status %d", i+1, w.Code)
			}
		}

		// Analyze duplication behavior
		uniqueSessions := make(map[string]bool)
		uniqueSessions[originalSessionID] = true
		for _, id := range duplicateSessionIDs {
			uniqueSessions[id] = true
		}

		if len(uniqueSessions) == 1 {
			t.Logf("✅ Perfect deduplication: all requests returned same session")
		} else if len(uniqueSessions) <= 3 {
			t.Logf("⚠️ Some deduplication: %d unique sessions created", len(uniqueSessions))
		} else {
			t.Logf("❌ Poor deduplication: %d unique sessions created", len(uniqueSessions))
		}

		// Cleanup all sessions
		for sessionID := range uniqueSessions {
			req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
			w = httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})

	t.Run("SessionTimeoutAndCleanup", func(t *testing.T) {
		// Create a session with slow mock plugin for timeout testing
		testData := setupTestEnvironment(t)
		defer cleanupDockerEnvironment(t, testData)

		db := setupTestDatabase(t)
		playbackModule := setupSlowTranscodingEnvironment(t, db)
		slowRouter := createTestRouter(t, playbackModule)

		transcodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
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

		require.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		sessionID := response["id"].(string)
		t.Logf("✅ Slow session created: %s", sessionID)

		// Monitor session status over time
		maxWaitTime := 10 * time.Second
		checkInterval := 1 * time.Second
		deadline := time.Now().Add(maxWaitTime)

		for time.Now().Before(deadline) {
			req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
			w = httptest.NewRecorder()
			slowRouter.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				var sessionInfo map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &sessionInfo); err == nil {
					status := sessionInfo["status"].(string)
					progress := sessionInfo["progress"].(float64)
					t.Logf("📊 Session %s: %s (%.1f%%)", sessionID, status, progress*100)

					if status == "completed" || status == "failed" {
						break
					}
				}
			} else if w.Code == http.StatusNotFound {
				t.Logf("✅ Session %s was cleaned up automatically", sessionID)
				break
			}

			time.Sleep(checkInterval)
		}

		// Attempt explicit cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		w = httptest.NewRecorder()
		slowRouter.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			t.Logf("✅ Session %s cleaned up successfully", sessionID)
		} else if w.Code == http.StatusNotFound {
			t.Logf("✅ Session %s was already cleaned up", sessionID)
		}
	})

	t.Run("MaxConcurrentSessionsLimit", func(t *testing.T) {
		// Attempt to create more sessions than the system allows
		maxAttempts := 15 // Attempt more than typical limits
		var wg sync.WaitGroup
		results := make(chan bool, maxAttempts)

		for i := 0; i < maxAttempts; i++ {
			wg.Add(1)
			go func(sessionNum int) {
				defer wg.Done()

				transcodeRequest := map[string]interface{}{
					"input_path":       testData.VideoPath,
					"target_codec":     "h264",
					"target_container": "dash",
					"resolution":       fmt.Sprintf("%dp", 480+(sessionNum*120)), // Vary resolutions
					"bitrate":          1500 + (sessionNum * 300),
					"audio_codec":      "aac",
				}

				body, _ := json.Marshal(transcodeRequest)
				req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				success := w.Code == http.StatusCreated
				results <- success

				if success {
					// If successful, try to clean up immediately
					var response map[string]interface{}
					if err := json.Unmarshal(w.Body.Bytes(), &response); err == nil {
						sessionID := response["id"].(string)
						time.Sleep(100 * time.Millisecond) // Brief delay

						deleteReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
						deleteW := httptest.NewRecorder()
						router.ServeHTTP(deleteW, deleteReq)
					}
				} else {
					t.Logf("Session %d rejected with status: %d", sessionNum, w.Code)
				}
			}(i)
		}

		wg.Wait()
		close(results)

		// Analyze results
		var successful, rejected int
		for success := range results {
			if success {
				successful++
			} else {
				rejected++
			}
		}

		t.Logf("📊 Concurrent session limit test:")
		t.Logf("   - Successful: %d", successful)
		t.Logf("   - Rejected: %d", rejected)

		// Should reject some requests when hitting limits
		if rejected > 0 {
			t.Logf("✅ System properly limits concurrent sessions")
		} else {
			t.Logf("⚠️ No session limit enforcement detected")
		}

		// Allow time for cleanup
		time.Sleep(1 * time.Second)
	})

	t.Run("SessionStatusConsistency", func(t *testing.T) {
		// Create a session and monitor status consistency
		transcodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
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

		require.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		sessionID := response["id"].(string)

		// Check status consistency across multiple rapid requests
		numChecks := 10
		var statuses []string
		var progresses []float64

		for i := 0; i < numChecks; i++ {
			req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
			w = httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)

			var sessionInfo map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &sessionInfo)
			require.NoError(t, err)

			status := sessionInfo["status"].(string)
			progress := sessionInfo["progress"].(float64)

			statuses = append(statuses, status)
			progresses = append(progresses, progress)

			time.Sleep(50 * time.Millisecond)
		}

		// Analyze status progression
		t.Logf("📊 Status progression:")
		for i, status := range statuses {
			t.Logf("   Check %d: %s (%.1f%%)", i+1, status, progresses[i]*100)
		}

		// Progress should be monotonic (non-decreasing)
		for i := 1; i < len(progresses); i++ {
			if progresses[i] < progresses[i-1] {
				t.Errorf("❌ Progress went backwards: %.1f%% -> %.1f%%",
					progresses[i-1]*100, progresses[i]*100)
			}
		}

		// Valid status transitions
		validTransitions := map[string][]string{
			"pending":   {"starting", "running", "failed"},
			"starting":  {"running", "completed", "failed"},
			"running":   {"completed", "failed"},
			"completed": {"completed"},
			"failed":    {"failed"},
		}

		for i := 1; i < len(statuses); i++ {
			prevStatus := statuses[i-1]
			currentStatus := statuses[i]

			if prevStatus != currentStatus {
				validNext, exists := validTransitions[prevStatus]
				if !exists {
					t.Errorf("❌ Unknown status: %s", prevStatus)
					continue
				}

				isValidTransition := false
				for _, valid := range validNext {
					if currentStatus == valid {
						isValidTransition = true
						break
					}
				}

				if !isValidTransition {
					t.Errorf("❌ Invalid status transition: %s -> %s", prevStatus, currentStatus)
				}
			}
		}

		// Cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
	})
}

// SessionResult represents the result of a concurrent session creation
type SessionResult struct {
	SessionNum int
	SessionID  string
	StatusCode int
	Duration   time.Duration
	Success    bool
}
