package strategy

import (
	"testing"
)

func TestDetermineStrategy(t *testing.T) {
	tests := []struct {
		name             string
		videoInfo        *VideoInfo
		expectedStrategy StreamStrategy
		expectedReason   string
	}{
		{"Direct Play - H.264 + stereo + MP4", &VideoInfo{Codec: "h264", Width: 1920, Height: 1080, AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "mp4"}, DirectPlay, "direct playback"},
		{"Direct Play - AVC1 variant", &VideoInfo{Codec: "avc1", AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "mp4"}, DirectPlay, "direct playback"},
		{"Direct Play - FFprobe multi-format", &VideoInfo{Codec: "h264", AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "mov,mp4,m4a,3gp,3g2,mj2"}, DirectPlay, "direct playback"},
		{"Direct Play - Mono MP3", &VideoInfo{Codec: "h264", AudioCodec: "mp3", AudioChannels: 1, ContainerFormat: "mp4"}, DirectPlay, "direct playback"},

		{"Remux - H.264 + stereo + MKV", &VideoInfo{Codec: "h264", Width: 1920, Height: 1080, AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "matroska"}, Remux, "container remux"},
		{"Remux - H.264 + stereo + WebM", &VideoInfo{Codec: "h264", Width: 1280, Height: 720, AudioCodec: "opus", AudioChannels: 1, ContainerFormat: "webm"}, Remux, "container remux"},
		{"Remux - H.264 + MP3 + AVI", &VideoInfo{Codec: "h264", Width: 1280, Height: 720, AudioCodec: "mp3", AudioChannels: 2, ContainerFormat: "avi"}, Remux, "container remux"},
		{"Remux - matroska,webm format", &VideoInfo{Codec: "h264", AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "matroska,webm"}, Remux, "container remux"},

		{"Remux with Audio Downmix - 5.1 AAC", &VideoInfo{Codec: "h264", Width: 1920, Height: 1080, AudioCodec: "aac", AudioChannels: 6, ContainerFormat: "mp4"}, RemuxWithAudioDownmix, "audio needs transcode"},
		{"Remux with Audio Downmix - 7.1 DTS", &VideoInfo{Codec: "h264", Width: 3840, Height: 2160, AudioCodec: "dts", AudioChannels: 8, ContainerFormat: "matroska,webm"}, RemuxWithAudioDownmix, "audio needs transcode"},
		{"Remux with Audio Downmix - stereo DTS", &VideoInfo{Codec: "h264", AudioCodec: "dts", AudioChannels: 2, ContainerFormat: "matroska"}, RemuxWithAudioDownmix, "audio needs transcode"},
		{"Remux with Audio Downmix - stereo AC3", &VideoInfo{Codec: "h264", AudioCodec: "ac3", AudioChannels: 2, ContainerFormat: "mp4"}, RemuxWithAudioDownmix, "audio needs transcode"},
		{"Remux with Audio Downmix - stereo TrueHD", &VideoInfo{Codec: "h264", AudioCodec: "truehd", AudioChannels: 2, ContainerFormat: "matroska"}, RemuxWithAudioDownmix, "audio needs transcode"},
		{"Remux with Audio Downmix - 4 channels", &VideoInfo{Codec: "h264", AudioCodec: "aac", AudioChannels: 4, ContainerFormat: "mp4"}, RemuxWithAudioDownmix, "audio needs transcode"},
		{"Remux with Audio Downmix - 3 channels", &VideoInfo{Codec: "h264", AudioCodec: "aac", AudioChannels: 3, ContainerFormat: "mp4"}, RemuxWithAudioDownmix, "audio needs transcode"},

		{"Transcode - HEVC", &VideoInfo{Codec: "hevc", Width: 3840, Height: 2160, AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "mp4"}, Transcode, "incompatible"},
		{"Transcode - VP9", &VideoInfo{Codec: "vp9", Width: 1920, Height: 1080, AudioCodec: "opus", AudioChannels: 2, ContainerFormat: "webm"}, Transcode, "incompatible"},
		{"Transcode - H.265 + 5.1", &VideoInfo{Codec: "h265", Width: 3840, Height: 2160, AudioCodec: "ac3", AudioChannels: 6, ContainerFormat: "matroska,webm"}, Transcode, "incompatible"},
		{"Transcode - nil VideoInfo", nil, Transcode, "no video info"},
		{"Transcode - empty codec", &VideoInfo{Codec: "", Width: 1920, Height: 1080, AudioChannels: 2, ContainerFormat: "mp4"}, Transcode, "incompatible"},
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
		{"HEVC remux - client supports, non-HDR", &VideoInfo{Codec: "hevc", AudioCodec: "ac3", AudioChannels: 6, ContainerFormat: "matroska", IsHDR: false}, &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "hevc"}}, RemuxHEVC, "remuxing to HLS"},
		{"HEVC transcode - HDR content", &VideoInfo{Codec: "hevc", AudioCodec: "ac3", AudioChannels: 6, ContainerFormat: "matroska", IsHDR: true}, &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "hevc"}}, Transcode, "incompatible"},
		{"HEVC transcode - no client support", &VideoInfo{Codec: "hevc", AudioCodec: "ac3", AudioChannels: 6, ContainerFormat: "matroska"}, &ClientCapabilities{SupportedVideoCodecs: []string{"h264"}}, Transcode, "incompatible"},
		{"HEVC transcode - nil caps", &VideoInfo{Codec: "hevc", AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "mp4"}, nil, Transcode, "incompatible"},
		{"HEVC direct play - client supports", &VideoInfo{Codec: "hevc", AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "mp4"}, &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "hevc"}}, DirectPlay, "direct playback"},
		{"HEV1 remux - client supports, non-HDR", &VideoInfo{Codec: "hev1", AudioCodec: "flac", AudioChannels: 2, ContainerFormat: "matroska", IsHDR: false}, &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "hevc"}}, RemuxHEVC, "remuxing to HLS"},
		{"H265 direct play - client supports", &VideoInfo{Codec: "h265", AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "mp4"}, &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "hevc"}}, DirectPlay, "direct playback"},

		{"VP9 direct play - client supports", &VideoInfo{Codec: "vp9", AudioCodec: "opus", AudioChannels: 2, ContainerFormat: "mp4"}, &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "vp9"}}, DirectPlay, "direct playback"},
		{"VP9 transcode - no client support", &VideoInfo{Codec: "vp9", AudioCodec: "opus", AudioChannels: 2, ContainerFormat: "webm"}, &ClientCapabilities{SupportedVideoCodecs: []string{"h264"}}, Transcode, "incompatible"},

		{"AV1 direct play - client supports", &VideoInfo{Codec: "av1", Width: 3840, Height: 2160, AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "mp4"}, &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "av1"}}, DirectPlay, "direct playback"},
		{"AV1 transcode - no client support", &VideoInfo{Codec: "av1", AudioCodec: "opus", AudioChannels: 2, ContainerFormat: "mp4"}, &ClientCapabilities{SupportedVideoCodecs: []string{"h264"}}, Transcode, "incompatible"},
		{"AV01 direct play - client supports", &VideoInfo{Codec: "av01", AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "mp4"}, &ClientCapabilities{SupportedVideoCodecs: []string{"h264", "av1"}}, DirectPlay, "direct playback"},
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

