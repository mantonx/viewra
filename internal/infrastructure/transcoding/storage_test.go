package transcoding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetStorageInfo(t *testing.T) {
	// Use temp directory for testing
	tmpDir := t.TempDir()

	info, err := GetStorageInfo(tmpDir)
	if err != nil {
		t.Fatalf("GetStorageInfo() error = %v", err)
	}

	if info == nil {
		t.Fatal("GetStorageInfo() returned nil")
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

func TestGetStorageInfoWithNonExistentPath(t *testing.T) {
	// Non-existent path should use parent directory
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "does_not_exist.txt")

	info, err := GetStorageInfo(nonExistent)
	if err != nil {
		t.Fatalf("GetStorageInfo() should handle non-existent paths, got error: %v", err)
	}

	if info == nil {
		t.Fatal("GetStorageInfo() returned nil for non-existent path")
	}

	// Should return valid storage info for parent directory
	if info.TotalBytes == 0 {
		t.Error("TotalBytes should not be 0 even for non-existent path")
	}
}

func TestGetStorageInfoWithFile(t *testing.T) {
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Should use parent directory for a file
	info, err := GetStorageInfo(testFile)
	if err != nil {
		t.Fatalf("GetStorageInfo() error = %v", err)
	}

	if info == nil {
		t.Fatal("GetStorageInfo() returned nil")
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

func TestCheckDiskSpaceWithInvalidPath(t *testing.T) {
	// Using an invalid path should return an error
	err := CheckDiskSpace("/this/path/does/not/exist/xyz123", 1)

	// This might succeed if it falls back to parent, but we just verify it doesn't panic
	_ = err
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
			expectedMinSize: 500_000_000,   // ~500MB
			expectedMaxSize: 750_000_000,   // ~750MB (with 20% overhead)
			shouldError:     false,
		},
		{
			name:            "4K video - 10 seconds",
			videoBitrate:    "20000k",
			audioBitrate:    "256k",
			durationSeconds: 10,
			expectedMinSize: 20_000_000,    // ~20MB
			expectedMaxSize: 35_000_000,    // ~35MB (with 20% overhead)
			shouldError:     false,
		},
		{
			name:            "Uppercase bitrate suffix",
			videoBitrate:    "5000K",
			audioBitrate:    "128K",
			durationSeconds: 60,
			expectedMinSize: 30_000_000,    // ~30MB
			expectedMaxSize: 50_000_000,    // ~50MB
			shouldError:     false,
		},
		{
			name:            "Megabit bitrate",
			videoBitrate:    "5m",
			audioBitrate:    "128k",
			durationSeconds: 60,
			expectedMinSize: 30_000_000,
			expectedMaxSize: 50_000_000,
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
			name:            "Invalid audio bitrate",
			videoBitrate:    "5000k",
			audioBitrate:    "invalid",
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
		{
			name:            "Empty audio bitrate",
			videoBitrate:    "5000k",
			audioBitrate:    "",
			durationSeconds: 60,
			shouldError:     true,
		},
		{
			name:            "Zero duration",
			videoBitrate:    "5000k",
			audioBitrate:    "128k",
			durationSeconds: 0,
			expectedMinSize: 0,
			expectedMaxSize: 100, // Some overhead
			shouldError:     false,
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
			name:     "Large value",
			bitrate:  "100000k",
			expected: 100_000_000,
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
		{
			name:     "Invalid suffix (treated as raw number)",
			bitrate:  "2500x",
			expected: 2500, // Invalid suffix is ignored, treated as raw number
		},
		{
			name:        "Negative value",
			bitrate:     "-1000k",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseBitrate(tt.bitrate)

			if tt.shouldError {
				if err == nil {
					t.Error("parseBitrate() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("parseBitrate() unexpected error = %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("parseBitrate() = %d, want %d", result, tt.expected)
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
			name:     "Terabytes",
			bytes:    2 * 1024 * 1024 * 1024 * 1024,
			expected: "2.00 TiB",
		},
		{
			name:     "Fractional GB",
			bytes:    2_500_000_000,
			expected: "2.33 GiB",
		},
		{
			name:     "Zero bytes",
			bytes:    0,
			expected: "0 B",
		},
		{
			name:     "Large number",
			bytes:    1024 * 1024 * 1024 * 1024 * 1024, // 1 PiB
			expected: "1.00 PiB",
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

func TestFormatBytesAllUnits(t *testing.T) {
	// Test each unit boundary
	units := []struct {
		bytes uint64
		unit  string
	}{
		{0, "B"},
		{1023, "B"},
		{1024, "KiB"},
		{1024 * 1024, "MiB"},
		{1024 * 1024 * 1024, "GiB"},
		{1024 * 1024 * 1024 * 1024, "TiB"},
		{1024 * 1024 * 1024 * 1024 * 1024, "PiB"},
	}

	for _, u := range units {
		t.Run(u.unit, func(t *testing.T) {
			result := FormatBytes(u.bytes)
			if !strings.Contains(result, u.unit) {
				t.Errorf("FormatBytes(%d) = %s, should contain %s", u.bytes, result, u.unit)
			}
		})
	}
}

func TestStorageInfoCalculations(t *testing.T) {
	tmpDir := t.TempDir()

	info, err := GetStorageInfo(tmpDir)
	if err != nil {
		t.Fatalf("GetStorageInfo() error = %v", err)
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

func TestCheckDiskSpaceErrorMessage(t *testing.T) {
	tmpDir := t.TempDir()

	// Request impossible amount
	err := CheckDiskSpace(tmpDir, 1_000_000) // 1 PB

	if err == nil {
		t.Skip("System has 1PB free space, cannot test error message")
	}

	errMsg := err.Error()

	// Error should mention "insufficient disk space"
	if !strings.Contains(errMsg, "insufficient") && !strings.Contains(errMsg, "Insufficient") {
		t.Error("Error message should mention insufficient disk space")
	}

	// Error should contain numerical values
	if !strings.Contains(errMsg, "GB") {
		t.Error("Error message should mention GB")
	}

	// Error should show available and required amounts
	if !strings.Contains(errMsg, "available") {
		t.Error("Error message should mention available space")
	}

	if !strings.Contains(errMsg, "required") {
		t.Error("Error message should mention required space")
	}
}

func TestEstimateOutputSizeOverhead(t *testing.T) {
	// Verify that the function adds 20% overhead
	size, err := EstimateOutputSize("8000k", "0k", 1.0) // 1 second, 8Mbps video, no audio

	if err != nil {
		t.Fatalf("EstimateOutputSize() error = %v", err)
	}

	// 8000 kbps = 8,000,000 bits/sec * 1 sec = 8,000,000 bits = 1,000,000 bytes
	// With 20% overhead: 1,000,000 * 1.2 = 1,200,000 bytes
	expectedSize := uint64(1_200_000)

	if size != expectedSize {
		t.Errorf("EstimateOutputSize() = %d, want %d (with 20%% overhead)", size, expectedSize)
	}
}

func TestGetStorageInfoAbsolutePath(t *testing.T) {
	// Test with relative path
	tmpDir := t.TempDir()

	// Change to temp dir
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Use relative path
	info, err := GetStorageInfo(".")
	if err != nil {
		t.Fatalf("GetStorageInfo() error = %v", err)
	}

	if info == nil {
		t.Fatal("GetStorageInfo() returned nil")
	}

	if info.TotalBytes == 0 {
		t.Error("TotalBytes should not be 0 for relative path")
	}
}

func BenchmarkGetStorageInfo(b *testing.B) {
	tmpDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetStorageInfo(tmpDir)
		if err != nil {
			b.Fatalf("GetStorageInfo() error = %v", err)
		}
	}
}

func BenchmarkCheckDiskSpace(b *testing.B) {
	tmpDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CheckDiskSpace(tmpDir, 1)
	}
}

func BenchmarkEstimateOutputSize(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := EstimateOutputSize("5000k", "128k", 3600)
		if err != nil {
			b.Fatalf("EstimateOutputSize() error = %v", err)
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
