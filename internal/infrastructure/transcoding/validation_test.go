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
			name: "Remux - H.264 + mono + WebM",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				Width:           1280,
				Height:          720,
				AudioCodec:      "opus",
				AudioChannels:   1,
				ContainerFormat: "webm",
			},
			expectedStrategy: Remux,
			expectedReason:   "container remux",
		},
		{
			name: "Remux - H.264 + stereo + MKV",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "matroska", // Pure MKV (not WebM)
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
			name: "Remux with Audio Downmix - H.264 + DTS stereo + MKV (incompatible audio codec)",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "dts", // DTS is NOT web-compatible even in stereo
				AudioChannels:   2,     // stereo
				ContainerFormat: "matroska",
			},
			expectedStrategy: RemuxWithAudioDownmix,
			expectedReason:   "audio needs transcode",
		},
		{
			name: "Remux with Audio Downmix - H.264 + AC3 stereo + MP4 (incompatible audio codec)",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "ac3", // AC3/Dolby Digital NOT web-compatible
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			expectedStrategy: RemuxWithAudioDownmix,
			expectedReason:   "audio needs transcode",
		},
		{
			name: "Remux with Audio Downmix - H.264 + TrueHD stereo + MKV (incompatible audio codec)",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "truehd", // TrueHD NOT web-compatible
				AudioChannels:   2,
				ContainerFormat: "matroska",
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
		{RemuxHEVC, "remux_hevc"},
		{Transcode, "transcode"},
	}

	for _, tt := range tests {
		if string(tt.strategy) != tt.expected {
			t.Errorf("StreamStrategy constant mismatch: got %v, want %v", tt.strategy, tt.expected)
		}
	}
}

func TestDetermineStreamStrategyWithHEVCSupport(t *testing.T) {
	// Test HEVC handling with client capabilities
	// NOTE: HEVC remux is currently DISABLED in strategy.go due to seek corruption issues
	tests := []struct {
		name             string
		videoInfo        *VideoInfo
		clientCaps       *ClientCapabilitiesForStrategy
		expectedStrategy StreamStrategy
		expectedReason   string
	}{
		{
			name: "HEVC transcode - client supports HEVC but remux disabled + AC3 audio",
			videoInfo: &VideoInfo{
				Codec:           "hevc",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "ac3",
				AudioChannels:   6,
				ContainerFormat: "matroska",
			},
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "hevc"},
			},
			expectedStrategy: Transcode,
			expectedReason:   "incompatible",
		},
		{
			name: "HEVC transcode - client supports H.265 but remux disabled + DTS audio",
			videoInfo: &VideoInfo{
				Codec:           "h265",
				Width:           3840,
				Height:          2160,
				AudioCodec:      "dts",
				AudioChannels:   8,
				ContainerFormat: "mkv",
			},
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "h265"},
			},
			expectedStrategy: Transcode,
			expectedReason:   "incompatible",
		},
		{
			name: "HEVC transcode - client does NOT support HEVC",
			videoInfo: &VideoInfo{
				Codec:           "hevc",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "ac3",
				AudioChannels:   6,
				ContainerFormat: "matroska",
			},
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264"}, // Only H.264
			},
			expectedStrategy: Transcode,
			expectedReason:   "incompatible",
		},
		{
			name: "HEVC transcode - no client capabilities (legacy)",
			videoInfo: &VideoInfo{
				Codec:           "hevc",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			clientCaps:       nil,
			expectedStrategy: Transcode,
			expectedReason:   "incompatible",
		},
		{
			name: "HEVC direct play - client supports HEVC + AAC stereo + MP4",
			videoInfo: &VideoInfo{
				Codec:           "hevc",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "hevc"},
			},
			expectedStrategy: DirectPlay,
			expectedReason:   "direct playback",
		},
		{
			name: "HEVC transcode - HEV1 variant even with client support (remux disabled)",
			videoInfo: &VideoInfo{
				Codec:           "hev1",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "flac",
				AudioChannels:   2,
				ContainerFormat: "matroska",
			},
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "hevc"},
			},
			expectedStrategy: Transcode,
			expectedReason:   "incompatible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, reason := DetermineStreamStrategyWithCapabilities(tt.videoInfo, tt.clientCaps)

			if strategy != tt.expectedStrategy {
				t.Errorf("DetermineStreamStrategyWithCapabilities() strategy = %v, want %v", strategy, tt.expectedStrategy)
			}

			if !contains(reason, tt.expectedReason) {
				t.Errorf("DetermineStreamStrategyWithCapabilities() reason = %v, want to contain %v", reason, tt.expectedReason)
			}
		})
	}
}

