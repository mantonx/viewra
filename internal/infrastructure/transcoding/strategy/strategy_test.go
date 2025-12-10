package strategy

import (
	"testing"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/hls"
)

// vi is a helper to create VideoInfo with embedded hls.VideoInfo fields for tests.
func vi(codec string, width, height int, audioCodec string, audioChannels int, containerFormat string, isHDR bool, bitrate int64) *VideoInfo {
	return &VideoInfo{
		VideoInfo: hls.VideoInfo{
			Codec:           codec,
			Width:           width,
			Height:          height,
			Bitrate:         bitrate,
			AudioCodec:      audioCodec,
			AudioChannels:   audioChannels,
			ContainerFormat: containerFormat,
			IsHDR:           isHDR,
		},
	}
}

func TestDetermineStrategy(t *testing.T) {
	tests := []struct {
		name             string
		videoInfo        *VideoInfo
		expectedStrategy StreamStrategy
		expectedReason   string
	}{
		{"Direct Play - H.264 + stereo + MP4", vi("h264", 1920, 1080, "aac", 2, "mp4", false, 0), DirectPlay, "direct playback"},
		{"Direct Play - AVC1 variant", vi("avc1", 0, 0, "aac", 2, "mp4", false, 0), DirectPlay, "direct playback"},
		{"Direct Play - FFprobe multi-format", vi("h264", 0, 0, "aac", 2, "mov,mp4,m4a,3gp,3g2,mj2", false, 0), DirectPlay, "direct playback"},
		{"Direct Play - Mono MP3", vi("h264", 0, 0, "mp3", 1, "mp4", false, 0), DirectPlay, "direct playback"},

		{"Remux - H.264 + stereo + MKV", vi("h264", 1920, 1080, "aac", 2, "matroska", false, 0), Remux, "container remux"},
		{"Remux - H.264 + stereo + WebM", vi("h264", 1280, 720, "opus", 1, "webm", false, 0), Remux, "container remux"},
		{"Remux - H.264 + MP3 + AVI", vi("h264", 1280, 720, "mp3", 2, "avi", false, 0), Remux, "container remux"},
		{"Remux - matroska,webm format", vi("h264", 0, 0, "aac", 2, "matroska,webm", false, 0), Remux, "container remux"},

		{"Remux with Audio Downmix - 5.1 AAC", vi("h264", 1920, 1080, "aac", 6, "mp4", false, 0), RemuxWithAudioDownmix, "audio needs transcode"},
		{"Remux with Audio Downmix - 7.1 DTS", vi("h264", 3840, 2160, "dts", 8, "matroska,webm", false, 0), RemuxWithAudioDownmix, "audio needs transcode"},
		{"Remux with Audio Downmix - stereo DTS", vi("h264", 0, 0, "dts", 2, "matroska", false, 0), RemuxWithAudioDownmix, "audio needs transcode"},
		{"Remux with Audio Downmix - stereo AC3", vi("h264", 0, 0, "ac3", 2, "mp4", false, 0), RemuxWithAudioDownmix, "audio needs transcode"},
		{"Remux with Audio Downmix - stereo TrueHD", vi("h264", 0, 0, "truehd", 2, "matroska", false, 0), RemuxWithAudioDownmix, "audio needs transcode"},
		{"Remux with Audio Downmix - 4 channels", vi("h264", 0, 0, "aac", 4, "mp4", false, 0), RemuxWithAudioDownmix, "audio needs transcode"},
		{"Remux with Audio Downmix - 3 channels", vi("h264", 0, 0, "aac", 3, "mp4", false, 0), RemuxWithAudioDownmix, "audio needs transcode"},

		{"Transcode - HEVC", vi("hevc", 3840, 2160, "aac", 2, "mp4", false, 0), Transcode, "incompatible"},
		{"Transcode - VP9", vi("vp9", 1920, 1080, "opus", 2, "webm", false, 0), Transcode, "incompatible"},
		{"Transcode - H.265 + 5.1", vi("h265", 3840, 2160, "ac3", 6, "matroska,webm", false, 0), Transcode, "incompatible"},
		{"Transcode - nil VideoInfo", nil, Transcode, "no video info"},
		{"Transcode - empty codec", vi("", 1920, 1080, "", 2, "mp4", false, 0), Transcode, "incompatible"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, reason := DetermineStrategy(tt.videoInfo)
			if strategy != tt.expectedStrategy {
				t.Errorf("strategy = %v, want %v", strategy, tt.expectedStrategy)
			}
			if !contains(reason, tt.expectedReason) {
				t.Errorf("reason = %v, want to contain %v", reason, tt.expectedReason)
			}
		})
	}
}

func TestStreamStrategyConstants(t *testing.T) {
	tests := []struct {
		strategy StreamStrategy
		expected string
	}{
		{DirectPlay, "direct_play"},
		{Remux, "remux"},
		{RemuxWithAudioDownmix, "remux_audio"},
		{RemuxHEVC, "remux_hevc"},
		{Transcode, "transcode"},
	}

	for _, tt := range tests {
		if string(tt.strategy) != tt.expected {
			t.Errorf("StreamStrategy constant mismatch: got %v, want %v", tt.strategy, tt.expected)
		}
	}
}

