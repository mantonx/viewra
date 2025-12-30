package proxy

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mantonx/viewra/internal/infrastructure/plugins/registry"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/types"
)

// mockPluginLookup implements PluginLookup for testing
type mockPluginLookup struct {
	plugins map[string]*types.Instance
}

func (m *mockPluginLookup) GetPlugin(id string) (*types.Instance, bool) {
	p, ok := m.plugins[id]
	return p, ok
}

func (m *mockPluginLookup) RestartPlugin(ctx context.Context, pluginID string) error {
	return nil
}

func TestNewHTTPProxy(t *testing.T) {
	lookup := &mockPluginLookup{}
	routeReg := registry.NewRouteRegistry()
	capReg := registry.NewCapabilityRegistry()
	rateLimiter := registry.NewRouteRateLimiter()
	logger := slog.Default()

	proxy := NewHTTPProxy(lookup, routeReg, capReg, rateLimiter, logger)

	if proxy == nil {
		t.Fatal("expected non-nil proxy")
	}
	if proxy.GetRouteRegistry() != routeReg {
		t.Error("route registry mismatch")
	}
	if proxy.GetCapabilityRegistry() != capReg {
		t.Error("capability registry mismatch")
	}
}

func TestHTTPProxy_UnregisterPlugin(t *testing.T) {
	lookup := &mockPluginLookup{}
	routeReg := registry.NewRouteRegistry()
	capReg := registry.NewCapabilityRegistry()
	rateLimiter := registry.NewRouteRateLimiter()
	logger := slog.Default()

	proxy := NewHTTPProxy(lookup, routeReg, capReg, rateLimiter, logger)

	// Register some routes and capabilities
	capReg.Register("test-plugin", "test-capability", "/test")

	// Verify registered
	if capReg.Resolve("test-capability") == nil {
		t.Fatal("expected capability to be registered")
	}

	// Unregister
	proxy.UnregisterPlugin("test-plugin")

	// Verify unregistered
	if capReg.Resolve("test-capability") != nil {
		t.Error("expected capability to be unregistered")
	}
}

func TestFormatRateLimit(t *testing.T) {
	tests := []struct {
		input    int32
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{60, "60"},
		{1000, "1000"},
	}

	for _, tt := range tests {
		result := formatRateLimit(tt.input)
		if result != tt.expected {
			t.Errorf("formatRateLimit(%d) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"regular error", errors.New("some error"), false},
		{"gRPC unavailable", status.Error(codes.Unavailable, "unavailable"), true},
		{"gRPC internal", status.Error(codes.Internal, "internal"), true},
		{"gRPC not found", status.Error(codes.NotFound, "not found"), false},
		{"connection refused string", errors.New("connection refused"), true},
		{"connection reset string", errors.New("connection reset by peer"), true},
		{"broken pipe string", errors.New("broken pipe"), true},
		{"EOF string", errors.New("EOF"), true},
		{"transport closing", errors.New("transport is closing"), true},
		{"no connection", errors.New("no connection available"), true},
		{"case insensitive", errors.New("CONNECTION REFUSED"), true},
		{"syscall ECONNREFUSED", syscall.ECONNREFUSED, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isConnectionError(tt.err)
			if result != tt.expected {
				t.Errorf("isConnectionError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		setup    func(*gin.Context)
		expected string
	}{
		{
			name:     "no user_id",
			setup:    func(c *gin.Context) {},
			expected: "",
		},
		{
			name: "string user_id",
			setup: func(c *gin.Context) {
				c.Set("user_id", "user123")
			},
			expected: "user123",
		},
		{
			name: "int64 user_id",
			setup: func(c *gin.Context) {
				c.Set("user_id", int64(456))
			},
			expected: "456",
		},
		{
			name: "int user_id",
			setup: func(c *gin.Context) {
				c.Set("user_id", 789)
			},
			expected: "789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			tt.setup(c)

			result := getUserIDFromContext(c)
			if result != tt.expected {
				t.Errorf("getUserIDFromContext() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestHTTPProxy_HandleCapabilityRoute_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	lookup := &mockPluginLookup{}
	routeReg := registry.NewRouteRegistry()
	capReg := registry.NewCapabilityRegistry()
	rateLimiter := registry.NewRouteRateLimiter()
	logger := slog.Default()

	proxy := NewHTTPProxy(lookup, routeReg, capReg, rateLimiter, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	handler := proxy.HandleCapabilityRoute("nonexistent")
	handler(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestHTTPProxy_HandlePluginRoute_PluginNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	lookup := &mockPluginLookup{plugins: make(map[string]*types.Instance)}
	routeReg := registry.NewRouteRegistry()
	capReg := registry.NewCapabilityRegistry()
	rateLimiter := registry.NewRouteRateLimiter()
	logger := slog.Default()

	proxy := NewHTTPProxy(lookup, routeReg, capReg, rateLimiter, logger)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{
		{Key: "plugin_id", Value: "nonexistent"},
		{Key: "path", Value: "/test"},
	}

	proxy.HandlePluginRoute(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestHTTPProxy_Stop(t *testing.T) {
	lookup := &mockPluginLookup{}
	routeReg := registry.NewRouteRegistry()
	capReg := registry.NewCapabilityRegistry()
	rateLimiter := registry.NewRouteRateLimiter()
	logger := slog.Default()

	proxy := NewHTTPProxy(lookup, routeReg, capReg, rateLimiter, logger)

	// Should not panic
	proxy.Stop()
}
