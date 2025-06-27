// Package ffmpeg provides FFmpeg command argument generation for video transcoding.
// This package builds optimized FFmpeg commands that ensure high-quality output
// with reliable playback across different devices and players. It handles the
// complexity of FFmpeg's vast parameter space by providing intelligent defaults
// and codec-specific optimizations.
//
// The argument builder focuses on:
// - Keyframe alignment for smooth seeking and segment boundaries
// - Proper codec settings for device compatibility
// - Audio normalization to prevent artifacts
// - Container-specific optimizations (DASH, HLS, MP4)
// - Performance vs quality trade-offs
//
// Critical design decisions:
// - Always disable scene detection (sc_threshold=0) for consistent GOP boundaries
// - Force closed GOPs for better seeking and segment independence
// - Use conservative thread counts to prevent resource contention
// - Prefer timeline addressing for precise seeking in adaptive streams
//
// Example usage:
//
//	builder := ffmpeg.NewFFmpegArgsBuilder(logger)
//	args := builder.BuildArgs(transcodeRequest, "/output/path.mpd")
//	cmd := exec.Command("ffmpeg", args...)
package ffmpeg

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/abr"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/types"
)

// FFmpegArgsBuilder handles building FFmpeg command arguments
type FFmpegArgsBuilder struct {
	logger          hclog.Logger
	resourceManager *ResourceManager
	mediaProber     *MediaProber
}

// NewFFmpegArgsBuilder creates a new FFmpeg args builder
func NewFFmpegArgsBuilder(logger hclog.Logger) *FFmpegArgsBuilder {
	return &FFmpegArgsBuilder{
		logger:          logger,
		resourceManager: NewResourceManager(logger),
		mediaProber:     NewMediaProber(logger),
	}
}

// BuildArgs builds optimized FFmpeg arguments for transcoding
func (b *FFmpegArgsBuilder) BuildArgs(req types.TranscodeRequest, outputPath string) []string {
	var args []string

	// Hardware acceleration - auto-detect best available method
	args = append(args, "-hwaccel", "auto")

	// Get resource configuration
	resources := b.resourceManager.GetOptimalResources(
		(req.Container == "dash" || req.Container == "hls") && req.EnableABR,
		b.getStreamCount(req),
		req.SpeedPriority,
	)

	// CRITICAL: Optimized flags for VOD transcoding
	// NOTE: Do NOT use -re or low_delay for VOD content
	args = append(args, "-fflags", "+genpts+fastseek")                 // Generate PTS, fast seeking
	args = append(args, "-probesize", resources.ProbeSize)             // Dynamic probe size based on memory
	args = append(args, "-analyzeduration", resources.AnalyzeDuration) // Dynamic analyze duration

	// Global options
	args = append(args, "-y")           // Always overwrite output files
	args = append(args, "-hide_banner") // Hide FFmpeg banner for cleaner logs

	// Seek to position if specified (input seeking for efficiency)
	if req.Seek > 0 {
		args = append(args, InputArgs.SeekStart...)
		args = append(args, fmt.Sprintf("%.3f", req.Seek.Seconds()))
	}

	// Input file
	args = append(args, InputArgs.Input...)
	args = append(args, req.InputPath)

	// For ABR (Adaptive Bitrate) streaming, let the specific function handle everything
	if (req.Container == "dash" || req.Container == "hls") && req.EnableABR {
		// Container-specific settings will handle all mapping and encoding
		containerArgs := b.getContainerSpecificArgs(req, outputPath)
		args = append(args, containerArgs...)
	} else {
		// Advanced video mapping and filtering
		args = append(args, StreamMappingArgs.Map...)
		args = append(args, "0:v:0") // Map first video stream
		args = append(args, StreamMappingArgs.Map...)
		args = append(args, "0:a:0") // Map first audio stream

		// Video codec with intelligent defaults
		videoCodec := b.getOptimalVideoCodec(req)
		args = append(args, VideoEncodingArgs.Codec...)
		args = append(args, videoCodec)

		// Preset optimization based on speed priority
		preset := b.getOptimalPreset(req.SpeedPriority, videoCodec)
		if preset != "" {
			args = append(args, "-preset", preset)
		}

		// Quality settings optimized for content
		qualityArgs := b.getOptimalQualitySettings(req, videoCodec)
		args = append(args, qualityArgs...)

		// Keyframe alignment for optimal seeking and segment boundaries
		keyframeArgs := b.getKeyframeAlignmentArgs(req)
		args = append(args, keyframeArgs...)

		// Video filtering for quality enhancement
		videoFilters := b.getVideoFilters(req)
		if len(videoFilters) > 0 {
			args = append(args, "-vf", videoFilters)
		}

		// Audio settings optimized for source content
		audioArgs := b.getOptimalAudioSettings(req)
		args = append(args, audioArgs...)

		// Apply resource optimizations
		args = append(args, b.applyResourceOptimizations(resources, false)...)

		// Container-specific settings with quality optimizations
		containerArgs := b.getContainerSpecificArgs(req, outputPath)
		args = append(args, containerArgs...)
	}

	// Output file
	args = append(args, outputPath)

	return args
}

