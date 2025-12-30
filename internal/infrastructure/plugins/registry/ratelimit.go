package registry

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// RouteRateLimiter provides per-route rate limiting for plugin routes.
// Supports both global and per-user rate limits.
type RouteRateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*tokenBucket // key: route+userID (or just route for global)
	cleanup *time.Ticker
	done    chan struct{}
}

// tokenBucket implements a simple token bucket algorithm.
type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewRouteRateLimiter creates a new rate limiter.
func NewRouteRateLimiter() *RouteRateLimiter {
	rl := &RouteRateLimiter{
		buckets: make(map[string]*tokenBucket),
		cleanup: time.NewTicker(5 * time.Minute),
		done:    make(chan struct{}),
	}

	// Background cleanup of stale buckets
	go rl.cleanupLoop()

	return rl
}

// Stop stops the rate limiter's background cleanup.
func (rl *RouteRateLimiter) Stop() {
	close(rl.done)
	rl.cleanup.Stop()
}

// cleanupLoop periodically removes stale buckets.
func (rl *RouteRateLimiter) cleanupLoop() {
	for {
		select {
		case <-rl.cleanup.C:
			rl.cleanupStaleBuckets()
		case <-rl.done:
			return
		}
	}
}

// cleanupStaleBuckets removes buckets that haven't been used recently.
func (rl *RouteRateLimiter) cleanupStaleBuckets() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, bucket := range rl.buckets {
		// Remove buckets that haven't been used in 10 minutes
		if now.Sub(bucket.lastRefill) > 10*time.Minute {
			delete(rl.buckets, key)
		}
	}
}

// Middleware returns a gin middleware that enforces rate limiting for a route.
func (rl *RouteRateLimiter) Middleware(route *RegisteredRoute, getUserID func(c *gin.Context) string) gin.HandlerFunc {
	if route.RateLimit == nil || route.RateLimit.RequestsPerMinute <= 0 {
		// No rate limit configured
		return func(c *gin.Context) {
			c.Next()
		}
	}

	limit := route.RateLimit
	rpm := float64(limit.RequestsPerMinute)

	return func(c *gin.Context) {
		var key string
		if limit.PerUser {
			userID := getUserID(c)
			if userID == "" {
				// No user ID, can't rate limit per-user
				c.Next()
				return
			}
			key = route.FullPath + ":" + userID
		} else {
			key = route.FullPath + ":global"
		}

		if !rl.allow(key, rpm) {
			c.Header("X-RateLimit-Limit", formatFloat(rpm))
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": 60,
			})
			return
		}

		c.Next()
	}
}

// allow checks if a request is allowed under the rate limit.
func (rl *RouteRateLimiter) allow(key string, rpm float64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, ok := rl.buckets[key]
	if !ok {
		// Create new bucket
		bucket = &tokenBucket{
			tokens:     rpm,        // Start full
			maxTokens:  rpm,        // Max tokens = requests per minute
			refillRate: rpm / 60.0, // Tokens per second
			lastRefill: time.Now(),
		}
		rl.buckets[key] = bucket
	}

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * bucket.refillRate
	if bucket.tokens > bucket.maxTokens {
		bucket.tokens = bucket.maxTokens
	}
	bucket.lastRefill = now

	// Check if we have a token available
	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		return true
	}

	return false
}

// CheckRateLimit checks if a request would be allowed (without consuming a token).
func (rl *RouteRateLimiter) CheckRateLimit(route *RegisteredRoute, userID string) bool {
	if route.RateLimit == nil || route.RateLimit.RequestsPerMinute <= 0 {
		return true
	}

	var key string
	if route.RateLimit.PerUser {
		key = route.FullPath + ":" + userID
	} else {
		key = route.FullPath + ":global"
	}

	rl.mu.RLock()
	bucket, ok := rl.buckets[key]
	rl.mu.RUnlock()

	if !ok {
		return true // No bucket yet means no requests made
	}

	rl.mu.RLock()
	defer rl.mu.RUnlock()

	// Calculate current token count
	elapsed := time.Since(bucket.lastRefill).Seconds()
	tokens := bucket.tokens + elapsed*bucket.refillRate
	if tokens > bucket.maxTokens {
		tokens = bucket.maxTokens
	}

	return tokens >= 1.0
}

// sharedRateLimiter is a singleton rate limiter shared across all routes.
// This prevents goroutine leaks from creating a new limiter per route.
var (
	sharedRateLimiter     *RouteRateLimiter
	sharedRateLimiterOnce sync.Once
)

// GetSharedRateLimiter returns the singleton rate limiter.
func GetSharedRateLimiter() *RouteRateLimiter {
	sharedRateLimiterOnce.Do(func() {
		sharedRateLimiter = NewRouteRateLimiter()
	})
	return sharedRateLimiter
}

// CreateRateLimitMiddleware creates a rate limit middleware from proto config.
// Uses a shared rate limiter to prevent goroutine leaks.
func CreateRateLimitMiddleware(limit *pluginv1.PluginRateLimit, routePath string, getUserID func(c *gin.Context) string) gin.HandlerFunc {
	if limit == nil || limit.RequestsPerMinute <= 0 {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	rl := GetSharedRateLimiter()
	route := &RegisteredRoute{
		FullPath:  routePath,
		RateLimit: limit,
	}

	return rl.Middleware(route, getUserID)
}

func formatFloat(f float64) string {
	if f == float64(int(f)) {
		return fmt.Sprintf("%d", int(f))
	}
	return fmt.Sprintf("%.2f", f)
}
