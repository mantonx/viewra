package hls

// AddHardwareVideoEncoding adds hardware-specific video encoding settings.
// Handles NVENC, QSV, VAAPI, and VideoToolbox with appropriate parameters.
// Supports H.264, H.265, VP9, and AV1 codecs based on hardware support.
func (b *Builder) AddHardwareVideoEncoding(hwAccel HardwareAccel) *Builder {
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
func (b *Builder) addNVENCEncoding(p *Profile, codec VideoCodec) {
	// NVENC preset (p1-p7, where p1=fastest, p7=slowest/best quality)
	// Use p4 (balanced) + hq (high quality) for better gradient/sky handling
	// p1+ll caused visible grain artifacts in HDR→SDR tone mapped content
	b.args = append(b.args,
		"-preset", "p4",
		"-tune", "hq", // High quality: better for preserving fine details
		"-rc", "vbr",
		"-cq", "21", // Quality level (lower = better quality, 18-23 typical range)
	)

	b.addBitrateArgs()
	b.addCodecProfileArgs(codec, AccelNVENC)

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

	b.addVideoFilterChain(AccelNVENC, true)

	// GOP structure
	// sc_threshold 0: Disable scene change detection (use fixed GOP for HLS compatibility)
	b.addGOPArgs(true)
}

// addQSVEncoding adds Intel Quick Sync Video encoding parameters.
// Note: QSV has a limitation - no pad_qsv filter exists and scale_qsv doesn't support
// force_original_aspect_ratio. We use scale_qsv with -1 for adaptive dimension to maintain
// aspect ratio, then let the encoder handle any necessary padding.
func (b *Builder) addQSVEncoding(p *Profile, codec VideoCodec) {
	b.args = append(b.args,
		"-preset", "medium",
		"-global_quality", "23",
	)

	b.addBitrateArgs()
	b.addCodecProfileArgs(codec, AccelQSV)
	b.addVideoFilterChain(AccelQSV, false)
	b.addGOPArgs(false)
}

// addVAAPIEncoding adds VAAPI (Intel/AMD) encoding parameters.
// Optimized for full GPU pipeline: VAAPI decode → scale_vaapi → pad_vaapi → VAAPI encode
func (b *Builder) addVAAPIEncoding(p *Profile, codec VideoCodec) {
	b.args = append(b.args, "-quality", "4")

	b.addBitrateArgs()
	b.addCodecProfileArgs(codec, AccelVAAPI)
	b.addVideoFilterChain(AccelVAAPI, false)
	b.addGOPArgs(false)
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
func (b *Builder) addVideoToolboxEncoding(p *Profile, codec VideoCodec) {
	b.addBitrateArgs()
	b.addCodecProfileArgs(codec, AccelVideoToolbox)

	// Build filter chain: tone mapping (if needed) + scaling
	// Note: VideoToolbox doesn't support hardware scaling in FFmpeg, so CPU filters are used
	b.addVideoFilterChain(AccelVideoToolbox, false)
	b.addGOPArgs(false)
}
