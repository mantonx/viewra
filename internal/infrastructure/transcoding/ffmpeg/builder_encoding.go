package ffmpeg

import (
	"strconv"
)

// AddVideoEncoding adds full video encoding settings (profile, level, bitrate, etc).
// This is for software encoding - hardware encoders should use AddHardwareVideoEncoding.
// Supports H.264, H.265, VP9, and AV1 codecs.
func (b *FFmpegArgsBuilder) AddVideoEncoding() *FFmpegArgsBuilder {
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
func (b *FFmpegArgsBuilder) addCodecProfile(codec VideoCodec, hwAccel HardwareAccel) {
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
func (b *FFmpegArgsBuilder) addH264Profile(_ HardwareAccel) {
	// Use appropriate level for resolution: 4.1 for 1080p, 5.1 for 4K
	h264Level := getH264Level(b.opts.Profile.Width, b.opts.Profile.Height)
	b.args = append(b.args,
		"-profile:v", "high",
		"-level", h264Level,
		"-pix_fmt", "yuv420p",
	)
}

// addH265Profile adds H.265/HEVC-specific encoding parameters.
func (b *FFmpegArgsBuilder) addH265Profile(hwAccel HardwareAccel) {
	// H.265 Main profile for broad compatibility
	// Main10 profile would support 10-bit but requires more decoder support
	b.args = append(b.args,
		"-profile:v", "main",
		"-level", "5.1", // Level 5.1 supports up to 4K@60fps
		"-pix_fmt", "yuv420p",
	)
	// H.265 specific: use x265 params for software encoding
	if hwAccel == AccelNone {
		b.args = append(b.args, "-x265-params", "log-level=error")
	}
}

// addVP9Profile adds VP9-specific encoding parameters.
func (b *FFmpegArgsBuilder) addVP9Profile(hwAccel HardwareAccel) {
	// VP9 doesn't have traditional profiles like H.264/H.265
	b.args = append(b.args,
		"-pix_fmt", "yuv420p",
		"-quality", "good", // good/best/realtime - good is balanced
		"-speed", "2",      // 0-5 for good quality, lower = slower but better
	)
	// For software VP9, set row-mt for better multi-threading
	if hwAccel == AccelNone {
		b.args = append(b.args, "-row-mt", "1")
	}
}

// addAV1Profile adds AV1-specific encoding parameters.
func (b *FFmpegArgsBuilder) addAV1Profile(hwAccel HardwareAccel) {
	b.args = append(b.args,
		"-pix_fmt", "yuv420p",
	)
	// SVT-AV1 specific parameters for software encoding
	if hwAccel == AccelNone {
		b.args = append(b.args,
			"-preset", "6", // 0-13, 6 is balanced speed/quality for SVT-AV1
		)
	}
}

// AddAudioCodec adds audio codec setting.
func (b *FFmpegArgsBuilder) AddAudioCodec(codec string) *FFmpegArgsBuilder {
	b.args = append(b.args, "-c:a", codec)
	return b
}

// AddAudioEncoding adds full audio encoding settings (bitrate, channels, sample rate).
// Channel count is capped to source channels - we can't expand stereo to 7.1.
// If source has fewer channels than target, FFmpeg would add silent channels which wastes bandwidth.
func (b *FFmpegArgsBuilder) AddAudioEncoding() *FFmpegArgsBuilder {
	p := b.opts.Profile

	// Determine target channel count: min(profile target, source channels)
	// This prevents expanding stereo to surround (which adds silent channels)
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
// A/V sync is handled by -noaccurate_seek in the calling code when copying video.
func (b *FFmpegArgsBuilder) AddAudioDownmix() *FFmpegArgsBuilder {
	p := b.opts.Profile

	b.args = append(b.args,
		"-c:a", "aac",
		"-b:a", formatBitrate(p.AudioBitrate),
		"-ac", "2", // Force stereo - FFmpeg auto-downmixes
		"-ar", strconv.Itoa(p.AudioSampleRate),
	)

	return b
}
