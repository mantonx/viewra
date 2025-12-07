package transcoding

import (
	"path/filepath"
	"testing"
)

func TestSegmentNumber(t *testing.T) {
	tests := []struct {
		name              string
		timestampSeconds  float64
		expectedSegment   int
	}{
		{
			name:             "Zero timestamp",
			timestampSeconds: 0,
			expectedSegment:  0,
		},
		{
			name:             "First segment boundary",
			timestampSeconds: 1.99,
			expectedSegment:  0,
		},
		{
			name:             "Second segment start",
			timestampSeconds: 2.0,
			expectedSegment:  1,
		},
		{
			name:             "Second segment middle",
			timestampSeconds: 3.5,
			expectedSegment:  1,
		},
		{
			name:             "Example from comment: 37 seconds",
			timestampSeconds: 37,
			expectedSegment:  18,
		},
		{
			name:             "Large timestamp",
			timestampSeconds: 3600.0,
			expectedSegment:  1800,
		},
		{
			name:             "Fractional second",
			timestampSeconds: 5.25,
			expectedSegment:  2,
		},
		{
			name:             "Just before segment boundary",
			timestampSeconds: 11.99,
			expectedSegment:  5,
		},
		{
			name:             "Exactly on segment boundary",
			timestampSeconds: 12.0,
			expectedSegment:  6,
		},
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
		name          string
		segmentNumber int
		expectedTime  float64
	}{
		{
			name:          "Segment 0",
			segmentNumber: 0,
			expectedTime:  0.0,
		},
		{
			name:          "Segment 1",
			segmentNumber: 1,
			expectedTime:  2.0,
		},
		{
			name:          "Segment 6",
			segmentNumber: 6,
			expectedTime:  12.0,
		},
		{
			name:          "Segment 10",
			segmentNumber: 10,
			expectedTime:  20.0,
		},
		{
			name:          "Segment 100",
			segmentNumber: 100,
			expectedTime:  200.0,
		},
		{
			name:          "Large segment number",
			segmentNumber: 1800,
			expectedTime:  3600.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
		name          string
		segmentNumber int
		expectedTime  float64
	}{
		{
			name:          "Segment 0 ends at 2",
			segmentNumber: 0,
			expectedTime:  2.0,
		},
		{
			name:          "Segment 1 ends at 4",
			segmentNumber: 1,
			expectedTime:  4.0,
		},
		{
			name:          "Segment 6 ends at 14",
			segmentNumber: 6,
			expectedTime:  14.0,
		},
		{
			name:          "Segment 10 ends at 22",
			segmentNumber: 10,
			expectedTime:  22.0,
		},
		{
			name:          "Segment 100 ends at 202",
			segmentNumber: 100,
			expectedTime:  202.0,
		},
		{
			name:          "Large segment",
			segmentNumber: 1800,
			expectedTime:  3602.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SegmentEndTime(tt.segmentNumber)
			if result != tt.expectedTime {
				t.Errorf("SegmentEndTime(%d) = %f, want %f",
					tt.segmentNumber, result, tt.expectedTime)
			}
		})
	}
}

func TestSegmentRange(t *testing.T) {
	tests := []struct {
		name            string
		startTime       float64
		duration        float64
		expectedStart   int
		expectedEnd     int
	}{
		{
			name:          "From start, short duration",
			startTime:     0,
			duration:      5.0,
			expectedStart: 0,
			expectedEnd:   2,
		},
		{
			name:          "Mid-segment start",
			startTime:     3.5,
			duration:      10.0,
			expectedStart: 1,
			expectedEnd:   6,
		},
		{
			name:          "Example from comment: 35 seconds for 20 duration",
			startTime:     35,
			duration:      20,
			expectedStart: 17,
			expectedEnd:   27,
		},
		{
			name:          "Single segment duration",
			startTime:     10.0,
			duration:      2.0,
			expectedStart: 5,
			expectedEnd:   6,
		},
		{
			name:          "Exact segment boundaries",
			startTime:     12.0,
			duration:      6.0,
			expectedStart: 6,
			expectedEnd:   9,
		},
		{
			name:          "Very short duration within segment",
			startTime:     5.5,
			duration:      0.5,
			expectedStart: 2,
			expectedEnd:   3,
		},
		{
			name:          "Zero duration",
			startTime:     10.0,
			duration:      0.0,
			expectedStart: 5,
			expectedEnd:   5,
		},
		{
			name:          "Large range",
			startTime:     100.0,
			duration:      200.0,
			expectedStart: 50,
			expectedEnd:   150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startSeg, endSeg := SegmentRange(tt.startTime, tt.duration)
			if startSeg != tt.expectedStart {
				t.Errorf("SegmentRange(%f, %f) start = %d, want %d",
					tt.startTime, tt.duration, startSeg, tt.expectedStart)
			}
			if endSeg != tt.expectedEnd {
				t.Errorf("SegmentRange(%f, %f) end = %d, want %d",
					tt.startTime, tt.duration, endSeg, tt.expectedEnd)
			}
		})
	}
}

