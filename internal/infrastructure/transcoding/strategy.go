package transcoding

import (
	"fmt"
	"strings"
)

// StreamStrategy represents the type of streaming operation needed
type StreamStrategy string

const (
	// DirectPlay serves the file directly without any processing (instant)
	DirectPlay StreamStrategy = "direct_play"
	// Remux copies streams to HLS container without re-encoding (2-5 min)
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
	isWebContainer := !strings.Contains(containerLower, "matroska") && (strings.Contains(containerLower, "mp4") ||
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
	// Copy streams to HLS without re-encoding (2-5 minutes)
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

// ShouldTranscode determines if transcoding is necessary based on current codec/resolution vs target.
// Returns true if transcoding is needed, false if source already matches target.
// This function checks VIDEO transcoding needs only - audio is handled separately by DetermineStreamStrategy.
func ShouldTranscode(videoInfo *VideoInfo, profile *AdaptiveProfile) (bool, string) {
	// Always transcode if we can't determine source codec/resolution
	if videoInfo == nil || videoInfo.Codec == "" {
		return true, "unable to determine source codec"
	}

	// Check if already H.264
	isH264 := videoInfo.Codec == "h264" || videoInfo.Codec == "H264" || videoInfo.Codec == "avc1"
	if !isH264 {
		return true, fmt.Sprintf("source codec %s needs transcoding to H.264", videoInfo.Codec)
	}

	// Check audio codec compatibility only if audio info is available
	// If AudioChannels is 0, assume no audio info and skip audio checks
	if videoInfo.AudioChannels > 0 {
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
		// AdaptiveProfile uses integer bitrates (bits per second)
		if videoInfo.Bitrate < int64(profile.VideoBitrate) {
			return false, fmt.Sprintf("source bitrate %d is already lower than target %d",
				videoInfo.Bitrate, profile.VideoBitrate)
		}
	}

	// Needs transcoding
	return true, fmt.Sprintf("transcoding from %s %dx%d to H.264 %dx%d",
		videoInfo.Codec, videoInfo.Width, videoInfo.Height, profile.Width, profile.Height)
}
