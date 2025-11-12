package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger returns a middleware that logs HTTP requests using structured logging
func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get status code
		status := c.Writer.Status()

		// Build log attributes
		attrs := []any{
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"latency", latency.String(),
			"ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
		}

		// Add query string if present
		if query != "" {
			attrs = append(attrs, "query", query)
		}

		// Add error if present
		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		// Log based on status code
		switch {
		case status >= 500:
			logger.Error("HTTP request", attrs...)
		case status >= 400:
			logger.Warn("HTTP request", attrs...)
		default:
			logger.Info("HTTP request", attrs...)
		}
	}
}
