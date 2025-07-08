// Package utils provides common utility functions for the Viewra application
package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// HTTPRange represents a parsed HTTP range request
type HTTPRange struct {
	Start int64
	End   int64
}

// ParseRangeHeader parses an HTTP Range header and returns the requested byte range.
// The header format is "bytes=start-end" where start and end are byte positions.
// Returns an error if the header is malformed or invalid.
//
// Examples:
//   - "bytes=0-1023" -> start=0, end=1023
//   - "bytes=1024-" -> start=1024, end=fileSize-1
//   - "bytes=-1024" -> last 1024 bytes (not supported, returns error)
func ParseRangeHeader(rangeHeader string, fileSize int64) (*HTTPRange, error) {
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return nil, fmt.Errorf("invalid range header format")
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeSpec, "-")
	
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range specification")
	}
	
	r := &HTTPRange{}
	var err error
	
	// Parse start byte
	if parts[0] != "" {
		r.Start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || r.Start < 0 {
			return nil, fmt.Errorf("invalid start byte")
		}
	} else {
		// bytes=-100 format (last N bytes) - not commonly used for video streaming
		return nil, fmt.Errorf("suffix byte range not supported")
	}
	
	// Parse end byte
	if parts[1] != "" {
		r.End, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || r.End >= fileSize {
			r.End = fileSize - 1
		}
	} else {
		r.End = fileSize - 1
	}
	
	// Validate range
	if r.Start > r.End || r.Start >= fileSize {
		return nil, fmt.Errorf("invalid byte range")
	}
	
	return r, nil
}

// GetMediaContentType returns the appropriate MIME type for a media container format.
// It supports common video, audio, and subtitle formats.
//
// Examples:
//   - "mp4" -> "video/mp4"
//   - "mkv" -> "video/x-matroska"
//   - "mp3" -> "audio/mpeg"
//   - "unknown" -> "application/octet-stream"
func GetMediaContentType(container string) string {
	switch strings.ToLower(container) {
	// Video formats
	case "mp4", "m4v":
		return "video/mp4"
	case "mkv", "matroska":
		return "video/x-matroska"
	case "webm":
		return "video/webm"
	case "mov":
		return "video/quicktime"
	case "avi":
		return "video/x-msvideo"
	case "flv":
		return "video/x-flv"
	case "wmv":
		return "video/x-ms-wmv"
	case "mpg", "mpeg":
		return "video/mpeg"
	case "ts", "mts":
		return "video/mp2t"
	case "3gp":
		return "video/3gpp"
	case "ogv":
		return "video/ogg"
		
	// Audio formats
	case "mp3":
		return "audio/mpeg"
	case "aac", "m4a":
		return "audio/mp4"
	case "ogg", "oga":
		return "audio/ogg"
	case "flac":
		return "audio/flac"
	case "wav":
		return "audio/wav"
	case "opus":
		return "audio/opus"
	case "wma":
		return "audio/x-ms-wma"
	case "alac":
		return "audio/x-alac"
		
	// Subtitle formats
	case "srt":
		return "text/plain; charset=utf-8"
	case "vtt":
		return "text/vtt"
	case "ass", "ssa":
		return "text/x-ssa"
		
	// Default
	default:
		return "application/octet-stream"
	}
}

// ServeFileWithRange serves a file with HTTP range request support.
// This is essential for video streaming to support seeking.
//
// Parameters:
//   - w: HTTP response writer
//   - r: HTTP request
//   - filePath: Path to the file to serve
//   - contentType: MIME type of the file
//
// The function handles:
//   - Full file requests (no Range header)
//   - Partial content requests (Range header present)
//   - HEAD requests (returns headers only)
func ServeFileWithRange(w http.ResponseWriter, r *http.Request, filePath string, contentType string) error {
	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	
	fileSize := fileInfo.Size()
	
	// Set common headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	
	// Handle HEAD request
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))
		w.WriteHeader(http.StatusOK)
		return nil
	}
	
	// Check for range header
	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		// Serve entire file
		w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))
		w.WriteHeader(http.StatusOK)
		_, err = io.Copy(w, file)
		return err
	}
	
	// Parse range header
	httpRange, err := ParseRangeHeader(rangeHeader, fileSize)
	if err != nil {
		http.Error(w, "Invalid range", http.StatusRequestedRangeNotSatisfiable)
		return nil
	}
	
	// Seek to start position
	_, err = file.Seek(httpRange.Start, 0)
	if err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}
	
	// Calculate content length
	contentLength := httpRange.End - httpRange.Start + 1
	
	// Set range response headers
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", httpRange.Start, httpRange.End, fileSize))
	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.WriteHeader(http.StatusPartialContent)
	
	// Copy the requested range
	_, err = io.CopyN(w, file, contentLength)
	return err
}

// GetFileExtension returns the file extension from a file path.
// The extension is returned in lowercase without the leading dot.
//
// Examples:
//   - "/path/to/video.MP4" -> "mp4"
//   - "/path/to/audio.FLAC" -> "flac"
//   - "/path/to/file" -> ""
func GetFileExtension(filePath string) string {
	parts := strings.Split(filePath, ".")
	if len(parts) < 2 {
		return ""
	}
	return strings.ToLower(parts[len(parts)-1])
}