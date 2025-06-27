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
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
	"gorm.io/gorm"
)

// Register FFmpeg core plugin with the correct pluginmodule registry
func init() {
	pluginmodule.RegisterCorePluginFactory("ffmpeg", func() pluginmodule.CorePlugin {
		return NewFFmpegCorePlugin()
	})
}

// FFprobe availability cache
var (
	ffprobeAvailable     *bool
	ffprobeCheckTime     time.Time
	ffprobeCheckMutex    sync.RWMutex
	ffprobeCheckInterval = 5 * time.Minute // Cache for 5 minutes

	// Debug flag - set to false to reduce logging in production
	FFProbeDebugLogging = true
)

func debugLog(format string, args ...interface{}) {
	if FFProbeDebugLogging {
		fmt.Printf(format, args...)
	}
}

// FFProbeOutput represents the JSON output from ffprobe
type FFProbeOutput struct {
	Format  FFProbeFormat   `json:"format"`
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
	Index          int    `json:"index"`
	CodecName      string `json:"codec_name"`
	CodecLongName  string `json:"codec_long_name"`
	Profile        string `json:"profile"`
	Level          int    `json:"level"`
	CodecType      string `json:"codec_type"`
	CodecTagString string `json:"codec_tag_string"`
	CodecTag       string `json:"codec_tag"`

	// Video-specific fields
	Width              int    `json:"width"`
	Height             int    `json:"height"`
	SampleAspectRatio  string `json:"sample_aspect_ratio"`
	DisplayAspectRatio string `json:"display_aspect_ratio"`
	PixFmt             string `json:"pix_fmt"`
	FieldOrder         string `json:"field_order"`
	ChromaLocation     string `json:"chroma_location"`
	Refs               int    `json:"refs"`
	IsAvc              string `json:"is_avc"`
	NalLengthSize      string `json:"nal_length_size"`

	// Color and HDR information
	ColorRange       string `json:"color_range"`
	ColorSpace       string `json:"color_space"`
	ColorTransfer    string `json:"color_transfer"`
	ColorPrimaries   string `json:"color_primaries"`
	BitsPerRawSample string `json:"bits_per_raw_sample"`

	// Audio-specific fields
	SampleFmt     string `json:"sample_fmt"`
	SampleRate    string `json:"sample_rate"`
	Channels      int    `json:"channels"`
	ChannelLayout string `json:"channel_layout"`
	BitsPerSample int    `json:"bits_per_sample"`

	// Frame rate and timing
	RFrameRate   string `json:"r_frame_rate"`
	AvgFrameRate string `json:"avg_frame_rate"`
	TimeBase     string `json:"time_base"`
	StartPts     int    `json:"start_pts"`
	StartTime    string `json:"start_time"`
	DurationTs   int64  `json:"duration_ts"`
	Duration     string `json:"duration"`

	// Bitrate information
	BitRate    string `json:"bit_rate"`
	MaxBitRate string `json:"max_bit_rate"`
	NbFrames   string `json:"nb_frames"`

	// Stream metadata and tags
	Tags        map[string]string `json:"tags"`
	Disposition map[string]int    `json:"disposition"`
}

// AudioTechnicalInfo represents technical audio information extracted from ffprobe
type AudioTechnicalInfo struct {
	Format     string  // File format (flac, mp3, ogg, etc.)
	Bitrate    int     // Bitrate in bits per second
	SampleRate int     // Sample rate in Hz
	Channels   int     // Number of channels
	Duration   float64 // Duration in seconds
	Codec      string  // Audio codec
	IsLossless bool    // Whether the format is lossless
}

// FFmpegCorePlugin implements the CorePlugin interface for audio and video files
type FFmpegCorePlugin struct {
	name          string
	supportedExts []string
	enabled       bool
	initialized   bool
}

