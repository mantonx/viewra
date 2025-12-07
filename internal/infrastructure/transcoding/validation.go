package transcoding

import (
	"fmt"

	"github.com/mantonx/viewra/internal/infrastructure/transcoding/validation"
)

// ValidateAndSanitizePath validates and sanitizes a file path to prevent path traversal attacks.
// Re-exported from validation subpackage for backward compatibility.
func ValidateAndSanitizePath(path string, allowedBasePaths []string) (string, error) {
	return validation.ValidateAndSanitizePath(path, allowedBasePaths)
}

// ValidateInputFile checks if the input file exists and is accessible.
// Re-exported from validation subpackage for backward compatibility.
func ValidateInputFile(path string) error {
	return validation.ValidateInputFile(path)
}

// SanitizeFilename sanitizes a filename by removing dangerous characters.
// Re-exported from validation subpackage for backward compatibility.
func SanitizeFilename(filename string) string {
	return validation.SanitizeFilename(filename)
}

// ValidateTranscodeRequest performs comprehensive validation before starting a transcode.
// This function remains in the root package because it depends on other transcoding types.
func ValidateTranscodeRequest(inputPath string, outputDir string, profile *AdaptiveProfile, allowedBasePaths []string) error {
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