func TestValidateSegmentNumber(t *testing.T) {
	tests := []struct {
		name            string
		segmentNumber   int
		videoDuration   float64
		expectedValid   bool
	}{
		{
			name:          "Negative segment number",
			segmentNumber: -1,
			videoDuration: 100.0,
			expectedValid: false,
		},
		{
			name:          "Zero segment for short video",
			segmentNumber: 0,
			videoDuration: 5.0,
			expectedValid: true,
		},
		{
			name:          "Valid middle segment",
			segmentNumber: 50,
			videoDuration: 200.0,
			expectedValid: true,
		},
		{
			name:          "Last valid segment",
			segmentNumber: 100,
			videoDuration: 200.0,
			expectedValid: true,
		},
		{
			name:          "Segment beyond video duration",
			segmentNumber: 101,
			videoDuration: 200.0,
			expectedValid: false,
		},
		{
			name:          "Far beyond duration",
			segmentNumber: 1000,
			videoDuration: 100.0,
			expectedValid: false,
		},
		{
			name:          "Exactly at boundary",
			segmentNumber: 3,
			videoDuration: 6.0,
			expectedValid: true,
		},
		{
			name:          "Just over boundary",
			segmentNumber: 4,
			videoDuration: 6.0,
			expectedValid: false,
		},
		{
			name:          "Zero duration video",
			segmentNumber: 0,
			videoDuration: 0.0,
			expectedValid: true,
		},
		{
			name:          "Segment 1 with zero duration video",
			segmentNumber: 1,
			videoDuration: 0.0,
			expectedValid: false,
		},
		{
			name:          "Very long video",
			segmentNumber: 1800,
			videoDuration: 3600.0,
			expectedValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSegmentNumber(tt.segmentNumber, tt.videoDuration)
			if result != tt.expectedValid {
				t.Errorf("ValidateSegmentNumber(%d, %f) = %v, want %v",
					tt.segmentNumber, tt.videoDuration, result, tt.expectedValid)
			}
		})
	}
}

