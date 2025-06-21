package plugins

// This file provides type aliases to transcoding types for the main SDK interfaces

import (
	"github.com/mantonx/viewra/sdk/transcoding"
)

// Type aliases for clean interface definitions
type (
	TranscodeRequest       = transcoding.TranscodeRequest
	TranscodeResult        = transcoding.TranscodeResult
	TranscodingProgress    = transcoding.TranscodingProgress
	TranscodeStatus        = transcoding.TranscodeStatus
	SpeedPriority          = transcoding.SpeedPriority
	HardwareType           = transcoding.HardwareType
	VideoResolution        = transcoding.VideoResolution
	HardwareInfo           = transcoding.HardwareInfo
	QualityMapper          = transcoding.QualityMapper
)

// Constants
const (
	SpeedPriorityFastest  = transcoding.SpeedPriorityFastest
	SpeedPriorityBalanced = transcoding.SpeedPriorityBalanced
	SpeedPriorityQuality  = transcoding.SpeedPriorityQuality

	HardwareTypeNone         = transcoding.HardwareTypeNone
	HardwareTypeCUDA         = transcoding.HardwareTypeCUDA
	HardwareTypeVAAPI        = transcoding.HardwareTypeVAAPI
	HardwareTypeQSV          = transcoding.HardwareTypeQSV
	HardwareTypeVideoToolbox = transcoding.HardwareTypeVideoToolbox
	HardwareTypeAMF          = transcoding.HardwareTypeAMF

	TranscodeStatusPending   = transcoding.TranscodeStatusPending
	TranscodeStatusStarting  = transcoding.TranscodeStatusStarting
	TranscodeStatusRunning   = transcoding.TranscodeStatusRunning
	TranscodeStatusCompleted = transcoding.TranscodeStatusCompleted
	TranscodeStatusFailed    = transcoding.TranscodeStatusFailed
	TranscodeStatusCancelled = transcoding.TranscodeStatusCancelled
)

// Variables
var (
	Resolution480p  = transcoding.Resolution480p
	Resolution720p  = transcoding.Resolution720p
	Resolution1080p = transcoding.Resolution1080p
	Resolution1440p = transcoding.Resolution1440p
	Resolution4K    = transcoding.Resolution4K
	Resolution8K    = transcoding.Resolution8K
)