package transcoding

import (
	"testing"
)

func TestDetermineStreamStrategy(t *testing.T) {
	tests := []struct {
		name             string
		videoInfo        *VideoInfo
		expectedStrategy StreamStrategy
		expectedReason   string
	}{
		{
			name: "Direct Play - H.264 + stereo + MP4",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			expectedStrategy: DirectPlay,
			expectedReason:   "direct playback",
		},
		{
			name: "Direct Play - H.264 + mono + WebM",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				Width:           1280,
				Height:          720,
				AudioCodec:      "opus",
				AudioChannels:   1,
				ContainerFormat: "webm",
			},
			expectedStrategy: DirectPlay,
			expectedReason:   "direct playback",
		},
		{
			name: "Remux - H.264 + stereo + MKV",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "matroska,webm", // MKV
			},
			expectedStrategy: Remux,
			expectedReason:   "container remux",
		},
		{
			name: "Remux - H.264 + stereo + AVI",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				Width:           1280,
				Height:          720,
				AudioCodec:      "mp3",
				AudioChannels:   2,
				ContainerFormat: "avi",
			},
			expectedStrategy: Remux,
			expectedReason:   "container remux",
		},
		{
			name: "Remux with Audio Downmix - H.264 + 5.1 + MP4",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "aac",
				AudioChannels:   6, // 5.1 surround
				ContainerFormat: "mp4",
			},
			expectedStrategy: RemuxWithAudioDownmix,
			expectedReason:   "audio needs transcode",
		},
		{
			name: "Remux with Audio Downmix - H.264 + 7.1 + MKV",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				Width:           3840,
				Height:          2160,
				AudioCodec:      "dts",
				AudioChannels:   8, // 7.1 surround
				ContainerFormat: "matroska,webm",
			},
			expectedStrategy: RemuxWithAudioDownmix,
			expectedReason:   "audio needs transcode",
		},
		{
			name: "Transcode - HEVC + stereo + MP4",
			videoInfo: &VideoInfo{
				Codec:           "hevc",
				Width:           3840,
				Height:          2160,
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			expectedStrategy: Transcode,
			expectedReason:   "incompatible",
		},
		{
			name: "Transcode - VP9 + stereo + WebM",
			videoInfo: &VideoInfo{
				Codec:           "vp9",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "opus",
				AudioChannels:   2,
				ContainerFormat: "webm",
			},
			expectedStrategy: Transcode,
			expectedReason:   "incompatible",
		},
		{
			name: "Transcode - H.265 + 5.1 + MKV",
			videoInfo: &VideoInfo{
				Codec:           "h265",
				Width:           3840,
				Height:          2160,
				AudioCodec:      "ac3",
				AudioChannels:   6,
				ContainerFormat: "matroska,webm",
			},
			expectedStrategy: Transcode,
			expectedReason:   "incompatible",
		},
		{
			name:             "Transcode - Nil VideoInfo",
			videoInfo:        nil,
			expectedStrategy: Transcode,
			expectedReason:   "no video info",
		},
		{
			name: "Transcode - Empty codec",
			videoInfo: &VideoInfo{
				Codec:           "",
				Width:           1920,
				Height:          1080,
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			expectedStrategy: Transcode,
			expectedReason:   "incompatible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, reason := DetermineStreamStrategy(tt.videoInfo)

			if strategy != tt.expectedStrategy {
				t.Errorf("DetermineStreamStrategy() strategy = %v, want %v", strategy, tt.expectedStrategy)
			}

			// Check if reason contains expected keywords
			if !contains(reason, tt.expectedReason) {
				t.Errorf("DetermineStreamStrategy() reason = %v, want to contain %v", reason, tt.expectedReason)
			}
		})
	}
}

func TestStreamStrategyConstants(t *testing.T) {
	// Verify strategy constants match expected values
	tests := []struct {
		strategy StreamStrategy
		expected string
	}{
		{DirectPlay, "direct_play"},
		{Remux, "remux"},
		{RemuxWithAudioDownmix, "remux_audio"},
		{Transcode, "transcode"},
	}

	for _, tt := range tests {
		if string(tt.strategy) != tt.expected {
			t.Errorf("StreamStrategy constant mismatch: got %v, want %v", tt.strategy, tt.expected)
		}
	}
}

