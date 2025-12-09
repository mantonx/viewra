package hls

import (
	"fmt"
)

// buildToneMappingFilter constructs the tone mapping filter chain for a given hardware acceleration type.
// Returns an empty string if tone mapping is not needed.
// This centralizes tone mapping logic to avoid duplication across encoder functions.
//
// Filter priority for NVENC:
//  1. tonemap_cuda - CUDA-native, fastest (no memory transfers)
//  2. libplacebo - High quality via Vulkan (requires GPU→CPU→GPU)
//  3. tonemap_opencl - OpenCL fallback (requires GPU→CPU→GPU)
func (b *Builder) buildToneMappingFilter(hwAccel HardwareAccel) string {
	if !b.needsHDRToneMapping() {
		return ""
	}

	p := b.opts.Profile

	// For NVENC, use OpenCL tone mapping (reliable and fast)
	// tonemap_cuda has issues with Dolby Vision content, so we default to OpenCL
	if hwAccel == AccelNVENC && b.shouldUseOpenCL() {
		algorithm := b.getToneMappingAlgorithm()
		// NVENC with OpenCL: CUDA → CPU → OpenCL (tonemap) → CPU → CUDA
		// - hwdownload: transfer from CUDA to CPU
		// - format=p010le: ensure 10-bit HDR format for tonemap input
		// - hwupload: upload to OpenCL context (uses -filter_hw_device ocl)
		// - tonemap_opencl: HDR→SDR conversion
		// - hwdownload,format=nv12: back to CPU in nv12 format
		// - hwupload_cuda: transfer to CUDA for NVENC encoding
		// - scale_cuda: GPU-accelerated scaling (no pad_cuda available in standard FFmpeg)
		return fmt.Sprintf("hwdownload,format=p010le,hwupload,tonemap_opencl=tonemap=%s:desat=0:format=nv12:t=bt709:m=bt709:p=bt709,hwdownload,format=nv12,hwupload_cuda,scale_cuda=%d:%d:force_original_aspect_ratio=decrease:format=nv12",
			algorithm, p.Width, p.Height)
	}

	// Try libplacebo for software encoding (best quality)
	if hwAccel == AccelNone && b.shouldUseLibPlacebo(hwAccel) {
		algorithm := b.getToneMappingLibPlaceboAlgorithm()
		peakDetect := "false"
		if b.opts.LibPlaceboPeakDetect {
			peakDetect = "true"
		}
		contrastRecovery := b.opts.LibPlaceboContrastRecovery
		// Software encoding with libplacebo (CPU-optimized)
		return fmt.Sprintf("libplacebo=w=%d:h=%d:tonemapping=%s:peak_detect=%s:contrast_recovery=%.2f:format=yuv420p",
			p.Width, p.Height, algorithm, peakDetect, contrastRecovery)
	}

	// Fallback to VAAPI/CPU tone mapping
	algorithm := b.getToneMappingAlgorithm()

	switch hwAccel {
	case AccelNVENC:
		// NVENC with OpenCL: CUDA → CPU → OpenCL (tonemap) → CPU → CUDA
		// - hwdownload: transfer from CUDA to CPU
		// - format=p010le: ensure 10-bit HDR format for tonemap input
		// - hwupload: upload to OpenCL context (uses -filter_hw_device ocl)
		// - tonemap_opencl: HDR→SDR with format=nv12 output
		// - hwdownload,format=nv12: back to CPU in nv12 format
		// - hwupload_cuda: transfer to CUDA for NVENC encoding (added by buildScalingFilter)
		// Trailing comma allows scaling filter to be appended
		return fmt.Sprintf("hwdownload,format=p010le,hwupload,tonemap_opencl=tonemap=%s:desat=0:format=nv12:t=bt709:m=bt709:p=bt709,hwdownload,format=nv12,",
			algorithm)
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
func (b *Builder) buildScalingFilter(hwAccel HardwareAccel, skipIfLibPlacebo bool) string {
	p := b.opts.Profile

	// If tonemap_cuda was used, it already includes scaling - skip
	if hwAccel == AccelNVENC && b.shouldUseToneCuda() && b.needsHDRToneMapping() {
		return ""
	}

	// If libplacebo already handled scaling during tone mapping, skip
	if skipIfLibPlacebo && b.shouldUseLibPlacebo(hwAccel) && b.needsHDRToneMapping() {
		return ""
	}

	// Skip scaling if source resolution matches target (avoid redundant processing)
	// This is especially important for 4K→4K where scaling adds latency with no benefit
	sourceMatchesTarget := b.opts.VideoInfo != nil &&
		b.opts.VideoInfo.Width == p.Width &&
		b.opts.VideoInfo.Height == p.Height

	// For NVENC with HDR tone mapping, the input is nv12 on CPU from tonemap_opencl
	// We need hwupload_cuda to transfer back to GPU, optionally with scaling
	if hwAccel == AccelNVENC && b.needsHDRToneMapping() {
		if sourceMatchesTarget {
			// No scaling needed - just upload to CUDA for encoding
			return "hwupload_cuda"
		}
		// Scale on CUDA - no pad_cuda available in standard FFmpeg builds
		// Use force_original_aspect_ratio=decrease to maintain aspect ratio (may letterbox)
		// Download to CPU for padding, then re-upload to CUDA
		return fmt.Sprintf("hwupload_cuda,scale_cuda=%d:%d:force_original_aspect_ratio=decrease:format=nv12,hwdownload,format=nv12,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,hwupload_cuda",
			p.Width, p.Height, p.Width, p.Height)
	}

	switch hwAccel {
	case AccelNVENC:
		// NVENC: GPU-accelerated scaling (non-HDR path)
		// Add format=nv12 to scale_cuda to convert 10-bit (yuv420p10le) to 8-bit (nv12)
		// No pad_cuda available in standard FFmpeg - download, pad on CPU, re-upload
		return fmt.Sprintf("scale_cuda=%d:%d:format=nv12:force_original_aspect_ratio=decrease,hwdownload,format=nv12,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,hwupload_cuda",
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
func (b *Builder) needsHDRToneMapping() bool {
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
// Maps advanced algorithms (bt.2390, etc) to closest available OpenCL equivalents.
func (b *Builder) getToneMappingAlgorithm() string {
	algorithm := b.opts.ToneMappingAlgorithm
	if algorithm == "" {
		return "hable" // Default to Uncharted 2 filmic tone mapping
	}

	// OpenCL/VAAPI/CPU support: none, linear, gamma, clip, reinhard, hable, mobius
	// Map advanced algorithms to their closest equivalents
	switch algorithm {
	case "bt.2390", "bt2390", "bt.2446a", "bt2446a", "st2094-40", "st2094-10", "spline":
		// These advanced algorithms don't exist in OpenCL, use mobius as closest
		// mobius provides good highlight rolloff similar to bt.2390
		return "mobius"
	case "reinhard", "hable", "mobius", "linear", "gamma", "clip", "none":
		// Direct pass-through for supported algorithms
		return algorithm
	default:
		return "hable" // Safe default
	}
}

// getToneMappingLibPlaceboAlgorithm returns the tone mapping algorithm name for libplacebo.
// Maps our algorithm names to libplacebo's algorithm names.
func (b *Builder) getToneMappingLibPlaceboAlgorithm() string {
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

// shouldUseOpenCL determines if tonemap_opencl should be used for tone mapping.
// OpenCL is the most reliable option for HDR tone mapping on NVIDIA GPUs.
// Returns true if:
//  1. Tone mapping backend is "auto" or "opencl"
//  2. tonemap_opencl filter is available in FFmpeg
func (b *Builder) shouldUseOpenCL() bool {
	backend := b.opts.ToneMappingBackend
	if backend == "" {
		backend = "auto"
	}

	// If explicitly set to opencl, try to use it
	if backend == "opencl" {
		return CheckFFmpegFilter("tonemap_opencl")
	}

	// If explicitly set to something else (cuda, libplacebo), don't use opencl
	if backend != "auto" {
		return false
	}

	// Auto mode: prefer tonemap_opencl for NVENC (most reliable)
	return CheckFFmpegFilter("tonemap_opencl")
}

// shouldUseToneCuda determines if tonemap_cuda should be used for NVENC tone mapping.
// tonemap_cuda is the fastest option as it stays entirely in CUDA memory.
// Returns true if:
//  1. Tone mapping backend is "auto" or "cuda"
//  2. tonemap_cuda filter is available in FFmpeg (requires patched FFmpeg)
func (b *Builder) shouldUseToneCuda() bool {
	backend := b.opts.ToneMappingBackend
	if backend == "" {
		backend = "auto"
	}

	// If explicitly set to cuda, try to use it
	if backend == "cuda" {
		return CheckFFmpegFilter("tonemap_cuda")
	}

	// If explicitly set to something else (libplacebo, opencl), don't use cuda
	if backend != "auto" {
		return false
	}

	// Auto mode: prefer tonemap_cuda if available (fastest for NVENC)
	return CheckFFmpegFilter("tonemap_cuda")
}

// getToneMappingCudaAlgorithm returns the tone mapping algorithm for tonemap_cuda.
// Maps our algorithm names to tonemap_cuda's supported algorithms.
// Supported: none, clip, linear, gamma, reinhard, hable, mobius, bt2390
func (b *Builder) getToneMappingCudaAlgorithm() string {
	algorithm := b.opts.ToneMappingAlgorithm
	if algorithm == "" {
		return "bt2390" // Default to ITU-R BT.2390 (best quality)
	}

	// Map algorithm names to tonemap_cuda equivalents
	switch algorithm {
	case "bt.2390", "bt2390":
		return "bt2390"
	case "reinhard":
		return "reinhard"
	case "hable":
		return "hable"
	case "mobius":
		return "mobius"
	case "linear":
		return "linear"
	case "gamma":
		return "gamma"
	case "clip", "none":
		return "clip"
	default:
		return "bt2390" // Default to broadcast standard
	}
}

// shouldUseLibPlacebo determines if libplacebo should be used for tone mapping.
// Returns true if:
//  1. Tone mapping backend is explicitly set to "libplacebo"
//  2. libplacebo filter is available in FFmpeg
//  3. We're using software encoding OR NVENC (where libplacebo works reliably)
//
// For VAAPI and QSV, native tone mapping is preferred as it's fully GPU-based.
// For NVENC, libplacebo is preferred over OpenCL because:
//   - OpenCL tone mapping has reliability issues (memory allocation failures)
//   - libplacebo uses Vulkan which is more stable on NVIDIA
//   - Quality is excellent with bt.2390 algorithm
//
// Note: tonemap_cuda is checked before this function and takes priority when available.
func (b *Builder) shouldUseLibPlacebo(hwAccel HardwareAccel) bool {
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

	// Auto mode: Use libplacebo for software encoding and NVENC (if tonemap_cuda not available)
	// VAAPI/QSV have native tone mapping that's fully GPU-based
	switch hwAccel {
	case AccelNone:
		// Software encoding: libplacebo provides best quality without transfer overhead
		return CheckFFmpegFilter("libplacebo")
	case AccelNVENC:
		// NVENC: libplacebo via Vulkan is more reliable than OpenCL
		// (tonemap_cuda is checked before this, so we only reach here if it's not available)
		return CheckFFmpegFilter("libplacebo")
	default:
		// VAAPI/QSV: use native tone mapping for fully GPU-based processing
		return false
	}
}