// NewFFmpegCorePlugin creates a new FFmpeg core plugin instance
func NewFFmpegCorePlugin() pluginmodule.CorePlugin {
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

// GetName returns the plugin name (implements FileHandlerPlugin)
func (p *FFmpegCorePlugin) GetName() string {
	return p.name
}

// GetPluginType returns the plugin type (implements FileHandlerPlugin)
func (p *FFmpegCorePlugin) GetPluginType() string {
	return "ffmpeg"
}

// GetType returns the plugin type (implements FileHandlerPlugin)
func (p *FFmpegCorePlugin) GetType() string {
	return "ffmpeg"
}

// GetSupportedExtensions returns the file extensions this plugin supports (implements FileHandlerPlugin)
func (p *FFmpegCorePlugin) GetSupportedExtensions() []string {
	return p.supportedExts
}

// GetDisplayName returns a human-readable display name for the plugin (implements CorePlugin)
func (p *FFmpegCorePlugin) GetDisplayName() string {
	return "FFmpeg Probe Core Plugin"
}

// IsEnabled returns whether the plugin is enabled (implements CorePlugin)
func (p *FFmpegCorePlugin) IsEnabled() bool {
	return p.enabled
}

// Enable enables the plugin (implements CorePlugin)
func (p *FFmpegCorePlugin) Enable() error {
	p.enabled = true
	return nil
}

// Disable disables the plugin (implements CorePlugin)
func (p *FFmpegCorePlugin) Disable() error {
	p.enabled = false
	return nil
}

// Initialize performs any setup needed for the plugin (implements CorePlugin)
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

// Shutdown performs any cleanup needed when the plugin is disabled (implements CorePlugin)
func (p *FFmpegCorePlugin) Shutdown() error {
	fmt.Printf("DEBUG: Shutting down FFmpeg Core Plugin\n")
	p.initialized = false
	return nil
}

// Match determines if this plugin can handle the given file (implements FileHandlerPlugin)
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

// HandleFile processes a media file and extracts metadata using FFprobe (implements FileHandlerPlugin)
func (p *FFmpegCorePlugin) HandleFile(path string, ctx *pluginmodule.MetadataContext) error {
	if !p.enabled || !p.initialized {
		return fmt.Errorf("FFmpeg plugin is disabled or not initialized")
	}

	// Check if we support this file extension
	ext := strings.ToLower(filepath.Ext(path))
	if !p.isExtensionSupported(ext) {
		return fmt.Errorf("unsupported file extension: %s", ext)
	}

	// Get database connection from context
	db := ctx.DB

	// Extract technical metadata ONLY (for both audio and video files)
	// DO NOT create Artist/Album/Track records - leave that to enrichment plugins
	if p.isFFProbeAvailable() {
		if err := p.updateMediaFileWithTechnicalInfo(path, ctx.MediaFile.ID, db); err != nil {
			fmt.Printf("WARNING: Failed to extract technical metadata for %s: %v\n", path, err)
			// Continue without technical metadata - not a fatal error
		}
	}

	// NOTE: Removed createMusicRecords() call - enrichment plugins will handle metadata creation
	fmt.Printf("DEBUG: FFmpeg plugin processed %s (technical metadata only)\n", path)
	return nil
}

// AudioTrackInfo holds extracted audio track information
type AudioTrackInfo struct {
	Title       string
	Artist      string
	Album       string
	Duration    int
	MediaFileID string
}

// extractAudioTrackInfo extracts audio metadata and creates a track info structure
func (p *FFmpegCorePlugin) extractAudioTrackInfo(filePath string, mediaFile *database.MediaFile) (*AudioTrackInfo, error) {
	if !p.isFFProbeAvailable() {
		return nil, fmt.Errorf("FFprobe not available")
	}

	// Extract technical audio information
	audioInfo, err := p.extractAudioTechnicalInfo(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract audio info: %w", err)
	}

	// Create track info with basic metadata
	trackInfo := &AudioTrackInfo{
		Title:       p.getBaseName(filePath),
		Artist:      "Unknown Artist",
		Album:       "Unknown Album",
		Duration:    int(audioInfo.Duration),
		MediaFileID: mediaFile.ID,
	}

	return trackInfo, nil
}

// createMusicRecords creates Artist/Album/Track records for audio files
func (p *FFmpegCorePlugin) createMusicRecords(db *gorm.DB, trackInfo *AudioTrackInfo, mediaFileID string) error {
	// Create or get Artist
	artist, err := p.createOrGetArtist(db, trackInfo.Artist)
	if err != nil {
		return fmt.Errorf("failed to create/get artist: %w", err)
	}

	// Create or get Album
	album, err := p.createOrGetAlbum(db, trackInfo.Album, artist.ID)
	if err != nil {
		return fmt.Errorf("failed to create/get album: %w", err)
	}

	// Create or update Track
	track, err := p.createOrUpdateTrack(db, trackInfo, artist.ID, album.ID)
	if err != nil {
		return fmt.Errorf("failed to create/update track: %w", err)
	}

	// Update MediaFile to link to the track
	err = db.Model(&database.MediaFile{}).
		Where("id = ?", mediaFileID).
		Updates(map[string]interface{}{
			"media_id":   track.ID,
			"media_type": database.MediaTypeTrack,
		}).Error

	return err
}

// createOrGetArtist creates a new artist or returns existing one
func (p *FFmpegCorePlugin) createOrGetArtist(db *gorm.DB, artistName string) (*database.Artist, error) {
	var artist database.Artist

	// Check if artist already exists
	result := db.Where("name = ?", artistName).First(&artist)
	if result.Error == nil {
		return &artist, nil
	}

	// Create new artist
	artist = database.Artist{
		ID:   fmt.Sprintf("artist-%s", strings.ReplaceAll(strings.ToLower(artistName), " ", "-")),
		Name: artistName,
	}

	// If ID already exists, generate a unique one
	var existingArtist database.Artist
	if db.Where("id = ?", artist.ID).First(&existingArtist).Error == nil {
		artist.ID = fmt.Sprintf("artist-%s-%d", strings.ReplaceAll(strings.ToLower(artistName), " ", "-"), time.Now().Unix())
	}

	if err := db.Create(&artist).Error; err != nil {
		return nil, fmt.Errorf("failed to create artist: %w", err)
	}

	return &artist, nil
}

// createOrGetAlbum creates a new album or returns existing one
func (p *FFmpegCorePlugin) createOrGetAlbum(db *gorm.DB, albumTitle string, artistID string) (*database.Album, error) {
	var album database.Album

	// Check if album already exists for this artist
	result := db.Where("title = ? AND artist_id = ?", albumTitle, artistID).First(&album)
	if result.Error == nil {
		return &album, nil
	}

	// Create new album
	album = database.Album{
		ID:       fmt.Sprintf("album-%s-%s", artistID, strings.ReplaceAll(strings.ToLower(albumTitle), " ", "-")),
		Title:    albumTitle,
		ArtistID: artistID,
	}

	// If ID already exists, generate a unique one
	var existingAlbum database.Album
	if db.Where("id = ?", album.ID).First(&existingAlbum).Error == nil {
		album.ID = fmt.Sprintf("album-%s-%s-%d", artistID, strings.ReplaceAll(strings.ToLower(albumTitle), " ", "-"), time.Now().Unix())
	}

	if err := db.Create(&album).Error; err != nil {
		return nil, fmt.Errorf("failed to create album: %w", err)
	}

	return &album, nil
}

// createOrUpdateTrack creates a new track or updates existing one
func (p *FFmpegCorePlugin) createOrUpdateTrack(db *gorm.DB, trackInfo *AudioTrackInfo, artistID string, albumID string) (*database.Track, error) {
	var track database.Track

	// Check if track already exists for this album
	result := db.Where("title = ? AND album_id = ?", trackInfo.Title, albumID).First(&track)

	if result.Error == nil {
		// Update existing track
		track.ArtistID = artistID
		track.Duration = trackInfo.Duration

		if err := db.Save(&track).Error; err != nil {
			return nil, fmt.Errorf("failed to update track: %w", err)
		}

		return &track, nil
	}

	// Create new track
	track = database.Track{
		ID:       fmt.Sprintf("track-%s-%s", albumID, strings.ReplaceAll(strings.ToLower(trackInfo.Title), " ", "-")),
		Title:    trackInfo.Title,
		AlbumID:  albumID,
		ArtistID: artistID,
		Duration: trackInfo.Duration,
	}

	// If ID already exists, generate a unique one
	var existingTrack database.Track
	if db.Where("id = ?", track.ID).First(&existingTrack).Error == nil {
		track.ID = fmt.Sprintf("track-%s-%s-%d", albumID, strings.ReplaceAll(strings.ToLower(trackInfo.Title), " ", "-"), time.Now().Unix())
	}

	if err := db.Create(&track).Error; err != nil {
		return nil, fmt.Errorf("failed to create track: %w", err)
	}

	return &track, nil
}

// isAudioFile determines if a file is an audio file based on extension
func (p *FFmpegCorePlugin) isAudioFile(ext string) bool {
	audioExts := map[string]bool{
		".mp3": true, ".flac": true, ".wav": true, ".m4a": true, ".aac": true,
		".ogg": true, ".wma": true, ".opus": true, ".aiff": true, ".ape": true, ".wv": true,
	}
	return audioExts[ext]
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
		"mp3":                     "mp3",
		"flac":                    "flac",
		"ogg":                     "ogg",
		"wav":                     "wav",
		"aiff":                    "aiff",
		"mp4":                     "aac", // or m4a
		"matroska":                "mkv", // could be audio
		"avi":                     "avi",
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
		"flac":      true,
		"pcm_s16le": true,
		"pcm_s24le": true,
		"pcm_s32le": true,
		"pcm_f32le": true,
		"pcm_f64le": true,
		"ape":       true,
		"wavpack":   true,
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

// ExtractAudioTechnicalInfo exposes the audio technical information extraction
func (p *FFmpegCorePlugin) ExtractAudioTechnicalInfo(filePath string) (*AudioTechnicalInfo, error) {
	return p.extractAudioTechnicalInfo(filePath)
}

// ExtractComprehensiveTechnicalInfo exposes the comprehensive video technical information extraction
func (p *FFmpegCorePlugin) ExtractComprehensiveTechnicalInfo(filePath string) (*VideoTechnicalInfo, error) {
	return p.extractComprehensiveTechnicalInfo(filePath)
}

// updateMediaFileWithTechnicalInfo extracts technical metadata and updates the MediaFile record
func (p *FFmpegCorePlugin) updateMediaFileWithTechnicalInfo(filePath string, mediaFileID string, db *gorm.DB) error {
	// Extract technical information using FFprobe
	if p.isAudioFile(strings.ToLower(filepath.Ext(filePath))) {
		// For audio files, extract detailed audio technical info
		audioInfo, err := p.extractAudioTechnicalInfo(filePath)
		if err != nil {
			return fmt.Errorf("failed to extract audio technical info: %w", err)
		}

		// Update MediaFile with technical metadata
		updates := map[string]interface{}{
			"container":    audioInfo.Format,
			"audio_codec":  audioInfo.Codec,
			"channels":     fmt.Sprintf("%d", audioInfo.Channels),
			"sample_rate":  audioInfo.SampleRate,
			"duration":     int(audioInfo.Duration),
			"bitrate_kbps": audioInfo.Bitrate / 1000, // Convert to kbps

			// Enhanced audio fields
			"audio_channels":    audioInfo.Channels,
			"audio_sample_rate": audioInfo.SampleRate,
			"audio_language":    "und", // undetermined, could be enhanced later
		}

		err = db.Model(&database.MediaFile{}).
			Where("id = ?", mediaFileID).
			Updates(updates).Error

		if err != nil {
			return fmt.Errorf("failed to update media file with audio technical info: %w", err)
		}

		fmt.Printf("DEBUG: Updated MediaFile %s with technical info - Format: %s, Codec: %s, Channels: %d, Duration: %ds, Bitrate: %d kbps\n",
			mediaFileID, audioInfo.Format, audioInfo.Codec, audioInfo.Channels, int(audioInfo.Duration), audioInfo.Bitrate/1000)

	} else {
		// For video files, extract comprehensive video technical info
		videoInfo, err := p.extractComprehensiveTechnicalInfo(filePath)
		if err != nil {
			return fmt.Errorf("failed to extract comprehensive video technical info: %w", err)
		}

		// Serialize the comprehensive technical info to JSON
		technicalInfoJSON, err := json.Marshal(videoInfo)
		if err != nil {
			return fmt.Errorf("failed to serialize technical info: %w", err)
		}

		videoStreamsJSON, err := json.Marshal(videoInfo.VideoStreams)
		if err != nil {
			return fmt.Errorf("failed to serialize video streams: %w", err)
		}

		audioStreamsJSON, err := json.Marshal(videoInfo.AudioStreams)
		if err != nil {
			return fmt.Errorf("failed to serialize audio streams: %w", err)
		}

		subtitleStreamsJSON, err := json.Marshal(videoInfo.SubtitleStreams)
		if err != nil {
			return fmt.Errorf("failed to serialize subtitle streams: %w", err)
		}

		// Prepare comprehensive updates map
		updates := map[string]interface{}{
			"container":    videoInfo.Container,
			"video_codec":  videoInfo.VideoCodec,
			"audio_codec":  videoInfo.AudioCodec,
			"resolution":   videoInfo.Resolution,
			"duration":     int(videoInfo.Duration),
			"bitrate_kbps": videoInfo.Bitrate / 1000, // Convert to kbps

			// Store comprehensive JSON data
			"technical_info":   string(technicalInfoJSON),
			"video_streams":    string(videoStreamsJSON),
			"audio_streams":    string(audioStreamsJSON),
			"subtitle_streams": string(subtitleStreamsJSON),

			// Enhanced boolean flags
			"has_video":     videoInfo.HasVideo,
			"has_audio":     videoInfo.HasAudio,
			"has_subtitles": videoInfo.HasSubtitles,
		}

		// Add enhanced fields from first video stream if available
		if len(videoInfo.VideoStreams) > 0 {
			firstVideo := videoInfo.VideoStreams[0]
			updates["video_width"] = firstVideo.Width
			updates["video_height"] = firstVideo.Height
			updates["video_framerate"] = firstVideo.Framerate
			updates["video_profile"] = firstVideo.Profile
			updates["video_level"] = firstVideo.Level
			updates["video_bit_depth"] = firstVideo.BitDepth
			updates["aspect_ratio"] = firstVideo.AspectRatio
			updates["pixel_format"] = firstVideo.PixelFormat
			updates["color_space"] = firstVideo.ColorSpace
			updates["color_primaries"] = firstVideo.ColorPrimaries
			updates["color_transfer"] = firstVideo.ColorTransfer
			updates["hdr_format"] = firstVideo.HDRFormat
			updates["interlaced"] = firstVideo.Interlaced
			updates["reference_frames"] = firstVideo.ReferenceFrames
		}

		// Add enhanced fields from first audio stream if available
		if len(videoInfo.AudioStreams) > 0 {
			firstAudio := videoInfo.AudioStreams[0]
			updates["audio_channels"] = firstAudio.Channels
			updates["audio_layout"] = firstAudio.Layout
			updates["audio_sample_rate"] = firstAudio.SampleRate
			updates["audio_bit_depth"] = firstAudio.BitDepth
			updates["audio_language"] = firstAudio.Language
			updates["audio_profile"] = firstAudio.Profile
		}

		err = db.Model(&database.MediaFile{}).
			Where("id = ?", mediaFileID).
			Updates(updates).Error

		if err != nil {
			return fmt.Errorf("failed to update media file with comprehensive video technical info: %w", err)
		}

		fmt.Printf("DEBUG: Updated MediaFile %s with comprehensive technical info - Container: %s, Video: %s, Audio: %s, Resolution: %s, Duration: %ds, Video streams: %d, Audio streams: %d, Subtitle streams: %d\n",
			mediaFileID, videoInfo.Container, videoInfo.VideoCodec, videoInfo.AudioCodec, videoInfo.Resolution, int(videoInfo.Duration),
			len(videoInfo.VideoStreams), len(videoInfo.AudioStreams), len(videoInfo.SubtitleStreams))

		// Log detailed stream information for debugging
		for i, stream := range videoInfo.VideoStreams {
			fmt.Printf("DEBUG: Video Stream %d - Codec: %s, Profile: %s, Resolution: %dx%d, Framerate: %s, Bitrate: %s, HDR: %s, Color: %s/%s/%s\n",
				i, stream.Codec, stream.Profile, stream.Width, stream.Height, stream.Framerate, stream.Bitrate, stream.HDRFormat, stream.ColorSpace, stream.ColorPrimaries, stream.ColorTransfer)
		}

		for i, stream := range videoInfo.AudioStreams {
			fmt.Printf("DEBUG: Audio Stream %d - Codec: %s, Layout: %s, Channels: %d, Sample Rate: %d, Bitrate: %s, Language: %s\n",
				i, stream.Codec, stream.Layout, stream.Channels, stream.SampleRate, stream.Bitrate, stream.Language)
		}

		for i, stream := range videoInfo.SubtitleStreams {
			fmt.Printf("DEBUG: Subtitle Stream %d - Codec: %s, Language: %s, Title: %s\n",
				i, stream.Codec, stream.Language, stream.Title)
		}
	}

	return nil
}

// VideoTechnicalInfo represents comprehensive technical video information
type VideoTechnicalInfo struct {
	// Container information
	Container string  `json:"container"`
	Duration  float64 `json:"duration"`
	Bitrate   int     `json:"bitrate"`
	Size      int64   `json:"size"`

	// Video stream details
	VideoStreams    []VideoStreamInfo    `json:"video_streams"`
	AudioStreams    []AudioStreamInfo    `json:"audio_streams"`
	SubtitleStreams []SubtitleStreamInfo `json:"subtitle_streams"`

	// Overall media information
	HasVideo     bool `json:"has_video"`
	HasAudio     bool `json:"has_audio"`
	HasSubtitles bool `json:"has_subtitles"`

	// Legacy fields for backward compatibility
	VideoCodec string `json:"video_codec"`
	AudioCodec string `json:"audio_codec"`
	Resolution string `json:"resolution"`
}

// Detailed video stream information
type VideoStreamInfo struct {
	Index int    `json:"index"`
	Title string `json:"title"`

	// Codec information
	Codec         string `json:"codec"`
	CodecLongName string `json:"codec_long_name"`
	Profile       string `json:"profile"`
	Level         string `json:"level"`

	// Resolution and aspect ratio
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Resolution  string `json:"resolution"`
	AspectRatio string `json:"aspect_ratio"`

	// Frame rate and interlacing
	Framerate    string `json:"framerate"`
	AvgFramerate string `json:"avg_framerate"`
	Interlaced   string `json:"interlaced"`
	FieldOrder   string `json:"field_order"`

	// Bitrate and quality
	Bitrate    string `json:"bitrate"`
	MaxBitrate string `json:"max_bitrate"`

	// Color information
	ColorPrimaries string `json:"color_primaries"`
	ColorSpace     string `json:"color_space"`
	ColorTransfer  string `json:"color_transfer"`
	ColorRange     string `json:"color_range"`
	BitDepth       string `json:"bit_depth"`
	PixelFormat    string `json:"pixel_format"`
	ChromaLocation string `json:"chroma_location"`

	// HDR metadata
	HDRFormat    string `json:"hdr_format"`
	MaxLuminance string `json:"max_luminance"`
	MinLuminance string `json:"min_luminance"`
	MaxCLL       string `json:"max_cll"`
	MaxFALL      string `json:"max_fall"`

	// Advanced codec information
	ReferenceFrames int    `json:"reference_frames"`
	IsAVC           bool   `json:"is_avc"`
	NALLengthSize   string `json:"nal_length_size"`

	// Stream metadata
	Language string  `json:"language"`
	Default  bool    `json:"default"`
	Forced   bool    `json:"forced"`
	Duration float64 `json:"duration"`
}

// Detailed audio stream information
type AudioStreamInfo struct {
	Index         int    `json:"index"`
	Title         string `json:"title"`
	EmbeddedTitle string `json:"embedded_title"`

	// Codec information
	Codec         string `json:"codec"`
	CodecLongName string `json:"codec_long_name"`
	Profile       string `json:"profile"`

	// Channel information
	Channels      int    `json:"channels"`
	ChannelLayout string `json:"channel_layout"`
	Layout        string `json:"layout"`

	// Quality information
	SampleRate   int    `json:"sample_rate"`
	BitDepth     int    `json:"bit_depth"`
	Bitrate      string `json:"bitrate"`
	SampleFormat string `json:"sample_format"`

	// Stream metadata
	Language string  `json:"language"`
	Default  bool    `json:"default"`
	Forced   bool    `json:"forced"`
	Duration float64 `json:"duration"`
}

// Subtitle stream information
type SubtitleStreamInfo struct {
	Index         int    `json:"index"`
	Title         string `json:"title"`
	Codec         string `json:"codec"`
	CodecLongName string `json:"codec_long_name"`
	Language      string `json:"language"`
	Default       bool   `json:"default"`
	Forced        bool   `json:"forced"`
}

// extractComprehensiveTechnicalInfo extracts comprehensive technical info for video files
func (p *FFmpegCorePlugin) extractComprehensiveTechnicalInfo(filePath string) (*VideoTechnicalInfo, error) {
	// Run ffprobe command with comprehensive output
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-show_entries", "stream=index,codec_name,codec_long_name,profile,level,codec_type,width,height,sample_aspect_ratio,display_aspect_ratio,pix_fmt,field_order,color_range,color_space,color_transfer,color_primaries,chroma_location,refs,is_avc,nal_length_size,r_frame_rate,avg_frame_rate,bit_rate,max_bit_rate,bits_per_raw_sample,sample_rate,channels,channel_layout,bits_per_sample,duration,tags,disposition",
		"-show_entries", "format=filename,nb_streams,format_name,format_long_name,duration,size,bit_rate",
		filePath)

	debugLog("DEBUG: Running comprehensive ffprobe on: %s\n", filePath)

	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			stderr := string(exitError.Stderr)
			fmt.Printf("ERROR: ffprobe failed for %s - Exit code: %d, Stderr: %s\n", filePath, exitError.ExitCode(), stderr)
			return nil, fmt.Errorf("ffprobe command failed with exit code %d: %s - stderr: %s", exitError.ExitCode(), err, stderr)
		}
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

	info := &VideoTechnicalInfo{}

	// Extract container format
	info.Container = p.determineContainerFormat(probeOutput.Format.FormatName, filePath)

	// Extract duration
	if probeOutput.Format.Duration != "" {
		if duration, err := strconv.ParseFloat(probeOutput.Format.Duration, 64); err == nil {
			info.Duration = duration
		}
	}

	// Extract overall bitrate
	if probeOutput.Format.BitRate != "" {
		if bitrate, err := strconv.Atoi(probeOutput.Format.BitRate); err == nil {
			info.Bitrate = bitrate
		}
	}

	// Extract file size
	if probeOutput.Format.Size != "" {
		if size, err := strconv.ParseInt(probeOutput.Format.Size, 10, 64); err == nil {
			info.Size = size
		}
	}

	// Initialize stream slices
	info.VideoStreams = []VideoStreamInfo{}
	info.AudioStreams = []AudioStreamInfo{}
	info.SubtitleStreams = []SubtitleStreamInfo{}

	// Process each stream
	for _, stream := range probeOutput.Streams {
		switch stream.CodecType {
		case "video":
			videoStream := p.extractVideoStreamInfo(stream)
			info.VideoStreams = append(info.VideoStreams, videoStream)
			info.HasVideo = true

			// Set legacy fields for backward compatibility (use first video stream)
			if info.VideoCodec == "" {
				info.VideoCodec = videoStream.Codec
				info.Resolution = videoStream.Resolution
			}

		case "audio":
			audioStream := p.extractAudioStreamInfo(stream)
			info.AudioStreams = append(info.AudioStreams, audioStream)
			info.HasAudio = true

			// Set legacy field for backward compatibility (use first audio stream)
			if info.AudioCodec == "" {
				info.AudioCodec = audioStream.Codec
			}

		case "subtitle":
			subtitleStream := p.extractSubtitleStreamInfo(stream)
			info.SubtitleStreams = append(info.SubtitleStreams, subtitleStream)
			info.HasSubtitles = true
		}
	}

	debugLog("SUCCESS: Comprehensive metadata extraction complete for %s - Video streams: %d, Audio streams: %d, Subtitle streams: %d\n",
		filePath, len(info.VideoStreams), len(info.AudioStreams), len(info.SubtitleStreams))

	return info, nil
}

