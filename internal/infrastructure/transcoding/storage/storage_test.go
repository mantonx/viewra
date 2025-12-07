package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetInfo(t *testing.T) {
	// Use temp directory for testing
	tmpDir := t.TempDir()

	info, err := GetInfo(tmpDir)
	if err != nil {
		t.Fatalf("GetInfo() error = %v", err)
	}

	if info == nil {
		t.Fatal("GetInfo() returned nil")
	}

	// Verify all fields are populated
	if info.TotalBytes == 0 {
		t.Error("TotalBytes should not be 0")
	}

	if info.FreeBytes == 0 {
		t.Error("FreeBytes should not be 0")
	}

	if info.AvailableBytes == 0 {
		t.Error("AvailableBytes should not be 0")
	}

	// UsedBytes should be less than TotalBytes
	if info.UsedBytes > info.TotalBytes {
		t.Errorf("UsedBytes (%d) should be <= TotalBytes (%d)", info.UsedBytes, info.TotalBytes)
	}

	// UsedPercent should be between 0 and 100
	if info.UsedPercent < 0 || info.UsedPercent > 100 {
		t.Errorf("UsedPercent = %f, want 0-100", info.UsedPercent)
	}

	// FreeBytes should be <= TotalBytes
	if info.FreeBytes > info.TotalBytes {
		t.Errorf("FreeBytes (%d) should be <= TotalBytes (%d)", info.FreeBytes, info.TotalBytes)
	}

	// AvailableBytes should be <= FreeBytes (usually equal on most systems)
	if info.AvailableBytes > info.FreeBytes {
		t.Errorf("AvailableBytes (%d) should be <= FreeBytes (%d)", info.AvailableBytes, info.FreeBytes)
	}
}

func TestGetInfoWithNonExistentPath(t *testing.T) {
	// Non-existent path should use parent directory
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "does_not_exist.txt")

	info, err := GetInfo(nonExistent)
	if err != nil {
		t.Fatalf("GetInfo() should handle non-existent paths, got error: %v", err)
	}

	if info == nil {
		t.Fatal("GetInfo() returned nil for non-existent path")
	}

	// Should return valid storage info for parent directory
	if info.TotalBytes == 0 {
		t.Error("TotalBytes should not be 0 even for non-existent path")
	}
}

func TestGetInfoWithFile(t *testing.T) {
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Should use parent directory for a file
	info, err := GetInfo(testFile)
	if err != nil {
		t.Fatalf("GetInfo() error = %v", err)
	}

	if info == nil {
		t.Fatal("GetInfo() returned nil")
	}

	if info.TotalBytes == 0 {
		t.Error("TotalBytes should not be 0")
	}
}

func TestCheckDiskSpace(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		path        string
		minGB       int64
		shouldError bool
	}{
		{
			name:        "0 GB required - should pass",
			path:        tmpDir,
			minGB:       0,
			shouldError: false,
		},
		{
			name:        "1 GB required - likely to pass on test systems",
			path:        tmpDir,
			minGB:       1,
			shouldError: false,
		},
		{
			name:        "Extremely high requirement - should fail",
			path:        tmpDir,
			minGB:       1000000, // 1 PB - unrealistic
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckDiskSpace(tt.path, tt.minGB)

			if tt.shouldError && err == nil {
				t.Error("CheckDiskSpace() expected error but got none")
			}

			if !tt.shouldError && err != nil {
				t.Errorf("CheckDiskSpace() unexpected error = %v", err)
			}

			// If error is returned, it should contain helpful information
			if err != nil {
				errMsg := err.Error()
				if errMsg == "" {
					t.Error("Error message should not be empty")
				}
				// Should mention GB in the error
				if len(errMsg) > 0 && !strings.Contains(errMsg, "GB") {
					t.Error("Error message should mention GB units")
				}
			}
		})
	}
}

