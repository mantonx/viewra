package subtitles

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Converter handles subtitle extraction and conversion to WebVTT format.
type Converter struct {
	subtitleExtractorPath string
	cacheDir              string
}

// NewConverter creates a new subtitle converter.
func NewConverter(cacheDir string) *Converter {
	return &Converter{
		subtitleExtractorPath: findSubtitleExtractor(),
		cacheDir:              cacheDir,
	}
}

// findSubtitleExtractor locates the subtitle-extractor binary.
// It looks in the same directory as the viewra binary first, then PATH.
func findSubtitleExtractor() string {
	// Try to find in the same directory as the executable
	if execPath, err := os.Executable(); err == nil {
		binDir := filepath.Dir(execPath)
		extractorPath := filepath.Join(binDir, "subtitle-extractor")
		if _, err := os.Stat(extractorPath); err == nil {
			return extractorPath
		}
	}

	// Fall back to PATH lookup
	if path, err := exec.LookPath("subtitle-extractor"); err == nil {
		return path
	}

	// Default: assume it's in bin/ relative to working directory
	return "bin/subtitle-extractor"
}

// getSubtitleCacheDir returns the cache directory for a media file's subtitles.
// Structure: {cacheDir}/{media_id}/subtitles/
func (c *Converter) getSubtitleCacheDir(mediaID int64) string {
	return filepath.Join(c.cacheDir, fmt.Sprintf("%d", mediaID), "subtitles")
}

// ExtractAndConvert extracts an embedded subtitle track and converts it to WebVTT.
// Returns the path to the cached WebVTT file.
//
// The streamIndex parameter is the FFmpeg stream index (0-based, across all streams).
// This function queries subtitle-extractor to find the correct track number for
// the container format (MKV uses 1-based track numbers, MP4 uses track_id, TS uses PID).
func (c *Converter) ExtractAndConvert(ctx context.Context, mediaID int64, mediaPath string, streamIndex int) (string, error) {
	// Create cache directory for this media file
	subtitleDir := c.getSubtitleCacheDir(mediaID)
	if err := os.MkdirAll(subtitleDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create subtitle cache dir: %w", err)
	}

	outputPath := filepath.Join(subtitleDir, fmt.Sprintf("text_stream_%d.vtt", streamIndex))

	// Check if already cached and has content
	if info, err := os.Stat(outputPath); err == nil && info.Size() > 0 {
		return outputPath, nil
	}

	// Get the correct track number for this container
	trackNumber, err := c.findTrackNumber(ctx, mediaPath, streamIndex)
	if err != nil {
		return "", fmt.Errorf("failed to find track number: %w", err)
	}

	cmd := exec.CommandContext(ctx, c.subtitleExtractorPath,
		"stream",
		"--track", fmt.Sprintf("%d", trackNumber),
		"--format", "webvtt",
		mediaPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("subtitle-extractor failed: %w", err)
	}

	// Write output to file
	if err := os.WriteFile(outputPath, output, 0644); err != nil {
		return "", fmt.Errorf("failed to write WebVTT file: %w", err)
	}

	// Verify non-empty output (at least WEBVTT header + some content)
	if len(output) < 10 {
		os.Remove(outputPath)
		return "", fmt.Errorf("subtitle-extractor produced empty output")
	}

	return outputPath, nil
}

