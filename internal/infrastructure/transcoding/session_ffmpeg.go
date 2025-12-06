package transcoding

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"syscall"
)

// buildFFmpegArgs builds the FFmpeg command arguments for progressive HLS transcoding.
// Supports hardware acceleration for Transcode strategy.
// clientSupportedCodecs is used to select the best codec from the profile's preferred/fallback list.
func (s *TranscodeSession) buildFFmpegArgs(inputPath string, profile *AdaptiveProfile, strategy StreamStrategy, hwAccel HardwareAccel, hwDevice string, videoInfo *VideoInfo, config *TranscodeConfig, clientSupportedCodecs []string) []string {
	// Determine target codec based on client support and profile preferences
	// This ensures we encode to a codec the client can actually play
	targetCodec := selectBestCodec(profile, clientSupportedCodecs, hwAccel)

	s.logger.Debug("Selected target codec for transcoding",
		"session_id", s.ID,
		"target_codec", targetCodec,
		"profile_preferred", profile.PreferredCodec,
		"profile_fallbacks", profile.FallbackCodecs,
		"client_codecs", clientSupportedCodecs,
		"hw_accel", hwAccel)

	// Create TranscodeOptions from session data
	opts := TranscodeOptions{
		InputPath:                  inputPath,
		OutputDir:                  s.OutputDir,
		Profile:                    profile,
		UseStartPosition:           s.StartPosition > 0,
		StartPosition:              int(s.StartPosition),
		AudioTrackIndex:            s.AudioTrackIndex,
		UseSpecificAudioTrack:      s.AudioTrackIndex >= 0,
		VideoInfo:                  videoInfo,
		ToneMappingEnabled:         config.ToneMappingEnabled,
		ToneMappingAlgorithm:       config.ToneMappingAlgorithm,
		ToneMappingBackend:         config.ToneMappingBackend,
		LibPlaceboPeakDetect:       config.LibPlaceboPeakDetect,
		LibPlaceboContrastRecovery: config.LibPlaceboContrastRecovery,
		VideoCodec:                 targetCodec,
	}

	// Build arguments based on strategy
	builder := NewFFmpegArgsBuilder(opts)

	// Add hardware acceleration args (if not None)
	if hwAccel != AccelNone && strategy == Transcode {
		builder.AddHardwareAccel(GetHardwareAccelArgsWithDevice(hwAccel, hwDevice))

		// For QSV with HDR content, initialize OpenCL device for GPU tone mapping
		// NVENC uses libplacebo (Vulkan) for better reliability than OpenCL
		// Only QSV needs OpenCL initialization now
		if hwAccel == AccelQSV && config.ToneMappingEnabled && videoInfo != nil && videoInfo.IsHDR {
			// Check if we need OpenCL (not using libplacebo)
			backend := config.ToneMappingBackend
			if backend == "" {
				backend = "auto"
			}
			// QSV uses OpenCL tone mapping in auto mode
			// libplacebo is only used when explicitly requested for QSV
			if backend == "opencl" || backend == "auto" {
				builder.AddOpenCLDevice().AddOpenCLFilterDevice()
			}
		}
	}

	// Add memory safety options for ALL transcoding operations to prevent OOM crashes
	// Memory-intensive operations include: HDR tone mapping (especially libplacebo
	// with peak_detect), 4K scaling, and complex filter chains
	builder.AddMemorySafetyOptions(config.MaxMemoryMB)

	// Determine if we need -noaccurate_seek for A/V sync.
	// When copying video but transcoding audio, FFmpeg seeks video to the nearest keyframe
	// but starts audio from the exact seek position without this flag.
	needNoAccurateSeek := strategy == RemuxWithAudioDownmix || strategy == RemuxHEVC

	builder.AddSeekPosition()
	if needNoAccurateSeek {
		builder.AddNoAccurateSeek()
	}
	builder.AddInput().AddTimestampReset()

	// Add encoding based on strategy
	// IMPORTANT: Always add explicit stream mapping to prevent FFmpeg from auto-selecting
	// all streams (including subtitles), which breaks HLS output by creating separate .vtt files
	//
	// For remux strategies, we use the segment muxer instead of HLS muxer for better
	// timestamp handling when seeking. This requires patched FFmpeg (viewra-ffmpeg)
	// with start_pts tracking fix for correct A/V sync.
	useSegmentMuxer := false

	switch strategy {
	case Remux:
		// Copy both streams with H.264 bitstream filter for MPEG-TS compatibility
		// Use segment muxer for proper A/V sync when seeking
		// Note: Don't use -copyts here - let FFmpeg reset timestamps naturally
		// The segment muxer works better when output timestamps start from 0
		builder.AddStreamMapping().AddH264Copy().AddAudioCodec("copy")
		useSegmentMuxer = true

	case RemuxWithAudioDownmix:
		// Copy video with H.264 bitstream filter, transcode audio to AAC
		// Use -noaccurate_seek (added above) to align audio with video keyframe
		builder.AddStreamMapping().AddH264Copy().AddAudioDownmix()
		useSegmentMuxer = true

	case RemuxHEVC:
		// Copy HEVC video with bitstream filter, transcode audio to AAC
		// This is extremely fast (~50x realtime) since no video encoding happens
		// The hevc_mp4toannexb filter converts NAL units for MPEG-TS compatibility
		// Use -noaccurate_seek (added above) to align audio with video keyframe
		s.logger.Info("Using HEVC remux strategy",
			"session_id", s.ID,
			"media_id", s.MediaID)
		builder.AddStreamMapping().AddHEVCCopy().AddAudioDownmix()
		useSegmentMuxer = true

	case Transcode:
		// Get encoder and preset based on hardware acceleration and target codec
		// targetCodec was computed at function start and set in opts.VideoCodec
		videoEncoder, videoPreset := GetVideoCodecAndPresetForCodec(hwAccel, targetCodec)

		// For real-time progressive transcoding, override software preset to veryfast
		if hwAccel == AccelNone {
			videoPreset = "veryfast"
		}

		s.logger.Debug("Selected video encoder",
			"session_id", s.ID,
			"target_codec", targetCodec,
			"encoder", videoEncoder,
			"preset", videoPreset,
			"hw_accel", hwAccel)

		builder.AddStreamMapping().AddVideoCodec(videoEncoder, videoPreset)

		// Use hardware or software encoding
		if hwAccel != AccelNone {
			builder.AddHardwareVideoEncoding(hwAccel)
		} else {
			builder.AddVideoEncoding()
		}

		builder.AddAudioEncoding()
		// Note: No need for force_key_frames - the GOP settings (-g/-keyint_min)
		// already ensure frame 0 is a keyframe since it's the start of the first GOP.
	}

	// Add output settings based on muxer type
	var args []string
	if useSegmentMuxer {
		// Use segment muxer for remux strategies (requires patched FFmpeg)
		// This provides proper A/V sync when seeking into the middle of files
		builder.AddSegmentMuxerOutput().AddOverwriteOutput().AddSegmentMuxerOutputFile()
		args = builder.Build()
	} else {
		// Use HLS muxer for transcode (generates new keyframes, so timing is controlled)
		builder.AddHLSOutput().AddOverwriteOutput()
		args = builder.Build()
		args = append(args, s.ManifestPath)
	}

	return args
}

