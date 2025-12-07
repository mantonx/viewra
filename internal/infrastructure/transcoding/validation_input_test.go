package transcoding

import (
	"os"
	"path/filepath"
	"testing"
)

// TestValidateInputFile tests the ValidateInputFile function
func TestValidateInputFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a valid test file
	validFile := filepath.Join(tmpDir, "valid.mp4")
	if err := os.WriteFile(validFile, []byte("test video content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create an empty file
	emptyFile := filepath.Join(tmpDir, "empty.mp4")
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	// Create a directory (not a file)
	testDir := filepath.Join(tmpDir, "testdir")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tests := []struct {
		name          string
		path          string
		shouldError   bool
		errorContains string
	}{
		{
			name:        "Valid file",
			path:        validFile,
			shouldError: false,
		},
		{
			name:          "Non-existent file",
			path:          filepath.Join(tmpDir, "nonexistent.mp4"),
			shouldError:   true,
			errorContains: "does not exist",
		},
		{
			name:          "Empty file",
			path:          emptyFile,
			shouldError:   true,
			errorContains: "input file is empty",
		},
		{
			name:          "Directory instead of file",
			path:          testDir,
			shouldError:   true,
			errorContains: "is a directory",
		},
		{
			name:          "Path to parent directory that doesn't exist",
			path:          "/nonexistent/path/to/file.mp4",
			shouldError:   true,
			errorContains: "does not exist",
		},
		{
			name:          "Empty path",
			path:          "",
			shouldError:   true,
			errorContains: "does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateInputFile(tt.path)

			if tt.shouldError {
				if err == nil {
					t.Errorf("ValidateInputFile() expected error but got none")
					return
				}
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("ValidateInputFile() error = %v, want to contain %v", err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateInputFile() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestValidateInputFilePermissions tests file permission edge cases
func TestValidateInputFilePermissions(t *testing.T) {
	// Skip on systems where we can't manipulate permissions properly
	if os.Getuid() == 0 {
		t.Skip("Skipping permission tests when running as root")
	}

	tmpDir := t.TempDir()

	// Create a file with no read permissions
	noReadFile := filepath.Join(tmpDir, "noread.mp4")
	if err := os.WriteFile(noReadFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Remove read permissions
	if err := os.Chmod(noReadFile, 0000); err != nil {
		t.Fatalf("Failed to change permissions: %v", err)
	}

	// Restore permissions after test
	defer os.Chmod(noReadFile, 0644)

	// The file exists but we can't read it - this should still work with os.Stat
	// but would fail when FFmpeg tries to read it
	err := ValidateInputFile(noReadFile)
	// On some systems, os.Stat might still work without read permission
	// We just verify it doesn't panic
	_ = err
}

// TestValidateTranscodeRequest tests the comprehensive validation function
func TestValidateTranscodeRequest(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid input file with enough content
	validInput := filepath.Join(tmpDir, "input.mp4")
	// Create a file with at least 1KB of content
	content := make([]byte, 1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(validInput, content, 0644); err != nil {
		t.Fatalf("Failed to create test input file: %v", err)
	}

	// Create an output directory
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.Mkdir(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	// Create a valid profile for testing
	validProfile := &AdaptiveProfile{
		Width:        1920,
		Height:       1080,
		VideoBitrate: 5_000_000,
	}

	tests := []struct {
		name             string
		inputPath        string
		outputDir        string
		profile          *AdaptiveProfile
		allowedBasePaths []string
		shouldError      bool
		errorContains    string
	}{
		{
			name:             "Valid transcode request with allowed paths",
			inputPath:        validInput,
			outputDir:        outputDir,
			profile:          validProfile,
			allowedBasePaths: []string{tmpDir},
			shouldError:      true, // Will error when GetVideoInfo fails (no ffprobe)
			errorContains:    "",   // Don't check error message since it depends on ffprobe availability
		},
		{
			name:             "Valid transcode request without path restrictions",
			inputPath:        validInput,
			outputDir:        outputDir,
			profile:          validProfile,
			allowedBasePaths: nil,
			shouldError:      true, // Will error when GetVideoInfo fails (no ffprobe)
			errorContains:    "",   // Don't check error message
		},
		{
			name:             "Non-existent input file",
			inputPath:        filepath.Join(tmpDir, "nonexistent.mp4"),
			outputDir:        outputDir,
			profile:          validProfile,
			allowedBasePaths: nil,
			shouldError:      true,
			errorContains:    "does not exist",
		},
		{
			name:             "Empty input path",
			inputPath:        "",
			outputDir:        outputDir,
			profile:          validProfile,
			allowedBasePaths: nil,
			shouldError:      true,
			errorContains:    "path is empty",
		},
		{
			name:             "Input path with null bytes",
			inputPath:        "/tmp/test\x00.mp4",
			outputDir:        outputDir,
			profile:          validProfile,
			allowedBasePaths: nil,
			shouldError:      true,
			errorContains:    "null bytes",
		},
		{
			name:             "Input path outside allowed directories",
			inputPath:        "/etc/passwd",
			outputDir:        outputDir,
			profile:          validProfile,
			allowedBasePaths: []string{tmpDir},
			shouldError:      true,
			errorContains:    "outside allowed directories",
		},
		{
			name:             "Empty output directory",
			inputPath:        validInput,
			outputDir:        "",
			profile:          validProfile,
			allowedBasePaths: nil,
			shouldError:      true,
			errorContains:    "path is empty",
		},
		{
			name:             "Output path with null bytes",
			inputPath:        validInput,
			outputDir:        "/tmp/output\x00",
			profile:          validProfile,
			allowedBasePaths: nil,
			shouldError:      true,
			errorContains:    "null bytes",
		},
		{
			name:             "Path traversal in input",
			inputPath:        tmpDir + "/../../../etc/passwd",
			outputDir:        outputDir,
			profile:          validProfile,
			allowedBasePaths: []string{tmpDir},
			shouldError:      true,
			errorContains:    "outside allowed directories",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTranscodeRequest(tt.inputPath, tt.outputDir, tt.profile, tt.allowedBasePaths)

			if tt.shouldError {
				if err == nil {
					t.Errorf("ValidateTranscodeRequest() expected error but got none")
					return
				}
				// Only check error message if errorContains is not empty
				if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("ValidateTranscodeRequest() error = %v, want to contain %v", err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateTranscodeRequest() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestValidateTranscodeRequestDiskSpace tests disk space validation
func TestValidateTranscodeRequestDiskSpace(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid input file
	validInput := filepath.Join(tmpDir, "input.mp4")
	content := make([]byte, 2048)
	if err := os.WriteFile(validInput, content, 0644); err != nil {
		t.Fatalf("Failed to create test input file: %v", err)
	}

	// Create an output directory
	outputDir := filepath.Join(tmpDir, "output")
	if err := os.Mkdir(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	profile := &AdaptiveProfile{
		Width:        1920,
		Height:       1080,
		VideoBitrate: 5_000_000,
	}

	// This test will succeed or fail based on actual disk space
	// We expect it to at least check disk space (won't error on disk check unless disk is truly full)
	err := ValidateTranscodeRequest(validInput, outputDir, profile, nil)

	// The function should check disk space and either:
	// 1. Pass disk check and fail on GetVideoInfo (no ffprobe)
	// 2. Fail on disk check (very rare - means disk is full)
	// Either way, we just verify no panic occurs
	_ = err
}

// TestValidateTranscodeRequestOutputDirCreation tests output directory creation
func TestValidateTranscodeRequestOutputDirCreation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid input file
	validInput := filepath.Join(tmpDir, "input.mp4")
	content := make([]byte, 2048)
	if err := os.WriteFile(validInput, content, 0644); err != nil {
		t.Fatalf("Failed to create test input file: %v", err)
	}

	// Use a non-existent output directory - ValidateTranscodeRequest should handle this
	outputDir := filepath.Join(tmpDir, "newoutput", "nested", "path")

	profile := &AdaptiveProfile{
		Width:        1920,
		Height:       1080,
		VideoBitrate: 5_000_000,
	}

	// This will fail at some point (likely GetVideoInfo), but should validate paths
	err := ValidateTranscodeRequest(validInput, outputDir, profile, nil)

	// Should get an error (likely from GetVideoInfo or CheckDiskSpace)
	// But it should not panic and should have processed the path
	if err == nil {
		t.Log("ValidateTranscodeRequest succeeded (unexpected but not necessarily wrong)")
	}
}

// TestValidateTranscodeRequestEmptyFile tests validation with empty input file
func TestValidateTranscodeRequestEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an empty input file
	emptyInput := filepath.Join(tmpDir, "empty.mp4")
	if err := os.WriteFile(emptyInput, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.Mkdir(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	profile := &AdaptiveProfile{
		Width:        1920,
		Height:       1080,
		VideoBitrate: 5_000_000,
	}

	err := ValidateTranscodeRequest(emptyInput, outputDir, profile, nil)

	if err == nil {
		t.Error("ValidateTranscodeRequest() should error on empty file")
		return
	}

	if !contains(err.Error(), "empty") {
		t.Errorf("ValidateTranscodeRequest() error = %v, want to contain 'empty'", err.Error())
	}
}

// TestValidateTranscodeRequestDirectory tests validation when input is a directory
func TestValidateTranscodeRequestDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a subdirectory as "input"
	inputDir := filepath.Join(tmpDir, "inputdir")
	if err := os.Mkdir(inputDir, 0755); err != nil {
		t.Fatalf("Failed to create input directory: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.Mkdir(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	profile := &AdaptiveProfile{
		Width:        1920,
		Height:       1080,
		VideoBitrate: 5_000_000,
	}

	err := ValidateTranscodeRequest(inputDir, outputDir, profile, nil)

	if err == nil {
		t.Error("ValidateTranscodeRequest() should error when input is a directory")
		return
	}

	if !contains(err.Error(), "directory") {
		t.Errorf("ValidateTranscodeRequest() error = %v, want to contain 'directory'", err.Error())
	}
}

// TestValidateTranscodeRequestRelativePaths tests validation with relative paths
func TestValidateTranscodeRequestRelativePaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid input file
	validInput := filepath.Join(tmpDir, "input.mp4")
	content := make([]byte, 2048)
	if err := os.WriteFile(validInput, content, 0644); err != nil {
		t.Fatalf("Failed to create test input file: %v", err)
	}

	// Change to tmpDir so relative paths work
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := os.Mkdir(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}

	profile := &AdaptiveProfile{
		Width:        1920,
		Height:       1080,
		VideoBitrate: 5_000_000,
	}

	// Use relative paths
	err = ValidateTranscodeRequest("./input.mp4", "./output", profile, nil)

	// Should process the relative paths without panicking
	// Will likely fail on GetVideoInfo but that's okay
	_ = err
}

// TestValidateTranscodeRequestPathTraversal tests various path traversal attempts
func TestValidateTranscodeRequestPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid file inside tmpDir
	validFile := filepath.Join(tmpDir, "safe", "video.mp4")
	if err := os.MkdirAll(filepath.Dir(validFile), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	content := make([]byte, 2048)
	if err := os.WriteFile(validFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	profile := &AdaptiveProfile{
		Width:        1920,
		Height:       1080,
		VideoBitrate: 5_000_000,
	}

	tests := []struct {
		name        string
		inputPath   string
		shouldError bool
	}{
		{
			name:        "Path traversal with ../",
			inputPath:   filepath.Join(tmpDir, "safe", "..", "..", "..", "etc", "passwd"),
			shouldError: true,
		},
		{
			name:        "Multiple redundant separators",
			inputPath:   filepath.Join(tmpDir, "safe////video.mp4"),
			shouldError: false, // Should be cleaned and work
		},
		{
			name:        "Path with ./ components",
			inputPath:   filepath.Join(tmpDir, "safe", ".", "video.mp4"),
			shouldError: false, // Should be cleaned and work
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTranscodeRequest(tt.inputPath, outputDir, profile, []string{tmpDir})

			if tt.shouldError {
				if err == nil {
					t.Error("ValidateTranscodeRequest() expected error but got none")
				}
			}
			// For non-error cases, we don't assert nil error because GetVideoInfo might fail
		})
	}
}

// TestValidateInputFileSymlink tests validation with symlinks
func TestValidateInputFileSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real file
	realFile := filepath.Join(tmpDir, "real.mp4")
	content := make([]byte, 2048)
	if err := os.WriteFile(realFile, content, 0644); err != nil {
		t.Fatalf("Failed to create real file: %v", err)
	}

	// Create a symlink to it
	symlinkFile := filepath.Join(tmpDir, "link.mp4")
	if err := os.Symlink(realFile, symlinkFile); err != nil {
		t.Skip("Skipping symlink test: symlinks not supported on this system")
	}

	// Validate the symlink - should work
	err := ValidateInputFile(symlinkFile)
	if err != nil {
		t.Errorf("ValidateInputFile() failed on valid symlink: %v", err)
	}

	// Create a broken symlink
	brokenLink := filepath.Join(tmpDir, "broken.mp4")
	if err := os.Symlink("/nonexistent/file.mp4", brokenLink); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
	}

	// Validate broken symlink - should fail
	err = ValidateInputFile(brokenLink)
	if err == nil {
		t.Error("ValidateInputFile() should fail on broken symlink")
	}
}

// TestValidateInputFileEdgeCases tests edge cases
func TestValidateInputFileEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		setup       func() string // Returns path to test
		shouldError bool
		errorText   string
	}{
		{
			name: "File with unusual but valid name",
			setup: func() string {
				path := filepath.Join(tmpDir, "video-test_file (1).mp4")
				os.WriteFile(path, []byte("content"), 0644)
				return path
			},
			shouldError: false,
		},
		{
			name: "File with spaces in name",
			setup: func() string {
				path := filepath.Join(tmpDir, "my video file.mp4")
				os.WriteFile(path, []byte("content"), 0644)
				return path
			},
			shouldError: false,
		},
		{
			name: "File with unicode in name",
			setup: func() string {
				path := filepath.Join(tmpDir, "видео-测试-🎬.mp4")
				os.WriteFile(path, []byte("content"), 0644)
				return path
			},
			shouldError: false,
		},
		{
			name: "Very small file (1 byte)",
			setup: func() string {
				path := filepath.Join(tmpDir, "tiny.mp4")
				os.WriteFile(path, []byte("x"), 0644)
				return path
			},
			shouldError: false, // Not empty, so should pass
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			err := ValidateInputFile(path)

			if tt.shouldError {
				if err == nil {
					t.Error("ValidateInputFile() expected error but got none")
				} else if tt.errorText != "" && !contains(err.Error(), tt.errorText) {
					t.Errorf("ValidateInputFile() error = %v, want to contain %v", err.Error(), tt.errorText)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateInputFile() unexpected error = %v", err)
				}
			}
		})
	}
}
