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
			qualities := uc.buildAvailableQualities(tc.mediaItem, profile.Quality720p4m)

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

func TestBuildSingleVariantPlaylist_UsesSourceBitrateForRemux(t *testing.T) {
	uc := &ServeMasterPlaylistUseCase{}

	mediaItem := &media.Media{
		Width:   3840,
		Height:  2160,
		Bitrate: 80_000_000, // 80 Mbps source
	}

	params := buildVariantParams{
		strategy: strategy.RemuxHEVC,
	}

	playlist := uc.buildSingleVariantPlaylist(mediaItem, profile.Quality4k40m, nil, nil, params, "", "")

	// For remux, should use source bitrate (80 Mbps * 1.1 = 88 Mbps)
	// Profile 4k-40m would normally be 40 Mbps
	expectedBandwidth := "BANDWIDTH=88000000"
	if !strings.Contains(playlist, expectedBandwidth) {
		t.Errorf("expected playlist to contain %s for remux strategy, got:\n%s", expectedBandwidth, playlist)
	}

	// Should also use source resolution
	expectedResolution := "RESOLUTION=3840x2160"
	if !strings.Contains(playlist, expectedResolution) {
		t.Errorf("expected playlist to contain %s for remux strategy, got:\n%s", expectedResolution, playlist)
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
	variant, _ := profile.GetABRVariant(profile.Quality4k40m)
	expectedBandwidth := fmt.Sprintf("BANDWIDTH=%d", variant.Bandwidth)
	if !strings.Contains(playlist, expectedBandwidth) {
		t.Errorf("expected playlist to contain %s for transcode strategy, got:\n%s", expectedBandwidth, playlist)
	}
}
