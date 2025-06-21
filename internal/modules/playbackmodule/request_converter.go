package playbackmodule

import (
	"github.com/hashicorp/go-hclog"
	plugins "github.com/mantonx/viewra/sdk"
)

// RequestConverter handles validation of transcode requests
type RequestConverter struct {
	logger hclog.Logger
}

// NewRequestConverter creates a new request converter
func NewRequestConverter(logger hclog.Logger) *RequestConverter {
	return &RequestConverter{
		logger: logger,
	}
}

// ValidateRequest validates and normalizes a transcode request
func (rc *RequestConverter) ValidateRequest(req *plugins.TranscodeRequest) *plugins.TranscodeRequest {
	// Set defaults if not provided
	if req.Container == "" {
		req.Container = "mp4"
	}
	if req.VideoCodec == "" {
		req.VideoCodec = "h264"
	}
	if req.AudioCodec == "" {
		req.AudioCodec = "aac"
	}
	if req.Quality == 0 {
		req.Quality = 50 // Default to medium quality
	}
	if req.SpeedPriority == "" {
		req.SpeedPriority = plugins.SpeedPriorityBalanced
	}

	// Clamp quality to valid range
	if req.Quality < 0 {
		req.Quality = 0
	} else if req.Quality > 100 {
		req.Quality = 100
	}

	// Validate speed priority
	switch req.SpeedPriority {
	case plugins.SpeedPriorityFastest, plugins.SpeedPriorityBalanced, plugins.SpeedPriorityQuality:
		// Valid
	default:
		req.SpeedPriority = plugins.SpeedPriorityBalanced
	}

	// Set hardware type if prefer hardware is true but type not specified
	if req.PreferHardware && req.HardwareType == "" {
		req.HardwareType = plugins.HardwareTypeNone // Will auto-detect
	}

	rc.logger.Debug("validated transcode request",
		"quality", req.Quality,
		"speed_priority", req.SpeedPriority,
		"container", req.Container,
		"video_codec", req.VideoCodec,
		"prefer_hardware", req.PreferHardware,
		"hardware_type", req.HardwareType,
	)

	return req
}
