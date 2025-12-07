package transcoding

import (
	"path/filepath"
	"testing"
)

// These tests verify that the re-exports from the segment subpackage work correctly.
// The detailed implementation tests are in internal/infrastructure/transcoding/segment/segment_test.go

func TestSegmentNumber(t *testing.T) {
	tests := []struct {
		name              string
		timestampSeconds  float64
		expectedSegment   int
	}{
		{"Zero timestamp", 0, 0},
		{"First segment boundary", 1.99, 0},
		{"Second segment start", 2.0, 1},
		{"Example from comment: 37 seconds", 37, 18},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SegmentNumber(tt.timestampSeconds)
			if result != tt.expectedSegment {
				t.Errorf("SegmentNumber(%f) = %d, want %d",
					tt.timestampSeconds, result, tt.expectedSegment)
			}
		})
	}
}

func TestSegmentStartTime(t *testing.T) {
	tests := []struct {
		segmentNumber int
		expectedTime  float64
	}{
		{0, 0.0},
		{1, 2.0},
		{6, 12.0},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := SegmentStartTime(tt.segmentNumber)
			if result != tt.expectedTime {
				t.Errorf("SegmentStartTime(%d) = %f, want %f",
					tt.segmentNumber, result, tt.expectedTime)
			}
		})
	}
}

func TestSegmentEndTime(t *testing.T) {
	tests := []struct {
		segmentNumber int
		expectedTime  float64
	}{
		{0, 2.0},
		{1, 4.0},
		{6, 14.0},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := SegmentEndTime(tt.segmentNumber)
			if result != tt.expectedTime {
				t.Errorf("SegmentEndTime(%d) = %f, want %f",
					tt.segmentNumber, result, tt.expectedTime)
			}
		})
	}
}

func TestSegmentFilename(t *testing.T) {
	tests := []struct {
		segment  int
		expected string
	}{
		{0, "seg_000000.ts"},
		{123, "seg_000123.ts"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := SegmentFilename(tt.segment)
			if result != tt.expected {
				t.Errorf("SegmentFilename(%d) = %s, want %s", tt.segment, result, tt.expected)
			}
		})
	}
}

func TestParseSegmentNumber(t *testing.T) {
	tests := []struct {
		filename string
		expected int
	}{
		{"seg_000000.ts", 0},
		{"seg_000123.ts", 123},
		{"invalid.ts", -1},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := ParseSegmentNumber(tt.filename)
			if result != tt.expected {
				t.Errorf("ParseSegmentNumber(%s) = %d, want %d", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestSegmentRange(t *testing.T) {
	startSeg, endSeg := SegmentRange(35, 20)
	if startSeg != 17 || endSeg != 27 {
		t.Errorf("SegmentRange(35, 20) = (%d, %d), want (17, 27)", startSeg, endSeg)
	}
}

func TestValidateSegmentNumber(t *testing.T) {
	tests := []struct {
		segmentNumber int
		videoDuration float64
		expectedValid bool
	}{
		{-1, 100.0, false},
		{0, 100.0, true},
		{50, 100.0, true},
		{51, 100.0, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := ValidateSegmentNumber(tt.segmentNumber, tt.videoDuration)
			if result != tt.expectedValid {
				t.Errorf("ValidateSegmentNumber(%d, %f) = %v, want %v",
					tt.segmentNumber, tt.videoDuration, result, tt.expectedValid)
			}
		})
	}
}

func TestSegmentConstants(t *testing.T) {
	if SegmentDuration != 2 {
		t.Errorf("SegmentDuration = %d, want 2", SegmentDuration)
	}

	if SegmentFilenameFormat != "seg_%06d.ts" {
		t.Errorf("SegmentFilenameFormat = %s, want seg_%%06d.ts", SegmentFilenameFormat)
	}
}

// Path utility tests (testing paths.go, not segment_utils.go)

func TestGetHLSOutputPath(t *testing.T) {
	tests := []struct {
		name      string
		outputDir string
		mediaID   int64
		quality   string
		expected  string
	}{
		{
			name:      "Basic path with lowercase quality",
			outputDir: "/tmp/dash",
			mediaID:   12345,
			quality:   "720p",
			expected:  filepath.Join("/tmp/dash", "hls", "12345", "720p"),
		},
		{
			name:      "Quality normalized to lowercase",
			outputDir: "/tmp/dash",
			mediaID:   12345,
			quality:   "1080P",
			expected:  filepath.Join("/tmp/dash", "hls", "12345", "1080p"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetHLSOutputPath(tt.outputDir, tt.mediaID, tt.quality)
			if result != tt.expected {
				t.Errorf("GetHLSOutputPath(%s, %d, %s) = %s, want %s",
					tt.outputDir, tt.mediaID, tt.quality, result, tt.expected)
			}
		})
	}
}

func TestGetHLSManifestPath(t *testing.T) {
	result := GetHLSManifestPath("/tmp/dash", 12345, "720p")
	expected := filepath.Join("/tmp/dash", "hls", "12345", "720p", "playlist.m3u8")
	if result != expected {
		t.Errorf("GetHLSManifestPath() = %s, want %s", result, expected)
	}
}

func TestGetHLSSegmentPath(t *testing.T) {
	result := GetHLSSegmentPath("/tmp/dash", 12345, "720p", 42)
	expected := filepath.Join("/tmp/dash", "hls", "12345", "720p", "seg_000042.ts")
	if result != expected {
		t.Errorf("GetHLSSegmentPath() = %s, want %s", result, expected)
	}
}

// TestPathConsistency verifies that path functions are consistent with each other.
func TestPathConsistency(t *testing.T) {
	outputDir := "/test/output"
	mediaID := int64(12345)
	quality := "720p"
	segmentNum := 42

	outputPath := GetHLSOutputPath(outputDir, mediaID, quality)
	manifestPath := GetHLSManifestPath(outputDir, mediaID, quality)
	segmentPath := GetHLSSegmentPath(outputDir, mediaID, quality, segmentNum)

	// Manifest path should be inside output path
	expectedManifest := filepath.Join(outputPath, "playlist.m3u8")
	if manifestPath != expectedManifest {
		t.Errorf("Manifest path %s does not match expected %s", manifestPath, expectedManifest)
	}

	// Segment path should be inside output path
	expectedSegment := filepath.Join(outputPath, SegmentFilename(segmentNum))
	if segmentPath != expectedSegment {
		t.Errorf("Segment path %s does not match expected %s", segmentPath, expectedSegment)
	}

	// Both should share the same directory
	manifestDir := filepath.Dir(manifestPath)
	segmentDir := filepath.Dir(segmentPath)
	if manifestDir != segmentDir {
		t.Errorf("Manifest dir %s != segment dir %s", manifestDir, segmentDir)
	}
}
