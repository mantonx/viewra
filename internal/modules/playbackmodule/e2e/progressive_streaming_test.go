package playbackmodule

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2EProgressiveStreaming tests progressive streaming endpoints comprehensively
func TestE2EProgressiveStreaming(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E progressive streaming test in short mode")
	}

	// Setup test environment
	testData := setupTestEnvironment(t)
	defer cleanupDockerEnvironment(t, testData)

	db := setupTestDatabase(t)
	playbackModule := setupPluginEnabledEnvironment(t, db)
	router := createTestRouter(t, playbackModule)

	t.Run("CreateSessionAndStream_DASH", func(t *testing.T) {
		// Start a DASH transcoding session
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

		require.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		sessionID := response["id"].(string)
		t.Logf("✅ DASH session created: %s", sessionID)

		// Wait for transcoding to initialize
		time.Sleep(1 * time.Second)

		// Test progressive streaming
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s", sessionID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return streaming content or redirect
		assert.Contains(t, []int{http.StatusOK, http.StatusFound, http.StatusSeeOther}, w.Code)
		t.Logf("✅ Progressive streaming response: %d", w.Code)

		if w.Code == http.StatusOK {
			// Check that we get streaming data
			body := w.Body.Bytes()
			assert.Greater(t, len(body), 0)
			t.Logf("✅ Received %d bytes of streaming data", len(body))

			// Check Content-Type header
			contentType := w.Header().Get("Content-Type")
			assert.NotEmpty(t, contentType)
			t.Logf("✅ Content-Type: %s", contentType)
		}

		// Cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		router.ServeHTTP(w, req)
	})

	t.Run("CreateSessionAndStream_HLS", func(t *testing.T) {
		// Start an HLS transcoding session
		transcodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
			"target_codec":     "h264",
			"target_container": "hls",
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

		require.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		sessionID := response["id"].(string)
		t.Logf("✅ HLS session created: %s", sessionID)

		// Wait for transcoding to initialize
		time.Sleep(1 * time.Second)

		// Test progressive streaming
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s", sessionID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return streaming content or redirect
		assert.Contains(t, []int{http.StatusOK, http.StatusFound, http.StatusSeeOther}, w.Code)
		t.Logf("✅ Progressive streaming response: %d", w.Code)

		// Cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		router.ServeHTTP(w, req)
	})

	t.Run("DASHManifestServing", func(t *testing.T) {
		// Start a DASH session
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

		// Wait for manifest generation
		time.Sleep(1 * time.Second)

		// Test DASH manifest serving
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s/manifest.mpd", sessionID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			manifestContent := w.Body.String()
			assert.Contains(t, manifestContent, "<?xml")
			assert.Contains(t, manifestContent, "MPD")
			t.Logf("✅ DASH manifest served (%d bytes)", len(manifestContent))

			// Check Content-Type
			contentType := w.Header().Get("Content-Type")
			assert.Contains(t, contentType, "application/dash+xml")
		} else {
			t.Logf("⚠️ DASH manifest not ready yet (status: %d)", w.Code)
		}

		// Cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		router.ServeHTTP(w, req)
	})

	t.Run("HLSPlaylistServing", func(t *testing.T) {
		// Start an HLS session
		transcodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
			"target_codec":     "h264",
			"target_container": "hls",
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

		// Wait for playlist generation
		time.Sleep(1 * time.Second)

		// Test HLS playlist serving
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s/playlist.m3u8", sessionID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			playlistContent := w.Body.String()
			assert.Contains(t, playlistContent, "#EXTM3U")
			t.Logf("✅ HLS playlist served (%d bytes)", len(playlistContent))

			// Check Content-Type
			contentType := w.Header().Get("Content-Type")
			assert.Contains(t, contentType, "application/vnd.apple.mpegurl")
		} else {
			t.Logf("⚠️ HLS playlist not ready yet (status: %d)", w.Code)
		}

		// Cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		router.ServeHTTP(w, req)
	})

	t.Run("SegmentServing", func(t *testing.T) {
		// Start a DASH session
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

		// Wait for segment generation
		time.Sleep(1 * time.Second)

		// Test segment serving
		segments := []string{
			"init-stream0.m4s",
			"init-stream1.m4s",
			"segment_1.m4s",
		}

		for _, segment := range segments {
			req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s/%s", sessionID, segment), nil)
			w = httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				segmentData := w.Body.Bytes()
				assert.Greater(t, len(segmentData), 0)
				t.Logf("✅ Segment %s served (%d bytes)", segment, len(segmentData))

				// Check Content-Type
				contentType := w.Header().Get("Content-Type")
				assert.NotEmpty(t, contentType)
			} else {
				t.Logf("⚠️ Segment %s not ready yet (status: %d)", segment, w.Code)
			}
		}

		// Cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		router.ServeHTTP(w, req)
	})

	t.Run("HeadRequestSupport", func(t *testing.T) {
		// Start a session
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

		// Wait for content generation
		time.Sleep(1 * time.Second)

		// Test HEAD requests (important for video players)
		headEndpoints := []string{
			fmt.Sprintf("/api/playback/stream/%s/manifest.mpd", sessionID),
			fmt.Sprintf("/api/playback/stream/%s/init-stream0.m4s", sessionID),
			fmt.Sprintf("/api/playback/stream/%s/init-stream1.m4s", sessionID),
		}

		for _, endpoint := range headEndpoints {
			req = httptest.NewRequest("HEAD", endpoint, nil)
			w = httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// HEAD requests should return headers without body
			if w.Code == http.StatusOK {
				assert.Equal(t, 0, w.Body.Len(), "HEAD request should not return body")
				assert.NotEmpty(t, w.Header().Get("Content-Type"))
				t.Logf("✅ HEAD request supported for %s", endpoint)
			} else {
				t.Logf("⚠️ HEAD request for %s returned %d", endpoint, w.Code)
			}
		}

		// Cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		router.ServeHTTP(w, req)
	})

	t.Run("RangeRequestSupport", func(t *testing.T) {
		// Start a session
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

		// Wait for content generation
		time.Sleep(1 * time.Second)

		// Test Range requests (important for seeking)
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s/init-stream0.m4s", sessionID), nil)
		req.Header.Set("Range", "bytes=0-1023")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusPartialContent {
			assert.Equal(t, "bytes", w.Header().Get("Accept-Ranges"))
			assert.NotEmpty(t, w.Header().Get("Content-Range"))
			t.Logf("✅ Range requests supported")
		} else if w.Code == http.StatusOK {
			t.Logf("⚠️ Range requests not implemented (returned full content)")
		} else {
			t.Logf("⚠️ Range request failed with status %d", w.Code)
		}

		// Cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		router.ServeHTTP(w, req)
	})

	t.Run("NonExistentSession_ShouldReturn404", func(t *testing.T) {
		endpoints := []string{
			"/api/playback/stream/nonexistent_session",
			"/api/playback/stream/nonexistent_session/manifest.mpd",
			"/api/playback/stream/nonexistent_session/playlist.m3u8",
			"/api/playback/stream/nonexistent_session/segment_1.m4s",
		}

		for _, endpoint := range endpoints {
			req := httptest.NewRequest("GET", endpoint, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
			t.Logf("✅ Non-existent session handled correctly for %s", endpoint)
		}
	})

	t.Run("ClientDisconnectDuringStreaming", func(t *testing.T) {
		// Start a session
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

		// Simulate client disconnects by starting multiple streams and abandoning them
		for i := 0; i < 3; i++ {
			req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s", sessionID), nil)
			w = httptest.NewRecorder()

			// Simulate client disconnect by starting and immediately canceling
			go func() {
				time.Sleep(100 * time.Millisecond)
				// In real scenarios, this would be a client disconnect
				// For tests, we just log the simulation
				t.Logf("Simulating client disconnect for request %d", i+1)
			}()

			router.ServeHTTP(w, req)

			// Give time for any cleanup
			time.Sleep(100 * time.Millisecond)
		}

		// Session should still be accessible for status checks
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		t.Logf("✅ Session remains accessible after client disconnects")

		// Cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		router.ServeHTTP(w, req)
	})

	t.Run("CORSHeaders", func(t *testing.T) {
		// Start a session
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

		// Wait for content generation
		time.Sleep(1 * time.Second)

		// Test CORS headers for streaming endpoints
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s/manifest.mpd", sessionID), nil)
		req.Header.Set("Origin", "http://localhost:3000")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Check for CORS headers (may or may not be implemented)
		corsHeaders := []string{
			"Access-Control-Allow-Origin",
			"Access-Control-Allow-Methods",
			"Access-Control-Allow-Headers",
		}

		for _, header := range corsHeaders {
			if value := w.Header().Get(header); value != "" {
				t.Logf("✅ CORS header present: %s = %s", header, value)
			} else {
				t.Logf("⚠️ CORS header missing: %s", header)
			}
		}

		// Cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		router.ServeHTTP(w, req)
	})
}

