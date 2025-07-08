// Package types contains type definitions for the playback module.
package types

import (
	"github.com/mantonx/viewra/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
)

// PlaybackDecision represents a detailed playback decision.
// It extends the base types.PlaybackDecision with additional fields needed
// for the playback module's decision engine.
type PlaybackDecision struct {
	Method          types.PlaybackMethod      `json:"method"` // direct, remux, transcode
	MediaPath       string                    `json:"media_path"`
	Reason          string                    `json:"reason"`
	MediaInfo       *types.MediaInfo          `json:"media_info"`
	DeviceInfo      *DeviceProfile            `json:"device_info"`
	TranscodeParams *plugins.TranscodeRequest `json:"transcode_params,omitempty"`
	DirectPlayURL   string                    `json:"direct_play_url,omitempty"`
}

// DeviceProfile extends the base types.DeviceProfile with additional fields
type DeviceProfile struct {
	*types.DeviceProfile
	Name                 string   `json:"name"`                   // Device name for logging
	PreferredContainer   string   `json:"preferred_container"`    // Preferred container format
	PreferredVideoCodec  string   `json:"preferred_video_codec"`  // Preferred video codec
	PreferredAudioCodec  string   `json:"preferred_audio_codec"`  // Preferred audio codec
	SupportedContainers  []string `json:"supported_containers"`   // List of supported containers
	SupportedVideoCodecs []string `json:"supported_video_codecs"` // List of supported video codecs
	SupportedAudioCodecs []string `json:"supported_audio_codecs"` // List of supported audio codecs
}
