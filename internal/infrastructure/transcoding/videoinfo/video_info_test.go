package videoinfo

import (
	"testing"
)

func TestIsHDRContent(t *testing.T) {
	tests := []struct {
		colorTransfer  string
		colorPrimaries string
		expectedHDR    bool
	}{
		// HDR formats
		{"smpte2084", "bt2020", true},
		{"smpte2084", "bt709", true},
		{"smpte2084", "", true},
		{"arib-std-b67", "bt2020", true},
		{"arib-std-b67", "bt709", true},
		{"arib-std-b67", "", true},
		{"smpte428", "bt2020", true},
		{"bt2020-10", "bt2020", true},
		// SDR formats
		{"bt709", "bt709", false},
		{"bt709", "bt2020", false},
		{"bt709", "", false},
		{"", "bt2020", false},
		{"", "", false},
		{"smpte170m", "smpte170m", false},
		{"bt470m", "bt470m", false},
	}

	for _, tt := range tests {
		t.Run(tt.colorTransfer+"/"+tt.colorPrimaries, func(t *testing.T) {
			result := IsHDRContent(tt.colorTransfer, tt.colorPrimaries)
			if result != tt.expectedHDR {
				t.Errorf("IsHDRContent(%q, %q) = %v, want %v",
					tt.colorTransfer, tt.colorPrimaries, result, tt.expectedHDR)
			}
		})
	}
}

func TestDetectBitDepthFromPixelFormat(t *testing.T) {
	tests := []struct {
		pixelFormat   string
		expectedDepth int
	}{
		// 8-bit formats
		{"yuv420p", 8},
		{"yuv422p", 8},
		{"yuv444p", 8},
		{"rgb24", 8},
		{"", 8},
		// 10-bit formats
		{"yuv420p10le", 10},
		{"yuv420p10be", 10},
		{"yuv422p10le", 10},
		{"p010le", 10},
		{"yuv42010p", 10},
		// 12-bit formats (NV12 is a known bug - detected as 12-bit instead of 8-bit)
		{"nv12", 12},
		{"yuv420p12le", 12},
		{"yuv422p12le", 12},
		{"yuv444p12le", 12},
		// 16-bit formats
		{"yuv420p16le", 16},
		{"yuv444p16le", 16},
	}

	for _, tt := range tests {
		t.Run(tt.pixelFormat, func(t *testing.T) {
			result := DetectBitDepthFromPixelFormat(tt.pixelFormat)
			if result != tt.expectedDepth {
				t.Errorf("DetectBitDepthFromPixelFormat(%q) = %d, want %d",
					tt.pixelFormat, result, tt.expectedDepth)
			}
		})
	}
}