// extractVideoStreamInfo extracts detailed information from a video stream
func (p *FFmpegCorePlugin) extractVideoStreamInfo(stream FFProbeStream) VideoStreamInfo {
	info := VideoStreamInfo{
		Index:          stream.Index,
		Codec:          stream.CodecName,
		CodecLongName:  stream.CodecLongName,
		Profile:        stream.Profile,
		Width:          stream.Width,
		Height:         stream.Height,
		PixelFormat:    stream.PixFmt,
		FieldOrder:     stream.FieldOrder,
		ChromaLocation: stream.ChromaLocation,
	}

	// Extract title from tags
	if stream.Tags != nil {
		if title, exists := stream.Tags["title"]; exists {
			info.Title = title
		}
		if language, exists := stream.Tags["language"]; exists {
			info.Language = language
		}
	}

	// Extract level information
	if stream.Level > 0 {
		info.Level = fmt.Sprintf("%.1f", float64(stream.Level)/10.0) // FFmpeg reports level as integer (e.g., 150 = 15.0)
	}

	// Determine resolution string
	if stream.Width > 0 && stream.Height > 0 {
		if stream.Height == 2160 {
			if stream.Width >= 3840 {
				info.Resolution = "4K"
			} else {
				info.Resolution = "2160p"
			}
		} else if stream.Height == 1440 {
			info.Resolution = "1440p"
		} else if stream.Height == 1080 {
			info.Resolution = "1080p"
		} else if stream.Height == 720 {
			info.Resolution = "720p"
		} else if stream.Height == 480 {
			info.Resolution = "480p"
		} else {
			info.Resolution = fmt.Sprintf("%dx%d", stream.Width, stream.Height)
		}
	}

	// Extract aspect ratio
	if stream.DisplayAspectRatio != "" {
		info.AspectRatio = stream.DisplayAspectRatio
	} else if stream.SampleAspectRatio != "" && stream.Width > 0 && stream.Height > 0 {
		// Calculate display aspect ratio from sample aspect ratio
		info.AspectRatio = p.calculateDisplayAspectRatio(stream.Width, stream.Height, stream.SampleAspectRatio)
	} else if stream.Width > 0 && stream.Height > 0 {
		// Calculate from dimensions
		ratio := float64(stream.Width) / float64(stream.Height)
		if ratio > 1.7 && ratio < 1.8 {
			info.AspectRatio = "16:9"
		} else if ratio > 1.3 && ratio < 1.4 {
			info.AspectRatio = "4:3"
		} else {
			info.AspectRatio = fmt.Sprintf("%.2f:1", ratio)
		}
	}

	// Extract framerate
	if stream.RFrameRate != "" {
		info.Framerate = p.formatFramerate(stream.RFrameRate)
	}
	if stream.AvgFrameRate != "" {
		info.AvgFramerate = p.formatFramerate(stream.AvgFrameRate)
	}

	// Determine interlacing
	if stream.FieldOrder != "" && stream.FieldOrder != "progressive" {
		info.Interlaced = "Yes"
	} else {
		info.Interlaced = "No"
	}

	// Extract bitrate information
	info.Bitrate = stream.BitRate
	info.MaxBitrate = stream.MaxBitRate

	// Extract color information
	info.ColorPrimaries = stream.ColorPrimaries
	info.ColorSpace = stream.ColorSpace
	info.ColorTransfer = stream.ColorTransfer
	info.ColorRange = stream.ColorRange

	// Extract bit depth
	if stream.BitsPerRawSample != "" {
		info.BitDepth = stream.BitsPerRawSample + " bit"
	}

	// Extract codec-specific information
	if stream.Refs > 0 {
		info.ReferenceFrames = stream.Refs
	}
	if stream.IsAvc == "true" {
		info.IsAVC = true
	}
	info.NALLengthSize = stream.NalLengthSize

	// Extract HDR information (basic detection)
	info.HDRFormat = p.detectHDRFormat(stream)

	// Extract stream disposition
	if stream.Disposition != nil {
		if defaultVal, exists := stream.Disposition["default"]; exists && defaultVal == 1 {
			info.Default = true
		}
		if forcedVal, exists := stream.Disposition["forced"]; exists && forcedVal == 1 {
			info.Forced = true
		}
	}

	// Extract duration
	if stream.Duration != "" {
		if duration, err := strconv.ParseFloat(stream.Duration, 64); err == nil {
			info.Duration = duration
		}
	}

	return info
}

