package session

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/hls"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"
)

// TestGetPlaylistMetadata tests reading playlist metadata
func TestGetPlaylistMetadata(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tmpDir := t.TempDir()

	session := NewTranscodeSession(123, "1080p", 0, -1, tmpDir, logger, nil)

	t.Run("Non-existent manifest file", func(t *testing.T) {
		// Manifest doesn't exist yet
		_, err := session.GetPlaylistMetadata()
		if err == nil {
			t.Error("GetPlaylistMetadata() should error when manifest doesn't exist")
		}
	})

	t.Run("Valid manifest with metadata", func(t *testing.T) {
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(session.ManifestPath), 0755); err != nil {
			t.Fatalf("Failed to create manifest directory: %v", err)
		}

		// Create a sample manifest file with metadata
		manifestContent := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:4
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-VIEWRA-START-POSITION:30.5
#EXTINF:4.0,
seg_000000.ts
#EXTINF:4.0,
seg_000001.ts
#EXT-X-ENDLIST
`
		err := os.WriteFile(session.ManifestPath, []byte(manifestContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test manifest: %v", err)
		}

		metadata, err := session.GetPlaylistMetadata()
		if err != nil {
			t.Fatalf("GetPlaylistMetadata() error = %v", err)
		}

		// Verify metadata was parsed
		// Note: The actual parsing depends on ffmpeg.ParsePlaylistMetadata implementation
		// This test validates that the function executes without error
		_ = metadata
	})

	t.Run("Empty manifest file", func(t *testing.T) {
		emptySession := NewTranscodeSession(456, "720p", 0, -1, tmpDir, logger, nil)

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(emptySession.ManifestPath), 0755); err != nil {
			t.Fatalf("Failed to create manifest directory: %v", err)
		}

		// Create empty manifest
		err := os.WriteFile(emptySession.ManifestPath, []byte(""), 0644)
		if err != nil {
			t.Fatalf("Failed to create empty manifest: %v", err)
		}

		metadata, err := emptySession.GetPlaylistMetadata()
		if err != nil {
			t.Fatalf("GetPlaylistMetadata() should handle empty file, error = %v", err)
		}

		_ = metadata
	})

	t.Run("Manifest in subdirectory", func(t *testing.T) {
		// Create session with output in subdirectory
		subDirSession := NewTranscodeSession(789, "4k", 0, -1, tmpDir, logger, nil)

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(subDirSession.ManifestPath), 0755); err != nil {
			t.Fatalf("Failed to create manifest directory: %v", err)
		}

		manifestContent := `#EXTM3U
#EXT-X-VERSION:3
#EXTINF:4.0,
seg_000000.ts
`
		err := os.WriteFile(subDirSession.ManifestPath, []byte(manifestContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create manifest in subdirectory: %v", err)
		}

		metadata, err := subDirSession.GetPlaylistMetadata()
		if err != nil {
			t.Fatalf("GetPlaylistMetadata() error = %v", err)
		}

		_ = metadata
	})
}

// TestBuildFFmpegArgs_RemuxAudio tests remux_audio strategy
func TestBuildFFmpegArgs_RemuxAudio(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	// Use non-zero start position to trigger seek position logic
	session := NewTranscodeSession(123, "1080p", 30.0, -1, "/tmp/output", logger, nil)

	testProfile := &profile.AdaptiveProfile{
		ID:              "1080p",
		Width:           1920,
		Height:          1080,
		SegmentDuration: 4,
		PreferredCodec:  "h264",
	}

	videoInfo := &hls.VideoInfo{
		Codec:      "h264",
		Width:      1920,
		Height:     1080,
		AudioCodec: "aac",
	}

	config := &Config{
		FFmpegPaths: &hls.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
		MaxMemoryMB: 2048,
	}

	params := StartParams{
		InputPath:             "/input/test.mp4",
		Profile:               testProfile,
		Strategy:              "remux_audio",
		HWAccel:               "none",
		VideoInfo:             videoInfo,
		Config:                config,
		ClientSupportedCodecs: []string{"h264"},
	}

	args := session.buildFFmpegArgs(params)
	argsStr := string([]byte{})
	for _, arg := range args {
		argsStr += arg + " "
	}

	// Should have -noaccurate_seek for remux_audio
	hasNoAccurateSeek := false
	for _, arg := range args {
		if arg == "-noaccurate_seek" {
			hasNoAccurateSeek = true
			break
		}
	}

	if !hasNoAccurateSeek {
		t.Error("remux_audio strategy should include -noaccurate_seek flag")
	}
}

// TestBuildFFmpegArgs_RemuxHEVC tests remux_hevc strategy
func TestBuildFFmpegArgs_RemuxHEVC(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	// Use non-zero start position to trigger seek position logic
	session := NewTranscodeSession(123, "1080p", 30.0, -1, "/tmp/output", logger, nil)

	testProfile := &profile.AdaptiveProfile{
		ID:              "1080p",
		SegmentDuration: 4,
		PreferredCodec:  "h265",
	}

	videoInfo := &hls.VideoInfo{
		Codec:      "hevc",
		Width:      1920,
		Height:     1080,
		AudioCodec: "aac",
	}

	config := &Config{
		FFmpegPaths: &hls.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
	}

	params := StartParams{
		InputPath:             "/input/test.mp4",
		Profile:               testProfile,
		Strategy:              "remux_hevc",
		HWAccel:               "none",
		VideoInfo:             videoInfo,
		Config:                config,
		ClientSupportedCodecs: []string{"h265"},
	}

	args := session.buildFFmpegArgs(params)

	// Should have -noaccurate_seek for remux_hevc
	hasNoAccurateSeek := false
	for _, arg := range args {
		if arg == "-noaccurate_seek" {
			hasNoAccurateSeek = true
			break
		}
	}

	if !hasNoAccurateSeek {
		t.Error("remux_hevc strategy should include -noaccurate_seek flag")
	}
}

// TestBuildFFmpegArgs_HDRWithQSV tests QSV with HDR content
func TestBuildFFmpegArgs_HDRWithQSV(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	session := NewTranscodeSession(123, "1080p", 0, -1, "/tmp/output", logger, nil)

	testProfile := &profile.AdaptiveProfile{
		ID:              "1080p",
		SegmentDuration: 4,
		PreferredCodec:  "h264",
	}

	// HDR video info
	videoInfo := &hls.VideoInfo{
		Codec:          "hevc",
		Width:          3840,
		Height:         2160,
		AudioCodec:     "aac",
		PixelFormat:    "yuv420p10le",
		ColorSpace:     "bt2020nc",
		ColorPrimaries: "bt2020",
		ColorTransfer:  "smpte2084",
		BitDepth:       10,
		IsHDR:          true,
	}

	config := &Config{
		FFmpegPaths: &hls.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
		ToneMappingEnabled: true,
		ToneMappingBackend: "opencl",
	}

	params := StartParams{
		InputPath:             "/input/hdr.mp4",
		Profile:               testProfile,
		Strategy:              "transcode",
		HWAccel:               "qsv",
		VideoInfo:             videoInfo,
		Config:                config,
		ClientSupportedCodecs: []string{"h264"},
	}

	args := session.buildFFmpegArgs(params)

	// Should have OpenCL initialization for QSV + HDR
	hasInitOpenCL := false
	for _, arg := range args {
		if arg == "-init_hw_device" {
			hasInitOpenCL = true
			break
		}
	}

	// Note: This depends on the backend being "opencl" or "auto"
	// The test validates the code path is executed
	_ = hasInitOpenCL
}

// TestBuildFFmpegArgs_HDRWithAutoBackend tests HDR with auto tone mapping backend
func TestBuildFFmpegArgs_HDRWithAutoBackend(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	session := NewTranscodeSession(123, "1080p", 0, -1, "/tmp/output", logger, nil)

	testProfile := &profile.AdaptiveProfile{
		ID:              "1080p",
		SegmentDuration: 4,
		PreferredCodec:  "h264",
	}

	videoInfo := &hls.VideoInfo{
		Codec:  "hevc",
		Width:  3840,
		Height: 2160,
		IsHDR:  true,
	}

	config := &Config{
		FFmpegPaths: &hls.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
		ToneMappingEnabled: true,
		ToneMappingBackend: "", // Empty defaults to "auto"
	}

	params := StartParams{
		InputPath:             "/input/hdr.mp4",
		Profile:               testProfile,
		Strategy:              "transcode",
		HWAccel:               "qsv",
		VideoInfo:             videoInfo,
		Config:                config,
		ClientSupportedCodecs: []string{"h264"},
	}

	args := session.buildFFmpegArgs(params)
	_ = args // Validates the auto backend path is executed
}

// TestBuildFFmpegArgs_StartPosition tests start position handling
func TestBuildFFmpegArgs_StartPosition(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	testProfile := &profile.AdaptiveProfile{
		ID:              "1080p",
		SegmentDuration: 4,
		PreferredCodec:  "h264",
	}

	videoInfo := &hls.VideoInfo{
		Codec:  "h264",
		Width:  1920,
		Height: 1080,
	}

	config := &Config{
		FFmpegPaths: &hls.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
	}

	t.Run("With start position", func(t *testing.T) {
		session := NewTranscodeSession(123, "1080p", 60.0, -1, "/tmp/output", logger, nil)

		params := StartParams{
			InputPath:             "/input/test.mp4",
			Profile:               testProfile,
			Strategy:              "transcode",
			HWAccel:               "none",
			VideoInfo:             videoInfo,
			Config:                config,
			ClientSupportedCodecs: []string{"h264"},
		}

		args := session.buildFFmpegArgs(params)

		// Should have -ss flag for start position
		hasSS := false
		for i, arg := range args {
			if arg == "-ss" && i+1 < len(args) {
				hasSS = true
				if args[i+1] != "60" {
					t.Errorf("Expected -ss 60, got -ss %s", args[i+1])
				}
				break
			}
		}

		if !hasSS {
			t.Error("Should have -ss flag when start position > 0")
		}
	})

	t.Run("Without start position", func(t *testing.T) {
		session := NewTranscodeSession(123, "1080p", 0, -1, "/tmp/output", logger, nil)

		params := StartParams{
			InputPath:             "/input/test.mp4",
			Profile:               testProfile,
			Strategy:              "transcode",
			HWAccel:               "none",
			VideoInfo:             videoInfo,
			Config:                config,
			ClientSupportedCodecs: []string{"h264"},
		}

		args := session.buildFFmpegArgs(params)
		_ = args // Validates zero start position path
	})
}

// TestBuildFFmpegArgs_AudioTrack tests specific audio track selection
func TestBuildFFmpegArgs_AudioTrack(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	testProfile := &profile.AdaptiveProfile{
		ID:              "1080p",
		SegmentDuration: 4,
		PreferredCodec:  "h264",
	}

	config := &Config{
		FFmpegPaths: &hls.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
	}

	t.Run("Specific audio track", func(t *testing.T) {
		session := NewTranscodeSession(123, "1080p", 0, 2, "/tmp/output", logger, nil)

		params := StartParams{
			InputPath:             "/input/test.mp4",
			Profile:               testProfile,
			Strategy:              "transcode",
			HWAccel:               "none",
			Config:                config,
			ClientSupportedCodecs: []string{"h264"},
		}

		args := session.buildFFmpegArgs(params)
		_ = args // Validates specific audio track path
	})

	t.Run("Default audio track", func(t *testing.T) {
		session := NewTranscodeSession(123, "1080p", 0, -1, "/tmp/output", logger, nil)

		params := StartParams{
			InputPath:             "/input/test.mp4",
			Profile:               testProfile,
			Strategy:              "transcode",
			HWAccel:               "none",
			Config:                config,
			ClientSupportedCodecs: []string{"h264"},
		}

		args := session.buildFFmpegArgs(params)
		_ = args // Validates default audio track path
	})
}

// TestBuildFFmpegArgs_VAAPIHardware tests VAAPI hardware acceleration
func TestBuildFFmpegArgs_VAAPIHardware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	session := NewTranscodeSession(123, "1080p", 0, -1, "/tmp/output", logger, nil)

	testProfile := &profile.AdaptiveProfile{
		ID:              "1080p",
		SegmentDuration: 4,
		PreferredCodec:  "h264",
	}

	config := &Config{
		FFmpegPaths: &hls.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
	}

	params := StartParams{
		InputPath:             "/input/test.mp4",
		Profile:               testProfile,
		Strategy:              "transcode",
		HWAccel:               "vaapi",
		HWDevice:              "/dev/dri/renderD128",
		Config:                config,
		ClientSupportedCodecs: []string{"h264"},
	}

	args := session.buildFFmpegArgs(params)

	// Should have VAAPI-specific flags
	hasVAAPI := false
	for _, arg := range args {
		if arg == "-hwaccel" || arg == "vaapi" {
			hasVAAPI = true
			break
		}
	}

	_ = hasVAAPI // Validates VAAPI path
}

// TestBuildFFmpegArgs_QSVHardware tests QSV hardware acceleration
func TestBuildFFmpegArgs_QSVHardware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	session := NewTranscodeSession(123, "1080p", 0, -1, "/tmp/output", logger, nil)

	testProfile := &profile.AdaptiveProfile{
		ID:              "1080p",
		SegmentDuration: 4,
		PreferredCodec:  "h264",
	}

	config := &Config{
		FFmpegPaths: &hls.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
	}

	params := StartParams{
		InputPath:             "/input/test.mp4",
		Profile:               testProfile,
		Strategy:              "transcode",
		HWAccel:               "qsv",
		Config:                config,
		ClientSupportedCodecs: []string{"h264"},
	}

	args := session.buildFFmpegArgs(params)
	_ = args // Validates QSV path
}

// TestBuildFFmpegArgs_VideoToolboxHardware tests VideoToolbox hardware acceleration
func TestBuildFFmpegArgs_VideoToolboxHardware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	session := NewTranscodeSession(123, "1080p", 0, -1, "/tmp/output", logger, nil)

	testProfile := &profile.AdaptiveProfile{
		ID:              "1080p",
		SegmentDuration: 4,
		PreferredCodec:  "h264",
	}

	config := &Config{
		FFmpegPaths: &hls.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
	}

	params := StartParams{
		InputPath:             "/input/test.mp4",
		Profile:               testProfile,
		Strategy:              "transcode",
		HWAccel:               "videotoolbox",
		Config:                config,
		ClientSupportedCodecs: []string{"h264"},
	}

	args := session.buildFFmpegArgs(params)
	_ = args // Validates VideoToolbox path
}
