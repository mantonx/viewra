package transcoding

import (
	"testing"
)

// Note: Tests for internal profile builder functions (newProfile, withFrameRate, etc.)
// have been moved to internal/infrastructure/transcoding/profile/adaptive_test.go
// This file tests the re-exported public API from the parent package.

func TestAdaptiveProfilesReExports(t *testing.T) {
	// Verify re-exports work correctly

	// Test ABRLadder is available
	if len(ABRLadder) == 0 {
		t.Error("ABRLadder should not be empty")
	}

	// Test GetABRVariant
	variant, found := GetABRVariant("1080p-10m")
	if !found {
		t.Error("GetABRVariant should find 1080p-10m")
	}
	if variant.Bandwidth != 10_000_000 {
		t.Errorf("Bandwidth = %d, want 10000000", variant.Bandwidth)
	}

	// Test GetAdaptiveProfile
	profile, err := GetAdaptiveProfile("720p-4m")
	if err != nil {
		t.Errorf("GetAdaptiveProfile error = %v", err)
	}
	if profile.Width != 1280 {
		t.Errorf("Width = %d, want 1280", profile.Width)
	}

	// Test GetAllAdaptiveProfiles
	profiles := GetAllAdaptiveProfiles()
	if len(profiles) != len(ABRLadder) {
		t.Errorf("profiles count = %d, want %d", len(profiles), len(ABRLadder))
	}

	// Test IsAdaptiveQualitySupported
	if !IsAdaptiveQualitySupported("4k-60m") {
		t.Error("4k-60m should be supported")
	}
	if IsAdaptiveQualitySupported("invalid") {
		t.Error("invalid should not be supported")
	}

	// Test quality constants
	if Quality360p != "360p" {
		t.Errorf("Quality360p = %s, want 360p", Quality360p)
	}
	if Quality4k60m != "4k-60m" {
		t.Errorf("Quality4k60m = %s, want 4k-60m", Quality4k60m)
	}
}

func TestFilterProfilesByScreenSize(t *testing.T) {
	allProfiles := GetAllAdaptiveProfiles()

	tests := []struct {
		screenWidth  int
		screenHeight int
	}{
		{640, 360},
		{1280, 720},
		{1920, 1080},
		{3840, 2160},
	}

	for _, tt := range tests {
		filtered := FilterProfilesByScreenSize(allProfiles, tt.screenWidth, tt.screenHeight)

		for _, profile := range filtered {
			if profile.Width > tt.screenWidth || profile.Height > tt.screenHeight {
				t.Errorf("profile %s (%dx%d) exceeds screen size %dx%d",
					profile.ID, profile.Width, profile.Height, tt.screenWidth, tt.screenHeight)
			}
		}

		for _, profile := range allProfiles {
			if profile.Width <= tt.screenWidth && profile.Height <= tt.screenHeight {
				found := false
				for _, f := range filtered {
					if f.ID == profile.ID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("excluded valid profile %s", profile.ID)
				}
			}
		}
	}
}

