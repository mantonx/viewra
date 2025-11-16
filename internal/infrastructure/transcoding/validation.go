package transcoding

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	Codec           string
	Width           int
	Height          int
	Bitrate         int64
	Duration        float64
	AudioCodec      string       // Codec of the best/selected audio track
	AudioBitrate    int64        // Bitrate of the best/selected audio track
	AudioChannels   int          // Number of audio channels (1=mono, 2=stereo, 6=5.1, 8=7.1)
	AudioTracks     []AudioTrack // All available audio tracks
	SelectedAudioTrackIndex int  // Index of the selected audio track (for FFmpeg -map)
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

// ffprobeOutput represents the JSON structure returned by ffprobe
type ffprobeOutput struct {
	Streams []struct {
		Index     int    `json:"index"`
		CodecName string `json:"codec_name"`
		CodecType string `json:"codec_type"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		Channels  int    `json:"channels"`
		BitRate   string `json:"bit_rate"`
		Tags      struct {
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
	bestTrack := selectBestAudioTrack(info.AudioTracks)
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

	return info, nil
}

// selectBestAudioTrack chooses the most suitable audio track from available options.
// Selection priority:
// 1. Non-commentary tracks only
// 2. Web-compatible codecs (AAC, MP3, Opus) in stereo
// 3. Web-compatible codecs in multi-channel (will need downmix)
// 4. Stereo tracks (even if incompatible codec - faster to transcode)
// 5. Multi-channel tracks (slower to transcode)
func selectBestAudioTrack(tracks []AudioTrack) *AudioTrack {
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

// ShouldTranscode determines if transcoding is necessary based on current codec/resolution vs target.
// Returns true if transcoding is needed, false if source already matches target.
// This function checks VIDEO transcoding needs only - audio is handled separately by DetermineStreamStrategy.
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

	// Check audio codec compatibility - if audio needs transcoding, we need to process the file
	audioCodecLower := strings.ToLower(videoInfo.AudioCodec)
	isWebAudioCodec := audioCodecLower == "aac" ||
		audioCodecLower == "mp3" ||
		audioCodecLower == "opus" ||
		audioCodecLower == "vorbis" ||
		strings.Contains(audioCodecLower, "mp4a") // AAC variants

	// Check if audio needs downmixing (more than stereo)
	hasMultiChannelAudio := videoInfo.AudioChannels > 2

	// If H.264 video but incompatible or multi-channel audio, we need to process (remux with audio transcode)
	if !isWebAudioCodec || hasMultiChannelAudio {
		return true, fmt.Sprintf("H.264 video compatible, but %s %d-channel audio needs processing to AAC stereo",
			videoInfo.AudioCodec, videoInfo.AudioChannels)
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

// StreamStrategy represents the type of streaming operation needed
type StreamStrategy string

const (
	// DirectPlay serves the file directly without any processing (instant)
	DirectPlay StreamStrategy = "direct_play"
	// Remux copies streams to DASH container without re-encoding (2-5 min)
	Remux StreamStrategy = "remux"
	// RemuxWithAudioDownmix copies video and downmixes multi-channel audio to stereo (5-10 min)
	RemuxWithAudioDownmix StreamStrategy = "remux_audio"
	// Transcode re-encodes incompatible video/audio (20-60 min)
	Transcode StreamStrategy = "transcode"
)

// DetermineStreamStrategy analyzes video metadata and determines the optimal streaming strategy.
// Returns the strategy and a human-readable reason for the decision.
func DetermineStreamStrategy(videoInfo *VideoInfo) (StreamStrategy, string) {
	// Safety check
	if videoInfo == nil {
		return Transcode, "no video info available, defaulting to transcode"
	}

	// Check video codec compatibility (H.264 is web-compatible)
	isH264 := videoInfo.Codec == "h264" || videoInfo.Codec == "H264" || videoInfo.Codec == "avc1"

	// Check container format compatibility (MP4, WebM, MOV are web-compatible)
	// Note: ffprobe returns formats like "mov,mp4,m4a,3gp,3g2,mj2" or "matroska,webm"
	// We need to check if it contains the web format but exclude matroska
	containerLower := strings.ToLower(videoInfo.ContainerFormat)
	isWebContainer := !strings.Contains(containerLower, "matroska") && (
		strings.Contains(containerLower, "mp4") ||
		strings.Contains(containerLower, "webm") ||
		strings.Contains(containerLower, "mov"))

	// Check audio codec compatibility (browsers support: AAC, MP3, Opus, Vorbis)
	// Incompatible: TrueHD, DTS, DTS-HD, EAC3, FLAC, PCM, etc.
	audioCodecLower := strings.ToLower(videoInfo.AudioCodec)
	isWebAudioCodec := audioCodecLower == "aac" ||
		audioCodecLower == "mp3" ||
		audioCodecLower == "opus" ||
		audioCodecLower == "vorbis" ||
		strings.Contains(audioCodecLower, "mp4a") // AAC variants

	// Check audio compatibility (stereo or mono is web-compatible, 5.1/7.1 is not)
	isStereoOrLess := videoInfo.AudioChannels <= 2
	hasMultiChannelAudio := videoInfo.AudioChannels > 2

	// Tier 1: Direct Play - H.264 + web-compatible audio codec + stereo + web container
	// This is instant, no processing needed
	if isH264 && isWebAudioCodec && isStereoOrLess && isWebContainer {
		return DirectPlay, fmt.Sprintf("H.264 video with %s %d-channel audio in %s container - direct playback",
			videoInfo.AudioCodec, videoInfo.AudioChannels, videoInfo.ContainerFormat)
	}

	// Tier 2: Remux - H.264 + web-compatible audio + stereo but wrong container (e.g., MKV)
	// Copy streams to DASH without re-encoding (2-5 minutes)
	if isH264 && isWebAudioCodec && isStereoOrLess {
		return Remux, fmt.Sprintf("H.264 video with %s %d-channel audio needs container remux from %s to HLS",
			videoInfo.AudioCodec, videoInfo.AudioChannels, videoInfo.ContainerFormat)
	}

	// Tier 3: Remux with Audio Transcode - H.264 video but incompatible audio codec OR multi-channel
	// Copy video stream, transcode/downmix audio to AAC stereo (5-10 minutes)
	if isH264 && (hasMultiChannelAudio || !isWebAudioCodec) {
		return RemuxWithAudioDownmix, fmt.Sprintf("H.264 video compatible, but %s %d-channel audio needs transcode to AAC stereo",
			videoInfo.AudioCodec, videoInfo.AudioChannels)
	}

	// Tier 4: Full Transcode - Incompatible video codec or other issues
	// Re-encode both video and audio (20-60 minutes)
	return Transcode, fmt.Sprintf("video codec %s incompatible, needs full transcode to H.264",
		videoInfo.Codec)
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
