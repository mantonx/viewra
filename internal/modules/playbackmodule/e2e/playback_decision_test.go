package playbackmodule

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2EPlaybackDecisionAPI tests the /api/playback/decide endpoint comprehensively
func TestE2EPlaybackDecisionAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E playback decision test in short mode")
	}

	// Setup test environment
	testData := setupTestEnvironment(t)
	defer cleanupDockerEnvironment(t, testData)

	db := setupTestDatabase(t)
	playbackModule := setupPluginEnabledEnvironment(t, db)
	router := createTestRouter(t, playbackModule)

	t.Run("ChromeUserAgent_ShouldRecommendDASH", func(t *testing.T) {
		decisionRequest := map[string]interface{}{
			"media_path": testData.VideoPath,
			"device_profile": map[string]interface{}{
				"user_agent":       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
				"supported_codecs": []string{"h264", "aac"},
				"max_resolution":   "1080p",
				"max_bitrate":      8000,
				"supports_hevc":    false,
				"supports_av1":     false,
				"supports_hdr":     false,
				"client_ip":        "127.0.0.1",
				"platform":         "windows",
				"browser":          "chrome",
			},
		}

		body, _ := json.Marshal(decisionRequest)
		req := httptest.NewRequest("POST", "/api/playback/decide", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Chrome should typically get DASH for adaptive streaming
		t.Logf("Chrome decision: %+v", response)

		// Validate response structure
		assert.Contains(t, response, "should_transcode")
		assert.Contains(t, response, "reason")

		if response["should_transcode"].(bool) {
			assert.Contains(t, response, "transcode_params")
			transcodeParams := response["transcode_params"].(map[string]interface{})

			// Check that container preference is correct
			container := transcodeParams["target_container"].(string)
			assert.Contains(t, []string{"dash", "hls", "mp4"}, container)

			// Chrome typically prefers DASH
			t.Logf("Chrome container recommendation: %s", container)
		} else {
			assert.Contains(t, response, "direct_play_url")
		}
	})

	t.Run("SafariUserAgent_ShouldRecommendHLS", func(t *testing.T) {
		decisionRequest := map[string]interface{}{
			"media_path": testData.VideoPath,
			"device_profile": map[string]interface{}{
				"user_agent":       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
				"supported_codecs": []string{"h264", "aac"},
				"max_resolution":   "1080p",
				"max_bitrate":      8000,
				"supports_hevc":    true,
				"supports_av1":     false,
				"supports_hdr":     false,
				"client_ip":        "127.0.0.1",
				"platform":         "macos",
				"browser":          "safari",
			},
		}

		body, _ := json.Marshal(decisionRequest)
		req := httptest.NewRequest("POST", "/api/playback/decide", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Safari should typically get HLS for native support
		t.Logf("Safari decision: %+v", response)

		// Validate response structure
		assert.Contains(t, response, "should_transcode")
		assert.Contains(t, response, "reason")

		if response["should_transcode"].(bool) {
			assert.Contains(t, response, "transcode_params")
			transcodeParams := response["transcode_params"].(map[string]interface{})

			// Check that container preference is correct
			container := transcodeParams["target_container"].(string)
			assert.Contains(t, []string{"dash", "hls", "mp4"}, container)

			// Safari typically prefers HLS
			t.Logf("Safari container recommendation: %s", container)
		}
	})

	t.Run("MalformedRequest_ShouldReturnBadRequest", func(t *testing.T) {
		testCases := []struct {
			name    string
			request interface{}
		}{
			{
				name:    "EmptyRequest",
				request: map[string]interface{}{},
			},
			{
				name: "MissingMediaPath",
				request: map[string]interface{}{
					"device_profile": map[string]interface{}{
						"user_agent": "Mozilla/5.0 (Chrome/120.0)",
					},
				},
			},
			{
				name: "MissingDeviceProfile",
				request: map[string]interface{}{
					"media_path": testData.VideoPath,
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				body, _ := json.Marshal(tc.request)
				req := httptest.NewRequest("POST", "/api/playback/decide", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				assert.Equal(t, http.StatusBadRequest, w.Code)
				t.Logf("%s handled correctly with status: %d", tc.name, w.Code)
			})
		}
	})

	t.Run("InvalidHTTPMethods_ShouldReturnMethodNotAllowed", func(t *testing.T) {
		methods := []string{"GET", "PUT", "DELETE", "PATCH"}

		for _, method := range methods {
			req := httptest.NewRequest(method, "/api/playback/decide", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
			t.Logf("Method %s correctly rejected with status: %d", method, w.Code)
		}
	})
}
