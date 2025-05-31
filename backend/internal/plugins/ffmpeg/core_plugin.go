package ffmpeg

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/plugins"
	"gorm.io/gorm"
)

// FFprobe availability cache
var (
	ffprobeAvailable     *bool
	ffprobeCheckTime     time.Time
	ffprobeCheckMutex    sync.RWMutex
	ffprobeCheckInterval = 5 * time.Minute // Cache for 5 minutes
	
	// Debug flag - set to false to reduce logging in production
	FFProbeDebugLogging = false
)

func debugLog(format string, args ...interface{}) {
	if FFProbeDebugLogging {
		fmt.Printf(format, args...)
	}
}

// FFProbeOutput represents the JSON output from ffprobe
type FFProbeOutput struct {
	Format FFProbeFormat `json:"format"`
	Streams []FFProbeStream `json:"streams"`
}

type FFProbeFormat struct {
	Filename       string            `json:"filename"`
	NbStreams      int               `json:"nb_streams"`
	NbPrograms     int               `json:"nb_programs"`
	FormatName     string            `json:"format_name"`
	FormatLongName string            `json:"format_long_name"`
	StartTime      string            `json:"start_time"`
	Duration       string            `json:"duration"`
	Size           string            `json:"size"`
	BitRate        string            `json:"bit_rate"`
	ProbeScore     int               `json:"probe_score"`
	Tags           map[string]string `json:"tags"`
}

type FFProbeStream struct {
	Index              int               `json:"index"`
	CodecName          string            `json:"codec_name"`
	CodecLongName      string            `json:"codec_long_name"`
	Profile            string            `json:"profile"`
	CodecType          string            `json:"codec_type"`
	CodecTagString     string            `json:"codec_tag_string"`
	CodecTag           string            `json:"codec_tag"`
	SampleFmt          string            `json:"sample_fmt"`
	SampleRate         string            `json:"sample_rate"`
	Channels           int               `json:"channels"`
	ChannelLayout      string            `json:"channel_layout"`
	BitsPerSample      int               `json:"bits_per_sample"`
	RFrameRate         string            `json:"r_frame_rate"`
	AvgFrameRate       string            `json:"avg_frame_rate"`
	TimeBase           string            `json:"time_base"`
	StartPts           int               `json:"start_pts"`
	StartTime          string            `json:"start_time"`
	DurationTs         int64             `json:"duration_ts"`
	Duration           string            `json:"duration"`
	BitRate            string            `json:"bit_rate"`
	MaxBitRate         string            `json:"max_bit_rate"`
	BitsPerRawSample   string            `json:"bits_per_raw_sample"`
	NbFrames           string            `json:"nb_frames"`
	Tags               map[string]string `json:"tags"`
}

// AudioTechnicalInfo represents technical audio information extracted from ffprobe
type AudioTechnicalInfo struct {
	Format      string  // File format (flac, mp3, ogg, etc.)
	Bitrate     int     // Bitrate in bits per second
	SampleRate  int     // Sample rate in Hz
	Channels    int     // Number of channels
	Duration    float64 // Duration in seconds
	Codec       string  // Audio codec
	IsLossless  bool    // Whether the format is lossless
}

// FFmpegCorePlugin implements the CorePlugin interface for audio and video files
type FFmpegCorePlugin struct {
	name               string
	supportedExts      []string
	enabled            bool
	initialized        bool
}

// NewFFmpegCorePlugin creates a new FFmpeg core plugin instance
func NewFFmpegCorePlugin() plugins.CorePlugin {
	return &FFmpegCorePlugin{
		name:    "ffmpeg_probe_core_plugin",
		enabled: true,
		supportedExts: []string{
			// Video formats
			".mp4", ".mkv", ".avi", ".mov", ".wmv", 
			".flv", ".webm", ".m4v", ".3gp", ".ogv",
			".mpg", ".mpeg", ".ts", ".mts", ".m2ts",
			// Audio formats (for enhanced FFprobe metadata)
			".mp3", ".flac", ".wav", ".m4a", ".aac", 
			".ogg", ".wma", ".opus", ".aiff", ".ape", ".wv",
		},
	}
}

