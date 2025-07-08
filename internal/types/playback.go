package types

// DeviceProfile captures client playback capabilities
type DeviceProfile struct {
	UserAgent       string   `json:"user_agent"`
	SupportedCodecs []string `json:"supported_codecs"`
	MaxResolution   string   `json:"max_resolution"`
	MaxBitrate      int      `json:"max_bitrate"`
	SupportsHEVC    bool     `json:"supports_hevc"`
	SupportsAV1     bool     `json:"supports_av1"`
	SupportsHDR     bool     `json:"supports_hdr"`
	ClientIP        string   `json:"client_ip"`
	
	// New separated codec fields
	SupportedContainers  []string `json:"supported_containers"`
	SupportedVideoCodecs []string `json:"supported_video_codecs"`
	SupportedAudioCodecs []string `json:"supported_audio_codecs"`
}

// PlaybackMethod represents how media will be delivered
type PlaybackMethod string

const (
	PlaybackMethodDirect    PlaybackMethod = "direct"
	PlaybackMethodRemux     PlaybackMethod = "remux"
	PlaybackMethodTranscode PlaybackMethod = "transcode"
)

// PlaybackDecision represents the decision made by the planner
type PlaybackDecision struct {
	Method          PlaybackMethod `json:"method"`                      // Explicit method: direct, remux, or transcode
	DirectPlayURL   string         `json:"direct_play_url,omitempty"`
	TranscodeParams interface{}    `json:"transcode_params,omitempty"` // Using interface{} to avoid circular imports
	Reason          string         `json:"reason"`
}

// TranscodingStats represents overall transcoding statistics
type TranscodingStats struct {
	ActiveSessions    int                      `json:"active_sessions"`
	TotalSessions     int64                    `json:"total_sessions"`
	CompletedSessions int64                    `json:"completed_sessions"`
	FailedSessions    int64                    `json:"failed_sessions"`
	TotalBytesOut     int64                    `json:"total_bytes_out"`
	AverageSpeed      float64                  `json:"average_speed"`
	Backends          map[string]*BackendStats `json:"backends"`
	RecentSessions    interface{}              `json:"recent_sessions"` // Using interface{} to avoid circular imports
}

// BackendStats contains backend-specific statistics
type BackendStats struct {
	Name         string                 `json:"name"`
	Priority     int                    `json:"priority"`
	Capabilities map[string]interface{} `json:"capabilities"`
}

// MediaInfo represents analyzed media file information
type MediaInfo struct {
	Path         string           `json:"path"`
	Size         int64            `json:"size"`
	Duration     float64          `json:"duration"`     // in seconds
	Container    string           `json:"container"`
	VideoStreams []VideoStream    `json:"video_streams"`
	AudioStreams []AudioStream    `json:"audio_streams"`
	Subtitles    []SubtitleStream `json:"subtitles"`
	
	// Compatibility fields for existing code
	VideoCodec   string   `json:"video_codec"`
	AudioCodec   string   `json:"audio_codec"`
	Resolution   string   `json:"resolution"`
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	Bitrate      int64    `json:"bitrate"`
	FrameRate    float64  `json:"frame_rate"`
	HasHDR       bool     `json:"has_hdr"`
	HasSubtitles bool     `json:"has_subtitles"`
	FileSize     int64    `json:"file_size"`
}

// VideoStream contains video stream information
type VideoStream struct {
	Index       int    `json:"index"`
	Codec       string `json:"codec"`
	CodecLong   string `json:"codec_long"`
	Profile     string `json:"profile"`
	PixFmt      string `json:"pix_fmt"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Bitrate     int64  `json:"bitrate"`
	FrameRate   string `json:"frame_rate"`
	AspectRatio string `json:"aspect_ratio"`
}

// AudioStream contains audio stream information
type AudioStream struct {
	Index         int    `json:"index"`
	Codec         string `json:"codec"`
	CodecLong     string `json:"codec_long"`
	Channels      int    `json:"channels"`
	ChannelLayout string `json:"channel_layout"`
	SampleRate    int    `json:"sample_rate"`
	Bitrate       int64  `json:"bitrate"`
	Language      string `json:"language"`
	Title         string `json:"title"`
	Default       bool   `json:"default"`
}

// SubtitleStream contains subtitle stream information
type SubtitleStream struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Default  bool   `json:"default"`
	Forced   bool   `json:"forced"`
}
