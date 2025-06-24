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
//   builder := ffmpeg.NewFFmpegArgsBuilder(logger)
//   args := builder.BuildArgs(transcodeRequest, "/output/path.mpd")
//   cmd := exec.Command("ffmpeg", args...)
package ffmpeg

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mantonx/viewra/sdk/transcoding/abr"
	"github.com/mantonx/viewra/sdk/transcoding/types"
)

// FFmpegArgsBuilder handles building FFmpeg command arguments
type FFmpegArgsBuilder struct {
	logger types.Logger
}

// NewFFmpegArgsBuilder creates a new FFmpeg args builder
func NewFFmpegArgsBuilder(logger types.Logger) *FFmpegArgsBuilder {
	return &FFmpegArgsBuilder{
		logger: logger,
	}
}

// BuildArgs builds optimized FFmpeg arguments for transcoding
func (b *FFmpegArgsBuilder) BuildArgs(req types.TranscodeRequest, outputPath string) []string {
	var args []string

	// Always overwrite output files
	args = append(args, "-y")

	// Seek to position if specified (input seeking for efficiency)
	if req.Seek > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", req.Seek.Seconds()))
	}

	// Input file
	args = append(args, "-i", req.InputPath)

	// For ABR (Adaptive Bitrate) streaming, let the specific function handle everything
	if (req.Container == "dash" || req.Container == "hls") && req.EnableABR {
		// Container-specific settings will handle all mapping and encoding
		containerArgs := b.getContainerSpecificArgs(req, outputPath)
		args = append(args, containerArgs...)
	} else {
		// Advanced video mapping and filtering
		args = append(args, "-map", "0:v:0") // Map first video stream
		args = append(args, "-map", "0:a:0") // Map first audio stream

		// Video codec with intelligent defaults
		videoCodec := b.getOptimalVideoCodec(req)
		args = append(args, "-c:v", videoCodec)

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

		// Conservative threading to prevent resource contention
		args = append(args, "-threads", "4") // Limit to 4 threads for stability

		// Container-specific settings with quality optimizations
		containerArgs := b.getContainerSpecificArgs(req, outputPath)
		args = append(args, containerArgs...)
	}

	// Output file
	args = append(args, outputPath)

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
	switch speedPriority {
	case types.SpeedPriorityFastest:
		return "medium"     // More conservative than "faster"
	case types.SpeedPriorityQuality:
		return "slow"       // High quality
	default:
		return "slow"       // Default to slow for quality and stability
	}
}

