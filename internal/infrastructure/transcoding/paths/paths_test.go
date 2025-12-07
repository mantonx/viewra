package paths

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestGetHLSOutputPath(t *testing.T) {
	tests := []struct {
		name      string
		outputDir string
		mediaID   int64
		quality   string
		expected  string
	}{
		{
			name:      "Basic path",
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
		{
			name:      "Mixed case quality",
			outputDir: "/var/media",
			mediaID:   99999,
			quality:   "4K",
			expected:  filepath.Join("/var/media", "hls", "99999", "4k"),
		},
		{
			name:      "Relative output dir",
			outputDir: "./data/dash",
			mediaID:   1,
			quality:   "original",
			expected:  filepath.Join("./data/dash", "hls", "1", "original"),
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
	tests := []struct {
		name      string
		outputDir string
		mediaID   int64
		quality   string
		expected  string
	}{
		{
			name:      "Basic manifest path",
			outputDir: "/tmp/dash",
			mediaID:   12345,
			quality:   "720p",
			expected:  filepath.Join("/tmp/dash", "hls", "12345", "720p", "playlist.m3u8"),
		},
		{
			name:      "Quality normalized",
			outputDir: "/tmp/dash",
			mediaID:   12345,
			quality:   "1080P",
			expected:  filepath.Join("/tmp/dash", "hls", "12345", "1080p", "playlist.m3u8"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetHLSManifestPath(tt.outputDir, tt.mediaID, tt.quality)
			if result != tt.expected {
				t.Errorf("GetHLSManifestPath(%s, %d, %s) = %s, want %s",
					tt.outputDir, tt.mediaID, tt.quality, result, tt.expected)
			}
		})
	}
}

func TestGetHLSSegmentPath(t *testing.T) {
	// Mock segment filename function
	segmentFilename := func(n int) string {
		return fmt.Sprintf("seg_%06d.ts", n)
	}

	tests := []struct {
		name       string
		outputDir  string
		mediaID    int64
		quality    string
		segmentNum int
		expected   string
	}{
		{
			name:       "Segment 0",
			outputDir:  "/tmp/dash",
			mediaID:    12345,
			quality:    "720p",
			segmentNum: 0,
			expected:   filepath.Join("/tmp/dash", "hls", "12345", "720p", "seg_000000.ts"),
		},
		{
			name:       "Segment 123",
			outputDir:  "/tmp/dash",
			mediaID:    12345,
			quality:    "720p",
			segmentNum: 123,
			expected:   filepath.Join("/tmp/dash", "hls", "12345", "720p", "seg_000123.ts"),
		},
		{
			name:       "Quality normalized",
			outputDir:  "/var/media",
			mediaID:    999,
			quality:    "1080P",
			segmentNum: 50,
			expected:   filepath.Join("/var/media", "hls", "999", "1080p", "seg_000050.ts"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetHLSSegmentPath(tt.outputDir, tt.mediaID, tt.quality, tt.segmentNum, segmentFilename)
			if result != tt.expected {
				t.Errorf("GetHLSSegmentPath(%s, %d, %s, %d) = %s, want %s",
					tt.outputDir, tt.mediaID, tt.quality, tt.segmentNum, result, tt.expected)
			}
		})
	}
}

func TestPathConsistency(t *testing.T) {
	segmentFilename := func(n int) string {
		return fmt.Sprintf("seg_%06d.ts", n)
	}

	outputDir := "/test/output"
	mediaID := int64(12345)
	quality := "720p"
	segmentNum := 42

	outputPath := GetHLSOutputPath(outputDir, mediaID, quality)
	manifestPath := GetHLSManifestPath(outputDir, mediaID, quality)
	segmentPath := GetHLSSegmentPath(outputDir, mediaID, quality, segmentNum, segmentFilename)

	// Manifest path should be inside output path
	expectedManifest := filepath.Join(outputPath, "playlist.m3u8")
	if manifestPath != expectedManifest {
		t.Errorf("Manifest path %s does not match expected %s", manifestPath, expectedManifest)
	}

	// Segment path should be inside output path
	expectedSegment := filepath.Join(outputPath, segmentFilename(segmentNum))
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
