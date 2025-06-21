package middleware

import (
	"bytes"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
)

// RequestLogger logs all HTTP requests in development mode
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip logging for health checks
		if c.Request.URL.Path == "/api/health" {
			c.Next()
			return
		}

		start := time.Now()
		
		// Read and log request body
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			// Restore the body for further processing
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Log request
		logger.Debug("HTTP Request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"query", c.Request.URL.RawQuery,
			"body", string(bodyBytes),
			"headers", c.Request.Header,
			"ip", c.ClientIP(),
		)

		// Process request
		c.Next()

		// Log response
		duration := time.Since(start)
		logger.Debug("HTTP Response",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", duration.String(),
			"size", c.Writer.Size(),
		)
	}
}

// ErrorLogger logs errors with context
func ErrorLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		
		// Log any errors
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				logger.Error("Request error",
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
					"error", err.Error(),
					"type", err.Type,
				)
			}
		}
	}
}