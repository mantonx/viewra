package profile

import (
	"io"
	"log/slog"
	"testing"
)

// createTestLogger creates a logger that discards output for testing
func createTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewQualityRecommender(t *testing.T) {
	logger := createTestLogger()
	r := NewQualityRecommender(logger)

	if r == nil {
		t.Fatal("NewQualityRecommender returned nil")
	}
	if r.logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestRecommendQuality(t *testing.T) {
	logger := createTestLogger()
	r := NewQualityRecommender(logger)

	tests := []struct {
		name        string
		caps        ClientCapabilities
		wantQuality string
	}{
		{
			name: "4K screen with excellent network",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 100.0,
				ScreenHeight:     2160,
				ScreenWidth:      3840,
				DeviceType:       "tv",
			},
			wantQuality: Quality4k100m, // 100 Mbps network gets 4k-100m
		},
		{
			name: "4K screen with good network",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 50.0,
				ScreenHeight:     2160,
				ScreenWidth:      3840,
				DeviceType:       "tv",
			},
			wantQuality: Quality4k40m,
		},
		{
			name: "4K screen with moderate network",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 30.0,
				ScreenHeight:     2160,
				ScreenWidth:      3840,
				DeviceType:       "tv",
			},
			wantQuality: Quality4k25m,
		},
		{
			name: "4K screen with acceptable network",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 18.0,
				ScreenHeight:     2160,
				ScreenWidth:      3840,
				DeviceType:       "tv",
			},
			wantQuality: Quality4k15m,
		},
		{
			name: "1080p screen with excellent network (desktop gets 4K-60m)",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 60.0,
				ScreenHeight:     1080,
				ScreenWidth:      1920,
				DeviceType:       "desktop",
			},
			wantQuality: Quality4k60m, // Desktop with >=60 Mbps gets highest 4K
		},
		{
			name: "1080p screen with good network (desktop gets 4K-40m)",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 50.0,
				ScreenHeight:     1080,
				ScreenWidth:      1920,
				DeviceType:       "desktop",
			},
			wantQuality: Quality4k40m, // Desktop with >=40 Mbps gets 4K-40m
		},
		{
			name: "1080p screen with moderate network",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 12.0,
				ScreenHeight:     1080,
				ScreenWidth:      1920,
				DeviceType:       "desktop",
			},
			wantQuality: Quality1080p10m,
		},
		{
			name: "1080p screen with limited network",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 6.0,
				ScreenHeight:     1080,
				ScreenWidth:      1920,
				DeviceType:       "tablet",
			},
			wantQuality: Quality1080p4m,
		},
		{
			name: "720p screen with moderate network",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 5.0,
				ScreenHeight:     720,
				ScreenWidth:      1280,
				DeviceType:       "tablet",
			},
			wantQuality: Quality720p4m,
		},
		{
			name: "720p screen with limited network",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 3.0,
				ScreenHeight:     720,
				ScreenWidth:      1280,
				DeviceType:       "mobile",
			},
			wantQuality: Quality720p2m,
		},
		{
			name: "SD fallback for poor network",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 1.5,
				ScreenHeight:     720,
				ScreenWidth:      1280,
				DeviceType:       "mobile",
			},
			wantQuality: Quality480p,
		},
		{
			name: "Lowest quality for very poor connection",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 0.5,
				ScreenHeight:     720,
				ScreenWidth:      1280,
				DeviceType:       "mobile",
			},
			wantQuality: Quality360p,
		},
		{
			name: "Unknown network speed defaults to 50 Mbps (desktop gets 4K-40m)",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 0,
				ScreenHeight:     1080,
				ScreenWidth:      1920,
				DeviceType:       "desktop",
			},
			wantQuality: Quality4k40m, // 50 Mbps default, desktop gets 4K-40m
		},
		{
			name: "Negative network speed defaults to 50 Mbps (desktop gets 4K-40m)",
			caps: ClientCapabilities{
				NetworkSpeedMbps: -10.0,
				ScreenHeight:     1080,
				ScreenWidth:      1920,
				DeviceType:       "desktop",
			},
			wantQuality: Quality4k40m, // 50 Mbps default, desktop gets 4K-40m
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recommendation := r.RecommendQuality(tt.caps)

			if recommendation == nil {
				t.Fatal("recommendation should not be nil")
			}
			if recommendation.Profile == nil {
				t.Fatal("profile should not be nil")
			}
			if recommendation.Profile.ID != tt.wantQuality {
				t.Errorf("Profile.ID = %v, want %v", recommendation.Profile.ID, tt.wantQuality)
			}
			if recommendation.Reason == "" {
				t.Error("Reason should not be empty")
			}
			if recommendation.Score != 1.0 {
				t.Errorf("Score = %v, want 1.0", recommendation.Score)
			}
		})
	}
}