// ConvertSRTToWebVTT converts an SRT file to WebVTT format.
func (c *Converter) ConvertSRTToWebVTT(ctx context.Context, srtPath string) (string, error) {
	// Create cache directory - external subtitles use hash of subtitle path
	srtHash := hashPath(srtPath)
	subtitleDir := filepath.Join(c.cacheDir, "external_subtitles")
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

// ConvertASSToWebVTT converts an ASS/SSA file to WebVTT format.
func (c *Converter) ConvertASSToWebVTT(ctx context.Context, assPath string) (string, error) {
	// Create cache directory - external subtitles use hash of subtitle path
	assHash := hashPath(assPath)
	subtitleDir := filepath.Join(c.cacheDir, "external_subtitles")
	if err := os.MkdirAll(subtitleDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create subtitle cache dir: %w", err)
	}

	outputPath := filepath.Join(subtitleDir, assHash+".vtt")

	// Check if already cached and has content
	if info, err := os.Stat(outputPath); err == nil && info.Size() > 0 {
		return outputPath, nil
	}

	// Read ASS file and convert in-process
	assContent, err := os.ReadFile(assPath)
	if err != nil {
		return "", fmt.Errorf("failed to read ASS file: %w", err)
	}

	vttContent := ASSToWebVTT(string(assContent))

	if err := os.WriteFile(outputPath, []byte(vttContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write WebVTT file: %w", err)
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

// ASSToWebVTT converts ASS/SSA content to WebVTT format.
func ASSToWebVTT(assContent string) string {
	var result strings.Builder

	// WebVTT header
	result.WriteString("WEBVTT\n\n")

	// Normalize line endings
	content := strings.ReplaceAll(assContent, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	// Remove BOM if present
	content = strings.TrimPrefix(content, "\ufeff")

	lines := strings.Split(content, "\n")

	// Parse ASS dialogue lines
	// Format: Dialogue: Layer,Start,End,Style,Name,MarginL,MarginR,MarginV,Effect,Text
	dialogueRegex := regexp.MustCompile(`^Dialogue:\s*\d+,(\d+:\d{2}:\d{2}\.\d{2}),(\d+:\d{2}:\d{2}\.\d{2}),[^,]*,[^,]*,[^,]*,[^,]*,[^,]*,[^,]*,(.*)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		matches := dialogueRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		startTime := convertASSTimestamp(matches[1])
		endTime := convertASSTimestamp(matches[2])
		text := stripASSTags(matches[3])

		if text == "" {
			continue
		}

		result.WriteString(fmt.Sprintf("%s --> %s\n", startTime, endTime))
		result.WriteString(text)
		result.WriteString("\n\n")
	}

	return result.String()
}

// convertASSTimestamp converts ASS timestamp (H:MM:SS.cc) to WebVTT (HH:MM:SS.mmm)
func convertASSTimestamp(assTime string) string {
	// ASS format: H:MM:SS.cc (centiseconds)
	// WebVTT format: HH:MM:SS.mmm (milliseconds)
	parts := strings.Split(assTime, ":")
	if len(parts) != 3 {
		return assTime
	}

	hours := parts[0]
	minutes := parts[1]
	secParts := strings.Split(parts[2], ".")
	if len(secParts) != 2 {
		return assTime
	}

	seconds := secParts[0]
	centiseconds := secParts[1]

	// Pad hours to 2 digits
	if len(hours) == 1 {
		hours = "0" + hours
	}

	// Convert centiseconds to milliseconds (multiply by 10)
	ms := centiseconds + "0"

	return fmt.Sprintf("%s:%s:%s.%s", hours, minutes, seconds, ms)
}

// stripASSTags removes ASS formatting tags from text
func stripASSTags(text string) string {
	// Remove {...} tags
	result := regexp.MustCompile(`\{[^}]*\}`).ReplaceAllString(text, "")
	// Convert \N and \n to actual newlines
	result = strings.ReplaceAll(result, "\\N", "\n")
	result = strings.ReplaceAll(result, "\\n", "\n")
	return strings.TrimSpace(result)
}

// trackInfo represents a track from subtitle-extractor's JSON output.
type trackInfo struct {
	Number   int64  `json:"number"`
	Type     string `json:"type"`
	Codec    string `json:"codec"`
	Language string `json:"language,omitempty"`
	Name     string `json:"name,omitempty"`
}

// findTrackNumber queries subtitle-extractor to find the correct track number
// for a given FFmpeg stream index. This is necessary because:
// - MKV uses 1-based track numbers
// - MP4 uses track_id (can be arbitrary)
// - TS/M2TS uses PIDs (packet identifiers)
//
// The FFmpeg stream index is 0-based and counts all streams (video, audio, subtitle).
// We compute the relative subtitle index by counting non-subtitle tracks.
func (c *Converter) findTrackNumber(ctx context.Context, mediaPath string, ffmpegStreamIndex int) (int64, error) {
	// Run subtitle-extractor tracks command to get track listing
	cmd := exec.CommandContext(ctx, c.subtitleExtractorPath, "tracks", mediaPath)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("subtitle-extractor tracks failed: %w", err)
	}

	// Parse JSON output
	var tracks []trackInfo
	if err := json.Unmarshal(output, &tracks); err != nil {
		return 0, fmt.Errorf("failed to parse tracks output: %w", err)
	}

	// subtitle-extractor returns ALL tracks (video, audio, subtitle) in container order.
	// FFmpeg also orders streams in container order.
	// So we can compute the relative subtitle index by counting non-subtitle tracks
	// that appear before this FFmpeg stream index.
	//
	// FFmpeg stream index N means "the Nth stream overall (0-based)".
	// We need to find which subtitle track this corresponds to.

	// Count video and audio tracks (non-subtitle) in the container
	nonSubtitleCount := 0
	var subtitleTracks []trackInfo
	for _, t := range tracks {
		if t.Type == "subtitle" {
			subtitleTracks = append(subtitleTracks, t)
		} else {
			nonSubtitleCount++
		}
	}

	if len(subtitleTracks) == 0 {
		return 0, fmt.Errorf("no subtitle tracks found in file")
	}

	// The relative subtitle index is the FFmpeg stream index minus the number
	// of non-subtitle tracks that come before subtitles in the stream order.
	//
	// In most containers, the order is: video(s), audio(s), subtitle(s).
	// So if FFmpeg says stream index 5 is a subtitle, and there are 4 non-subtitle
	// tracks, then it's the (5 - 4) = 1st subtitle (0-indexed), which is index 1.
	//
	// However, this assumes all non-subtitle tracks come before subtitles.
	// For robustness, we should match by position more carefully.

	// Calculate relative subtitle index
	relativeSubtitleIndex := ffmpegStreamIndex - nonSubtitleCount
	if relativeSubtitleIndex < 0 {
		// FFmpeg index is lower than number of non-subtitle tracks.
		// This shouldn't happen for subtitle streams, but handle gracefully.
		relativeSubtitleIndex = 0
	}

	// Return the subtitle track at that relative index
	if relativeSubtitleIndex < len(subtitleTracks) {
		return subtitleTracks[relativeSubtitleIndex].Number, nil
	}

	// Fallback: if index is out of range, return last subtitle track
	return subtitleTracks[len(subtitleTracks)-1].Number, nil
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
// that cannot be converted to WebVTT (requires image overlay rendering).
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

// PGSFrame represents a single PGS subtitle frame rendered as WebP.
type PGSFrame struct {
	StartMS      int64  `json:"start_ms"`
	EndMS        int64  `json:"end_ms"`
	X            int    `json:"x"`
	Y            int    `json:"y"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	CanvasWidth  int    `json:"canvas_width"`  // PGS authoring resolution width (e.g., 1920 for 1080p subs on 4K video)
	CanvasHeight int    `json:"canvas_height"` // PGS authoring resolution height (e.g., 1080 for 1080p subs on 4K video)
	ImageBase64  string `json:"image_base64"`
}

// pgsFramePath returns the cache path for a PGS frame.
// Format: {cacheDir}/{media_id}/subtitles/pgs_{track}_{startMS}.webp
func (c *Converter) pgsFramePath(mediaID int64, trackNumber int64, startMS int64) string {
	subtitleDir := c.getSubtitleCacheDir(mediaID)
	return filepath.Join(subtitleDir, fmt.Sprintf("pgs_%d_%d.webp", trackNumber, startMS))
}

// loadCachedPGSFrame attempts to load a cached PGS frame from disk.
// Returns nil if not cached.
func (c *Converter) loadCachedPGSFrame(mediaID int64, trackNumber int64, startMS int64) *PGSFrame {
	framePath := c.pgsFramePath(mediaID, trackNumber, startMS)
	metaPath := framePath + ".json"

	// Check if both files exist
	if _, err := os.Stat(framePath); os.IsNotExist(err) {
		return nil
	}
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		return nil
	}

	// Load metadata
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return nil
	}

	var frame PGSFrame
	if err := json.Unmarshal(metaData, &frame); err != nil {
		return nil
	}

	// Load image data
	imgData, err := os.ReadFile(framePath)
	if err != nil {
		return nil
	}

	frame.ImageBase64 = base64.StdEncoding.EncodeToString(imgData)
	return &frame
}

// cachePGSFrame saves a PGS frame to disk cache.
func (c *Converter) cachePGSFrame(mediaID int64, trackNumber int64, frame *PGSFrame) error {
	subtitleDir := c.getSubtitleCacheDir(mediaID)
	if err := os.MkdirAll(subtitleDir, 0755); err != nil {
		return err
	}

	framePath := c.pgsFramePath(mediaID, trackNumber, frame.StartMS)
	metaPath := framePath + ".json"

	// Decode and save image
	imgData, err := base64.StdEncoding.DecodeString(frame.ImageBase64)
	if err != nil {
		return err
	}
	if err := os.WriteFile(framePath, imgData, 0644); err != nil {
		return err
	}

	// Save metadata (without image data)
	meta := PGSFrame{
		StartMS:      frame.StartMS,
		EndMS:        frame.EndMS,
		X:            frame.X,
		Y:            frame.Y,
		Width:        frame.Width,
		Height:       frame.Height,
		CanvasWidth:  frame.CanvasWidth,
		CanvasHeight: frame.CanvasHeight,
	}
	metaData, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, metaData, 0644)
}

// listCachedPGSFrames returns all cached PGS frames for a track within a time range.
func (c *Converter) listCachedPGSFrames(mediaID int64, trackNumber int64, startMS, endMS int64) []PGSFrame {
	subtitleDir := c.getSubtitleCacheDir(mediaID)
	pattern := filepath.Join(subtitleDir, fmt.Sprintf("pgs_%d_*.webp", trackNumber))

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil
	}

	var frames []PGSFrame
	prefix := fmt.Sprintf("pgs_%d_", trackNumber)

	for _, match := range matches {
		// Extract timestamp from filename
		base := filepath.Base(match)
		if !strings.HasPrefix(base, prefix) || !strings.HasSuffix(base, ".webp") {
			continue
		}
		tsStr := strings.TrimSuffix(strings.TrimPrefix(base, prefix), ".webp")
		var ts int64
		if _, err := fmt.Sscanf(tsStr, "%d", &ts); err != nil {
			continue
		}

		// Check if in range
		if ts < startMS || (endMS > 0 && ts >= endMS) {
			continue
		}

		// Load the frame
		if frame := c.loadCachedPGSFrame(mediaID, trackNumber, ts); frame != nil {
			frames = append(frames, *frame)
		}
	}

	// Sort by start time
	sort.Slice(frames, func(i, j int) bool {
		return frames[i].StartMS < frames[j].StartMS
	})

	return frames
}

// StreamPGSFrames extracts PGS subtitle frames as WebP images.
// It streams frames from the given time range and returns them as a channel
// for efficient memory usage with large subtitle tracks.
// Frames are cached to disk for fast subsequent requests.
//
// The caller should iterate over the returned channel until it's closed.
// Any error encountered during streaming will be sent on the error channel.
func (c *Converter) StreamPGSFrames(ctx context.Context, mediaID int64, mediaPath string, streamIndex int, startMS, endMS int64) (<-chan PGSFrame, <-chan error) {
	frames := make(chan PGSFrame, 10)
	errs := make(chan error, 1)

	go func() {
		defer close(frames)
		defer close(errs)

		// Get the correct track number for this container
		trackNumber, err := c.findTrackNumber(ctx, mediaPath, streamIndex)
		if err != nil {
			errs <- fmt.Errorf("failed to find track number: %w", err)
			return
		}

		// Check for cached frames first
		cachedFrames := c.listCachedPGSFrames(mediaID, trackNumber, startMS, endMS)
		if len(cachedFrames) > 0 {
			// Return cached frames
			for _, frame := range cachedFrames {
				select {
				case frames <- frame:
				case <-ctx.Done():
					return
				}
			}
			return
		}

		// No cache - extract from file
		args := []string{
			"stream-pgs",
			"--track", fmt.Sprintf("%d", trackNumber),
			"--start", fmt.Sprintf("%d", startMS),
		}
		if endMS > 0 {
			args = append(args, "--end", fmt.Sprintf("%d", endMS))
		}
		args = append(args, mediaPath)

		cmd := exec.CommandContext(ctx, c.subtitleExtractorPath, args...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			errs <- fmt.Errorf("failed to create stdout pipe: %w", err)
			return
		}

		if err := cmd.Start(); err != nil {
			errs <- fmt.Errorf("failed to start subtitle-extractor: %w", err)
			return
		}

		// Parse JSONL output
		scanner := bufio.NewScanner(stdout)
		// Increase buffer size for large base64 images (up to 10MB per line)
		scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var frame PGSFrame
			if err := json.Unmarshal([]byte(line), &frame); err != nil {
				continue // Skip malformed lines
			}

			// Cache the frame (ignore errors - caching is best-effort)
			_ = c.cachePGSFrame(mediaID, trackNumber, &frame)

			select {
			case frames <- frame:
			case <-ctx.Done():
				cmd.Process.Kill()
				return
			}
		}

		if err := scanner.Err(); err != nil {
			errs <- fmt.Errorf("error reading output: %w", err)
		}

		if err := cmd.Wait(); err != nil {
			// Don't report error if context was cancelled
			if ctx.Err() == nil {
				errs <- fmt.Errorf("subtitle-extractor failed: %w", err)
			}
		}
	}()

	return frames, errs
}

// GetAllPGSFrames extracts all PGS frames for a given stream.
// This is a convenience wrapper that collects all frames into a slice.
// For large subtitle tracks, prefer StreamPGSFrames to avoid high memory usage.
func (c *Converter) GetAllPGSFrames(ctx context.Context, mediaID int64, mediaPath string, streamIndex int) ([]PGSFrame, error) {
	frames, errs := c.StreamPGSFrames(ctx, mediaID, mediaPath, streamIndex, 0, 0)

	var result []PGSFrame
	for frame := range frames {
		result = append(result, frame)
	}

	// Check for any errors
	select {
	case err := <-errs:
		if err != nil {
			return nil, err
		}
	default:
	}

	return result, nil
}
