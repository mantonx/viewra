package transcode

import (
	"testing"

	"github.com/mantonx/viewra/internal/infrastructure/transcoding/profile"
)

func TestFilterVariants_MultipleQualitiesReturned(t *testing.T) {
	uc := &ServeMasterPlaylistUseCase{}

	tests := []struct {
		name          string
		sourceWidth   int
		sourceHeight  int
		sourceBitrate int64
		minExpected   int // minimum number of variants expected
		maxHeight     int // max height that should be included
	}{
		{
			name:          "1080p source returns multiple qualities",
			sourceWidth:   1920,
			sourceHeight:  1080,
			sourceBitrate: 20_000_000,
			minExpected:   5, // 360p, 480p, 720p-2m, 720p-4m, 1080p variants
			maxHeight:     1080,
		},
		{
			name:          "4K source includes 4K variants",
			sourceWidth:   3840,
			sourceHeight:  2160,
			sourceBitrate: 60_000_000,
			minExpected:   10, // All non-4K plus some 4K variants
			maxHeight:     2160,
		},
		{
			name:          "720p source excludes 1080p+",
			sourceWidth:   1280,
			sourceHeight:  720,
			sourceBitrate: 4_000_000,
			minExpected:   3, // 360p, 480p, 720p variants
			maxHeight:     720,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			variants := uc.filterVariants(profile.ABRLadder, tc.sourceWidth, tc.sourceHeight, tc.sourceBitrate)

			// Should return multiple variants
			if len(variants) < tc.minExpected {
				t.Errorf("expected at least %d variants, got %d", tc.minExpected, len(variants))
			}

			// All variants should be at or below max height
			for _, v := range variants {
				if v.Height > tc.maxHeight {
					t.Errorf("variant %s has height %d, exceeds max %d", v.ID, v.Height, tc.maxHeight)
				}
			}

			// Log what we got for debugging
			t.Logf("Got %d variants for %dx%d source:", len(variants), tc.sourceWidth, tc.sourceHeight)
			for _, v := range variants {
				t.Logf("  - %s (%dx%d @ %d bps)", v.ID, v.Width, v.Height, v.Bandwidth)
			}
		})
	}
}

func TestFilterVariants_AlwaysIncludesLowQualities(t *testing.T) {
	uc := &ServeMasterPlaylistUseCase{}

	// Even for 4K source, should include low quality fallbacks
	variants := uc.filterVariants(profile.ABRLadder, 3840, 2160, 100_000_000)

	has360p := false
	has480p := false
	for _, v := range variants {
		if v.Height == 360 {
			has360p = true
		}
		if v.Height == 480 {
			has480p = true
		}
	}

	if !has360p {
		t.Error("expected 360p fallback to be included")
	}
	if !has480p {
		t.Error("expected 480p fallback to be included")
	}
}
