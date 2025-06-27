// Package pipeline provides device-specific transcoding parameter optimization
package pipeline

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

// DeviceProfileOptimizer optimizes transcoding parameters for specific devices and use cases
type DeviceProfileOptimizer struct {
	logger hclog.Logger

	// Device profiles database
	deviceProfiles map[string]*DeviceProfile
	useProfiles    map[string]*UseProfile

	// Content analysis integration
	fastStartOptimizer *FastStartOptimizer
	adaptiveSizer      *AdaptiveSegmentSizer

	// Optimization settings
	config OptimizerConfig
}

// DeviceProfile defines optimal parameters for a specific device category
type DeviceProfile struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Category    DeviceCategory `json:"category"`
	Description string         `json:"description"`

	// Video capabilities
	MaxResolution   Resolution     `json:"max_resolution"`
	SupportedCodecs []string       `json:"supported_codecs"`
	OptimalBitrates map[string]int `json:"optimal_bitrates"` // resolution -> bitrate

	// Hardware constraints
	MaxFrameRate       float64           `json:"max_frame_rate"`
	DecodingCapability string            `json:"decoding_capability"` // hardware, software, hybrid
	MemoryConstraints  MemoryProfile     `json:"memory_constraints"`
	ProcessingPower    ProcessingProfile `json:"processing_power"`

	// Streaming preferences
	PreferredSegmentDuration int            `json:"preferred_segment_duration"`
	BufferingStrategy        string         `json:"buffering_strategy"`
	AdaptiveBitrate          bool           `json:"adaptive_bitrate"`
	StartupOptimization      StartupProfile `json:"startup_optimization"`

	// Quality preferences
	QualityPriority  QualityPriority  `json:"quality_priority"`
	LatencyTolerance LatencyTolerance `json:"latency_tolerance"`

	// Network assumptions
	NetworkProfile NetworkProfile `json:"network_profile"`

	// Last updated
	UpdatedAt time.Time `json:"updated_at"`
}

// UseProfile defines optimization parameters for specific use cases
type UseProfile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	// Use case characteristics
	ViewingPattern     ViewingPattern     `json:"viewing_pattern"`
	InteractionLevel   InteractionLevel   `json:"interaction_level"`
	QualityExpectation QualityExpectation `json:"quality_expectation"`

	// Optimization priorities
	OptimizeFor []OptimizationGoal `json:"optimize_for"`
	Constraints UseCaseConstraints `json:"constraints"`

	// Encoding preferences
	EncodingPreferences EncodingPreferences `json:"encoding_preferences"`
}

// OptimizationParameters contains the final optimized parameters
type OptimizationParameters struct {
	// Basic video settings
	Resolution Resolution `json:"resolution"`
	FrameRate  float64    `json:"frame_rate"`
	Bitrate    int        `json:"bitrate"`
	Quality    int        `json:"quality"` // CRF value

	// Codec settings
	Codec   string `json:"codec"`
	Profile string `json:"profile"`
	Level   string `json:"level"`
	Preset  string `json:"preset"`
	Tune    string `json:"tune"`

	// Advanced settings
	GOPSize         int `json:"gop_size"`
	BFrames         int `json:"b_frames"`
	ReferenceFrames int `json:"reference_frames"`

	// Streaming settings
	SegmentDuration int `json:"segment_duration"`
	BufferSize      int `json:"buffer_size"`
	MaxBitrate      int `json:"max_bitrate"`
	MinBitrate      int `json:"min_bitrate"`

	// Optimization flags
	FastStart         bool `json:"fast_start"`
	LowLatency        bool `json:"low_latency"`
	AdaptiveStreaming bool `json:"adaptive_streaming"`

	// Metadata
	OptimizedFor      []string  `json:"optimized_for"`
	OptimizationScore float64   `json:"optimization_score"`
	GeneratedAt       time.Time `json:"generated_at"`
}

// Supporting types
type DeviceCategory string
type QualityPriority string
type LatencyTolerance string
type ViewingPattern string
type InteractionLevel string
type QualityExpectation string
type OptimizationGoal string

const (
	DeviceMobile   DeviceCategory = "mobile"
	DeviceTablet   DeviceCategory = "tablet"
	DeviceDesktop  DeviceCategory = "desktop"
	DeviceTV       DeviceCategory = "tv"
	DeviceSTB      DeviceCategory = "set_top_box"
	DeviceConsole  DeviceCategory = "gaming_console"
	DeviceEmbedded DeviceCategory = "embedded"

	QualityHigh      QualityPriority = "high"
	QualityBalanced  QualityPriority = "balanced"
	QualityEfficient QualityPriority = "efficient"

	LatencyVeryLow LatencyTolerance = "very_low"
	LatencyLow     LatencyTolerance = "low"
	LatencyNormal  LatencyTolerance = "normal"
	LatencyHigh    LatencyTolerance = "high"

	ViewingLinear      ViewingPattern = "linear"
	ViewingInteractive ViewingPattern = "interactive"
	ViewingRandom      ViewingPattern = "random_access"
	ViewingLive        ViewingPattern = "live"

	InteractionNone   InteractionLevel = "none"
	InteractionLow    InteractionLevel = "low"
	InteractionMedium InteractionLevel = "medium"
	InteractionHigh   InteractionLevel = "high"

	QualityBasic    QualityExpectation = "basic"
	QualityStandard QualityExpectation = "standard"
	QualityPremium  QualityExpectation = "premium"
	QualityUltra    QualityExpectation = "ultra"

	OptimizeSpeed     OptimizationGoal = "encoding_speed"
	OptimizeQuality   OptimizationGoal = "quality"
	OptimizeSize      OptimizationGoal = "file_size"
	OptimizeStartup   OptimizationGoal = "startup_time"
	OptimizeLatency   OptimizationGoal = "latency"
	OptimizeBandwidth OptimizationGoal = "bandwidth"
)

type Resolution struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Name   string `json:"name"`
}