func TestRecommendQuality_MeteredConnection(t *testing.T) {
	logger := createTestLogger()
	r := NewQualityRecommender(logger)

	tests := []struct {
		name         string
		caps         ClientCapabilities
		wantQuality  string
		wantContains string
	}{
		{
			name: "Metered connection downgrade from 4k-100m",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 100.0,
				ScreenHeight:     2160,
				ScreenWidth:      3840,
				DeviceType:       "mobile",
				IsMetered:        true,
			},
			wantQuality:  Quality4k40m, // 4k-100m downgraded to 4k-40m on metered
			wantContains: "metered",
		},
		{
			name: "Metered connection downgrade from 4k-40m",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 50.0,
				ScreenHeight:     2160,
				ScreenWidth:      3840,
				DeviceType:       "mobile",
				IsMetered:        true,
			},
			wantQuality:  Quality4k25m,
			wantContains: "metered",
		},
		{
			name: "Metered connection downgrade from 4k-25m",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 30.0,
				ScreenHeight:     2160,
				ScreenWidth:      3840,
				DeviceType:       "mobile",
				IsMetered:        true,
			},
			wantQuality:  Quality4k15m,
			wantContains: "metered",
		},
		{
			name: "Metered connection downgrade from 4k-15m",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 18.0,
				ScreenHeight:     2160,
				ScreenWidth:      3840,
				DeviceType:       "mobile",
				IsMetered:        true,
			},
			wantQuality:  Quality1080p20m,
			wantContains: "metered",
		},
		{
			name: "Metered connection downgrade from 1080p-40m",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 50.0,
				ScreenHeight:     1080,
				ScreenWidth:      1920,
				DeviceType:       "mobile",
				IsMetered:        true,
			},
			wantQuality:  Quality1080p20m,
			wantContains: "metered",
		},
		{
			name: "Metered connection downgrade from 1080p-20m",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 25.0,
				ScreenHeight:     1080,
				ScreenWidth:      1920,
				DeviceType:       "mobile",
				IsMetered:        true,
			},
			wantQuality:  Quality1080p10m,
			wantContains: "metered",
		},
		{
			name: "Metered connection downgrade from 1080p-10m",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 12.0,
				ScreenHeight:     1080,
				ScreenWidth:      1920,
				DeviceType:       "mobile",
				IsMetered:        true,
			},
			wantQuality:  Quality1080p4m,
			wantContains: "metered",
		},
		{
			name: "Metered connection downgrade from 1080p-4m",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 6.0,
				ScreenHeight:     1080,
				ScreenWidth:      1920,
				DeviceType:       "mobile",
				IsMetered:        true,
			},
			wantQuality:  Quality720p4m,
			wantContains: "metered",
		},
		{
			name: "Metered connection downgrade from 720p-4m",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 6.0,
				ScreenHeight:     720,
				ScreenWidth:      1280,
				DeviceType:       "mobile",
				IsMetered:        true,
			},
			wantQuality:  Quality720p2m,
			wantContains: "metered",
		},
		{
			name: "Metered connection with low speed (no downgrade)",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 3.0,
				ScreenHeight:     720,
				ScreenWidth:      1280,
				DeviceType:       "mobile",
				IsMetered:        true,
			},
			wantQuality: Quality720p2m, // Already at low quality, no downgrade
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recommendation := r.RecommendQuality(tt.caps)

			if recommendation.Profile.ID != tt.wantQuality {
				t.Errorf("Profile.ID = %v, want %v", recommendation.Profile.ID, tt.wantQuality)
			}
			if tt.wantContains != "" && !containsString(recommendation.Reason, tt.wantContains) {
				t.Errorf("Reason = %v, should contain %v", recommendation.Reason, tt.wantContains)
			}
		})
	}
}

