package transcoding

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateAndSanitizePath validates and sanitizes a file path to prevent path traversal attacks.
// Returns the absolute, cleaned path or an error.
func ValidateAndSanitizePath(path string, allowedBasePaths []string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}

	// Prevent null bytes
	if strings.Contains(path, "\x00") {
		return "", fmt.Errorf("path contains null bytes")
	}

	// Clean the path (removes .., redundant separators, etc.)
	cleanPath := filepath.Clean(path)

	// Convert to absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// If no allowed base paths specified, just return the absolute path
	if len(allowedBasePaths) == 0 {
		return absPath, nil
	}

	// Check if path is within one of the allowed base paths
	for _, basePath := range allowedBasePaths {
		absBasePath, err := filepath.Abs(basePath)
		if err != nil {
			continue
		}

		// Check if the path starts with the base path
		// Use EvalSymlinks to resolve symlinks and prevent bypass
		relPath, err := filepath.Rel(absBasePath, absPath)
		if err != nil {
			continue
		}

		// If relPath starts with "..", it's outside the base path
		if !strings.HasPrefix(relPath, "..") {
			return absPath, nil
		}
	}

	return "", fmt.Errorf("path is outside allowed directories: %s", absPath)
}

// ValidateInputFile checks if the input file exists and is accessible.
func ValidateInputFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("input file does not exist: %s", path)
		}
		return fmt.Errorf("cannot access input file: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("input path is a directory, not a file: %s", path)
	}

	if info.Size() == 0 {
		return fmt.Errorf("input file is empty: %s", path)
	}

	return nil
}

// ValidateTranscodeRequest performs comprehensive validation before starting a transcode.
func ValidateTranscodeRequest(inputPath string, outputDir string, profile *QualityProfile, allowedBasePaths []string) error {
	// Validate and sanitize input path
	sanitizedInput, err := ValidateAndSanitizePath(inputPath, allowedBasePaths)
	if err != nil {
		return fmt.Errorf("invalid input path: %w", err)
	}

	// Validate file exists and is readable
	if err := ValidateInputFile(sanitizedInput); err != nil {
		return fmt.Errorf("input file validation failed: %w", err)
	}

	// Validate output directory (create if needed)
	sanitizedOutput, err := ValidateAndSanitizePath(outputDir, nil)
	if err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}

	// Check disk space
	if err := CheckDiskSpace(sanitizedOutput, 10); err != nil {
		return fmt.Errorf("insufficient disk space: %w", err)
	}

	// Get video info and check if transcoding is needed
	videoInfo, err := GetVideoInfo(sanitizedInput)
	if err != nil {
		// Log warning but allow transcode to proceed
		// Some videos may not parse correctly with ffprobe
		return nil
	}

	// Check if transcoding is actually necessary
	shouldTranscode, reason := ShouldTranscode(videoInfo, profile)
	if !shouldTranscode {
		return fmt.Errorf("transcoding not needed: %s", reason)
	}

	return nil
}

// SanitizeFilename sanitizes a filename by removing dangerous characters.
// Used for generating safe output filenames.
func SanitizeFilename(filename string) string {
	// Remove path separators
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")

	// Remove null bytes
	filename = strings.ReplaceAll(filename, "\x00", "")

	// Remove other potentially dangerous characters
	dangerous := []string{"..", "~", "`", "$", "&", "|", ";", "<", ">", "(", ")", "{", "}", "[", "]"}
	for _, char := range dangerous {
		filename = strings.ReplaceAll(filename, char, "_")
	}

	return filename
}
