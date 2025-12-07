package profile

import (
	"testing"

	"github.com/mantonx/viewra/internal/domain/transcode"
)

func TestWithFrameRate(t *testing.T) {
	tests := []struct {
		fps         float64
		wantGOPSize int
	}{
		{24.0, 48},
		{30.0, 60},
		{60.0, 120},
		{23.976, 47},
	}

	for _, tt := range tests {
		pb := newProfile("test", "Test", 1920, 1080, 5_000_000)
		pb.withFrameRate(tt.fps)
		profile := pb.build()

		if profile.FrameRate != tt.fps {
			t.Errorf("FrameRate = %v, want %v", profile.FrameRate, tt.fps)
		}
		if profile.GOPSize != tt.wantGOPSize {
			t.Errorf("GOPSize = %v, want %v", profile.GOPSize, tt.wantGOPSize)
		}
	}
}

func TestWithAspectRatio(t *testing.T) {
	tests := []struct {
		width      int
		height     int
		ratio      string
		wantHeight int
	}{
		{1920, 1080, "16:9", 1080},
		{3840, 2160, "2.39:1", 1606},
		{1920, 1080, "2.39:1", 1080},
	}

	for _, tt := range tests {
		pb := newProfile("test", "Test", tt.width, tt.height, 5_000_000)
		pb.withAspectRatio(tt.ratio)
		profile := pb.build()

		if profile.AspectRatio != tt.ratio {
			t.Errorf("AspectRatio = %v, want %v", profile.AspectRatio, tt.ratio)
		}
		if profile.Height != tt.wantHeight {
			t.Errorf("Height = %v, want %v", profile.Height, tt.wantHeight)
		}
	}
}

func TestWith3D(t *testing.T) {
	tests := []struct {
		stereoMode string
		wantIs3D   bool
	}{
		{"sbs", true},
		{"tab", true},
		{"", true},
	}

	for _, tt := range tests {
		pb := newProfile("test", "Test", 1920, 1080, 5_000_000)
		pb.with3D(tt.stereoMode)
		profile := pb.build()

		if profile.Is3D != tt.wantIs3D {
			t.Errorf("Is3D = %v, want %v", profile.Is3D, tt.wantIs3D)
		}
		if profile.StereoMode != tt.stereoMode {
			t.Errorf("StereoMode = %v, want %v", profile.StereoMode, tt.stereoMode)
		}
	}
}

func TestProfileBuilderChaining(t *testing.T) {
	profile := newProfile("test-chain", "Test Chain", 1920, 1080, 5_000_000).
		withCodec("h264", "h265").
		withPreset("medium", 23).
		withNetwork(5.0).
		withScreen(1920, 1080).
		withDevices("desktop", "tv").
		withDescription("Test profile", "high").
		withFrameRate(30.0).
		withAspectRatio("16:9").
		with3D("sbs").
		build()

	if profile.ID != "test-chain" {
		t.Errorf("ID = %v, want test-chain", profile.ID)
	}
	if profile.PreferredCodec != "h264" {
		t.Errorf("PreferredCodec = %v, want h264", profile.PreferredCodec)
	}
	if len(profile.FallbackCodecs) != 1 || profile.FallbackCodecs[0] != "h265" {
		t.Errorf("FallbackCodecs = %v, want [h265]", profile.FallbackCodecs)
	}
	if profile.Preset != "medium" {
		t.Errorf("Preset = %v, want medium", profile.Preset)
	}
	if profile.CRF != 23 {
		t.Errorf("CRF = %v, want 23", profile.CRF)
	}
	if profile.MinNetworkMbps != 5.0 {
		t.Errorf("MinNetworkMbps = %v, want 5.0", profile.MinNetworkMbps)
	}
	if profile.FrameRate != 30.0 {
		t.Errorf("FrameRate = %v, want 30.0", profile.FrameRate)
	}
	if profile.GOPSize != 60 {
		t.Errorf("GOPSize = %v, want 60", profile.GOPSize)
	}
	if profile.AspectRatio != "16:9" {
		t.Errorf("AspectRatio = %v, want 16:9", profile.AspectRatio)
	}
	if !profile.Is3D {
		t.Error("Is3D should be true")
	}
	if profile.StereoMode != "sbs" {
		t.Errorf("StereoMode = %v, want sbs", profile.StereoMode)
	}
	if profile.QualityTier != "high" {
		t.Errorf("QualityTier = %v, want high", profile.QualityTier)
	}
	if len(profile.RecommendedFor) != 2 {
		t.Errorf("RecommendedFor length = %v, want 2", len(profile.RecommendedFor))
	}
}

