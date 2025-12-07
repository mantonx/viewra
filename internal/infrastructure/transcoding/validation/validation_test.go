package validation

import (
	"os"
	"path/filepath"
	"testing"
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

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
