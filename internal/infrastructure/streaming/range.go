package streaming

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseRangeHeader parses an HTTP Range header and returns byte ranges
// Supports formats:
// - "bytes=0-499"     -> range from 0 to 499
// - "bytes=500-"      -> range from 500 to end
// - "bytes=-500"      -> last 500 bytes
// - "bytes=0-0,-1"    -> multiple ranges (returns error for now)
func ParseRangeHeader(rangeHeader string, fileSize int64) ([]ByteRange, error) {
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return nil, fmt.Errorf("invalid range header format")
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	rangeParts := strings.Split(rangeSpec, ",")

	var ranges []ByteRange
	for _, part := range rangeParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		r, err := parseRangePart(part, fileSize)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, r)
	}

	if len(ranges) == 0 {
		return nil, fmt.Errorf("no valid ranges found")
	}

	return ranges, nil
}

// parseRangePart parses a single range part (e.g., "0-499", "500-", "-500")
func parseRangePart(part string, fileSize int64) (ByteRange, error) {
	dashIndex := strings.Index(part, "-")
	if dashIndex == -1 {
		return ByteRange{}, fmt.Errorf("invalid range format: %s", part)
	}

	startStr := part[:dashIndex]
	endStr := part[dashIndex+1:]

	var start, end int64

	if startStr == "" {
		// Suffix range: "-500" means last 500 bytes
		if endStr == "" {
			return ByteRange{}, fmt.Errorf("invalid range format: %s", part)
		}
		suffixLength, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil {
			return ByteRange{}, fmt.Errorf("invalid suffix length: %s", endStr)
		}
		start = fileSize - suffixLength
		if start < 0 {
			start = 0
		}
		end = fileSize - 1
	} else if endStr == "" {
		// Open-ended range: "500-" means from byte 500 to end
		var err error
		start, err = strconv.ParseInt(startStr, 10, 64)
		if err != nil {
			return ByteRange{}, fmt.Errorf("invalid start position: %s", startStr)
		}
		end = fileSize - 1
	} else {
		// Normal range: "500-999"
		var err error
		start, err = strconv.ParseInt(startStr, 10, 64)
		if err != nil {
			return ByteRange{}, fmt.Errorf("invalid start position: %s", startStr)
		}
		end, err = strconv.ParseInt(endStr, 10, 64)
		if err != nil {
			return ByteRange{}, fmt.Errorf("invalid end position: %s", endStr)
		}
	}

	// Validate range
	if start < 0 || start >= fileSize {
		return ByteRange{}, fmt.Errorf("start position out of range: %d", start)
	}
	if end < start {
		return ByteRange{}, fmt.Errorf("end position before start: %d < %d", end, start)
	}
	if end >= fileSize {
		end = fileSize - 1
	}

	return ByteRange{Start: start, End: end}, nil
}