func TestDetermineStreamStrategyWithModernCodecs(t *testing.T) {
	// Test VP9 and AV1 codec support with client capabilities
	tests := []struct {
		name             string
		videoInfo        *VideoInfo
		clientCaps       *ClientCapabilitiesForStrategy
		expectedStrategy StreamStrategy
		expectedReason   string
	}{
		{
			name: "VP9 direct play - client supports VP9 + Opus stereo + MP4",
			videoInfo: &VideoInfo{
				Codec:           "vp9",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "opus",
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "vp9"},
			},
			expectedStrategy: DirectPlay,
			expectedReason:   "direct playback",
		},
		{
			name: "VP9 transcode - client does NOT support VP9",
			videoInfo: &VideoInfo{
				Codec:           "vp9",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "opus",
				AudioChannels:   2,
				ContainerFormat: "webm",
			},
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264"},
			},
			expectedStrategy: Transcode,
			expectedReason:   "incompatible",
		},
		{
			name: "AV1 direct play - client supports AV1 + AAC stereo + MP4",
			videoInfo: &VideoInfo{
				Codec:           "av1",
				Width:           3840,
				Height:          2160,
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "av1"},
			},
			expectedStrategy: DirectPlay,
			expectedReason:   "direct playback",
		},
		{
			name: "AV1 transcode - client does NOT support AV1",
			videoInfo: &VideoInfo{
				Codec:           "av1",
				Width:           3840,
				Height:          2160,
				AudioCodec:      "opus",
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264"},
			},
			expectedStrategy: Transcode,
			expectedReason:   "incompatible",
		},
		{
			name: "AV01 variant direct play - client supports AV1",
			videoInfo: &VideoInfo{
				Codec:           "av01",
				Width:           1920,
				Height:          1080,
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "av1"},
			},
			expectedStrategy: DirectPlay,
			expectedReason:   "direct playback",
		},
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

func TestIsCodecSupportedByClient(t *testing.T) {
	tests := []struct {
		name       string
		codec      string
		clientCaps *ClientCapabilitiesForStrategy
		expected   bool
	}{
		// H.264 variants - always supported
		{
			name:       "H.264 always supported (no caps)",
			codec:      "h264",
			clientCaps: nil,
			expected:   true,
		},
		{
			name:       "AVC1 always supported (no caps)",
			codec:      "avc1",
			clientCaps: nil,
			expected:   true,
		},
		{
			name:       "AVC always supported (no caps)",
			codec:      "avc",
			clientCaps: nil,
			expected:   true,
		},

		// H.265/HEVC variants
		{
			name:       "HEVC not supported without caps",
			codec:      "hevc",
			clientCaps: nil,
			expected:   false,
		},
		{
			name:  "HEVC supported with h265 in caps",
			codec: "hevc",
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "h265"},
			},
			expected: true,
		},
		{
			name:  "H265 supported with hevc in caps",
			codec: "h265",
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "hevc"},
			},
			expected: true,
		},
		{
			name:  "HEV1 supported with h265 in caps",
			codec: "hev1",
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "h265"},
			},
			expected: true,
		},

		// VP9
		{
			name:       "VP9 not supported without caps",
			codec:      "vp9",
			clientCaps: nil,
			expected:   false,
		},
		{
			name:  "VP9 supported with caps",
			codec: "vp9",
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "vp9"},
			},
			expected: true,
		},

		// AV1
		{
			name:       "AV1 not supported without caps",
			codec:      "av1",
			clientCaps: nil,
			expected:   false,
		},
		{
			name:  "AV1 supported with caps",
			codec: "av1",
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "av1"},
			},
			expected: true,
		},
		{
			name:  "AV01 variant supported with av1 in caps",
			codec: "av01",
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "av1"},
			},
			expected: true,
		},

		// Case insensitivity in client caps (codec param should already be lowercased)
		{
			name:  "Client caps case insensitive for HEVC",
			codec: "hevc",
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"H264", "H265"},
			},
			expected: true,
		},
		{
			name:  "Client caps case insensitive for VP9",
			codec: "vp9",
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"H264", "VP9"},
			},
			expected: true,
		},
		{
			name:  "Client caps case insensitive for AV1",
			codec: "av1",
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "AV1"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCodecSupportedByClient(tt.codec, tt.clientCaps)
			if result != tt.expected {
				t.Errorf("isCodecSupportedByClient(%q, %v) = %v, want %v",
					tt.codec, tt.clientCaps, result, tt.expected)
			}
		})
	}
}

