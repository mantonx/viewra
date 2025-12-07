// Package probe provides video file analysis using ffprobe.
//
// This package extracts metadata from video files including:
//   - Video codec, resolution, bitrate
//   - Audio tracks with codec, channels, language
//   - HDR and color space information
//   - Container format and duration
//
// Results are cached to avoid repeated ffprobe calls for the same file.
package probe

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AudioTrack represents a single audio track in a video file
type AudioTrack struct {
	Index        int    // Stream index in the file
	Codec        string // Audio codec (aac, ac3, dts, etc.)
	Channels     int    // Number of channels (1=mono, 2=stereo, 6=5.1, etc.)
	Bitrate      int64  // Audio bitrate in bits per second
	Language     string // Track language (eng, jpn, etc.)
	Title        string // Track title/description
	IsCommentary bool   // True if this is a commentary track
}

// VideoInfo contains metadata about a video file extracted via ffprobe.
type VideoInfo struct {
	Codec                   string
	Width                   int
	Height                  int
	Bitrate                 int64
	Duration                float64
	AudioCodec              string       // Codec of the best/selected audio track
	AudioBitrate            int64        // Bitrate of the best/selected audio track
	AudioChannels           int          // Number of audio channels (1=mono, 2=stereo, 6=5.1, 8=7.1)
	AudioTracks             []AudioTrack // All available audio tracks
	SelectedAudioTrackIndex int          // Index of the selected audio track (for FFmpeg -map)
	ContainerFormat         string

	// HDR and color space metadata
	PixelFormat    string // e.g., "yuv420p" (8-bit), "yuv420p10le" (10-bit), "yuv420p12le" (12-bit)
	ColorSpace     string // e.g., "bt709" (SDR), "bt2020nc" (HDR)
	ColorPrimaries string // e.g., "bt709" (SDR), "bt2020" (HDR)
	ColorTransfer  string // e.g., "bt709" (SDR), "smpte2084" (HDR10/PQ), "arib-std-b67" (HLG)
	BitDepth       int    // 8, 10, or 12 bits per channel
	IsHDR          bool   // True if video is HDR (computed from color metadata)
}

// videoInfoCache provides thread-safe caching of video metadata to avoid repeated ffprobe calls.
// Cache entries are keyed by file path and invalidated if the file's modification time changes.
type videoInfoCache struct {
	mu      sync.RWMutex
	entries map[string]*videoInfoCacheEntry
}

type videoInfoCacheEntry struct {
	info     *VideoInfo
	modTime  time.Time // File modification time when cached
	cachedAt time.Time // When this entry was created
}

// GlobalCache is the shared cache instance for video info lookups.
var GlobalCache = &videoInfoCache{
	entries: make(map[string]*videoInfoCacheEntry),
}

// cacheMaxAge is the maximum age for cache entries (24 hours)
const cacheMaxAge = 24 * time.Hour

// Get retrieves a cached VideoInfo if valid, or nil if not cached/stale.
func (c *videoInfoCache) Get(path string) *VideoInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[path]
	if !exists {
		return nil
	}

	// Check if cache entry is too old
	if time.Since(entry.cachedAt) > cacheMaxAge {
		return nil
	}

	// Check if file has been modified since caching
	stat, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if !stat.ModTime().Equal(entry.modTime) {
		return nil
	}

	return entry.info
}

// Set stores a VideoInfo in the cache.
func (c *videoInfoCache) Set(path string, info *VideoInfo) {
	stat, err := os.Stat(path)
	if err != nil {
		return // Don't cache if we can't stat the file
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[path] = &videoInfoCacheEntry{
		info:     info,
		modTime:  stat.ModTime(),
		cachedAt: time.Now(),
	}
}

// Clear removes all entries from the cache.
func (c *videoInfoCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*videoInfoCacheEntry)
}

