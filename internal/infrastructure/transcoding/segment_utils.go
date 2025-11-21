package transcoding

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
)

const (
	// SegmentDuration is the duration of each HLS segment in seconds
	SegmentDuration = 6

	// SegmentFilenameFormat is the format string for segment filenames
	SegmentFilenameFormat = "seg_%06d.ts"

	// SegmentFilenamePattern is the regex pattern for parsing segment filenames
	SegmentFilenamePattern = `^seg_(\d{6})\.ts$`
)

var segmentFilenameRegex = regexp.MustCompile(SegmentFilenamePattern)

// SegmentNumber calculates the segment number from a timestamp in seconds.
// Example: timestamp=37 → segment 6 (37 / 6 = 6.166... → floor = 6)
func SegmentNumber(timestampSeconds float64) int {
	return int(math.Floor(timestampSeconds / SegmentDuration))
}

// SegmentStartTime calculates the start time in seconds for a given segment number.
// Example: segment=6 → startTime=36 (6 * 6 = 36)
func SegmentStartTime(segmentNumber int) float64 {
	return float64(segmentNumber * SegmentDuration)
}

// SegmentEndTime calculates the end time in seconds for a given segment number.
// Example: segment=6 → endTime=42 (36 + 6 = 42)
func SegmentEndTime(segmentNumber int) float64 {
	return SegmentStartTime(segmentNumber) + SegmentDuration
}

// SegmentFilename generates a filename for a segment number.
// Example: segment=123 → "seg_000123.ts"
func SegmentFilename(segmentNumber int) string {
	return fmt.Sprintf(SegmentFilenameFormat, segmentNumber)
}

// ParseSegmentNumber extracts the segment number from a filename.
// Example: "seg_000123.ts" → 123
// Returns -1 if the filename doesn't match the expected pattern.
func ParseSegmentNumber(filename string) int {
	matches := segmentFilenameRegex.FindStringSubmatch(filename)
	if len(matches) != 2 {
		return -1
	}

	segmentNum, err := strconv.Atoi(matches[1])
	if err != nil {
		return -1
	}

	return segmentNum
}

// SegmentRange calculates the range of segment numbers that cover a time range.
// Example: startTime=35, duration=20 → segments 5-9 (35/6=5.8→5, 55/6=9.1→9)
func SegmentRange(startTimeSeconds, durationSeconds float64) (startSegment, endSegment int) {
	startSegment = SegmentNumber(startTimeSeconds)
	endTime := startTimeSeconds + durationSeconds
	endSegment = SegmentNumber(endTime)
	return startSegment, endSegment
}

// ValidateSegmentNumber checks if a segment number is valid for a given video duration.
func ValidateSegmentNumber(segmentNumber int, videoDurationSeconds float64) bool {
	if segmentNumber < 0 {
		return false
	}

	maxSegment := SegmentNumber(videoDurationSeconds)
	return segmentNumber <= maxSegment
}
