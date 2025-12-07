package transcoding

import (
	"log/slog"
	"os"
	"testing"
)

func TestNewQualityRecommender(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	qr := NewQualityRecommender(logger)

	if qr == nil || qr.logger == nil {
		t.Fatal("NewQualityRecommender returned invalid instance")
	}
}

func TestRecommendQuality(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	qr := NewQualityRecommender(logger)

	tests := []struct {
		name       string
		caps       ClientCapabilities
		expectedID string
	}{
		{"4K 60Mbps excellent", ClientCapabilities{NetworkSpeedMbps: 70, ScreenHeight: 2160}, Quality4k60m},
		{"4K 40Mbps good", ClientCapabilities{NetworkSpeedMbps: 50, ScreenHeight: 2160}, Quality4k40m},
		{"4K 25Mbps moderate", ClientCapabilities{NetworkSpeedMbps: 30, ScreenHeight: 2160}, Quality4k25m},
		{"4K 15Mbps acceptable", ClientCapabilities{NetworkSpeedMbps: 20, ScreenHeight: 2160}, Quality4k15m},
		{"1080p 40Mbps excellent", ClientCapabilities{NetworkSpeedMbps: 45, ScreenHeight: 1080}, Quality1080p40m},
		{"1080p 20Mbps good", ClientCapabilities{NetworkSpeedMbps: 25, ScreenHeight: 1080}, Quality1080p20m},
		{"1080p 10Mbps moderate", ClientCapabilities{NetworkSpeedMbps: 15, ScreenHeight: 1080}, Quality1080p10m},
		{"1080p 4Mbps limited", ClientCapabilities{NetworkSpeedMbps: 5, ScreenHeight: 1080}, Quality1080p4m},
		{"720p 4Mbps moderate", ClientCapabilities{NetworkSpeedMbps: 6, ScreenHeight: 720}, Quality720p4m},
		{"720p 2Mbps limited", ClientCapabilities{NetworkSpeedMbps: 3, ScreenHeight: 720}, Quality720p2m},
		{"480p poor network", ClientCapabilities{NetworkSpeedMbps: 1.5, ScreenHeight: 720}, Quality480p},
		{"360p very poor", ClientCapabilities{NetworkSpeedMbps: 0.5, ScreenHeight: 480}, Quality360p},
		{"Unknown network 1080p", ClientCapabilities{NetworkSpeedMbps: 0, ScreenHeight: 1080}, Quality1080p40m},
		{"Unknown network 4K", ClientCapabilities{NetworkSpeedMbps: -1, ScreenHeight: 2160}, Quality4k40m},
		{"4K metered 60Mbps", ClientCapabilities{NetworkSpeedMbps: 70, ScreenHeight: 2160, IsMetered: true}, Quality4k25m},
		{"4K metered 25Mbps", ClientCapabilities{NetworkSpeedMbps: 30, ScreenHeight: 2160, IsMetered: true}, Quality4k15m},
		{"4K metered 15Mbps", ClientCapabilities{NetworkSpeedMbps: 20, ScreenHeight: 2160, IsMetered: true}, Quality1080p20m},
		{"1080p metered 40Mbps", ClientCapabilities{NetworkSpeedMbps: 45, ScreenHeight: 1080, IsMetered: true}, Quality1080p20m},
		{"1080p metered 20Mbps", ClientCapabilities{NetworkSpeedMbps: 25, ScreenHeight: 1080, IsMetered: true}, Quality1080p10m},
		{"1080p metered 10Mbps", ClientCapabilities{NetworkSpeedMbps: 15, ScreenHeight: 1080, IsMetered: true}, Quality1080p4m},
		{"1080p metered 4Mbps", ClientCapabilities{NetworkSpeedMbps: 6, ScreenHeight: 1080, IsMetered: true}, Quality720p4m},
		{"720p metered 4Mbps", ClientCapabilities{NetworkSpeedMbps: 6, ScreenHeight: 720, IsMetered: true}, Quality720p2m},
		{"4K poor network", ClientCapabilities{NetworkSpeedMbps: 5, ScreenHeight: 2160}, Quality1080p4m},
		{"720p high network", ClientCapabilities{NetworkSpeedMbps: 100, ScreenHeight: 720}, Quality720p4m},
		{"Edge: zero height", ClientCapabilities{NetworkSpeedMbps: 50, ScreenHeight: 0}, ""},
		{"Edge: 8K screen", ClientCapabilities{NetworkSpeedMbps: 100, ScreenHeight: 8192}, ""},
		{"Edge: 1000Mbps", ClientCapabilities{NetworkSpeedMbps: 1000, ScreenHeight: 2160}, ""},
		{"Edge: negative speed", ClientCapabilities{NetworkSpeedMbps: -100, ScreenHeight: 1080}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := qr.RecommendQuality(tt.caps)
			if rec == nil || rec.Profile == nil {
				t.Fatal("RecommendQuality returned nil")
			}
			if tt.expectedID != "" && rec.Profile.ID != tt.expectedID {
				t.Errorf("Expected %q, got %q", tt.expectedID, rec.Profile.ID)
			}
			if rec.Score != 1.0 {
				t.Errorf("Expected score 1.0, got %f", rec.Score)
			}
			if rec.Profile.ID == "" {
				t.Error("Profile ID is empty")
			}
		})
	}
}

