// Package ffmpeg provides types and structures for FFmpeg command generation
package ffmpeg

// FFmpegArgs represents the complete set of arguments for an FFmpeg command
type FFmpegArgs struct {
	// Global options (before input)
	GlobalOptions []string
	
	// Input options
	InputOptions []string
	InputPath    string
	
	// Video encoding options
	VideoOptions []string
	
	// Audio encoding options
	AudioOptions []string
	
	// Container/format options
	ContainerOptions []string
	
	// Output mapping
	StreamMappings []string
	
	// Output path
	OutputPath string
}

// VideoCodecArgs contains video codec-specific arguments
type VideoCodecArgs struct {
	Codec       string
	Preset      string
	Profile     string
	Level       string
	CRF         int
	Bitrate     string
	MaxRate     string
	BufSize     string
	PixelFormat string
	KeyframeArgs []string
}

// AudioCodecArgs contains audio codec-specific arguments
type AudioCodecArgs struct {
	Codec      string
	Bitrate    string
	SampleRate int
	Channels   int
	Profile    string
}

// ContainerArgs contains container format-specific arguments
type ContainerArgs struct {
	Format          string
	SegmentDuration int
	InitSegName     string
	MediaSegName    string
	UseTimeline     bool
	UseTemplate     bool
	SingleFile      bool
	DashArgs        []string
	HLSArgs         []string
}

// ABRStreamConfig represents configuration for one stream in adaptive bitrate
type ABRStreamConfig struct {
	Height      int
	Width       int
	Bitrate     string
	MaxRate     string
	BufSize     string
	CRF         int
	Profile     string
	Level       string
	StreamIndex int
}

// CommonFFmpegOptions contains commonly used FFmpeg options with detailed explanations
var CommonFFmpegOptions = struct {
	// Global options
	OverwriteOutput   []string // -y: Overwrite output files without asking
	HideBanner        []string // -hide_banner: Hide FFmpeg build configuration banner
	StatsLogLevel     []string // -loglevel error -stats_period 1 -stats: Show encoding progress every 1 second
	ProgressURL       []string // -progress: URL/file to send progress information
	
	// Performance options
	ThreadsAuto       []string // -threads 0: Let FFmpeg auto-detect optimal thread count
	
	// Quality options
	FastStart         []string // -movflags +faststart: Move metadata to beginning for streaming
	MovFlags          []string // -movflags: Enable fragmented MP4 for DASH/HLS compatibility
}{
	OverwriteOutput:   []string{"-y"},
	HideBanner:        []string{"-hide_banner"},
	StatsLogLevel:     []string{"-loglevel", "error", "-stats_period", "1", "-stats"},
	ProgressURL:       []string{"-progress"},
	ThreadsAuto:       []string{"-threads", "0"},
	FastStart:         []string{"-movflags", "+faststart"},
	MovFlags:          []string{"-movflags", "frag_keyframe+empty_moov+delay_moov+default_base_moof"},
}

// VideoProfiles defines standard H.264/H.265 profiles
var VideoProfiles = struct {
	H264 struct {
		Baseline   string
		Main       string
		High       string
		High10     string
		High422    string
		High444    string
	}
	H265 struct {
		Main       string
		Main10     string
		MainStill  string
	}
}{
	H264: struct {
		Baseline   string
		Main       string
		High       string
		High10     string
		High422    string
		High444    string
	}{
		Baseline: "baseline",
		Main:     "main",
		High:     "high",
		High10:   "high10",
		High422:  "high422",
		High444:  "high444",
	},
	H265: struct {
		Main       string
		Main10     string
		MainStill  string
	}{
		Main:      "main",
		Main10:    "main10",
		MainStill: "mainstillpicture",
	},
}

// VideoLevels defines standard H.264/H.265 levels
var VideoLevels = struct {
	H264 struct {
		Level30 string
		Level31 string
		Level40 string
		Level41 string
		Level42 string
		Level50 string
		Level51 string
		Level52 string
	}
	H265 struct {
		Level30 string
		Level31 string
		Level40 string
		Level41 string
		Level50 string
		Level51 string
		Level52 string
	}
}{
	H264: struct {
		Level30 string
		Level31 string
		Level40 string
		Level41 string
		Level42 string
		Level50 string
		Level51 string
		Level52 string
	}{
		Level30: "3.0",
		Level31: "3.1",
		Level40: "4.0",
		Level41: "4.1",
		Level42: "4.2",
		Level50: "5.0",
		Level51: "5.1",
		Level52: "5.2",
	},
	H265: struct {
		Level30 string
		Level31 string
		Level40 string
		Level41 string
		Level50 string
		Level51 string
		Level52 string
	}{
		Level30: "3.0",
		Level31: "3.1",
		Level40: "4.0",
		Level41: "4.1",
		Level50: "5.0",
		Level51: "5.1",
		Level52: "5.2",
	},
}

// PresetNames defines x264/x265 encoding presets
var PresetNames = struct {
	UltraFast string
	SuperFast string
	VeryFast  string
	Faster    string
	Fast      string
	Medium    string
	Slow      string
	Slower    string
	VerySlow  string
	Placebo   string
}{
	UltraFast: "ultrafast",
	SuperFast: "superfast",
	VeryFast:  "veryfast",
	Faster:    "faster",
	Fast:      "fast",
	Medium:    "medium",
	Slow:      "slow",
	Slower:    "slower",
	VerySlow:  "veryslow",
	Placebo:   "placebo",
}

// PixelFormats defines common pixel formats
var PixelFormats = struct {
	YUV420P   string
	YUV422P   string
	YUV444P   string
	YUV420P10 string
	YUV422P10 string
	YUV444P10 string
	NV12      string
}{
	YUV420P:   "yuv420p",
	YUV422P:   "yuv422p",
	YUV444P:   "yuv444p",
	YUV420P10: "yuv420p10le",
	YUV422P10: "yuv422p10le",
	YUV444P10: "yuv444p10le",
	NV12:      "nv12",
}

// AudioCodecs defines common audio codec names
var AudioCodecs = struct {
	AAC    string
	MP3    string
	Opus   string
	Vorbis string
	FLAC   string
	PCM    string
}{
	AAC:    "aac",
	MP3:    "libmp3lame",
	Opus:   "libopus",
	Vorbis: "libvorbis",
	FLAC:   "flac",
	PCM:    "pcm_s16le",
}

// VideoCodecs defines common video codec names
var VideoCodecs = struct {
	H264 string
	H265 string
	VP9  string
	VP8  string
	AV1  string
}{
	H264: "libx264",
	H265: "libx265",
	VP9:  "libvpx-vp9",
	VP8:  "libvpx",
	AV1:  "libaom-av1",
}