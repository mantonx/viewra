// Package http provides HTTP-related utilities for the transcoding module.
// It includes helpers for handling HTTP headers, CORS configuration, and
// media type detection.
package http

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// SetContentHeaders sets appropriate HTTP headers based on file type.
// This is used by content serving handlers to ensure proper MIME types
// and CORS headers for video streaming.
func SetContentHeaders(c *gin.Context, fileName string) {
	// Set content type based on file extension
	switch {
	case strings.HasSuffix(fileName, ".mpd"):
		c.Header("Content-Type", "application/dash+xml")
		c.Header("Access-Control-Allow-Origin", "*") // Required for DASH
		c.Header("Access-Control-Allow-Headers", "Content-Type")

	case strings.HasSuffix(fileName, ".m3u8"):
		c.Header("Content-Type", "application/vnd.apple.mpegurl")
		c.Header("Access-Control-Allow-Origin", "*") // Required for HLS
		c.Header("Access-Control-Allow-Headers", "Content-Type")

	case strings.HasSuffix(fileName, ".m4s"):
		c.Header("Content-Type", "video/iso.segment")

	case strings.HasSuffix(fileName, ".ts"):
		c.Header("Content-Type", "video/mp2t")

	case strings.HasSuffix(fileName, ".mp4"):
		c.Header("Content-Type", "video/mp4")

	case strings.HasSuffix(fileName, ".webm"):
		c.Header("Content-Type", "video/webm")

	default:
		// Let Gin determine the content type
	}

	// Add security headers
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
}

// SetCacheHeaders sets appropriate cache headers for content.
// - Permanent content (with hash) can be cached forever
// - Temporary content (session-based) should not be cached
func SetCacheHeaders(c *gin.Context, isPermanent bool, identifier string) {
	if isPermanent {
		// Content-addressable storage can be cached forever
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.Header("ETag", `"`+identifier+`"`)
	} else {
		// Session content is temporary, don't cache it
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		if identifier != "" {
			c.Header("ETag", `"`+identifier+`"`)
		}
	}
}
