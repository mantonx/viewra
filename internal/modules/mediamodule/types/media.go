// Package types provides type definitions for the media module
package types

import (
	"time"
)

// MediaFile represents extended information about a media file
// This extends the database model with additional computed fields
type MediaFileInfo struct {
	ID          string    `json:"id"`
	MediaID     string    `json:"media_id"`
	MediaType   string    `json:"media_type"`
	LibraryID   uint32    `json:"library_id"`
	Path        string    `json:"path"`
	Container   string    `json:"container"`
	VideoCodec  string    `json:"video_codec,omitempty"`
	AudioCodec  string    `json:"audio_codec,omitempty"`
	Resolution  string    `json:"resolution,omitempty"`
	Duration    int       `json:"duration,omitempty"`
	SizeBytes   int64     `json:"size_bytes"`
	BitrateKbps int       `json:"bitrate_kbps,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	
	// Computed fields
	PlaybackMethod string `json:"playback_method,omitempty"` // direct, remux, transcode
	CanDirectPlay  bool   `json:"can_direct_play"`
}

// MediaAnalysis represents the result of analyzing a media file
type MediaAnalysis struct {
	FileInfo      MediaFileInfo
	VideoStreams  []VideoStream
	AudioStreams  []AudioStream
	Subtitles     []SubtitleStream
	Chapters      []Chapter
	Metadata      map[string]string
}

// VideoStream represents video stream information
type VideoStream struct {
	Index       int    `json:"index"`
	Codec       string `json:"codec"`
	Profile     string `json:"profile,omitempty"`
	Level       string `json:"level,omitempty"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	FrameRate   string `json:"frame_rate"`
	BitRate     int64  `json:"bit_rate,omitempty"`
	PixelFormat string `json:"pixel_format,omitempty"`
	ColorSpace  string `json:"color_space,omitempty"`
	HDR         bool   `json:"hdr"`
}

// AudioStream represents audio stream information
type AudioStream struct {
	Index        int    `json:"index"`
	Codec        string `json:"codec"`
	Profile      string `json:"profile,omitempty"`
	Channels     int    `json:"channels"`
	ChannelLayout string `json:"channel_layout"`
	SampleRate   int    `json:"sample_rate"`
	BitRate      int64  `json:"bit_rate,omitempty"`
	Language     string `json:"language,omitempty"`
	Title        string `json:"title,omitempty"`
	Default      bool   `json:"default"`
}

// SubtitleStream represents subtitle stream information
type SubtitleStream struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
	Default  bool   `json:"default"`
	Forced   bool   `json:"forced"`
}

// Chapter represents chapter information
type Chapter struct {
	ID        int    `json:"id"`
	TimeBase  string `json:"time_base"`
	Start     int64  `json:"start"`
	StartTime string `json:"start_time"`
	End       int64  `json:"end"`
	EndTime   string `json:"end_time"`
	Title     string `json:"title,omitempty"`
}