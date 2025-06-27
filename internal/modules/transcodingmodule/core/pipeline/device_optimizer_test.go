package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceProfileOptimizer_NewDeviceProfileOptimizer(t *testing.T) {
	logger := hclog.NewNullLogger()
	fastStartOptimizer := NewFastStartOptimizer(logger)
	adaptiveSizer := NewAdaptiveSegmentSizer(logger, fastStartOptimizer)

	optimizer := NewDeviceProfileOptimizer(logger, fastStartOptimizer, adaptiveSizer)

	assert.NotNil(t, optimizer)
	assert.NotNil(t, optimizer.deviceProfiles)
	assert.NotNil(t, optimizer.useProfiles)
	assert.True(t, len(optimizer.deviceProfiles) > 0, "Should have default device profiles")
	assert.True(t, len(optimizer.useProfiles) > 0, "Should have default use profiles")
}

func TestDeviceProfileOptimizer_DefaultProfiles(t *testing.T) {
	logger := hclog.NewNullLogger()
	fastStartOptimizer := NewFastStartOptimizer(logger)
	adaptiveSizer := NewAdaptiveSegmentSizer(logger, fastStartOptimizer)

	optimizer := NewDeviceProfileOptimizer(logger, fastStartOptimizer, adaptiveSizer)

	// Check default device profiles
	deviceProfiles := optimizer.GetDeviceProfiles()
	expectedDeviceProfiles := []string{"mobile_standard", "desktop_standard", "tv_4k", "balanced_device"}

	for _, profileID := range expectedDeviceProfiles {
		profile, exists := deviceProfiles[profileID]
		assert.True(t, exists, "Should have device profile: %s", profileID)
		assert.NotEmpty(t, profile.Name)
		assert.NotEmpty(t, profile.Description)
		assert.True(t, len(profile.SupportedCodecs) > 0)
		assert.True(t, profile.MaxResolution.Width > 0)
		assert.True(t, profile.MaxResolution.Height > 0)
	}

	// Check default use profiles
	useProfiles := optimizer.GetUseProfiles()
	expectedUseProfiles := []string{"standard_streaming", "live_streaming", "high_quality", "interactive_video", "mobile_optimized"}

	for _, profileID := range expectedUseProfiles {
		profile, exists := useProfiles[profileID]
		assert.True(t, exists, "Should have use profile: %s", profileID)
		assert.NotEmpty(t, profile.Name)
		assert.NotEmpty(t, profile.Description)
		assert.True(t, len(profile.OptimizeFor) > 0)
	}
}

func TestDeviceProfileOptimizer_OptimizeForDevice(t *testing.T) {
	logger := hclog.NewNullLogger()
	fastStartOptimizer := NewFastStartOptimizer(logger)
	adaptiveSizer := NewAdaptiveSegmentSizer(logger, fastStartOptimizer)

	optimizer := NewDeviceProfileOptimizer(logger, fastStartOptimizer, adaptiveSizer)

	tests := []struct {
		name           string
		deviceProfile  string
		useProfile     string
		expectedCodec  string
		expectedMinRes int // minimum expected width
		expectedMaxRes int // maximum expected width
	}{
		{
			name:           "mobile standard streaming",
			deviceProfile:  "mobile_standard",
			useProfile:     "standard_streaming",
			expectedCodec:  "libx264",
			expectedMinRes: 320,
			expectedMaxRes: 1280,
		},
		{
			name:           "desktop high quality",
			deviceProfile:  "desktop_standard",
			useProfile:     "high_quality",
			expectedCodec:  "libx265", // Should prefer H.265 for high quality
			expectedMinRes: 720,
			expectedMaxRes: 1920,
		},
		{
			name:           "TV 4K streaming",
			deviceProfile:  "tv_4k",
			useProfile:     "standard_streaming",
			expectedCodec:  "libx264",
			expectedMinRes: 1080,
			expectedMaxRes: 3840,
		},
		{
			name:           "mobile live streaming",
			deviceProfile:  "mobile_standard",
			useProfile:     "live_streaming",
			expectedCodec:  "libx264",
			expectedMinRes: 320,
			expectedMaxRes: 1280,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := OptimizationRequest{
				DeviceProfileID: tt.deviceProfile,
				UseProfileID:    tt.useProfile,
				ContentType:     "video",
				TargetFormats:   []string{"mp4"},
			}

			ctx := context.Background()
			params, err := optimizer.OptimizeForDevice(ctx, req)

			assert.NoError(t, err)
			assert.NotNil(t, params)
			assert.Equal(t, tt.expectedCodec, params.Codec)
			assert.True(t, params.Resolution.Width >= tt.expectedMinRes,
				"Resolution width %d should be >= %d", params.Resolution.Width, tt.expectedMinRes)
			assert.True(t, params.Resolution.Width <= tt.expectedMaxRes,
				"Resolution width %d should be <= %d", params.Resolution.Width, tt.expectedMaxRes)
			assert.True(t, params.Bitrate > 0, "Bitrate should be positive")
			assert.True(t, params.Quality >= 15 && params.Quality <= 35, "Quality should be in reasonable range")
			assert.True(t, params.OptimizationScore >= 0.0 && params.OptimizationScore <= 1.0, "Score should be 0-1")
		})
	}
}