// extractAudioStreamInfo extracts detailed information from an audio stream
func (p *FFmpegCorePlugin) extractAudioStreamInfo(stream FFProbeStream) AudioStreamInfo {
	info := AudioStreamInfo{
		Index:         stream.Index,
		Codec:         stream.CodecName,
		CodecLongName: stream.CodecLongName,
		Profile:       stream.Profile,
		Channels:      stream.Channels,
		ChannelLayout: stream.ChannelLayout,
		SampleFormat:  stream.SampleFmt,
		Bitrate:       stream.BitRate,
	}

	// Extract title and language from tags
	if stream.Tags != nil {
		if title, exists := stream.Tags["title"]; exists {
			info.Title = title
			info.EmbeddedTitle = title
		}
		if language, exists := stream.Tags["language"]; exists {
			info.Language = language
		}
	}

	// Extract sample rate
	if stream.SampleRate != "" {
		if sampleRate, err := strconv.Atoi(stream.SampleRate); err == nil {
			info.SampleRate = sampleRate
		}
	}

	// Extract bit depth
	if stream.BitsPerSample > 0 {
		info.BitDepth = stream.BitsPerSample
	}

	// Format channel layout as human-readable
	info.Layout = p.formatChannelLayout(stream.ChannelLayout, stream.Channels)

	// Extract stream disposition
	if stream.Disposition != nil {
		if defaultVal, exists := stream.Disposition["default"]; exists && defaultVal == 1 {
			info.Default = true
		}
		if forcedVal, exists := stream.Disposition["forced"]; exists && forcedVal == 1 {
			info.Forced = true
		}
	}

	// Extract duration
	if stream.Duration != "" {
		if duration, err := strconv.ParseFloat(stream.Duration, 64); err == nil {
			info.Duration = duration
		}
	}

	return info
}

