package transcoding

import (
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/segment"
)

// Segment duration and filename constants.
// Re-exported from segment subpackage for backward compatibility.
const (
	// SegmentDuration is the duration of each HLS segment in seconds.
	SegmentDuration = segment.Duration

	// SegmentFilenameFormat is the format string for segment filenames
	SegmentFilenameFormat = segment.FilenameFormat

	// SegmentFilenamePattern is the regex pattern for parsing segment filenames
	SegmentFilenamePattern = segment.FilenamePattern

	// InitSegmentFilename is kept for backwards compatibility (not used with MPEG-TS)
	InitSegmentFilename = segment.InitFilename
)

// SegmentNumber calculates the segment number from a timestamp in seconds.
// Re-exported from segment subpackage for backward compatibility.
func SegmentNumber(timestampSeconds float64) int {
	return segment.Number(timestampSeconds)
}

// SegmentStartTime calculates the start time in seconds for a given segment number.
// Re-exported from segment subpackage for backward compatibility.
func SegmentStartTime(segmentNumber int) float64 {
	return segment.StartTime(segmentNumber)
}

// SegmentEndTime calculates the end time in seconds for a given segment number.
// Re-exported from segment subpackage for backward compatibility.
func SegmentEndTime(segmentNumber int) float64 {
	return segment.EndTime(segmentNumber)
}

// SegmentFilename generates a filename for a segment number.
// Re-exported from segment subpackage for backward compatibility.
func SegmentFilename(segmentNumber int) string {
	return segment.Filename(segmentNumber)
}

// ParseSegmentNumber extracts the segment number from a filename.
// Re-exported from segment subpackage for backward compatibility.
func ParseSegmentNumber(filename string) int {
	return segment.ParseNumber(filename)
}

// SegmentRange calculates the range of segment numbers that cover a time range.
// Re-exported from segment subpackage for backward compatibility.
func SegmentRange(startTimeSeconds, durationSeconds float64) (startSegment, endSegment int) {
	return segment.Range(startTimeSeconds, durationSeconds)
}

// ValidateSegmentNumber checks if a segment number is valid for a given video duration.
// Re-exported from segment subpackage for backward compatibility.
func ValidateSegmentNumber(segmentNumber int, videoDurationSeconds float64) bool {
	return segment.ValidateNumber(segmentNumber, videoDurationSeconds)
}
