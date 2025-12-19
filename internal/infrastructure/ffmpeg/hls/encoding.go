package hls

import (
	"fmt"
	"strconv"
)

// formatBitrate converts an integer bitrate (bits per second) to FFmpeg format string.
// e.g., 5000000 -> "5000k", 15000000 -> "15000k"
func formatBitrate(bps int) string {
	return fmt.Sprintf("%dk", bps/1000)
}

// getH264Level returns the appropriate H.264 level for a given resolution.
func getH264Level(width, height int) string {
	pixels := width * height
	if pixels > 2073600 { // > 1920x1080
		return "5.1" // Required for 4K
	}
	if pixels > 921600 { // > 1280x720
		return "4.1" // 1080p
	}
	return "4.0" // 720p and below
}

// getVideoCodec returns the video codec to use, defaulting to H.264 if not specified.
func (b *Builder) getVideoCodec() VideoCodec {
	if b.opts.VideoCodec == "" {
		return CodecH264
	}
	return b.opts.VideoCodec
}

// AddVideoEncoding adds full video encoding settings (profile, level, bitrate, etc).
// This is for software encoding - hardware encoders should use AddHardwareVideoEncoding.
func (b *Builder) AddVideoEncoding() *Builder {
	codec := b.getVideoCodec()

	// Add codec-specific profile and settings
	b.addCodecProfile(codec, AccelNone)

	// Video bitrate control
	b.addBitrateArgs()

	// Build filter chain: tone mapping (if needed) + scaling
	b.addVideoFilterChain(AccelNone, true)

	// GOP structure for HLS
	b.addGOPArgs(true)

	return b
}

// addCodecProfile adds codec-specific profile, level, and pixel format settings.
func (b *Builder) addCodecProfile(codec VideoCodec, hwAccel HardwareAccel) {
	switch codec {
	case CodecH265:
		b.addH265Profile(hwAccel)
	case CodecVP9:
		b.addVP9Profile(hwAccel)
	case CodecAV1:
		b.addAV1Profile(hwAccel)
	default: // CodecH264
		b.addH264Profile(hwAccel)
	}
}

// addH264Profile adds H.264-specific encoding parameters.
func (b *Builder) addH264Profile(_ HardwareAccel) {
	h264Level := getH264Level(b.opts.Profile.Width, b.opts.Profile.Height)
	b.args = append(b.args,
		"-profile:v", "high",
		"-level", h264Level,
		"-pix_fmt", "yuv420p",
	)
}

// addH265Profile adds H.265/HEVC-specific encoding parameters.
func (b *Builder) addH265Profile(hwAccel HardwareAccel) {
	b.args = append(b.args,
		"-profile:v", "main",
		"-level", "5.1",
		"-pix_fmt", "yuv420p",
	)
	if hwAccel == AccelNone {
		b.args = append(b.args, "-x265-params", "log-level=error")
	}
}

// addVP9Profile adds VP9-specific encoding parameters.
func (b *Builder) addVP9Profile(hwAccel HardwareAccel) {
	b.args = append(b.args,
		"-pix_fmt", "yuv420p",
		"-quality", "good",
		"-speed", "2",
	)
	if hwAccel == AccelNone {
		b.args = append(b.args, "-row-mt", "1")
	}
}

// addAV1Profile adds AV1-specific encoding parameters.
func (b *Builder) addAV1Profile(hwAccel HardwareAccel) {
	b.args = append(b.args, "-pix_fmt", "yuv420p")
	if hwAccel == AccelNone {
		b.args = append(b.args, "-preset", "6")
	}
}

// AddAudioCodec adds audio codec setting.
func (b *Builder) AddAudioCodec(codec string) *Builder {
	b.args = append(b.args, "-c:a", codec)
	return b
}

// AddAudioEncoding adds full audio encoding settings (bitrate, channels, sample rate).
func (b *Builder) AddAudioEncoding() *Builder {
	p := b.opts.Profile

	// Determine target channel count: min(profile target, source channels)
	targetChannels := p.AudioChannels
	if b.opts.VideoInfo != nil && b.opts.VideoInfo.AudioChannels > 0 {
		sourceChannels := b.opts.VideoInfo.AudioChannels
		if sourceChannels < targetChannels {
			targetChannels = sourceChannels
		}
	}

	b.args = append(b.args,
		"-c:a", "aac",
		"-b:a", formatBitrate(p.AudioBitrate),
		"-ac", strconv.Itoa(targetChannels),
		"-ar", strconv.Itoa(p.AudioSampleRate),
	)

	return b
}

// AddAudioDownmix adds audio downmix to stereo settings.
func (b *Builder) AddAudioDownmix() *Builder {
	p := b.opts.Profile

	b.args = append(b.args,
		"-c:a", "aac",
		"-b:a", formatBitrate(p.AudioBitrate),
		"-ac", "2",
		"-ar", strconv.Itoa(p.AudioSampleRate),
	)

	return b
}

