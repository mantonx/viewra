package registry

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRouteRateLimiter_Allow(t *testing.T) {
	rl := NewRouteRateLimiter()
	defer rl.Stop()

	// Allow 10 requests per minute (1 every 6 seconds)
	key := "test-route:global"
	rpm := 10.0

	// First 10 requests should be allowed (bucket starts full)
	for i := 0; i < 10; i++ {
		if !rl.allow(key, rpm) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 11th request should be blocked
	if rl.allow(key, rpm) {
		t.Error("11th request should be blocked")
	}
}

func TestRouteRateLimiter_Refill(t *testing.T) {
	rl := NewRouteRateLimiter()
	defer rl.Stop()

	key := "test-route:global"
	rpm := 60.0 // 1 request per second

	// Exhaust the bucket
	for i := 0; i < 60; i++ {
		rl.allow(key, rpm)
	}

	// Should be blocked now
	if rl.allow(key, rpm) {
		t.Error("Request should be blocked after exhausting bucket")
	}

	// Wait for 1 second to get a token back
	time.Sleep(1100 * time.Millisecond)

	// Should be allowed now
	if !rl.allow(key, rpm) {
		t.Error("Request should be allowed after refill")
	}
}

func TestRouteRateLimiter_PerUserLimit(t *testing.T) {
	rl := NewRouteRateLimiter()
	defer rl.Stop()

	rpm := 5.0

	// User A and User B should have separate buckets
	keyA := "test-route:user-a"
	keyB := "test-route:user-b"

	// Exhaust user A's bucket
	for i := 0; i < 5; i++ {
		rl.allow(keyA, rpm)
	}

	// User A should be blocked
	if rl.allow(keyA, rpm) {
		t.Error("User A should be blocked")
	}

	// User B should still be allowed
	if !rl.allow(keyB, rpm) {
		t.Error("User B should still be allowed")
	}
}

func TestRouteRateLimiter_Middleware(t *testing.T) {
	rl := NewRouteRateLimiter()
	defer rl.Stop()

	route := &RegisteredRoute{
		FullPath: "/test",
		RateLimit: &pluginv1.PluginRateLimit{
			RequestsPerMinute: 2,
			PerUser:           false,
		},
	}

	getUserID := func(c *gin.Context) string {
		return "test-user"
	}

	middleware := rl.Middleware(route, getUserID)

	router := gin.New()
	router.GET("/test", middleware, func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: got status %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	// 3rd request should be rate limited
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Request 3: got status %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	// Check headers
	if w.Header().Get("X-RateLimit-Limit") != "2" {
		t.Errorf("X-RateLimit-Limit = %s, want 2", w.Header().Get("X-RateLimit-Limit"))
	}
	if w.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %s, want 60", w.Header().Get("Retry-After"))
	}
}

func TestRouteRateLimiter_MiddlewarePerUser(t *testing.T) {
	rl := NewRouteRateLimiter()
	defer rl.Stop()

	route := &RegisteredRoute{
		FullPath: "/test",
		RateLimit: &pluginv1.PluginRateLimit{
			RequestsPerMinute: 1,
			PerUser:           true,
		},
	}

	router := gin.New()

	// Middleware that extracts user from header
	getUserID := func(c *gin.Context) string {
		return c.GetHeader("X-User-ID")
	}

	middleware := rl.Middleware(route, getUserID)
	router.GET("/test", middleware, func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// User A's first request succeeds
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-User-ID", "user-a")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("User A request 1: got status %d, want %d", w.Code, http.StatusOK)
	}

	// User A's second request is blocked
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-User-ID", "user-a")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("User A request 2: got status %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	// User B's request should succeed (separate bucket)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-User-ID", "user-b")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("User B request 1: got status %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRouteRateLimiter_NoLimit(t *testing.T) {
	rl := NewRouteRateLimiter()
	defer rl.Stop()

	// Route with no rate limit
	route := &RegisteredRoute{
		FullPath:  "/test",
		RateLimit: nil,
	}

	getUserID := func(c *gin.Context) string {
		return "test-user"
	}

	middleware := rl.Middleware(route, getUserID)

	router := gin.New()
	router.GET("/test", middleware, func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// All requests should succeed
	for i := 0; i < 100; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: got status %d, want %d", i+1, w.Code, http.StatusOK)
			break
		}
	}
}

func TestRouteRateLimiter_CheckRateLimit(t *testing.T) {
	rl := NewRouteRateLimiter()
	defer rl.Stop()

	route := &RegisteredRoute{
		FullPath: "/test",
		RateLimit: &pluginv1.PluginRateLimit{
			RequestsPerMinute: 1,
			PerUser:           false,
		},
	}

	// Before any requests, should pass
	if !rl.CheckRateLimit(route, "") {
		t.Error("CheckRateLimit should return true before any requests")
	}

	// Consume the token
	rl.allow(route.FullPath+":global", 1.0)

	// Now should fail check
	if rl.CheckRateLimit(route, "") {
		t.Error("CheckRateLimit should return false after exhausting bucket")
	}
}

func TestCreateRateLimitMiddleware(t *testing.T) {
	limit := &pluginv1.PluginRateLimit{
		RequestsPerMinute: 5,
		PerUser:           false,
	}

	getUserID := func(c *gin.Context) string {
		return ""
	}

	middleware := CreateRateLimitMiddleware(limit, "/api/test", getUserID)

	router := gin.New()
	router.GET("/test", middleware, func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// First 5 requests should succeed
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Request %d: got status %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	// 6th request should be blocked
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Request 6: got status %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}

func TestCreateRateLimitMiddleware_NoLimit(t *testing.T) {
	getUserID := func(c *gin.Context) string {
		return ""
	}

	// nil limit should create passthrough middleware
	middleware := CreateRateLimitMiddleware(nil, "/api/test", getUserID)

	router := gin.New()
	router.GET("/test", middleware, func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// All requests should succeed
	for i := 0; i < 100; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Request %d: got status %d, want %d", i+1, w.Code, http.StatusOK)
			break
		}
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{10.0, "10"},
		{10.5, "10.50"},
		{100.0, "100"},
		{0.5, "0.50"},
	}

	for _, tt := range tests {
		got := formatFloat(tt.input)
		if got != tt.want {
			t.Errorf("formatFloat(%v) = %s, want %s", tt.input, got, tt.want)
		}
	}
}