type MemoryProfile struct {
	Available   int64 `json:"available_mb"`
	Recommended int64 `json:"recommended_mb"`
	MaxBuffer   int64 `json:"max_buffer_mb"`
}

type ProcessingProfile struct {
	Capability      string  `json:"capability"` // low, medium, high, ultra
	Cores           int     `json:"cores"`
	Frequency       float64 `json:"frequency_ghz"`
	Architecture    string  `json:"architecture"` // arm, x86, etc.
	GPUAcceleration bool    `json:"gpu_acceleration"`
}

type StartupProfile struct {
	MaxStartupTime   time.Duration `json:"max_startup_time"`
	PrefetchSegments int           `json:"prefetch_segments"`
	FastStart        bool          `json:"fast_start"`
	InitialBitrate   int           `json:"initial_bitrate"`
}

type NetworkProfile struct {
	TypicalBandwidth int     `json:"typical_bandwidth_kbps"`
	Variability      float64 `json:"variability"` // 0.0-1.0
	Reliability      float64 `json:"reliability"` // 0.0-1.0
	LatencyMs        int     `json:"latency_ms"`
}

type UseCaseConstraints struct {
	MaxFileSize     int64         `json:"max_file_size_mb"`
	MaxEncodingTime time.Duration `json:"max_encoding_time"`
	MaxBandwidth    int           `json:"max_bandwidth_kbps"`
	RequiredFormats []string      `json:"required_formats"`
}

type EncodingPreferences struct {
	PreferHardware bool         `json:"prefer_hardware"`
	AllowedPresets []string     `json:"allowed_presets"`
	QualityRange   QualityRange `json:"quality_range"`
	BitrateRange   BitrateRange `json:"bitrate_range"`
}

type QualityRange struct {
	Min int `json:"min_crf"`
	Max int `json:"max_crf"`
}

type BitrateRange struct {
	Min int `json:"min_kbps"`
	Max int `json:"max_kbps"`
}

type OptimizerConfig struct {
	EnableAdaptiveOptimization bool   `json:"enable_adaptive_optimization"`
	ContentAnalysisDepth       int    `json:"content_analysis_depth"`
	CacheOptimizations         bool   `json:"cache_optimizations"`
	DefaultDeviceProfile       string `json:"default_device_profile"`
	DefaultUseProfile          string `json:"default_use_profile"`
}

// NewDeviceProfileOptimizer creates a new device profile optimizer
func NewDeviceProfileOptimizer(logger hclog.Logger, fastStartOptimizer *FastStartOptimizer, adaptiveSizer *AdaptiveSegmentSizer) *DeviceProfileOptimizer {
	optimizer := &DeviceProfileOptimizer{
		logger:             logger,
		deviceProfiles:     make(map[string]*DeviceProfile),
		useProfiles:        make(map[string]*UseProfile),
		fastStartOptimizer: fastStartOptimizer,
		adaptiveSizer:      adaptiveSizer,
		config: OptimizerConfig{
			EnableAdaptiveOptimization: true,
			ContentAnalysisDepth:       2,
			CacheOptimizations:         true,
			DefaultDeviceProfile:       "balanced_device",
			DefaultUseProfile:          "standard_streaming",
		},
	}

	// Initialize default profiles
	optimizer.initializeDefaultProfiles()

	return optimizer
}

// OptimizeForDevice generates optimized parameters for a specific device and use case
func (dpo *DeviceProfileOptimizer) OptimizeForDevice(ctx context.Context, req OptimizationRequest) (*OptimizationParameters, error) {
	dpo.logger.Debug("Optimizing parameters for device",
		"device_profile", req.DeviceProfileID,
		"use_profile", req.UseProfileID,
		"content_type", req.ContentType)

	// Get device profile
	deviceProfile := dpo.getDeviceProfile(req.DeviceProfileID)
	if deviceProfile == nil {
		return nil, fmt.Errorf("device profile not found: %s", req.DeviceProfileID)
	}

	// Get use case profile
	useProfile := dpo.getUseProfile(req.UseProfileID)
	if useProfile == nil {
		return nil, fmt.Errorf("use profile not found: %s", req.UseProfileID)
	}

	// Analyze content if provided
	var contentAnalysis *ContentAnalysis
	if req.InputPath != "" && dpo.config.EnableAdaptiveOptimization {
		var err error
		contentAnalysis, err = dpo.analyzeContent(ctx, req.InputPath)
		if err != nil {
			dpo.logger.Warn("Failed to analyze content, using defaults", "error", err)
		}
	}

	// Generate optimized parameters
	params := dpo.generateOptimizedParameters(deviceProfile, useProfile, contentAnalysis, req)

	// Calculate optimization score
	params.OptimizationScore = dpo.calculateOptimizationScore(params, deviceProfile, useProfile)
	params.GeneratedAt = time.Now()

	dpo.logger.Debug("Generated optimized parameters",
		"resolution", fmt.Sprintf("%dx%d", params.Resolution.Width, params.Resolution.Height),
		"bitrate", params.Bitrate,
		"quality", params.Quality,
		"optimization_score", params.OptimizationScore)

	return params, nil
}

// OptimizationRequest contains the request parameters for optimization
type OptimizationRequest struct {
	DeviceProfileID string                 `json:"device_profile_id"`
	UseProfileID    string                 `json:"use_profile_id"`
	ContentType     string                 `json:"content_type"`
	InputPath       string                 `json:"input_path,omitempty"`
	TargetFormats   []string               `json:"target_formats"`
	Constraints     map[string]interface{} `json:"constraints,omitempty"`
	UserPreferences map[string]interface{} `json:"user_preferences,omitempty"`
}

// ContentAnalysis contains analyzed content characteristics
type ContentAnalysis struct {
	Resolution       Resolution    `json:"resolution"`
	FrameRate        float64       `json:"frame_rate"`
	Duration         time.Duration `json:"duration"`
	Bitrate          int           `json:"bitrate"`
	Codec            string        `json:"codec"`
	ComplexityScore  float64       `json:"complexity_score"`
	MotionLevel      string        `json:"motion_level"`
	SceneChanges     int           `json:"scene_changes"`
	KeyframeInterval float64       `json:"keyframe_interval"`
}

