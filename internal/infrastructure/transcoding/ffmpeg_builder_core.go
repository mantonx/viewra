// Package transcoding provides FFmpeg-based video transcoding capabilities.
//
// The FFmpeg argument builder is split across multiple files for better organization:
//   - ffmpeg_builder_core.go: Builder struct, constructor, input/output basics (this file)
//   - ffmpeg_builder_encoding.go: Software video/audio encoding
//   - ffmpeg_builder_hardware.go: Hardware-accelerated encoding (NVENC, QSV, VAAPI, VideoToolbox)
//   - ffmpeg_builder_filters.go: Video filters (tone mapping, scaling)
//   - ffmpeg_builder_output.go: HLS and segment muxer output configuration
package transcoding

import (
	"fmt"
	"strconv"
)

// FFmpegArgsBuilder provides a fluent interface for building FFmpeg command arguments.
// This eliminates code duplication across the 4 different transcoding strategies.
type FFmpegArgsBuilder struct {
	args []string
	opts TranscodeOptions
}

// NewFFmpegArgsBuilder creates a new FFmpeg arguments builder.
func NewFFmpegArgsBuilder(opts TranscodeOptions) *FFmpegArgsBuilder {
	return &FFmpegArgsBuilder{
		args: []string{},
		opts: opts,
	}
}

// Build returns the final arguments slice.
func (b *FFmpegArgsBuilder) Build() []string {
	return b.args
}

// formatBitrate converts an integer bitrate (bits per second) to FFmpeg format string.
// e.g., 5000000 -> "5000k", 15000000 -> "15000k"
func formatBitrate(bps int) string {
	return fmt.Sprintf("%dk", bps/1000)
}

// getH264Level returns the appropriate H.264 level for a given resolution.
// H.264 levels define maximum resolution, bitrate, and macroblocks per second.
// Level 4.1: up to 1920x1080@30fps or 1280x720@60fps
// Level 5.0: up to 2560x1920@30fps
// Level 5.1: up to 4096x2160@30fps (4K)
// Level 5.2: up to 4096x2160@60fps
func getH264Level(width, height int) string {
	pixels := width * height
	if pixels > 2073600 { // > 1920x1080
		return "5.1" // Required for 4K (3840x2160)
	}
	if pixels > 921600 { // > 1280x720
		return "4.1" // 1080p
	}
	return "4.0" // 720p and below
}

// getVideoCodec returns the video codec to use, defaulting to H.264 if not specified.
func (b *FFmpegArgsBuilder) getVideoCodec() VideoCodec {
	if b.opts.VideoCodec == "" {
		return CodecH264
	}
	return b.opts.VideoCodec
}

// AddLogLevel sets FFmpeg's log level to reduce verbosity.
// Use "error" to only show errors, "warning" for warnings and errors,
// "info" for normal output (default), or "quiet" to suppress all output.
func (b *FFmpegArgsBuilder) AddLogLevel(level string) *FFmpegArgsBuilder {
	b.args = append(b.args, "-loglevel", level)
	return b
}

// AddHardwareAccel adds hardware acceleration arguments.
func (b *FFmpegArgsBuilder) AddHardwareAccel(hwAccelArgs []string) *FFmpegArgsBuilder {
	b.args = append(b.args, hwAccelArgs...)
	return b
}

// AddOpenCLDevice initializes an OpenCL hardware device for GPU tone mapping.
// This must be called before AddInput() to initialize the OpenCL context.
func (b *FFmpegArgsBuilder) AddOpenCLDevice() *FFmpegArgsBuilder {
	// Initialize OpenCL device on platform 0, device 0 (usually the primary GPU)
	// The device name 'ocl' will be referenced in filters with -filter_hw_device
	b.args = append(b.args, "-init_hw_device", "opencl=ocl:0.0")
	return b
}

// AddOpenCLFilterDevice sets the hardware device context for filters.
// This must be called after AddOpenCLDevice() and before filter usage.
func (b *FFmpegArgsBuilder) AddOpenCLFilterDevice() *FFmpegArgsBuilder {
	b.args = append(b.args, "-filter_hw_device", "ocl")
	return b
}

// AddSeekPosition adds seek position argument (before input for fast seeking).
// Also resets output timestamps to 0 so players can seek within the stream correctly.
func (b *FFmpegArgsBuilder) AddSeekPosition() *FFmpegArgsBuilder {
	if b.opts.UseStartPosition && b.opts.StartPosition > 0 {
		b.args = append(b.args, "-ss", strconv.Itoa(b.opts.StartPosition))
	}
	return b
}

// AddNoAccurateSeek disables accurate seek to align audio with video keyframes.
// When copying video but transcoding audio, FFmpeg seeks video to the nearest keyframe
// but starts audio from the exact seek position, causing A/V desync. This flag makes
// audio also start from the keyframe position, ensuring A/V sync.
// Must be called AFTER AddSeekPosition() and BEFORE AddInput().
func (b *FFmpegArgsBuilder) AddNoAccurateSeek() *FFmpegArgsBuilder {
	if b.opts.UseStartPosition && b.opts.StartPosition > 0 {
		b.args = append(b.args, "-noaccurate_seek")
	}
	return b
}

// AddTimestampReset resets output timestamps to start from 0 (used after seeking).
// This ensures the HLS stream reports time from 0, not from the seek position.
// Must be called AFTER AddInput() since it's an output option.
func (b *FFmpegArgsBuilder) AddTimestampReset() *FFmpegArgsBuilder {
	// No-op: Frontend handles timestamp offset tracking via streamOffsetRef.
	// Attempting to reset timestamps in FFmpeg causes issues with HLS segment timing.
	return b
}