// GetName returns the plugin name
func (p *FFmpegCorePlugin) GetName() string {
	return p.name
}

// GetPluginType returns the plugin type
func (p *FFmpegCorePlugin) GetPluginType() string {
	return "ffmpeg"
}

// GetSupportedExtensions returns the file extensions this plugin supports
func (p *FFmpegCorePlugin) GetSupportedExtensions() []string {
	return p.supportedExts
}

// IsEnabled returns whether the plugin is enabled
func (p *FFmpegCorePlugin) IsEnabled() bool {
	return p.enabled
}

// Initialize performs any setup needed for the plugin
func (p *FFmpegCorePlugin) Initialize() error {
	if p.initialized {
		return nil
	}
	
	fmt.Printf("DEBUG: Initializing FFmpeg Core Plugin\n")
	fmt.Printf("DEBUG: FFmpeg plugin supports %d file types: %v\n", len(p.supportedExts), p.supportedExts)
	
	// Check if FFprobe is available
	if p.isFFProbeAvailable() {
		fmt.Printf("✅ FFprobe detected - Enhanced media metadata available\n")
	} else {
		fmt.Printf("⚠️  FFprobe not found - Media metadata extraction will be limited\n")
	}
	
	p.initialized = true
	return nil
}

// Shutdown performs any cleanup needed when the plugin is disabled
func (p *FFmpegCorePlugin) Shutdown() error {
	fmt.Printf("DEBUG: Shutting down FFmpeg Core Plugin\n")
	p.initialized = false
	return nil
}

// Match determines if this plugin can handle the given file
func (p *FFmpegCorePlugin) Match(path string, info fs.FileInfo) bool {
	if !p.enabled || !p.initialized {
		return false
	}
	
	// Skip directories
	if info.IsDir() {
		return false
	}
	
	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	for _, supportedExt := range p.supportedExts {
		if ext == supportedExt {
			return true
		}
	}
	
	return false
}

// HandleFile processes a media file and extracts metadata using FFprobe
func (p *FFmpegCorePlugin) HandleFile(path string, ctx plugins.MetadataContext) error {
	if !p.enabled || !p.initialized {
		return fmt.Errorf("FFmpeg plugin is disabled or not initialized")
	}

	// Check if we support this file extension
	ext := strings.ToLower(filepath.Ext(path))
	if !p.isExtensionSupported(ext) {
		return fmt.Errorf("unsupported file extension: %s", ext)
	}

	// Extract metadata using FFprobe if available
	mediaMeta, err := p.extractMediaMetadata(path, ctx.MediaFile)
	if err != nil {
		return fmt.Errorf("failed to extract media metadata: %w", err)
	}

	// Save metadata to database
	db, ok := ctx.DB.(*gorm.DB)
	if !ok {
		return fmt.Errorf("invalid database context")
	}
	
	if err := db.Create(mediaMeta).Error; err != nil {
		return fmt.Errorf("failed to save media metadata: %w", err)
	}

	fmt.Printf("DEBUG: FFmpeg plugin processed %s → metadata saved\n", path)
	return nil
}

// isExtensionSupported checks if the file extension is supported
func (p *FFmpegCorePlugin) isExtensionSupported(ext string) bool {
	for _, supportedExt := range p.supportedExts {
		if ext == supportedExt {
			return true
		}
	}
	return false
}

// extractMediaMetadata extracts metadata using FFprobe and returns a MusicMetadata struct
func (p *FFmpegCorePlugin) extractMediaMetadata(filePath string, mediaFile *database.MediaFile) (*database.MusicMetadata, error) {
	if !p.isFFProbeAvailable() {
		return nil, fmt.Errorf("FFprobe not available")
	}

	// Extract technical audio information
	audioInfo, err := p.extractAudioTechnicalInfo(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract audio info: %w", err)
	}

	// Create metadata record
	mediaMeta := &database.MusicMetadata{
		MediaFileID: mediaFile.ID,
		Title:       p.getBaseName(filePath),
		Artist:      "Unknown Artist",
		Album:       "Unknown Album",
		Duration:    int(audioInfo.Duration),
	}

	// Try to extract additional metadata from FFprobe tags
	if audioInfo.Format != "" {
		// Format information is available but not stored in the simplified schema
		debugLog("Audio format: %s, Codec: %s\n", audioInfo.Format, audioInfo.Codec)
	}

	return mediaMeta, nil
}

