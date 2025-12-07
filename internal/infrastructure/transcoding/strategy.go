package transcoding

import (
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/strategy"
)

// StreamStrategy is re-exported from the strategy subpackage for backward compatibility.
type StreamStrategy = strategy.StreamStrategy

// Stream strategy constants - re-exported from strategy subpackage.
const (
	DirectPlay        = strategy.DirectPlay
	Remux             = strategy.Remux
	RemuxWithAudioDownmix = strategy.RemuxWithAudioDownmix
	RemuxHEVC         = strategy.RemuxHEVC
	Transcode         = strategy.Transcode
)

// ClientCapabilitiesForStrategy contains codec support info from the client browser.
// Re-exported from strategy subpackage for backward compatibility.
type ClientCapabilitiesForStrategy = strategy.ClientCapabilities

// DetermineStreamStrategy analyzes video metadata and determines the optimal streaming strategy.
// Re-exported from strategy subpackage for backward compatibility.
func DetermineStreamStrategy(videoInfo *VideoInfo) (StreamStrategy, string) {
	// Convert VideoInfo to strategy.VideoInfo
	strategyVideoInfo := convertToStrategyVideoInfo(videoInfo)
	return strategy.DetermineStrategy(strategyVideoInfo)
}

// DetermineStreamStrategyWithCapabilities analyzes video metadata against client capabilities.
// Re-exported from strategy subpackage for backward compatibility.
func DetermineStreamStrategyWithCapabilities(videoInfo *VideoInfo, clientCaps *ClientCapabilitiesForStrategy) (StreamStrategy, string) {
	strategyVideoInfo := convertToStrategyVideoInfo(videoInfo)
	return strategy.DetermineStrategyWithCapabilities(strategyVideoInfo, clientCaps)
}

// ShouldTranscode determines if transcoding is necessary based on current codec/resolution vs target.
// Re-exported from strategy subpackage for backward compatibility.
func ShouldTranscode(videoInfo *VideoInfo, profile *AdaptiveProfile) (bool, string) {
	strategyVideoInfo := convertToStrategyVideoInfo(videoInfo)
	strategyProfile := &strategy.AdaptiveProfile{
		Width:        profile.Width,
		Height:       profile.Height,
		VideoBitrate: profile.VideoBitrate,
		AudioBitrate: profile.AudioBitrate,
	}
	return strategy.ShouldTranscode(strategyVideoInfo, strategyProfile)
}

// convertToStrategyVideoInfo converts probe.VideoInfo (via transcoding.VideoInfo alias) to strategy.VideoInfo.
func convertToStrategyVideoInfo(v *VideoInfo) *strategy.VideoInfo {
	if v == nil {
		return nil
	}
	return &strategy.VideoInfo{
		Codec:           v.Codec,
		Width:           v.Width,
		Height:          v.Height,
		Bitrate:         v.Bitrate,
		Duration:        v.Duration,
		AudioCodec:      v.AudioCodec,
		AudioBitrate:    v.AudioBitrate,
		AudioChannels:   v.AudioChannels,
		ContainerFormat: v.ContainerFormat,
		IsHDR:           v.IsHDR,
	}
}