func TestRecommendMultipleQualities(t *testing.T) {
	logger := createTestLogger()
	r := NewQualityRecommender(logger)

	caps := ClientCapabilities{
		NetworkSpeedMbps: 50.0,
		ScreenHeight:     1080,
		ScreenWidth:      1920,
		DeviceType:       "desktop",
	}

	recommendations := r.RecommendMultipleQualities(caps, 5)

	if len(recommendations) == 0 {
		t.Fatal("should return at least one recommendation")
	}
	if recommendations[0].Profile == nil {
		t.Fatal("first recommendation profile should not be nil")
	}
	// Should return the single best recommendation
	if len(recommendations) != 1 {
		t.Errorf("expected 1 recommendation, got %d", len(recommendations))
	}
}

func TestGetAdaptiveLadder(t *testing.T) {
	logger := createTestLogger()
	r := NewQualityRecommender(logger)

	tests := []struct {
		name         string
		caps         ClientCapabilities
		wantMinCount int
		wantHas4K    bool
	}{
		{
			name: "4K screen with fast network",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 50.0,
				ScreenHeight:     2160,
				ScreenWidth:      3840,
				DeviceType:       "tv",
			},
			wantMinCount: 6,
			wantHas4K:    true,
		},
		{
			name: "1080p screen with moderate network",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 15.0,
				ScreenHeight:     1080,
				ScreenWidth:      1920,
				DeviceType:       "desktop",
			},
			wantMinCount: 5,
			wantHas4K:    false,
		},
		{
			name: "720p screen with slow network",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 5.0,
				ScreenHeight:     720,
				ScreenWidth:      1280,
				DeviceType:       "mobile",
			},
			wantMinCount: 4,
			wantHas4K:    false,
		},
		{
			name: "Unknown network speed defaults to 50 Mbps",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 0,
				ScreenHeight:     1080,
				ScreenWidth:      1920,
				DeviceType:       "desktop",
			},
			wantMinCount: 5,
			wantHas4K:    false,
		},
		{
			name: "4K screen with network >= 20 Mbps gets 4K",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 20.0,
				ScreenHeight:     2160,
				ScreenWidth:      3840,
				DeviceType:       "tv",
			},
			wantMinCount: 6,
			wantHas4K:    true,
		},
		{
			name: "4K screen with network < 20 Mbps no 4K",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 19.0,
				ScreenHeight:     2160,
				ScreenWidth:      3840,
				DeviceType:       "tv",
			},
			wantMinCount: 5,
			wantHas4K:    false,
		},
		{
			name: "Network >= 10 Mbps gets 1080p-10m",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 10.0,
				ScreenHeight:     1080,
				ScreenWidth:      1920,
				DeviceType:       "desktop",
			},
			wantMinCount: 5,
			wantHas4K:    false,
		},
		{
			name: "Network < 10 Mbps no 1080p-10m",
			caps: ClientCapabilities{
				NetworkSpeedMbps: 9.0,
				ScreenHeight:     1080,
				ScreenWidth:      1920,
				DeviceType:       "desktop",
			},
			wantMinCount: 4,
			wantHas4K:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ladder := r.GetAdaptiveLadder(tt.caps)

			if len(ladder) < tt.wantMinCount {
				t.Errorf("ladder length = %v, want >= %v", len(ladder), tt.wantMinCount)
			}

			has4K := false
			for _, profile := range ladder {
				if profile.Height >= 2160 {
					has4K = true
					break
				}
			}

			if has4K != tt.wantHas4K {
				t.Errorf("has4K = %v, want %v", has4K, tt.wantHas4K)
			}

			// Verify all profiles are valid
			for _, profile := range ladder {
				if profile == nil {
					t.Error("ladder contains nil profile")
				}
			}
		})
	}
}