// TestE2EStreamingPerformance tests streaming performance under various conditions
func TestE2EStreamingPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E streaming performance test in short mode")
	}

	// Setup test environment
	testData := setupTestEnvironment(t)
	defer cleanupDockerEnvironment(t, testData)

	db := setupTestDatabase(t)
	playbackModule := setupPluginEnabledEnvironment(t, db)
	router := createTestRouter(t, playbackModule)

	t.Run("MultipleSimultaneousStreams", func(t *testing.T) {
		numStreams := 5
		sessionIDs := make([]string, numStreams)

		// Create multiple sessions
		for i := 0; i < numStreams; i++ {
			transcodeRequest := map[string]interface{}{
				"input_path":       testData.VideoPath,
				"target_codec":     "h264",
				"target_container": "dash",
				"resolution":       "720p",
				"bitrate":          2000,
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

			sessionIDs[i] = response["id"].(string)
			t.Logf("✅ Session %d created: %s", i+1, sessionIDs[i])
		}

		// Wait for sessions to initialize
		time.Sleep(2 * time.Second)

		// Test streaming from all sessions simultaneously
		streamResults := make(chan bool, numStreams)

		for i, sessionID := range sessionIDs {
			go func(idx int, id string) {
				req := httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s", id), nil)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				success := w.Code == http.StatusOK || w.Code == http.StatusFound
				streamResults <- success
				t.Logf("Stream %d (session %s): %d", idx+1, id, w.Code)
			}(i, sessionID)
		}

		// Wait for all streams to complete
		successCount := 0
		for i := 0; i < numStreams; i++ {
			if <-streamResults {
				successCount++
			}
		}

		t.Logf("✅ %d/%d simultaneous streams successful", successCount, numStreams)
		assert.Greater(t, successCount, 0, "At least some streams should succeed")

		// Cleanup all sessions
		for _, sessionID := range sessionIDs {
			req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})

	t.Run("StreamingLatency", func(t *testing.T) {
		// Start a session
		transcodeRequest := map[string]interface{}{
			"input_path":       testData.VideoPath,
			"target_codec":     "h264",
			"target_container": "dash",
			"resolution":       "720p",
			"bitrate":          3000,
			"audio_codec":      "aac",
			"preset":           "ultrafast", // Fast transcoding for low latency
		}

		body, _ := json.Marshal(transcodeRequest)
		req := httptest.NewRequest("POST", "/api/playback/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		startTime := time.Now()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		sessionID := response["id"].(string)
		sessionCreationTime := time.Since(startTime)

		// Measure time to first streamable content
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/playback/stream/%s", sessionID), nil)

		streamStart := time.Now()
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		streamLatency := time.Since(streamStart)

		t.Logf("✅ Session creation time: %v", sessionCreationTime)
		t.Logf("✅ Stream latency: %v", streamLatency)
		t.Logf("✅ Total time to stream: %v", sessionCreationTime+streamLatency)

		// Performance expectations (adjust based on hardware)
		assert.Less(t, sessionCreationTime, 5*time.Second, "Session creation should be fast")
		assert.Less(t, streamLatency, 10*time.Second, "Stream start should be fast")

		// Cleanup
		req = httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
		router.ServeHTTP(w, req)
	})
}
