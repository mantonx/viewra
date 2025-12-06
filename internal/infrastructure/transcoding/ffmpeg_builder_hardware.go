package transcoding

import (
	"strconv"
)

// AddHardwareVideoEncoding adds hardware-specific video encoding settings.
// Handles NVENC, QSV, VAAPI, and VideoToolbox with appropriate parameters.
// Supports H.264, H.265, VP9, and AV1 codecs based on hardware support.
func (b *FFmpegArgsBuilder) AddHardwareVideoEncoding(hwAccel HardwareAccel) *FFmpegArgsBuilder {
	p := b.opts.Profile
	codec := b.getVideoCodec()

	switch hwAccel {
	case AccelNVENC:
		b.addNVENCEncoding(p, codec)
	case AccelQSV:
		b.addQSVEncoding(p, codec)
	case AccelVAAPI:
		b.addVAAPIEncoding(p, codec)
	case AccelVideoToolbox:
		b.addVideoToolboxEncoding(p, codec)
	default:
		// Fallback to software encoding
		b.AddVideoEncoding()
	}

	return b
}

// addNVENCEncoding adds NVIDIA NVENC-specific encoding parameters.
// Optimized for full GPU pipeline: NVDEC (decode) → tonemap_opencl → scale_cuda → NVENC (encode)
// Uses p4 preset with hq tune for good balance of speed and quality.
func (b *FFmpegArgsBuilder) addNVENCEncoding(p *AdaptiveProfile, codec VideoCodec) {
	// NVENC preset (p1-p7, where p1=fastest, p7=slowest/best quality)
	// Use p4 (balanced) + hq (high quality) for better gradient/sky handling
	// p1+ll caused visible grain artifacts in HDR→SDR tone mapped content
	b.args = append(b.args,
		"-preset", "p4",
		"-tune", "hq", // High quality: better for preserving fine details
		"-rc", "vbr",
		"-cq", "21", // Quality level (lower = better quality, 18-23 typical range)
		"-b:v", formatBitrate(p.VideoBitrate),
		"-maxrate", formatBitrate(p.VideoMaxRate),
		"-bufsize", formatBitrate(p.VideoBufSize),
	)

	// Add codec-specific profile/level
	switch codec {
	case CodecH265:
		b.args = append(b.args, "-profile:v", "main", "-level", "5.1")
	case CodecAV1:
		// AV1 NVENC (RTX 40 series+) doesn't use profile/level the same way
		b.args = append(b.args, "-tier", "main")
	default: // H.264
		// Use appropriate level for resolution: 4.1 for 1080p, 5.1 for 4K
		h264Level := getH264Level(p.Width, p.Height)
		b.args = append(b.args, "-profile:v", "high", "-level", h264Level)
	}

	// NVENC quality optimizations:
	// - spatial-aq: Adaptive quantization based on spatial complexity (reduces blocking in flat areas)
	// - temporal-aq: Adaptive quantization based on temporal complexity (better motion handling)
	// - aq-strength: How aggressively AQ redistributes bits (1-15, 8 is moderate)
	// - rc-lookahead: Frames to look ahead for rate control decisions (improves quality consistency)
	// - b_ref_mode middle: Use middle B-frame as reference (better compression)
	// - bf 3: Use 3 B-frames between P-frames (good compression/latency balance)
	b.args = append(b.args,
		"-spatial-aq", "1",
		"-temporal-aq", "1",
		"-aq-strength", "8",
		"-rc-lookahead", "32",
		"-b_ref_mode", "middle",
		"-bf", "3",
	)

	// Build filter chain: tone mapping (if needed) + scaling
	filterChain := b.buildToneMappingFilter(AccelNVENC) + b.buildScalingFilter(AccelNVENC, true)
	b.args = append(b.args, "-vf", filterChain)

	// GOP structure
	// sc_threshold 0: Disable scene change detection (use fixed GOP for HLS compatibility)
	b.args = append(b.args,
		"-g", strconv.Itoa(p.GOPSize),
		"-keyint_min", strconv.Itoa(p.GOPSize),
		"-sc_threshold", "0",
	)
}

// addQSVEncoding adds Intel Quick Sync Video encoding parameters.
// Note: QSV has a limitation - no pad_qsv filter exists and scale_qsv doesn't support
// force_original_aspect_ratio. We use scale_qsv with -1 for adaptive dimension to maintain
// aspect ratio, then let the encoder handle any necessary padding.
func (b *FFmpegArgsBuilder) addQSVEncoding(p *AdaptiveProfile, codec VideoCodec) {
	b.args = append(b.args,
		"-preset", "medium",
		"-global_quality", "23",
		"-b:v", formatBitrate(p.VideoBitrate),
		"-maxrate", formatBitrate(p.VideoMaxRate),
		"-bufsize", formatBitrate(p.VideoBufSize),
	)

	// Add codec-specific profile/level
	switch codec {
	case CodecH265:
		b.args = append(b.args, "-profile:v", "main", "-level", "5.1")
	case CodecVP9:
		// VP9 QSV doesn't use profile/level
	case CodecAV1:
		// AV1 QSV (Intel Arc/12th gen+)
		b.args = append(b.args, "-tier", "main")
	default: // H.264
		// Use appropriate level for resolution: 4.1 for 1080p, 5.1 for 4K
		h264Level := getH264Level(p.Width, p.Height)
		b.args = append(b.args, "-profile:v", "high", "-level", h264Level)
	}

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
func (b *FFmpegArgsBuilder) addVAAPIEncoding(p *AdaptiveProfile, codec VideoCodec) {
	b.args = append(b.args,
		"-quality", "4",
		"-b:v", formatBitrate(p.VideoBitrate),
		"-maxrate", formatBitrate(p.VideoMaxRate),
		"-bufsize", formatBitrate(p.VideoBufSize),
	)

	// Add codec-specific profile/level
	switch codec {
	case CodecH265:
		b.args = append(b.args, "-profile:v", "main", "-level", "5.1")
	case CodecVP9:
		// VP9 VAAPI doesn't use profile/level
	case CodecAV1:
		// AV1 VAAPI
		b.args = append(b.args, "-tier", "main")
	default: // H.264
		// Use appropriate level for resolution: 4.1 for 1080p, 5.1 for 4K
		h264Level := getH264Level(p.Width, p.Height)
		b.args = append(b.args, "-profile:v", "high", "-level", h264Level)
	}

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
func (b *FFmpegArgsBuilder) addVideoToolboxEncoding(p *AdaptiveProfile, codec VideoCodec) {
	b.args = append(b.args,
		"-b:v", formatBitrate(p.VideoBitrate),
		"-maxrate", formatBitrate(p.VideoMaxRate),
		"-bufsize", formatBitrate(p.VideoBufSize),
	)

	// Add codec-specific profile/level
	switch codec {
	case CodecH265:
		b.args = append(b.args, "-profile:v", "main", "-level", "5.1")
	default: // H.264 (VideoToolbox doesn't support VP9 or AV1 encoding)
		// Use appropriate level for resolution: 4.1 for 1080p, 5.1 for 4K
		h264Level := getH264Level(p.Width, p.Height)
		b.args = append(b.args, "-profile:v", "high", "-level", h264Level)
	}

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