// AddFastInputOptions adds options to speed up input file analysis.
// This reduces startup latency by limiting how much FFmpeg analyzes before starting.
func (b *FFmpegArgsBuilder) AddFastInputOptions() *FFmpegArgsBuilder {
	b.args = append(b.args,
		// Limit input analysis to 5 seconds / 5MB for faster startup
		"-analyzeduration", "5000000",
		"-probesize", "5000000",
		// Generate timestamps and discard corrupt frames for smoother playback
		// Note: Removed +fastseek as it causes quality issues at seek points,
		// especially with HDR content where tone mapping needs reference frames
		"-fflags", "+genpts+discardcorrupt",
	)
	return b
}

// AddMemorySafetyOptions adds options to prevent excessive memory usage during transcoding.
// This is especially important for subtitle burn-in with PGS/bitmap subtitles, which can
// cause memory spikes that crash the system. These options limit FFmpeg's internal buffers.
// See: Jellyfin #11052 for similar memory issues with subtitle burn-in.
//
// maxAllocMB: Maximum single allocation size in MB (0 = use default of 25% system RAM, capped at 4GB)
func (b *FFmpegArgsBuilder) AddMemorySafetyOptions(maxAllocMB int) *FFmpegArgsBuilder {
	// Convert MB to bytes for FFmpeg's -max_alloc option
	var maxAllocBytes int64
	if maxAllocMB > 0 {
		maxAllocBytes = int64(maxAllocMB) * 1024 * 1024
	} else {
		// Default: 2GB - reasonable for most systems
		// This will be overridden by callers who have access to system profile
		maxAllocBytes = 2 * 1024 * 1024 * 1024
	}

	b.args = append(b.args,
		// Limit maximum allocation size - prevents single massive allocations
		// that could exhaust system memory. Default is unlimited.
		"-max_alloc", strconv.FormatInt(maxAllocBytes, 10),
		// Limit thread queue size to 512 packets per stream - prevents unbounded
		// buffering when filter processing is slower than demuxing.
		// Critical for subtitle overlay which can cause filter bottlenecks.
		"-thread_queue_size", "512",
	)
	return b
}

// AddInput adds the input file argument.
func (b *FFmpegArgsBuilder) AddInput() *FFmpegArgsBuilder {
	b.args = append(b.args, "-i", b.opts.InputPath)
	return b
}

// AddStreamMapping adds stream mapping for video and audio.
func (b *FFmpegArgsBuilder) AddStreamMapping() *FFmpegArgsBuilder {
	// Map first video stream
	b.args = append(b.args, "-map", "0:v:0")

	// Map audio stream (specific track or default)
	if b.opts.UseSpecificAudioTrack {
		b.args = append(b.args, "-map", fmt.Sprintf("0:%d", b.opts.AudioTrackIndex))
	} else {
		b.args = append(b.args, "-map", "0:a:0")
	}

	return b
}

// AddVideoCodec adds video codec settings.
func (b *FFmpegArgsBuilder) AddVideoCodec(codec, preset string) *FFmpegArgsBuilder {
	b.args = append(b.args, "-c:v", codec)

	if preset != "" {
		b.args = append(b.args, "-preset", preset)
	}

	return b
}

// AddH264Copy adds H.264 video stream copy with the h264_mp4toannexb bitstream filter.
// This converts H.264 NAL units from length-prefixed (MP4/MKV) to Annex B format (MPEG-TS).
// Required for HLS with MPEG-TS segments when copying H.264 streams.
// This is extremely fast (~50x realtime) compared to full transcode (~1x realtime).
func (b *FFmpegArgsBuilder) AddH264Copy() *FFmpegArgsBuilder {
	b.args = append(b.args,
		"-c:v", "copy",
		"-bsf:v", "h264_mp4toannexb",
	)
	return b
}

// AddHEVCCopy adds HEVC video stream copy with the hevc_mp4toannexb bitstream filter.
// This converts HEVC NAL units from length-prefixed (MP4/MKV) to Annex B format (MPEG-TS).
// Required for HLS with MPEG-TS segments when copying HEVC streams.
// This is extremely fast (~50x realtime) compared to full transcode (~1x realtime).
func (b *FFmpegArgsBuilder) AddHEVCCopy() *FFmpegArgsBuilder {
	b.args = append(b.args,
		"-c:v", "copy",
		"-bsf:v", "hevc_mp4toannexb",
	)
	return b
}

// AddCopyTimestamps preserves original timestamps when copying streams.
// This reduces processing overhead for remux operations.
func (b *FFmpegArgsBuilder) AddCopyTimestamps() *FFmpegArgsBuilder {
	b.args = append(b.args, "-copyts")
	return b
}

// AddCopyTimestampsWithReset preserves original timestamps but resets output to start at 0.
// This is the recommended approach for remux with seeking:
// - -copyts: Preserve original PTS values from input
// - -start_at_zero: Shift output timestamps so first frame starts at 0
// This combination ensures the patched segment muxer correctly tracks start_pts
// for proper A/V sync when seeking into the middle of a file.
func (b *FFmpegArgsBuilder) AddCopyTimestampsWithReset() *FFmpegArgsBuilder {
	b.args = append(b.args, "-copyts", "-start_at_zero")
	return b
}

// AddProgressReporting adds progress reporting arguments.
func (b *FFmpegArgsBuilder) AddProgressReporting() *FFmpegArgsBuilder {
	b.args = append(b.args,
		"-progress", "pipe:2",
		"-stats",
	)
	return b
}

// AddOverwriteOutput adds the -y flag to overwrite output files.
func (b *FFmpegArgsBuilder) AddOverwriteOutput() *FFmpegArgsBuilder {
	b.args = append(b.args, "-y")
	return b
}
