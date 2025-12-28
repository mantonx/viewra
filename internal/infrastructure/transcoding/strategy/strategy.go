// Package strategy provides stream strategy determination for transcoding decisions.
// It analyzes video metadata and client capabilities to determine the optimal
// streaming approach (direct play, remux, or transcode).
package strategy

import (
	"fmt"
	"strings"

	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/videoinfo"
)

// StreamStrategy represents the type of streaming operation needed.
type StreamStrategy string

const (
	// DirectPlay serves the file directly without any processing (instant)
	DirectPlay StreamStrategy = "direct_play"
	// Remux copies streams to HLS container without re-encoding (2-5 min)
	Remux StreamStrategy = "remux"
	// RemuxWithAudioDownmix copies video and downmixes multi-channel audio to stereo (5-10 min)
	RemuxWithAudioDownmix StreamStrategy = "remux_audio"
	// RemuxHEVC copies HEVC video stream to HLS with only audio transcoding (very fast)
	// Used when client supports HEVC but audio needs conversion (e.g., AC3 → AAC)
	RemuxHEVC StreamStrategy = "remux_hevc"
	// Transcode re-encodes incompatible video/audio (20-60 min)
	Transcode StreamStrategy = "transcode"
)

// DisplayName returns a human-readable name for the strategy.
func (s StreamStrategy) DisplayName() string {
	switch s {
	case DirectPlay:
		return "Direct Play"
	case Remux:
		return "Remux"
	case RemuxWithAudioDownmix:
		return "Remux + Audio Transcode"
	case RemuxHEVC:
		return "HEVC Remux"
	case Transcode:
		return "Transcode"
	default:
		return string(s)
	}
}

// VideoInfo is an alias for videoinfo.VideoInfo for strategy determination.
// This allows consumers to use strategy.VideoInfo interchangeably with videoinfo.VideoInfo.
type VideoInfo = videoinfo.VideoInfo

// ClientCapabilities contains codec support info from the client browser.
// This allows the server to make informed decisions about direct play.
type ClientCapabilities struct {
	// SupportedVideoCodecs lists video codecs the client can decode (e.g., "h264", "h265", "vp9", "av1")
	SupportedVideoCodecs []string
	// SupportedContainers lists container formats the client can play (e.g., "mp4", "webm", "matroska")
	SupportedContainers []string
	// SupportsHDRDisplay indicates the client has an HDR-capable display that can show HDR content
	// If true and video codec is supported, HDR content can be remuxed without tone mapping
	SupportsHDRDisplay bool
	// SupportsDolbyVision indicates the client can decode Dolby Vision content
	// This requires both hardware decoder support and display support for DV metadata
	// Most browsers do NOT support DV even if they claim HEVC support
	SupportsDolbyVision bool
}

// DetermineStrategy analyzes video metadata and determines the optimal streaming strategy.
// Returns the strategy and a human-readable reason for the decision.
// This version assumes only H.264 support (legacy behavior).
func DetermineStrategy(videoInfo *VideoInfo) (StreamStrategy, string) {
	return DetermineStrategyWithCapabilities(videoInfo, nil)
}