func TestRecommendMultipleQualities(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	qr := NewQualityRecommender(logger)

	recs := qr.RecommendMultipleQualities(ClientCapabilities{NetworkSpeedMbps: 25, ScreenHeight: 1080}, 3)
	if len(recs) == 0 {
		t.Error("Expected at least one recommendation")
	}
	if recs[0].Profile.ID != Quality1080p20m {
		t.Errorf("Expected %q, got %q", Quality1080p20m, recs[0].Profile.ID)
	}
}

func TestGetAdaptiveLadder(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	qr := NewQualityRecommender(logger)

	tests := []struct {
		name             string
		caps             ClientCapabilities
		expectedRungs    int
		shouldInclude4K  bool
		shouldInclude1080p10m bool
	}{
		{"High 4K", ClientCapabilities{NetworkSpeedMbps: 50, ScreenHeight: 2160}, 6, true, true},
		{"Moderate 1080p", ClientCapabilities{NetworkSpeedMbps: 15, ScreenHeight: 1080}, 5, false, true},
		{"Low 720p", ClientCapabilities{NetworkSpeedMbps: 5, ScreenHeight: 720}, 4, false, false},
		{"Unknown 4K", ClientCapabilities{NetworkSpeedMbps: 0, ScreenHeight: 2160}, 6, true, true},
		{"4K low network", ClientCapabilities{NetworkSpeedMbps: 15, ScreenHeight: 2160}, 5, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ladder := qr.GetAdaptiveLadder(tt.caps)
			if len(ladder) != tt.expectedRungs {
				t.Errorf("Expected %d rungs, got %d", tt.expectedRungs, len(ladder))
			}

			has360p, has4K, has1080p10m := false, false, false
			for _, profile := range ladder {
				if profile.ID == Quality360p {
					has360p = true
				}
				if profile.ID == Quality4k20m {
					has4K = true
				}
				if profile.ID == Quality1080p10m {
					has1080p10m = true
				}
			}

			if !has360p {
				t.Error("Ladder should always include 360p")
			}
			if tt.shouldInclude4K != has4K {
				t.Errorf("4K inclusion mismatch: expected %v, got %v", tt.shouldInclude4K, has4K)
			}
			if tt.shouldInclude1080p10m != has1080p10m {
				t.Errorf("1080p-10m inclusion mismatch: expected %v, got %v", tt.shouldInclude1080p10m, has1080p10m)
			}
		})
	}

	essentials := []string{Quality360p, Quality480p, Quality720p2m, Quality1080p4m}
	fullLadder := qr.GetAdaptiveLadder(ClientCapabilities{NetworkSpeedMbps: 100, ScreenHeight: 2160})
	for _, quality := range essentials {
		found := false
		for _, profile := range fullLadder {
			if profile.ID == quality {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing essential quality: %s", quality)
		}
	}
}