func TestGetABRVariant(t *testing.T) {
	tests := []struct {
		qualityID   string
		wantFound   bool
		wantWidth   int
		wantHeight  int
		wantBitrate int
	}{
		{"360p", true, 640, 360, 800_000},
		{"720p-4m", true, 1280, 720, 4_000_000},
		{"1080p-10m", true, 1920, 1080, 10_000_000},
		{"4k-60m", true, 3840, 2160, 60_000_000},
		{"invalid-quality", false, 0, 0, 0},
		{"", false, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.qualityID, func(t *testing.T) {
			variant, found := GetABRVariant(tt.qualityID)

			if found != tt.wantFound {
				t.Errorf("found = %v, want %v", found, tt.wantFound)
			}

			if tt.wantFound {
				if variant.ID != tt.qualityID {
					t.Errorf("ID = %v, want %v", variant.ID, tt.qualityID)
				}
				if variant.Width != tt.wantWidth {
					t.Errorf("Width = %v, want %v", variant.Width, tt.wantWidth)
				}
				if variant.Height != tt.wantHeight {
					t.Errorf("Height = %v, want %v", variant.Height, tt.wantHeight)
				}
				if variant.Bandwidth != tt.wantBitrate {
					t.Errorf("Bandwidth = %v, want %v", variant.Bandwidth, tt.wantBitrate)
				}
				if variant.Codecs == "" {
					t.Error("Codecs should not be empty")
				}
			}
		})
	}
}

func TestGetAdaptiveProfile(t *testing.T) {
	tests := []struct {
		quality   string
		wantError bool
		wantID    string
	}{
		{"360p", false, "360p"},
		{"720p-4m", false, "720p-4m"},
		{"1080p-10m", false, "1080p-10m"},
		{"4k-60m", false, "4k-60m"},
		{"invalid-quality", true, ""},
		{"", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.quality, func(t *testing.T) {
			profile, err := GetAdaptiveProfile(tt.quality)

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				if profile != nil {
					t.Errorf("expected nil profile but got %v", profile)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error = %v", err)
				}
				if profile == nil {
					t.Fatal("returned nil profile")
				}
				if profile.ID != tt.wantID {
					t.Errorf("ID = %v, want %v", profile.ID, tt.wantID)
				}
			}
		})
	}
}

func TestGetAllAdaptiveProfiles(t *testing.T) {
	profiles := GetAllAdaptiveProfiles()

	expectedCount := len(ABRLadder)
	if len(profiles) != expectedCount {
		t.Errorf("count = %v, want %v", len(profiles), expectedCount)
	}

	for _, profile := range profiles {
		if profile == nil {
			t.Error("returned nil profile")
			continue
		}
		if profile.ID == "" {
			t.Error("profile has empty ID")
		}
		if profile.DisplayName == "" {
			t.Error("profile has empty DisplayName")
		}
		if profile.Width <= 0 || profile.Height <= 0 {
			t.Errorf("profile %s has invalid dimensions %dx%d", profile.ID, profile.Width, profile.Height)
		}
		if profile.VideoBitrate <= 0 {
			t.Errorf("profile %s has invalid VideoBitrate = %d", profile.ID, profile.VideoBitrate)
		}
	}
}

