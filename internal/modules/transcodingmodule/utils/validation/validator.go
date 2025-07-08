// Package validation provides output validation utilities for transcoding operations.
package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Result represents the result of a validation check
type Result struct {
	Valid    bool
	Issues   []string
	Warnings []string
}

// IsValid returns true if validation passed
func (r *Result) IsValid() bool {
	return r.Valid
}

// GetIssues returns all validation issues
func (r *Result) GetIssues() []string {
	return r.Issues
}

// ValidatePlaybackOutput validates DASH/HLS output for playback
func ValidatePlaybackOutput(outputDir string) *Result {
	result := &Result{
		Valid:    true,
		Issues:   []string{},
		Warnings: []string{},
	}

	// Check if directory exists
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		result.Valid = false
		result.Issues = append(result.Issues, "Output directory does not exist")
		return result
	}

	// List all files
	files, err := os.ReadDir(outputDir)
	if err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("Failed to read directory: %v", err))
		return result
	}

	// Check for manifest files
	hasManifest := false
	hasSegments := false
	manifestType := ""

	for _, file := range files {
		name := file.Name()

		// Check for DASH manifest
		if strings.HasSuffix(name, ".mpd") {
			hasManifest = true
			manifestType = "dash"
			// Validate manifest size
			info, err := file.Info()
			if err == nil && info.Size() < 100 {
				result.Issues = append(result.Issues, "DASH manifest file is too small")
				result.Valid = false
			}
		}

		// Check for HLS manifest
		if strings.HasSuffix(name, ".m3u8") {
			hasManifest = true
			manifestType = "hls"
			// Validate manifest size
			info, err := file.Info()
			if err == nil && info.Size() < 50 {
				result.Issues = append(result.Issues, "HLS manifest file is too small")
				result.Valid = false
			}
		}

		// Check for segments
		if strings.HasSuffix(name, ".m4s") || strings.HasSuffix(name, ".ts") {
			hasSegments = true
		}
	}

	// Validate based on type
	if !hasManifest {
		result.Valid = false
		result.Issues = append(result.Issues, "No manifest file found (expected .mpd or .m3u8)")
	}

	if !hasSegments {
		result.Valid = false
		result.Issues = append(result.Issues, "No segment files found")
	}

	// Additional validation based on manifest type
	if manifestType == "dash" {
		validateDashOutput(outputDir, files, result)
	} else if manifestType == "hls" {
		validateHLSOutput(outputDir, files, result)
	}

	return result
}

// validateDashOutput performs DASH-specific validation
func validateDashOutput(outputDir string, files []os.DirEntry, result *Result) {
	// Check for init segments
	hasInitSegment := false
	segmentCount := 0

	for _, file := range files {
		name := file.Name()
		if strings.Contains(name, "init-") && strings.HasSuffix(name, ".m4s") {
			hasInitSegment = true
		}
		if strings.Contains(name, "chunk-") && strings.HasSuffix(name, ".m4s") {
			segmentCount++
		}
	}

	if !hasInitSegment {
		result.Issues = append(result.Issues, "No DASH init segment found")
		result.Valid = false
	}

	if segmentCount < 2 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Only %d DASH segments found, expected more", segmentCount))
	}
}

// validateHLSOutput performs HLS-specific validation
func validateHLSOutput(outputDir string, files []os.DirEntry, result *Result) {
	// Count TS segments
	segmentCount := 0

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".ts") {
			segmentCount++
		}
	}

	if segmentCount < 2 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Only %d HLS segments found, expected more", segmentCount))
	}
}

// GenerateValidationReport writes a validation report to a file
func GenerateValidationReport(result *Result, reportPath string) error {
	var content strings.Builder

	content.WriteString("Transcoding Output Validation Report\n")
	content.WriteString("===================================\n\n")

	if result.Valid {
		content.WriteString("Status: PASSED\n\n")
	} else {
		content.WriteString("Status: FAILED\n\n")
	}

	if len(result.Issues) > 0 {
		content.WriteString("Issues:\n")
		for _, issue := range result.Issues {
			content.WriteString(fmt.Sprintf("- %s\n", issue))
		}
		content.WriteString("\n")
	}

	if len(result.Warnings) > 0 {
		content.WriteString("Warnings:\n")
		for _, warning := range result.Warnings {
			content.WriteString(fmt.Sprintf("- %s\n", warning))
		}
		content.WriteString("\n")
	}

	// Write report
	return os.WriteFile(reportPath, []byte(content.String()), 0644)
}

// ValidateMP4Output validates MP4 output files
func ValidateMP4Output(outputPath string) *Result {
	result := &Result{
		Valid:    true,
		Issues:   []string{},
		Warnings: []string{},
	}

	// Check if file exists
	info, err := os.Stat(outputPath)
	if err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("Output file not found: %v", err))
		return result
	}

	// Check file size
	if info.Size() < 1024 {
		result.Valid = false
		result.Issues = append(result.Issues, "Output file is too small (less than 1KB)")
	}

	// Check file extension
	if !strings.HasSuffix(outputPath, ".mp4") {
		result.Warnings = append(result.Warnings, "Output file does not have .mp4 extension")
	}

	return result
}

// ValidateDirectory ensures a directory is suitable for transcoding output
func ValidateDirectory(dir string) error {
	// Check if directory exists
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// Try to create it
			return os.MkdirAll(dir, 0755)
		}
		return err
	}

	// Check if it's actually a directory
	if !info.IsDir() {
		return fmt.Errorf("path exists but is not a directory: %s", dir)
	}

	// Check if we can write to it
	testFile := filepath.Join(dir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("directory is not writable: %w", err)
	}
	os.Remove(testFile)

	return nil
}