// getStreamCount determines how many streams will be encoded
func (b *FFmpegArgsBuilder) getStreamCount(req types.TranscodeRequest) int {
	if !req.EnableABR {
		return 1
	}

	// For ABR, use the default ladder size (3 quality levels)
	// This could be made configurable in the future
	return 3
}

// applyResourceOptimizations applies all resource-related FFmpeg arguments
func (b *FFmpegArgsBuilder) applyResourceOptimizations(resources ResourceConfig, isABR bool) []string {
	var args []string

	// Thread settings
	args = append(args, "-threads", resources.ThreadCount)

	// Memory and buffer settings
	args = append(args, "-max_muxing_queue_size", resources.MuxingQueueSize)
	args = append(args, "-max_delay", resources.MaxDelay)

	// Filter thread settings (if using filters)
	args = append(args, "-filter_threads", resources.FilterThreads)

	// Decoder threads
	args = append(args, "-threads:0", resources.DecodeThreads) // Input thread count

	// Scale filter threads (applied via filter_complex)
	// This is handled within the scale filter itself

	return args
}

// getOptimalVideoCodec selects the best video codec based on request and available hardware
func (b *FFmpegArgsBuilder) getOptimalVideoCodec(req types.TranscodeRequest) string {
	if req.VideoCodec != "" {
		return req.VideoCodec
	}

	// Default to H.264 for compatibility, hardware acceleration will auto-detect
	return "libx264"
}

// getOptimalPreset selects the best encoding preset for quality/speed balance
func (b *FFmpegArgsBuilder) getOptimalPreset(speedPriority types.SpeedPriority, codec string) string {
	// Use the resource manager to get system-aware preset
	return b.resourceManager.GetEncodingPreset(speedPriority, runtime.NumCPU())
}

// getOptimalQualitySettings returns quality parameters optimized for content
func (b *FFmpegArgsBuilder) getOptimalQualitySettings(req types.TranscodeRequest, codec string) []string {
	var args []string

	// CRF calculation optimized for streaming
	// Map 0-100 quality to CRF 35-18 for better streaming performance
	crf := 35 - (req.Quality * 17 / 100)
	if crf < 18 {
		crf = 18 // Maximum quality
	}
	if crf > 35 {
		crf = 35 // Minimum quality for streaming
	}

	args = append(args, "-crf", strconv.Itoa(crf))

	// Additional quality settings for H.264
	if codec == "libx264" {
		// Use baseline profile for low bitrate stream, high for others
		if req.Quality < 30 {
			args = append(args, "-profile:v", "baseline")
			args = append(args, "-level", "3.0")
		} else {
			args = append(args, "-profile:v", "high")
			args = append(args, "-level", "4.1")
		}
		// Optimized x264 params based on system resources
		resources := b.resourceManager.GetOptimalResources(false, 1, req.SpeedPriority)
		x264Params := fmt.Sprintf("ref=1:bframes=0:me=hex:subme=2:rc-lookahead=%s:keyint=48:min-keyint=24:no-scenecut=1:aq-mode=0",
			resources.RCLookahead)
		args = append(args, "-x264-params", x264Params)
	} else if codec == "libx265" {
		args = append(args, "-preset", "medium")
		args = append(args, "-x265-params", "keyint=48:min-keyint=24:no-open-gop=1")
	} else if codec == "libvpx-vp9" {
		args = append(args, "-b:v", "0") // Use constant quality mode
		args = append(args, "-deadline", "good")
		args = append(args, "-cpu-used", "2")
		args = append(args, "-row-mt", "1")
		args = append(args, "-tile-columns", "2")
		args = append(args, "-tile-rows", "1")
		args = append(args, "-g", "48") // Keyframe interval
	}

	return args
}

