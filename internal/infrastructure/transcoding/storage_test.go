package transcoding

import (
	"strings"
	"testing"
)

// Tests for the re-exported storage functions from the root transcoding package.
// Comprehensive storage tests are in the storage subpackage.

func TestGetStorageInfo_ReExport(t *testing.T) {
	tmpDir := t.TempDir()

	info, err := GetStorageInfo(tmpDir)
	if err != nil {
		t.Fatalf("GetStorageInfo() error = %v", err)
	}

	if info == nil {
		t.Fatal("GetStorageInfo() returned nil")
	}

	if info.TotalBytes == 0 {
		t.Error("TotalBytes should not be 0")
	}

	if info.UsedPercent < 0 || info.UsedPercent > 100 {
		t.Errorf("UsedPercent = %f, want 0-100", info.UsedPercent)
	}
}

func TestCheckDiskSpace_ReExport(t *testing.T) {
	tmpDir := t.TempDir()

	// Should pass with 0 GB requirement
	err := CheckDiskSpace(tmpDir, 0)
	if err != nil {
		t.Errorf("CheckDiskSpace() unexpected error = %v", err)
	}

	// Should fail with unrealistic requirement
	err = CheckDiskSpace(tmpDir, 1000000) // 1 PB
	if err == nil {
		t.Error("CheckDiskSpace() expected error but got none")
	}
}

func TestEstimateOutputSize_ReExport(t *testing.T) {
	size, err := EstimateOutputSize("5000k", "128k", 3600)
	if err != nil {
		t.Fatalf("EstimateOutputSize() error = %v", err)
	}

	// Should be around 2-3 GB for 1 hour at 5Mbps
	if size < 2_000_000_000 || size > 3_000_000_000 {
		t.Errorf("EstimateOutputSize() = %d, want ~2-3GB", size)
	}
}

func TestFormatBytes_ReExport(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{512, "512 B"},
		{1024, "1.00 KiB"},
		{1024 * 1024 * 1024, "1.00 GiB"},
	}

	for _, tt := range tests {
		result := FormatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("FormatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestCheckDiskSpaceErrorMessage_ReExport(t *testing.T) {
	tmpDir := t.TempDir()

	err := CheckDiskSpace(tmpDir, 1_000_000) // 1 PB
	if err == nil {
		t.Skip("System has 1PB free space")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "GB") {
		t.Error("Error message should mention GB")
	}
}
