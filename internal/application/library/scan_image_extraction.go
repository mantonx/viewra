package library

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/domain/media"
)

// extractImagesForMovie extracts and catalogs images for a movie
// Shared helper to eliminate duplication between create and update paths
func (uc *ScanLibraryUseCase) extractImagesForMovie(ctx context.Context, movie *media.Movie, filePath string) {
	if uc.movieImageExtractor == nil {
		return
	}

	mediaID := int(movie.Media.ID)
	if err := uc.movieImageExtractor.Execute(ctx, filePath, images.MediaTypeMovie, mediaID, &mediaID); err != nil {
		uc.logger.Warn("failed to extract images for movie",
			"file_path", filePath,
			"error", err)
		// Track as warning in scan_state for user visibility
		if setErr := uc.scanRepos.ScanState.SetWarning(ctx, movie.Media.LibraryID, filePath, err.Error(), "image_extraction"); setErr != nil {
			uc.logger.Warn("failed to set image extraction warning in scan_state",
				"file_path", filePath,
				"error", setErr)
		}
	}
}

// extractImagesForEpisode extracts images for a TV episode, its show, and season
// Shared helper to eliminate duplication between create and update paths
func (uc *ScanLibraryUseCase) extractImagesForEpisode(ctx context.Context, episode *media.TVEpisode, filePath string, libraryID int64) {
	if uc.episodeImageExtractor == nil {
		return
	}

	// Extract episode images
	mediaID := int(episode.Media.ID)
	if err := uc.episodeImageExtractor.Execute(ctx, filePath, images.MediaTypeTVEpisode, mediaID, &mediaID); err != nil {
		uc.logger.Warn("failed to extract images for episode",
			"file_path", filePath,
			"error", err)
		// Track as warning in scan_state for user visibility
		if setErr := uc.scanRepos.ScanState.SetWarning(ctx, libraryID, filePath, err.Error(), "image_extraction"); setErr != nil {
			uc.logger.Warn("failed to set image extraction warning in scan_state",
				"file_path", filePath,
				"error", setErr)
		}
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

	uc.logger.Info("extracting show images",
		"show_title", episode.ShowTitle,
		"episode_file", filePath,
		"episode_dir", episodeDir,
		"episode_dir_name", episodeDirName,
		"show_dir", showDir,
		"has_season_subdir", strings.HasPrefix(strings.ToLower(episodeDirName), "season"))

	uc.extractTVShowAndSeasonImages(ctx, episode.ShowTitle, libraryID, showDir, episode.Season)
}

// extractImagesForTrack extracts images for a music track (album and artist)
// Shared helper to eliminate duplication between create and update paths
func (uc *ScanLibraryUseCase) extractImagesForTrack(ctx context.Context, track *media.MusicTrack, filePath string) {
	// Extract album images
	if track.Album != "" && uc.albumImageExtractor != nil {
		albumDir := filepath.Dir(filePath)
		entityID := int(track.Media.ID)
		if err := uc.albumImageExtractor.Execute(ctx, albumDir, images.MediaTypeMusicAlbum, entityID); err != nil {
			uc.logger.Warn("failed to extract images for album",
				"album", track.Album,
				"file_path", filePath,
				"error", err)
			// Track as warning in scan_state for user visibility
			if setErr := uc.scanRepos.ScanState.SetWarning(ctx, track.Media.LibraryID, filePath, err.Error(), "image_extraction"); setErr != nil {
				uc.logger.Warn("failed to set image extraction warning in scan_state",
					"file_path", filePath,
					"error", setErr)
			}
		}
	}

	// Extract artist images (once per artist)
	if track.Artist != "" && uc.artistImageExtractor != nil {
		artistDir := filepath.Dir(filepath.Dir(filePath)) // Parent of album dir
		entityID := int(track.Media.ID)

		// Atomically check and mark artist as processed (prevents race condition)
		if uc.tryMarkArtistProcessed(track.Artist) {
			if err := uc.artistImageExtractor.Execute(ctx, artistDir, images.MediaTypeMusicArtist, entityID); err != nil {
				uc.logger.Warn("failed to extract artist images",
					"artist", track.Artist,
					"error", err)
				// Artist image extraction failures don't get tracked per-file since they're per-artist
			}
		}
	}
}

// extractTVShowAndSeasonImages extracts images for a TV show and season
// This is a helper to avoid code duplication between create and update paths
func (uc *ScanLibraryUseCase) extractTVShowAndSeasonImages(ctx context.Context, showTitle string, libraryID int64, showDir string, seasonNumber int) {
	// Get show ID by title (show was created/ensured by CreateTVEpisode)
	show, err := uc.mediaRepos.TV.GetTVShowByTitle(ctx, libraryID, showTitle)
	if err != nil {
		uc.logger.Warn("failed to get TV show for image extraction",
			"show_title", showTitle,
			"error", err)
		return
	}

	// Extract show images
	if uc.showImageExtractor != nil {
		uc.logger.Info("calling image extractor for show",
			"show_title", showTitle,
			"show_id", show.ID,
			"show_dir", showDir,
			"media_type", images.MediaTypeTVShow)

		if err := uc.showImageExtractor.Execute(ctx, showDir, images.MediaTypeTVShow, int(show.ID)); err != nil {
			uc.logger.Warn("failed to extract images for show",
				"show_title", showTitle,
				"show_dir", showDir,
				"error", err)
			// Show/season image extraction failures don't get tracked per-file since they're per-show/season
		} else {
			uc.logger.Info("successfully extracted images for show",
				"show_title", showTitle,
				"show_id", show.ID)
		}
	}

	// Get season ID
	season, err := uc.mediaRepos.TV.GetTVSeasonByShowAndNumber(ctx, show.ID, int64(seasonNumber))
	if err != nil {
		uc.logger.Warn("failed to get TV season for image extraction",
			"show_title", showTitle,
			"season", seasonNumber,
			"error", err)
		return
	}

	// Extract season images
	if uc.seasonImageExtractor != nil {
		if err := uc.seasonImageExtractor.Execute(ctx, showDir, seasonNumber, images.MediaTypeTVSeason, int(season.ID)); err != nil {
			uc.logger.Warn("failed to extract images for season",
				"show_title", showTitle,
				"season", seasonNumber,
				"error", err)
			// Show/season image extraction failures don't get tracked per-file since they're per-show/season
		}
	}
}

// tryMarkArtistProcessed atomically checks if an artist has been processed and marks it as processed
// Returns true if this is the first time the artist is being processed (caller should extract images)
// Returns false if the artist was already processed (caller should skip extraction)
// This uses LoadOrStore for atomic check-and-set to prevent race conditions
func (uc *ScanLibraryUseCase) tryMarkArtistProcessed(artistName string) bool {
	_, alreadyProcessed := uc.processedArtists.LoadOrStore(artistName, true)
	return !alreadyProcessed // Return true if this is the first time
}
