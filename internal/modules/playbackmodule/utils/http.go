// Package utils provides playback-specific utility functions.
package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// HTTPRange represents a byte range for HTTP range requests
type HTTPRange struct {
	Start  int64
	End    int64
	Length int64
}

// ParseRangeHeader parses an HTTP Range header and returns the requested ranges.
// Supports single ranges only for now (most common case for media streaming).
// Returns an error if the range header is invalid or requests multiple ranges.
func ParseRangeHeader(rangeHeader string, fileSize int64) (*HTTPRange, error) {
	if rangeHeader == "" {
		return nil, nil
	}

	// Remove "bytes=" prefix
	const prefix = "bytes="
	if !strings.HasPrefix(rangeHeader, prefix) {
		return nil, fmt.Errorf("invalid range header: %s", rangeHeader)
	}

	rangeSpec := rangeHeader[len(prefix):]

	// Check for multiple ranges (not supported)
	if strings.Contains(rangeSpec, ",") {
		return nil, fmt.Errorf("multiple ranges not supported")
	}

	// Split range into start and end
	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range format")
	}

	var start, end int64
	var err error

	// Parse start
	if parts[0] != "" {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid range start: %s", parts[0])
		}
	}

	// Parse end
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid range end: %s", parts[1])
		}
	} else {
		// If no end specified, go to end of file
		end = fileSize - 1
	}

	// Handle suffix-length form (e.g., "-500" for last 500 bytes)
	if parts[0] == "" && parts[1] != "" {
		start = fileSize - end
		end = fileSize - 1
	}

	// Validate range
	if start < 0 || end >= fileSize || start > end {
		return nil, fmt.Errorf("range out of bounds: %d-%d (file size: %d)", start, end, fileSize)
	}

	return &HTTPRange{
		Start:  start,
		End:    end,
		Length: end - start + 1,
	}, nil
}

// FormatContentRange formats a Content-Range header value
func FormatContentRange(start, end, total int64) string {
	return fmt.Sprintf("bytes %d-%d/%d", start, end, total)
}

// CalculatePartialContentLength calculates the content length for a partial response
func CalculatePartialContentLength(start, end int64) int64 {
	return end - start + 1
}
