// Package ffmpeg provides comprehensive FFmpeg argument reference
package ffmpeg

// FFmpegArgsReference documents all FFmpeg arguments used in the transcoding system
// with detailed explanations of their purpose and effects
type FFmpegArgsReference struct{}

// GlobalArgs are FFmpeg options that appear before input specification
var GlobalArgs = struct {
	Overwrite      []string // -y: Overwrite output files without prompting
	NoOverwrite    []string // -n: Never overwrite output files
	HideBanner     []string // -hide_banner: Suppress printing banner (version, config, etc.)
	LogLevel       []string // -loglevel [level]: Set logging verbosity (quiet, panic, fatal, error, warning, info, verbose, debug)
	Stats          []string // -stats: Print encoding progress/statistics
	StatsPeriod    []string // -stats_period [seconds]: Set period between progress updates
	Progress       []string // -progress [url]: Send progress info to URL/file
	NoStats        []string // -nostats: Disable printing progress during encoding
	Threads        []string // -threads [count]: Thread count (0=auto)
	FilterThreads  []string // -filter_threads [count]: Number of threads for filters
	ThreadQueue    []string // -thread_queue_size [size]: Frame queue size between threads
}{
	Overwrite:      []string{"-y"},
	NoOverwrite:    []string{"-n"},
	HideBanner:     []string{"-hide_banner"},
	LogLevel:       []string{"-loglevel"},
	Stats:          []string{"-stats"},
	StatsPeriod:    []string{"-stats_period"},
	Progress:       []string{"-progress"},
	NoStats:        []string{"-nostats"},
	Threads:        []string{"-threads"},
	FilterThreads:  []string{"-filter_threads"},
	ThreadQueue:    []string{"-thread_queue_size"},
}

// InputArgs are options that affect input file reading
var InputArgs = struct {
	Input          []string // -i [file]: Input file path
	Format         []string // -f [format]: Force input format
	SeekStart      []string // -ss [time]: Seek to position before reading input
	Duration       []string // -t [duration]: Limit input duration
	To             []string // -to [time]: Stop reading at position
	FrameRate      []string // -r [fps]: Set input frame rate
	ReInit         []string // -re: Read input at native frame rate
	Loop           []string // -stream_loop [count]: Loop input stream
	ThreadQueue    []string // -thread_queue_size [size]: Packet queue for demuxer
	Hwaccel        []string // -hwaccel [type]: Hardware acceleration method
	HwaccelDevice  []string // -hwaccel_device [device]: Hardware device to use
}{
	Input:          []string{"-i"},
	Format:         []string{"-f"},
	SeekStart:      []string{"-ss"},
	Duration:       []string{"-t"},
	To:             []string{"-to"},
	FrameRate:      []string{"-r"},
	ReInit:         []string{"-re"},
	Loop:           []string{"-stream_loop"},
	ThreadQueue:    []string{"-thread_queue_size"},
	Hwaccel:        []string{"-hwaccel"},
	HwaccelDevice:  []string{"-hwaccel_device"},
}