func TestDeviceProfileOptimizer_OptimizeResolution(t *testing.T) {
	logger := hclog.NewNullLogger()
	fastStartOptimizer := NewFastStartOptimizer(logger)
	adaptiveSizer := NewAdaptiveSegmentSizer(logger, fastStartOptimizer)

	optimizer := NewDeviceProfileOptimizer(logger, fastStartOptimizer, adaptiveSizer)

	// Mobile device should limit resolution
	mobileProfile := optimizer.getDeviceProfile("mobile_standard")
	standardUse := optimizer.getUseProfile("standard_streaming")

	resolution := optimizer.optimizeResolution(mobileProfile, standardUse, nil)
	assert.True(t, resolution.Width <= 1280, "Mobile should not exceed 720p width")
	assert.True(t, resolution.Height <= 720, "Mobile should not exceed 720p height")

	// Desktop can handle higher resolutions
	desktopProfile := optimizer.getDeviceProfile("desktop_standard")
	resolution = optimizer.optimizeResolution(desktopProfile, standardUse, nil)
	assert.True(t, resolution.Width <= 1920, "Desktop should handle up to 1080p")
	assert.True(t, resolution.Height <= 1080, "Desktop should handle up to 1080p")

	// Content with lower resolution shouldn't be upscaled for standard quality
	content := &ContentAnalysis{
		Resolution: Resolution{Width: 854, Height: 480, Name: "480p"},
	}
	resolution = optimizer.optimizeResolution(desktopProfile, standardUse, content)
	assert.Equal(t, 854, resolution.Width, "Should not upscale 480p content for standard quality")
	assert.Equal(t, 480, resolution.Height, "Should not upscale 480p content for standard quality")
}

func TestDeviceProfileOptimizer_OptimizeBitrate(t *testing.T) {
	logger := hclog.NewNullLogger()
	fastStartOptimizer := NewFastStartOptimizer(logger)
	adaptiveSizer := NewAdaptiveSegmentSizer(logger, fastStartOptimizer)

	optimizer := NewDeviceProfileOptimizer(logger, fastStartOptimizer, adaptiveSizer)

	mobileProfile := optimizer.getDeviceProfile("mobile_standard")

	tests := []struct {
		name               string
		useProfile         string
		resolution         Resolution
		expectedMinBitrate int
		expectedMaxBitrate int
	}{
		{
			name:               "standard quality 720p",
			useProfile:         "standard_streaming",
			resolution:         Resolution{Width: 1280, Height: 720, Name: "720p"},
			expectedMinBitrate: 1500,
			expectedMaxBitrate: 4000,
		},
		{
			name:               "high quality 720p",
			useProfile:         "high_quality",
			resolution:         Resolution{Width: 1280, Height: 720, Name: "720p"},
			expectedMinBitrate: 3000,
			expectedMaxBitrate: 6000,
		},
		{
			name:               "mobile optimized 480p",
			useProfile:         "mobile_optimized",
			resolution:         Resolution{Width: 854, Height: 480, Name: "480p"},
			expectedMinBitrate: 300,
			expectedMaxBitrate: 2000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useProfile := optimizer.getUseProfile(tt.useProfile)
			bitrate := optimizer.optimizeBitrate(mobileProfile, useProfile, nil, tt.resolution)

			assert.True(t, bitrate >= tt.expectedMinBitrate,
				"Bitrate %d should be >= %d", bitrate, tt.expectedMinBitrate)
			assert.True(t, bitrate <= tt.expectedMaxBitrate,
				"Bitrate %d should be <= %d", bitrate, tt.expectedMaxBitrate)
		})
	}
}

