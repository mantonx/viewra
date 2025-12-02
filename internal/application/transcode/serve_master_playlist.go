package transcode

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding"
)

// ServeMasterPlaylistRequest represents a request for the master HLS playlist.
type ServeMasterPlaylistRequest struct {
	MediaID int64

	// Client capabilities for direct play decisions
	SupportedVideoCodecs []string // e.g., ["h264", "h265", "vp9", "av1"]
	SupportedContainers  []string // e.g., ["mp4", "webm", "matroska"]

	// StartPosition for seeking (passed through to variant URLs)
	StartPosition string
}

// ServeMasterPlaylistResponse represents the result.
type ServeMasterPlaylistResponse struct {
	// Strategy indicates what action to take
	Strategy MasterPlaylistStrategy

	// PlaylistContent is the generated M3U8 content (for StrategyServePlaylist)
	PlaylistContent string

	// DirectPlayURL is the URL for direct playback (for StrategyDirectPlay)
	DirectPlayURL string

	// Reason explains why this strategy was chosen
	Reason string
}

// MasterPlaylistStrategy indicates how to handle the master playlist request.
type MasterPlaylistStrategy int

const (
	// StrategyServePlaylist means master playlist was generated
	StrategyServePlaylist MasterPlaylistStrategy = iota

	// StrategyMasterDirectPlay means video is compatible and can be played directly
	StrategyMasterDirectPlay
)

// ABRVariant represents a single quality variant in the ABR ladder.
type ABRVariant struct {
	Quality   string
	Bandwidth int
	Width     int
	Height    int
	Codecs    string
}

// ServeMasterPlaylistUseCase handles generating the HLS master playlist.
type ServeMasterPlaylistUseCase struct {
	mediaRepo media.Repository
}

// NewServeMasterPlaylistUseCase creates a new use case.
func NewServeMasterPlaylistUseCase(mediaRepo media.Repository) *ServeMasterPlaylistUseCase {
	return &ServeMasterPlaylistUseCase{
		mediaRepo: mediaRepo,
	}
}

// Execute generates the master playlist or determines direct play is possible.
func (uc *ServeMasterPlaylistUseCase) Execute(ctx context.Context, req ServeMasterPlaylistRequest) (*ServeMasterPlaylistResponse, error) {
	// Get media to determine source resolution and properties
	mediaItem, err := uc.mediaRepo.GetByID(ctx, req.MediaID)
	if err != nil {
		return nil, fmt.Errorf("media not found: %w", err)
	}

	// Check if direct play is possible
	videoInfo, err := transcoding.GetVideoInfo(mediaItem.FilePath)
	if err == nil && videoInfo != nil {
		var clientCaps *transcoding.ClientCapabilitiesForStrategy
		if len(req.SupportedVideoCodecs) > 0 || len(req.SupportedContainers) > 0 {
			clientCaps = &transcoding.ClientCapabilitiesForStrategy{
				SupportedVideoCodecs: req.SupportedVideoCodecs,
				SupportedContainers:  req.SupportedContainers,
			}
		}

		strategy, reason := transcoding.DetermineStreamStrategyWithCapabilities(videoInfo, clientCaps)

		if strategy == transcoding.DirectPlay {
			return &ServeMasterPlaylistResponse{
				Strategy:      StrategyMasterDirectPlay,
				DirectPlayURL: fmt.Sprintf("/api/stream/%d", req.MediaID),
				Reason:        reason,
			}, nil
		}
	}

	// Build master playlist
	playlist := uc.buildMasterPlaylist(mediaItem, req.StartPosition)

	return &ServeMasterPlaylistResponse{
		Strategy:        StrategyServePlaylist,
		PlaylistContent: playlist,
		Reason:          "Master playlist generated with quality variants",
	}, nil
}

// buildMasterPlaylist creates the HLS master playlist content.
func (uc *ServeMasterPlaylistUseCase) buildMasterPlaylist(mediaItem *media.Media, startPosition string) string {
	sourceHeight := mediaItem.Height
	sourceWidth := mediaItem.Width
	sourceBitrate := mediaItem.Bitrate

	// Get ABR ladder filtered for this source
	variants := uc.getFilteredABRLadder(sourceWidth, sourceHeight, sourceBitrate)

	// Build playlist
	playlist := "#EXTM3U\n"
	playlist += "#EXT-X-VERSION:4\n"
	playlist += "#EXT-X-INDEPENDENT-SEGMENTS\n\n"

	for _, variant := range variants {
		playlist += fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"%s\",NAME=\"%s\"\n",
			variant.Bandwidth,
			variant.Width,
			variant.Height,
			variant.Codecs,
			variant.Quality,
		)

		variantURL := fmt.Sprintf("%s/playlist.m3u8", variant.Quality)
		if startPosition != "" {
			variantURL += "?start=" + startPosition
		}
		playlist += variantURL + "\n\n"
	}

	return playlist
}