func TestValidateAndSanitizePath(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()

	tests := []struct {
		name             string
		path             string
		allowedBasePaths []string
		shouldError      bool
		errorContains    string
	}{
		{
			name:             "Valid absolute path",
			path:             tmpDir + "/test.mp4",
			allowedBasePaths: []string{tmpDir},
			shouldError:      false,
		},
		{
			name:             "Valid relative path",
			path:             "./test.mp4",
			allowedBasePaths: nil,
			shouldError:      false,
		},
		{
			name:             "Path traversal with ..",
			path:             tmpDir + "/../../../etc/passwd",
			allowedBasePaths: []string{tmpDir},
			shouldError:      true,
			errorContains:    "outside allowed directories",
		},
		{
			name:             "Path with null byte",
			path:             "/tmp/test\x00.mp4",
			allowedBasePaths: nil,
			shouldError:      true,
			errorContains:    "null bytes",
		},
		{
			name:             "Empty path",
			path:             "",
			allowedBasePaths: nil,
			shouldError:      true,
			errorContains:    "path is empty",
		},
		{
			name:             "Path outside allowed directories",
			path:             "/etc/passwd",
			allowedBasePaths: []string{tmpDir},
			shouldError:      true,
			errorContains:    "outside allowed directories",
		},
		{
			name:             "Valid path with no base path restrictions",
			path:             tmpDir + "/video.mkv",
			allowedBasePaths: nil,
			shouldError:      false,
		},
		{
			name:             "Path with redundant separators",
			path:             tmpDir + "///test//video.mp4",
			allowedBasePaths: []string{tmpDir},
			shouldError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateAndSanitizePath(tt.path, tt.allowedBasePaths)

			if tt.shouldError {
				if err == nil {
					t.Errorf("ValidateAndSanitizePath() expected error but got none")
					return
				}
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("ValidateAndSanitizePath() error = %v, want to contain %v", err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateAndSanitizePath() unexpected error = %v", err)
					return
				}
				if result == "" {
					t.Errorf("ValidateAndSanitizePath() returned empty path")
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
		{
			name:     "Clean filename",
			input:    "video.mp4",
			expected: "video.mp4",
		},
		{
			name:     "Filename with path separator /",
			input:    "../../../etc/passwd",
			expected: "______etc_passwd",
		},
		{
			name:     "Filename with path separator \\",
			input:    "..\\..\\windows\\system32",
			expected: "____windows_system32",
		},
		{
			name:     "Filename with null byte",
			input:    "test\x00.mp4",
			expected: "test.mp4",
		},
		{
			name:     "Filename with dangerous characters",
			input:    "test`$&|;<>(){}[].mp4",
			expected: "test_____________.mp4",
		},
		{
			name:     "Filename with tildes",
			input:    "~/secret/file.mp4",
			expected: "__secret_file.mp4",
		},
		{
			name:     "Complex dangerous filename",
			input:    "$(rm -rf /)&whoami",
			expected: "__rm -rf ___whoami",
		},
		{
			name:     "Empty filename",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeFilename() = %v, want %v", result, tt.expected)
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
		{
			name:      "Nil video info",
			videoInfo: nil,
			profile: &AdaptiveProfile{
				Width:        1920,
				Height:       1080,
				VideoBitrate: 5_000_000,
			},
			shouldTranscode: true,
			reasonContains:  "unable to determine",
		},
		{
			name: "Empty codec",
			videoInfo: &VideoInfo{
				Codec:  "",
				Width:  1920,
				Height: 1080,
			},
			profile: &AdaptiveProfile{
				Width:        1920,
				Height:       1080,
				VideoBitrate: 5_000_000,
			},
			shouldTranscode: true,
			reasonContains:  "unable to determine",
		},
		{
			name: "Non-H264 codec needs transcode",
			videoInfo: &VideoInfo{
				Codec:   "hevc",
				Width:   3840,
				Height:  2160,
				Bitrate: 20000000,
			},
			profile: &AdaptiveProfile{
				Width:        1920,
				Height:       1080,
				VideoBitrate: 5_000_000,
			},
			shouldTranscode: true,
			reasonContains:  "needs transcoding to H.264",
		},
		{
			name: "H264 with lower resolution - skip upscale",
			videoInfo: &VideoInfo{
				Codec:   "h264",
				Width:   1280,
				Height:  720,
				Bitrate: 3000000,
			},
			profile: &AdaptiveProfile{
				Width:        1920,
				Height:       1080,
				VideoBitrate: 5_000_000,
			},
			shouldTranscode: false,
			reasonContains:  "lower than target",
		},
		{
			name: "H264 matching resolution",
			videoInfo: &VideoInfo{
				Codec:   "h264",
				Width:   1920,
				Height:  1080,
				Bitrate: 5000000,
			},
			profile: &AdaptiveProfile{
				Width:        1920,
				Height:       1080,
				VideoBitrate: 5_000_000,
			},
			shouldTranscode: false,
			reasonContains:  "already matches target",
		},
		{
			name: "H264 with lower bitrate but different resolution",
			videoInfo: &VideoInfo{
				Codec:   "h264",
				Width:   2560,
				Height:  1440,
				Bitrate: 2000000,
			},
			profile: &AdaptiveProfile{
				Width:        1920,
				Height:       1080,
				VideoBitrate: 5_000_000,
			},
			shouldTranscode: false,
			reasonContains:  "already lower than target",
		},
		{
			name: "H264 needs downscaling",
			videoInfo: &VideoInfo{
				Codec:   "h264",
				Width:   3840,
				Height:  2160,
				Bitrate: 20000000,
			},
			profile: &AdaptiveProfile{
				Width:        1920,
				Height:       1080,
				VideoBitrate: 5_000_000,
			},
			shouldTranscode: true,
			reasonContains:  "transcoding from h264",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldTranscode, reason := ShouldTranscode(tt.videoInfo, tt.profile)

			if shouldTranscode != tt.shouldTranscode {
				t.Errorf("ShouldTranscode() = %v, want %v", shouldTranscode, tt.shouldTranscode)
			}

			if !contains(reason, tt.reasonContains) {
				t.Errorf("ShouldTranscode() reason = %v, want to contain %v", reason, tt.reasonContains)
			}
		})
	}
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