func TestFilterProfilesByScreenSize(t *testing.T) {
	profiles := GetAllAdaptiveProfiles()

	tests := []struct {
		name         string
		screenWidth  int
		screenHeight int
		wantMinCount int
		wantMaxRes   int
	}{
		{
			name:         "4K screen",
			screenWidth:  3840,
			screenHeight: 2160,
			wantMinCount: 10,
			wantMaxRes:   2160,
		},
		{
			name:         "1080p screen",
			screenWidth:  1920,
			screenHeight: 1080,
			wantMinCount: 5,
			wantMaxRes:   1080,
		},
		{
			name:         "720p screen",
			screenWidth:  1280,
			screenHeight: 720,
			wantMinCount: 3,
			wantMaxRes:   720,
		},
		{
			name:         "480p screen",
			screenWidth:  854,
			screenHeight: 480,
			wantMinCount: 2,
			wantMaxRes:   480,
		},
		{
			name:         "360p mobile",
			screenWidth:  640,
			screenHeight: 360,
			wantMinCount: 1,
			wantMaxRes:   360,
		},
		{
			name:         "Zero dimensions",
			screenWidth:  0,
			screenHeight: 0,
			wantMinCount: 0,
			wantMaxRes:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterProfilesByScreenSize(profiles, tt.screenWidth, tt.screenHeight)

			if len(filtered) < tt.wantMinCount {
				t.Errorf("count = %v, want >= %v", len(filtered), tt.wantMinCount)
			}

			for _, profile := range filtered {
				if profile.Width > tt.screenWidth || profile.Height > tt.screenHeight {
					t.Errorf("profile %s has resolution %dx%d, exceeds screen %dx%d",
						profile.ID, profile.Width, profile.Height, tt.screenWidth, tt.screenHeight)
				}
				if profile.Height > tt.wantMaxRes {
					t.Errorf("profile %s has height %d, want <= %d", profile.ID, profile.Height, tt.wantMaxRes)
				}
			}
		})
	}
}

func TestFilterProfilesByNetworkSpeed(t *testing.T) {
	profiles := GetAllAdaptiveProfiles()

	tests := []struct {
		name         string
		speedMbps    float64
		wantMinCount int
	}{
		{
			name:         "Gigabit connection 1000Mbps",
			speedMbps:    1000.0,
			wantMinCount: 10,
		},
		{
			name:         "Fast connection 100Mbps",
			speedMbps:    100.0,
			wantMinCount: 10,
		},
		{
			name:         "Good connection 50Mbps",
			speedMbps:    50.0,
			wantMinCount: 5,
		},
		{
			name:         "Medium 20Mbps",
			speedMbps:    20.0,
			wantMinCount: 5,
		},
		{
			name:         "Slow 5Mbps",
			speedMbps:    5.0,
			wantMinCount: 3,
		},
		{
			name:         "Very slow 2Mbps",
			speedMbps:    2.0,
			wantMinCount: 2,
		},
		{
			name:         "Poor 1Mbps",
			speedMbps:    1.0,
			wantMinCount: 1,
		},
		{
			name:         "Zero speed",
			speedMbps:    0.0,
			wantMinCount: 0,
		},
		{
			name:         "Negative speed",
			speedMbps:    -5.0,
			wantMinCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterProfilesByNetworkSpeed(profiles, tt.speedMbps)

			if len(filtered) < tt.wantMinCount {
				t.Errorf("count = %v, want >= %v", len(filtered), tt.wantMinCount)
			}

			for _, profile := range filtered {
				if profile.MinNetworkMbps > tt.speedMbps {
					t.Errorf("profile %s requires %.1f Mbps, exceeds available %.1f Mbps",
						profile.ID, profile.MinNetworkMbps, tt.speedMbps)
				}
			}
		})
	}
}

