package segment

import (
	"testing"
)

func TestNumber(t *testing.T) {
	tests := []struct {
		name      string
		timestamp float64
		expected  int
	}{
		{"Zero", 0, 0},
		{"First segment", 1.5, 0},
		{"Exact boundary", 2.0, 1},
		{"Mid segment", 3.5, 1},
		{"Later segment", 37.0, 18},
		{"Large timestamp", 3600.0, 1800},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Number(tt.timestamp)
			if result != tt.expected {
				t.Errorf("Number(%f) = %d, want %d", tt.timestamp, result, tt.expected)
			}
		})
	}
}

func TestStartTime(t *testing.T) {
	tests := []struct {
		name     string
		segment  int
		expected float64
	}{
		{"Segment 0", 0, 0},
		{"Segment 1", 1, 2},
		{"Segment 18", 18, 36},
		{"Segment 100", 100, 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StartTime(tt.segment)
			if result != tt.expected {
				t.Errorf("StartTime(%d) = %f, want %f", tt.segment, result, tt.expected)
			}
		})
	}
}

func TestEndTime(t *testing.T) {
	tests := []struct {
		name     string
		segment  int
		expected float64
	}{
		{"Segment 0", 0, 2},
		{"Segment 1", 1, 4},
		{"Segment 18", 18, 38},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EndTime(tt.segment)
			if result != tt.expected {
				t.Errorf("EndTime(%d) = %f, want %f", tt.segment, result, tt.expected)
			}
		})
	}
}

func TestFilename(t *testing.T) {
	tests := []struct {
		name     string
		segment  int
		expected string
	}{
		{"Segment 0", 0, "seg_000000.ts"},
		{"Segment 1", 1, "seg_000001.ts"},
		{"Segment 123", 123, "seg_000123.ts"},
		{"Segment 999999", 999999, "seg_999999.ts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Filename(tt.segment)
			if result != tt.expected {
				t.Errorf("Filename(%d) = %s, want %s", tt.segment, result, tt.expected)
			}
		})
	}
}

func TestParseNumber(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected int
	}{
		{"Valid segment 0", "seg_000000.ts", 0},
		{"Valid segment 1", "seg_000001.ts", 1},
		{"Valid segment 123", "seg_000123.ts", 123},
		{"Valid segment 999999", "seg_999999.ts", 999999},
		{"Invalid extension", "seg_000001.mp4", -1},
		{"Invalid prefix", "segment_000001.ts", -1},
		{"Invalid format", "seg_1.ts", -1},
		{"Empty string", "", -1},
		{"Random string", "random.txt", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseNumber(tt.filename)
			if result != tt.expected {
				t.Errorf("ParseNumber(%s) = %d, want %d", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestRange(t *testing.T) {
	tests := []struct {
		name          string
		startTime     float64
		duration      float64
		expectedStart int
		expectedEnd   int
	}{
		{"From start", 0, 10, 0, 5},
		{"Mid video", 35, 20, 17, 27},
		{"Single segment", 10, 1, 5, 5},
		{"Exact boundaries", 4, 4, 2, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := Range(tt.startTime, tt.duration)
			if start != tt.expectedStart || end != tt.expectedEnd {
				t.Errorf("Range(%f, %f) = (%d, %d), want (%d, %d)",
					tt.startTime, tt.duration, start, end, tt.expectedStart, tt.expectedEnd)
			}
		})
	}
}

func TestValidateNumber(t *testing.T) {
	tests := []struct {
		name       string
		segment    int
		duration   float64
		expected   bool
	}{
		{"Valid first segment", 0, 100, true},
		{"Valid mid segment", 25, 100, true},
		{"Valid last segment", 50, 100, true},
		{"Invalid negative", -1, 100, false},
		{"Invalid beyond duration", 60, 100, false},
		{"Edge case at boundary", 50, 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateNumber(tt.segment, tt.duration)
			if result != tt.expected {
				t.Errorf("ValidateNumber(%d, %f) = %v, want %v",
					tt.segment, tt.duration, result, tt.expected)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	if Duration != 2 {
		t.Errorf("Duration = %d, want 2", Duration)
	}

	if FilenameFormat != "seg_%06d.ts" {
		t.Errorf("FilenameFormat = %s, want seg_%%06d.ts", FilenameFormat)
	}

	if InitFilename != "init.mp4" {
		t.Errorf("InitFilename = %s, want init.mp4", InitFilename)
	}
}

func BenchmarkNumber(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Number(3600.5)
	}
}

func BenchmarkParseNumber(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseNumber("seg_001234.ts")
	}
}