func TestDeviceProfileOptimizer_OptimizeCodec(t *testing.T) {
	logger := hclog.NewNullLogger()
	fastStartOptimizer := NewFastStartOptimizer(logger)
	adaptiveSizer := NewAdaptiveSegmentSizer(logger, fastStartOptimizer)

	optimizer := NewDeviceProfileOptimizer(logger, fastStartOptimizer, adaptiveSizer)

	// Test device with H.265 support
	desktopProfile := optimizer.getDeviceProfile("desktop_standard")
	hqProfile := optimizer.getUseProfile("high_quality")

	codec, profile, level := optimizer.optimizeCodec(desktopProfile, hqProfile, nil)
	assert.Equal(t, "libx265", codec, "Should use H.265 for high quality on capable device")
	assert.Equal(t, "main", profile, "Should use main profile for H.265")

	// Test mobile device (also supports H.265 for high quality)
	mobileProfile := optimizer.getDeviceProfile("mobile_standard")
	codec, profile, level = optimizer.optimizeCodec(mobileProfile, hqProfile, nil)
	// Mobile also supports H.265 for high quality, so it will use it
	assert.Equal(t, "libx265", codec, "Mobile should also use H.265 for high quality")
	assert.NotEmpty(t, level, "Should have a level specified")

	// Test with standard quality (should prefer H.264)
	standardProfile := optimizer.getUseProfile("standard_streaming")
	codec, profile, level = optimizer.optimizeCodec(mobileProfile, standardProfile, nil)
	assert.Equal(t, "libx264", codec, "Should use H.264 for standard quality")
	assert.NotEmpty(t, profile, "Should have a profile specified")
}

func TestDeviceProfileOptimizer_OptimizePreset(t *testing.T) {
	logger := hclog.NewNullLogger()
	fastStartOptimizer := NewFastStartOptimizer(logger)
	adaptiveSizer := NewAdaptiveSegmentSizer(logger, fastStartOptimizer)

	optimizer := NewDeviceProfileOptimizer(logger, fastStartOptimizer, adaptiveSizer)

	mobileProfile := optimizer.getDeviceProfile("mobile_standard")

	// Live streaming should use fast preset
	liveProfile := optimizer.getUseProfile("live_streaming")
	preset, tune := optimizer.optimizePreset(mobileProfile, liveProfile, nil)
	assert.Equal(t, "ultrafast", preset, "Live streaming should use ultrafast preset")
	assert.Equal(t, "zerolatency", tune, "Live streaming should use zerolatency tune")

	// High quality should use slower preset
	hqProfile := optimizer.getUseProfile("high_quality")
	preset, tune = optimizer.optimizePreset(mobileProfile, hqProfile, nil)
	assert.Contains(t, []string{"slow", "veryslow"}, preset, "High quality should use slow preset")
}

func TestDeviceProfileOptimizer_ContentAnalysis(t *testing.T) {
	logger := hclog.NewNullLogger()
	fastStartOptimizer := NewFastStartOptimizer(logger)
	adaptiveSizer := NewAdaptiveSegmentSizer(logger, fastStartOptimizer)

	optimizer := NewDeviceProfileOptimizer(logger, fastStartOptimizer, adaptiveSizer)

	// Create a temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.mp4")

	// Create a dummy file for testing
	err := os.WriteFile(testFile, []byte("dummy video content"), 0644)
	require.NoError(t, err)

	ctx := context.Background()

	// Test with actual file path (will fail analysis but should handle gracefully)
	req := OptimizationRequest{
		DeviceProfileID: "mobile_standard",
		UseProfileID:    "standard_streaming",
		ContentType:     "video",
		InputPath:       testFile,
		TargetFormats:   []string{"mp4"},
	}

	params, err := optimizer.OptimizeForDevice(ctx, req)
	assert.NoError(t, err, "Should handle content analysis failure gracefully")
	assert.NotNil(t, params)
}

