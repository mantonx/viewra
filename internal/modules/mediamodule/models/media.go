// Package models provides database models for the media module
//
// IMPORTANT: These models are NOT currently in use. The media module is still
// using shared database models from /internal/database/ to avoid conflicts.
//
// This package contains the target models for future migration when:
// 1. A comprehensive service layer is implemented across all modules
// 2. All modules are updated to use services instead of direct model access
// 3. Plugin system is updated to work with service interfaces
// 4. Database migration is performed to handle ID type changes (string → uint)
//
// TODO: Complete migration to these module-specific models
// TODO: Remove duplicate models from /internal/database/
//
package models

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// MediaType enum for media_files.media_type and related fields
type MediaType string

const (
	MediaTypeMovie   MediaType = "movie"
	MediaTypeEpisode MediaType = "episode"
	MediaTypeTrack   MediaType = "track"
	MediaTypeImage   MediaType = "image"
)

func (mt MediaType) Value() (driver.Value, error) {
	return string(mt), nil
}

func (mt *MediaType) Scan(value interface{}) error {
	if value == nil {
		*mt = ""
		return nil
	}
	switch s := value.(type) {
	case string:
		*mt = MediaType(s)
	case []byte:
		*mt = MediaType(s)
	default:
		return fmt.Errorf("cannot scan %T into MediaType", value)
	}
	return nil
}

// MediaLibrary represents a directory to scan for media files
type MediaLibrary struct {
	ID        uint32    `gorm:"primaryKey" json:"id"`
	Path      string    `gorm:"not null" json:"path"`
	Type      string    `gorm:"not null" json:"type"` // "movie", "tv", "music"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MediaLibraryRequest represents the request to create a new media library
type MediaLibraryRequest struct {
	Path string `json:"path" binding:"required"`
	Type string `json:"type" binding:"required,oneof=movie tv music"`
}

// MediaFile represents each file version of a media item
type MediaFile struct {
	ID          string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	MediaID     string    `gorm:"type:varchar(36);not null;index" json:"media_id"` // FK to movie, episode, or track
	MediaType   MediaType `gorm:"type:text;not null;index" json:"media_type"`      // ENUM: movie, episode, track
	LibraryID   uint32    `gorm:"not null;index" json:"library_id"`                // FK to MediaLibrary
	ScanJobID   *uint32   `gorm:"index" json:"scan_job_id,omitempty"`              // Track which job discovered this file
	Path        string    `gorm:"not null;uniqueIndex" json:"path"`                // Absolute or relative file path
	Container   string    `json:"container"`                                       // e.g., mkv, mp4, flac
	VideoCodec  string    `json:"video_codec"`                                     // Optional: h264, vp9, etc.
	AudioCodec  string    `json:"audio_codec"`                                     // Optional: aac, flac, dts
	Channels    string    `json:"channels"`                                        // e.g., 2.0, 5.1, 7.1
	SampleRate  int       `json:"sample_rate"`                                     // Audio sample rate in Hz
	Resolution  string    `json:"resolution"`                                      // e.g., 1080p, 4K
	Duration    int       `json:"duration"`                                        // In seconds
	SizeBytes   int64     `gorm:"not null" json:"size_bytes"`                      // File size
	BitrateKbps int       `json:"bitrate_kbps"`                                    // Total bitrate estimate
	Language    string    `json:"language"`                                        // Default language
	Hash        string    `gorm:"index" json:"hash"`                               // SHA256 for deduplication
	VersionName string    `json:"version_name"`                                    // e.g., "Director's Cut"

	// Comprehensive technical metadata as JSON (from FFmpeg probe)
	TechnicalInfo   string `gorm:"type:text" json:"technical_info"`   // Complete technical info as JSON
	VideoStreams    string `gorm:"type:text" json:"video_streams"`    // VideoStreamInfo array as JSON
	AudioStreams    string `gorm:"type:text" json:"audio_streams"`    // AudioStreamInfo array as JSON
	SubtitleStreams string `gorm:"type:text" json:"subtitle_streams"` // SubtitleStreamInfo array as JSON

	// Enhanced technical fields for easier querying
	VideoWidth      int    `json:"video_width"`      // Video width in pixels
	VideoHeight     int    `json:"video_height"`     // Video height in pixels
	VideoFramerate  string `json:"video_framerate"`  // Video framerate (e.g., "23.976")
	VideoProfile    string `json:"video_profile"`    // Video codec profile (e.g., "Main 10")
	VideoLevel      int    `json:"video_level"`      // Video codec level
	VideoBitDepth   string `json:"video_bit_depth"`  // Video bit depth
	AspectRatio     string `json:"aspect_ratio"`     // Display aspect ratio
	PixelFormat     string `json:"pixel_format"`     // Pixel format (e.g., "yuv420p10le")
	ColorSpace      string `json:"color_space"`      // Color space (e.g., "bt709")
	ColorPrimaries  string `json:"color_primaries"`  // Color primaries
	ColorTransfer   string `json:"color_transfer"`   // Color transfer
	HDRFormat       string `json:"hdr_format"`       // HDR format (HDR10, DV, etc.)
	Interlaced      string `json:"interlaced"`       // Interlaced status
	ReferenceFrames int    `json:"reference_frames"` // Number of reference frames

	// Enhanced audio fields
	AudioChannels   int    `json:"audio_channels"`    // Number of audio channels
	AudioLayout     string `json:"audio_layout"`      // Audio channel layout
	AudioSampleRate int    `json:"audio_sample_rate"` // Audio sample rate
	AudioBitDepth   int    `json:"audio_bit_depth"`   // Audio bit depth
	AudioLanguage   string `json:"audio_language"`    // Primary audio language
	AudioProfile    string `json:"audio_profile"`     // Audio codec profile

	LastSeen  time.Time `gorm:"not null" json:"last_seen"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}