// VideoEncodingArgs are options for video encoding
var VideoEncodingArgs = struct {
	Codec          []string // -c:v [codec]: Video codec (libx264, libx265, etc.)
	Bitrate        []string // -b:v [bitrate]: Target video bitrate (e.g., 5M, 5000k)
	MinRate        []string // -minrate [bitrate]: Minimum bitrate for VBV
	MaxRate        []string // -maxrate [bitrate]: Maximum bitrate for VBV
	BufSize        []string // -bufsize [size]: VBV buffer size
	CRF            []string // -crf [value]: Constant Rate Factor (0-51, lower=better)
	QP             []string // -qp [value]: Constant Quantization Parameter
	Preset         []string // -preset [name]: Encoding preset (ultrafast to veryslow)
	Tune           []string // -tune [type]: Tune for specific content (film, animation, etc.)
	Profile        []string // -profile:v [name]: H.264/H.265 profile (baseline, main, high)
	Level          []string // -level [value]: H.264/H.265 level (3.0, 4.0, 4.1, etc.)
	PixFmt         []string // -pix_fmt [format]: Pixel format (yuv420p, yuv422p, etc.)
	ColorSpace     []string // -colorspace [space]: Color space
	ColorTrc       []string // -color_trc [trc]: Color transfer characteristics
	ColorPrimaries []string // -color_primaries [primaries]: Color primaries
	ColorRange     []string // -color_range [range]: Color range (tv, pc)
	
	// Keyframe/GOP settings
	KeyInt         []string // -g [frames]: GOP size (keyframe interval)
	KeyIntMin      []string // -keyint_min [frames]: Minimum GOP size
	ScThreshold    []string // -sc_threshold [value]: Scene change threshold (0=disabled)
	BFrames        []string // -bf [count]: Maximum B-frames between references
	BStrategy      []string // -b_strategy [strategy]: B-frame placement strategy
	RefFrames      []string // -refs [count]: Reference frames for P-frames
	
	// Rate control
	RcLookahead    []string // -rc-lookahead [frames]: Frames to look ahead for rate control
	AQ             []string // -aq-mode [mode]: Adaptive quantization mode
	AQStrength     []string // -aq-strength [strength]: AQ strength
	
	// Motion estimation
	ME             []string // -me_method [method]: Motion estimation method
	MERange        []string // -me_range [range]: Motion estimation search range
	SubQ           []string // -subq [quality]: Subpixel motion estimation quality
	
	// Filters
	VF             []string // -vf [filtergraph]: Video filter graph
	Scale          []string // scale=[w]:[h]: Scaling filter
	Fps            []string // fps=[fps]: Frame rate conversion filter
	
	// x264/x265 specific
	X264Params     []string // -x264-params [params]: Additional x264 parameters
	X265Params     []string // -x265-params [params]: Additional x265 parameters
}{
	Codec:          []string{"-c:v"},
	Bitrate:        []string{"-b:v"},
	MinRate:        []string{"-minrate"},
	MaxRate:        []string{"-maxrate"},
	BufSize:        []string{"-bufsize"},
	CRF:            []string{"-crf"},
	QP:             []string{"-qp"},
	Preset:         []string{"-preset"},
	Tune:           []string{"-tune"},
	Profile:        []string{"-profile:v"},
	Level:          []string{"-level"},
	PixFmt:         []string{"-pix_fmt"},
	ColorSpace:     []string{"-colorspace"},
	ColorTrc:       []string{"-color_trc"},
	ColorPrimaries: []string{"-color_primaries"},
	ColorRange:     []string{"-color_range"},
	KeyInt:         []string{"-g"},
	KeyIntMin:      []string{"-keyint_min"},
	ScThreshold:    []string{"-sc_threshold"},
	BFrames:        []string{"-bf"},
	BStrategy:      []string{"-b_strategy"},
	RefFrames:      []string{"-refs"},
	RcLookahead:    []string{"-rc-lookahead"},
	AQ:             []string{"-aq-mode"},
	AQStrength:     []string{"-aq-strength"},
	ME:             []string{"-me_method"},
	MERange:        []string{"-me_range"},
	SubQ:           []string{"-subq"},
	VF:             []string{"-vf"},
	Scale:          []string{"scale="},
	Fps:            []string{"fps="},
	X264Params:     []string{"-x264-params"},
	X265Params:     []string{"-x265-params"},
}

