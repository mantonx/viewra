package validation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"
)

func TestValidateAndSanitizePath(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name             string
		path             string
		allowedBasePaths []string
		shouldError      bool
		errorContains    string
	}{
		{"Valid absolute path", tmpDir + "/test.mp4", []string{tmpDir}, false, ""},
		{"Valid relative path", "./test.mp4", nil, false, ""},
		{"Path traversal", tmpDir + "/../../../etc/passwd", []string{tmpDir}, true, "outside allowed directories"},
		{"Null byte", "/tmp/test\x00.mp4", nil, true, "null bytes"},
		{"Empty path", "", nil, true, "path is empty"},
		{"Path outside allowed dirs", "/etc/passwd", []string{tmpDir}, true, "outside allowed directories"},
		{"No base path restrictions", tmpDir + "/video.mkv", nil, false, ""},
		{"Redundant separators", tmpDir + "///test//video.mp4", []string{tmpDir}, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateAndSanitizePath(tt.path, tt.allowedBasePaths)
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("error = %v, want to contain %v", err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error = %v", err)
					return
				}
				if result == "" {
					t.Errorf("returned empty path")
				}
			}
		})
	}
}

func TestValidateInputFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	validFile := filepath.Join(tmpDir, "valid.mp4")
	if err := os.WriteFile(validFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	emptyFile := filepath.Join(tmpDir, "empty.mp4")
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}

	tests := []struct {
		name          string
		path          string
		shouldError   bool
		errorContains string
	}{
		{"Valid file", validFile, false, ""},
		{"Non-existent file", filepath.Join(tmpDir, "nonexistent.mp4"), true, "does not exist"},
		{"Directory instead of file", tmpDir, true, "is a directory"},
		{"Empty file", emptyFile, true, "is empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInputFile(tt.path)
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("error = %v, want to contain %v", err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error = %v", err)
				}
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Clean filename", "video.mp4", "video.mp4"},
		{"Path separator /", "../../../etc/passwd", "______etc_passwd"},
		{"Path separator \\", "..\\..\\windows\\system32", "____windows_system32"},
		{"Null byte", "test\x00.mp4", "test.mp4"},
		{"Dangerous characters", "test`$&|;<>(){}[].mp4", "test_____________.mp4"},
		{"Tildes", "~/secret/file.mp4", "__secret_file.mp4"},
		{"Complex dangerous", "$(rm -rf /)&whoami", "__rm -rf ___whoami"},
		{"Empty filename", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateAndSanitizePath_MultipleBasePaths(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	tests := []struct {
		name        string
		path        string
		basePaths   []string
		shouldError bool
	}{
		{"Path in first base", tmpDir1 + "/test.mp4", []string{tmpDir1, tmpDir2}, false},
		{"Path in second base", tmpDir2 + "/test.mp4", []string{tmpDir1, tmpDir2}, false},
		{"Path outside both bases", "/etc/passwd", []string{tmpDir1, tmpDir2}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateAndSanitizePath(tt.path, tt.basePaths)
			if tt.shouldError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateAndSanitizePath_InvalidBasePath(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp4")

	// Test with a base path that contains null bytes (will fail filepath.Abs)
	// The function should continue to check other base paths or return error
	_, err := ValidateAndSanitizePath(testFile, []string{"/invalid\x00path", tmpDir})
	// Should succeed because tmpDir is a valid base path
	if err != nil {
		t.Errorf("should have matched second base path, got error: %v", err)
	}

	// Test with only invalid base paths
	_, err = ValidateAndSanitizePath(testFile, []string{"/invalid\x00path"})
	// Should fail - path is outside invalid directory
	if err == nil {
		t.Errorf("expected error for path outside invalid base paths")
	}
}

func TestValidateInputFile_PermissionDenied(t *testing.T) {
	// Skip on non-Unix or if running as root
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	tmpDir := t.TempDir()

	// Create a file with no read permissions
	noReadFile := filepath.Join(tmpDir, "noread.mp4")
	if err := os.WriteFile(noReadFile, []byte("test content"), 0000); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer os.Chmod(noReadFile, 0644) // Restore permissions for cleanup

	// Note: os.Stat() can still read file metadata even without read permissions
	// This test verifies that we can stat the file (permission for stat != permission for read)
	err := ValidateInputFile(noReadFile)
	// Stat should still work - it only checks if file exists and is not empty
	if err != nil {
		// If we get an error, it should be about access, not about path validation
		if !contains(err.Error(), "cannot access") {
			t.Logf("unexpected error type: %v", err)
		}
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestValidateTranscodeRequest(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid test file
	validFile := filepath.Join(tmpDir, "valid.mp4")
	if err := os.WriteFile(validFile, []byte("test video content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create an empty file
	emptyFile := filepath.Join(tmpDir, "empty.mp4")
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}

	// Create output directory
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	testProfile := &profile.AdaptiveProfile{
		Width:        1920,
		Height:       1080,
		VideoBitrate: 5_000_000,
	}

	tests := []struct {
		name          string
		inputPath     string
		outputDir     string
		allowedPaths  []string
		shouldError   bool
		errorContains string
	}{
		{
			name:          "empty input path",
			inputPath:     "",
			outputDir:     outputDir,
			allowedPaths:  nil,
			shouldError:   true,
			errorContains: "invalid input path",
		},
		{
			name:          "null byte in input path",
			inputPath:     "/tmp/test\x00.mp4",
			outputDir:     outputDir,
			allowedPaths:  nil,
			shouldError:   true,
			errorContains: "invalid input path",
		},
		{
			name:          "input file does not exist",
			inputPath:     filepath.Join(tmpDir, "nonexistent.mp4"),
			outputDir:     outputDir,
			allowedPaths:  nil,
			shouldError:   true,
			errorContains: "input file validation failed",
		},
		{
			name:          "input is a directory",
			inputPath:     tmpDir,
			outputDir:     outputDir,
			allowedPaths:  nil,
			shouldError:   true,
			errorContains: "input file validation failed",
		},
		{
			name:          "input file is empty",
			inputPath:     emptyFile,
			outputDir:     outputDir,
			allowedPaths:  nil,
			shouldError:   true,
			errorContains: "input file validation failed",
		},
		{
			name:          "input outside allowed directories",
			inputPath:     "/etc/passwd",
			outputDir:     outputDir,
			allowedPaths:  []string{tmpDir},
			shouldError:   true,
			errorContains: "invalid input path",
		},
		{
			name:          "path traversal in input",
			inputPath:     filepath.Join(tmpDir, "..", "..", "etc", "passwd"),
			outputDir:     outputDir,
			allowedPaths:  []string{tmpDir},
			shouldError:   true,
			errorContains: "invalid input path",
		},
		{
			name:          "empty output path",
			inputPath:     validFile,
			outputDir:     "",
			allowedPaths:  nil,
			shouldError:   true,
			errorContains: "invalid output path",
		},
		{
			name:          "null byte in output path",
			inputPath:     validFile,
			outputDir:     "/tmp/output\x00dir",
			allowedPaths:  nil,
			shouldError:   true,
			errorContains: "invalid output path",
		},
		{
			name:          "valid input and output passes early validation",
			inputPath:     validFile,
			outputDir:     outputDir,
			allowedPaths:  nil,
			shouldError:   false,
			errorContains: "", // May fail on disk space or ffprobe, but not on path validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTranscodeRequest(tt.inputPath, tt.outputDir, testProfile, tt.allowedPaths)
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorContains)
					return
				}
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errorContains)
				}
			} else {
				// For valid paths, we might get errors from:
				// - disk space check (environment dependent)
				// - ffprobe (not installed or failing)
				// But we should NOT get path validation errors
				if err != nil {
					errStr := err.Error()
					// These are acceptable errors for valid paths
					if contains(errStr, "insufficient disk space") ||
						contains(errStr, "transcoding not needed") {
						// Expected in some environments
						return
					}
					// Path validation errors are NOT acceptable
					if contains(errStr, "invalid input path") ||
						contains(errStr, "invalid output path") ||
						contains(errStr, "input file validation failed") {
						t.Errorf("unexpected validation error: %v", err)
					}
				}
			}
		})
	}
}