func TestIsWebCompatibleContainer(t *testing.T) {
	tests := []struct {
		name       string
		container  string
		clientCaps *ClientCapabilitiesForStrategy
		expected   bool
	}{
		// Web-compatible containers
		{
			name:       "MP4 is web-compatible",
			container:  "mp4",
			clientCaps: nil,
			expected:   true,
		},
		{
			name:       "MOV is web-compatible",
			container:  "mov",
			clientCaps: nil,
			expected:   true,
		},
		{
			name:       "M4V is web-compatible",
			container:  "m4v",
			clientCaps: nil,
			expected:   true,
		},
		{
			name:       "M4A is web-compatible",
			container:  "m4a",
			clientCaps: nil,
			expected:   true,
		},
		{
			name:       "3GP is web-compatible",
			container:  "3gp",
			clientCaps: nil,
			expected:   true,
		},
		{
			name:       "3G2 is web-compatible",
			container:  "3g2",
			clientCaps: nil,
			expected:   true,
		},
		{
			name:       "FFprobe multi-format MP4",
			container:  "mov,mp4,m4a,3gp,3g2,mj2",
			clientCaps: nil,
			expected:   true,
		},

		// Matroska/WebM containers - NOT web-compatible by default
		{
			name:       "MKV not web-compatible",
			container:  "matroska,webm",
			clientCaps: nil,
			expected:   false,
		},
		{
			name:       "WebM not web-compatible (requires codec check)",
			container:  "webm",
			clientCaps: nil,
			expected:   false,
		},
		{
			name:       "Matroska not web-compatible",
			container:  "matroska",
			clientCaps: nil,
			expected:   false,
		},
		{
			name:       "MKV literal not web-compatible",
			container:  "mkv",
			clientCaps: nil,
			expected:   false,
		},
		{
			name:      "MKV supported with explicit client caps",
			container: "matroska,webm",
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedContainers: []string{"matroska"},
			},
			expected: true,
		},
		{
			name:      "MKV literal supported with mkv in caps",
			container: "mkv",
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedContainers: []string{"mkv"},
			},
			expected: true,
		},

		// Other containers
		{
			name:       "AVI not web-compatible",
			container:  "avi",
			clientCaps: nil,
			expected:   false,
		},
		{
			name:       "FLV not web-compatible",
			container:  "flv",
			clientCaps: nil,
			expected:   false,
		},
		{
			name:       "MPEG-TS not web-compatible",
			container:  "mpegts",
			clientCaps: nil,
			expected:   false,
		},

		// Case insensitivity
		{
			name:       "Case insensitive MP4",
			container:  "MP4",
			clientCaps: nil,
			expected:   true,
		},
		{
			name:       "Case insensitive MOV",
			container:  "MOV",
			clientCaps: nil,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isWebCompatibleContainer(tt.container, tt.clientCaps)
			if result != tt.expected {
				t.Errorf("isWebCompatibleContainer(%q, %v) = %v, want %v",
					tt.container, tt.clientCaps, result, tt.expected)
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
		// Web-compatible audio codecs for HLS/fMP4
		{
			name:     "AAC is web-compatible",
			codec:    "aac",
			expected: true,
		},
		{
			name:     "MP3 is web-compatible",
			codec:    "mp3",
			expected: true,
		},
		{
			name:     "Opus is web-compatible",
			codec:    "opus",
			expected: true,
		},
		{
			name:     "Vorbis is web-compatible",
			codec:    "vorbis",
			expected: true,
		},
		{
			name:     "MP4A (AAC variant) is web-compatible",
			codec:    "mp4a",
			expected: true,
		},
		{
			name:     "AAC_LATM is web-compatible",
			codec:    "aac_latm",
			expected: true,
		},
		{
			name:     "mp4a.40.2 variant is web-compatible",
			codec:    "mp4a.40.2",
			expected: true,
		},

		// NOT web-compatible for HLS/fMP4
		{
			name:     "FLAC not web-compatible in HLS",
			codec:    "flac",
			expected: false,
		},
		{
			name:     "AC3 (Dolby Digital) not web-compatible",
			codec:    "ac3",
			expected: false,
		},
		{
			name:     "EAC3 (Dolby Digital Plus) not web-compatible",
			codec:    "eac3",
			expected: false,
		},
		{
			name:     "DTS not web-compatible",
			codec:    "dts",
			expected: false,
		},
		{
			name:     "DTS-HD MA not web-compatible",
			codec:    "dts-hd",
			expected: false,
		},
		{
			name:     "TrueHD not web-compatible",
			codec:    "truehd",
			expected: false,
		},
		{
			name:     "PCM not web-compatible",
			codec:    "pcm_s16le",
			expected: false,
		},
		{
			name:     "PCM big-endian not web-compatible",
			codec:    "pcm_s16be",
			expected: false,
		},
		{
			name:     "PCM 24-bit not web-compatible",
			codec:    "pcm_s24le",
			expected: false,
		},

		// Case insensitivity
		{
			name:     "Case insensitive AAC",
			codec:    "AAC",
			expected: true,
		},
		{
			name:     "Case insensitive MP3",
			codec:    "MP3",
			expected: true,
		},
		{
			name:     "Case insensitive Opus",
			codec:    "OPUS",
			expected: true,
		},
		{
			name:     "Case insensitive AC3",
			codec:    "AC3",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isWebCompatibleAudioCodec(tt.codec)
			if result != tt.expected {
				t.Errorf("isWebCompatibleAudioCodec(%q) = %v, want %v",
					tt.codec, result, tt.expected)
			}
		})
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
		{
			name: "Needs transcode - H264 with multi-channel audio",
			videoInfo: &VideoInfo{
				Codec:         "h264",
				AudioCodec:    "ac3",
				AudioChannels: 6,
				Width:         1920,
				Height:        1080,
				Bitrate:       5000000,
			},
			profile: &AdaptiveProfile{
				Width:        1920,
				Height:       1080,
				VideoBitrate: 5_000_000,
			},
			shouldTranscode: true,
			reasonContains:  "audio needs processing",
		},
		{
			name: "Needs transcode - H264 with FLAC audio",
			videoInfo: &VideoInfo{
				Codec:         "h264",
				AudioCodec:    "flac",
				AudioChannels: 2,
				Width:         1920,
				Height:        1080,
				Bitrate:       3000000,
			},
			profile: &AdaptiveProfile{
				Width:        1920,
				Height:       1080,
				VideoBitrate: 5_000_000,
			},
			shouldTranscode: true,
			reasonContains:  "audio needs processing",
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

func TestMultiChannelAudioHandling(t *testing.T) {
	// Test various multi-channel audio scenarios
	tests := []struct {
		name             string
		videoInfo        *VideoInfo
		expectedStrategy StreamStrategy
		expectedReason   string
	}{
		{
			name: "Stereo AAC - no downmix needed",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			expectedStrategy: DirectPlay,
			expectedReason:   "direct playback",
		},
		{
			name: "Mono MP3 - no downmix needed",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				AudioCodec:      "mp3",
				AudioChannels:   1,
				ContainerFormat: "mp4",
			},
			expectedStrategy: DirectPlay,
			expectedReason:   "direct playback",
		},
		{
			name: "5.1 AAC - needs downmix",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				AudioCodec:      "aac",
				AudioChannels:   6,
				ContainerFormat: "mp4",
			},
			expectedStrategy: RemuxWithAudioDownmix,
			expectedReason:   "audio needs transcode",
		},
		{
			name: "7.1 DTS - needs downmix",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				AudioCodec:      "dts",
				AudioChannels:   8,
				ContainerFormat: "mkv",
			},
			expectedStrategy: RemuxWithAudioDownmix,
			expectedReason:   "audio needs transcode",
		},
		{
			name: "Quad audio (4 channels) - needs downmix",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				AudioCodec:      "aac",
				AudioChannels:   4,
				ContainerFormat: "mp4",
			},
			expectedStrategy: RemuxWithAudioDownmix,
			expectedReason:   "audio needs transcode",
		},
		{
			name: "3 channel audio - needs downmix",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				AudioCodec:      "aac",
				AudioChannels:   3,
				ContainerFormat: "mp4",
			},
			expectedStrategy: RemuxWithAudioDownmix,
			expectedReason:   "audio needs transcode",
		},
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

