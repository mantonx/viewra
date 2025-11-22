package transcoding

import (
	"fmt"

	"github.com/mantonx/viewra/internal/domain/transcode"
)

// QualityProfile defines the encoding parameters for a specific quality level.
type QualityProfile struct {
	// Quality identifier (360p, 720p, 1080p, 4k)
	Quality string

	// Video parameters
	Width        int    // Target video width in pixels
	Height       int    // Target video height in pixels
	VideoBitrate string // Video bitrate (e.g., "800k", "2500k")
	VideoMaxRate string // Maximum video bitrate for buffer control
	VideoBufSize string // Video buffer size

	// Audio parameters
	AudioBitrate    string // Audio bitrate (e.g., "96k", "128k")
	AudioChannels   int    // Number of audio channels (1=mono, 2=stereo)
	AudioSampleRate int    // Audio sample rate in Hz (typically 44100 or 48000)

	// DASH parameters
	SegmentDuration int // Segment duration in seconds
	GOPSize         int // Group of Pictures size (keyframe interval)
}

// GetQualityProfile returns the encoding profile for a given quality level.
// Returns an error if the quality level is not supported.
func GetQualityProfile(quality string) (*QualityProfile, error) {
	profile, exists := qualityProfiles[quality]
	if !exists {
		return nil, fmt.Errorf("%w: %s", transcode.ErrInvalidQuality, quality)
	}
	return profile, nil
}

// GetAllProfiles returns all available quality profiles.
func GetAllProfiles() []*QualityProfile {
	profiles := make([]*QualityProfile, 0, len(qualityProfiles))
	for _, profile := range qualityProfiles {
		profiles = append(profiles, profile)
	}
	return profiles
}

// qualityProfiles defines the encoding parameters for each quality level.
// These profiles are optimized for DASH streaming with H.264 video and AAC audio.
var qualityProfiles = map[string]*QualityProfile{
	transcode.Quality360p: {
		Quality:         transcode.Quality360p,
		Width:           640,
		Height:          360,
		VideoBitrate:    "800k",
		VideoMaxRate:    "880k",  // 110% of target bitrate
		VideoBufSize:    "1600k", // 2x target bitrate
		AudioBitrate:    "96k",
		AudioChannels:   2,
		AudioSampleRate: 44100,
		SegmentDuration: 2,  // 2-second segments for faster progressive streaming startup
		GOPSize:         48, // 2 seconds at 24fps, aligns with segment boundaries
	},
	transcode.Quality720p: {
		Quality:         transcode.Quality720p,
		Width:           1280,
		Height:          720,
		VideoBitrate:    "2500k",
		VideoMaxRate:    "2750k",
		VideoBufSize:    "5000k",
		AudioBitrate:    "128k",
		AudioChannels:   2,
		AudioSampleRate: 48000,
		SegmentDuration: 2, // 2-second segments for faster progressive streaming startup
		GOPSize:         48,
	},
	transcode.Quality1080p: {
		Quality:         transcode.Quality1080p,
		Width:           1920,
		Height:          1080,
		VideoBitrate:    "5000k",
		VideoMaxRate:    "5500k",
		VideoBufSize:    "10000k",
		AudioBitrate:    "192k",
		AudioChannels:   2,
		AudioSampleRate: 48000,
		SegmentDuration: 2, // 2-second segments for faster progressive streaming startup
		GOPSize:         48,
	},
	transcode.Quality4K: {
		Quality:         transcode.Quality4K,
		Width:           3840,
		Height:          2160,
		VideoBitrate:    "15000k",
		VideoMaxRate:    "16500k",
		VideoBufSize:    "30000k",
		AudioBitrate:    "256k",
		AudioChannels:   2,
		AudioSampleRate: 48000,
		SegmentDuration: 2, // 2-second segments for faster progressive streaming startup
		GOPSize:         48,
	},
}

// IsQualitySupported checks if a quality level is supported.
func IsQualitySupported(quality string) bool {
	_, exists := qualityProfiles[quality]
	return exists
}

// GetSupportedQualities returns a list of all supported quality levels.
func GetSupportedQualities() []string {
	return transcode.GetAllQualities()
}