func TestEstimateOutputSize(t *testing.T) {
	tests := []struct {
		name            string
		videoBitrate    string
		audioBitrate    string
		durationSeconds float64
		expectedMinSize uint64 // Minimum expected size in bytes
		expectedMaxSize uint64 // Maximum expected size in bytes
		shouldError     bool
	}{
		{
			name:            "1080p video - 1 hour",
			videoBitrate:    "5000k",
			audioBitrate:    "128k",
			durationSeconds: 3600,
			expectedMinSize: 2_000_000_000, // ~2GB
			expectedMaxSize: 3_000_000_000, // ~3GB
			shouldError:     false,
		},
		{
			name:            "720p video - 30 minutes",
			videoBitrate:    "2500k",
			audioBitrate:    "128k",
			durationSeconds: 1800,
			expectedMinSize: 500_000_000, // ~500MB
			expectedMaxSize: 750_000_000, // ~750MB (with 20% overhead)
			shouldError:     false,
		},
		{
			name:            "Invalid video bitrate",
			videoBitrate:    "invalid",
			audioBitrate:    "128k",
			durationSeconds: 60,
			shouldError:     true,
		},
		{
			name:            "Empty video bitrate",
			videoBitrate:    "",
			audioBitrate:    "128k",
			durationSeconds: 60,
			shouldError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, err := EstimateOutputSize(tt.videoBitrate, tt.audioBitrate, tt.durationSeconds)

			if tt.shouldError {
				if err == nil {
					t.Error("EstimateOutputSize() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("EstimateOutputSize() unexpected error = %v", err)
				return
			}

			if size < tt.expectedMinSize {
				t.Errorf("EstimateOutputSize() = %d bytes, want >= %d bytes", size, tt.expectedMinSize)
			}

			if size > tt.expectedMaxSize {
				t.Errorf("EstimateOutputSize() = %d bytes, want <= %d bytes", size, tt.expectedMaxSize)
			}
		})
	}
}

func TestParseBitrate(t *testing.T) {
	tests := []struct {
		name        string
		bitrate     string
		expected    uint64
		shouldError bool
	}{
		{
			name:     "Kilobit lowercase",
			bitrate:  "2500k",
			expected: 2_500_000,
		},
		{
			name:     "Kilobit uppercase",
			bitrate:  "2500K",
			expected: 2_500_000,
		},
		{
			name:     "Megabit lowercase",
			bitrate:  "5m",
			expected: 5_000_000,
		},
		{
			name:     "Megabit uppercase",
			bitrate:  "5M",
			expected: 5_000_000,
		},
		{
			name:     "No suffix (raw bits per second)",
			bitrate:  "128000",
			expected: 128_000,
		},
		{
			name:        "Empty string",
			bitrate:     "",
			shouldError: true,
		},
		{
			name:        "Invalid characters",
			bitrate:     "abc",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseBitrate(tt.bitrate)

			if tt.shouldError {
				if err == nil {
					t.Error("ParseBitrate() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseBitrate() unexpected error = %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("ParseBitrate() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    uint64
		expected string
	}{
		{
			name:     "Bytes",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "Kilobytes",
			bytes:    1024,
			expected: "1.00 KiB",
		},
		{
			name:     "Megabytes",
			bytes:    1024 * 1024,
			expected: "1.00 MiB",
		},
		{
			name:     "Gigabytes",
			bytes:    5 * 1024 * 1024 * 1024,
			expected: "5.00 GiB",
		},
		{
			name:     "Zero bytes",
			bytes:    0,
			expected: "0 B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBytes(tt.bytes)

			if result != tt.expected {
				t.Errorf("FormatBytes() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestStorageInfoCalculations(t *testing.T) {
	tmpDir := t.TempDir()

	info, err := GetInfo(tmpDir)
	if err != nil {
		t.Fatalf("GetInfo() error = %v", err)
	}

	// Verify the relationship: Total = Used + Free
	// Allow small difference due to rounding
	calculatedUsed := info.TotalBytes - info.FreeBytes
	diff := int64(info.UsedBytes) - int64(calculatedUsed)
	if diff < 0 {
		diff = -diff
	}

	// Difference should be minimal (within 1% or 1GB, whichever is larger)
	tolerance := info.TotalBytes / 100 // 1%
	if tolerance < 1024*1024*1024 {
		tolerance = 1024 * 1024 * 1024 // 1GB minimum tolerance
	}

	if uint64(diff) > tolerance {
		t.Errorf("Used+Free != Total: UsedBytes=%d, calculated=%d, diff=%d",
			info.UsedBytes, calculatedUsed, diff)
	}

	// Verify UsedPercent calculation
	expectedPercent := float64(info.UsedBytes) / float64(info.TotalBytes) * 100
	percentDiff := info.UsedPercent - expectedPercent
	if percentDiff < 0 {
		percentDiff = -percentDiff
	}

	if percentDiff > 0.1 { // Within 0.1%
		t.Errorf("UsedPercent calculation incorrect: got %f, expected %f",
			info.UsedPercent, expectedPercent)
	}
}

func BenchmarkGetInfo(b *testing.B) {
	tmpDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetInfo(tmpDir)
		if err != nil {
			b.Fatalf("GetInfo() error = %v", err)
		}
	}
}

func BenchmarkFormatBytes(b *testing.B) {
	sizes := []uint64{
		512,
		1024 * 1024,
		1024 * 1024 * 1024,
		1024 * 1024 * 1024 * 1024,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, size := range sizes {
			_ = FormatBytes(size)
		}
	}
}
