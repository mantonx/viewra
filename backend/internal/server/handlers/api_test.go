package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/apiroutes"
	"github.com/stretchr/testify/assert"
)

// TestAPIRootHandler checks the /api endpoint response.
func TestAPIRootHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiroutes.ClearForTesting() // Clear routes before test

	// Register routes that ApiRootHandler specifically looks for to create the endpointsMap summary
	apiroutes.Register("/api", "GET", "API root discovery.")
	apiroutes.Register("/api/v1/users", "GET", "Manages user accounts and authentication.")
	apiroutes.Register("/api/v1/media", "GET", "Manages media items, libraries, and metadata.")
	apiroutes.Register("/api/v1/plugins", "GET", "Manages plugins, their configurations, and status.")
	apiroutes.Register("/swagger/index.html", "GET", "Serves API documentation (Swagger UI).")
	// Add one more route to ensure it appears in registered_routes but not endpointsMap if not special
	apiroutes.Register("/api/v1/extra", "GET", "An extra test route.")

	// Simulate a plugin registering a route
	pluginID := "test-plugin"
	pluginRoutePath := "/my-data"
	fullPluginPath := "/api/plugins/" + pluginID + pluginRoutePath
	apiroutes.Register(fullPluginPath, "GET", "Test plugin route.")

	r := gin.New()
	r.GET("/api", ApiRootHandler)

	req, _ := http.NewRequest(http.MethodGet, "/api", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var responseBody map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &responseBody)
	assert.NoError(t, err, "Failed to unmarshal response: %s", w.Body.String())

	assert.Equal(t, "v1", responseBody["version"], "Version should be v1")
	assert.Equal(t, "OK", responseBody["status"], "Status should be OK")

	endpoints, ok := responseBody["endpoints"].(map[string]interface{})
	assert.True(t, ok, "Endpoints map should exist")
	assert.Equal(t, "/api/v1/users", endpoints["users"])
	assert.Equal(t, "/api/v1/media", endpoints["media"])
	assert.Equal(t, "/api/v1/plugins", endpoints["plugins"])
	assert.Equal(t, "/swagger/index.html", endpoints["docs"])
	assert.Equal(t, "/api", endpoints["self"])
	_, extraExists := endpoints["extra"] // Should not exist in summary map
	assert.False(t, extraExists, "Extra route should not be in the summarized endpoints map")

	registered, ok := responseBody["registered_routes"].([]interface{})
	assert.True(t, ok, "Registered routes list should exist")
	assert.Len(t, registered, 6, "Should have all 6 registered routes in the detailed list")

	foundExtra := false
	for _, item := range registered {
		route, _ := item.(map[string]interface{})
		if route["path"] == "/api/v1/extra" {
			assert.Equal(t, "GET", route["method"])
			assert.Equal(t, "An extra test route.", route["description"])
			foundExtra = true
			break
		}
	}
	assert.True(t, foundExtra, "Detailed extra route not found in registered_routes")
}

// It would be better to have a helper in apiroutes to clear for testing.
// Add to backend/internal/apiroutes/registry.go:
/*
func ClearForTesting() {
	registryMu.Lock()
	defer registryMu.Unlock()
	routeRegistry = make([]APIRoute, 0)
}
*/ 