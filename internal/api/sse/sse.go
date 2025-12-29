package sse

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/domain/events"
)

// SetHeaders configures the standard headers required for Server-Sent Events.
// This includes Content-Type, Cache-Control, Connection keep-alive, and
// X-Accel-Buffering to disable nginx buffering.
func SetHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
}

// Flush writes and flushes SSE data to the client.
func Flush(c *gin.Context) {
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

// GetInt64FromEvent extracts an int64 value from an event's Data map by key.
// It handles multiple numeric types that can occur from direct assignment (int64),
// JSON unmarshaling (float64), or Go defaults (int).
// Returns the value and true if found, or 0 and false if not present or invalid.
func GetInt64FromEvent(event events.Event, key string) (int64, bool) {
	if event.Data == nil {
		return 0, false
	}

	switch v := event.Data[key].(type) {
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case int:
		return int64(v), true
	default:
		return 0, false
	}
}