// getVideoFilters returns video filters for quality enhancement
func (b *FFmpegArgsBuilder) getVideoFilters(req types.TranscodeRequest) string {
	var filters []string

	// Resolution scaling if specified
	if req.Resolution != nil && req.Resolution.Width > 0 && req.Resolution.Height > 0 {
		// Use lanczos for high quality downscaling
		scaleFilter := fmt.Sprintf("scale=%d:%d:flags=lanczos", req.Resolution.Width, req.Resolution.Height)
		filters = append(filters, scaleFilter)
	}

	// Deinterlacing if needed (detect interlaced content)
	filters = append(filters, "yadif=mode=send_field:deint=interlaced")

	// Pixel format conversion for compatibility
	filters = append(filters, "format=yuv420p")

	if len(filters) > 0 {
		return strings.Join(filters, ",")
	}

	return ""
}

// getOptimalAudioSettings returns optimized audio encoding settings
func (b *FFmpegArgsBuilder) getOptimalAudioSettings(req types.TranscodeRequest) []string {
	var args []string

	audioCodec := req.AudioCodec
	if audioCodec == "" {
		audioCodec = "aac"
	}
	args = append(args, "-c:a", audioCodec)

	// Conservative audio settings to prevent pops and artifacts
	if audioCodec == "aac" {
		// Optimized audio bitrate for streaming
		args = append(args, "-b:a", "96k") // Lower bitrate for better streaming
		args = append(args, "-profile:a", "aac_low")
		args = append(args, "-ar", "48000") // Standard sample rate

		// Force stereo output for maximum compatibility
		// This prevents issues with multichannel audio
		args = append(args, "-ac", "2") // Stereo output

		// No audio filters - let FFmpeg handle conversion naturally
		// Audio filters can introduce artifacts and pops
	}

	return args
}

// getKeyframeAlignmentArgs returns FFmpeg arguments for keyframe alignment
func (b *FFmpegArgsBuilder) getKeyframeAlignmentArgs(req types.TranscodeRequest) []string {
	var args []string

	// Determine segment duration for adaptive streaming
	segmentDuration := 2.0 // Default 2 seconds for fast startup
	if req.Container == "dash" || req.Container == "hls" {
		// For ABR, use even shorter segments for the first few
		if req.EnableABR {
			segmentDuration = 2.0 // 2 second segments for ABR (good balance)
		} else {
			segmentDuration = b.getSegmentDurationFloat(req)
		}
	}

	// CRITICAL: GOP alignment - GOP size must equal segment duration × frame rate
	// Assume 30fps for now (in real implementation, detect from source)
	frameRate := 30.0
	gopSize := int(segmentDuration * frameRate)

	// Force keyframes at exact segment boundaries for perfect alignment
	keyframeExpr := fmt.Sprintf("expr:gte(t,n_forced*%.1f)", segmentDuration)
	args = append(args, "-force_key_frames", keyframeExpr)

	// Set GOP size to match segment duration exactly
	args = append(args, VideoEncodingArgs.KeyInt...)
	args = append(args, strconv.Itoa(gopSize))

	// Set minimum keyframe interval to match GOP size
	args = append(args, VideoEncodingArgs.KeyIntMin...)
	args = append(args, strconv.Itoa(gopSize))

	// Ensure closed GOPs for better seeking and segment independence
	args = append(args, "-flags", "+cgop")

	// CRITICAL: Disable scene change detection for consistent GOP boundaries
	args = append(args, VideoEncodingArgs.ScThreshold...)
	args = append(args, "0")

	// GOP structure is controlled by keyint settings above

	return args
}

// getSegmentDurationFloat returns segment duration as float for calculations
func (b *FFmpegArgsBuilder) getSegmentDurationFloat(req types.TranscodeRequest) float64 {
	// Use shorter segments for faster startup
	return 2.0 // 2 seconds for faster startup
}