func TestDeviceProfileOptimizer_OptimizationFlags(t *testing.T) {
	logger := hclog.NewNullLogger()
	fastStartOptimizer := NewFastStartOptimizer(logger)
	adaptiveSizer := NewAdaptiveSegmentSizer(logger, fastStartOptimizer)

	optimizer := NewDeviceProfileOptimizer(logger, fastStartOptimizer, adaptiveSizer)

	desktopProfile := optimizer.getDeviceProfile("desktop_standard")

	// Interactive content should enable fast start
	interactiveProfile := optimizer.getUseProfile("interactive_video")
	fastStart := optimizer.shouldEnableFastStart(desktopProfile, interactiveProfile)
	assert.True(t, fastStart, "Interactive content should enable fast start")

	// Live content should enable low latency
	liveProfile := optimizer.getUseProfile("live_streaming")
	lowLatency := optimizer.shouldEnableLowLatency(desktopProfile, liveProfile)
	assert.True(t, lowLatency, "Live content should enable low latency")

	// Standard streaming should enable adaptive streaming
	standardProfile := optimizer.getUseProfile("standard_streaming")
	adaptive := optimizer.shouldEnableAdaptiveStreaming(desktopProfile, standardProfile)
	assert.True(t, adaptive, "Standard streaming should enable adaptive bitrate")
}

func TestDeviceProfileOptimizer_CustomProfiles(t *testing.T) {
	logger := hclog.NewNullLogger()
	fastStartOptimizer := NewFastStartOptimizer(logger)
	adaptiveSizer := NewAdaptiveSegmentSizer(logger, fastStartOptimizer)

	optimizer := NewDeviceProfileOptimizer(logger, fastStartOptimizer, adaptiveSizer)

	// Add custom device profile
	customDevice := &DeviceProfile{
		ID:              "custom_device",
		Name:            "Custom Test Device",
		Category:        DeviceEmbedded,
		Description:     "Test device for custom optimization",
		MaxResolution:   Resolution{Width: 640, Height: 480, Name: "480p"},
		SupportedCodecs: []string{"h264"},
		OptimalBitrates: map[string]int{
			"640x480": 800,
		},
		MaxFrameRate:             15.0,
		DecodingCapability:       "software",
		MemoryConstraints:        MemoryProfile{Available: 512, Recommended: 256, MaxBuffer: 64},
		ProcessingPower:          ProcessingProfile{Capability: "low", Cores: 1, Frequency: 1.0, Architecture: "arm", GPUAcceleration: false},
		PreferredSegmentDuration: 2,
		AdaptiveBitrate:          false,
		QualityPriority:          QualityEfficient,
		LatencyTolerance:         LatencyHigh,
		NetworkProfile:           NetworkProfile{TypicalBandwidth: 1000, Variability: 0.6, Reliability: 0.6, LatencyMs: 100},
	}

	optimizer.AddDeviceProfile(customDevice)

	// Test optimization with custom profile
	req := OptimizationRequest{
		DeviceProfileID: "custom_device",
		UseProfileID:    "standard_streaming",
		ContentType:     "video",
		TargetFormats:   []string{"mp4"},
	}

	ctx := context.Background()
	params, err := optimizer.OptimizeForDevice(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, params)
	assert.True(t, params.Resolution.Width <= 640, "Should respect custom device max resolution")
	assert.True(t, params.FrameRate <= 15.0, "Should respect custom device max frame rate")
	assert.Equal(t, "libx264", params.Codec, "Should use supported codec")
}

