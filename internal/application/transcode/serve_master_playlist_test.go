package transcode

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/strategy"
)

func TestBuildAvailableQualities_FiltersToSourceResolution(t *testing.T) {
	uc := &ServeMasterPlaylistUseCase{}

	tests := []struct {
		name        string
		mediaItem   *media.Media
		maxHeight   int
		minExpected int
	}{
		{
			name: "1080p source excludes 4K",
			mediaItem: &media.Media{
				Width:   1920,
				Height:  1080,
				Bitrate: 20_000_000,
			},
			maxHeight:   1080,
			minExpected: 5,
		},
		{
			name: "4K source includes all",
			mediaItem: &media.Media{
				Width:   3840,
				Height:  2160,
				Bitrate: 80_000_000,
			},
			maxHeight:   2160,
			minExpected: 10,
		},
		{
			name: "720p source excludes 1080p+",
			mediaItem: &media.Media{
				Width:   1280,
				Height:  720,
				Bitrate: 4_000_000,
			},
			maxHeight:   720,
			minExpected: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Use Transcode strategy for resolution filtering tests
			qualities := uc.buildAvailableQualities(tc.mediaItem, profile.Quality720p4m, strategy.Transcode)

			if len(qualities) < tc.minExpected {
				t.Errorf("expected at least %d qualities, got %d", tc.minExpected, len(qualities))
			}

			for _, q := range qualities {
				if q.Height > tc.maxHeight {
					t.Errorf("quality %s has height %d, exceeds max %d", q.ID, q.Height, tc.maxHeight)
				}
			}
		})
	}
}

func TestBuildAvailableQualities_RemuxIncludesOriginal(t *testing.T) {
	uc := &ServeMasterPlaylistUseCase{}

	mediaItem := &media.Media{
		Width:   3840,
		Height:  2160,
		Bitrate: 80_000_000, // 80 Mbps source
	}

	qualities := uc.buildAvailableQualities(mediaItem, "original", strategy.RemuxHEVC)

	// Should have "Original" as first option
	if len(qualities) == 0 {
		t.Fatal("expected at least one quality option")
	}

	if qualities[0].ID != "original" {
		t.Errorf("expected first quality to be 'original', got %s", qualities[0].ID)
	}

	if qualities[0].Bandwidth != 80_000_000 {
		t.Errorf("expected original bandwidth to be 80000000, got %d", qualities[0].Bandwidth)
	}

	if !qualities[0].IsSelected {
		t.Error("expected original to be selected")
	}

	// Original quality should have IsOriginalQuality=true
	if !qualities[0].IsOriginalQuality {
		t.Error("expected original quality to have IsOriginalQuality=true")
	}

	// Should include lower bitrate options (60m, 40m, 25m, etc.)
	hasLowerOptions := false
	for _, q := range qualities[1:] {
		if q.ID == profile.Quality4k60m || q.ID == profile.Quality4k40m {
			hasLowerOptions = true
			break
		}
	}
	if !hasLowerOptions {
		t.Error("expected lower bitrate options to be available")
	}
}

func TestBuildAvailableQualities_IsOriginalQualityForMatchingResolution(t *testing.T) {
	uc := &ServeMasterPlaylistUseCase{}

	// 1080p source at 20 Mbps
	mediaItem := &media.Media{
		Width:   1920,
		Height:  1080,
		Bitrate: 20_000_000,
	}

	qualities := uc.buildAvailableQualities(mediaItem, profile.Quality1080p10m, strategy.Transcode)

	// For transcode, only the HIGHEST bitrate at source height should be marked original
	// With 20 Mbps source (120% = 24 Mbps limit), highest available is 1080p-20m
	originalCount := 0
	var originalID string
	for _, q := range qualities {
		if q.IsOriginalQuality {
			originalCount++
			originalID = q.ID
		}
	}

	if originalCount != 1 {
		t.Errorf("expected exactly 1 original quality, got %d", originalCount)
	}

	// The highest 1080p profile within bitrate limit should be original
	if originalID != profile.Quality1080p20m {
		t.Errorf("expected %s to be original, got %s", profile.Quality1080p20m, originalID)
	}
}

