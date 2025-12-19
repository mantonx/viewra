package session

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"syscall"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/hls"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"
)

// buildFFmpegArgs builds the FFmpeg command arguments for progressive HLS transcoding.
// Supports hardware acceleration for Transcode strategy.
func (s *TranscodeSession) buildFFmpegArgs(params StartParams) []string {
	// Determine target codec based on client support and profile preferences
	targetCodec := selectBestCodec(params.Profile, params.ClientSupportedCodecs, params.HWAccel)

	s.logger.Debug("Selected target codec for transcoding",
		"session_id", s.ID,
		"target_codec", targetCodec,
		"profile_preferred", params.Profile.PreferredCodec,
		"profile_fallbacks", params.Profile.FallbackCodecs,
		"client_codecs", params.ClientSupportedCodecs,
		"hw_accel", params.HWAccel)

	// Create FFmpeg options
	ffmpegOpts := hls.Options{
		InputPath:                  params.InputPath,
		OutputDir:                  s.OutputDir,
		Profile:                    convertToHLSProfile(params.Profile),
		AudioTrackIndex:            s.AudioTrackIndex,
		UseSpecificAudioTrack:      s.AudioTrackIndex >= 0,
		StartPosition:              int(s.StartPosition),
		UseStartPosition:           s.StartPosition > 0,
		VideoInfo:                  convertToHLSVideoInfo(params.VideoInfo),
		ToneMappingEnabled:         params.Config.ToneMappingEnabled,
		ToneMappingAlgorithm:       params.Config.ToneMappingAlgorithm,
		ToneMappingBackend:         params.Config.ToneMappingBackend,
		LibPlaceboPeakDetect:       params.Config.LibPlaceboPeakDetect,
		LibPlaceboContrastRecovery: params.Config.LibPlaceboContrastRecovery,
		VideoCodec:                 targetCodec,
	}
	builder := hls.NewBuilder(ffmpegOpts)

	hwAccel := hls.HardwareAccel(params.HWAccel)
	strategy := params.Strategy

	// Add hardware acceleration args (if not None)
	if hwAccel != hls.AccelNone && strategy == "transcode" {
		builder.AddHardwareAccel(hls.GetHardwareAccelArgsWithDevice(hwAccel, params.HWDevice))

		// For NVENC/QSV with HDR content, initialize GPU device for tone mapping
		// Backend selection: vulkan (consistent startup), opencl (default), cuda (fastest if available)
		if (hwAccel == hls.AccelNVENC || hwAccel == hls.AccelQSV) && params.Config.ToneMappingEnabled && params.VideoInfo != nil && params.VideoInfo.IsHDR {
			backend := params.Config.ToneMappingBackend
			if backend == "" {
				backend = "auto"
			}
			switch backend {
			case "vulkan":
				// Vulkan + libplacebo: consistent ~1.9s startup, best quality bt.2390
				builder.AddVulkanDevice().AddVulkanFilterDevice()
			case "opencl", "auto":
				// OpenCL: 3s first run, ~1s cached (default)
				builder.AddOpenCLDevice().AddOpenCLFilterDevice()
			}
			// cuda and libplacebo (CPU) backends don't need device initialization here
		}
	}

	// Add memory safety options
	builder.AddMemorySafetyOptions(params.Config.MaxMemoryMB)

	// Determine if we need -noaccurate_seek for A/V sync and faster seeking.
	// For all remux strategies (pure remux, remux_audio, remux_hevc), we want to:
	// 1. Skip to the nearest keyframe immediately (faster startup)
	// 2. Align audio with video keyframes (proper A/V sync)
	isRemuxStrategy := strategy == "remux" || strategy == "remux_audio" || strategy == "remux_hevc"

	// Add fast input options to reduce startup latency by limiting input analysis.
	// This applies to BOTH remux and transcode strategies - saves 500ms-1s on startup.
	// The -analyzeduration and -probesize limits are safe for transcoding since
	// we only need enough data to detect streams, not full container analysis.
	builder.AddFastInputOptions()

	builder.AddSeekPosition(int(s.StartPosition))
	// Use -noaccurate_seek for remux strategies only.
	// For transcode, accurate seeking is fine since we re-encode anyway,
	// and the fast input options already provide the main startup benefit.
	if isRemuxStrategy {
		builder.AddNoAccurateSeek()
	}
	builder.AddInput()

	// Only add timestamp handling for transcode (HLS muxer).
	// For remux strategies using the segment muxer, -copyts -start_at_zero
	// causes incorrect PTS calculations that prevent segment splitting.
	// The segment muxer handles timestamps correctly without these flags.
	if !isRemuxStrategy {
		builder.AddTimestampHandling()
	}

	// Add encoding based on strategy
	useSegmentMuxer := false

	switch strategy {
	case "remux":
		builder.AddStreamMapping().AddH264Copy().AddAudioCodec("copy")
		useSegmentMuxer = true

	case "remux_audio":
		builder.AddStreamMapping().AddH264Copy().AddAudioDownmix()
		useSegmentMuxer = true

	case "remux_hevc":
		s.logger.Info("Using HEVC remux strategy",
			"session_id", s.ID,
			"media_id", s.MediaID)
		builder.AddStreamMapping().AddHEVCCopy().AddAudioDownmix()
		useSegmentMuxer = true

	case "transcode":
		videoEncoder, videoPreset := hls.GetVideoCodecAndPresetForCodec(hwAccel, targetCodec)

		// For real-time progressive transcoding, override software preset to veryfast
		if hwAccel == hls.AccelNone {
			videoPreset = "veryfast"
		}

		s.logger.Debug("Selected video encoder",
			"session_id", s.ID,
			"target_codec", targetCodec,
			"encoder", videoEncoder,
			"preset", videoPreset,
			"hw_accel", hwAccel)

		builder.AddStreamMapping().AddVideoCodec(videoEncoder, videoPreset)

		if hwAccel != hls.AccelNone {
			builder.AddHardwareVideoEncoding(hwAccel)
		} else {
			builder.AddVideoEncoding()
		}

		builder.AddAudioEncoding()
	}

	// Add output settings based on muxer type
	var args []string
	if useSegmentMuxer {
		builder.AddSegmentMuxerOutput().AddOverwriteOutput().AddSegmentMuxerOutputFile()
		args = builder.Build()
	} else {
		builder.AddHLSOutput().AddOverwriteOutput()
		args = builder.Build()
		args = append(args, s.ManifestPath)
	}

	return args
}

