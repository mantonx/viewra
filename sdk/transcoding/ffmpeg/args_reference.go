// Package ffmpeg provides predefined argument groups for FFmpeg commands.
// This file contains organized collections of common FFmpeg arguments
// to ensure consistency and reduce duplication across the transcoding system.
package ffmpeg

import (
	"fmt"
	"strings"
)

// GlobalArgs contains general FFmpeg arguments that apply to the entire command
var GlobalArgs = struct {
	// Overwrite output files without asking
	Overwrite []string
	// Hide FFmpeg banner for cleaner logs
	HideBanner []string
	// Hardware acceleration
	HwAccel []string
	// Threading options
	Threads []string
}{
	Overwrite:  []string{"-y"},
	HideBanner: []string{"-hide_banner"},
	HwAccel:    []string{"-hwaccel", "auto"},
	Threads:    []string{"-threads", "0"},
}

// InputArgs contains arguments related to input handling
var InputArgs = struct {
	// Input file specification
	Input []string
	// Seek to start position
	SeekStart []string
	// Probe size for stream detection
	ProbeSize []string
	// Analysis duration
	AnalyzeDuration []string
	// Format flags
	FormatFlags []string
}{
	Input:           []string{"-i"},
	SeekStart:       []string{"-ss"},
	ProbeSize:       []string{"-probesize", "5000000"},
	AnalyzeDuration: []string{"-analyzeduration", "5000000"},
	FormatFlags:     []string{"-fflags", "+genpts+fastseek"},
}

// StreamMappingArgs contains arguments for stream selection and mapping
var StreamMappingArgs = struct {
	// Map streams from input to output
	Map []string
	// Avoid negative timestamps
	AvoidNegativeTs []string
}{
	Map:             []string{"-map"},
	AvoidNegativeTs: []string{"-avoid_negative_ts", "make_zero"},
}

// VideoEncodingArgs contains video encoding specific arguments
var VideoEncodingArgs = struct {
	// Video codec specification
	Codec []string
	// Constant Rate Factor (quality)
	CRF []string
	// Video bitrate
	Bitrate []string
	// Maximum bitrate
	MaxRate []string
	// Buffer size
	BufSize []string
	// Keyframe interval (GOP size)
	KeyInt []string
	// Minimum keyframe interval
	KeyIntMin []string
	// Scene change threshold
	ScThreshold []string
	// Video profile
	Profile []string
	// Video level
	Level []string
	// Preset for encoding speed/quality tradeoff
	Preset []string
	// Video filters
	VideoFilter []string
	// Force key frames
	ForceKeyFrames []string
}{
	Codec:          []string{"-c:v"},
	CRF:            []string{"-crf"},
	Bitrate:        []string{"-b:v"},
	MaxRate:        []string{"-maxrate"},
	BufSize:        []string{"-bufsize"},
	KeyInt:         []string{"-g"},
	KeyIntMin:      []string{"-keyint_min"},
	ScThreshold:    []string{"-sc_threshold"},
	Profile:        []string{"-profile:v"},
	Level:          []string{"-level"},
	Preset:         []string{"-preset"},
	VideoFilter:    []string{"-vf"},
	ForceKeyFrames: []string{"-force_key_frames"},
}

// AudioEncodingArgs contains audio encoding specific arguments
var AudioEncodingArgs = struct {
	// Audio codec specification
	Codec []string
	// Audio bitrate
	Bitrate []string
	// Audio sample rate
	SampleRate []string
	// Audio channels
	Channels []string
	// Audio profile
	Profile []string
	// Audio filters
	AudioFilter []string
}{
	Codec:       []string{"-c:a"},
	Bitrate:     []string{"-b:a"},
	SampleRate:  []string{"-ar"},
	Channels:    []string{"-ac"},
	Profile:     []string{"-profile:a"},
	AudioFilter: []string{"-af"},
}

// ContainerArgs contains container/muxer specific arguments
var ContainerArgs = struct {
	// Output format
	Format []string
	// MOV flags for MP4/MOV containers
	MovFlags []string
	// Fragment duration
	FragDuration []string
	// Minimum fragment duration
	MinFragDuration []string
}{
	Format:          []string{"-f"},
	MovFlags:        []string{"-movflags"},
	FragDuration:    []string{"-frag_duration"},
	MinFragDuration: []string{"-min_frag_duration"},
}