// DetermineStrategyWithCapabilities analyzes video metadata against client capabilities
// to determine the optimal streaming strategy. When clientCaps is provided, it enables
// direct play for modern codecs (H.265, VP9, AV1) if the client supports them.
func DetermineStrategyWithCapabilities(videoInfo *VideoInfo, clientCaps *ClientCapabilities) (StreamStrategy, string) {
	// Safety check
	if videoInfo == nil {
		return Transcode, "no video info available, defaulting to transcode"
	}

	// Normalize video codec name
	videoCodecLower := strings.ToLower(videoInfo.Codec)

	// Check if client supports the video codec
	isVideoCodecSupported := isCodecSupportedByClient(videoCodecLower, clientCaps)

	// Check container format compatibility
	isWebContainer := isWebCompatibleContainer(videoInfo.ContainerFormat, clientCaps)

	// Check audio codec compatibility (browsers support: AAC, MP3, Opus, Vorbis)
	isWebAudioCodec := isWebCompatibleAudioCodec(videoInfo.AudioCodec)

	// Check audio channel compatibility (stereo or mono is web-compatible, 5.1/7.1 is not)
	isStereoOrLess := videoInfo.AudioChannels <= 2
	hasMultiChannelAudio := videoInfo.AudioChannels > 2

	// Tier 1: Direct Play - supported video codec + web-compatible audio + stereo + web container
	// This is instant, no processing needed
	if isVideoCodecSupported && isWebAudioCodec && isStereoOrLess && isWebContainer {
		return DirectPlay, fmt.Sprintf("%s video with %s %d-channel audio in %s container - direct playback",
			videoInfo.Codec, videoInfo.AudioCodec, videoInfo.AudioChannels, videoInfo.ContainerFormat)
	}

	// Check codec types for remux decisions
	isH264 := videoCodecLower == "h264" || videoCodecLower == "avc1"
	isHEVC := videoCodecLower == "hevc" || videoCodecLower == "h265" || videoCodecLower == "hev1"

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

	// Tier 4: HEVC Remux - Client supports HEVC, copy video stream with bitstream filter
	// This is much faster than full transcode (~50x realtime vs ~1x realtime)
	// Audio still needs transcoding (AC3/DTS → AAC) for browser compatibility
	//
	// HDR handling:
	// - If client has HDR display (SupportsHDRDisplay=true), HDR content can be remuxed directly
	// - If client does NOT have HDR display, HDR content must be transcoded with tone mapping
	//
	// Dolby Vision handling:
	// - Dolby Vision requires explicit decoder support beyond basic HEVC
	// - Most browsers claim HEVC support but CANNOT decode Dolby Vision
	// - If content is DV and client doesn't explicitly support DV, must transcode
	//
	// NOTE: HEVC remux now uses -noaccurate_seek which seeks to the nearest keyframe,
	// avoiding the previous "green blocky video" corruption issue from seeking between keyframes.
	clientHasHDRDisplay := clientCaps != nil && clientCaps.SupportsHDRDisplay
	clientSupportsDV := clientCaps != nil && clientCaps.SupportsDolbyVision

	// Dolby Vision content requires explicit DV support - cannot remux if client doesn't support it
	if videoInfo.IsDolbyVision && !clientSupportsDV {
		return Transcode, fmt.Sprintf("Dolby Vision content requires transcoding (client does not support DV decoding)")
	}

	canRemuxHDR := !videoInfo.IsHDR || clientHasHDRDisplay

	if isHEVC && isVideoCodecSupported && canRemuxHDR {
		if videoInfo.IsDolbyVision && clientSupportsDV {
			return RemuxHEVC, fmt.Sprintf("HEVC Dolby Vision video remuxing to HLS (client supports DV) with %s audio transcode to AAC",
				videoInfo.AudioCodec)
		}
		if videoInfo.IsHDR && clientHasHDRDisplay {
			return RemuxHEVC, fmt.Sprintf("HEVC HDR video remuxing to HLS (client has HDR display) with %s audio transcode to AAC",
				videoInfo.AudioCodec)
		}
		return RemuxHEVC, fmt.Sprintf("HEVC video supported by client, remuxing to HLS with %s audio transcode to AAC",
			videoInfo.AudioCodec)
	}

	// Tier 5: Full Transcode - Incompatible video codec or other issues
	// Re-encode both video and audio (20-60 minutes)
	return Transcode, fmt.Sprintf("video codec %s incompatible, needs full transcode to H.264",
		videoInfo.Codec)
}