func TestIsAdaptiveQualitySupported(t *testing.T) {
	tests := []struct {
		quality string
		want    bool
	}{
		{"360p", true},
		{"720p-4m", true},
		{"1080p-10m", true},
		{"4k-200m", true},
		{"invalid-quality", false},
		{"", false},
	}

	for _, tt := range tests {
		got := IsAdaptiveQualitySupported(tt.quality)
		if got != tt.want {
			t.Errorf("IsAdaptiveQualitySupported(%s) = %v, want %v", tt.quality, got, tt.want)
		}
	}
}

func TestGetAdaptiveQualitiesByTier(t *testing.T) {
	tests := []struct {
		tier        string
		wantMinimum int
	}{
		{"low", 1},
		{"medium", 1},
		{"high", 1},
		{"ultra", 1},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			profiles := GetAdaptiveQualitiesByTier(tt.tier)

			if len(profiles) < tt.wantMinimum {
				t.Errorf("count = %v, want >= %v", len(profiles), tt.wantMinimum)
			}

			for _, profile := range profiles {
				if profile.QualityTier != tt.tier {
					t.Errorf("profile %s has tier %s, want %s", profile.ID, profile.QualityTier, tt.tier)
				}
			}
		})
	}
}

func TestGetAdaptiveProfileForQuality(t *testing.T) {
	tests := []struct {
		quality   string
		wantError bool
		wantID    string
	}{
		{"360p", false, "360p"},
		{"720p-4m", false, "720p-4m"},
		{"1080p-10m", false, "1080p-10m"},
		{"4k-60m", false, "4k-60m"},
		{transcode.Quality360p, false, Quality360p},
		{transcode.Quality480p, false, Quality480p},
		{transcode.Quality720p, false, Quality720p4m},
		{transcode.Quality1080p, false, Quality1080p10m},
		{transcode.Quality4K, false, Quality4k60m},
		{"invalid-quality", true, ""},
		{"", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.quality, func(t *testing.T) {
			profile, err := GetAdaptiveProfileForQuality(tt.quality)

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				if profile != nil {
					t.Errorf("expected nil profile but got %v", profile)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error = %v", err)
				}
				if profile == nil {
					t.Fatal("returned nil profile")
				}
				if profile.ID != tt.wantID {
					t.Errorf("ID = %v, want %v", profile.ID, tt.wantID)
				}
			}
		})
	}
}

func TestABRLadderConsistency(t *testing.T) {
	for _, variant := range ABRLadder {
		t.Run(variant.ID, func(t *testing.T) {
			profile, err := GetAdaptiveProfile(variant.ID)
			if err != nil {
				t.Fatalf("GetAdaptiveProfile(%s) error = %v", variant.ID, err)
			}

			if profile.ID != variant.ID {
				t.Errorf("Profile ID = %v, want %v", profile.ID, variant.ID)
			}
			if profile.Width != variant.Width {
				t.Errorf("Profile Width = %v, want %v", profile.Width, variant.Width)
			}
			if profile.Height != variant.Height {
				t.Errorf("Profile Height = %v, want %v", profile.Height, variant.Height)
			}
			if profile.VideoBitrate != variant.Bandwidth {
				t.Errorf("Profile VideoBitrate = %v, want %v", profile.VideoBitrate, variant.Bandwidth)
			}
		})
	}
}

func TestQualityTierDistribution(t *testing.T) {
	tiers := []string{"low", "medium", "high", "ultra"}

	for _, tier := range tiers {
		t.Run(tier, func(t *testing.T) {
			profiles := GetAdaptiveQualitiesByTier(tier)
			if len(profiles) == 0 {
				t.Errorf("No profiles found for tier %s", tier)
			}

			for _, profile := range profiles {
				if profile.QualityTier != tier {
					t.Errorf("Profile %s has tier %s, expected %s", profile.ID, profile.QualityTier, tier)
				}
			}
		})
	}
}