func TestGetHLSOutputPath(t *testing.T) {
	tests := []struct {
		name       string
		outputDir  string
		mediaID    int64
		quality    string
		expected   string
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
		{
			name:      "Large media ID",
			outputDir: "/output",
			mediaID:   999999999,
			quality:   "480p",
			expected:  filepath.Join("/output", "hls", "999999999", "480p"),
		},
		{
			name:      "Quality with underscores",
			outputDir: "/media",
			mediaID:   500,
			quality:   "high_quality",
			expected:  filepath.Join("/media", "hls", "500", "high_quality"),
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
		name       string
		outputDir  string
		mediaID    int64
		quality    string
		expected   string
	}{
		{
			name:      "Basic manifest path",
			outputDir: "/tmp/dash",
			mediaID:   12345,
			quality:   "720p",
			expected:  filepath.Join("/tmp/dash", "hls", "12345", "720p", "playlist.m3u8"),
		},
		{
			name:      "Quality normalized in path",
			outputDir: "/tmp/dash",
			mediaID:   12345,
			quality:   "1080P",
			expected:  filepath.Join("/tmp/dash", "hls", "12345", "1080p", "playlist.m3u8"),
		},
		{
			name:      "Original quality",
			outputDir: "/var/media",
			mediaID:   999,
			quality:   "original",
			expected:  filepath.Join("/var/media", "hls", "999", "original", "playlist.m3u8"),
		},
		{
			name:      "Relative path",
			outputDir: "./data/dash",
			mediaID:   1,
			quality:   "480p",
			expected:  filepath.Join("./data/dash", "hls", "1", "480p", "playlist.m3u8"),
		},
		{
			name:      "Large media ID",
			outputDir: "/output",
			mediaID:   999999999,
			quality:   "4k",
			expected:  filepath.Join("/output", "hls", "999999999", "4k", "playlist.m3u8"),
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
			name:       "Segment 1",
			outputDir:  "/tmp/dash",
			mediaID:    12345,
			quality:    "720p",
			segmentNum: 1,
			expected:   filepath.Join("/tmp/dash", "hls", "12345", "720p", "seg_000001.ts"),
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
			name:       "Large segment number",
			outputDir:  "/tmp/dash",
			mediaID:    12345,
			quality:    "720p",
			segmentNum: 999999,
			expected:   filepath.Join("/tmp/dash", "hls", "12345", "720p", "seg_999999.ts"),
		},
		{
			name:       "Quality normalized",
			outputDir:  "/var/media",
			mediaID:    999,
			quality:    "1080P",
			segmentNum: 50,
			expected:   filepath.Join("/var/media", "hls", "999", "1080p", "seg_000050.ts"),
		},
		{
			name:       "Original quality with segment",
			outputDir:  "/output",
			mediaID:    1,
			quality:    "original",
			segmentNum: 10,
			expected:   filepath.Join("/output", "hls", "1", "original", "seg_000010.ts"),
		},
		{
			name:       "Relative path with segment",
			outputDir:  "./data/dash",
			mediaID:    100,
			quality:    "480p",
			segmentNum: 5,
			expected:   filepath.Join("./data/dash", "hls", "100", "480p", "seg_000005.ts"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetHLSSegmentPath(tt.outputDir, tt.mediaID, tt.quality, tt.segmentNum)
			if result != tt.expected {
				t.Errorf("GetHLSSegmentPath(%s, %d, %s, %d) = %s, want %s",
					tt.outputDir, tt.mediaID, tt.quality, tt.segmentNum, result, tt.expected)
			}
		})
	}
}

// TestSegmentTimingConsistency verifies that segment calculations are consistent.
// If we calculate a segment number from a time, the start time of that segment
// should be less than or equal to the original time.
func TestSegmentTimingConsistency(t *testing.T) {
	testTimes := []float64{0, 1.5, 2.0, 5.7, 10.0, 37.3, 100.5, 1000.0}

	for _, time := range testTimes {
		t.Run("", func(t *testing.T) {
			segNum := SegmentNumber(time)
			startTime := SegmentStartTime(segNum)
			endTime := SegmentEndTime(segNum)

			if startTime > time {
				t.Errorf("For time %f: segment %d start time %f is greater than input time",
					time, segNum, startTime)
			}

			if endTime < time {
				t.Errorf("For time %f: segment %d end time %f is less than input time",
					time, segNum, endTime)
			}

			if endTime-startTime != SegmentDuration {
				t.Errorf("For segment %d: duration %f != %d",
					segNum, endTime-startTime, SegmentDuration)
			}
		})
	}
}

// TestSegmentRangeConsistency verifies that SegmentRange covers the entire time range.
func TestSegmentRangeConsistency(t *testing.T) {
	tests := []struct {
		startTime float64
		duration  float64
	}{
		{0, 10},
		{5.5, 20.3},
		{100, 50},
		{37, 20},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			startSeg, endSeg := SegmentRange(tt.startTime, tt.duration)

			// Start segment should cover the start time
			segStart := SegmentStartTime(startSeg)
			segEnd := SegmentEndTime(startSeg)
			if segStart > tt.startTime || segEnd < tt.startTime {
				t.Errorf("Start segment %d (time %f-%f) does not cover start time %f",
					startSeg, segStart, segEnd, tt.startTime)
			}

			// End segment should cover the end time
			endTime := tt.startTime + tt.duration
			endSegStart := SegmentStartTime(endSeg)
			endSegEnd := SegmentEndTime(endSeg)
			if endSegStart > endTime || endSegEnd < endTime {
				t.Errorf("End segment %d (time %f-%f) does not cover end time %f",
					endSeg, endSegStart, endSegEnd, endTime)
			}
		})
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
	if manifestDir != outputPath {
		t.Errorf("Manifest dir %s != output path %s", manifestDir, outputPath)
	}
}