// extractSubtitleStreamInfo extracts information from a subtitle stream
func (p *FFmpegCorePlugin) extractSubtitleStreamInfo(stream FFProbeStream) SubtitleStreamInfo {
	info := SubtitleStreamInfo{
		Index:         stream.Index,
		Codec:         stream.CodecName,
		CodecLongName: stream.CodecLongName,
	}

	// Extract title and language from tags
	if stream.Tags != nil {
		if title, exists := stream.Tags["title"]; exists {
			info.Title = title
		}
		if language, exists := stream.Tags["language"]; exists {
			info.Language = language
		}
	}

	// Extract stream disposition
	if stream.Disposition != nil {
		if defaultVal, exists := stream.Disposition["default"]; exists && defaultVal == 1 {
			info.Default = true
		}
		if forcedVal, exists := stream.Disposition["forced"]; exists && forcedVal == 1 {
			info.Forced = true
		}
	}

	return info
}

// Helper functions for formatting and processing

// calculateDisplayAspectRatio calculates display aspect ratio from sample aspect ratio
func (p *FFmpegCorePlugin) calculateDisplayAspectRatio(width, height int, sar string) string {
	// Parse sample aspect ratio (e.g., "1:1" or "40:33")
	parts := strings.Split(sar, ":")
	if len(parts) != 2 {
		return ""
	}

	sarNum, err1 := strconv.ParseFloat(parts[0], 64)
	sarDen, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || sarDen == 0 {
		return ""
	}

	darRatio := (float64(width) * sarNum / sarDen) / float64(height)

	// Common aspect ratios
	if darRatio > 1.7 && darRatio < 1.8 {
		return "16:9"
	} else if darRatio > 1.3 && darRatio < 1.4 {
		return "4:3"
	} else if darRatio > 2.3 && darRatio < 2.4 {
		return "2.35:1"
	} else {
		return fmt.Sprintf("%.2f:1", darRatio)
	}
}

