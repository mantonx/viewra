// Package types provides types and interfaces for the transcoding module.
package types

import "time"

// SpeedPriority represents encoding speed vs quality tradeoff
type SpeedPriority int

const (
	SpeedPriorityBalanced SpeedPriority = iota
	SpeedPriorityQuality
	SpeedPriorityFastest
)

// Resolution represents video dimensions
type Resolution struct {
	Width  int
	Height int
}

// TranscodeRequest contains the parameters for a transcoding request
type TranscodeRequest struct {
	MediaID          string
	SessionID        string
	InputPath        string
	OutputPath       string
	Container        string
	VideoCodec       string
	AudioCodec       string
	Resolution       *Resolution
	Quality          int
	SpeedPriority    SpeedPriority
	Seek             time.Duration
	Duration         time.Duration
	EnableABR        bool
	PreferHardware   bool
	ProviderSettings []byte
	VideoBitrate     int
	AudioBitrate     int
}

// EncodeRequest represents a request to encode video to intermediate format
type EncodeRequest struct {
	InputPath    string
	OutputPath   string
	VideoCodec   string
	AudioCodec   string
	Quality      int
	Resolution   *Resolution
	Container    string
	VideoBitrate int
	AudioBitrate int
}