package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter provides configurable rate limiting for API endpoints.
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string]*bucket
	rate     int           // max requests per window
	window   time.Duration // time window
	cleanup  time.Duration // how often to clean old entries
}

type bucket struct {
	count    int
	windowStart time.Time
}

// NewRateLimiter creates a rate limiter with the specified rate and window.
// For example, NewRateLimiter(5, time.Minute) allows 5 requests per minute.
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*bucket),
		rate:     rate,
		window:   window,
		cleanup:  5 * time.Minute,
	}
	go rl.cleanupLoop()
	return rl
}

// Allow checks if a request from the given key should be allowed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.requests[key]

	if !exists || now.Sub(b.windowStart) >= rl.window {
		// New window
		rl.requests[key] = &bucket{
			count:       1,
			windowStart: now,
		}
		return true
	}

	// Same window
	if b.count >= rl.rate {
		return false
	}

	b.count++
	return true
}

// cleanupLoop periodically removes stale entries.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, b := range rl.requests {
			if now.Sub(b.windowStart) >= rl.window*2 {
				delete(rl.requests, key)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimitByIP returns middleware that rate limits by client IP.
func RateLimitByIP(limiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		if !limiter.Allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimitByIPAndPath returns middleware that rate limits by IP + path.
// This allows different endpoints to have independent rate limits.
func RateLimitByIPAndPath(limiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP() + ":" + c.Request.URL.Path
		if !limiter.Allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// AuthRateLimiters holds rate limiters for auth endpoints.
type AuthRateLimiters struct {
	Login   *RateLimiter
	Refresh *RateLimiter
	Setup   *RateLimiter
}

// NewAuthRateLimiters creates rate limiters with sensible defaults for auth endpoints.
func NewAuthRateLimiters() *AuthRateLimiters {
	return &AuthRateLimiters{
		// Login: 5 attempts per minute per IP (brute force protection)
		Login: NewRateLimiter(5, time.Minute),
		// Refresh: 30 per minute per IP (tokens are short-lived, legitimate refreshes are frequent)
		Refresh: NewRateLimiter(30, time.Minute),
		// Setup: 3 per minute per IP (only needed once, prevent abuse)
		Setup: NewRateLimiter(3, time.Minute),
	}
}