func TestCodecVariantHandling(t *testing.T) {
	// Test various codec variant names that FFprobe might return
	tests := []struct {
		name             string
		videoInfo        *VideoInfo
		clientCaps       *ClientCapabilitiesForStrategy
		expectedStrategy StreamStrategy
		expectedReason   string
	}{
		{
			name: "AVC1 variant of H.264",
			videoInfo: &VideoInfo{
				Codec:           "avc1",
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			clientCaps:       nil,
			expectedStrategy: DirectPlay,
			expectedReason:   "direct playback",
		},
		{
			name: "H265 variant of HEVC with client support",
			videoInfo: &VideoInfo{
				Codec:           "h265",
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "hevc"},
			},
			expectedStrategy: DirectPlay,
			expectedReason:   "direct playback",
		},
		{
			name: "HEV1 variant of HEVC without client support",
			videoInfo: &VideoInfo{
				Codec:           "hev1",
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			clientCaps:       nil,
			expectedStrategy: Transcode,
			expectedReason:   "incompatible",
		},
		{
			name: "AV01 variant of AV1 with client support",
			videoInfo: &VideoInfo{
				Codec:           "av01",
				AudioCodec:      "opus",
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			clientCaps: &ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: []string{"h264", "av01"},
			},
			expectedStrategy: DirectPlay,
			expectedReason:   "direct playback",
		},
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

func TestContainerFormatParsing(t *testing.T) {
	// Test FFprobe's comma-separated container format parsing
	tests := []struct {
		name             string
		videoInfo        *VideoInfo
		expectedStrategy StreamStrategy
		expectedReason   string
	}{
		{
			name: "FFprobe MP4 multi-format string",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "mov,mp4,m4a,3gp,3g2,mj2",
			},
			expectedStrategy: DirectPlay,
			expectedReason:   "direct playback",
		},
		{
			name: "FFprobe Matroska/WebM string",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "matroska,webm",
			},
			expectedStrategy: Remux,
			expectedReason:   "container remux",
		},
		{
			name: "Single format MP4",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "mp4",
			},
			expectedStrategy: DirectPlay,
			expectedReason:   "direct playback",
		},
		{
			name: "Single format matroska",
			videoInfo: &VideoInfo{
				Codec:           "h264",
				AudioCodec:      "aac",
				AudioChannels:   2,
				ContainerFormat: "matroska",
			},
			expectedStrategy: Remux,
			expectedReason:   "container remux",
		},
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

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