// isFFProbeAvailable checks if ffprobe is available on the system (cached)
func (p *FFmpegCorePlugin) isFFProbeAvailable() bool {
	ffprobeCheckMutex.RLock()
	
	// Check if we have a cached result that's still valid
	if ffprobeAvailable != nil && time.Since(ffprobeCheckTime) < ffprobeCheckInterval {
		result := *ffprobeAvailable
		ffprobeCheckMutex.RUnlock()
		return result
	}
	
	ffprobeCheckMutex.RUnlock()
	ffprobeCheckMutex.Lock()
	defer ffprobeCheckMutex.Unlock()
	
	// Double-check after acquiring write lock
	if ffprobeAvailable != nil && time.Since(ffprobeCheckTime) < ffprobeCheckInterval {
		return *ffprobeAvailable
	}
	
	// Check if ffprobe is available
	cmd := exec.Command("ffprobe", "-version")
	err := cmd.Run()
	
	available := err == nil
	ffprobeAvailable = &available
	ffprobeCheckTime = time.Now()
	
	if available {
		debugLog("DEBUG: FFprobe is available\n")
	} else {
		debugLog("DEBUG: FFprobe is not available: %v\n", err)
	}
	
	return available
}

// extractAudioTechnicalInfo uses ffprobe to extract technical audio information
func (p *FFmpegCorePlugin) extractAudioTechnicalInfo(filePath string) (*AudioTechnicalInfo, error) {
	// Run ffprobe command with better error handling
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath)

	debugLog("DEBUG: Running ffprobe on: %s\n", filePath)
	
	output, err := cmd.Output()
	if err != nil {
		// Get more detailed error information
		if exitError, ok := err.(*exec.ExitError); ok {
			stderr := string(exitError.Stderr)
			fmt.Printf("ERROR: ffprobe failed for %s - Exit code: %d, Stderr: %s\n", filePath, exitError.ExitCode(), stderr)
			return nil, fmt.Errorf("ffprobe command failed with exit code %d: %s - stderr: %s", exitError.ExitCode(), err, stderr)
		}
		fmt.Printf("ERROR: ffprobe command execution failed for %s: %v\n", filePath, err)
		return nil, fmt.Errorf("ffprobe command failed: %w", err)
	}

	debugLog("DEBUG: ffprobe output length: %d bytes\n", len(output))

	// Parse JSON output
	var probeOutput FFProbeOutput
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		fmt.Printf("ERROR: Failed to parse ffprobe JSON output for %s: %v\n", filePath, err)
		debugLog("DEBUG: Raw ffprobe output: %s\n", string(output)[:min(500, len(output))])
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	debugLog("DEBUG: Parsed ffprobe output - Format: %s, Streams: %d\n", probeOutput.Format.FormatName, len(probeOutput.Streams))

	// Find the first audio stream
	var audioStream *FFProbeStream
	for i := range probeOutput.Streams {
		if probeOutput.Streams[i].CodecType == "audio" {
			audioStream = &probeOutput.Streams[i]
			debugLog("DEBUG: Found audio stream - Codec: %s, Channels: %d, Sample Rate: %s, Bitrate: %s\n", 
				audioStream.CodecName, audioStream.Channels, audioStream.SampleRate, audioStream.BitRate)
			break
		}
	}

	if audioStream == nil {
		fmt.Printf("WARNING: No audio stream found in file %s\n", filePath)
		return nil, fmt.Errorf("no audio stream found in file")
	}

	// Extract technical information
	info := &AudioTechnicalInfo{}

	// Determine format from format_name (prioritize this over file extension)
	info.Format = p.determineAudioFormat(probeOutput.Format.FormatName, filePath)
	debugLog("DEBUG: Determined format: %s (from ffprobe: %s)\n", info.Format, probeOutput.Format.FormatName)

	// Extract bitrate (prefer stream bitrate over format bitrate)
	if audioStream.BitRate != "" {
		if bitrate, err := strconv.Atoi(audioStream.BitRate); err == nil {
			info.Bitrate = bitrate
		} else {
			debugLog("WARNING: Failed to parse stream bitrate '%s': %v\n", audioStream.BitRate, err)
		}
	} else if probeOutput.Format.BitRate != "" {
		if bitrate, err := strconv.Atoi(probeOutput.Format.BitRate); err == nil {
			info.Bitrate = bitrate
		} else {
			debugLog("WARNING: Failed to parse format bitrate '%s': %v\n", probeOutput.Format.BitRate, err)
		}
	}

	// Extract sample rate
	if audioStream.SampleRate != "" {
		if sampleRate, err := strconv.Atoi(audioStream.SampleRate); err == nil {
			info.SampleRate = sampleRate
		} else {
			debugLog("WARNING: Failed to parse sample rate '%s': %v\n", audioStream.SampleRate, err)
		}
	}

	// Extract channels
	info.Channels = audioStream.Channels

	// Extract duration
	if probeOutput.Format.Duration != "" {
		if duration, err := strconv.ParseFloat(probeOutput.Format.Duration, 64); err == nil {
			info.Duration = duration
		} else {
			debugLog("WARNING: Failed to parse duration '%s': %v\n", probeOutput.Format.Duration, err)
		}
	}

	// Extract codec
	info.Codec = audioStream.CodecName

	// Determine if format is lossless
	info.IsLossless = p.isLosslessFormat(info.Format, info.Codec)

	debugLog("SUCCESS: FFprobe extraction complete for %s - Format: %s, Bitrate: %d, SampleRate: %d, Channels: %d\n", 
		filePath, info.Format, info.Bitrate, info.SampleRate, info.Channels)

	return info, nil
}