// isCodecSupportedByClient checks if the video codec is supported by the client.
// If no client capabilities are provided, only H.264 is assumed to be supported.
func isCodecSupportedByClient(videoCodecLower string, clientCaps *ClientCapabilities) bool {
	// H.264 variants - universally supported
	if videoCodecLower == "h264" || videoCodecLower == "avc1" || videoCodecLower == "avc" {
		return true
	}

	// If no client capabilities provided, only H.264 is safe
	if clientCaps == nil || len(clientCaps.SupportedVideoCodecs) == 0 {
		return false
	}

	// Check if client explicitly supports this codec
	for _, supported := range clientCaps.SupportedVideoCodecs {
		supportedLower := strings.ToLower(supported)

		// H.265/HEVC variants
		if (videoCodecLower == "hevc" || videoCodecLower == "h265" || videoCodecLower == "hev1") &&
			(supportedLower == "h265" || supportedLower == "hevc" || supportedLower == "hev1") {
			return true
		}

		// VP9
		if videoCodecLower == "vp9" && supportedLower == "vp9" {
			return true
		}

		// AV1
		if (videoCodecLower == "av1" || videoCodecLower == "av01") &&
			(supportedLower == "av1" || supportedLower == "av01") {
			return true
		}
	}

	return false
}

// isWebCompatibleContainer checks if the container format is web-compatible.
// Properly parses ffprobe format strings like "mov,mp4,m4a,3gp,3g2,mj2" or "matroska,webm".
// IMPORTANT: FFprobe returns "matroska,webm" for ALL Matroska files (MKV and WebM alike)
// because WebM is technically a subset of Matroska. We cannot distinguish them by container
// alone - we must check codecs at a higher level. So we treat all matroska containers as
// NOT web-compatible and let them go through HLS transcoding.
func isWebCompatibleContainer(containerFormat string, clientCaps *ClientCapabilities) bool {
	containerLower := strings.ToLower(containerFormat)

	// Parse comma-separated format list from ffprobe
	formats := strings.Split(containerLower, ",")

	// Check for Matroska (MKV/WebM) - requires special handling
	// FFprobe returns "matroska,webm" for ALL Matroska files, even MKV with H.264+FLAC
	// We cannot determine true WebM from container format alone - must check codecs
	// So we treat ALL matroska containers as NOT directly playable
	for _, format := range formats {
		format = strings.TrimSpace(format)
		if strings.Contains(format, "matroska") || format == "webm" || format == "mkv" {
			// Client explicitly supports matroska/mkv?
			if clientCaps != nil {
				for _, supported := range clientCaps.SupportedContainers {
					if strings.ToLower(supported) == "matroska" || strings.ToLower(supported) == "mkv" {
						return true
					}
				}
			}
			return false
		}
	}

	// Check for standard web containers
	webFormats := []string{"mp4", "m4v", "m4a", "mov", "3gp", "3g2"}
	for _, format := range formats {
		format = strings.TrimSpace(format)
		for _, webFormat := range webFormats {
			if format == webFormat {
				return true
			}
		}
	}

	return false
}

// isWebCompatibleAudioCodec checks if the audio codec is compatible with HLS/fMP4 streaming.
// Note: This is specifically for HLS compatibility (fragmented MP4 segments), NOT general browser support.
// FLAC is NOT included because browsers cannot play FLAC in fMP4/HLS segments, even though
// they can play standalone FLAC files or FLAC in WebM containers.
func isWebCompatibleAudioCodec(audioCodec string) bool {
	audioCodecLower := strings.ToLower(audioCodec)
	return audioCodecLower == "aac" ||
		audioCodecLower == "mp3" ||
		audioCodecLower == "opus" ||
		audioCodecLower == "vorbis" ||
		strings.Contains(audioCodecLower, "mp4a") || // AAC variants
		strings.Contains(audioCodecLower, "aac") // Other AAC variants like aac_latm
}

// IsWebCompatibleAudioCodec is exported for use by other packages.
func IsWebCompatibleAudioCodec(audioCodec string) bool {
	return isWebCompatibleAudioCodec(audioCodec)
}

// AdaptiveProfile is an alias for profile.AdaptiveProfile for transcoding decisions.
// This allows consumers to use strategy.AdaptiveProfile interchangeably with profile.AdaptiveProfile.
type AdaptiveProfile = profile.AdaptiveProfile

// ShouldTranscode determines if transcoding is necessary based on current codec/resolution vs target.
// Returns true if transcoding is needed, false if source already matches target.
// This function checks VIDEO transcoding needs only - audio is handled separately by DetermineStrategy.
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