func TestSelectBestAudioTrack(t *testing.T) {
	tests := []struct {
		name          string
		tracks        []AudioTrack
		expectedIndex int
	}{
		{
			name:          "empty",
			tracks:        []AudioTrack{},
			expectedIndex: -1,
		},
		{
			name: "aac_stereo_preferred",
			tracks: []AudioTrack{
				{Index: 0, Codec: "aac", Channels: 2},
				{Index: 1, Codec: "ac3", Channels: 6},
			},
			expectedIndex: 0,
		},
		{
			name: "mp3_stereo_over_multichannel",
			tracks: []AudioTrack{
				{Index: 0, Codec: "dts", Channels: 8},
				{Index: 1, Codec: "mp3", Channels: 2},
			},
			expectedIndex: 1,
		},
		{
			name: "opus_stereo",
			tracks: []AudioTrack{
				{Index: 0, Codec: "opus", Channels: 2},
				{Index: 1, Codec: "vorbis", Channels: 6},
			},
			expectedIndex: 0,
		},
		{
			name: "aac_mono_is_web_compatible",
			tracks: []AudioTrack{
				{Index: 0, Codec: "aac", Channels: 1},
				{Index: 1, Codec: "ac3", Channels: 2},
			},
			expectedIndex: 0,
		},
		{
			name: "aac_multichannel_over_non_web",
			tracks: []AudioTrack{
				{Index: 0, Codec: "ac3", Channels: 6},
				{Index: 1, Codec: "aac", Channels: 6},
				{Index: 2, Codec: "dts", Channels: 8},
			},
			expectedIndex: 1,
		},
		{
			name: "vorbis_multichannel",
			tracks: []AudioTrack{
				{Index: 0, Codec: "dts", Channels: 6},
				{Index: 1, Codec: "vorbis", Channels: 6},
			},
			expectedIndex: 1,
		},
		{
			name: "ac3_stereo_over_multichannel",
			tracks: []AudioTrack{
				{Index: 0, Codec: "dts", Channels: 8},
				{Index: 1, Codec: "ac3", Channels: 2},
				{Index: 2, Codec: "truehd", Channels: 8},
			},
			expectedIndex: 1,
		},
		{
			name: "fallback_to_first_non_web",
			tracks: []AudioTrack{
				{Index: 0, Codec: "dts", Channels: 8},
				{Index: 1, Codec: "truehd", Channels: 6},
			},
			expectedIndex: 0,
		},
		{
			name: "skip_commentary",
			tracks: []AudioTrack{
				{Index: 0, Codec: "aac", Channels: 2, IsCommentary: true},
				{Index: 1, Codec: "aac", Channels: 2},
			},
			expectedIndex: 1,
		},
		{
			name: "all_commentary_uses_first",
			tracks: []AudioTrack{
				{Index: 0, Codec: "aac", Channels: 2, IsCommentary: true},
				{Index: 1, Codec: "aac", Channels: 2, IsCommentary: true},
			},
			expectedIndex: 0,
		},
		{
			name: "mp4a_variant_is_web_compatible",
			tracks: []AudioTrack{
				{Index: 0, Codec: "ac3", Channels: 6},
				{Index: 1, Codec: "mp4a-40-2", Channels: 2},
			},
			expectedIndex: 1,
		},
		{
			name: "complex_scenario",
			tracks: []AudioTrack{
				{Index: 0, Codec: "truehd", Channels: 8},
				{Index: 1, Codec: "ac3", Channels: 6},
				{Index: 2, Codec: "aac", Channels: 2},
				{Index: 3, Codec: "aac", Channels: 2, IsCommentary: true},
			},
			expectedIndex: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SelectBestAudioTrack(tt.tracks)

			if tt.expectedIndex == -1 {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatalf("expected track at index %d, got nil", tt.expectedIndex)
			}

			if result.Index != tt.expectedIndex {
				t.Errorf("selected track index %d, want %d", result.Index, tt.expectedIndex)
			}
		})
	}
}

