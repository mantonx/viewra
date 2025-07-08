// Package types contains type definitions for the playback module.
package types

// DeviceAnalytics contains detailed device and session analytics information
// collected from the client for dashboard and debugging purposes.
type DeviceAnalytics struct {
	// Network information
	IPAddress string `json:"ip_address"`
	Location  string `json:"location"` // City, Country format

	// Device information
	UserAgent  string `json:"user_agent"`
	DeviceName string `json:"device_name"` // e.g., "Chrome on Windows"
	DeviceType string `json:"device_type"` // desktop, mobile, tablet, tv
	Browser    string `json:"browser"`     // Chrome, Firefox, Safari, etc.
	OS         string `json:"os"`          // Windows, macOS, Linux, iOS, Android

	// Capabilities
	Capabilities map[string]bool `json:"capabilities"` // codec support, DRM, etc.

	// Playback quality information
	QualityPlayed string `json:"quality_played"` // 1080p, 720p, etc.
	Bandwidth     int64  `json:"bandwidth"`      // Estimated bandwidth in bps

	// Debug information
	DebugInfo map[string]interface{} `json:"debug_info"`
}

// SessionStartRequest contains all information needed to start a playback session
type SessionStartRequest struct {
	MediaFileID string           `json:"media_file_id" binding:"required"`
	UserID      string           `json:"user_id,omitempty"`
	DeviceID    string           `json:"device_id" binding:"required"`
	Method      string           `json:"method" binding:"required,oneof=direct remux transcode"`
	Analytics   *DeviceAnalytics `json:"analytics,omitempty"`
}

// SessionUpdateRequest contains fields that can be updated in a session
type SessionUpdateRequest struct {
	Position      *int64                 `json:"position,omitempty"`
	State         *string                `json:"state,omitempty" binding:"omitempty,oneof=playing paused stopped"`
	QualityPlayed *string                `json:"quality_played,omitempty"`
	Bandwidth     *int64                 `json:"bandwidth,omitempty"`
	DebugInfo     map[string]interface{} `json:"debug_info,omitempty"`
}