// determineAudioFormat determines the audio format from ffprobe output
func (p *FFmpegCorePlugin) determineAudioFormat(formatName, filePath string) string {
	// Map ffprobe format names to our standard format names
	formatMap := map[string]string{
		"mp3":        "mp3",
		"flac":       "flac",
		"ogg":        "ogg",
		"wav":        "wav",
		"aiff":       "aiff",
		"mp4":        "aac", // or m4a
		"matroska":   "mkv", // could be audio
		"avi":        "avi",
		"mov,mp4,m4a,3gp,3g2,mj2": "m4a", // Complex format string for m4a/mp4
	}
	
	// First try exact match
	if format, exists := formatMap[formatName]; exists {
		return format
	}
	
	// Try partial matches for complex format strings
	lowerFormatName := strings.ToLower(formatName)
	for key, value := range formatMap {
		if strings.Contains(lowerFormatName, key) {
			return value
		}
	}
	
	// Fallback to file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if len(ext) > 1 {
		return ext[1:] // Remove the dot
	}
	
	return "unknown"
}

// isLosslessFormat determines if a format/codec combination is lossless
func (p *FFmpegCorePlugin) isLosslessFormat(format, codec string) bool {
	losslessFormats := map[string]bool{
		"flac": true,
		"wav":  true,
		"aiff": true,
		"ape":  true,
		"wv":   true, // WavPack
	}
	
	losslessCodecs := map[string]bool{
		"flac":    true,
		"pcm_s16le": true,
		"pcm_s24le": true,
		"pcm_s32le": true,
		"pcm_f32le": true,
		"pcm_f64le": true,
		"ape":     true,
		"wavpack": true,
	}
	
	return losslessFormats[format] || losslessCodecs[codec]
}

// getMediaType determines if a file is audio or video based on extension
func (p *FFmpegCorePlugin) getMediaType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	audioExts := map[string]bool{
		".mp3": true, ".flac": true, ".wav": true, ".m4a": true, ".aac": true,
		".ogg": true, ".wma": true, ".opus": true, ".aiff": true, ".ape": true, ".wv": true,
	}
	
	if audioExts[ext] {
		return "audio"
	}
	
	return "video"
}

// getBaseName returns the filename without path and extension
func (p *FFmpegCorePlugin) getBaseName(filePath string) string {
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// min function for Go versions that don't have it built-in
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}