func TestBuildAvailableQualities_RemuxOnlyOriginalHasFlag(t *testing.T) {
	uc := &ServeMasterPlaylistUseCase{}

	// 4K source at 80 Mbps (remux scenario)
	mediaItem := &media.Media{
		Width:   3840,
		Height:  2160,
		Bitrate: 80_000_000,
	}

	qualities := uc.buildAvailableQualities(mediaItem, "original", strategy.RemuxHEVC)

	// For remux, only the "original" entry should have IsOriginalQuality=true
	// All other profiles should have IsOriginalQuality=false
	originalCount := 0
	for _, q := range qualities {
		if q.IsOriginalQuality {
			originalCount++
			if q.ID != "original" {
				t.Errorf("only 'original' should have IsOriginalQuality=true, but %s has it", q.ID)
			}
		}
	}

	if originalCount != 1 {
		t.Errorf("expected exactly 1 original quality in remux, got %d", originalCount)
	}
}

func TestBuildAvailableQualities_IsOriginalQualityForNonStandardAspectRatio(t *testing.T) {
	uc := &ServeMasterPlaylistUseCase{}

	// Cinemascope 1080p source (2.39:1 aspect ratio)
	mediaItem := &media.Media{
		Width:   1920,
		Height:  804, // ~2.39:1 aspect ratio, no matching profile height
		Bitrate: 15_000_000,
	}

	qualities := uc.buildAvailableQualities(mediaItem, profile.Quality720p4m, strategy.Transcode)

	// Since 804 doesn't match any profile height, no qualities should be marked original
	for _, q := range qualities {
		if q.IsOriginalQuality {
			t.Errorf("quality %s should not be original for non-standard height 804", q.ID)
		}
	}
}

func TestBuildSingleVariantPlaylist_UsesSourceBitrateForOriginalRemux(t *testing.T) {
	uc := &ServeMasterPlaylistUseCase{}

	mediaItem := &media.Media{
		Width:   3840,
		Height:  2160,
		Bitrate: 80_000_000, // 80 Mbps source
	}

	params := buildVariantParams{
		strategy: strategy.RemuxHEVC,
	}

	// "original" quality on remux uses source bitrate/resolution
	playlist := uc.buildSingleVariantPlaylist(mediaItem, "original", nil, nil, params, "", "")

	// For remux with "original", should use source bitrate (80 Mbps * 1.1 = 88 Mbps)
	expectedBandwidth := "BANDWIDTH=88000000"
	if !strings.Contains(playlist, expectedBandwidth) {
		t.Errorf("expected playlist to contain %s for original remux, got:\n%s", expectedBandwidth, playlist)
	}

	// Should also use source resolution
	expectedResolution := "RESOLUTION=3840x2160"
	if !strings.Contains(playlist, expectedResolution) {
		t.Errorf("expected playlist to contain %s for original remux, got:\n%s", expectedResolution, playlist)
	}
}

func TestBuildSingleVariantPlaylist_UsesProfileBitrateForNonOriginalRemux(t *testing.T) {
	uc := &ServeMasterPlaylistUseCase{}

	mediaItem := &media.Media{
		Width:   3840,
		Height:  2160,
		Bitrate: 80_000_000, // 80 Mbps source
	}

	params := buildVariantParams{
		strategy: strategy.RemuxHEVC, // Will be overridden to transcode in serve_manifest
	}

	// Non-original quality on remux source: manifest uses profile bitrate
	// (serve_manifest.go will switch to transcode, so we report the transcode bitrate)
	playlist := uc.buildSingleVariantPlaylist(mediaItem, profile.Quality4k40m, nil, nil, params, "", "")

	// Should use profile bitrate (40 Mbps), not source
	variant, _ := profile.GetQualityVariant(profile.Quality4k40m)
	expectedBandwidth := fmt.Sprintf("BANDWIDTH=%d", variant.Bandwidth)
	if !strings.Contains(playlist, expectedBandwidth) {
		t.Errorf("expected playlist to contain %s for non-original quality, got:\n%s", expectedBandwidth, playlist)
	}
}

func TestBuildSingleVariantPlaylist_UsesProfileBitrateForTranscode(t *testing.T) {
	uc := &ServeMasterPlaylistUseCase{}

	mediaItem := &media.Media{
		Width:   3840,
		Height:  2160,
		Bitrate: 80_000_000, // 80 Mbps source
	}

	params := buildVariantParams{
		strategy: strategy.Transcode,
	}

	playlist := uc.buildSingleVariantPlaylist(mediaItem, profile.Quality4k40m, nil, nil, params, "", "")

	// For transcode, should use profile bitrate (40 Mbps from 4k-40m)
	variant, _ := profile.GetQualityVariant(profile.Quality4k40m)
	expectedBandwidth := fmt.Sprintf("BANDWIDTH=%d", variant.Bandwidth)
	if !strings.Contains(playlist, expectedBandwidth) {
		t.Errorf("expected playlist to contain %s for transcode strategy, got:\n%s", expectedBandwidth, playlist)
	}
}
