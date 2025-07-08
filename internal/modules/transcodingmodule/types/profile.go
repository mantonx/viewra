// Package types provides types and interfaces for the transcoding module.
package types

// EncodingProfile represents an encoding configuration
type EncodingProfile struct {
	Name         string `json:"name"`
	VideoCodec   string `json:"video_codec"`
	AudioCodec   string `json:"audio_codec"`
	VideoBitrate int    `json:"video_bitrate"` // in kbps
	AudioBitrate int    `json:"audio_bitrate"` // in kbps
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}