// DashArgs contains DASH-specific arguments
var DashArgs = struct {
	// DASH segment type
	SegmentType []string
	// Segment duration
	SegDuration []string
	// Use timeline addressing
	UseTimeline []string
	// Use template addressing
	UseTemplate []string
	// Single file mode
	SingleFile []string
	// Window size for live streaming
	WindowSize []string
	// Remove segments at exit
	RemoveAtExit []string
	// Adaptation sets
	AdaptationSets []string
	// Media segment naming
	MediaSegName []string
	// Init segment naming
	InitSegName []string
	// Streaming mode
	Streaming []string
	// Low latency DASH
	LDash []string
	// Minimum segment duration
	MinSegDuration []string
}{
	SegmentType:    []string{"-dash_segment_type"},
	SegDuration:    []string{"-seg_duration"},
	UseTimeline:    []string{"-use_timeline"},
	UseTemplate:    []string{"-use_template"},
	SingleFile:     []string{"-single_file"},
	WindowSize:     []string{"-window_size"},
	RemoveAtExit:   []string{"-remove_at_exit"},
	AdaptationSets: []string{"-adaptation_sets"},
	MediaSegName:   []string{"-media_seg_name"},
	InitSegName:    []string{"-init_seg_name"},
	Streaming:      []string{"-streaming"},
	LDash:          []string{"-ldash"},
	MinSegDuration: []string{"-min_seg_duration"},
}

// HlsArgs contains HLS-specific arguments
var HlsArgs = struct {
	// HLS segment duration
	Time []string
	// HLS playlist type
	PlaylistType []string
	// HLS segment type
	SegmentType []string
	// HLS flags
	Flags []string
	// HLS list size
	ListSize []string
	// HLS segment filename pattern
	SegmentFilename []string
	// HLS fMP4 init filename
	FMP4InitFilename []string
	// Master playlist name
	MasterPlaylistName []string
	// Variant stream map
	VarStreamMap []string
}{
	Time:             []string{"-hls_time"},
	PlaylistType:     []string{"-hls_playlist_type"},
	SegmentType:      []string{"-hls_segment_type"},
	Flags:            []string{"-hls_flags"},
	ListSize:         []string{"-hls_list_size"},
	SegmentFilename:  []string{"-hls_segment_filename"},
	FMP4InitFilename: []string{"-hls_fmp4_init_filename"},
	MasterPlaylistName: []string{"-master_pl_name"},
	VarStreamMap:     []string{"-var_stream_map"},
}

// FilterArgs contains video and audio filter arguments
var FilterArgs = struct {
	// Complex filter graph
	FilterComplex []string
	// Video filter
	VideoFilter []string
	// Audio filter
	AudioFilter []string
	// Scale filter
	Scale []string
	// Format filter
	Format []string
	// Deinterlace filter
	Deinterlace []string
}{
	FilterComplex: []string{"-filter_complex"},
	VideoFilter:   []string{"-vf"},
	AudioFilter:   []string{"-af"},
	Scale:         []string{"scale"},
	Format:        []string{"format"},
	Deinterlace:   []string{"yadif"},
}

// QualityArgs contains quality control arguments
var QualityArgs = struct {
	// Constant Rate Factor
	CRF []string
	// Quantizer scale
	Qscale []string
	// Quality factor for VBR
	Quality []string
	// x264 parameters
	X264Params []string
	// x265 parameters
	X265Params []string
}{
	CRF:        []string{"-crf"},
	Qscale:     []string{"-qscale"},
	Quality:    []string{"-q"},
	X264Params: []string{"-x264-params"},
	X265Params: []string{"-x265-params"},
}

// TimingArgs contains timing and synchronization arguments
var TimingArgs = struct {
	// Force framerate
	Framerate []string
	// Input framerate
	InputFramerate []string
	// Video sync method
	VideoSync []string
	// Audio sync method
	AudioSync []string
	// Timestamp offset
	TimestampOffset []string
}{
	Framerate:       []string{"-r"},
	InputFramerate:  []string{"-framerate"},
	VideoSync:       []string{"-vsync"},
	AudioSync:       []string{"-async"},
	TimestampOffset: []string{"-itsoffset"},
}

// BufferingArgs contains buffering and performance arguments
var BufferingArgs = struct {
	// Maximum muxing queue size
	MaxMuxingQueue []string
	// Maximum delay
	MaxDelay []string
	// Buffer size
	BufferSize []string
	// Read timeout
	ReadTimeout []string
}{
	MaxMuxingQueue: []string{"-max_muxing_queue_size"},
	MaxDelay:       []string{"-max_delay"},
	BufferSize:     []string{"-bufsize"},
	ReadTimeout:    []string{"-rw_timeout"},
}

// ValidateArgs performs basic validation on FFmpeg arguments
func ValidateArgs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no arguments provided")
	}
	
	// Check for required components
	hasInput := false
	hasOutput := false
	
	for i, arg := range args {
		if arg == "-i" && i+1 < len(args) {
			hasInput = true
		}
		// Output is typically the last argument
		if i == len(args)-1 && !strings.HasPrefix(arg, "-") {
			hasOutput = true
		}
	}
	
	if !hasInput {
		return fmt.Errorf("no input file specified")
	}
	if !hasOutput {
		return fmt.Errorf("no output file specified")
	}
	
	return nil
}