// createFFmpegCommand creates an FFmpeg command with memory limits via systemd-run (if available).
func createFFmpegCommand(ctx context.Context, args []string, config *Config, logger *slog.Logger) *exec.Cmd {
	paths := config.FFmpegPaths
	maxMemoryMB := config.MaxMemoryMB

	// On Linux with systemd, use systemd-run to apply memory limits
	if runtime.GOOS == "linux" && maxMemoryMB > 0 {
		if _, err := exec.LookPath("systemd-run"); err == nil {
			limitMB := maxMemoryMB * 2

			systemdArgs := []string{
				"--scope",
				"--user",
				"-p", fmt.Sprintf("MemoryMax=%dM", limitMB),
				"-p", "MemorySwapMax=0",
			}

			if paths.LibPath != "" {
				systemdArgs = append(systemdArgs, "-E", "LD_LIBRARY_PATH="+paths.LibPath)
			}

			systemdArgs = append(systemdArgs, "--", paths.FFmpeg)
			systemdArgs = append(systemdArgs, args...)

			logger.Debug("Using systemd-run for memory-limited FFmpeg",
				"memory_limit_mb", limitMB,
				"ffmpeg_max_alloc_mb", maxMemoryMB,
				"ffmpeg_path", paths.FFmpeg,
				"lib_path", paths.LibPath)

			cmd := exec.CommandContext(ctx, "systemd-run", systemdArgs...)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Setpgid: true,
			}
			return cmd
		}
	}

	// Fallback: use Paths.PrepareCommand
	cmd := paths.PrepareCommand("ffmpeg", args...)
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
	}
	return cmd
}

// selectBestCodec selects the best codec for transcoding based on client support.
func selectBestCodec(p *profile.AdaptiveProfile, clientSupportedCodecs []string, hwAccel string) hls.VideoCodec {
	if len(clientSupportedCodecs) == 0 {
		clientSupportedCodecs = []string{"h264"}
	}

	supported := make(map[string]bool)
	for _, codec := range clientSupportedCodecs {
		supported[codec] = true
	}

	if p == nil {
		return hls.CodecH264
	}

	hw := hls.HardwareAccel(hwAccel)

	if supported[p.PreferredCodec] && hls.IsCodecSupported(hw, hls.VideoCodec(p.PreferredCodec)) {
		return hls.VideoCodec(p.PreferredCodec)
	}

	for _, fallback := range p.FallbackCodecs {
		if supported[fallback] && hls.IsCodecSupported(hw, hls.VideoCodec(fallback)) {
			return hls.VideoCodec(fallback)
		}
	}

	return hls.CodecH264
}

// convertToHLSVideoInfo passes through hls.VideoInfo (already the correct type)
func convertToHLSVideoInfo(v *hls.VideoInfo) *hls.VideoInfo {
	return v
}

// convertToHLSProfile converts a profile.AdaptiveProfile to hls.Profile
func convertToHLSProfile(p *profile.AdaptiveProfile) *hls.Profile {
	if p == nil {
		return nil
	}
	return &hls.Profile{
		ID:               p.ID,
		DisplayName:      p.DisplayName,
		Width:            p.Width,
		Height:           p.Height,
		VideoBitrate:     p.VideoBitrate,
		VideoMaxRate:     p.VideoMaxRate,
		VideoBufSize:     p.VideoBufSize,
		AudioBitrate:     p.AudioBitrate,
		AudioChannels:    p.AudioChannels,
		AudioSampleRate:  p.AudioSampleRate,
		PreserveMultiCh:  p.PreserveMultiCh,
		AudioCodec:       p.AudioCodec,
		MaxAudioChannels: p.MaxAudioChannels,
		PreferredCodec:   p.PreferredCodec,
		FallbackCodecs:   p.FallbackCodecs,
		Preset:           p.Preset,
		CRF:              p.CRF,
		EnableHWAccel:    p.EnableHWAccel,
		EnableFastStart:  p.EnableFastStart,
		SegmentDuration:  p.SegmentDuration,
		GOPSize:          p.GOPSize,
		FrameRate:        p.FrameRate,
		AspectRatio:      p.AspectRatio,
		Is3D:             p.Is3D,
		StereoMode:       p.StereoMode,
	}
}