// generateOptimizedParameters creates optimized parameters based on profiles and content
func (dpo *DeviceProfileOptimizer) generateOptimizedParameters(device *DeviceProfile, useCase *UseProfile, content *ContentAnalysis, req OptimizationRequest) *OptimizationParameters {
	params := &OptimizationParameters{
		OptimizedFor: []string{device.ID, useCase.ID},
	}

	// Determine optimal resolution
	params.Resolution = dpo.optimizeResolution(device, useCase, content)

	// Determine optimal frame rate
	params.FrameRate = dpo.optimizeFrameRate(device, useCase, content)

	// Determine optimal bitrate
	params.Bitrate = dpo.optimizeBitrate(device, useCase, content, params.Resolution)

	// Determine optimal quality
	params.Quality = dpo.optimizeQuality(device, useCase, content)

	// Determine codec settings
	params.Codec, params.Profile, params.Level = dpo.optimizeCodec(device, useCase, content)

	// Determine encoding preset
	params.Preset, params.Tune = dpo.optimizePreset(device, useCase, content)

	// Determine advanced settings
	params.GOPSize = dpo.optimizeGOPSize(device, useCase, content)
	params.BFrames = dpo.optimizeBFrames(device, useCase, content)
	params.ReferenceFrames = dpo.optimizeReferenceFrames(device, useCase, content)

	// Determine streaming settings
	params.SegmentDuration = dpo.optimizeSegmentDuration(device, useCase, content)
	params.BufferSize = dpo.optimizeBufferSize(device, useCase, content)
	params.MaxBitrate = int(float64(params.Bitrate) * 1.5) // 150% of target
	params.MinBitrate = int(float64(params.Bitrate) * 0.5) // 50% of target

	// Determine optimization flags
	params.FastStart = dpo.shouldEnableFastStart(device, useCase)
	params.LowLatency = dpo.shouldEnableLowLatency(device, useCase)
	params.AdaptiveStreaming = dpo.shouldEnableAdaptiveStreaming(device, useCase)

	return params
}

// optimizeResolution determines the best resolution for the target device
func (dpo *DeviceProfileOptimizer) optimizeResolution(device *DeviceProfile, useCase *UseProfile, content *ContentAnalysis) Resolution {
	// Start with device max resolution
	targetRes := device.MaxResolution

	// Consider use case constraints
	if useCase.Constraints.MaxBandwidth > 0 {
		// Adjust resolution based on bandwidth constraints
		maxBitrateForBandwidth := int(float64(useCase.Constraints.MaxBandwidth) * 0.8) // 80% of bandwidth
		if maxBitrateForBandwidth < dpo.getBitrateForResolution(targetRes) {
			targetRes = dpo.getResolutionForBitrate(maxBitrateForBandwidth)
		}
	}

	// Consider content original resolution
	if content != nil {
		// Don't upscale unless specifically requested
		if content.Resolution.Width < targetRes.Width || content.Resolution.Height < targetRes.Height {
			if useCase.QualityExpectation != QualityUltra {
				targetRes = content.Resolution
			}
		}
	}

	return dpo.normalizeResolution(targetRes)
}

// optimizeFrameRate determines the optimal frame rate
func (dpo *DeviceProfileOptimizer) optimizeFrameRate(device *DeviceProfile, useCase *UseProfile, content *ContentAnalysis) float64 {
	maxFPS := device.MaxFrameRate

	// Consider content frame rate
	if content != nil && content.FrameRate > 0 {
		// Generally don't increase frame rate unless it's for gaming/interactive content
		if useCase.InteractionLevel == InteractionHigh || useCase.ViewingPattern == ViewingInteractive {
			return math.Min(maxFPS, math.Max(content.FrameRate, 60.0))
		} else {
			return math.Min(maxFPS, content.FrameRate)
		}
	}

	// Default frame rates based on use case
	switch useCase.ViewingPattern {
	case ViewingLive, ViewingInteractive:
		return math.Min(maxFPS, 60.0)
	case ViewingLinear:
		return math.Min(maxFPS, 30.0)
	default:
		return math.Min(maxFPS, 30.0)
	}
}

// optimizeBitrate calculates optimal bitrate based on resolution and quality requirements
func (dpo *DeviceProfileOptimizer) optimizeBitrate(device *DeviceProfile, useCase *UseProfile, content *ContentAnalysis, resolution Resolution) int {
	// Base bitrate from device profile
	resKey := fmt.Sprintf("%dx%d", resolution.Width, resolution.Height)
	baseBitrate := device.OptimalBitrates[resKey]

	if baseBitrate == 0 {
		// Fallback calculation
		baseBitrate = dpo.calculateBaseBitrate(resolution)
	}

	// Adjust based on quality expectation
	qualityMultiplier := 1.0
	switch useCase.QualityExpectation {
	case QualityBasic:
		qualityMultiplier = 0.7
	case QualityStandard:
		qualityMultiplier = 1.0
	case QualityPremium:
		qualityMultiplier = 1.3
	case QualityUltra:
		qualityMultiplier = 1.6
	}

	// Adjust based on content complexity
	if content != nil && content.ComplexityScore > 0 {
		complexityMultiplier := 0.8 + (content.ComplexityScore * 0.4) // 0.8 to 1.2 range
		qualityMultiplier *= complexityMultiplier
	}

	// Apply constraints
	targetBitrate := int(float64(baseBitrate) * qualityMultiplier)

	if useCase.EncodingPreferences.BitrateRange.Max > 0 {
		targetBitrate = int(math.Min(float64(targetBitrate), float64(useCase.EncodingPreferences.BitrateRange.Max)))
	}
	if useCase.EncodingPreferences.BitrateRange.Min > 0 {
		targetBitrate = int(math.Max(float64(targetBitrate), float64(useCase.EncodingPreferences.BitrateRange.Min)))
	}

	return targetBitrate
}