// formatFramerate converts ffprobe framerate to readable format
func (p *FFmpegCorePlugin) formatFramerate(frameRate string) string {
	parts := strings.Split(frameRate, "/")
	if len(parts) != 2 {
		return frameRate
	}

	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return frameRate
	}

	fps := num / den

	// Format common frame rates nicely
	if fps > 23.9 && fps < 24.1 {
		return "23.976"
	} else if fps > 24.9 && fps < 25.1 {
		return "25"
	} else if fps > 29.9 && fps < 30.1 {
		return "29.97"
	} else if fps > 49.9 && fps < 50.1 {
		return "50"
	} else if fps > 59.9 && fps < 60.1 {
		return "59.94"
	} else {
		return fmt.Sprintf("%.3f", fps)
	}
}

// formatChannelLayout converts channel layout to human-readable format
func (p *FFmpegCorePlugin) formatChannelLayout(layout string, channels int) string {
	if layout == "" {
		return fmt.Sprintf("%d ch", channels)
	}

	// Common layout mappings
	layoutMap := map[string]string{
		"mono":           "1.0",
		"stereo":         "2.0",
		"2.1":            "2.1",
		"3.0":            "3.0",
		"3.0(back)":      "3.0",
		"4.0":            "4.0",
		"quad":           "4.0",
		"quad(side)":     "4.0",
		"3.1":            "3.1",
		"5.0":            "5.0",
		"5.0(side)":      "5.0",
		"4.1":            "4.1",
		"5.1":            "5.1",
		"5.1(side)":      "5.1",
		"6.0":            "6.0",
		"6.0(front)":     "6.0",
		"hexagonal":      "6.0",
		"6.1":            "6.1",
		"6.1(back)":      "6.1",
		"6.1(front)":     "6.1",
		"7.0":            "7.0",
		"7.0(front)":     "7.0",
		"7.1":            "7.1",
		"7.1(wide)":      "7.1",
		"7.1(wide-side)": "7.1",
		"octagonal":      "8.0",
		"downmix":        "2.0",
	}

	if mapped, exists := layoutMap[layout]; exists {
		return mapped
	}

	return layout
}