// Size returns the number of entries in the cache.
func (c *videoInfoCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// ffprobeOutput represents the JSON structure returned by ffprobe
type ffprobeOutput struct {
	Streams []struct {
		Index            int    `json:"index"`
		CodecName        string `json:"codec_name"`
		CodecType        string `json:"codec_type"`
		Width            int    `json:"width"`
		Height           int    `json:"height"`
		Channels         int    `json:"channels"`
		BitRate          string `json:"bit_rate"`
		PixFmt           string `json:"pix_fmt"`             // Pixel format (yuv420p, yuv420p10le, etc.)
		ColorSpace       string `json:"color_space"`         // bt709, bt2020nc, etc.
		ColorPrimaries   string `json:"color_primaries"`     // bt709, bt2020, etc.
		ColorTransfer    string `json:"color_transfer"`      // bt709, smpte2084 (HDR10), arib-std-b67 (HLG)
		BitsPerRawSample string `json:"bits_per_raw_sample"` // Bit depth as string
		Tags             struct {
			Language string `json:"language"`
			Title    string `json:"title"`
		} `json:"tags"`
	} `json:"streams"`
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
}

// GetVideoInfo extracts video metadata using ffprobe.
// Results are cached to avoid repeated ffprobe calls for the same file.
func GetVideoInfo(inputPath string) (*VideoInfo, error) {
	// Check cache first
	if cached := GlobalCache.Get(inputPath); cached != nil {
		return cached, nil
	}

	// Check for custom FFprobe path (respects VIEWRA_FFPROBE_PATH)
	ffprobePath := os.Getenv("VIEWRA_FFPROBE_PATH")
	if ffprobePath == "" {
		var err error
		ffprobePath, err = exec.LookPath("ffprobe")
		if err != nil {
			return nil, fmt.Errorf("ffprobe not found: %w", err)
		}
	}

	// Get library path for custom FFmpeg/FFprobe builds
	libPath := os.Getenv("VIEWRA_FFMPEG_LIB_PATH")

	// Run ffprobe with JSON output
	cmd := exec.Command(ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		inputPath,
	)

	// Set LD_LIBRARY_PATH for custom FFmpeg/FFprobe builds
	if libPath != "" {
		cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+libPath)
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	// Parse JSON output properly
	var ffprobe ffprobeOutput
	if err := json.Unmarshal(output, &ffprobe); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	info := &VideoInfo{
		AudioTracks: []AudioTrack{},
	}

	// Extract video stream info (first video stream)
	for _, stream := range ffprobe.Streams {
		if stream.CodecType == "video" && info.Codec == "" {
			info.Codec = stream.CodecName
			info.Width = stream.Width
			info.Height = stream.Height

			// Extract HDR and color space metadata
			info.PixelFormat = stream.PixFmt
			info.ColorSpace = stream.ColorSpace
			info.ColorPrimaries = stream.ColorPrimaries
			info.ColorTransfer = stream.ColorTransfer

			// Parse bit depth
			if stream.BitsPerRawSample != "" {
				info.BitDepth, _ = strconv.Atoi(stream.BitsPerRawSample)
			} else {
				// Fallback: detect from pixel format
				info.BitDepth = DetectBitDepthFromPixelFormat(stream.PixFmt)
			}

			// Determine if content is HDR
			info.IsHDR = IsHDRContent(info.ColorTransfer, info.ColorPrimaries)
		}
		if stream.CodecType == "audio" {
			// Collect ALL audio tracks
			bitrate, _ := strconv.ParseInt(stream.BitRate, 10, 64)

			// Check if this is a commentary track
			titleLower := strings.ToLower(stream.Tags.Title)
			isCommentary := strings.Contains(titleLower, "commentary") ||
				strings.Contains(titleLower, "comment")

			track := AudioTrack{
				Index:        stream.Index,
				Codec:        stream.CodecName,
				Channels:     stream.Channels,
				Bitrate:      bitrate,
				Language:     stream.Tags.Language,
				Title:        stream.Tags.Title,
				IsCommentary: isCommentary,
			}
			info.AudioTracks = append(info.AudioTracks, track)
		}
	}

	// Select the best audio track (non-commentary, web-compatible, prefer stereo)
	bestTrack := SelectBestAudioTrack(info.AudioTracks)
	if bestTrack != nil {
		info.AudioCodec = bestTrack.Codec
		info.AudioChannels = bestTrack.Channels
		info.AudioBitrate = bestTrack.Bitrate
		info.SelectedAudioTrackIndex = bestTrack.Index
	} else if len(info.AudioTracks) > 0 {
		// Fallback to first track if no best track found
		info.AudioCodec = info.AudioTracks[0].Codec
		info.AudioChannels = info.AudioTracks[0].Channels
		info.AudioBitrate = info.AudioTracks[0].Bitrate
		info.SelectedAudioTrackIndex = info.AudioTracks[0].Index
	}

	// Extract format info
	info.ContainerFormat = ffprobe.Format.FormatName
	if ffprobe.Format.Duration != "" {
		info.Duration, _ = strconv.ParseFloat(ffprobe.Format.Duration, 64)
	}
	if ffprobe.Format.BitRate != "" {
		info.Bitrate, _ = strconv.ParseInt(ffprobe.Format.BitRate, 10, 64)
	}

	// Cache the result for future requests
	GlobalCache.Set(inputPath, info)

	return info, nil
}

// SelectBestAudioTrack chooses the most suitable audio track from available options.
//
// Selection priority:
//  1. Non-commentary tracks only
//  2. Web-compatible codecs (AAC, MP3, Opus) in stereo
//  3. Web-compatible codecs in multi-channel (will need downmix)
//  4. Stereo tracks (even if incompatible codec - faster to transcode)
//  5. Multi-channel tracks (slower to transcode)
//
// Examples:
//
//	// Movie with multiple audio tracks:
//	tracks := []AudioTrack{
//	    {Index: 1, Codec: "aac", Channels: 2, Title: "English"},        // ✅ Priority 1: Web codec + stereo
//	    {Index: 2, Codec: "ac3", Channels: 6, Title: "English 5.1"},    // Priority 3: Needs downmix
//	    {Index: 3, Codec: "aac", Channels: 2, Title: "Commentary"},     // Skipped: Commentary
//	}
//	selected := SelectBestAudioTrack(tracks) // Returns track 1 (English AAC stereo)
//
//	// Movie with only surround sound:
//	tracks := []AudioTrack{
//	    {Index: 1, Codec: "dts", Channels: 8, Title: "English 7.1"},    // Priority 4: Multi-channel
//	}
//	selected := SelectBestAudioTrack(tracks) // Returns track 1 (will need transcode + downmix)
//
//	// Movie with commentary as only track:
//	tracks := []AudioTrack{
//	    {Index: 1, Codec: "aac", Channels: 2, Title: "Director Commentary"},
//	}
//	selected := SelectBestAudioTrack(tracks) // Returns track 1 (commentary is better than no audio)
func SelectBestAudioTrack(tracks []AudioTrack) *AudioTrack {
	if len(tracks) == 0 {
		return nil
	}

	// Filter out commentary tracks
	nonCommentary := []AudioTrack{}
	for _, track := range tracks {
		if !track.IsCommentary {
			nonCommentary = append(nonCommentary, track)
		}
	}

	// If all tracks are commentary, use first track
	if len(nonCommentary) == 0 {
		return &tracks[0]
	}

	// Helper function to check if codec is web-compatible
	isWebCodec := func(codec string) bool {
		codecLower := strings.ToLower(codec)
		return codecLower == "aac" ||
			codecLower == "mp3" ||
			codecLower == "opus" ||
			codecLower == "vorbis" ||
			strings.Contains(codecLower, "mp4a") // AAC variants
	}

	// Priority 1: Web-compatible codec in stereo (perfect - no processing needed)
	for _, track := range nonCommentary {
		if isWebCodec(track.Codec) && track.Channels <= 2 {
			return &track
		}
	}

	// Priority 2: Web-compatible codec in multi-channel (needs downmix only)
	for _, track := range nonCommentary {
		if isWebCodec(track.Codec) && track.Channels > 2 {
			return &track
		}
	}

	// Priority 3: Stereo track with any codec (faster transcode - no downmix)
	for _, track := range nonCommentary {
		if track.Channels <= 2 {
			return &track
		}
	}

	// Priority 4: Multi-channel track (slowest - needs codec transcode + downmix)
	// Just return the first non-commentary multi-channel track
	return &nonCommentary[0]
}

// DetectBitDepthFromPixelFormat determines bit depth from pixel format string.
// FFmpeg pixel format naming convention includes bit depth for >8-bit formats.
//
// Examples:
//   - yuv420p → 8-bit (no suffix)
//   - yuv420p10le → 10-bit (10 suffix)
//   - yuv420p12le → 12-bit (12 suffix)
//   - yuv420p16le → 16-bit (16 suffix, rare)
func DetectBitDepthFromPixelFormat(pixFmt string) int {
	if strings.Contains(pixFmt, "16") {
		return 16
	} else if strings.Contains(pixFmt, "12") {
		return 12
	} else if strings.Contains(pixFmt, "10") {
		return 10
	}
	return 8
}

// IsHDRContent determines if video content is HDR based on color metadata.
// HDR detection criteria:
//  1. HDR10 (most common): smpte2084 transfer function (PQ curve)
//  2. HLG: arib-std-b67 transfer function (Hybrid Log-Gamma)
//  3. BT.2020 color primaries with non-SDR transfer (catches other HDR variants)
//
// Examples:
//   - HDR10: transfer="smpte2084", primaries="bt2020" → true
//   - HLG:   transfer="arib-std-b67", primaries="bt2020" → true
//   - SDR:   transfer="bt709", primaries="bt709" → false
//   - SDR UHD: transfer="bt709", primaries="bt2020" → false (SDR in wide gamut)
func IsHDRContent(colorTransfer, colorPrimaries string) bool {
	// HDR10 uses PQ (Perceptual Quantizer) transfer function
	if colorTransfer == "smpte2084" {
		return true
	}

	// HLG (Hybrid Log-Gamma) for broadcast HDR
	if colorTransfer == "arib-std-b67" {
		return true
	}

	// BT.2020 color space with non-SDR transfer
	// (Catches HDR variants that use BT.2020 but different transfer functions)
	if colorPrimaries == "bt2020" && colorTransfer != "bt709" && colorTransfer != "" {
		return true
	}

	return false
}
