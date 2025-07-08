// Package core provides the core functionality for the playback module.
package streaming

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	playbackutils "github.com/mantonx/viewra/internal/modules/playbackmodule/utils"
	"github.com/mantonx/viewra/internal/utils"
)

// ProgressiveHandler handles progressive download with HTTP range support.
// This enables efficient video streaming without requiring the entire file
// to be downloaded before playback can begin.
type ProgressiveHandler struct {
	logger hclog.Logger
}

// NewProgressiveHandler creates a new progressive download handler
func NewProgressiveHandler(logger hclog.Logger) *ProgressiveHandler {
	return &ProgressiveHandler{
		logger: logger,
	}
}

// ServeFile serves a media file with proper range support for progressive download
func (ph *ProgressiveHandler) ServeFile(c *gin.Context, filePath string) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		ph.logger.Error("Failed to open file", "path", filePath, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		ph.logger.Error("Failed to stat file", "path", filePath, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file info"})
		return
	}

	// Set content type based on file extension
	contentType := ph.getContentType(filePath)
	c.Header("Content-Type", contentType)

	// Handle range requests
	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" {
		ph.serveRangeRequest(c, file, fileInfo, rangeHeader)
	} else {
		ph.serveFullFile(c, file, fileInfo)
	}
}

// serveFullFile serves the entire file
func (ph *ProgressiveHandler) serveFullFile(c *gin.Context, file *os.File, fileInfo os.FileInfo) {
	// Set headers for full file
	c.Header("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "public, max-age=3600")

	// Set status
	c.Status(http.StatusOK)

	// Copy file to response
	if _, err := io.Copy(c.Writer, file); err != nil {
		ph.logger.Error("Failed to write file", "error", err)
	}
}

// serveRangeRequest handles HTTP range requests for partial content
func (ph *ProgressiveHandler) serveRangeRequest(c *gin.Context, file *os.File, fileInfo os.FileInfo, rangeHeader string) {
	// Parse range header using shared utils
	httpRange, err := playbackutils.ParseRangeHeader(rangeHeader, fileInfo.Size())
	if err != nil {
		ph.logger.Error("Invalid range header", "range", rangeHeader, "error", err)
		c.Header("Content-Range", fmt.Sprintf("bytes */%d", fileInfo.Size()))
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	// Check if we got a valid range
	if httpRange == nil {
		c.Header("Content-Range", fmt.Sprintf("bytes */%d", fileInfo.Size()))
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	// Seek to start position
	_, err = file.Seek(httpRange.Start, io.SeekStart)
	if err != nil {
		ph.logger.Error("Failed to seek", "offset", httpRange.Start, "error", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	// Set headers for partial content
	c.Header("Content-Range", playbackutils.FormatContentRange(httpRange.Start, httpRange.End, fileInfo.Size()))
	c.Header("Content-Length", strconv.FormatInt(httpRange.Length, 10))
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "public, max-age=3600")

	// Set status to partial content
	c.Status(http.StatusPartialContent)

	// Copy the requested range
	if _, err := io.CopyN(c.Writer, file, httpRange.Length); err != nil && err != io.EOF {
		ph.logger.Error("Failed to write range", "error", err)
	}
}

// getContentType determines the content type based on file extension
func (ph *ProgressiveHandler) getContentType(filePath string) string {
	// Use shared utils for content type detection
	contentType := utils.GetContentType(filePath)
	if contentType != "" && contentType != "application/octet-stream" {
		return contentType
	}

	// Additional media-specific types not in shared utils
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mkv":
		return "video/x-matroska"
	case ".webm":
		return "video/webm"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".flv":
		return "video/x-flv"
	case ".opus":
		return "audio/opus"
	case ".wma":
		return "audio/x-ms-wma"
	case ".aiff":
		return "audio/aiff"
	case ".ape":
		return "audio/x-ape"
	case ".dts":
		return "audio/vnd.dts"
	case ".ac3":
		return "audio/ac3"
	default:
		return "application/octet-stream"
	}
}

// SetHeaders sets common HTTP headers for media streaming
func (ph *ProgressiveHandler) SetHeaders(c *gin.Context, fileName string, fileSize int64, isRangeRequest bool) {
	// Security headers
	c.Header("X-Content-Type-Options", "nosniff")

	// Caching headers
	c.Header("Cache-Control", "public, max-age=3600")
	c.Header("ETag", fmt.Sprintf(`"%d"`, fileSize))

	// CORS headers for cross-origin playback
	origin := c.GetHeader("Origin")
	if origin != "" {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Range")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")
	}

	// Filename for downloads
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filepath.Base(fileName)))
}