// getOptimalQualitySettings returns quality parameters optimized for content
func (b *FFmpegArgsBuilder) getOptimalQualitySettings(req types.TranscodeRequest, codec string) []string {
	var args []string
	
	// CRF calculation with improved mapping for better quality
	// Map 0-100 quality to CRF 28-16 for better visual quality
	crf := 28 - (req.Quality * 12 / 100)
	if crf < 16 {
		crf = 16 // Maximum quality
	}
	if crf > 28 {
		crf = 28 // Minimum quality for streaming
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
		// Conservative x264 params for stability
		args = append(args, "-x264-params", "ref=2:bframes=2:me=hex:subme=6:rc-lookahead=40")
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
		// Moderate bitrate for compatibility
		args = append(args, "-b:a", "128k")      // Standard quality
		args = append(args, "-profile:a", "aac_low")
		args = append(args, "-ar", "48000")      // Standard sample rate
		
		// Force stereo output for maximum compatibility
		// This prevents issues with multichannel audio
		args = append(args, "-ac", "2")          // Stereo output
		
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
	args = append(args, "-g", strconv.Itoa(gopSize))
	
	// Set minimum keyframe interval to match GOP size
	args = append(args, "-keyint_min", strconv.Itoa(gopSize))
	
	// Ensure closed GOPs for better seeking and segment independence
	args = append(args, "-flags", "+cgop")
	
	// CRITICAL: Disable scene change detection for consistent GOP boundaries
	args = append(args, "-sc_threshold", "0")
	
	// Force strict GOP structure
	args = append(args, "-strict_gop", "1")
	
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
		// Check if ABR ladder is requested
		if req.EnableABR {
			return b.getDashABRArgs(req, outputPath)
		}
		
		// Single bitrate DASH for VOD content with critical optimizations
		args = append(args,
			"-f", "dash",
			// CRITICAL: Use static MPD for VOD content
			"-streaming", "0",                       // Disable streaming mode for VOD
			"-ldash", "0",                          // Disable low-latency DASH
			"-seg_duration", "2",                    // 2s segments for better startup
			"-frag_duration", "0.5",                // 500ms fragments for precise seeking
			// CRITICAL: Timeline addressing for precise seeking
			"-use_timeline", "1",                   // Enable timeline addressing
			"-segment_timeline", "1",               // Enable segment timeline
			"-use_template", "1",                   // Use template-based addressing
			// Static MPD profile
			"-mpd_profile", "onDemand",             // Force static (on-demand) profile
			// Segment naming
			"-init_seg_name", "init-$RepresentationID$.m4s",
			"-media_seg_name", "chunk-$RepresentationID$-$Number$.m4s",
			"-adaptation_sets", "id=0,streams=v id=1,streams=a",
			"-dash_segment_type", "mp4",
			"-single_file", "0",
			// CRITICAL: Proper init segments with all metadata
			"-frag_type", "duration",                // Use duration-based fragmentation
			"-movflags", "+dash+cmaf+faststart",    // CMAF with fast start
			// VOD optimizations
			"-write_prft", "0",                      // No producer reference time needed
			"-global_sidx", "1",                    // Global SIDX for better seeking
			"-http_persistent", "0",                // Don't keep connections open
			"-hls_playlist", "0",                   // Disable HLS compatibility
			// Keep all segments for VOD
			"-remove_at_exit", "0",                // Don't remove segments on exit
			"-window_size", "0",                     // Keep all segments (no sliding window)
			"-extra_window_size", "0",              // No extra window
			// Timestamp handling
			"-avoid_negative_ts", "make_zero",      // Fix timestamp issues
		)
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
			"-hls_time", segDuration,               // Adaptive segment duration
			"-hls_playlist_type", "vod",
			"-hls_segment_type", "fmp4",            // Use fMP4 for byte-range support
			"-hls_fmp4_init_filename", "init.mp4",  // Single init segment
			"-hls_segment_filename", filepath.Join(outputDir, "segment_%03d.m4s"),
			"-hls_flags", "independent_segments+program_date_time+single_file",
			// Enable byte-range support for efficient seeking
			"-hls_segment_options", "movflags=+cmaf+dash+delay_moov+global_sidx+write_colr+write_gama",
			// Low-latency HLS optimizations
			"-hls_list_size", "0",                  // Keep all segments in playlist
			"-hls_start_number_source", "datetime", // Better segment numbering
			// Partial segment support for LL-HLS
			"-hls_partial_duration", "0.5",        // 500ms partial segments
		)
		
		// Add LL-HLS specific settings if seek position indicates need for responsiveness
		if req.Seek > 0 {
			args = append(args,
				"-hls_flags", "independent_segments+program_date_time+single_file+temp_file",
				"-master_pl_name", "master.m3u8",
				"-master_pl_publish_rate", "2",     // Update master playlist every 2 segments
			)
		}
		
	default: // MP4 with streaming optimizations
		args = append(args,
			"-f", "mp4",
			"-movflags", "+faststart+frag_keyframe+empty_moov+dash+cmaf+global_sidx+write_colr",
			"-frag_duration", "1",                  // 1s fragments for better seeking
			"-min_frag_duration", "0.5",            // Min 500ms fragments
			"-brand", "mp42",                       // Better compatibility
			// Seek optimization
			"-write_tmcd", "0",                     // Disable timecode track
			"-strict", "experimental",              // Enable experimental features
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

// getDashABRArgs returns DASH arguments for adaptive bitrate streaming
func (b *FFmpegArgsBuilder) getDashABRArgs(req types.TranscodeRequest, outputPath string) []string {
	var args []string
	
	// Get source dimensions (simplified - in real implementation would probe the file)
	sourceWidth := 1920
	sourceHeight := 1080
	if req.Resolution != nil {
		sourceWidth = req.Resolution.Width
		sourceHeight = req.Resolution.Height
	}
	
	// Generate bitrate ladder
	abrGen := abr.NewGenerator(b.logger)
	ladder := abrGen.GenerateLadder(sourceWidth, sourceHeight, req.Quality)
	
	// Map streams for each quality level
	var maps []string
	var videoStreamIndices []string
	var audioStreamIndices []string
	
	// First add all the maps
	for range ladder {
		// Create a named output for each quality
		maps = append(maps,
			"-map", "0:v:0",
			"-map", "0:a:0",
		)
	}
	
	// Add all maps to args first
	args = append(args, maps...)
	
	// Then add encoding settings for each stream
	for i, rung := range ladder {
		// Video encoding settings for this rung
		streamIndex := i * 2
		args = append(args,
			fmt.Sprintf("-c:v:%d", streamIndex), "libx264",
			fmt.Sprintf("-b:v:%d", streamIndex), fmt.Sprintf("%dk", rung.VideoBitrate),
			fmt.Sprintf("-maxrate:%d", streamIndex), fmt.Sprintf("%dk", int(float64(rung.VideoBitrate)*1.5)),
			fmt.Sprintf("-bufsize:%d", streamIndex), fmt.Sprintf("%dk", rung.VideoBitrate*2),
			fmt.Sprintf("-vf:%d", streamIndex), fmt.Sprintf("scale=%d:%d:flags=lanczos", rung.Width, rung.Height),
			fmt.Sprintf("-profile:v:%d", streamIndex), rung.Profile,
			fmt.Sprintf("-level:%d", streamIndex), rung.Level,
			fmt.Sprintf("-crf:%d", streamIndex), strconv.Itoa(rung.CRF),
		)
		
		// Audio encoding settings for this rung
		audioIndex := streamIndex + 1
		args = append(args,
			fmt.Sprintf("-c:a:%d", audioIndex), "aac",
			fmt.Sprintf("-b:a:%d", audioIndex), fmt.Sprintf("%dk", rung.AudioBitrate),
			fmt.Sprintf("-ar:%d", audioIndex), "48000",
			fmt.Sprintf("-profile:a:%d", audioIndex), "aac_low",
			// Let FFmpeg handle channels automatically
		)
		
		// Collect stream indices for adaptation sets
		videoStreamIndices = append(videoStreamIndices, strconv.Itoa(streamIndex))
		audioStreamIndices = append(audioStreamIndices, strconv.Itoa(audioIndex))
	}
	
	// Build adaptation sets - one for all video streams, one for all audio streams
	adaptationSets := fmt.Sprintf("id=0,streams=%s id=1,streams=%s", 
		strings.Join(videoStreamIndices, ","),
		strings.Join(audioStreamIndices, ","))
	
	// DASH muxer settings with fast startup optimization
	segDuration := "1" // 1 second segments for ABR fast startup
	args = append(args,
		"-f", "dash",
		"-seg_duration", segDuration,
		"-frag_duration", "0.5",                // 500ms fragments
		"-use_template", "1",
		"-use_timeline", "1",
		"-single_file", "0",
		"-adaptation_sets", adaptationSets,
		"-media_seg_name", "chunk-$RepresentationID$-$Number$.m4s",
		"-init_seg_name", "init-$RepresentationID$.m4s",
		// VOD optimizations - keep all segments for complete playback
		"-streaming", "0",                       // Disable live streaming mode
		"-ldash", "0",                          // Disable low-latency DASH
		"-remove_at_exit", "0",                // Don't remove segments on exit
		"-window_size", "0",                     // Keep all segments (no sliding window)
		"-extra_window_size", "0",              // No extra window
		"-mpd_profile", "onDemand",             // Force on-demand (static) profile
	)
	
	return args
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
			fmt.Sprintf("-profile:a:%d", i), "aac_low",
		)
		
		// Variant playlist info
		variantStreams = append(variantStreams,
			fmt.Sprintf("v:%d,a:%d,name:%s", i, i, rung.Label),
		)
	}
	
	// HLS muxer settings
	segDuration := b.getAdaptiveSegmentDuration(req)
	args = append(args,
		"-f", "hls",
		"-hls_time", segDuration,
		"-hls_playlist_type", "vod",
		"-hls_segment_type", "mpegts",
		"-hls_flags", "independent_segments",
		"-master_pl_name", "playlist.m3u8",
		"-hls_segment_filename", filepath.Join(outputDir, "stream_%v/segment_%03d.ts"),
		"-var_stream_map", strings.Join(variantStreams, " "),
	)
	
	return args
}