func TestRecommendCodec(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	qr := NewQualityRecommender(logger)

	tests := []struct {
		name     string
		codecs   []string
		hwAccel  HardwareAccel
		expected VideoCodec
	}{
		{"AV1 best", []string{"av1", "h265", "h264"}, AccelNone, CodecAV1},
		{"H265 good", []string{"h265", "h264"}, AccelNone, CodecH265},
		{"VP9 VAAPI", []string{"vp9", "h264"}, AccelVAAPI, CodecVP9},
		{"H264 only", []string{"h264"}, AccelNone, CodecH264},
		{"Empty fallback", []string{}, AccelNone, CodecH264},
		{"All codecs", []string{"h264", "h265", "vp9", "av1"}, AccelNone, CodecAV1},
		{"VP9 NVENC skip", []string{"h264", "h265", "vp9"}, AccelNVENC, CodecH265},
		{"AV1 NVENC", []string{"av1", "h264"}, AccelNVENC, CodecAV1},
		{"H265 VideoToolbox", []string{"h265", "h264"}, AccelVideoToolbox, CodecH265},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codec := qr.RecommendCodec(ClientCapabilities{SupportedCodecs: tt.codecs}, tt.hwAccel)
			if codec != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, codec)
			}
		})
	}

	priorities := []struct {
		codecs   []string
		expected VideoCodec
	}{
		{[]string{"h264", "vp9", "h265", "av1"}, CodecAV1},
		{[]string{"h264", "vp9", "h265"}, CodecH265},
		{[]string{"h264", "vp9"}, CodecVP9},
		{[]string{"h264"}, CodecH264},
	}

	for _, p := range priorities {
		codec := qr.RecommendCodec(ClientCapabilities{SupportedCodecs: p.codecs}, AccelNone)
		if codec != p.expected {
			t.Errorf("For %v, expected %s, got %s", p.codecs, p.expected, codec)
		}
	}
}

func TestGetBestCodecForProfile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	qr := NewQualityRecommender(logger)

	tests := []struct {
		name       string
		preferred  string
		fallbacks  []string
		supported  []string
		hwAccel    HardwareAccel
		expected   VideoCodec
	}{
		{"H265 match", "h265", []string{"h264"}, []string{"h265", "h264"}, AccelNone, CodecH265},
		{"H265 fallback", "h265", []string{"h264"}, []string{"h264"}, AccelNone, CodecH264},
		{"AV1 multi fallback", "av1", []string{"h265", "vp9", "h264"}, []string{"vp9", "h264"}, AccelVAAPI, CodecVP9},
		{"VP9 NVENC skip", "vp9", []string{"h265", "h264"}, []string{"vp9", "h265", "h264"}, AccelNVENC, CodecH265},
		{"No match", "av1", []string{"h265", "vp9"}, []string{"h264"}, AccelNone, CodecH264},
		{"H264 direct", "h264", []string{}, []string{"h264", "h265"}, AccelNone, CodecH264},
		{"Empty client", "h265", []string{"h264"}, []string{}, AccelNone, CodecH264},
		{"QSV H265", "h265", []string{"h264"}, []string{"h265", "h264"}, AccelQSV, CodecH265},
		{"VAAPI AV1", "av1", []string{"h265", "h264"}, []string{"av1", "h265", "h264"}, AccelVAAPI, CodecAV1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := &AdaptiveProfile{PreferredCodec: tt.preferred, FallbackCodecs: tt.fallbacks}
			caps := ClientCapabilities{SupportedCodecs: tt.supported}
			codec := qr.GetBestCodecForProfile(profile, caps, tt.hwAccel)
			if codec != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, codec)
			}
		})
	}
}
