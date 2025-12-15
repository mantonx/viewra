package media

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/domain/media"
)

// ExtractImagesForMovie extracts and catalogs images for a movie.
// Shared helper to eliminate duplication between create and update paths.
func ExtractImagesForMovie(ctx context.Context, deps *Deps, movie *media.Movie, filePath string) {
	if deps.MovieExtractor == nil {
		return
	}

	mediaID := int(movie.Media.ID)
	if err := deps.MovieExtractor.Execute(ctx, filePath, images.MediaTypeMovie, mediaID, &mediaID); err != nil {
		deps.Logger.Warn("failed to extract images for movie",
			"file_path", filePath,
			"error", err)
		recordImageWarning(ctx, deps, movie.Media.LibraryID, filePath, err)
	}
}

// ExtractImagesForEpisode extracts images for a TV episode, its show, and season.
// Shared helper to eliminate duplication between create and update paths.
func ExtractImagesForEpisode(ctx context.Context, deps *Deps, episode *media.TVEpisode, filePath string, libraryID int64) {
	if deps.EpisodeExtractor == nil {
		return
	}

	// Extract episode images
	mediaID := int(episode.Media.ID)
	if err := deps.EpisodeExtractor.Execute(ctx, filePath, images.MediaTypeTVEpisode, mediaID, &mediaID); err != nil {
		deps.Logger.Warn("failed to extract images for episode",
			"file_path", filePath,
			"error", err)
		recordImageWarning(ctx, deps, libraryID, filePath, err)
	}

	// Extract show and season images
	// TV shows can have two different directory structures:
	// 1. Episodes in show directory: /tv/ShowName (Year)/ShowName - S01E01.mkv
	// 2. Episodes in season subdirectories: /tv/ShowName/Season 01/ShowName - S01E01.mkv
	// Detect which structure is used by checking if parent directory name starts with "Season"
	episodeDir := filepath.Dir(filePath)
	episodeDirName := filepath.Base(episodeDir)

	showDir := episodeDir
	if strings.HasPrefix(strings.ToLower(episodeDirName), "season") {
		// Episode is in a season subdirectory, go up one more level to get show directory
		showDir = filepath.Dir(episodeDir)
	}

	extractTVShowAndSeasonImages(ctx, deps, episode.ShowTitle, libraryID, showDir, episode.Season)
}

// ExtractImagesForTrack extracts images for a music track (track, album, and artist).
// Shared helper to eliminate duplication between create and update paths.
func ExtractImagesForTrack(ctx context.Context, deps *Deps, track *media.MusicTrack, filePath string) {
	// Extract track-level images (embedded album art)
	if deps.TrackExtractor != nil {
		mediaID := int(track.Media.ID)
		if err := deps.TrackExtractor.Execute(ctx, filePath, images.MediaTypeMusicTrack, mediaID, &mediaID); err != nil {
			deps.Logger.Warn("failed to extract images for track",
				"track", track.Title,
				"file_path", filePath,
				"error", err)
			recordImageWarning(ctx, deps, track.Media.LibraryID, filePath, err)
		}
	}

	// Extract album images
	if track.Album != "" && track.AlbumID > 0 && deps.AlbumExtractor != nil {
		albumDir := filepath.Dir(filePath)
		entityID := int(track.AlbumID)
		if err := deps.AlbumExtractor.Execute(ctx, albumDir, images.MediaTypeMusicAlbum, entityID); err != nil {
			deps.Logger.Warn("failed to extract images for album",
				"album", track.Album,
				"album_id", track.AlbumID,
				"file_path", filePath,
				"error", err)
			recordImageWarning(ctx, deps, track.Media.LibraryID, filePath, err)
		}
	}

	// Extract artist images (once per artist)
	if track.Artist != "" && track.ArtistID > 0 && deps.ArtistExtractor != nil {
		artistDir := filepath.Dir(filepath.Dir(filePath)) // Parent of album dir
		entityID := int(track.ArtistID)

		// Atomically check and mark artist as processed (prevents race condition)
		if deps.ProcessedArtists.TryMark(track.Artist) {
			if err := deps.ArtistExtractor.Execute(ctx, artistDir, images.MediaTypeMusicArtist, entityID); err != nil {
				deps.Logger.Warn("failed to extract artist images",
					"artist", track.Artist,
					"artist_id", track.ArtistID,
					"error", err)
				// Artist image extraction failures don't get tracked per-file since they're per-artist
			}
		}
	}
}

// extractTVShowAndSeasonImages extracts images for a TV show and season.
// This is a helper to avoid code duplication between create and update paths.
func extractTVShowAndSeasonImages(ctx context.Context, deps *Deps, showTitle string, libraryID int64, showDir string, seasonNumber int) {
	// Get show ID by title (show was created/ensured by CreateTVEpisode)
	show, err := deps.MediaRepos.TV.GetTVShowByTitle(ctx, libraryID, showTitle)
	if err != nil {
		deps.Logger.Warn("failed to get TV show for image extraction",
			"show_title", showTitle,
			"error", err)
		return
	}

	// Enqueue show for enrichment (once per show per scan session)
	if deps.ProcessedShows != nil && deps.ProcessedShows.TryMark(showTitle) {
		enqueueForEnrichment(ctx, deps, show.ID, enrichment.MediaTypeTVShow)
	}

	// Extract show images
	if deps.ShowExtractor != nil {
		if err := deps.ShowExtractor.Execute(ctx, showDir, images.MediaTypeTVShow, int(show.ID)); err != nil {
			deps.Logger.Warn("failed to extract images for show",
				"show_title", showTitle,
				"show_dir", showDir,
				"error", err)
			// Show/season image extraction failures don't get tracked per-file since they're per-show/season
		}
	}

	// Get season ID
	season, err := deps.MediaRepos.TV.GetTVSeasonByShowAndNumber(ctx, show.ID, int64(seasonNumber))
	if err != nil {
		deps.Logger.Warn("failed to get TV season for image extraction",
			"show_title", showTitle,
			"season", seasonNumber,
			"error", err)
		return
	}

	// Extract season images
	if deps.SeasonExtractor != nil {
		if err := deps.SeasonExtractor.Execute(ctx, showDir, seasonNumber, images.MediaTypeTVSeason, int(season.ID)); err != nil {
			deps.Logger.Warn("failed to extract images for season",
				"show_title", showTitle,
				"season", seasonNumber,
				"error", err)
			// Show/season image extraction failures don't get tracked per-file since they're per-show/season
		}
	}
}

// recordImageWarning persists an image extraction warning to scan_state for user visibility.
// If the warning cannot be persisted (e.g., database error), it logs the failure but doesn't
// propagate the error - image extraction warnings are informational, not critical.
func recordImageWarning(ctx context.Context, deps *Deps, libraryID int64, filePath string, err error) {
	if setErr := deps.ScanRepos.ScanState.SetWarning(ctx, libraryID, filePath, err.Error(), "image_extraction"); setErr != nil {
		deps.Logger.Warn("failed to set image extraction warning in scan_state",
			"library_id", libraryID,
			"file_path", filePath,
			"original_error", err.Error(),
			"set_warning_error", setErr.Error(),
		)
	}
}