// optimizeQuality determines optimal CRF value
func (dpo *DeviceProfileOptimizer) optimizeQuality(device *DeviceProfile, useCase *UseProfile, content *ContentAnalysis) int {
	// Base quality from priority
	baseQuality := 23 // Default CRF

	switch device.QualityPriority {
	case QualityHigh:
		baseQuality = 20
	case QualityBalanced:
		baseQuality = 23
	case QualityEfficient:
		baseQuality = 26
	}

	// Adjust based on use case expectations
	switch useCase.QualityExpectation {
	case QualityBasic:
		baseQuality += 3
	case QualityStandard:
		// No adjustment
	case QualityPremium:
		baseQuality -= 2
	case QualityUltra:
		baseQuality -= 4
	}

	// Apply quality range constraints
	if useCase.EncodingPreferences.QualityRange.Max > 0 {
		baseQuality = int(math.Max(float64(baseQuality), float64(useCase.EncodingPreferences.QualityRange.Max)))
	}
	if useCase.EncodingPreferences.QualityRange.Min > 0 {
		baseQuality = int(math.Min(float64(baseQuality), float64(useCase.EncodingPreferences.QualityRange.Min)))
	}

	// Clamp to reasonable range
	return int(math.Max(15, math.Min(35, float64(baseQuality))))
}

// Helper methods for resolution and bitrate calculations
func (dpo *DeviceProfileOptimizer) normalizeResolution(res Resolution) Resolution {
	// Common resolutions with proper names
	resolutions := []Resolution{
		{Width: 320, Height: 240, Name: "240p"},
		{Width: 640, Height: 360, Name: "360p"},
		{Width: 854, Height: 480, Name: "480p"},
		{Width: 1280, Height: 720, Name: "720p"},
		{Width: 1920, Height: 1080, Name: "1080p"},
		{Width: 2560, Height: 1440, Name: "1440p"},
		{Width: 3840, Height: 2160, Name: "4K"},
	}

	// Find closest standard resolution
	bestMatch := resolutions[0]
	minDiff := math.Abs(float64(res.Width*res.Height - bestMatch.Width*bestMatch.Height))

	for _, stdRes := range resolutions {
		diff := math.Abs(float64(res.Width*res.Height - stdRes.Width*stdRes.Height))
		if diff < minDiff && stdRes.Width <= res.Width && stdRes.Height <= res.Height {
			bestMatch = stdRes
			minDiff = diff
		}
	}

	return bestMatch
}

func (dpo *DeviceProfileOptimizer) calculateBaseBitrate(resolution Resolution) int {
	// Approximate bitrate calculation: pixels * frame_rate * bits_per_pixel
	pixels := resolution.Width * resolution.Height
	bitsPerPixel := 0.1 // Conservative estimate
	frameRate := 30.0

	bitrate := int(float64(pixels) * frameRate * bitsPerPixel / 1000) // Convert to kbps

	// Apply some practical bounds
	return int(math.Max(500, math.Min(20000, float64(bitrate))))
}

func (dpo *DeviceProfileOptimizer) getBitrateForResolution(resolution Resolution) int {
	return dpo.calculateBaseBitrate(resolution)
}

func (dpo *DeviceProfileOptimizer) getResolutionForBitrate(maxBitrate int) Resolution {
	// Find the highest resolution that fits within the bitrate
	resolutions := []Resolution{
		{Width: 320, Height: 240, Name: "240p"},
		{Width: 640, Height: 360, Name: "360p"},
		{Width: 854, Height: 480, Name: "480p"},
		{Width: 1280, Height: 720, Name: "720p"},
		{Width: 1920, Height: 1080, Name: "1080p"},
	}

	for i := len(resolutions) - 1; i >= 0; i-- {
		if dpo.calculateBaseBitrate(resolutions[i]) <= maxBitrate {
			return resolutions[i]
		}
	}

	return resolutions[0] // Fallback to lowest resolution
}

// optimizeCodec determines the best codec and profile settings
func (dpo *DeviceProfileOptimizer) optimizeCodec(device *DeviceProfile, useCase *UseProfile, content *ContentAnalysis) (codec, profile, level string) {
	// Default to H.264 for broad compatibility
	codec = "libx264"
	profile = "high"
	level = "4.0"

	// Adjust profile based on device capability for H.264
	switch device.ProcessingPower.Capability {
	case "low":
		profile = "baseline"
		level = "3.1"
	case "medium":
		profile = "main"
		level = "4.0"
	case "high", "ultra":
		profile = "high"
		level = "4.2"
	}

	// Check device codec support for H.265
	for _, supportedCodec := range device.SupportedCodecs {
		if supportedCodec == "h265" || supportedCodec == "hevc" {
			// Use H.265 for higher efficiency if supported
			if useCase.QualityExpectation == QualityPremium || useCase.QualityExpectation == QualityUltra {
				codec = "libx265"
				profile = "main" // H.265 uses main profile
				level = "4.0"    // Appropriate level for H.265
			}
		}
	}

	return codec, profile, level
}

// optimizePreset determines encoding preset and tune settings
func (dpo *DeviceProfileOptimizer) optimizePreset(device *DeviceProfile, useCase *UseProfile, content *ContentAnalysis) (preset, tune string) {
	// Default preset
	preset = "medium"
	tune = ""

	// Optimize for use case goals
	for _, goal := range useCase.OptimizeFor {
		switch goal {
		case OptimizeSpeed:
			preset = "ultrafast"
		case OptimizeQuality:
			preset = "slow"
		case OptimizeLatency:
			preset = "ultrafast"
			tune = "zerolatency"
		case OptimizeStartup:
			tune = "fastdecode"
		}
	}

	// Consider device processing power
	if device.ProcessingPower.Capability == "low" {
		preset = "fast" // Don't use ultrafast on low power devices
	} else if device.ProcessingPower.Capability == "ultra" {
		// High-end devices can handle slower presets for better quality
		if preset == "medium" {
			preset = "slow"
		}
	}

	// Check allowed presets
	if len(useCase.EncodingPreferences.AllowedPresets) > 0 {
		allowed := false
		for _, allowedPreset := range useCase.EncodingPreferences.AllowedPresets {
			if allowedPreset == preset {
				allowed = true
				break
			}
		}
		if !allowed {
			preset = useCase.EncodingPreferences.AllowedPresets[0] // Use first allowed preset
		}
	}

	return preset, tune
}