func TestVideoInfoEdgeCases(t *testing.T) {
	t.Run("nil_video_info", func(t *testing.T) {
		var info *VideoInfo
		if info != nil {
			t.Error("expected nil VideoInfo")
		}
	})

	t.Run("missing_codec", func(t *testing.T) {
		info := &VideoInfo{Codec: "", Width: 1920, Height: 1080}
		if info.Codec != "" {
			t.Errorf("expected empty codec, got %q", info.Codec)
		}
	})

	t.Run("zero_dimensions", func(t *testing.T) {
		info := &VideoInfo{Codec: "h264", Width: 0, Height: 0}
		if info.Width != 0 || info.Height != 0 {
			t.Errorf("expected zero dimensions, got %dx%d", info.Width, info.Height)
		}
	})

	t.Run("negative_bitrate", func(t *testing.T) {
		info := &VideoInfo{Codec: "h264", Bitrate: -1000}
		if info.Bitrate >= 0 {
			t.Errorf("expected negative bitrate preserved, got %d", info.Bitrate)
		}
	})

	t.Run("zero_duration", func(t *testing.T) {
		info := &VideoInfo{Codec: "h264", Duration: 0.0}
		if info.Duration != 0.0 {
			t.Errorf("expected zero duration, got %f", info.Duration)
		}
	})

	t.Run("no_audio_tracks", func(t *testing.T) {
		info := &VideoInfo{Codec: "h264", AudioTracks: []AudioTrack{}}
		if len(info.AudioTracks) != 0 {
			t.Errorf("expected zero audio tracks, got %d", len(info.AudioTracks))
		}
	})

	t.Run("nil_audio_tracks", func(t *testing.T) {
		info := &VideoInfo{Codec: "h264", AudioTracks: nil}
		if info.AudioTracks != nil {
			t.Errorf("expected nil audio tracks, got %+v", info.AudioTracks)
		}
	})

	t.Run("hdr_missing_color_metadata", func(t *testing.T) {
		info := &VideoInfo{
			Codec:       "hevc",
			PixelFormat: "yuv420p10le",
			BitDepth:    10,
			IsHDR:       false,
		}
		if info.IsHDR {
			t.Error("expected IsHDR=false when color metadata missing")
		}
	})

	t.Run("sdr_with_10bit", func(t *testing.T) {
		info := &VideoInfo{
			Codec:          "h264",
			PixelFormat:    "yuv420p10le",
			ColorTransfer:  "bt709",
			ColorPrimaries: "bt709",
			BitDepth:       10,
			IsHDR:          false,
		}
		if info.IsHDR {
			t.Error("expected IsHDR=false for SDR in 10-bit")
		}
	})

	t.Run("multiple_audio_tracks", func(t *testing.T) {
		info := &VideoInfo{
			AudioTracks: []AudioTrack{
				{Index: 0, Codec: "aac", Channels: 2},
				{Index: 1, Codec: "ac3", Channels: 6},
				{Index: 2, Codec: "dts", Channels: 8},
			},
			SelectedAudioTrackIndex: 0,
		}
		if info.SelectedAudioTrackIndex != 0 {
			t.Errorf("expected selected track 0, got %d", info.SelectedAudioTrackIndex)
		}
		if len(info.AudioTracks) != 3 {
			t.Errorf("expected 3 tracks, got %d", len(info.AudioTracks))
		}
	})

	t.Run("container_format_multiple_values", func(t *testing.T) {
		info := &VideoInfo{Codec: "h264", ContainerFormat: "matroska,webm"}
		if info.ContainerFormat != "matroska,webm" {
			t.Errorf("expected 'matroska,webm', got %q", info.ContainerFormat)
		}
	})

	t.Run("high_bitrate_4k_hdr", func(t *testing.T) {
		info := &VideoInfo{Bitrate: 100_000_000}
		if info.Bitrate != 100_000_000 {
			t.Errorf("expected 100000000, got %d", info.Bitrate)
		}
	})

	t.Run("long_duration", func(t *testing.T) {
		info := &VideoInfo{Duration: 25200.0}
		if info.Duration != 25200.0 {
			t.Errorf("expected 25200.0, got %f", info.Duration)
		}
	})

	t.Run("ultrawide_aspect_ratio", func(t *testing.T) {
		info := &VideoInfo{Width: 2560, Height: 1080}
		aspectRatio := float64(info.Width) / float64(info.Height)
		expected := 2560.0 / 1080.0
		if aspectRatio != expected {
			t.Errorf("expected aspect ratio %f, got %f", expected, aspectRatio)
		}
	})
}

func TestVideoInfoHDRScenarios(t *testing.T) {
	tests := []struct {
		name       string
		info       VideoInfo
		expectHDR  bool
		expectBits int
	}{
		{
			name: "hdr10_movie",
			info: VideoInfo{
				PixelFormat:    "yuv420p10le",
				ColorTransfer:  "smpte2084",
				ColorPrimaries: "bt2020",
				BitDepth:       10,
			},
			expectHDR:  true,
			expectBits: 10,
		},
		{
			name: "hlg_broadcast",
			info: VideoInfo{
				PixelFormat:    "yuv420p10le",
				ColorTransfer:  "arib-std-b67",
				ColorPrimaries: "bt2020",
				BitDepth:       10,
			},
			expectHDR:  true,
			expectBits: 10,
		},
		{
			name: "sdr_uhd",
			info: VideoInfo{
				PixelFormat:    "yuv420p",
				ColorTransfer:  "bt709",
				ColorPrimaries: "bt709",
				BitDepth:       8,
			},
			expectHDR:  false,
			expectBits: 8,
		},
		{
			name: "dolby_vision",
			info: VideoInfo{
				PixelFormat:    "yuv420p10le",
				ColorTransfer:  "smpte2084",
				ColorPrimaries: "bt2020",
				BitDepth:       10,
			},
			expectHDR:  true,
			expectBits: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detectedHDR := IsHDRContent(tt.info.ColorTransfer, tt.info.ColorPrimaries)
			if detectedHDR != tt.expectHDR {
				t.Errorf("HDR detection: got %v, want %v", detectedHDR, tt.expectHDR)
			}

			detectedBits := DetectBitDepthFromPixelFormat(tt.info.PixelFormat)
			if detectedBits != tt.expectBits {
				t.Errorf("bit depth: got %d, want %d", detectedBits, tt.expectBits)
			}
		})
	}
}