// getContainerSpecificArgs returns optimized settings for each container format
func (b *FFmpegArgsBuilder) getContainerSpecificArgs(req types.TranscodeRequest, outputPath string) []string {
	var args []string

	switch req.Container {
	case "dash":
		// For the new architecture, output intermediate MP4 files
		// These will be packaged into DASH by Shaka Packager
		if req.EnableABR {
			return b.getMP4ABRArgs(req, outputPath)
		}

		// Single bitrate MP4 output for DASH packaging
		return b.getMP4IntermediateArgs(req, outputPath)
	case "hls":
		// Check if ABR ladder is requested
		if req.EnableABR {
			return b.getHLSABRArgs(req, outputPath)
		}

		// Single bitrate HLS
		outputDir := filepath.Dir(outputPath)
		segDuration := b.getAdaptiveSegmentDuration(req)

		// Use fMP4 segments for better seeking with byte-range support
		args = append(args,
			"-f", "hls",
			"-hls_time", segDuration, // Adaptive segment duration
			"-hls_playlist_type", "vod",
			"-hls_segment_type", "fmp4", // Use fMP4 for byte-range support
			"-hls_fmp4_init_filename", "init.mp4", // Single init segment
			"-hls_segment_filename", filepath.Join(outputDir, "segment_%03d.m4s"),
			"-hls_flags", "independent_segments+program_date_time+single_file",
			// Enable byte-range support for efficient seeking
			"-hls_segment_options", "movflags=+cmaf+dash+delay_moov+global_sidx+write_colr+write_gama",
			// Low-latency HLS optimizations
			"-hls_list_size", "0", // Keep all segments in playlist
			"-hls_start_number_source", "datetime", // Better segment numbering
			// Partial segment support for LL-HLS
			"-hls_partial_duration", "0.5", // 500ms partial segments
		)

		// Add LL-HLS specific settings if seek position indicates need for responsiveness
		if req.Seek > 0 {
			args = append(args,
				"-hls_flags", "independent_segments+program_date_time+single_file+temp_file",
				"-master_pl_name", "master.m3u8",
				"-master_pl_publish_rate", "2", // Update master playlist every 2 segments
			)
		}

	default: // MP4 with streaming optimizations
		args = append(args,
			"-f", "mp4",
			"-movflags", "+faststart+frag_keyframe+empty_moov+dash+cmaf+global_sidx+write_colr",
			"-frag_duration", "1", // 1s fragments for better seeking
			"-min_frag_duration", "0.5", // Min 500ms fragments
			"-brand", "mp42", // Better compatibility
			// Seek optimization
			"-write_tmcd", "0", // Disable timecode track
			"-strict", "experimental", // Enable experimental features
		)
	}

	return args
}

// getAdaptiveSegmentDuration returns segment duration based on content and seek position
func (b *FFmpegArgsBuilder) getAdaptiveSegmentDuration(req types.TranscodeRequest) string {
	// For seek-ahead requests, use shorter segments for better responsiveness
	if req.Seek > 0 {
		return "2" // 2 second segments for seek-ahead
	}

	// For regular playback, optimize based on content characteristics
	// Start with shorter segments, can be adapted during transcoding

	// Default to 3 seconds for good balance of startup time vs efficiency
	// FFmpeg will adapt this based on keyframe intervals and content
	return "3"
}

