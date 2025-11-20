package middleware

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// Gzip returns a middleware that compresses responses using gzip compression
// This significantly reduces payload sizes for JSON responses and static assets
func Gzip() gin.HandlerFunc {
	return gzip.Gzip(gzip.DefaultCompression)
}