// getFilteredABRLadder returns quality variants appropriate for the source media.
func (uc *ServeMasterPlaylistUseCase) getFilteredABRLadder(sourceWidth, sourceHeight int, sourceBitrate int64) []ABRVariant {
	// Comprehensive ABR ladder organized by resolution, then by bitrate
	abrLadder := []ABRVariant{
		// 360p - Low quality for poor connections
		{"360p", 800_000, 640, 360, "avc1.4d401e,mp4a.40.2"},

		// 480p - SD quality
		{"480p", 1_500_000, 854, 480, "avc1.4d401e,mp4a.40.2"},
		{"480p-2m", 2_000_000, 854, 480, "avc1.4d401e,mp4a.40.2"},

		// 720p - HD quality (multiple bitrate tiers)
		{"720p-3m", 3_000_000, 1280, 720, "avc1.64001f,mp4a.40.2"},
		{"720p", 4_000_000, 1280, 720, "avc1.64001f,mp4a.40.2"},
		{"720p-6m", 6_000_000, 1280, 720, "avc1.64001f,mp4a.40.2"},

		// 1080p - Full HD (multiple bitrate tiers)
		{"1080p-6m", 6_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},
		{"1080p", 8_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},
		{"1080p-12m", 12_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},
		{"1080p-15m", 15_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},
		{"1080p-20m", 20_000_000, 1920, 1080, "avc1.640028,mp4a.40.2"},

		// 4K - Ultra HD (many bitrate tiers for high-quality sources)
		{"4k-20m", 20_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
		{"4k-25m", 25_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
		{"4k-35m", 35_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
		{"4k-50m", 50_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
		{"4k-60m", 60_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
		{"4k-80m", 80_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
		{"4k-100m", 100_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
		{"4k-120m", 120_000_000, 3840, 2160, "avc1.640033,mp4a.40.2"},
	}

	// Add "Original" quality if we know the source bitrate
	if sourceBitrate > 0 && sourceWidth > 0 && sourceHeight > 0 {
		// Round source bitrate up to nearest 5 Mbps for cleaner display
		originalBitrate := ((sourceBitrate / 5_000_000) + 1) * 5_000_000
		if originalBitrate < sourceBitrate {
			originalBitrate = sourceBitrate
		}

		abrLadder = append(abrLadder, ABRVariant{
			Quality:   "original",
			Bandwidth: int(originalBitrate),
			Width:     sourceWidth,
			Height:    sourceHeight,
			Codecs:    "avc1.640033,mp4a.40.2",
		})
	}

	// Filter based on source resolution and bitrate
	return uc.filterVariants(abrLadder, sourceWidth, sourceHeight, sourceBitrate)
}

// filterVariants removes variants that don't make sense for the source media.
func (uc *ServeMasterPlaylistUseCase) filterVariants(variants []ABRVariant, sourceWidth, sourceHeight int, sourceBitrate int64) []ABRVariant {
	addedVariants := make(map[string]bool)
	var filtered []ABRVariant

	for _, variant := range variants {
		// Skip duplicates
		if addedVariants[variant.Quality] {
			continue
		}

		// Skip variants with bitrate higher than source (except "original")
		if variant.Quality != "original" && sourceBitrate > 0 && int64(variant.Bandwidth) > sourceBitrate {
			continue
		}

		// Skip qualities higher than source if we know resolution
		if sourceHeight > 0 && sourceWidth > 0 {
			// For 4K detection, use width as primary indicator
			// Ultrawide 4K content has full 4K width but reduced height
			is4KSource := sourceWidth >= 3840
			is4KVariant := variant.Width >= 3840

			// Skip 4K variants if source is not 4K
			if is4KVariant && !is4KSource && variant.Quality != "original" {
				continue
			}

			// For non-4K variants, use height-based comparison
			if !is4KVariant && variant.Height > sourceHeight && variant.Quality != "original" {
				continue
			}
		} else {
			// Source resolution unknown - include up to 1080p as safe default
			if variant.Height > 1080 {
				continue
			}
		}

		addedVariants[variant.Quality] = true
		filtered = append(filtered, variant)
	}

	return filtered
}
