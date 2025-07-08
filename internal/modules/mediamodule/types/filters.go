// Package types - Filter types
package types

// PlaybackMethod represents how a media file can be played
type PlaybackMethod string

const (
	// PlaybackMethodDirect means the file can be played directly without any processing
	PlaybackMethodDirect PlaybackMethod = "direct"
	
	// PlaybackMethodRemux means the file needs container change but codecs are compatible
	PlaybackMethodRemux PlaybackMethod = "remux"
	
	// PlaybackMethodTranscode means the file needs full transcoding
	PlaybackMethodTranscode PlaybackMethod = "transcode"
)

// MediaQuery represents a query for media files with advanced filtering
type MediaQuery struct {
	// Basic filters
	LibraryIDs []uint32 `json:"library_ids,omitempty"`
	MediaTypes []string `json:"media_types,omitempty"`
	Search     string   `json:"search,omitempty"`
	
	// Format filters
	Containers   []string `json:"containers,omitempty"`
	VideoCodecs  []string `json:"video_codecs,omitempty"`
	AudioCodecs  []string `json:"audio_codecs,omitempty"`
	Resolutions  []string `json:"resolutions,omitempty"`
	
	// Playback filters
	PlaybackMethods []PlaybackMethod `json:"playback_methods,omitempty"`
	CanDirectPlay   *bool           `json:"can_direct_play,omitempty"`
	
	// Quality filters
	MinBitrate   int64 `json:"min_bitrate,omitempty"`
	MaxBitrate   int64 `json:"max_bitrate,omitempty"`
	MinDuration  int   `json:"min_duration,omitempty"`
	MaxDuration  int   `json:"max_duration,omitempty"`
	
	// Pagination
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
	
	// Sorting
	SortBy    string `json:"sort_by,omitempty"`
	SortOrder string `json:"sort_order,omitempty"` // asc, desc
}

// MediaQueryResult represents the result of a media query
type MediaQueryResult struct {
	Files      []MediaFileInfo `json:"files"`
	TotalCount int64          `json:"total_count"`
	HasMore    bool           `json:"has_more"`
}