// getHLSABRArgs returns HLS arguments for adaptive bitrate streaming
func (b *FFmpegArgsBuilder) getHLSABRArgs(req types.TranscodeRequest, outputPath string) []string {
	var args []string

	// Get source dimensions
	sourceWidth := 1920
	sourceHeight := 1080
	if req.Resolution != nil {
		sourceWidth = req.Resolution.Width
		sourceHeight = req.Resolution.Height
	}

	// Generate bitrate ladder
	abrGen := abr.NewGenerator(b.logger)
	ladder := abrGen.GenerateLadder(sourceWidth, sourceHeight, req.Quality)
	outputDir := filepath.Dir(outputPath)

	// Add optimized encoding settings for ABR before mapping
	for i := range ladder {
		// Force keyframe interval for all variants
		args = append(args,
			fmt.Sprintf("-force_key_frames:v:%d", i), "expr:gte(t,n_forced*2)",
		)
	}

	// Create variant streams
	var variantStreams []string

	for i, rung := range ladder {
		// Map video and audio
		args = append(args,
			"-map", "0:v:0",
			"-map", "0:a:0",
		)

		// Video encoding settings
		args = append(args,
			fmt.Sprintf("-c:v:%d", i), "libx264",
			fmt.Sprintf("-b:v:%d", i), fmt.Sprintf("%dk", rung.VideoBitrate),
			fmt.Sprintf("-maxrate:%d", i), fmt.Sprintf("%dk", int(float64(rung.VideoBitrate)*1.5)),
			fmt.Sprintf("-bufsize:%d", i), fmt.Sprintf("%dk", rung.VideoBitrate*2),
			fmt.Sprintf("-vf:%d", i), fmt.Sprintf("scale=%d:%d:flags=lanczos", rung.Width, rung.Height),
			fmt.Sprintf("-profile:v:%d", i), rung.Profile,
			fmt.Sprintf("-level:%d", i), rung.Level,
		)

		// Audio encoding settings
		args = append(args,
			fmt.Sprintf("-c:a:%d", i), "aac",
			fmt.Sprintf("-b:a:%d", i), fmt.Sprintf("%dk", rung.AudioBitrate),
			fmt.Sprintf("-ar:%d", i), "48000",
			fmt.Sprintf("-ac:%d", i), "2", // Force stereo for compatibility
			fmt.Sprintf("-profile:a:%d", i), "aac_low",
		)

		// Optimized GOP size and B-frames for this variant
		args = append(args,
			fmt.Sprintf("-g:%d", i), "48", // GOP size = 2s @ 24fps
			fmt.Sprintf("-bf:%d", i), "0", // No B-frames for real-time
			fmt.Sprintf("-sc_threshold:%d", i), "0", // Disable scene detection
		)

		// Variant playlist info
		variantStreams = append(variantStreams,
			fmt.Sprintf("v:%d,a:%d,name:%s", i, i, rung.Label),
		)
	}

	// Apply resource optimizations for HLS ABR
	resources := b.resourceManager.GetOptimalResources(true, len(ladder), req.SpeedPriority)
	args = append(args, b.applyResourceOptimizations(resources, true)...)

	// HLS muxer settings with optimizations
	segDuration := "2" // Fixed 2 second segments for ABR
	args = append(args,
		"-f", "hls",
		"-hls_time", segDuration,
		"-hls_playlist_type", "vod",
		"-hls_segment_type", "mpegts",
		"-hls_flags", "independent_segments+split_by_time",
		"-hls_list_size", "0", // Keep all segments
		"-master_pl_name", "playlist.m3u8",
		"-hls_segment_filename", filepath.Join(outputDir, "stream_%v/segment_%03d.ts"),
		"-var_stream_map", strings.Join(variantStreams, " "),
	)

	return args
}

// getMP4IntermediateArgs returns MP4 output arguments for single bitrate
// These MP4 files will be packaged into DASH/HLS by Shaka Packager
func (b *FFmpegArgsBuilder) getMP4IntermediateArgs(req types.TranscodeRequest, outputPath string) []string {
	var args []string

	// Get media duration for proper encoding
	duration, err := b.mediaProber.GetDuration(req.InputPath)
	if err != nil && b.logger != nil {
		b.logger.Warn("failed to probe media duration", "error", err)
	}

	// Set duration if available
	if duration > 0 {
		args = append(args,
			"-t", fmt.Sprintf("%.3f", duration.Seconds()),
		)
	}

	// MP4 container with streaming optimizations
	args = append(args,
		"-f", "mp4",
		// CMAF-compatible fragmented MP4 for easier packaging
		"-movflags", "+faststart+frag_keyframe+empty_moov+default_base_moof+cmaf",
		// Fragment duration matching our segment size
		"-frag_duration", "2000000", // 2 seconds in microseconds
		// Ensure compatibility with Shaka Packager
		"-brand", "mp42",
		"-min_frag_duration", "1000000", // 1 second minimum
		// Disable unnecessary tracks
		"-write_tmcd", "0",
		// Ensure proper timestamp handling
		"-avoid_negative_ts", "make_zero",
		"-fps_mode", "cfr",
	)

	if b.logger != nil {
		b.logger.Info("Generating intermediate MP4 for DASH packaging",
			"output", outputPath,
			"duration", duration,
		)
	}

	return args
}

// getMP4ABRArgs returns MP4 output arguments for ABR encoding
// NOTE: For now, this returns args for a single quality level
// The pipeline manager will need to run FFmpeg multiple times for ABR
func (b *FFmpegArgsBuilder) getMP4ABRArgs(req types.TranscodeRequest, outputPath string) []string {
	// For ABR, we'll need to run FFmpeg multiple times
	// This is handled by the pipeline manager
	// For now, just return single quality encoding
	if b.logger != nil {
		b.logger.Info("ABR encoding requested - pipeline will handle multiple qualities",
			"output", outputPath,
		)
	}

	// Return single quality MP4 encoding for now
	// The pipeline manager will call this multiple times with different settings
	return b.getMP4IntermediateArgs(req, outputPath)
}