// optimizeGOPSize determines optimal GOP (Group of Pictures) size
func (dpo *DeviceProfileOptimizer) optimizeGOPSize(device *DeviceProfile, useCase *UseProfile, content *ContentAnalysis) int {
	// Base GOP size on use case and device
	baseGOP := 60 // 2 seconds at 30fps

	switch useCase.ViewingPattern {
	case ViewingLive:
		baseGOP = 30 // 1 second for live content
	case ViewingInteractive:
		baseGOP = 30 // Shorter GOP for better seeking
	case ViewingLinear:
		baseGOP = 90 // Longer GOP for efficiency
	case ViewingRandom:
		baseGOP = 45 // Medium GOP for balance
	}

	// Adjust for latency requirements
	if device.LatencyTolerance == LatencyVeryLow || device.LatencyTolerance == LatencyLow {
		baseGOP = int(math.Min(float64(baseGOP), 30))
	}

	// Consider segment duration
	segmentDuration := device.PreferredSegmentDuration
	if segmentDuration > 0 {
		// GOP should align with segment boundaries
		gopSeconds := float64(baseGOP) / 30.0 // Assuming 30fps
		if gopSeconds > float64(segmentDuration) {
			baseGOP = segmentDuration * 30 // Align to segment duration
		}
	}

	return baseGOP
}

// optimizeBFrames determines optimal number of B-frames
func (dpo *DeviceProfileOptimizer) optimizeBFrames(device *DeviceProfile, useCase *UseProfile, content *ContentAnalysis) int {
	// Default B-frames
	bframes := 3

	// Reduce B-frames for low latency
	if device.LatencyTolerance == LatencyVeryLow {
		bframes = 0
	} else if device.LatencyTolerance == LatencyLow {
		bframes = 1
	}

	// Reduce for low processing power
	if device.ProcessingPower.Capability == "low" {
		bframes = 1
	}

	// Increase for high quality requirements
	if useCase.QualityExpectation == QualityUltra && device.ProcessingPower.Capability == "ultra" {
		bframes = 5
	}

	return bframes
}

// optimizeReferenceFrames determines optimal number of reference frames
func (dpo *DeviceProfileOptimizer) optimizeReferenceFrames(device *DeviceProfile, useCase *UseProfile, content *ContentAnalysis) int {
	// Default reference frames
	refFrames := 3

	// Adjust based on device memory constraints
	if device.MemoryConstraints.Available < 1024 { // Less than 1GB
		refFrames = 1
	} else if device.MemoryConstraints.Available < 2048 { // Less than 2GB
		refFrames = 2
	}

	// Increase for high quality on capable devices
	if useCase.QualityExpectation == QualityUltra && device.MemoryConstraints.Available > 4096 {
		refFrames = 5
	}

	// Consider content motion level
	if content != nil && content.MotionLevel == "high" {
		refFrames = int(math.Min(float64(refFrames)+1, 5))
	}

	return refFrames
}

// optimizeSegmentDuration determines optimal segment duration
func (dpo *DeviceProfileOptimizer) optimizeSegmentDuration(device *DeviceProfile, useCase *UseProfile, content *ContentAnalysis) int {
	// Start with device preference
	segmentDuration := device.PreferredSegmentDuration
	if segmentDuration == 0 {
		segmentDuration = 4 // Default 4 seconds
	}

	// Adjust based on use case
	switch useCase.ViewingPattern {
	case ViewingLive:
		segmentDuration = 2 // Shorter segments for live
	case ViewingInteractive:
		segmentDuration = 2 // Shorter for better seeking
	case ViewingLinear:
		segmentDuration = 6 // Longer for efficiency
	}

	// Consider network profile
	if device.NetworkProfile.Reliability < 0.8 {
		segmentDuration = 2 // Shorter segments for unreliable networks
	}

	return segmentDuration
}

// optimizeBufferSize determines optimal buffer size
func (dpo *DeviceProfileOptimizer) optimizeBufferSize(device *DeviceProfile, useCase *UseProfile, content *ContentAnalysis) int {
	// Base buffer size on device memory
	baseBuffer := int(device.MemoryConstraints.Recommended / 10) // 10% of recommended memory

	// Adjust for network conditions
	if device.NetworkProfile.Variability > 0.5 {
		baseBuffer = int(float64(baseBuffer) * 1.5) // Larger buffer for variable networks
	}

	// Consider latency tolerance
	if device.LatencyTolerance == LatencyVeryLow {
		baseBuffer = int(float64(baseBuffer) * 0.5) // Smaller buffer for low latency
	}

	// Clamp to reasonable range (in MB)
	return int(math.Max(16, math.Min(512, float64(baseBuffer))))
}

// shouldEnableFastStart determines if fast-start should be enabled
func (dpo *DeviceProfileOptimizer) shouldEnableFastStart(device *DeviceProfile, useCase *UseProfile) bool {
	// Enable for startup optimization
	for _, goal := range useCase.OptimizeFor {
		if goal == OptimizeStartup {
			return true
		}
	}

	// Enable for interactive viewing
	if useCase.ViewingPattern == ViewingInteractive || useCase.InteractionLevel == InteractionHigh {
		return true
	}

	// Enable based on device startup profile
	return device.StartupOptimization.FastStart
}

// shouldEnableLowLatency determines if low-latency mode should be enabled
func (dpo *DeviceProfileOptimizer) shouldEnableLowLatency(device *DeviceProfile, useCase *UseProfile) bool {
	// Enable for very low latency requirements
	if device.LatencyTolerance == LatencyVeryLow {
		return true
	}

	// Enable for live content
	if useCase.ViewingPattern == ViewingLive {
		return true
	}

	// Enable if latency optimization is requested
	for _, goal := range useCase.OptimizeFor {
		if goal == OptimizeLatency {
			return true
		}
	}

	return false
}