func TestFilterProfilesByNetworkSpeed(t *testing.T) {
	allProfiles := GetAllAdaptiveProfiles()

	tests := []float64{1.0, 5.0, 10.0, 25.0, 100.0}

	for _, speedMbps := range tests {
		filtered := FilterProfilesByNetworkSpeed(allProfiles, speedMbps)

		for _, profile := range filtered {
			if profile.MinNetworkMbps > speedMbps {
				t.Errorf("profile %s requires %f Mbps > available %f Mbps",
					profile.ID, profile.MinNetworkMbps, speedMbps)
			}
		}

		for _, profile := range allProfiles {
			if profile.MinNetworkMbps <= speedMbps {
				found := false
				for _, f := range filtered {
					if f.ID == profile.ID {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("excluded valid profile %s", profile.ID)
				}
			}
		}
	}
}

func TestAdaptiveProfileAudioSettings(t *testing.T) {
	tests := []struct {
		qualityID           string
		wantAudioCodec      string
		wantMinAudioBitrate int
		wantMaxChannels     int
		wantPreserveMultiCh bool
	}{
		{"360p", "aac", 64_000, 2, false},
		{"480p", "aac", 128_000, 2, false},
		{"720p-4m", "ac3", 256_000, 6, true},
		{"1080p-10m", "ac3", 256_000, 6, true},
		{"4k-60m", "eac3", 320_000, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.qualityID, func(t *testing.T) {
			profile, err := GetAdaptiveProfile(tt.qualityID)
			if err != nil {
				t.Fatalf("GetAdaptiveProfile() error = %v", err)
			}

			if profile.AudioCodec != tt.wantAudioCodec {
				t.Errorf("AudioCodec = %v, want %v", profile.AudioCodec, tt.wantAudioCodec)
			}
			if profile.AudioBitrate < tt.wantMinAudioBitrate {
				t.Errorf("AudioBitrate = %v, want >= %v", profile.AudioBitrate, tt.wantMinAudioBitrate)
			}
			if profile.MaxAudioChannels != tt.wantMaxChannels {
				t.Errorf("MaxAudioChannels = %v, want %v", profile.MaxAudioChannels, tt.wantMaxChannels)
			}
			if profile.PreserveMultiCh != tt.wantPreserveMultiCh {
				t.Errorf("PreserveMultiCh = %v, want %v", profile.PreserveMultiCh, tt.wantPreserveMultiCh)
			}
		})
	}
}

func TestAdaptiveProfileDefaults(t *testing.T) {
	allProfiles := GetAllAdaptiveProfiles()

	for _, profile := range allProfiles {
		t.Run(profile.ID, func(t *testing.T) {
			if profile.VideoBitrate <= 0 {
				t.Errorf("VideoBitrate should be > 0, got %d", profile.VideoBitrate)
			}
			if profile.VideoMaxRate <= profile.VideoBitrate {
				t.Errorf("VideoMaxRate (%d) should be > VideoBitrate (%d)", profile.VideoMaxRate, profile.VideoBitrate)
			}
			if profile.VideoBufSize <= profile.VideoBitrate {
				t.Errorf("VideoBufSize (%d) should be > VideoBitrate (%d)", profile.VideoBufSize, profile.VideoBitrate)
			}
			if profile.AudioBitrate <= 0 {
				t.Errorf("AudioBitrate should be > 0, got %d", profile.AudioBitrate)
			}
			if profile.AudioChannels <= 0 {
				t.Errorf("AudioChannels should be > 0, got %d", profile.AudioChannels)
			}
			if profile.AudioSampleRate <= 0 {
				t.Errorf("AudioSampleRate should be > 0, got %d", profile.AudioSampleRate)
			}
			if profile.AudioCodec == "" {
				t.Error("AudioCodec should not be empty")
			}
			if profile.PreferredCodec == "" {
				t.Error("PreferredCodec should not be empty")
			}
			if profile.Preset == "" {
				t.Error("Preset should not be empty")
			}
			if profile.CRF <= 0 || profile.CRF > 51 {
				t.Errorf("CRF should be in range 1-51, got %d", profile.CRF)
			}
			if profile.SegmentDuration <= 0 {
				t.Errorf("SegmentDuration should be > 0, got %d", profile.SegmentDuration)
			}
			if profile.GOPSize <= 0 {
				t.Errorf("GOPSize should be > 0, got %d", profile.GOPSize)
			}
			if profile.FrameRate <= 0 {
				t.Errorf("FrameRate should be > 0, got %f", profile.FrameRate)
			}
			if profile.AspectRatio == "" {
				t.Error("AspectRatio should not be empty")
			}
			if profile.MinNetworkMbps <= 0 {
				t.Errorf("MinNetworkMbps should be > 0, got %f", profile.MinNetworkMbps)
			}
			if profile.MinScreenWidth <= 0 {
				t.Errorf("MinScreenWidth should be > 0, got %d", profile.MinScreenWidth)
			}
			if profile.MinScreenHeight <= 0 {
				t.Errorf("MinScreenHeight should be > 0, got %d", profile.MinScreenHeight)
			}
			if profile.DataUsageMBPerHour <= 0 {
				t.Errorf("DataUsageMBPerHour should be > 0, got %d", profile.DataUsageMBPerHour)
			}
			if profile.QualityTier == "" {
				t.Error("QualityTier should not be empty")
			}
			if !profile.EnableHWAccel {
				t.Error("EnableHWAccel should be true by default")
			}
			if !profile.EnableFastStart {
				t.Error("EnableFastStart should be true by default")
			}
		})
	}
}
