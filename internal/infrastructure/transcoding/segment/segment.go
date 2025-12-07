// Package segment provides HLS segment number calculations and filename utilities.
package segment

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
)

const (
	// Duration is the duration of each HLS segment in seconds.
	// 2 seconds provides faster startup while maintaining reasonable file count.
	Duration = 2

	// FilenameFormat is the format string for segment filenames
	FilenameFormat = "seg_%06d.ts"

	// FilenamePattern is the regex pattern for parsing segment filenames
	FilenamePattern = `^seg_(\d{6})\.ts$`

	// InitFilename is kept for backwards compatibility (not used with MPEG-TS)
	InitFilename = "init.mp4"
)

var filenameRegex = regexp.MustCompile(FilenamePattern)

// Number calculates the segment number from a timestamp in seconds.
// Example: timestamp=37 → segment 18 (37 / 2 = 18.5 → floor = 18)
func Number(timestampSeconds float64) int {
	return int(math.Floor(timestampSeconds / Duration))
}

// StartTime calculates the start time in seconds for a given segment number.
// Example: segment=18 → startTime=36 (18 * 2 = 36)
func StartTime(segmentNumber int) float64 {
	return float64(segmentNumber * Duration)
}

// EndTime calculates the end time in seconds for a given segment number.
// Example: segment=18 → endTime=38 (36 + 2 = 38)
func EndTime(segmentNumber int) float64 {
	return StartTime(segmentNumber) + Duration
}

// Filename generates a filename for a segment number.
// Example: segment=123 → "seg_000123.ts"
func Filename(segmentNumber int) string {
	return fmt.Sprintf(FilenameFormat, segmentNumber)
}

// ParseNumber extracts the segment number from a filename.
// Example: "seg_000123.ts" → 123
// Returns -1 if the filename doesn't match the expected pattern.
func ParseNumber(filename string) int {
	matches := filenameRegex.FindStringSubmatch(filename)
	if len(matches) != 2 {
		return -1
	}

	segmentNum, err := strconv.Atoi(matches[1])
	if err != nil {
		return -1
	}

	return segmentNum
}

// Range calculates the range of segment numbers that cover a time range.
// Example: startTime=35, duration=20 → segments 17-27 (35/2=17.5→17, 55/2=27.5→27)
func Range(startTimeSeconds, durationSeconds float64) (startSegment, endSegment int) {
	startSegment = Number(startTimeSeconds)
	endTime := startTimeSeconds + durationSeconds
	endSegment = Number(endTime)
	return startSegment, endSegment
}

// ValidateNumber checks if a segment number is valid for a given video duration.
func ValidateNumber(segmentNumber int, videoDurationSeconds float64) bool {
	if segmentNumber < 0 {
		return false
	}

	maxSegment := Number(videoDurationSeconds)
	return segmentNumber <= maxSegment
}