func TestShouldTranscode(t *testing.T) {
	tests := []struct {
		name            string
		videoInfo       *VideoInfo
		profile         *AdaptiveProfile
		shouldTranscode bool
		reasonContains  string
	}{
		{"Nil video info", nil, &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, true, "unable to determine"},
		{"Empty codec", &VideoInfo{Codec: "", Width: 1920, Height: 1080}, &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, true, "unable to determine"},
		{"Non-H264 codec", &VideoInfo{Codec: "hevc", Width: 3840, Height: 2160, Bitrate: 20000000}, &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, true, "needs transcoding to H.264"},
		{"H264 lower resolution", &VideoInfo{Codec: "h264", Width: 1280, Height: 720, Bitrate: 3000000}, &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, false, "lower than target"},
		{"H264 matching resolution", &VideoInfo{Codec: "h264", Width: 1920, Height: 1080, Bitrate: 5000000}, &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, false, "already matches target"},
		{"H264 lower bitrate", &VideoInfo{Codec: "h264", Width: 2560, Height: 1440, Bitrate: 2000000}, &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, false, "already lower than target"},
		{"H264 needs downscaling", &VideoInfo{Codec: "h264", Width: 3840, Height: 2160, Bitrate: 20000000}, &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, true, "transcoding from h264"},
		{"H264 multi-channel audio", &VideoInfo{Codec: "h264", AudioCodec: "ac3", AudioChannels: 6, Width: 1920, Height: 1080, Bitrate: 5000000}, &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, true, "audio needs processing"},
		{"H264 FLAC audio", &VideoInfo{Codec: "h264", AudioCodec: "flac", AudioChannels: 2, Width: 1920, Height: 1080, Bitrate: 3000000}, &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, true, "audio needs processing"},
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