// detectHDRFormat detects HDR format from stream metadata
func (p *FFmpegCorePlugin) detectHDRFormat(stream FFProbeStream) string {
	// Basic HDR detection based on color characteristics
	if stream.ColorTransfer == "smpte2084" {
		return "HDR10"
	} else if stream.ColorTransfer == "arib-std-b67" {
		return "HLG"
	} else if stream.ColorTransfer == "bt709" && stream.ColorPrimaries == "bt2020" {
		return "HDR10"
	} else if stream.ColorSpace == "bt2020nc" || stream.ColorSpace == "bt2020c" {
		return "HDR"
	}

	return ""
}

// determineContainerFormat determines the container format from ffprobe output
func (p *FFmpegCorePlugin) determineContainerFormat(formatName, filePath string) string {
	// Map ffprobe format names to our standard container names
	containerMap := map[string]string{
		"matroska,webm":           "mkv",
		"mov,mp4,m4a,3gp,3g2,mj2": "mp4",
		"avi":                     "avi",
		"flv":                     "flv",
		"asf":                     "wmv",
		"ogg":                     "ogg",
	}

	// Try exact and partial matches
	lowerFormatName := strings.ToLower(formatName)
	for key, value := range containerMap {
		if formatName == key || strings.Contains(lowerFormatName, key) {
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