func TestApplyAudioSettings(t *testing.T) {
	tests := []struct {
		name              string
		qualityTier       string
		width             int
		wantAudioBitrate  int
		wantChannels      int
		wantSampleRate    int
		wantPreserveMulti bool
		wantCodec         string
		wantMaxChannels   int
	}{
		{
			name:              "Low tier - 360p",
			qualityTier:       "low",
			width:             640,
			wantAudioBitrate:  64_000,
			wantChannels:      2,
			wantSampleRate:    44100,
			wantPreserveMulti: false,
			wantCodec:         "aac",
			wantMaxChannels:   2,
		},
		{
			name:              "Medium tier - 480p",
			qualityTier:       "medium",
			width:             854,
			wantAudioBitrate:  128_000,
			wantChannels:      2,
			wantSampleRate:    48000,
			wantPreserveMulti: false,
			wantCodec:         "aac",
			wantMaxChannels:   2,
		},
		{
			name:              "High tier - 720p with width >= 1280",
			qualityTier:       "high",
			width:             1280,
			wantAudioBitrate:  256_000,
			wantChannels:      6,
			wantSampleRate:    48000,
			wantPreserveMulti: true,
			wantCodec:         "ac3",
			wantMaxChannels:   6,
		},
		{
			name:              "High tier - 720p with width < 1280",
			qualityTier:       "high",
			width:             1279,
			wantAudioBitrate:  192_000,
			wantChannels:      2,
			wantSampleRate:    48000,
			wantPreserveMulti: true,
			wantCodec:         "aac",
			wantMaxChannels:   6,
		},
		{
			name:              "High tier - 1080p",
			qualityTier:       "high",
			width:             1920,
			wantAudioBitrate:  256_000,
			wantChannels:      6,
			wantSampleRate:    48000,
			wantPreserveMulti: true,
			wantCodec:         "ac3",
			wantMaxChannels:   6,
		},
		{
			name:              "Ultra tier - 4K",
			qualityTier:       "ultra",
			width:             3840,
			wantAudioBitrate:  320_000,
			wantChannels:      8,
			wantSampleRate:    48000,
			wantPreserveMulti: true,
			wantCodec:         "eac3",
			wantMaxChannels:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &AdaptiveProfile{
				QualityTier: tt.qualityTier,
				Width:       tt.width,
			}

			applyAudioSettings(profile)

			if profile.AudioBitrate != tt.wantAudioBitrate {
				t.Errorf("AudioBitrate = %v, want %v", profile.AudioBitrate, tt.wantAudioBitrate)
			}
			if profile.AudioChannels != tt.wantChannels {
				t.Errorf("AudioChannels = %v, want %v", profile.AudioChannels, tt.wantChannels)
			}
			if profile.AudioSampleRate != tt.wantSampleRate {
				t.Errorf("AudioSampleRate = %v, want %v", profile.AudioSampleRate, tt.wantSampleRate)
			}
			if profile.PreserveMultiCh != tt.wantPreserveMulti {
				t.Errorf("PreserveMultiCh = %v, want %v", profile.PreserveMultiCh, tt.wantPreserveMulti)
			}
			if profile.AudioCodec != tt.wantCodec {
				t.Errorf("AudioCodec = %v, want %v", profile.AudioCodec, tt.wantCodec)
			}
			if profile.MaxAudioChannels != tt.wantMaxChannels {
				t.Errorf("MaxAudioChannels = %v, want %v", profile.MaxAudioChannels, tt.wantMaxChannels)
			}
		})
	}
}

// containsString is a helper to check if a string contains a substring
func containsString(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