// shouldEnableAdaptiveStreaming determines if adaptive streaming should be enabled
func (dpo *DeviceProfileOptimizer) shouldEnableAdaptiveStreaming(device *DeviceProfile, useCase *UseProfile) bool {
	// Enable if device supports it
	if !device.AdaptiveBitrate {
		return false
	}

	// Enable for variable network conditions
	if device.NetworkProfile.Variability > 0.3 {
		return true
	}

	// Enable for bandwidth optimization
	for _, goal := range useCase.OptimizeFor {
		if goal == OptimizeBandwidth {
			return true
		}
	}

	return true // Generally enable by default
}

// analyzeContent analyzes input content characteristics
func (dpo *DeviceProfileOptimizer) analyzeContent(ctx context.Context, inputPath string) (*ContentAnalysis, error) {
	if dpo.fastStartOptimizer == nil {
		return nil, fmt.Errorf("content analysis not available")
	}

	// Use existing optimizers for content analysis
	keyframes, err := dpo.fastStartOptimizer.AnalyzeKeyframes(ctx, inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze keyframes: %w", err)
	}

	complexityScores, err := dpo.fastStartOptimizer.AnalyzeSceneComplexity(ctx, inputPath)
	if err != nil {
		dpo.logger.Warn("Failed to analyze scene complexity", "error", err)
		complexityScores = []float64{0.5} // Default complexity
	}

	// Calculate average complexity
	avgComplexity := 0.5
	if len(complexityScores) > 0 {
		var sum float64
		for _, score := range complexityScores {
			sum += score
		}
		avgComplexity = sum / float64(len(complexityScores))
	}

	// Determine motion level
	motionLevel := "medium"
	if avgComplexity > 0.7 {
		motionLevel = "high"
	} else if avgComplexity < 0.3 {
		motionLevel = "low"
	}

	// Calculate keyframe interval
	keyframeInterval := 2.0 // Default 2 seconds
	if len(keyframes) > 1 {
		totalDuration := keyframes[len(keyframes)-1].Timestamp
		keyframeInterval = totalDuration.Seconds() / float64(len(keyframes)-1)
	}

	analysis := &ContentAnalysis{
		ComplexityScore:  avgComplexity,
		MotionLevel:      motionLevel,
		SceneChanges:     len(complexityScores),
		KeyframeInterval: keyframeInterval,
	}

	return analysis, nil
}

// calculateOptimizationScore calculates how well the parameters match the requirements
func (dpo *DeviceProfileOptimizer) calculateOptimizationScore(params *OptimizationParameters, device *DeviceProfile, useCase *UseProfile) float64 {
	score := 0.0

	// Resolution optimization score (25% weight)
	resolutionScore := 1.0
	if params.Resolution.Width <= device.MaxResolution.Width && params.Resolution.Height <= device.MaxResolution.Height {
		resolutionScore = 1.0
	} else {
		resolutionScore = 0.5 // Penalize for exceeding device capabilities
	}
	score += 0.25 * resolutionScore

	// Bitrate optimization score (25% weight)
	bitrateScore := 1.0
	if useCase.EncodingPreferences.BitrateRange.Max > 0 {
		if params.Bitrate <= useCase.EncodingPreferences.BitrateRange.Max {
			bitrateScore = 1.0
		} else {
			bitrateScore = float64(useCase.EncodingPreferences.BitrateRange.Max) / float64(params.Bitrate)
		}
	}
	score += 0.25 * bitrateScore

	// Quality optimization score (20% weight)
	qualityScore := 1.0
	if useCase.EncodingPreferences.QualityRange.Min > 0 && useCase.EncodingPreferences.QualityRange.Max > 0 {
		if params.Quality >= useCase.EncodingPreferences.QualityRange.Min && params.Quality <= useCase.EncodingPreferences.QualityRange.Max {
			qualityScore = 1.0
		} else {
			qualityScore = 0.7
		}
	}
	score += 0.20 * qualityScore

	// Codec compatibility score (15% weight)
	codecScore := 0.5 // Default partial score
	for _, supportedCodec := range device.SupportedCodecs {
		if strings.Contains(params.Codec, supportedCodec) {
			codecScore = 1.0
			break
		}
	}
	score += 0.15 * codecScore

	// Feature alignment score (15% weight)
	featureScore := 0.0
	featureCount := 0

	// Check fast-start alignment
	if params.FastStart == device.StartupOptimization.FastStart {
		featureScore += 1.0
	}
	featureCount++

	// Check adaptive streaming alignment
	if params.AdaptiveStreaming == device.AdaptiveBitrate {
		featureScore += 1.0
	}
	featureCount++

	// Check latency alignment
	lowLatencyExpected := device.LatencyTolerance == LatencyVeryLow || device.LatencyTolerance == LatencyLow
	if params.LowLatency == lowLatencyExpected {
		featureScore += 1.0
	}
	featureCount++

	if featureCount > 0 {
		score += 0.15 * (featureScore / float64(featureCount))
	}

	return math.Min(1.0, score)
}

// Profile management methods
func (dpo *DeviceProfileOptimizer) getDeviceProfile(id string) *DeviceProfile {
	if profile, exists := dpo.deviceProfiles[id]; exists {
		return profile
	}

	// Return default profile if not found
	if defaultProfile, exists := dpo.deviceProfiles[dpo.config.DefaultDeviceProfile]; exists {
		return defaultProfile
	}

	return nil
}

func (dpo *DeviceProfileOptimizer) getUseProfile(id string) *UseProfile {
	if profile, exists := dpo.useProfiles[id]; exists {
		return profile
	}

	// Return default profile if not found
	if defaultProfile, exists := dpo.useProfiles[dpo.config.DefaultUseProfile]; exists {
		return defaultProfile
	}

	return nil
}