func TestDetermineStrategyWithCapabilities(t *testing.T) {
	tests := []struct {
		name             string
		videoInfo        *VideoInfo
		clientCaps       *ClientCapabilities
		expectedStrategy StreamStrategy
		expectedReason   string
	}{
		// HEVC remux is now enabled for non-HDR content when client supports HEVC
		{"HEVC remux - client supports, non-HDR", vi("hevc", 0, 0, "ac3", 6, "matroska", false, 0), &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "hevc"}}, RemuxHEVC, "remuxing to HLS"},
		{"HEVC transcode - HDR content", vi("hevc", 0, 0, "ac3", 6, "matroska", true, 0), &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "hevc"}}, Transcode, "incompatible"},
		{"HEVC transcode - no client support", vi("hevc", 0, 0, "ac3", 6, "matroska", false, 0), &ClientCapabilities{SupportedVideoCodecs: []string{"h264"}}, Transcode, "incompatible"},
		{"HEVC transcode - nil caps", vi("hevc", 0, 0, "aac", 2, "mp4", false, 0), nil, Transcode, "incompatible"},
		{"HEVC direct play - client supports", vi("hevc", 0, 0, "aac", 2, "mp4", false, 0), &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "hevc"}}, DirectPlay, "direct playback"},
		{"HEV1 remux - client supports, non-HDR", vi("hev1", 0, 0, "flac", 2, "matroska", false, 0), &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "hevc"}}, RemuxHEVC, "remuxing to HLS"},
		{"H265 direct play - client supports", vi("h265", 0, 0, "aac", 2, "mp4", false, 0), &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "hevc"}}, DirectPlay, "direct playback"},

		{"VP9 direct play - client supports", vi("vp9", 0, 0, "opus", 2, "mp4", false, 0), &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "vp9"}}, DirectPlay, "direct playback"},
		{"VP9 transcode - no client support", vi("vp9", 0, 0, "opus", 2, "webm", false, 0), &ClientCapabilities{SupportedVideoCodecs: []string{"h264"}}, Transcode, "incompatible"},

		{"AV1 direct play - client supports", vi("av1", 3840, 2160, "aac", 2, "mp4", false, 0), &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "av1"}}, DirectPlay, "direct playback"},
		{"AV1 transcode - no client support", vi("av1", 0, 0, "opus", 2, "mp4", false, 0), &ClientCapabilities{SupportedVideoCodecs: []string{"h264"}}, Transcode, "incompatible"},
		{"AV01 direct play - client supports", vi("av01", 0, 0, "aac", 2, "mp4", false, 0), &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "av1"}}, DirectPlay, "direct playback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, reason := DetermineStrategyWithCapabilities(tt.videoInfo, tt.clientCaps)
			if strategy != tt.expectedStrategy {
				t.Errorf("strategy = %v, want %v", strategy, tt.expectedStrategy)
			}
			if !contains(reason, tt.expectedReason) {
				t.Errorf("reason = %v, want to contain %v", reason, tt.expectedReason)
			}
		})
	}
}

