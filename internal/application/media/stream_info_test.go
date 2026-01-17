package media

import (
	"testing"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/hls"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/strategy"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/videoinfo"
)

func TestNormalizeContainer(t *testing.T) {
	tests := []struct {
		format   string
		expected string
	}{
		{"matroska,webm", "MKV"},
		{"matroska", "MKV"},
		{"mp4", "MP4"},
		{"mov,mp4,m4a,3gp,3g2,mj2", "MP4"},
		{"avi", "AVI"},
		{"mov", "MOV"},
		{"webm", "WebM"},
		{"mpegts", "MPEG-TS"},
		{"mpeg", "MPEG"},
		{"unknown_format", "unknown_format"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			result := normalizeContainer(tt.format)
			if result != tt.expected {
				t.Errorf("normalizeContainer(%q) = %q, want %q", tt.format, result, tt.expected)
			}
		})
	}
}

func TestGetChannelLabel(t *testing.T) {
	tests := []struct {
		channels int
		expected string
	}{
		{1, "Mono"},
		{2, "Stereo"},
		{6, "5.1"},
		{8, "7.1"},
		{4, "4 ch"},
		{10, "10 ch"},
		{0, "0 ch"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := getChannelLabel(tt.channels)
			if result != tt.expected {
				t.Errorf("getChannelLabel(%d) = %q, want %q", tt.channels, result, tt.expected)
			}
		})
	}
}