// AddVideoCodec adds video codec settings.
func (b *Builder) AddVideoCodec(codec, preset string) *Builder {
	b.args = append(b.args, "-c:v", codec)

	if preset != "" {
		b.args = append(b.args, "-preset", preset)
	}

	return b
}

// AddH264Copy adds H.264 video stream copy with the h264_mp4toannexb bitstream filter.
func (b *Builder) AddH264Copy() *Builder {
	b.args = append(b.args,
		"-c:v", "copy",
		"-bsf:v", "h264_mp4toannexb",
	)
	return b
}

// AddHEVCCopy adds HEVC video stream copy with the hevc_mp4toannexb bitstream filter.
func (b *Builder) AddHEVCCopy() *Builder {
	b.args = append(b.args,
		"-c:v", "copy",
		"-bsf:v", "hevc_mp4toannexb",
	)
	return b
}

// AddCopyTimestamps preserves original timestamps when copying streams.
func (b *Builder) AddCopyTimestamps() *Builder {
	b.args = append(b.args, "-copyts")
	return b
}

// AddCopyTimestampsWithReset preserves original timestamps but resets output to start at 0.
func (b *Builder) AddCopyTimestampsWithReset() *Builder {
	b.args = append(b.args, "-copyts", "-start_at_zero")
	return b
}

// AddOverwriteOutput adds the -y flag to overwrite output files.
func (b *Builder) AddOverwriteOutput() *Builder {
	b.args = append(b.args, "-y")
	return b
}

// addBitrateArgs adds video bitrate control arguments.
func (b *Builder) addBitrateArgs() {
	p := b.opts.Profile
	b.args = append(b.args,
		"-b:v", formatBitrate(p.VideoBitrate),
		"-maxrate", formatBitrate(p.VideoMaxRate),
		"-bufsize", formatBitrate(p.VideoBufSize),
	)
}

// addCodecProfileArgs adds codec-specific profile and level arguments for hardware encoders.
func (b *Builder) addCodecProfileArgs(codec VideoCodec, hwAccel HardwareAccel) {
	p := b.opts.Profile

	switch codec {
	case CodecH265:
		b.args = append(b.args, "-profile:v", "main", "-level", "5.1")
	case CodecVP9:
		// VP9 with QSV/VAAPI doesn't use profile/level
	case CodecAV1:
		b.args = append(b.args, "-tier", "main")
	default: // H.264
		h264Level := getH264Level(p.Width, p.Height)
		b.args = append(b.args, "-profile:v", "high", "-level", h264Level)
	}
}

// addGOPArgs adds GOP structure arguments for HLS compatibility.
func (b *Builder) addGOPArgs(disableSceneDetection bool) {
	p := b.opts.Profile
	b.args = append(b.args,
		"-g", strconv.Itoa(p.GOPSize),
		"-keyint_min", strconv.Itoa(p.GOPSize),
	)
	if disableSceneDetection {
		b.args = append(b.args, "-sc_threshold", "0")
	}
}

// addVideoFilterChain builds and adds the complete video filter chain.
func (b *Builder) addVideoFilterChain(hwAccel HardwareAccel, skipIfLibPlacebo bool) {
	filterChain := b.buildToneMappingFilter(hwAccel) + b.buildScalingFilter(hwAccel, skipIfLibPlacebo)
	b.args = append(b.args, "-vf", filterChain)
}

// AddOpenCLDevice initializes an OpenCL hardware device for GPU tone mapping.
func (b *Builder) AddOpenCLDevice() *Builder {
	b.args = append(b.args, "-init_hw_device", "opencl=ocl:0.0")
	return b
}

// AddOpenCLFilterDevice sets the hardware device context for filters.
func (b *Builder) AddOpenCLFilterDevice() *Builder {
	b.args = append(b.args, "-filter_hw_device", "ocl")
	return b
}

// AddVulkanDevice initializes a Vulkan hardware device for GPU tone mapping via libplacebo.
// Uses device 0 by default (typically the primary GPU).
func (b *Builder) AddVulkanDevice() *Builder {
	b.args = append(b.args, "-init_hw_device", "vulkan=vk:0")
	return b
}

// AddVulkanFilterDevice sets the Vulkan device context for filter operations.
func (b *Builder) AddVulkanFilterDevice() *Builder {
	b.args = append(b.args, "-filter_hw_device", "vk")
	return b
}

// AddFastInputOptions adds options to speed up input file analysis.
func (b *Builder) AddFastInputOptions() *Builder {
	b.args = append(b.args,
		"-analyzeduration", "5000000",
		"-probesize", "5000000",
		"-fflags", "+genpts+discardcorrupt",
	)
	return b
}