// AudioEncodingArgs are options for audio encoding
var AudioEncodingArgs = struct {
	Codec          []string // -c:a [codec]: Audio codec (aac, mp3, opus, etc.)
	Bitrate        []string // -b:a [bitrate]: Audio bitrate (128k, 192k, etc.)
	Quality        []string // -q:a [quality]: Audio quality (codec-specific)
	SampleRate     []string // -ar [rate]: Audio sample rate (44100, 48000, etc.)
	Channels       []string // -ac [count]: Number of audio channels
	ChannelLayout  []string // -channel_layout [layout]: Channel layout (stereo, 5.1, etc.)
	Volume         []string // -vol [volume]: Change audio volume
	AF             []string // -af [filtergraph]: Audio filter graph
	
	// AAC specific
	AACProfile     []string // -profile:a [profile]: AAC profile (aac_low, aac_he, etc.)
	AACCoder       []string // -aac_coder [coder]: AAC coder (twoloop, anmr, etc.)
	
	// Normalization
	Loudnorm       []string // loudnorm=: EBU R128 loudness normalization filter
	Dynaudnorm     []string // dynaudnorm=: Dynamic audio normalizer filter
}{
	Codec:          []string{"-c:a"},
	Bitrate:        []string{"-b:a"},
	Quality:        []string{"-q:a"},
	SampleRate:     []string{"-ar"},
	Channels:       []string{"-ac"},
	ChannelLayout:  []string{"-channel_layout"},
	Volume:         []string{"-vol"},
	AF:             []string{"-af"},
	AACProfile:     []string{"-profile:a"},
	AACCoder:       []string{"-aac_coder"},
	Loudnorm:       []string{"loudnorm="},
	Dynaudnorm:     []string{"dynaudnorm="},
}

// StreamMappingArgs control stream selection and mapping
var StreamMappingArgs = struct {
	Map            []string // -map [input]:[stream]: Map input streams to output
	MapAll         []string // -map 0: Map all streams from first input
	MapVideo       []string // -map 0:v: Map all video streams
	MapAudio       []string // -map 0:a: Map all audio streams
	MapSubtitle    []string // -map 0:s: Map all subtitle streams
	MapMetadata    []string // -map_metadata [spec]: Map metadata
	NoVideo        []string // -vn: Disable video
	NoAudio        []string // -an: Disable audio
	NoSubtitle     []string // -sn: Disable subtitles
	NoData         []string // -dn: Disable data streams
}{
	Map:            []string{"-map"},
	MapAll:         []string{"-map", "0"},
	MapVideo:       []string{"-map", "0:v"},
	MapAudio:       []string{"-map", "0:a"},
	MapSubtitle:    []string{"-map", "0:s"},
	MapMetadata:    []string{"-map_metadata"},
	NoVideo:        []string{"-vn"},
	NoAudio:        []string{"-an"},
	NoSubtitle:     []string{"-sn"},
	NoData:         []string{"-dn"},
}