func TestIsCodecSupportedByClient(t *testing.T) {
	tests := []struct {
		name       string
		codec      string
		clientCaps *ClientCapabilities
		expected   bool
	}{
		{"H.264 always supported", "h264", nil, true},
		{"AVC1 always supported", "avc1", nil, true},
		{"AVC always supported", "avc", nil, true},

		{"HEVC not supported without caps", "hevc", nil, false},
		{"HEVC supported with h265 in caps", "hevc", &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "h265"}}, true},
		{"H265 supported with hevc in caps", "h265", &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "hevc"}}, true},
		{"HEV1 supported with h265 in caps", "hev1", &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "h265"}}, true},

		{"VP9 not supported without caps", "vp9", nil, false},
		{"VP9 supported with caps", "vp9", &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "vp9"}}, true},

		{"AV1 not supported without caps", "av1", nil, false},
		{"AV1 supported with caps", "av1", &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "av1"}}, true},
		{"AV01 supported with av1 in caps", "av01", &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "av1"}}, true},

		{"Case insensitive HEVC", "hevc", &ClientCapabilities{SupportedVideoCodecs: []string{"H264", "H265"}}, true},
		{"Case insensitive VP9", "vp9", &ClientCapabilities{SupportedVideoCodecs: []string{"H264", "VP9"}}, true},
		{"Case insensitive AV1", "av1", &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "AV1"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCodecSupportedByClient(tt.codec, tt.clientCaps)
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsWebCompatibleContainer(t *testing.T) {
	tests := []struct {
		name       string
		container  string
		clientCaps *ClientCapabilities
		expected   bool
	}{
		{"MP4", "mp4", nil, true},
		{"MOV", "mov", nil, true},
		{"M4V", "m4v", nil, true},
		{"M4A", "m4a", nil, true},
		{"3GP", "3gp", nil, true},
		{"3G2", "3g2", nil, true},
		{"FFprobe multi-format MP4", "mov,mp4,m4a,3gp,3g2,mj2", nil, true},
		{"Case insensitive MP4", "MP4", nil, true},
		{"Case insensitive MOV", "MOV", nil, true},

		{"MKV not web-compatible", "matroska,webm", nil, false},
		{"WebM not web-compatible", "webm", nil, false},
		{"Matroska not web-compatible", "matroska", nil, false},
		{"MKV literal not web-compatible", "mkv", nil, false},
		{"AVI not web-compatible", "avi", nil, false},
		{"FLV not web-compatible", "flv", nil, false},
		{"MPEG-TS not web-compatible", "mpegts", nil, false},

		{"MKV with explicit caps", "matroska,webm", &ClientCapabilities{SupportedContainers: []string{"matroska"}}, true},
		{"MKV literal with caps", "mkv", &ClientCapabilities{SupportedContainers: []string{"mkv"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isWebCompatibleContainer(tt.container, tt.clientCaps)
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsWebCompatibleAudioCodec(t *testing.T) {
	tests := []struct {
		name     string
		codec    string
		expected bool
	}{
		{"AAC", "aac", true},
		{"MP3", "mp3", true},
		{"Opus", "opus", true},
		{"Vorbis", "vorbis", true},
		{"MP4A (AAC variant)", "mp4a", true},
		{"AAC_LATM", "aac_latm", true},
		{"mp4a.40.2 variant", "mp4a.40.2", true},
		{"Case insensitive AAC", "AAC", true},
		{"Case insensitive MP3", "MP3", true},
		{"Case insensitive Opus", "OPUS", true},

		{"FLAC not web-compatible", "flac", false},
		{"AC3 not web-compatible", "ac3", false},
		{"EAC3 not web-compatible", "eac3", false},
		{"DTS not web-compatible", "dts", false},
		{"DTS-HD not web-compatible", "dts-hd", false},
		{"TrueHD not web-compatible", "truehd", false},
		{"PCM not web-compatible", "pcm_s16le", false},
		{"PCM big-endian not web-compatible", "pcm_s16be", false},
		{"PCM 24-bit not web-compatible", "pcm_s24le", false},
		{"Case insensitive AC3", "AC3", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isWebCompatibleAudioCodec(tt.codec)
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsWebCompatibleAudioCodec_Exported(t *testing.T) {
	// The exported function should behave identically to the internal one
	// Just verify it works for a few key codecs
	tests := []struct {
		codec    string
		expected bool
	}{
		{"aac", true},
		{"ac3", false},
		{"truehd", false},
	}

	for _, tt := range tests {
		t.Run(tt.codec, func(t *testing.T) {
			result := IsWebCompatibleAudioCodec(tt.codec)
			if result != tt.expected {
				t.Errorf("IsWebCompatibleAudioCodec(%q) = %v, want %v", tt.codec, result, tt.expected)
			}
		})
	}
}

func TestShouldTranscode(t *testing.T) {
	tests := []struct {
		name            string
		videoInfo       *VideoInfo
		profile         *AdaptiveProfile
		shouldTranscode bool
		reasonContains  string
	}{
		{"Nil video info", nil, &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, true, "unable to determine"},
		{"Empty codec", vi("", 1920, 1080, "", 0, "", false, 0), &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, true, "unable to determine"},
		{"Non-H264 codec", vi("hevc", 3840, 2160, "", 0, "", false, 20000000), &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, true, "needs transcoding to H.264"},
		{"H264 lower resolution", vi("h264", 1280, 720, "", 0, "", false, 3000000), &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, false, "lower than target"},
		{"H264 matching resolution", vi("h264", 1920, 1080, "", 0, "", false, 5000000), &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, false, "already matches target"},
		{"H264 lower bitrate", vi("h264", 2560, 1440, "", 0, "", false, 2000000), &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, false, "already lower than target"},
		{"H264 needs downscaling", vi("h264", 3840, 2160, "", 0, "", false, 20000000), &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, true, "transcoding from h264"},
		{"H264 multi-channel audio", vi("h264", 1920, 1080, "ac3", 6, "", false, 5000000), &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, true, "audio needs processing"},
		{"H264 FLAC audio", vi("h264", 1920, 1080, "flac", 2, "", false, 3000000), &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, true, "audio needs processing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldTranscode, reason := ShouldTranscode(tt.videoInfo, tt.profile)
			if shouldTranscode != tt.shouldTranscode {
				t.Errorf("got %v, want %v", shouldTranscode, tt.shouldTranscode)
			}
			if !contains(reason, tt.reasonContains) {
				t.Errorf("reason = %v, want to contain %v", reason, tt.reasonContains)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
