package transcoding

import (
	"testing"
)

// Tests for the re-exported strategy functions from the root transcoding package.
// Comprehensive strategy tests are in the strategy subpackage.

func TestDetermineStreamStrategy(t *testing.T) {
	tests := []struct {
		name             string
		videoInfo        *VideoInfo
		expectedStrategy StreamStrategy
		expectedReason   string
	}{
		{"Direct Play - H.264 + stereo + MP4", &VideoInfo{Codec: "h264", Width: 1920, Height: 1080, AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "mp4"}, DirectPlay, "direct playback"},
		{"Direct Play - AVC1 variant", &VideoInfo{Codec: "avc1", AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "mp4"}, DirectPlay, "direct playback"},
		{"Remux - H.264 + stereo + MKV", &VideoInfo{Codec: "h264", Width: 1920, Height: 1080, AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "matroska"}, Remux, "container remux"},
		{"Remux with Audio Downmix - 5.1 AAC", &VideoInfo{Codec: "h264", Width: 1920, Height: 1080, AudioCodec: "aac", AudioChannels: 6, ContainerFormat: "mp4"}, RemuxWithAudioDownmix, "audio needs transcode"},
		{"Transcode - HEVC", &VideoInfo{Codec: "hevc", Width: 3840, Height: 2160, AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "mp4"}, Transcode, "incompatible"},
		{"Transcode - nil VideoInfo", nil, Transcode, "no video info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, reason := DetermineStreamStrategy(tt.videoInfo)
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

func TestDetermineStreamStrategyWithCapabilities(t *testing.T) {
	tests := []struct {
		name             string
		videoInfo        *VideoInfo
		clientCaps       *ClientCapabilitiesForStrategy
		expectedStrategy StreamStrategy
		expectedReason   string
	}{
		{"HEVC transcode - client supports but remux disabled", &VideoInfo{Codec: "hevc", AudioCodec: "ac3", AudioChannels: 6, ContainerFormat: "matroska"}, &ClientCapabilitiesForStrategy{SupportedVideoCodecs: []string{"h264", "hevc"}}, Transcode, "incompatible"},
		{"HEVC direct play - client supports", &VideoInfo{Codec: "hevc", AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "mp4"}, &ClientCapabilitiesForStrategy{SupportedVideoCodecs: []string{"h264", "hevc"}}, DirectPlay, "direct playback"},
		{"VP9 direct play - client supports", &VideoInfo{Codec: "vp9", AudioCodec: "opus", AudioChannels: 2, ContainerFormat: "mp4"}, &ClientCapabilitiesForStrategy{SupportedVideoCodecs: []string{"h264", "vp9"}}, DirectPlay, "direct playback"},
		{"AV1 direct play - client supports", &VideoInfo{Codec: "av1", Width: 3840, Height: 2160, AudioCodec: "aac", AudioChannels: 2, ContainerFormat: "mp4"}, &ClientCapabilitiesForStrategy{SupportedVideoCodecs: []string{"h264", "av1"}}, DirectPlay, "direct playback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, reason := DetermineStreamStrategyWithCapabilities(tt.videoInfo, tt.clientCaps)
			if strategy != tt.expectedStrategy {
				t.Errorf("strategy = %v, want %v", strategy, tt.expectedStrategy)
			}
			if !contains(reason, tt.expectedReason) {
				t.Errorf("reason = %v, want to contain %v", reason, tt.expectedReason)
			}
		})
	}
}

func TestValidateAndSanitizePath(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name             string
		path             string
		allowedBasePaths []string
		shouldError      bool
		errorContains    string
	}{
		{"Valid absolute path", tmpDir + "/test.mp4", []string{tmpDir}, false, ""},
		{"Valid relative path", "./test.mp4", nil, false, ""},
		{"Path traversal", tmpDir + "/../../../etc/passwd", []string{tmpDir}, true, "outside allowed directories"},
		{"Null byte", "/tmp/test\x00.mp4", nil, true, "null bytes"},
		{"Empty path", "", nil, true, "path is empty"},
		{"Path outside allowed dirs", "/etc/passwd", []string{tmpDir}, true, "outside allowed directories"},
		{"No base path restrictions", tmpDir + "/video.mkv", nil, false, ""},
		{"Redundant separators", tmpDir + "///test//video.mp4", []string{tmpDir}, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateAndSanitizePath(tt.path, tt.allowedBasePaths)
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("error = %v, want to contain %v", err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error = %v", err)
					return
				}
				if result == "" {
					t.Errorf("returned empty path")
				}
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Clean filename", "video.mp4", "video.mp4"},
		{"Path separator /", "../../../etc/passwd", "______etc_passwd"},
		{"Path separator \\", "..\\..\\windows\\system32", "____windows_system32"},
		{"Null byte", "test\x00.mp4", "test.mp4"},
		{"Dangerous characters", "test`$&|;<>(){}[].mp4", "test_____________.mp4"},
		{"Tildes", "~/secret/file.mp4", "__secret_file.mp4"},
		{"Complex dangerous", "$(rm -rf /)&whoami", "__rm -rf ___whoami"},
		{"Empty filename", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeFilename(tt.input)
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
		{"Non-H264 codec", &VideoInfo{Codec: "hevc", Width: 3840, Height: 2160, Bitrate: 20000000}, &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, true, "needs transcoding to H.264"},
		{"H264 lower resolution", &VideoInfo{Codec: "h264", Width: 1280, Height: 720, Bitrate: 3000000}, &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, false, "lower than target"},
		{"H264 matching resolution", &VideoInfo{Codec: "h264", Width: 1920, Height: 1080, Bitrate: 5000000}, &AdaptiveProfile{Width: 1920, Height: 1080, VideoBitrate: 5_000_000}, false, "already matches target"},
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
