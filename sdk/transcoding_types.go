package plugins

// This file provides type aliases to transcoding types for the main SDK interfaces

import (
	"github.com/mantonx/viewra/sdk/transcoding/types"
)

// Type aliases for clean interface definitions
type (
	TranscodeRequest       = types.TranscodeRequest
	TranscodingProgress    = types.TranscodingProgress
	TranscodeStatus        = types.TranscodeStatus
	SpeedPriority          = types.SpeedPriority
	TranscodeHandle        = types.TranscodeHandle
	StreamHandle           = types.StreamHandle
	TranscodeResult        = types.TranscodeResult
	HardwareInfo           = types.HardwareInfo
	VideoInfo              = types.VideoInfo
	AudioInfo              = types.AudioInfo
	Resolution             = types.Resolution
)

// Constants
const (
	SpeedPriorityFastest  = types.SpeedPriorityFastest
	SpeedPriorityBalanced = types.SpeedPriorityBalanced
	SpeedPriorityQuality  = types.SpeedPriorityQuality

	TranscodeStatusStarting  = types.TranscodeStatusStarting
	TranscodeStatusRunning   = types.TranscodeStatusRunning
	TranscodeStatusCompleted = types.TranscodeStatusCompleted
	TranscodeStatusFailed    = types.TranscodeStatusFailed
	TranscodeStatusCancelled = types.TranscodeStatusCancelled

	HardwareTypeNone         = types.HardwareTypeNone
	HardwareTypeNVIDIA       = types.HardwareTypeNVIDIA
	HardwareTypeVAAPI        = types.HardwareTypeVAAPI
	HardwareTypeQSV          = types.HardwareTypeQSV
	HardwareTypeVideoToolbox = types.HardwareTypeVideoToolbox
)

