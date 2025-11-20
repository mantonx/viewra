package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestID adds a unique request ID to each request for tracing and debugging
func RequestID(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if client provided a request ID
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate a new UUID if not provided
			requestID = uuid.New().String()
		}

		// Store request ID in context for handlers to access
		c.Set("request_id", requestID)

		// Add request ID to response headers
		c.Writer.Header().Set("X-Request-ID", requestID)

		// Create a logger with the request ID for this request
		requestLogger := logger.With(
			"request_id", requestID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
		)

		// Store the request-specific logger in context
		c.Set("logger", requestLogger)

		c.Next()
	}
}
