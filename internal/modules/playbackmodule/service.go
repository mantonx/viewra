package playbackmodule

import (
	"fmt"

	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
)

// ServiceAdapter adapts the playback module to implement services.PlaybackService
type ServiceAdapter struct {
	module *Module
}

// NewServiceAdapter creates a new service adapter
func NewServiceAdapter(module *Module) *ServiceAdapter {
	return &ServiceAdapter{
		module: module,
	}
}

// DecidePlayback determines whether to direct play or transcode based on
// media file characteristics and device capabilities
func (s *ServiceAdapter) DecidePlayback(mediaPath string, deviceProfile *types.DeviceProfile) (*types.PlaybackDecision, error) {
	if s.module.manager == nil {
		return nil, fmt.Errorf("playback manager not initialized")
	}

	// Convert types.DeviceProfile to playbackmodule.DeviceProfile
	localProfile := &DeviceProfile{
		UserAgent:       deviceProfile.UserAgent,
		SupportedCodecs: deviceProfile.SupportedCodecs,
		MaxResolution:   deviceProfile.MaxResolution,
		MaxBitrate:      deviceProfile.MaxBitrate,
		SupportsHEVC:    deviceProfile.SupportsHEVC,
		SupportsAV1:     deviceProfile.SupportsAV1,
		SupportsHDR:     deviceProfile.SupportsHDR,
		ClientIP:        deviceProfile.ClientIP,
	}

	decision, err := s.module.manager.DecidePlayback(mediaPath, localProfile)
	if err != nil {
		return nil, err
	}

	// Convert playbackmodule.PlaybackDecision to types.PlaybackDecision
	return &types.PlaybackDecision{
		ShouldTranscode: decision.ShouldTranscode,
		DirectPlayURL:   decision.DirectPlayURL,
		TranscodeParams: decision.TranscodeParams,
		Reason:          decision.Reason,
	}, nil
}

// GetMediaInfo analyzes a media file and returns its characteristics
func (s *ServiceAdapter) GetMediaInfo(mediaPath string) (*types.MediaInfo, error) {
	if s.module.manager == nil {
		return nil, fmt.Errorf("playback manager not initialized")
	}

	analyzer := NewFFProbeMediaAnalyzer()
	info, err := analyzer.AnalyzeMedia(mediaPath)
	if err != nil {
		return nil, err
	}

	// Convert internal MediaInfo to types.MediaInfo
	return &types.MediaInfo{
		Path:         mediaPath,
		Container:    info.Container,
		VideoCodec:   info.VideoCodec,
		AudioCodec:   info.AudioCodec,
		Resolution:   info.Resolution,
		Bitrate:      info.Bitrate,
		Duration:     float64(info.Duration),
		HasHDR:       info.HasHDR,
		HasSubtitles: info.HasSubtitles,
	}, nil
}

// ValidatePlayback checks if a media file can be played on the given device
func (s *ServiceAdapter) ValidatePlayback(mediaPath string, deviceProfile *types.DeviceProfile) error {
	if s.module.manager == nil {
		return fmt.Errorf("playback manager not initialized")
	}

	// Convert types.DeviceProfile to playbackmodule.DeviceProfile
	localProfile := &DeviceProfile{
		UserAgent:       deviceProfile.UserAgent,
		SupportedCodecs: deviceProfile.SupportedCodecs,
		MaxResolution:   deviceProfile.MaxResolution,
		MaxBitrate:      deviceProfile.MaxBitrate,
		SupportsHEVC:    deviceProfile.SupportsHEVC,
		SupportsAV1:     deviceProfile.SupportsAV1,
		SupportsHDR:     deviceProfile.SupportsHDR,
		ClientIP:        deviceProfile.ClientIP,
	}

	decision, err := s.module.manager.DecidePlayback(mediaPath, localProfile)
	if err != nil {
		return err
	}

	// If transcode is required but we can't transcode, return error
	if decision.ShouldTranscode && decision.TranscodeParams == nil {
		return fmt.Errorf("media file requires transcoding but no transcoding parameters available")
	}

	return nil
}

// GetSupportedFormats returns formats supported for direct playback
func (s *ServiceAdapter) GetSupportedFormats(deviceProfile *types.DeviceProfile) []string {
	// Default supported formats based on device profile
	formats := []string{"mp4", "webm"}

	if deviceProfile.SupportsHEVC {
		formats = append(formats, "hevc")
	}

	if deviceProfile.SupportsAV1 {
		formats = append(formats, "av1")
	}

	return formats
}

// GetRecommendedTranscodeParams returns optimal transcoding parameters if needed
func (s *ServiceAdapter) GetRecommendedTranscodeParams(mediaPath string, deviceProfile *types.DeviceProfile) (*plugins.TranscodeRequest, error) {
	if s.module.manager == nil {
		return nil, fmt.Errorf("playback manager not initialized")
	}

	// Convert types.DeviceProfile to playbackmodule.DeviceProfile
	localProfile := &DeviceProfile{
		UserAgent:       deviceProfile.UserAgent,
		SupportedCodecs: deviceProfile.SupportedCodecs,
		MaxResolution:   deviceProfile.MaxResolution,
		MaxBitrate:      deviceProfile.MaxBitrate,
		SupportsHEVC:    deviceProfile.SupportsHEVC,
		SupportsAV1:     deviceProfile.SupportsAV1,
		SupportsHDR:     deviceProfile.SupportsHDR,
		ClientIP:        deviceProfile.ClientIP,
	}

	decision, err := s.module.manager.DecidePlayback(mediaPath, localProfile)
	if err != nil {
		return nil, err
	}

	if !decision.ShouldTranscode {
		return nil, fmt.Errorf("media file does not require transcoding")
	}

	return decision.TranscodeParams, nil
}

// Register the service adapter
func (m *Module) RegisterService() error {
	adapter := NewServiceAdapter(m)
	return services.Register("playback", adapter)
}