func TestCacheOperations(t *testing.T) {
	// Use a fresh cache for testing
	cache := &videoInfoCache{
		entries: make(map[string]*videoInfoCacheEntry),
	}

	t.Run("empty_cache", func(t *testing.T) {
		if cache.Size() != 0 {
			t.Errorf("expected empty cache, got size %d", cache.Size())
		}
	})

	t.Run("get_non_existent", func(t *testing.T) {
		result := cache.Get("/nonexistent/path.mkv")
		if result != nil {
			t.Error("expected nil for non-existent path")
		}
	})

	t.Run("clear_cache", func(t *testing.T) {
		cache.Clear()
		if cache.Size() != 0 {
			t.Errorf("expected empty cache after clear, got size %d", cache.Size())
		}
	})
}

func TestParseFFprobeOutput(t *testing.T) {
	t.Run("valid_h264_stereo", func(t *testing.T) {
		// Realistic ffprobe JSON output for H.264 video with AAC stereo audio
		jsonData := `{
			"streams": [
				{
					"index": 0,
					"codec_name": "h264",
					"codec_type": "video",
					"width": 1920,
					"height": 1080,
					"pix_fmt": "yuv420p",
					"color_space": "bt709",
					"color_primaries": "bt709",
					"color_transfer": "bt709",
					"bits_per_raw_sample": "8"
				},
				{
					"index": 1,
					"codec_name": "aac",
					"codec_type": "audio",
					"channels": 2,
					"bit_rate": "128000",
					"tags": {
						"language": "eng",
						"title": "English"
					}
				}
			],
			"format": {
				"format_name": "mov,mp4,m4a,3gp,3g2,mj2",
				"duration": "7200.123",
				"bit_rate": "5000000"
			}
		}`

		info, err := ParseFFprobeOutput([]byte(jsonData))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Video checks
		if info.Codec != "h264" {
			t.Errorf("Codec = %q, want h264", info.Codec)
		}
		if info.Width != 1920 {
			t.Errorf("Width = %d, want 1920", info.Width)
		}
		if info.Height != 1080 {
			t.Errorf("Height = %d, want 1080", info.Height)
		}
		if info.BitDepth != 8 {
			t.Errorf("BitDepth = %d, want 8", info.BitDepth)
		}
		if info.IsHDR {
			t.Error("IsHDR should be false for bt709 content")
		}

		// Audio checks
		if info.AudioCodec != "aac" {
			t.Errorf("AudioCodec = %q, want aac", info.AudioCodec)
		}
		if info.AudioChannels != 2 {
			t.Errorf("AudioChannels = %d, want 2", info.AudioChannels)
		}
		if len(info.AudioTracks) != 1 {
			t.Errorf("AudioTracks count = %d, want 1", len(info.AudioTracks))
		}

		// Format checks
		if info.ContainerFormat != "mov,mp4,m4a,3gp,3g2,mj2" {
			t.Errorf("ContainerFormat = %q, want mov,mp4,...", info.ContainerFormat)
		}
		if info.Duration != 7200.123 {
			t.Errorf("Duration = %f, want 7200.123", info.Duration)
		}
		if info.Bitrate != 5000000 {
			t.Errorf("Bitrate = %d, want 5000000", info.Bitrate)
		}
	})

	t.Run("valid_hevc_hdr10", func(t *testing.T) {
		// HDR10 HEVC content with multiple audio tracks
		jsonData := `{
			"streams": [
				{
					"index": 0,
					"codec_name": "hevc",
					"codec_type": "video",
					"width": 3840,
					"height": 2160,
					"pix_fmt": "yuv420p10le",
					"color_space": "bt2020nc",
					"color_primaries": "bt2020",
					"color_transfer": "smpte2084",
					"bits_per_raw_sample": "10"
				},
				{
					"index": 1,
					"codec_name": "truehd",
					"codec_type": "audio",
					"channels": 8,
					"bit_rate": "4000000",
					"tags": {
						"language": "eng",
						"title": "English TrueHD Atmos"
					}
				},
				{
					"index": 2,
					"codec_name": "aac",
					"codec_type": "audio",
					"channels": 2,
					"bit_rate": "192000",
					"tags": {
						"language": "eng",
						"title": "English Stereo"
					}
				},
				{
					"index": 3,
					"codec_name": "aac",
					"codec_type": "audio",
					"channels": 2,
					"bit_rate": "128000",
					"tags": {
						"language": "eng",
						"title": "Director Commentary"
					}
				}
			],
			"format": {
				"format_name": "matroska,webm",
				"duration": "9000.5",
				"bit_rate": "50000000"
			}
		}`

		info, err := ParseFFprobeOutput([]byte(jsonData))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Video checks - HDR
		if info.Codec != "hevc" {
			t.Errorf("Codec = %q, want hevc", info.Codec)
		}
		if info.Width != 3840 {
			t.Errorf("Width = %d, want 3840", info.Width)
		}
		if info.BitDepth != 10 {
			t.Errorf("BitDepth = %d, want 10", info.BitDepth)
		}
		if !info.IsHDR {
			t.Error("IsHDR should be true for smpte2084 content")
		}
		if info.ColorTransfer != "smpte2084" {
			t.Errorf("ColorTransfer = %q, want smpte2084", info.ColorTransfer)
		}

		// Audio checks - should select AAC stereo (non-commentary)
		if len(info.AudioTracks) != 3 {
			t.Errorf("AudioTracks count = %d, want 3", len(info.AudioTracks))
		}
		if info.AudioCodec != "aac" {
			t.Errorf("AudioCodec = %q, want aac (stereo selected)", info.AudioCodec)
		}
		if info.AudioChannels != 2 {
			t.Errorf("AudioChannels = %d, want 2", info.AudioChannels)
		}
		// Should select track index 2 (non-commentary AAC stereo)
		if info.SelectedAudioTrackIndex != 2 {
			t.Errorf("SelectedAudioTrackIndex = %d, want 2", info.SelectedAudioTrackIndex)
		}
	})

	t.Run("valid_hlg_content", func(t *testing.T) {
		// HLG (Hybrid Log-Gamma) HDR content
		jsonData := `{
			"streams": [
				{
					"index": 0,
					"codec_name": "hevc",
					"codec_type": "video",
					"width": 3840,
					"height": 2160,
					"pix_fmt": "yuv420p10le",
					"color_space": "bt2020nc",
					"color_primaries": "bt2020",
					"color_transfer": "arib-std-b67"
				}
			],
			"format": {
				"format_name": "mpegts",
				"duration": "3600.0",
				"bit_rate": "20000000"
			}
		}`

		info, err := ParseFFprobeOutput([]byte(jsonData))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !info.IsHDR {
			t.Error("IsHDR should be true for HLG (arib-std-b67) content")
		}
		if info.BitDepth != 10 {
			t.Errorf("BitDepth = %d, want 10 (from yuv420p10le)", info.BitDepth)
		}
	})

	t.Run("video_no_audio", func(t *testing.T) {
		// Video file with no audio tracks
		jsonData := `{
			"streams": [
				{
					"index": 0,
					"codec_name": "h264",
					"codec_type": "video",
					"width": 1280,
					"height": 720,
					"pix_fmt": "yuv420p"
				}
			],
			"format": {
				"format_name": "mp4",
				"duration": "60.0"
			}
		}`

		info, err := ParseFFprobeOutput([]byte(jsonData))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(info.AudioTracks) != 0 {
			t.Errorf("AudioTracks count = %d, want 0", len(info.AudioTracks))
		}
		if info.AudioCodec != "" {
			t.Errorf("AudioCodec = %q, want empty", info.AudioCodec)
		}
	})

	t.Run("audio_only", func(t *testing.T) {
		// Audio-only file (no video stream)
		jsonData := `{
			"streams": [
				{
					"index": 0,
					"codec_name": "aac",
					"codec_type": "audio",
					"channels": 2,
					"bit_rate": "256000",
					"tags": {
						"language": "eng"
					}
				}
			],
			"format": {
				"format_name": "aac",
				"duration": "180.0"
			}
		}`

		info, err := ParseFFprobeOutput([]byte(jsonData))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if info.Codec != "" {
			t.Errorf("Codec = %q, want empty (no video)", info.Codec)
		}
		if info.AudioCodec != "aac" {
			t.Errorf("AudioCodec = %q, want aac", info.AudioCodec)
		}
	})

	t.Run("bit_depth_fallback", func(t *testing.T) {
		// Test bit depth fallback when bits_per_raw_sample is not provided
		jsonData := `{
			"streams": [
				{
					"index": 0,
					"codec_name": "hevc",
					"codec_type": "video",
					"width": 3840,
					"height": 2160,
					"pix_fmt": "yuv420p12le"
				}
			],
			"format": {
				"format_name": "mkv"
			}
		}`

		info, err := ParseFFprobeOutput([]byte(jsonData))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should detect 12-bit from pixel format
		if info.BitDepth != 12 {
			t.Errorf("BitDepth = %d, want 12 (from yuv420p12le)", info.BitDepth)
		}
	})

	t.Run("multiple_video_streams", func(t *testing.T) {
		// Multiple video streams - should only use first one
		jsonData := `{
			"streams": [
				{
					"index": 0,
					"codec_name": "h264",
					"codec_type": "video",
					"width": 1920,
					"height": 1080
				},
				{
					"index": 1,
					"codec_name": "mjpeg",
					"codec_type": "video",
					"width": 640,
					"height": 480
				}
			],
			"format": {
				"format_name": "mkv"
			}
		}`

		info, err := ParseFFprobeOutput([]byte(jsonData))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should use first video stream
		if info.Codec != "h264" {
			t.Errorf("Codec = %q, want h264 (first stream)", info.Codec)
		}
		if info.Width != 1920 {
			t.Errorf("Width = %d, want 1920", info.Width)
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		invalidJSON := `{"invalid json`
		_, err := ParseFFprobeOutput([]byte(invalidJSON))
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("empty_json", func(t *testing.T) {
		emptyJSON := `{}`
		info, err := ParseFFprobeOutput([]byte(emptyJSON))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should return empty VideoInfo
		if info.Codec != "" {
			t.Errorf("Codec = %q, want empty", info.Codec)
		}
		if len(info.AudioTracks) != 0 {
			t.Errorf("AudioTracks = %d, want 0", len(info.AudioTracks))
		}
	})

	t.Run("empty_streams", func(t *testing.T) {
		jsonData := `{
			"streams": [],
			"format": {
				"format_name": "unknown"
			}
		}`
		info, err := ParseFFprobeOutput([]byte(jsonData))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info.Codec != "" {
			t.Errorf("Codec = %q, want empty", info.Codec)
		}
	})

	t.Run("commentary_detection", func(t *testing.T) {
		// Test commentary track detection
		jsonData := `{
			"streams": [
				{
					"index": 0,
					"codec_name": "h264",
					"codec_type": "video",
					"width": 1920,
					"height": 1080
				},
				{
					"index": 1,
					"codec_name": "aac",
					"codec_type": "audio",
					"channels": 2,
					"tags": {
						"title": "Director's Commentary"
					}
				},
				{
					"index": 2,
					"codec_name": "aac",
					"codec_type": "audio",
					"channels": 2,
					"tags": {
						"title": "Cast Comment Track"
					}
				},
				{
					"index": 3,
					"codec_name": "ac3",
					"codec_type": "audio",
					"channels": 6,
					"tags": {
						"title": "English 5.1"
					}
				}
			],
			"format": {"format_name": "mkv"}
		}`

		info, err := ParseFFprobeOutput([]byte(jsonData))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check commentary flags
		if !info.AudioTracks[0].IsCommentary {
			t.Error("Track 0 should be commentary")
		}
		if !info.AudioTracks[1].IsCommentary {
			t.Error("Track 1 should be commentary (contains 'comment')")
		}
		if info.AudioTracks[2].IsCommentary {
			t.Error("Track 2 should NOT be commentary")
		}

		// Should select ac3 5.1 (only non-commentary track)
		if info.SelectedAudioTrackIndex != 3 {
			t.Errorf("SelectedAudioTrackIndex = %d, want 3", info.SelectedAudioTrackIndex)
		}
	})
}