func TestDeviceProfileOptimizer_OptimizationScore(t *testing.T) {
	logger := hclog.NewNullLogger()
	fastStartOptimizer := NewFastStartOptimizer(logger)
	adaptiveSizer := NewAdaptiveSegmentSizer(logger, fastStartOptimizer)

	optimizer := NewDeviceProfileOptimizer(logger, fastStartOptimizer, adaptiveSizer)

	device := optimizer.getDeviceProfile("desktop_standard")
	useCase := optimizer.getUseProfile("standard_streaming")

	// Test well-matched parameters
	goodParams := &OptimizationParameters{
		Resolution:        Resolution{Width: 1280, Height: 720, Name: "720p"},
		Codec:             "libx264",
		Bitrate:           3000,
		Quality:           23,
		FastStart:         true,
		AdaptiveStreaming: true,
		LowLatency:        false,
	}

	score := optimizer.calculateOptimizationScore(goodParams, device, useCase)
	assert.True(t, score > 0.7, "Well-matched parameters should get high score: %f", score)

	// Test poorly matched parameters
	badParams := &OptimizationParameters{
		Resolution:        Resolution{Width: 3840, Height: 2160, Name: "4K"}, // Exceeds device capability
		Codec:             "libav1",                                          // Not supported
		Bitrate:           50000,                                             // Too high
		Quality:           10,                                                // Outside range
		FastStart:         false,                                             // Doesn't match device preference
		AdaptiveStreaming: false,                                             // Doesn't match device capability
		LowLatency:        true,                                              // Doesn't match device tolerance
	}

	score = optimizer.calculateOptimizationScore(badParams, device, useCase)
	assert.True(t, score < 0.6, "Poorly matched parameters should get lower score: %f", score)
}

func TestDeviceProfileOptimizer_ErrorHandling(t *testing.T) {
	logger := hclog.NewNullLogger()
	fastStartOptimizer := NewFastStartOptimizer(logger)
	adaptiveSizer := NewAdaptiveSegmentSizer(logger, fastStartOptimizer)

	optimizer := NewDeviceProfileOptimizer(logger, fastStartOptimizer, adaptiveSizer)

	ctx := context.Background()

	// Test with non-existent device profile (should fall back to default)
	req := OptimizationRequest{
		DeviceProfileID: "non_existent_device",
		UseProfileID:    "standard_streaming",
		ContentType:     "video",
		TargetFormats:   []string{"mp4"},
	}

	params, err := optimizer.OptimizeForDevice(ctx, req)
	// Should use default device profile instead of erroring
	assert.NoError(t, err, "Should fall back to default device profile")
	assert.NotNil(t, params)

	// Test with non-existent use profile (should fall back to default)
	req = OptimizationRequest{
		DeviceProfileID: "mobile_standard",
		UseProfileID:    "non_existent_use",
		ContentType:     "video",
		TargetFormats:   []string{"mp4"},
	}

	params, err = optimizer.OptimizeForDevice(ctx, req)
	// Should use default use profile instead of erroring
	assert.NoError(t, err, "Should fall back to default use profile")
	assert.NotNil(t, params)
}

func TestDeviceProfileOptimizer_SegmentDurationOptimization(t *testing.T) {
	logger := hclog.NewNullLogger()
	fastStartOptimizer := NewFastStartOptimizer(logger)
	adaptiveSizer := NewAdaptiveSegmentSizer(logger, fastStartOptimizer)

	optimizer := NewDeviceProfileOptimizer(logger, fastStartOptimizer, adaptiveSizer)

	// Test different viewing patterns
	device := optimizer.getDeviceProfile("mobile_standard")

	// Live streaming should use shorter segments
	liveProfile := optimizer.getUseProfile("live_streaming")
	segmentDuration := optimizer.optimizeSegmentDuration(device, liveProfile, nil)
	assert.Equal(t, 2, segmentDuration, "Live streaming should use 2-second segments")

	// Interactive content should use shorter segments
	interactiveProfile := optimizer.getUseProfile("interactive_video")
	segmentDuration = optimizer.optimizeSegmentDuration(device, interactiveProfile, nil)
	assert.Equal(t, 2, segmentDuration, "Interactive content should use 2-second segments")

	// Standard streaming should use longer segments
	standardProfile := optimizer.getUseProfile("standard_streaming")
	segmentDuration = optimizer.optimizeSegmentDuration(device, standardProfile, nil)
	assert.True(t, segmentDuration >= 4, "Standard streaming should use longer segments")
}
