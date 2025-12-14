package streaming

// ByteRange represents a requested byte range for HTTP range requests.
type ByteRange struct {
	Start int64
	End   int64
}

// Length returns the number of bytes in the range.
func (r ByteRange) Length() int64 {
	return r.End - r.Start + 1
}

// VideoInfo contains video file metadata extracted via ffprobe.
type VideoInfo struct {
	Codec           string
	Width           int
	Height          int
	Bitrate         int64
	Duration        float64
	AudioCodec      string
	AudioBitrate    int64
	AudioChannels   int
	ContainerFormat string

	// HDR and color space metadata
	PixelFormat    string
	ColorSpace     string
	ColorPrimaries string
	ColorTransfer  string
	BitDepth       int
	IsHDR          bool
}

// VideoCodec represents supported video codecs for transcoding.
type VideoCodec string

const (
	CodecH264 VideoCodec = "h264"
	CodecH265 VideoCodec = "h265"
	CodecVP9  VideoCodec = "vp9"
	CodecAV1  VideoCodec = "av1"
)

// HardwareAccel represents hardware acceleration type.
type HardwareAccel string

const (
	// AccelNone uses software encoding (slowest, most compatible)
	AccelNone HardwareAccel = "none"

	// AccelVAAPI uses Intel/AMD GPU via VAAPI (Linux)
	AccelVAAPI HardwareAccel = "vaapi"

	// AccelNVENC uses NVIDIA GPU (Linux/Windows)
	AccelNVENC HardwareAccel = "nvenc"

	// AccelQSV uses Intel Quick Sync Video (Linux/Windows)
	AccelQSV HardwareAccel = "qsv"

	// AccelVideoToolbox uses Apple VideoToolbox (macOS)
	AccelVideoToolbox HardwareAccel = "videotoolbox"
)

// IsValid checks if the video codec is a known valid value.
func (c VideoCodec) IsValid() bool {
	switch c {
	case CodecH264, CodecH265, CodecVP9, CodecAV1:
		return true
	default:
		return false
	}
}

// String returns the string representation of the codec.
func (c VideoCodec) String() string {
	return string(c)
}

// IsValid checks if the hardware acceleration type is a known valid value.
func (a HardwareAccel) IsValid() bool {
	switch a {
	case AccelNone, AccelVAAPI, AccelNVENC, AccelQSV, AccelVideoToolbox:
		return true
	default:
		return false
	}
}

// String returns the string representation of the hardware acceleration type.
func (a HardwareAccel) String() string {
	return string(a)
}
