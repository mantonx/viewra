package pipeline

import (
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/domain/images"
)

// intPtr returns a pointer to the given int value.
func intPtr(v int) *int { return &v }

// stringPtr returns a pointer to the given string value.
func stringPtr(v string) *string { return &v }

// mapToImageMediaType maps enrichment MediaType to images.MediaType.
func mapToImageMediaType(mt enrichment.MediaType) images.MediaType {
	switch mt {
	case enrichment.MediaTypeMovie:
		return images.MediaTypeMovie
	case enrichment.MediaTypeTV:
		return images.MediaTypeTVEpisode
	case enrichment.MediaTypeTVShow:
		return images.MediaTypeTVShow
	case enrichment.MediaTypeTVSeason:
		return images.MediaTypeTVSeason
	case enrichment.MediaTypeMusic:
		return images.MediaTypeMusicTrack
	case enrichment.MediaTypeMusicAlbum:
		return images.MediaTypeMusicAlbum
	case enrichment.MediaTypeMusicArtist:
		return images.MediaTypeMusicArtist
	default:
		return ""
	}
}

// isMediaTableEntity returns true if the media type has entries in the media table.
// Only movies, TV episodes, and music tracks have media.id references.
// Parent entities (tv_show, tv_season, music_album, music_artist) don't.
func isMediaTableEntity(mt enrichment.MediaType) bool {
	switch mt {
	case enrichment.MediaTypeMovie, enrichment.MediaTypeTV, enrichment.MediaTypeMusic:
		return true
	default:
		return false
	}
}

// convertToExternalIDMediaType converts enrichment MediaType to the format used by external_ids table.
// The external_ids table uses tv_episode instead of tv, and music_track instead of music.
func convertToExternalIDMediaType(mt enrichment.MediaType) enrichment.MediaType {
	switch mt {
	case enrichment.MediaTypeTV:
		return enrichment.MediaType("tv_episode")
	case enrichment.MediaTypeMusic:
		return enrichment.MediaType("music_track")
	default:
		return mt
	}
}