// AddDeviceProfile adds a custom device profile
func (dpo *DeviceProfileOptimizer) AddDeviceProfile(profile *DeviceProfile) {
	profile.UpdatedAt = time.Now()
	dpo.deviceProfiles[profile.ID] = profile
	dpo.logger.Debug("Added device profile", "id", profile.ID, "name", profile.Name)
}

// AddUseProfile adds a custom use case profile
func (dpo *DeviceProfileOptimizer) AddUseProfile(profile *UseProfile) {
	dpo.useProfiles[profile.ID] = profile
	dpo.logger.Debug("Added use case profile", "id", profile.ID, "name", profile.Name)
}

// GetDeviceProfiles returns all available device profiles
func (dpo *DeviceProfileOptimizer) GetDeviceProfiles() map[string]*DeviceProfile {
	return dpo.deviceProfiles
}

// GetUseProfiles returns all available use case profiles
func (dpo *DeviceProfileOptimizer) GetUseProfiles() map[string]*UseProfile {
	return dpo.useProfiles
}

// initializeDefaultProfiles creates standard device and use case profiles
func (dpo *DeviceProfileOptimizer) initializeDefaultProfiles() {
	// Initialize default device profiles
	dpo.initializeDeviceProfiles()

	// Initialize default use case profiles
	dpo.initializeUseProfiles()
}

func (dpo *DeviceProfileOptimizer) initializeDeviceProfiles() {
	// Mobile device profile
	mobileProfile := &DeviceProfile{
		ID:              "mobile_standard",
		Name:            "Standard Mobile Device",
		Category:        DeviceMobile,
		Description:     "Optimized for smartphones and mobile devices",
		MaxResolution:   Resolution{Width: 1280, Height: 720, Name: "720p"},
		SupportedCodecs: []string{"h264", "h265"},
		OptimalBitrates: map[string]int{
			"640x360":  800,
			"854x480":  1200,
			"1280x720": 2500,
		},
		MaxFrameRate:             30.0,
		DecodingCapability:       "hardware",
		MemoryConstraints:        MemoryProfile{Available: 2048, Recommended: 1024, MaxBuffer: 256},
		ProcessingPower:          ProcessingProfile{Capability: "medium", Cores: 4, Frequency: 2.0, Architecture: "arm", GPUAcceleration: true},
		PreferredSegmentDuration: 4,
		BufferingStrategy:        "aggressive",
		AdaptiveBitrate:          true,
		StartupOptimization:      StartupProfile{MaxStartupTime: 3 * time.Second, PrefetchSegments: 2, FastStart: true, InitialBitrate: 800},
		QualityPriority:          QualityBalanced,
		LatencyTolerance:         LatencyNormal,
		NetworkProfile:           NetworkProfile{TypicalBandwidth: 5000, Variability: 0.4, Reliability: 0.8, LatencyMs: 50},
		UpdatedAt:                time.Now(),
	}
	dpo.deviceProfiles[mobileProfile.ID] = mobileProfile

	// Desktop device profile
	desktopProfile := &DeviceProfile{
		ID:              "desktop_standard",
		Name:            "Standard Desktop",
		Category:        DeviceDesktop,
		Description:     "Optimized for desktop computers and laptops",
		MaxResolution:   Resolution{Width: 1920, Height: 1080, Name: "1080p"},
		SupportedCodecs: []string{"h264", "h265", "av1"},
		OptimalBitrates: map[string]int{
			"1280x720":  3000,
			"1920x1080": 5000,
			"2560x1440": 8000,
		},
		MaxFrameRate:             60.0,
		DecodingCapability:       "hardware",
		MemoryConstraints:        MemoryProfile{Available: 8192, Recommended: 4096, MaxBuffer: 1024},
		ProcessingPower:          ProcessingProfile{Capability: "high", Cores: 8, Frequency: 3.0, Architecture: "x86", GPUAcceleration: true},
		PreferredSegmentDuration: 6,
		BufferingStrategy:        "balanced",
		AdaptiveBitrate:          true,
		StartupOptimization:      StartupProfile{MaxStartupTime: 2 * time.Second, PrefetchSegments: 3, FastStart: true, InitialBitrate: 2000},
		QualityPriority:          QualityHigh,
		LatencyTolerance:         LatencyLow,
		NetworkProfile:           NetworkProfile{TypicalBandwidth: 25000, Variability: 0.2, Reliability: 0.9, LatencyMs: 20},
		UpdatedAt:                time.Now(),
	}
	dpo.deviceProfiles[desktopProfile.ID] = desktopProfile

	// TV/Smart TV profile
	tvProfile := &DeviceProfile{
		ID:              "tv_4k",
		Name:            "4K Smart TV",
		Category:        DeviceTV,
		Description:     "Optimized for 4K smart TVs and streaming devices",
		MaxResolution:   Resolution{Width: 3840, Height: 2160, Name: "4K"},
		SupportedCodecs: []string{"h264", "h265"},
		OptimalBitrates: map[string]int{
			"1920x1080": 6000,
			"2560x1440": 10000,
			"3840x2160": 18000,
		},
		MaxFrameRate:             60.0,
		DecodingCapability:       "hardware",
		MemoryConstraints:        MemoryProfile{Available: 4096, Recommended: 2048, MaxBuffer: 512},
		ProcessingPower:          ProcessingProfile{Capability: "high", Cores: 4, Frequency: 1.8, Architecture: "arm", GPUAcceleration: true},
		PreferredSegmentDuration: 6,
		BufferingStrategy:        "conservative",
		AdaptiveBitrate:          true,
		StartupOptimization:      StartupProfile{MaxStartupTime: 4 * time.Second, PrefetchSegments: 3, FastStart: true, InitialBitrate: 3000},
		QualityPriority:          QualityHigh,
		LatencyTolerance:         LatencyNormal,
		NetworkProfile:           NetworkProfile{TypicalBandwidth: 50000, Variability: 0.3, Reliability: 0.9, LatencyMs: 30},
		UpdatedAt:                time.Now(),
	}
	dpo.deviceProfiles[tvProfile.ID] = tvProfile

	// Balanced profile (default)
	balancedProfile := &DeviceProfile{
		ID:              "balanced_device",
		Name:            "Balanced Universal Device",
		Category:        DeviceDesktop,
		Description:     "Balanced settings for broad compatibility",
		MaxResolution:   Resolution{Width: 1920, Height: 1080, Name: "1080p"},
		SupportedCodecs: []string{"h264"},
		OptimalBitrates: map[string]int{
			"854x480":   1500,
			"1280x720":  2500,
			"1920x1080": 4000,
		},
		MaxFrameRate:             30.0,
		DecodingCapability:       "software",
		MemoryConstraints:        MemoryProfile{Available: 4096, Recommended: 2048, MaxBuffer: 512},
		ProcessingPower:          ProcessingProfile{Capability: "medium", Cores: 4, Frequency: 2.5, Architecture: "x86", GPUAcceleration: false},
		PreferredSegmentDuration: 4,
		BufferingStrategy:        "balanced",
		AdaptiveBitrate:          true,
		StartupOptimization:      StartupProfile{MaxStartupTime: 3 * time.Second, PrefetchSegments: 2, FastStart: true, InitialBitrate: 1500},
		QualityPriority:          QualityBalanced,
		LatencyTolerance:         LatencyNormal,
		NetworkProfile:           NetworkProfile{TypicalBandwidth: 10000, Variability: 0.3, Reliability: 0.8, LatencyMs: 40},
		UpdatedAt:                time.Now(),
	}
	dpo.deviceProfiles[balancedProfile.ID] = balancedProfile
}

