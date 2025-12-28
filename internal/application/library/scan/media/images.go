package media

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/domain/media"
)

// EnqueueTVParentEntities enqueues TV show and season for enrichment.
// This should be called once per episode to ensure parent entities get their images/metadata.
// Deduplication ensures each show/season is only enqueued once per scan session.
// libraryID is passed through to enrichment events for SSE filtering.
// priority is inherited from the child episode for consistent ordering.
func EnqueueTVParentEntities(ctx context.Context, deps *Deps, showTitle string, libraryID int64, episodeFilePath string, seasonNumber int, priority int) {
	// Determine show directory from episode file path
	// TV shows can have two different directory structures:
	// 1. Episodes in show directory: /tv/ShowName (Year)/ShowName - S01E01.mkv
	// 2. Episodes in season subdirectories: /tv/ShowName/Season 01/ShowName - S01E01.mkv
	episodeDir := filepath.Dir(episodeFilePath)
	episodeDirName := filepath.Base(episodeDir)

	showDir := episodeDir
	if strings.HasPrefix(strings.ToLower(episodeDirName), "season") {
		// Episode is in a season subdirectory, go up one more level to get show directory
		showDir = filepath.Dir(episodeDir)
	}

	// Get show ID by title (show was created/ensured by CreateTVEpisode)
	show, err := deps.MediaRepos.TV.GetTVShowByTitle(ctx, libraryID, showTitle)
	if err != nil {
		deps.Logger.Warn("failed to get TV show for enrichment",
			"show_title", showTitle,
			"error", err)
		return
	}

	// Enqueue show for enrichment (once per show per scan session)
	// The enrichment pipeline will extract images and metadata via the LocalImagesEnricher
	if deps.ProcessedShows != nil && deps.ProcessedShows.TryMark(showTitle) {
		enqueueForEnrichment(ctx, deps, show.ID, libraryID, enrichment.MediaTypeTVShow, priority)
	}

	// Get season and enqueue for enrichment
	season, err := deps.MediaRepos.TV.GetTVSeasonByShowAndNumber(ctx, show.ID, int64(seasonNumber))
	if err != nil {
		deps.Logger.Warn("failed to get TV season for enrichment",
			"show_title", showTitle,
			"season", seasonNumber,
			"error", err)
		return
	}

	// Enqueue season for enrichment (once per season per scan session)
	// Use show:season as the key for deduplication
	seasonKey := showTitle + ":" + filepath.Base(showDir) + ":" + string(rune(seasonNumber+'0'))
	if deps.ProcessedShows != nil && deps.ProcessedShows.TryMark(seasonKey) {
		enqueueForEnrichment(ctx, deps, season.ID, libraryID, enrichment.MediaTypeTVSeason, priority)
	}
}

// EnqueueMusicParentEntities enqueues music album and artist for enrichment.
// This should be called once per track to ensure parent entities get their images/metadata.
// Deduplication ensures each album/artist is only enqueued once per scan session.
// libraryID is passed through to enrichment events for SSE filtering.
// priority is inherited from the child track for consistent ordering.
func EnqueueMusicParentEntities(ctx context.Context, deps *Deps, track *media.MusicTrack, libraryID int64, filePath string, priority int) {
	// Enqueue album for enrichment (if exists)
	if track.Album != "" && track.AlbumID > 0 {
		// Use album name + artist as key for deduplication
		albumKey := track.Album + ":" + track.Artist
		if deps.ProcessedArtists != nil && deps.ProcessedArtists.TryMark(albumKey) {
			enqueueForEnrichment(ctx, deps, track.AlbumID, libraryID, enrichment.MediaTypeMusicAlbum, priority)
		}
	}

	// Enqueue artist for enrichment (once per artist per scan session)
	if track.Artist != "" && track.ArtistID > 0 {
		if deps.ProcessedArtists != nil && deps.ProcessedArtists.TryMark(track.Artist) {
			enqueueForEnrichment(ctx, deps, track.ArtistID, libraryID, enrichment.MediaTypeMusicArtist, priority)
		}
	}
}
