package transcoding

import (
	"fmt"
	"path/filepath"
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
func (b *FFmpegArgsBuilder) AddSeekPosition() *FFmpegArgsBuilder {
	if b.opts.UseStartPosition && b.opts.StartPosition > 0 {
		b.args = append(b.args, "-ss", strconv.Itoa(b.opts.StartPosition))
	}
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

// AddVideoEncoding adds full video encoding settings (profile, level, bitrate, etc).
// This is for software encoding (libx264) - hardware encoders should use AddHardwareVideoEncoding.
func (b *FFmpegArgsBuilder) AddVideoEncoding() *FFmpegArgsBuilder {
	p := b.opts.Profile

	// Common video settings
	b.args = append(b.args,
		"-profile:v", "high",
		"-level", "4.1",
		"-pix_fmt", "yuv420p",
	)

	// Video bitrate control
	b.args = append(b.args,
		"-b:v", p.VideoBitrate,
		"-maxrate", p.VideoMaxRate,
		"-bufsize", p.VideoBufSize,
	)

	// Build filter chain: tone mapping (if needed) + scaling
	filterChain := b.buildToneMappingFilter(AccelNone) + b.buildScalingFilter(AccelNone, true)
	b.args = append(b.args, "-vf", filterChain)

	// GOP structure for HLS
	b.args = append(b.args,
		"-g", strconv.Itoa(p.GOPSize),
		"-keyint_min", strconv.Itoa(p.GOPSize),
		"-sc_threshold", "0",
	)

	return b
}

// AddHardwareVideoEncoding adds hardware-specific video encoding settings.
// Handles NVENC, QSV, VAAPI, and VideoToolbox with appropriate parameters.
func (b *FFmpegArgsBuilder) AddHardwareVideoEncoding(hwAccel HardwareAccel) *FFmpegArgsBuilder {
	p := b.opts.Profile

	switch hwAccel {
	case AccelNVENC:
		b.addNVENCEncoding(p)
	case AccelQSV:
		b.addQSVEncoding(p)
	case AccelVAAPI:
		b.addVAAPIEncoding(p)
	case AccelVideoToolbox:
		b.addVideoToolboxEncoding(p)
	default:
		// Fallback to software encoding
		b.AddVideoEncoding()
	}

	return b
}

// addNVENCEncoding adds NVIDIA NVENC-specific encoding parameters.
// Optimized for full GPU pipeline: NVDEC (decode) → libplacebo/scale_cuda → pad_cuda → NVENC (encode)
func (b *FFmpegArgsBuilder) addNVENCEncoding(p *QualityProfile) {
	// NVENC preset (p1-p7, where p1=fastest, p7=slowest/best quality)
	// Use p2 (faster) for real-time streaming - excellent speed/quality balance
	// p2 is ~15-20x faster than software while maintaining good quality
	b.args = append(b.args,
		"-preset", "p2",
		"-tune", "hq", // High quality tuning
		"-profile:v", "high",
		"-level", "4.1",
		"-rc", "vbr", // Variable bitrate
		"-cq", "23", // Constant quality (0-51, lower=better, 23 is good default)
		"-b:v", p.VideoBitrate,
		"-maxrate", p.VideoMaxRate,
		"-bufsize", p.VideoBufSize,
	)

	// Build filter chain: tone mapping (if needed) + scaling
	filterChain := b.buildToneMappingFilter(AccelNVENC) + b.buildScalingFilter(AccelNVENC, true)
	b.args = append(b.args, "-vf", filterChain)

	// GOP structure
	b.args = append(b.args,
		"-g", strconv.Itoa(p.GOPSize),
		"-keyint_min", strconv.Itoa(p.GOPSize),
	)
}

// addQSVEncoding adds Intel Quick Sync Video encoding parameters.
// Note: QSV has a limitation - no pad_qsv filter exists and scale_qsv doesn't support
// force_original_aspect_ratio. We use scale_qsv with -1 for adaptive dimension to maintain
// aspect ratio, then let the encoder handle any necessary padding.
func (b *FFmpegArgsBuilder) addQSVEncoding(p *QualityProfile) {
	b.args = append(b.args,
		"-preset", "medium", // QSV preset: veryfast, faster, fast, medium, slow, slower, veryslow
		"-profile:v", "high",
		"-level", "4.1",
		"-global_quality", "23", // Quality (0-51, lower=better)
		"-b:v", p.VideoBitrate,
		"-maxrate", p.VideoMaxRate,
		"-bufsize", p.VideoBufSize,
	)

	// Build filter chain: tone mapping (if needed) + scaling
	filterChain := b.buildToneMappingFilter(AccelQSV) + b.buildScalingFilter(AccelQSV, false)
	b.args = append(b.args, "-vf", filterChain)

	// GOP structure
	b.args = append(b.args,
		"-g", strconv.Itoa(p.GOPSize),
		"-keyint_min", strconv.Itoa(p.GOPSize),
	)
}

// addVAAPIEncoding adds VAAPI (Intel/AMD) encoding parameters.
// Optimized for full GPU pipeline: VAAPI decode → scale_vaapi → pad_vaapi → VAAPI encode
func (b *FFmpegArgsBuilder) addVAAPIEncoding(p *QualityProfile) {
	b.args = append(b.args,
		"-profile:v", "high",
		"-level", "4.1",
		"-quality", "4", // Quality (1-8, lower=better, 4=balanced)
		"-b:v", p.VideoBitrate,
		"-maxrate", p.VideoMaxRate,
		"-bufsize", p.VideoBufSize,
	)

	// Build filter chain: tone mapping (if needed) + scaling
	filterChain := b.buildToneMappingFilter(AccelVAAPI) + b.buildScalingFilter(AccelVAAPI, false)
	b.args = append(b.args, "-vf", filterChain)

	// GOP structure
	b.args = append(b.args,
		"-g", strconv.Itoa(p.GOPSize),
		"-keyint_min", strconv.Itoa(p.GOPSize),
	)
}

// addVideoToolboxEncoding adds Apple VideoToolbox encoding parameters.
//
// Known Limitation: VideoToolbox (Apple's hardware encoder) does NOT support hardware-accelerated
// scaling or padding filters in FFmpeg. This means:
//   - Video frames are decoded by VideoToolbox on the GPU
//   - Frames are transferred to CPU via hwdownload
//   - Scaling and padding happen on CPU (software filters)
//   - Frames are transferred back to GPU for encoding
//
// This breaks the GPU pipeline and causes ~20-30% performance degradation compared to
// a pure GPU pipeline (like NVENC or VAAPI). However, VideoToolbox is still 6-8x faster
// than pure software encoding (libx264), so it's worth using despite this limitation.
//
// This is a limitation of FFmpeg's VideoToolbox implementation, not our code.
// Apple's AVFoundation framework supports hardware scaling, but FFmpeg doesn't expose it.
func (b *FFmpegArgsBuilder) addVideoToolboxEncoding(p *QualityProfile) {
	b.args = append(b.args,
		"-profile:v", "high",
		"-level", "4.1",
		"-b:v", p.VideoBitrate,
		"-maxrate", p.VideoMaxRate,
		"-bufsize", p.VideoBufSize,
	)

	// Build filter chain: tone mapping (if needed) + scaling
	// Note: VideoToolbox doesn't support hardware scaling in FFmpeg, so CPU filters are used
	filterChain := b.buildToneMappingFilter(AccelVideoToolbox) + b.buildScalingFilter(AccelVideoToolbox, false)
	b.args = append(b.args, "-vf", filterChain)

	// GOP structure
	b.args = append(b.args,
		"-g", strconv.Itoa(p.GOPSize),
		"-keyint_min", strconv.Itoa(p.GOPSize),
	)
}

// AddAudioCodec adds audio codec setting.
func (b *FFmpegArgsBuilder) AddAudioCodec(codec string) *FFmpegArgsBuilder {
	b.args = append(b.args, "-c:a", codec)
	return b
}

// AddAudioEncoding adds full audio encoding settings (bitrate, channels, sample rate).
func (b *FFmpegArgsBuilder) AddAudioEncoding() *FFmpegArgsBuilder {
	p := b.opts.Profile

	b.args = append(b.args,
		"-c:a", "aac",
		"-b:a", p.AudioBitrate,
		"-ac", strconv.Itoa(p.AudioChannels),
		"-ar", strconv.Itoa(p.AudioSampleRate),
	)

	return b
}

// AddAudioDownmix adds audio downmix to stereo settings.
func (b *FFmpegArgsBuilder) AddAudioDownmix() *FFmpegArgsBuilder {
	p := b.opts.Profile

	b.args = append(b.args,
		"-c:a", "aac",
		"-b:a", p.AudioBitrate,
		"-ac", "2", // Force stereo - FFmpeg auto-downmixes
		"-ar", strconv.Itoa(p.AudioSampleRate),
	)

	return b
}

// AddHLSOutput adds HLS output settings (without output file - use AddOutputFile() separately).
func (b *FFmpegArgsBuilder) AddHLSOutput() *FFmpegArgsBuilder {
	p := b.opts.Profile
	segmentPath := filepath.Join(b.opts.OutputDir, SegmentFilenameFormat)

	// Calculate start segment number based on seek position
	startSegmentNum := 0
	if b.opts.UseStartPosition && b.opts.StartPosition > 0 {
		startSegmentNum = b.opts.StartPosition / p.SegmentDuration
	}

	b.args = append(b.args,
		"-f", "hls",
		"-hls_time", strconv.Itoa(p.SegmentDuration),
		"-hls_playlist_type", "event",
		"-hls_segment_filename", segmentPath,
		"-hls_segment_type", "mpegts",
		"-hls_flags", "independent_segments",
		"-start_number", strconv.Itoa(startSegmentNum),
	)

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

// AddOutputFile adds the output file path.
func (b *FFmpegArgsBuilder) AddOutputFile() *FFmpegArgsBuilder {
	outputPlaylist := filepath.Join(b.opts.OutputDir, "playlist.m3u8")
	b.args = append(b.args, outputPlaylist)
	return b
}

// buildToneMappingFilter constructs the tone mapping filter chain for a given hardware acceleration type.
// Returns an empty string if tone mapping is not needed.
// This centralizes tone mapping logic to avoid duplication across encoder functions.
func (b *FFmpegArgsBuilder) buildToneMappingFilter(hwAccel HardwareAccel) string {
	if !b.needsHDRToneMapping() {
		return ""
	}

	p := b.opts.Profile

	// Try libplacebo first (best quality, works for NVENC and software)
	if b.shouldUseLibPlacebo(hwAccel) {
		algorithm := b.getToneMappingLibPlaceboAlgorithm()
		peakDetect := "false"
		if b.opts.LibPlaceboPeakDetect {
			peakDetect = "true"
		}
		contrastRecovery := b.opts.LibPlaceboContrastRecovery

		switch hwAccel {
		case AccelNVENC:
			// CUDA → CPU → libplacebo (Vulkan) → CPU → CUDA
			return fmt.Sprintf("hwdownload,format=p010le,libplacebo=w=%d:h=%d:tonemapping=%s:peak_detect=%s:contrast_recovery=%.2f:format=yuv420p,hwupload_cuda,",
				p.Width, p.Height, algorithm, peakDetect, contrastRecovery)
		case AccelNone:
			// Software encoding with libplacebo (CPU-optimized)
			return fmt.Sprintf("libplacebo=w=%d:h=%d:tonemapping=%s:peak_detect=%s:contrast_recovery=%.2f:format=yuv420p",
				p.Width, p.Height, algorithm, peakDetect, contrastRecovery)
		}
	}

	// Fallback to OpenCL/VAAPI/CPU tone mapping
	algorithm := b.getToneMappingAlgorithm()

	switch hwAccel {
	case AccelNVENC:
		// NVENC with OpenCL: CUDA → OpenCL → CUDA
		return fmt.Sprintf("hwdownload,format=p010le,hwupload,tonemap_opencl=tonemap=%s:desat=0,hwdownload,format=nv12,hwupload_cuda,", algorithm)
	case AccelQSV:
		// QSV with OpenCL: QSV → OpenCL → QSV
		return fmt.Sprintf("hwdownload,format=p010le,hwupload,tonemap_opencl=tonemap=%s:desat=0,hwdownload,format=nv12,hwupload=extra_hw_frames=64,", algorithm)
	case AccelVAAPI:
		// VAAPI native tone mapping
		return fmt.Sprintf("tonemap_vaapi=%s,", algorithm)
	case AccelVideoToolbox, AccelNone:
		// CPU-based tone mapping with zscale + tonemap filter
		return fmt.Sprintf("zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=%s:desat=0,zscale=t=bt709:m=bt709:r=tv,", algorithm)
	}

	return ""
}

// buildScalingFilter constructs the scaling and padding filter chain for a given hardware acceleration type.
// If skipIfLibPlacebo is true and libplacebo handled both tone mapping and scaling, returns empty string.
// This centralizes scaling logic to avoid duplication across encoder functions.
func (b *FFmpegArgsBuilder) buildScalingFilter(hwAccel HardwareAccel, skipIfLibPlacebo bool) string {
	p := b.opts.Profile

	// If libplacebo already handled scaling during tone mapping, skip
	if skipIfLibPlacebo && b.shouldUseLibPlacebo(hwAccel) && b.needsHDRToneMapping() {
		return ""
	}

	switch hwAccel {
	case AccelNVENC:
		// NVENC: GPU-accelerated scaling and padding
		// Add format=nv12 to scale_cuda to convert 10-bit (yuv420p10le) to 8-bit (nv12)
		// pad_cuda doesn't support 10-bit input, so conversion must happen in scale_cuda
		return fmt.Sprintf("scale_cuda=%d:%d:format=nv12:force_original_aspect_ratio=decrease,pad_cuda=%d:%d:(ow-iw)/2:(oh-ih)/2",
			p.Width, p.Height, p.Width, p.Height)
	case AccelQSV:
		// QSV: GPU scaling (no pad_qsv available, encoder handles padding)
		return fmt.Sprintf("scale_qsv=w=%d:h=%d:format=nv12", p.Width, p.Height)
	case AccelVAAPI:
		// VAAPI: GPU-accelerated scaling and padding
		return fmt.Sprintf("scale_vaapi=w=%d:h=%d:format=nv12,pad_vaapi=width=%d:height=%d:x=(ow-iw)/2:y=(oh-ih)/2",
			p.Width, p.Height, p.Width, p.Height)
	case AccelVideoToolbox, AccelNone:
		// Software/VideoToolbox: CPU scaling and padding
		return fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,format=yuv420p",
			p.Width, p.Height, p.Width, p.Height)
	}

	return ""
}

// needsHDRToneMapping determines if HDR tone mapping should be applied.
// Returns true if:
//  1. Tone mapping is enabled in options
//  2. VideoInfo is available
//  3. Content is detected as HDR (HDR10, HLG, etc.)
func (b *FFmpegArgsBuilder) needsHDRToneMapping() bool {
	// Tone mapping must be explicitly enabled
	if !b.opts.ToneMappingEnabled {
		return false
	}

	// Need video metadata to detect HDR
	if b.opts.VideoInfo == nil {
		return false
	}

	// Check if content is HDR
	return b.opts.VideoInfo.IsHDR
}

// getToneMappingAlgorithm returns the configured tone mapping algorithm, defaulting to "hable" if not set.
// This is used for OpenCL, VAAPI, and CPU-based tone mapping.
func (b *FFmpegArgsBuilder) getToneMappingAlgorithm() string {
	if b.opts.ToneMappingAlgorithm == "" {
		return "hable" // Default to Uncharted 2 filmic tone mapping
	}
	return b.opts.ToneMappingAlgorithm
}

// getToneMappingLibPlaceboAlgorithm returns the tone mapping algorithm name for libplacebo.
// Maps our algorithm names to libplacebo's algorithm names.
func (b *FFmpegArgsBuilder) getToneMappingLibPlaceboAlgorithm() string {
	algorithm := b.opts.ToneMappingAlgorithm
	if algorithm == "" {
		return "bt.2390" // Default to ITU-R BT.2390 EETF (broadcast standard)
	}

	// Map algorithm names (some are direct matches, others need mapping)
	switch algorithm {
	case "bt.2390", "bt2390":
		return "bt.2390"
	case "bt.2446a", "bt2446a":
		return "bt.2446a"
	case "st2094-40", "st2094_40":
		return "st2094-40"
	case "st2094-10", "st2094_10":
		return "st2094-10"
	case "spline":
		return "spline"
	case "reinhard":
		return "reinhard"
	case "mobius":
		return "mobius"
	case "hable":
		return "hable"
	case "gamma":
		return "gamma"
	case "linear":
		return "linear"
	case "clip", "none":
		return "clip"
	default:
		return "bt.2390" // Default to broadcast standard
	}
}

// shouldUseLibPlacebo determines if libplacebo should be used for tone mapping.
// Returns true if:
//  1. Tone mapping backend is "libplacebo" or "auto"
//  2. libplacebo filter is available in FFmpeg
//  3. We're using NVENC or software encoding (libplacebo works best here)
func (b *FFmpegArgsBuilder) shouldUseLibPlacebo(hwAccel HardwareAccel) bool {
	backend := b.opts.ToneMappingBackend
	if backend == "" {
		backend = "auto"
	}

	// If explicitly set to libplacebo, try to use it
	if backend == "libplacebo" {
		return CheckFFmpegFilter("libplacebo")
	}

	// If explicitly set to something else, don't use libplacebo
	if backend != "auto" {
		return false
	}

	// Auto mode: Use libplacebo for NVENC and software encoding
	// VAAPI has native tonemap_vaapi which works well
	// QSV will use OpenCL (similar to current implementation)
	switch hwAccel {
	case AccelNVENC, AccelNone:
		return CheckFFmpegFilter("libplacebo")
	default:
		return false
	}
}

// Build returns the final arguments slice.
func (b *FFmpegArgsBuilder) Build() []string {
	return b.args
}