// createFFmpegCommand creates an FFmpeg command with memory limits via systemd-run (if available).
// On Linux with systemd, this wraps FFmpeg with cgroup memory limits as a hard safety net
// against memory spikes from subtitle burn-in, HDR tone mapping, or complex filter chains.
// Falls back to regular exec.CommandContext on other platforms or when systemd-run isn't available.
func createFFmpegCommand(ctx context.Context, args []string, config *TranscodeConfig, logger *slog.Logger) *exec.Cmd {
	paths := config.FFmpegPaths
	maxMemoryMB := config.MaxMemoryMB

	// On Linux with systemd, use systemd-run to apply memory limits
	if runtime.GOOS == "linux" && maxMemoryMB > 0 {
		if _, err := exec.LookPath("systemd-run"); err == nil {
			// Apply 2x multiplier for virtual memory headroom (shared libs, mmap, etc.)
			// The -max_alloc FFmpeg flag handles the tight limit; this is a safety net
			limitMB := maxMemoryMB * 2

			// Build systemd-run command with memory limit
			// --scope: Run as a transient scope unit (not a service)
			// --user: Run in user session (no root required)
			// -p MemoryMax: Hard memory limit
			// -p MemorySwapMax=0: Prevent swapping (fail fast rather than thrash)
			// -E: Pass environment variable to the child process
			systemdArgs := []string{
				"--scope",
				"--user",
				"-p", fmt.Sprintf("MemoryMax=%dM", limitMB),
				"-p", "MemorySwapMax=0",
			}

			// Pass LD_LIBRARY_PATH to FFmpeg via systemd-run's -E flag
			// (setting cmd.Env doesn't work because systemd-run spawns a new process)
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
				Setpgid: true, // Create new process group for clean shutdown
			}
			return cmd
		}
	}

	// Fallback: no system-level memory limit, rely on FFmpeg's -max_alloc
	// Use Paths.PrepareCommand to handle LD_LIBRARY_PATH consistently
	cmd := paths.PrepareCommand("ffmpeg", args...)
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
	}
	return cmd
}

// selectBestCodec selects the best codec for transcoding based on:
// 1. Client's supported codecs (what the browser can decode)
// 2. Profile's preferred codec and fallbacks
// 3. Hardware acceleration support
// Returns H.264 as the ultimate fallback for universal compatibility.
func selectBestCodec(profile *AdaptiveProfile, clientSupportedCodecs []string, hwAccel HardwareAccel) VideoCodec {
	// If no client codecs provided, assume H.264 only (most conservative)
	if len(clientSupportedCodecs) == 0 {
		clientSupportedCodecs = []string{"h264"}
	}

	// Build a set for fast lookup
	supported := make(map[string]bool)
	for _, codec := range clientSupportedCodecs {
		supported[codec] = true
	}

	// If profile is nil, return H.264
	if profile == nil {
		return CodecH264
	}

	// Check if preferred codec is supported by both client and hardware
	if supported[profile.PreferredCodec] && IsCodecSupported(hwAccel, VideoCodec(profile.PreferredCodec)) {
		return VideoCodec(profile.PreferredCodec)
	}

	// Check fallback codecs in order of preference
	for _, fallback := range profile.FallbackCodecs {
		if supported[fallback] && IsCodecSupported(hwAccel, VideoCodec(fallback)) {
			return VideoCodec(fallback)
		}
	}

	// Ultimate fallback: H.264 (universally supported)
	return CodecH264
}
