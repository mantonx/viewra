package session

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/hls"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"
)

// TestSelectBestCodec tests codec selection based on client capabilities and hardware support
func TestSelectBestCodec(t *testing.T) {
	tests := []struct {
		name                  string
		profile               *profile.AdaptiveProfile
		clientSupportedCodecs []string
		hwAccel               string
		expectedCodec         hls.VideoCodec
	}{
		{
			name:                  "nil profile returns H.264",
			profile:               nil,
			clientSupportedCodecs: []string{"h264", "h265"},
			hwAccel:               "none",
			expectedCodec:         hls.CodecH264,
		},
		{
			name: "empty client codecs defaults to H.264",
			profile: &profile.AdaptiveProfile{
				PreferredCodec: "h265",
				FallbackCodecs: []string{"h264"},
			},
			clientSupportedCodecs: []string{},
			hwAccel:               "none",
			expectedCodec:         hls.CodecH264,
		},
		{
			name: "client supports preferred codec and hardware supports it",
			profile: &profile.AdaptiveProfile{
				PreferredCodec: "h265",
				FallbackCodecs: []string{"h264"},
			},
			clientSupportedCodecs: []string{"h264", "h265"},
			hwAccel:               "nvenc",
			expectedCodec:         hls.CodecH265,
		},
		{
			name: "client doesn't support preferred codec, falls back to first supported",
			profile: &profile.AdaptiveProfile{
				PreferredCodec: "av1",
				FallbackCodecs: []string{"h265", "h264"},
			},
			clientSupportedCodecs: []string{"h264", "h265"},
			hwAccel:               "none",
			expectedCodec:         hls.CodecH265,
		},
		{
			name: "hardware doesn't support preferred codec, uses fallback",
			profile: &profile.AdaptiveProfile{
				PreferredCodec: "vp9",
				FallbackCodecs: []string{"h264"},
			},
			clientSupportedCodecs: []string{"vp9", "h264"},
			hwAccel:               "nvenc", // NVENC doesn't support VP9
			expectedCodec:         hls.CodecH264,
		},
		{
			name: "VP9 with VAAPI (supported)",
			profile: &profile.AdaptiveProfile{
				PreferredCodec: "vp9",
				FallbackCodecs: []string{"h264"},
			},
			clientSupportedCodecs: []string{"vp9", "h264"},
			hwAccel:               "vaapi",
			expectedCodec:         hls.CodecVP9,
		},
		{
			name: "AV1 with client and hardware support",
			profile: &profile.AdaptiveProfile{
				PreferredCodec: "av1",
				FallbackCodecs: []string{"h265", "h264"},
			},
			clientSupportedCodecs: []string{"av1", "h265", "h264"},
			hwAccel:               "nvenc",
			expectedCodec:         hls.CodecAV1,
		},
		{
			name: "no matching codecs ultimate fallback to H.264",
			profile: &profile.AdaptiveProfile{
				PreferredCodec: "av1",
				FallbackCodecs: []string{"vp9"},
			},
			clientSupportedCodecs: []string{"h264"},
			hwAccel:               "none",
			expectedCodec:         hls.CodecH264,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectBestCodec(tt.profile, tt.clientSupportedCodecs, tt.hwAccel)
			if result != tt.expectedCodec {
				t.Errorf("selectBestCodec() = %v, want %v", result, tt.expectedCodec)
			}
		})
	}
}

// TestCreateFFmpegCommand tests FFmpeg command creation with and without systemd-run
func TestCreateFFmpegCommand(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name        string
		maxMemoryMB int
		libPath     string
		checkCmd    func(t *testing.T, args []string, path string)
	}{
		{
			name:        "Linux with systemd-run and memory limit",
			maxMemoryMB: 2048,
			checkCmd: func(t *testing.T, args []string, path string) {
				if runtime.GOOS == "linux" {
					if _, err := os.Stat("/usr/bin/systemd-run"); err == nil {
						argsStr := strings.Join(args, " ")
						if strings.Contains(argsStr, "MemoryMax") {
							t.Log("systemd-run is being used with memory limits")
						}
					}
				}
			},
		},
		{
			name:        "No memory limit uses regular exec",
			maxMemoryMB: 0,
			checkCmd: func(t *testing.T, args []string, path string) {
				if path == "systemd-run" {
					t.Error("Should not use systemd-run when maxMemoryMB is 0")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				FFmpegPaths: &hls.Paths{
					FFmpeg:  "/usr/bin/ffmpeg",
					FFprobe: "/usr/bin/ffprobe",
					LibPath: tt.libPath,
				},
				MaxMemoryMB: tt.maxMemoryMB,
			}

			ctx := context.Background()
			args := []string{"-i", "input.mp4", "output.m3u8"}

			cmd := createFFmpegCommand(ctx, args, config, logger)

			if cmd == nil {
				t.Fatal("createFFmpegCommand returned nil")
			}

			if tt.checkCmd != nil {
				tt.checkCmd(t, cmd.Args, cmd.Path)
			}

			if runtime.GOOS != "windows" {
				if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
					t.Error("Process group should be set for clean shutdown")
				}
			}
		})
	}
}

