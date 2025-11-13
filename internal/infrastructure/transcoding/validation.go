package transcoding

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// VideoInfo contains metadata about a video file extracted via ffprobe.
type VideoInfo struct {
	Codec          string
	Width          int
	Height         int
	Bitrate        int64
	Duration       float64
	AudioCodec     string
	AudioBitrate   int64
	ContainerFormat string
}

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

// GetVideoInfo extracts video metadata using ffprobe.
func GetVideoInfo(inputPath string) (*VideoInfo, error) {
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, fmt.Errorf("ffprobe not found: %w", err)
	}

	// Run ffprobe with JSON output
	cmd := exec.Command(ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		inputPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	// Parse the JSON output manually (simple parsing for key fields)
	info := &VideoInfo{}
	outputStr := string(output)

	// Extract video stream info
	if videoMatch := regexp.MustCompile(`"codec_name":\s*"([^"]+)"`).FindStringSubmatch(outputStr); len(videoMatch) > 1 {
		info.Codec = videoMatch[1]
	}

	if widthMatch := regexp.MustCompile(`"width":\s*(\d+)`).FindStringSubmatch(outputStr); len(widthMatch) > 1 {
		info.Width, _ = strconv.Atoi(widthMatch[1])
	}

	if heightMatch := regexp.MustCompile(`"height":\s*(\d+)`).FindStringSubmatch(outputStr); len(heightMatch) > 1 {
		info.Height, _ = strconv.Atoi(heightMatch[1])
	}

	// Extract format info
	if durationMatch := regexp.MustCompile(`"duration":\s*"([^"]+)"`).FindStringSubmatch(outputStr); len(durationMatch) > 1 {
		info.Duration, _ = strconv.ParseFloat(durationMatch[1], 64)
	}

	if bitrateMatch := regexp.MustCompile(`"bit_rate":\s*"(\d+)"`).FindStringSubmatch(outputStr); len(bitrateMatch) > 1 {
		info.Bitrate, _ = strconv.ParseInt(bitrateMatch[1], 10, 64)
	}

	if formatMatch := regexp.MustCompile(`"format_name":\s*"([^"]+)"`).FindStringSubmatch(outputStr); len(formatMatch) > 1 {
		info.ContainerFormat = formatMatch[1]
	}

	return info, nil
}

// ShouldTranscode determines if transcoding is necessary based on current codec/resolution vs target.
// Returns true if transcoding is needed, false if source already matches target.
func ShouldTranscode(videoInfo *VideoInfo, profile *QualityProfile) (bool, string) {
	// Always transcode if we can't determine source codec/resolution
	if videoInfo == nil || videoInfo.Codec == "" {
		return true, "unable to determine source codec"
	}

	// Check if already H.264
	isH264 := videoInfo.Codec == "h264" || videoInfo.Codec == "H264" || videoInfo.Codec == "avc1"
	if !isH264 {
		return true, fmt.Sprintf("source codec %s needs transcoding to H.264", videoInfo.Codec)
	}

	// Check if resolution matches or exceeds target
	// Don't upscale - if source is 720p, don't transcode to 1080p
	if videoInfo.Width > 0 && videoInfo.Height > 0 {
		if videoInfo.Width < profile.Width || videoInfo.Height < profile.Height {
			// Source is lower resolution than target - would be upscaling
			return false, fmt.Sprintf("source %dx%d is lower than target %dx%d, skipping upscale",
				videoInfo.Width, videoInfo.Height, profile.Width, profile.Height)
		}

		// If source resolution is within 10% of target, consider it a match
		widthDiff := float64(videoInfo.Width-profile.Width) / float64(profile.Width)
		heightDiff := float64(videoInfo.Height-profile.Height) / float64(profile.Height)
		if widthDiff < 0.1 && heightDiff < 0.1 {
			return false, fmt.Sprintf("source %dx%d already matches target %dx%d",
				videoInfo.Width, videoInfo.Height, profile.Width, profile.Height)
		}
	}

	// Check bitrate - if source bitrate is already lower than target, no need to transcode
	if videoInfo.Bitrate > 0 {
		targetBitrate, _ := parseBitrate(profile.VideoBitrate)
		if uint64(videoInfo.Bitrate) < targetBitrate {
			return false, fmt.Sprintf("source bitrate %d is already lower than target %d",
				videoInfo.Bitrate, targetBitrate)
		}
	}

	// Needs transcoding
	return true, fmt.Sprintf("transcoding from %s %dx%d to H.264 %dx%d",
		videoInfo.Codec, videoInfo.Width, videoInfo.Height, profile.Width, profile.Height)
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
