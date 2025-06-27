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
}

// PlaybackDecision represents the decision made by the planner
type PlaybackDecision struct {
	ShouldTranscode bool        `json:"should_transcode"`
	DirectPlayURL   string      `json:"direct_play_url,omitempty"`
	TranscodeParams interface{} `json:"transcode_params,omitempty"` // Using interface{} to avoid circular imports
	Reason          string      `json:"reason"`
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
	Path         string   `json:"path"`
	Container    string   `json:"container"`
	VideoCodec   string   `json:"video_codec"`
	AudioCodec   string   `json:"audio_codec"`
	Resolution   string   `json:"resolution"`
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	Bitrate      int64    `json:"bitrate"`
	Duration     float64  `json:"duration"`
	FrameRate    float64  `json:"frame_rate"`
	HasHDR       bool     `json:"has_hdr"`
	HasSubtitles bool     `json:"has_subtitles"`
	AudioStreams []string `json:"audio_streams"`
	FileSize     int64    `json:"file_size"`
}
