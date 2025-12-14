package streaming

import (
	"testing"
)

func TestAdaptiveProfile_Fields(t *testing.T) {
	profile := AdaptiveProfile{
		ID:              "1080p-10m",
		DisplayName:     "1080p (10 Mbps)",
		Width:           1920,
		Height:          1080,
		VideoBitrate:    10_000_000,
		VideoMaxRate:    11_000_000,
		VideoBufSize:    20_000_000,
		AudioBitrate:    256_000,
		AudioChannels:   6,
		AudioSampleRate: 48000,
		PreserveMultiCh: true,
		AudioCodec:      "ac3",
		PreferredCodec:  "h264",
		Preset:          "medium",
		CRF:             21,
		EnableHWAccel:   true,
		EnableFastStart: true,
		SegmentDuration: 2,
		GOPSize:         48,
		FrameRate:       24.0,
		AspectRatio:     "16:9",
		MinNetworkMbps:  12.5,
		MinScreenWidth:  1920,
		MinScreenHeight: 1080,
		RecommendedFor:  []string{"desktop", "tv"},
		Description:     "Standard Full HD",
		QualityTier:     "high",
	}

	if profile.ID != "1080p-10m" {
		t.Errorf("expected ID '1080p-10m', got %s", profile.ID)
	}
	if profile.Width != 1920 {
		t.Errorf("expected width 1920, got %d", profile.Width)
	}
	if profile.Height != 1080 {
		t.Errorf("expected height 1080, got %d", profile.Height)
	}
	if profile.VideoBitrate != 10_000_000 {
		t.Errorf("expected video bitrate 10000000, got %d", profile.VideoBitrate)
	}
	if profile.QualityTier != "high" {
		t.Errorf("expected quality tier 'high', got %s", profile.QualityTier)
	}
	if len(profile.RecommendedFor) != 2 {
		t.Errorf("expected 2 recommended devices, got %d", len(profile.RecommendedFor))
	}
}

func TestABRVariant_Fields(t *testing.T) {
	variant := ABRVariant{
		ID:        "4k-60m",
		Bandwidth: 60_000_000,
		Width:     3840,
		Height:    2160,
		Codecs:    "avc1.640033,mp4a.40.2",
	}

	if variant.ID != "4k-60m" {
		t.Errorf("expected ID '4k-60m', got %s", variant.ID)
	}
	if variant.Bandwidth != 60_000_000 {
		t.Errorf("expected bandwidth 60000000, got %d", variant.Bandwidth)
	}
	if variant.Width != 3840 {
		t.Errorf("expected width 3840, got %d", variant.Width)
	}
	if variant.Height != 2160 {
		t.Errorf("expected height 2160, got %d", variant.Height)
	}
	if variant.Codecs != "avc1.640033,mp4a.40.2" {
		t.Errorf("expected codecs 'avc1.640033,mp4a.40.2', got %s", variant.Codecs)
	}
}

func TestClientCapabilities_Fields(t *testing.T) {
	caps := ClientCapabilities{
		NetworkSpeedMbps:     100.0,
		ConnectionType:       "ethernet",
		IsMetered:            false,
		DeviceType:           DeviceTypeDesktop,
		ScreenWidth:          2560,
		ScreenHeight:         1440,
		PixelRatio:           2.0,
		SupportedCodecs:      []string{"h264", "h265", "av1"},
		HardwareAcceleration: true,
	}

	if caps.NetworkSpeedMbps != 100.0 {
		t.Errorf("expected network speed 100.0, got %f", caps.NetworkSpeedMbps)
	}
	if caps.DeviceType != DeviceTypeDesktop {
		t.Errorf("expected device type 'desktop', got %s", caps.DeviceType)
	}
	if caps.ScreenWidth != 2560 {
		t.Errorf("expected screen width 2560, got %d", caps.ScreenWidth)
	}
	if caps.ScreenHeight != 1440 {
		t.Errorf("expected screen height 1440, got %d", caps.ScreenHeight)
	}
	if len(caps.SupportedCodecs) != 3 {
		t.Errorf("expected 3 supported codecs, got %d", len(caps.SupportedCodecs))
	}
}

func TestQualityRecommendation_Fields(t *testing.T) {
	profile := &AdaptiveProfile{
		ID:          "1080p-20m",
		DisplayName: "1080p (20 Mbps)",
		Width:       1920,
		Height:      1080,
	}

	rec := QualityRecommendation{
		Profile: profile,
		Score:   0.95,
		Reason:  "Best match for screen and network",
	}

	if rec.Profile != profile {
		t.Error("expected profile reference to match")
	}
	if rec.Score != 0.95 {
		t.Errorf("expected score 0.95, got %f", rec.Score)
	}
	if rec.Reason != "Best match for screen and network" {
		t.Errorf("unexpected reason: %s", rec.Reason)
	}
}

func TestQualityConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"Quality360p", Quality360p, "360p"},
		{"Quality480p", Quality480p, "480p"},
		{"Quality720p2m", Quality720p2m, "720p-2m"},
		{"Quality720p4m", Quality720p4m, "720p-4m"},
		{"Quality1080p4m", Quality1080p4m, "1080p-4m"},
		{"Quality1080p10m", Quality1080p10m, "1080p-10m"},
		{"Quality1080p20m", Quality1080p20m, "1080p-20m"},
		{"Quality1080p40m", Quality1080p40m, "1080p-40m"},
		{"Quality1080p60m", Quality1080p60m, "1080p-60m"},
		{"Quality4k15m", Quality4k15m, "4k-15m"},
		{"Quality4k20m", Quality4k20m, "4k-20m"},
		{"Quality4k25m", Quality4k25m, "4k-25m"},
		{"Quality4k40m", Quality4k40m, "4k-40m"},
		{"Quality4k60m", Quality4k60m, "4k-60m"},
		{"Quality4k100m", Quality4k100m, "4k-100m"},
		{"Quality4k200m", Quality4k200m, "4k-200m"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.constant != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, tc.constant)
			}
		})
	}
}

func TestQualityTierConstants(t *testing.T) {
	if QualityTierLow != "low" {
		t.Errorf("expected 'low', got %s", QualityTierLow)
	}
	if QualityTierMedium != "medium" {
		t.Errorf("expected 'medium', got %s", QualityTierMedium)
	}
	if QualityTierHigh != "high" {
		t.Errorf("expected 'high', got %s", QualityTierHigh)
	}
	if QualityTierUltra != "ultra" {
		t.Errorf("expected 'ultra', got %s", QualityTierUltra)
	}
}

func TestDeviceTypeConstants(t *testing.T) {
	if DeviceTypeMobile != "mobile" {
		t.Errorf("expected 'mobile', got %s", DeviceTypeMobile)
	}
	if DeviceTypeTablet != "tablet" {
		t.Errorf("expected 'tablet', got %s", DeviceTypeTablet)
	}
	if DeviceTypeDesktop != "desktop" {
		t.Errorf("expected 'desktop', got %s", DeviceTypeDesktop)
	}
	if DeviceTypeTV != "tv" {
		t.Errorf("expected 'tv', got %s", DeviceTypeTV)
	}
}

func TestAdaptiveProfile_3DFields(t *testing.T) {
	profile := AdaptiveProfile{
		ID:         "4k-3d-sbs",
		Is3D:       true,
		StereoMode: "sbs",
	}

	if !profile.Is3D {
		t.Error("expected Is3D to be true")
	}
	if profile.StereoMode != "sbs" {
		t.Errorf("expected stereo mode 'sbs', got %s", profile.StereoMode)
	}
}

func TestClientCapabilities_MeteredConnection(t *testing.T) {
	// Metered connection (cellular)
	meteredCaps := ClientCapabilities{
		NetworkSpeedMbps: 50.0,
		ConnectionType:   "4g",
		IsMetered:        true,
		DeviceType:       DeviceTypeMobile,
	}

	if !meteredCaps.IsMetered {
		t.Error("expected IsMetered to be true")
	}
	if meteredCaps.ConnectionType != "4g" {
		t.Errorf("expected connection type '4g', got %s", meteredCaps.ConnectionType)
	}

	// Unmetered connection (wifi)
	wifiCaps := ClientCapabilities{
		NetworkSpeedMbps: 100.0,
		ConnectionType:   "wifi",
		IsMetered:        false,
		DeviceType:       DeviceTypeMobile,
	}

	if wifiCaps.IsMetered {
		t.Error("expected IsMetered to be false for wifi")
	}
}

func TestClientCapabilities_PerformanceFields(t *testing.T) {
	caps := ClientCapabilities{
		CPUCores:     8,
		MemoryGB:     16.0,
		LowPowerMode: false,
		BatteryLevel: 0.85,
		IsCharging:   true,
	}

	if caps.CPUCores != 8 {
		t.Errorf("expected 8 CPU cores, got %d", caps.CPUCores)
	}
	if caps.MemoryGB != 16.0 {
		t.Errorf("expected 16.0 GB memory, got %f", caps.MemoryGB)
	}
	if caps.LowPowerMode {
		t.Error("expected LowPowerMode to be false")
	}
	if caps.BatteryLevel != 0.85 {
		t.Errorf("expected battery level 0.85, got %f", caps.BatteryLevel)
	}
	if !caps.IsCharging {
		t.Error("expected IsCharging to be true")
	}
}

func TestAdaptiveProfile_FallbackCodecs(t *testing.T) {
	profile := AdaptiveProfile{
		ID:             "1080p-10m",
		PreferredCodec: "h265",
		FallbackCodecs: []string{"h264", "vp9"},
	}

	if profile.PreferredCodec != "h265" {
		t.Errorf("expected preferred codec 'h265', got %s", profile.PreferredCodec)
	}
	if len(profile.FallbackCodecs) != 2 {
		t.Errorf("expected 2 fallback codecs, got %d", len(profile.FallbackCodecs))
	}
	if profile.FallbackCodecs[0] != "h264" {
		t.Errorf("expected first fallback 'h264', got %s", profile.FallbackCodecs[0])
	}
}
