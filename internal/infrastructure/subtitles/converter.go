package subtitles

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Converter handles subtitle extraction and conversion to WebVTT format.
type Converter struct {
	ffmpegPath  string
	cacheDir    string
}

// NewConverter creates a new subtitle converter.
func NewConverter(ffmpegPath string, cacheDir string) *Converter {
	return &Converter{
		ffmpegPath: ffmpegPath,
		cacheDir:   cacheDir,
	}
}

// ExtractAndConvert extracts an embedded subtitle track and converts it to WebVTT.
// Returns the path to the cached WebVTT file.
func (c *Converter) ExtractAndConvert(ctx context.Context, mediaPath string, streamIndex int) (string, error) {
	// Create cache directory for this media file
	mediaHash := hashPath(mediaPath)
	subtitleDir := filepath.Join(c.cacheDir, "subtitles", mediaHash)
	if err := os.MkdirAll(subtitleDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create subtitle cache dir: %w", err)
	}

	outputPath := filepath.Join(subtitleDir, fmt.Sprintf("stream_%d.vtt", streamIndex))

	// Check if already cached and has content
	if info, err := os.Stat(outputPath); err == nil && info.Size() > 0 {
		return outputPath, nil
	}

	// Extract and convert using FFmpeg
	cmd := exec.CommandContext(ctx, c.ffmpegPath,
		"-i", mediaPath,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-c:s", "webvtt",
		"-y",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Clean up empty file if extraction failed
		os.Remove(outputPath)
		return "", fmt.Errorf("ffmpeg subtitle extraction failed: %w, output: %s", err, string(output))
	}

	// Verify the output file has content (extraction can produce empty files on timeout)
	info, err := os.Stat(outputPath)
	if err != nil {
		return "", fmt.Errorf("subtitle file not created: %w", err)
	}
	if info.Size() == 0 {
		os.Remove(outputPath)
		return "", fmt.Errorf("subtitle extraction produced empty file")
	}

	return outputPath, nil
}

// ConvertSRTToWebVTT converts an SRT file to WebVTT format.
func (c *Converter) ConvertSRTToWebVTT(ctx context.Context, srtPath string) (string, error) {
	// Create cache directory
	srtHash := hashPath(srtPath)
	subtitleDir := filepath.Join(c.cacheDir, "subtitles", "external")
	if err := os.MkdirAll(subtitleDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create subtitle cache dir: %w", err)
	}

	outputPath := filepath.Join(subtitleDir, srtHash+".vtt")

	// Check if already cached and has content
	if info, err := os.Stat(outputPath); err == nil && info.Size() > 0 {
		return outputPath, nil
	}

	// Read SRT file
	srtContent, err := os.ReadFile(srtPath)
	if err != nil {
		return "", fmt.Errorf("failed to read SRT file: %w", err)
	}

	// Convert to WebVTT
	vttContent := SRTToWebVTT(string(srtContent))

	// Write WebVTT file
	if err := os.WriteFile(outputPath, []byte(vttContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write WebVTT file: %w", err)
	}

	return outputPath, nil
}

// ConvertASSToWebVTT converts an ASS/SSA file to WebVTT format using FFmpeg.
func (c *Converter) ConvertASSToWebVTT(ctx context.Context, assPath string) (string, error) {
	// Create cache directory
	assHash := hashPath(assPath)
	subtitleDir := filepath.Join(c.cacheDir, "subtitles", "external")
	if err := os.MkdirAll(subtitleDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create subtitle cache dir: %w", err)
	}

	outputPath := filepath.Join(subtitleDir, assHash+".vtt")

	// Check if already cached and has content
	if info, err := os.Stat(outputPath); err == nil && info.Size() > 0 {
		return outputPath, nil
	}

	// Use FFmpeg for ASS conversion (preserves timing, strips styling)
	cmd := exec.CommandContext(ctx, c.ffmpegPath,
		"-i", assPath,
		"-c:s", "webvtt",
		"-y",
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg ASS conversion failed: %w, output: %s", err, string(output))
	}

	return outputPath, nil
}

// ConvertExternalSubtitle converts an external subtitle file to WebVTT.
// Automatically detects the format based on file extension.
func (c *Converter) ConvertExternalSubtitle(ctx context.Context, subtitlePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(subtitlePath))

	switch ext {
	case ".vtt":
		// Already WebVTT, just return the path
		return subtitlePath, nil
	case ".srt":
		return c.ConvertSRTToWebVTT(ctx, subtitlePath)
	case ".ass", ".ssa":
		return c.ConvertASSToWebVTT(ctx, subtitlePath)
	default:
		return "", fmt.Errorf("unsupported subtitle format: %s", ext)
	}
}

// SRTToWebVTT converts SRT content to WebVTT format.
func SRTToWebVTT(srtContent string) string {
	var result strings.Builder

	// WebVTT header
	result.WriteString("WEBVTT\n\n")

	// Normalize line endings
	content := strings.ReplaceAll(srtContent, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// Remove BOM if present
	content = strings.TrimPrefix(content, "\ufeff")

	// Split into blocks
	blocks := strings.Split(content, "\n\n")

	// Regex to match SRT timestamp format: 00:00:00,000 --> 00:00:00,000
	timestampRegex := regexp.MustCompile(`(\d{2}:\d{2}:\d{2}),(\d{3})\s*-->\s*(\d{2}:\d{2}:\d{2}),(\d{3})`)

	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		lines := strings.Split(block, "\n")
		if len(lines) < 2 {
			continue
		}

		// Find the timestamp line
		timestampIdx := -1
		for i, line := range lines {
			if timestampRegex.MatchString(line) {
				timestampIdx = i
				break
			}
		}

		if timestampIdx == -1 {
			continue
		}

		// Convert timestamp from SRT format (comma) to WebVTT format (period)
		timestamp := timestampRegex.ReplaceAllString(lines[timestampIdx], "$1.$2 --> $3.$4")
		result.WriteString(timestamp)
		result.WriteString("\n")

		// Write subtitle text (everything after the timestamp)
		for i := timestampIdx + 1; i < len(lines); i++ {
			result.WriteString(lines[i])
			result.WriteString("\n")
		}

		result.WriteString("\n")
	}

	return result.String()
}

// hashPath creates a simple hash from a file path for caching.
func hashPath(path string) string {
	// Use a simple but sufficient hash for cache keys
	// In production, you might want to include file modification time
	h := uint32(0)
	for _, c := range path {
		h = h*31 + uint32(c)
	}
	return fmt.Sprintf("%08x", h)
}

// GetWebVTTContent reads and returns WebVTT content from a file.
func GetWebVTTContent(vttPath string) (string, error) {
	file, err := os.Open(vttPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var result strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		result.WriteString(scanner.Text())
		result.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return result.String(), nil
}

// IsBitmapFormat returns true if the subtitle codec is a bitmap format
// that cannot be converted to WebVTT (requires burn-in rendering).
func IsBitmapFormat(codec string) bool {
	codec = strings.ToLower(codec)
	bitmapFormats := []string{
		"hdmv_pgs_subtitle", // Blu-ray PGS
		"dvd_subtitle",      // DVD VOBSub
		"dvdsub",
		"pgs",
		"pgssub",
	}

	for _, format := range bitmapFormats {
		if strings.Contains(codec, format) {
			return true
		}
	}
	return false
}