func TestGetHDRFormat(t *testing.T) {
	tests := []struct {
		name     string
		info     *videoinfo.VideoInfo
		expected string
	}{
		{
			name:     "non-HDR content",
			info:     &videoinfo.VideoInfo{VideoInfo: hls.VideoInfo{IsHDR: false}},
			expected: "",
		},
		{
			name: "HDR10 (smpte2084)",
			info: &videoinfo.VideoInfo{
				VideoInfo: hls.VideoInfo{
					IsHDR:         true,
					ColorTransfer: "smpte2084",
				},
			},
			expected: "HDR10",
		},
		{
			name: "HLG",
			info: &videoinfo.VideoInfo{
				VideoInfo: hls.VideoInfo{
					IsHDR:         true,
					ColorTransfer: "arib-std-b67",
				},
			},
			expected: "HLG",
		},
		{
			name: "generic HDR with bt2020",
			info: &videoinfo.VideoInfo{
				VideoInfo: hls.VideoInfo{
					IsHDR:          true,
					ColorTransfer:  "unknown",
					ColorPrimaries: "bt2020",
				},
			},
			expected: "HDR",
		},
		{
			name: "HDR with unknown color info",
			info: &videoinfo.VideoInfo{
				VideoInfo: hls.VideoInfo{
					IsHDR:          true,
					ColorTransfer:  "unknown",
					ColorPrimaries: "unknown",
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getHDRFormat(tt.info)
			if result != tt.expected {
				t.Errorf("getHDRFormat() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"matroska,webm", "matroska", true},
		{"matroska,webm", "MATROSKA", true}, // case insensitive
		{"mp4", "mp4", true},
		{"mp4", "avi", false},
		{"", "foo", false},
		{"foo", "", true},
		{"", "", true},
		{"UPPERCASE", "upper", true},
		{"lowercase", "LOWER", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"Hello World", "hello", true},
		{"Hello World", "WORLD", true},
		{"Hello World", "foo", false},
		{"test", "TEST", true},
		{"TEST", "test", true},
		{"MixedCase", "mixedcase", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := containsIgnoreCase(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("containsIgnoreCase(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestEqualIgnoreCase(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected bool
	}{
		{"hello", "HELLO", true},
		{"WORLD", "world", true},
		{"Test", "TEST", true},
		{"foo", "bar", false},
		{"", "", true},
		{"a", "b", false},
		{"ab", "a", false},
		{"MKV", "mkv", true},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			result := equalIgnoreCase(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("equalIgnoreCase(%q, %q) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestStrategyToMode(t *testing.T) {
	tests := []struct {
		strategy strategy.StreamStrategy
		expected string
	}{
		{strategy.DirectPlay, "direct"},
		{strategy.Remux, "remux"},
		{strategy.RemuxWithAudioDownmix, "remux_audio"},
		{strategy.RemuxHEVC, "remux_hevc"},
		{strategy.Transcode, "transcode"},
		{strategy.StreamStrategy("unknown_strategy"), "transcode"}, // unknown defaults to transcode
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := strategyToMode(tt.strategy)
			if result != tt.expected {
				t.Errorf("strategyToMode(%v) = %q, want %q", tt.strategy, result, tt.expected)
			}
		})
	}
}

func TestNewStreamInfoUseCase(t *testing.T) {
	uc := NewStreamInfoUseCase(nil, nil)
	if uc == nil {
		t.Fatal("NewStreamInfoUseCase returned nil")
	}
}

func TestVideoStreamInfoCreation(t *testing.T) {
	info := VideoStreamInfo{
		Codec:          "h264",
		Profile:        "High",
		Width:          1920,
		Height:         1080,
		Bitrate:        8000000,
		FrameRate:      23.976,
		BitDepth:       8,
		IsHDR:          false,
		HDRFormat:      "",
		ColorSpace:     "bt709",
		ColorPrimaries: "bt709",
		PixelFormat:    "yuv420p",
	}

	if info.Codec != "h264" {
		t.Errorf("Codec = %q, want %q", info.Codec, "h264")
	}
	if info.Width != 1920 {
		t.Errorf("Width = %d, want %d", info.Width, 1920)
	}
	if info.Height != 1080 {
		t.Errorf("Height = %d, want %d", info.Height, 1080)
	}
	if info.IsHDR {
		t.Error("IsHDR should be false")
	}
}

func TestAudioTrackInfoCreation(t *testing.T) {
	info := AudioTrackInfo{
		Index:        0,
		Codec:        "aac",
		Channels:     6,
		ChannelLabel: "5.1",
		Bitrate:      384000,
		Language:     "eng",
		Title:        "English 5.1",
		IsDefault:    true,
		IsCommentary: false,
	}

	if info.Channels != 6 {
		t.Errorf("Channels = %d, want %d", info.Channels, 6)
	}
	if info.ChannelLabel != "5.1" {
		t.Errorf("ChannelLabel = %q, want %q", info.ChannelLabel, "5.1")
	}
	if !info.IsDefault {
		t.Error("IsDefault should be true")
	}
}

func TestStreamingStrategyCreation(t *testing.T) {
	s := StreamingStrategy{
		Mode:   "transcode",
		Reason: "HEVC codec requires transcoding for web playback",
	}

	if s.Mode != "transcode" {
		t.Errorf("Mode = %q, want %q", s.Mode, "transcode")
	}
	if s.Reason == "" {
		t.Error("Reason should not be empty")
	}
}

func TestToneMappingInfoCreation(t *testing.T) {
	info := ToneMappingInfo{
		Enabled:   true,
		Algorithm: "bt.2390",
		Backend:   "libplacebo",
	}

	if !info.Enabled {
		t.Error("Enabled should be true")
	}
	if info.Algorithm != "bt.2390" {
		t.Errorf("Algorithm = %q, want %q", info.Algorithm, "bt.2390")
	}
	if info.Backend != "libplacebo" {
		t.Errorf("Backend = %q, want %q", info.Backend, "libplacebo")
	}
}

func TestStreamInfoResponseCreation(t *testing.T) {
	response := StreamInfoResponse{
		Container: "MKV",
		FileSize:  1073741824, // 1GB
		Duration:  7200,       // 2 hours
		Bitrate:   10000000,
		Video: VideoStreamInfo{
			Codec:   "hevc",
			Width:   3840,
			Height:  2160,
			IsHDR:   true,
			BitDepth: 10,
		},
		AudioTracks: []AudioTrackInfo{
			{Index: 0, Codec: "truehd", Channels: 8, ChannelLabel: "7.1"},
			{Index: 1, Codec: "aac", Channels: 2, ChannelLabel: "Stereo"},
		},
		SelectedAudioIndex: 0,
		Strategy: StreamingStrategy{
			Mode:   "transcode",
			Reason: "HDR content requires tone mapping",
		},
		ToneMapping: &ToneMappingInfo{
			Enabled:   true,
			Algorithm: "bt.2390",
			Backend:   "libplacebo",
		},
	}

	if response.Container != "MKV" {
		t.Errorf("Container = %q, want %q", response.Container, "MKV")
	}
	if len(response.AudioTracks) != 2 {
		t.Errorf("AudioTracks count = %d, want 2", len(response.AudioTracks))
	}
	if response.Video.IsHDR != true {
		t.Error("Video.IsHDR should be true")
	}
	if response.ToneMapping == nil {
		t.Fatal("ToneMapping should not be nil")
	}
	if !response.ToneMapping.Enabled {
		t.Error("ToneMapping.Enabled should be true")
	}
}