// TestSessionBuildFFmpegArgs tests the buildFFmpegArgs method
func TestSessionBuildFFmpegArgs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	session := NewTranscodeSession(123, "1080p-8m", 0, -1, "/tmp/output", logger, nil)

	testProfile := &profile.AdaptiveProfile{
		ID:              "1080p-8m",
		Width:           1920,
		Height:          1080,
		VideoBitrate:    8_000_000,
		VideoMaxRate:    8_800_000,
		VideoBufSize:    16_000_000,
		AudioBitrate:    192_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		GOPSize:         60,
		SegmentDuration: 4,
		PreferredCodec:  "h264",
		FallbackCodecs:  []string{},
	}

	videoInfo := &hls.VideoInfo{
		Codec:         "h264",
		Width:         1920,
		Height:        1080,
		AudioCodec:    "aac",
		AudioChannels: 2,
		IsHDR:         false,
	}

	config := &Config{
		FFmpegPaths: &hls.Paths{
			FFmpeg:  "/usr/bin/ffmpeg",
			FFprobe: "/usr/bin/ffprobe",
		},
		MaxMemoryMB:          2048,
		ToneMappingEnabled:   false,
		ToneMappingAlgorithm: "bt.2390",
		ToneMappingBackend:   "auto",
	}

	t.Run("Remux strategy uses segment muxer", func(t *testing.T) {
		params := StartParams{
			InputPath:             "/input/test.mp4",
			Profile:               testProfile,
			Strategy:              "remux",
			HWAccel:               "none",
			HWDevice:              "",
			VideoInfo:             videoInfo,
			Config:                config,
			ClientSupportedCodecs: []string{"h264"},
		}
		args := session.buildFFmpegArgs(params)
		argsStr := strings.Join(args, " ")
		if !strings.Contains(argsStr, "-f segment") {
			t.Error("Remux should use segment muxer format")
		}
		if !strings.Contains(argsStr, "-c:a copy") {
			t.Error("Remux should copy audio codec")
		}
	})

	t.Run("Transcode strategy uses HLS muxer", func(t *testing.T) {
		params := StartParams{
			InputPath:             "/input/test.mp4",
			Profile:               testProfile,
			Strategy:              "transcode",
			HWAccel:               "none",
			HWDevice:              "",
			VideoInfo:             videoInfo,
			Config:                config,
			ClientSupportedCodecs: []string{"h264"},
		}
		args := session.buildFFmpegArgs(params)
		argsStr := strings.Join(args, " ")
		if !strings.Contains(argsStr, "-f hls") {
			t.Error("Transcode should use HLS muxer")
		}
		if !strings.Contains(argsStr, "-c:v libx264") {
			t.Error("Transcode should use libx264 codec")
		}
		if !strings.Contains(argsStr, "-preset veryfast") {
			t.Error("Transcode should use veryfast preset for real-time")
		}
	})

	t.Run("NVENC hardware acceleration", func(t *testing.T) {
		params := StartParams{
			InputPath:             "/input/test.mp4",
			Profile:               testProfile,
			Strategy:              "transcode",
			HWAccel:               "nvenc",
			HWDevice:              "",
			VideoInfo:             videoInfo,
			Config:                config,
			ClientSupportedCodecs: []string{"h264"},
		}
		args := session.buildFFmpegArgs(params)
		argsStr := strings.Join(args, " ")
		if !strings.Contains(argsStr, "-hwaccel cuda") {
			t.Error("NVENC should use CUDA hardware acceleration")
		}
		if !strings.Contains(argsStr, "-c:v h264_nvenc") {
			t.Error("NVENC should use h264_nvenc encoder")
		}
	})

	t.Run("Memory safety options present", func(t *testing.T) {
		params := StartParams{
			InputPath:             "/input/test.mp4",
			Profile:               testProfile,
			Strategy:              "transcode",
			HWAccel:               "none",
			HWDevice:              "",
			VideoInfo:             videoInfo,
			Config:                config,
			ClientSupportedCodecs: []string{"h264"},
		}
		args := session.buildFFmpegArgs(params)
		argsStr := strings.Join(args, " ")
		if !strings.Contains(argsStr, "-max_alloc") {
			t.Error("Should have -max_alloc flag for memory safety")
		}
	})
}