// ContainerOptions are format/muxer specific options
var ContainerOptions = struct {
	Format         []string // -f [format]: Output format (mp4, dash, hls, etc.)
	MovFlags       []string // -movflags [flags]: MOV/MP4 muxer flags
	
	// DASH specific
	SegDuration    []string // -seg_duration [seconds]: Segment duration
	UseTimeline    []string // -use_timeline [0|1]: Use SegmentTimeline in manifest
	UseTemplate    []string // -use_template [0|1]: Use SegmentTemplate in manifest
	SingleFile     []string // -single_file [0|1]: Store segments in single file
	InitSegName    []string // -init_seg_name [pattern]: Init segment name pattern
	MediaSegName   []string // -media_seg_name [pattern]: Media segment name pattern
	AdaptationSets []string // -adaptation_sets [sets]: Define adaptation sets
	WindowSize     []string // -window_size [size]: Number of segments in manifest
	ExtraWindow    []string // -extra_window_size [size]: Extra segments to keep
	MinSegDuration []string // -min_seg_duration [microseconds]: Minimum segment duration
	RemoveAtExit   []string // -remove_at_exit [0|1]: Remove segments on exit
	DashSegType    []string // -dash_segment_type [type]: Segment type (mp4, webm)
	LowLatency     []string // -ldash [0|1]: Low latency DASH mode
	Streaming      []string // -streaming [0|1]: Enable streaming mode
	
	// HLS specific
	HLSTime        []string // -hls_time [seconds]: Target segment duration
	HLSListSize    []string // -hls_list_size [size]: Maximum playlist entries
	HLSSegFilename []string // -hls_segment_filename [pattern]: Segment filename pattern
	HLSSegType     []string // -hls_segment_type [type]: Segment type (mpegts, fmp4)
	HLSFlags       []string // -hls_flags [flags]: HLS muxer flags
	HLSPlaylistType []string // -hls_playlist_type [type]: Playlist type (event, vod)
	
	// MP4 specific
	FastStart      []string // +faststart: Move index to beginning
	FragKeyframe   []string // frag_keyframe: Fragment at keyframes
	EmptyMoov      []string // empty_moov: No initial moov atom
	DelayMoov      []string // delay_moov: Delay writing moov
	DefaultBase    []string // default_base_moof: Default base in moof
	
	// Metadata
	Metadata       []string // -metadata [key=value]: Set metadata
	Title          []string // -metadata title=[title]: Set title
	Language       []string // -metadata:s:a:0 language=[lang]: Set stream language
}{
	Format:         []string{"-f"},
	MovFlags:       []string{"-movflags"},
	SegDuration:    []string{"-seg_duration"},
	UseTimeline:    []string{"-use_timeline"},
	UseTemplate:    []string{"-use_template"},
	SingleFile:     []string{"-single_file"},
	InitSegName:    []string{"-init_seg_name"},
	MediaSegName:   []string{"-media_seg_name"},
	AdaptationSets: []string{"-adaptation_sets"},
	WindowSize:     []string{"-window_size"},
	ExtraWindow:    []string{"-extra_window_size"},
	MinSegDuration: []string{"-min_seg_duration"},
	RemoveAtExit:   []string{"-remove_at_exit"},
	DashSegType:    []string{"-dash_segment_type"},
	LowLatency:     []string{"-ldash"},
	Streaming:      []string{"-streaming"},
	HLSTime:        []string{"-hls_time"},
	HLSListSize:    []string{"-hls_list_size"},
	HLSSegFilename: []string{"-hls_segment_filename"},
	HLSSegType:     []string{"-hls_segment_type"},
	HLSFlags:       []string{"-hls_flags"},
	HLSPlaylistType: []string{"-hls_playlist_type"},
	FastStart:      []string{"+faststart"},
	FragKeyframe:   []string{"frag_keyframe"},
	EmptyMoov:      []string{"empty_moov"},
	DelayMoov:      []string{"delay_moov"},
	DefaultBase:    []string{"default_base_moof"},
	Metadata:       []string{"-metadata"},
	Title:          []string{"-metadata", "title="},
	Language:       []string{"-metadata:s:a:0", "language="},
}

// HardwareArgs are hardware acceleration options
var HardwareArgs = struct {
	// Generic hardware options
	Hwaccel        []string // -hwaccel [method]: Hardware acceleration method
	HwaccelDevice  []string // -hwaccel_device [device]: Device to use
	HwaccelOutput  []string // -hwaccel_output_format [format]: Output pixel format
	
	// NVIDIA specific
	CudaDevice     []string // -gpu [index]: CUDA device index
	CuvidDecoder   []string // h264_cuvid: NVIDIA hardware decoder
	NvencEncoder   []string // h264_nvenc: NVIDIA hardware encoder
	
	// Intel Quick Sync
	QSVDevice      []string // -qsv_device [device]: QSV device
	QSVDecoder     []string // h264_qsv: Intel QSV decoder
	QSVEncoder     []string // h264_qsv: Intel QSV encoder
	
	// AMD
	AMFEncoder     []string // h264_amf: AMD AMF encoder
	
	// Apple VideoToolbox
	VTDecoder      []string // h264_videotoolbox: Apple VT decoder
	VTEncoder      []string // h264_videotoolbox: Apple VT encoder
}{
	Hwaccel:        []string{"-hwaccel"},
	HwaccelDevice:  []string{"-hwaccel_device"},
	HwaccelOutput:  []string{"-hwaccel_output_format"},
	CudaDevice:     []string{"-gpu"},
	CuvidDecoder:   []string{"h264_cuvid"},
	NvencEncoder:   []string{"h264_nvenc"},
	QSVDevice:      []string{"-qsv_device"},
	QSVDecoder:     []string{"h264_qsv"},
	QSVEncoder:     []string{"h264_qsv"},
	AMFEncoder:     []string{"h264_amf"},
	VTDecoder:      []string{"h264_videotoolbox"},
	VTEncoder:      []string{"h264_videotoolbox"},
}