func (dpo *DeviceProfileOptimizer) initializeUseProfiles() {
	// Standard streaming profile
	standardProfile := &UseProfile{
		ID:                 "standard_streaming",
		Name:               "Standard Video Streaming",
		Description:        "General video streaming with balanced quality and efficiency",
		ViewingPattern:     ViewingLinear,
		InteractionLevel:   InteractionLow,
		QualityExpectation: QualityStandard,
		OptimizeFor:        []OptimizationGoal{OptimizeQuality, OptimizeBandwidth},
		Constraints:        UseCaseConstraints{MaxEncodingTime: 30 * time.Minute},
		EncodingPreferences: EncodingPreferences{
			PreferHardware: true,
			AllowedPresets: []string{"fast", "medium", "slow"},
			QualityRange:   QualityRange{Min: 18, Max: 28},
			BitrateRange:   BitrateRange{Min: 500, Max: 10000},
		},
	}
	dpo.useProfiles[standardProfile.ID] = standardProfile

	// Live streaming profile
	liveProfile := &UseProfile{
		ID:                 "live_streaming",
		Name:               "Live Video Streaming",
		Description:        "Optimized for real-time live video streaming",
		ViewingPattern:     ViewingLive,
		InteractionLevel:   InteractionMedium,
		QualityExpectation: QualityStandard,
		OptimizeFor:        []OptimizationGoal{OptimizeLatency, OptimizeSpeed},
		Constraints:        UseCaseConstraints{MaxEncodingTime: 2 * time.Second},
		EncodingPreferences: EncodingPreferences{
			PreferHardware: true,
			AllowedPresets: []string{"ultrafast", "fast"},
			QualityRange:   QualityRange{Min: 20, Max: 30},
			BitrateRange:   BitrateRange{Min: 1000, Max: 8000},
		},
	}
	dpo.useProfiles[liveProfile.ID] = liveProfile

	// High quality profile
	hqProfile := &UseProfile{
		ID:                 "high_quality",
		Name:               "High Quality Video",
		Description:        "Maximum quality for premium content",
		ViewingPattern:     ViewingLinear,
		InteractionLevel:   InteractionLow,
		QualityExpectation: QualityUltra,
		OptimizeFor:        []OptimizationGoal{OptimizeQuality},
		Constraints:        UseCaseConstraints{MaxEncodingTime: 2 * time.Hour},
		EncodingPreferences: EncodingPreferences{
			PreferHardware: false,
			AllowedPresets: []string{"slow", "veryslow"},
			QualityRange:   QualityRange{Min: 15, Max: 22},
			BitrateRange:   BitrateRange{Min: 2000, Max: 25000},
		},
	}
	dpo.useProfiles[hqProfile.ID] = hqProfile

	// Interactive profile
	interactiveProfile := &UseProfile{
		ID:                 "interactive_video",
		Name:               "Interactive Video Content",
		Description:        "Optimized for interactive and seekable content",
		ViewingPattern:     ViewingInteractive,
		InteractionLevel:   InteractionHigh,
		QualityExpectation: QualityStandard,
		OptimizeFor:        []OptimizationGoal{OptimizeStartup, OptimizeLatency},
		Constraints:        UseCaseConstraints{MaxEncodingTime: 15 * time.Minute},
		EncodingPreferences: EncodingPreferences{
			PreferHardware: true,
			AllowedPresets: []string{"fast", "medium"},
			QualityRange:   QualityRange{Min: 18, Max: 26},
			BitrateRange:   BitrateRange{Min: 800, Max: 8000},
		},
	}
	dpo.useProfiles[interactiveProfile.ID] = interactiveProfile

	// Mobile optimized profile
	mobileProfile := &UseProfile{
		ID:                 "mobile_optimized",
		Name:               "Mobile Optimized",
		Description:        "Optimized for mobile devices and limited bandwidth",
		ViewingPattern:     ViewingLinear,
		InteractionLevel:   InteractionLow,
		QualityExpectation: QualityStandard,
		OptimizeFor:        []OptimizationGoal{OptimizeBandwidth, OptimizeSize},
		Constraints:        UseCaseConstraints{MaxBandwidth: 3000, MaxEncodingTime: 20 * time.Minute},
		EncodingPreferences: EncodingPreferences{
			PreferHardware: true,
			AllowedPresets: []string{"fast", "medium"},
			QualityRange:   QualityRange{Min: 22, Max: 30},
			BitrateRange:   BitrateRange{Min: 300, Max: 3000},
		},
	}
	dpo.useProfiles[mobileProfile.ID] = mobileProfile
}
