package library

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/domain/media"
)

// extractImagesForMovie extracts and catalogs images for a movie
// Shared helper to eliminate duplication between create and update paths
func (uc *ScanLibraryUseCase) extractImagesForMovie(ctx context.Context, movie *media.Movie, filePath string) {
	if uc.imageExtractor == nil {
		return
	}

	mediaID := int(movie.Media.ID)
	opts := ImageExtractionOptions{
		FilePath:  filePath,
		MediaType: images.MediaTypeMovie,
		EntityID:  mediaID,
		MediaID:   &mediaID,
	}
	if err := uc.imageExtractor.Extract(ctx, opts); err != nil {
		fmt.Printf("failed to extract images for movie %s: %v\n", filePath, err)
	}
}

// extractImagesForEpisode extracts images for a TV episode, its show, and season
// Shared helper to eliminate duplication between create and update paths
func (uc *ScanLibraryUseCase) extractImagesForEpisode(ctx context.Context, episode *media.TVEpisode, filePath string, libraryID int64) {
	if uc.imageExtractor == nil {
		return
	}

	// Extract episode images
	mediaID := int(episode.Media.ID)
	opts := ImageExtractionOptions{
		FilePath:  filePath,
		MediaType: images.MediaTypeTVEpisode,
		EntityID:  mediaID,
		MediaID:   &mediaID,
	}
	if err := uc.imageExtractor.Extract(ctx, opts); err != nil {
		fmt.Printf("failed to extract images for episode %s: %v\n", filePath, err)
	}

	// Extract show and season images
	showDir := filepath.Dir(filepath.Dir(filePath))
	uc.extractTVShowAndSeasonImages(ctx, episode.ShowTitle, libraryID, showDir, episode.Season)
}

// extractImagesForTrack extracts images for a music track (album and artist)
// Shared helper to eliminate duplication between create and update paths
func (uc *ScanLibraryUseCase) extractImagesForTrack(ctx context.Context, track *media.MusicTrack, filePath string) {
	if uc.imageExtractor == nil {
		return
	}

	// Extract album images
	if track.Album != "" {
		albumDir := filepath.Dir(filePath)
		entityID := int(track.Media.ID)
		opts := ImageExtractionOptions{
			FilePath:  albumDir,
			MediaType: images.MediaTypeMusicAlbum,
			EntityID:  entityID,
		}
		if err := uc.imageExtractor.Extract(ctx, opts); err != nil {
			fmt.Printf("failed to extract images for album %s: %v\n", track.Album, err)
		}
	}

	// Extract artist images (once per artist)
	if track.Artist != "" {
		artistDir := filepath.Dir(filepath.Dir(filePath)) // Parent of album dir
		entityID := int(track.Media.ID)

		// Atomically check and mark artist as processed (prevents race condition)
		if uc.tryMarkArtistProcessed(track.Artist) {
			opts := ImageExtractionOptions{
				FilePath:  artistDir,
				MediaType: images.MediaTypeMusicArtist,
				EntityID:  entityID,
			}
			if err := uc.imageExtractor.Extract(ctx, opts); err != nil {
				fmt.Printf("failed to extract artist images for %s: %v\n", track.Artist, err)
			}
		}
	}
}

// extractTVShowAndSeasonImages extracts images for a TV show and season
// This is a helper to avoid code duplication between create and update paths
func (uc *ScanLibraryUseCase) extractTVShowAndSeasonImages(ctx context.Context, showTitle string, libraryID int64, showDir string, seasonNumber int) {
	if uc.imageExtractor == nil {
		return
	}

	// Get show ID by title (show was created/ensured by CreateTVEpisode)
	show, err := uc.mediaRepos.TV.GetTVShowByTitle(ctx, libraryID, showTitle)
	if err != nil {
		fmt.Printf("failed to get TV show for image extraction: %v\n", err)
		return
	}

	// Extract show images
	opts := ImageExtractionOptions{
		FilePath:  showDir,
		MediaType: images.MediaTypeTVShow,
		EntityID:  int(show.ID),
	}
	if err := uc.imageExtractor.Extract(ctx, opts); err != nil {
		fmt.Printf("failed to extract images for show %s: %v\n", showTitle, err)
	}

	// Get season ID
	season, err := uc.mediaRepos.TV.GetTVSeasonByShowAndNumber(ctx, show.ID, int64(seasonNumber))
	if err != nil {
		fmt.Printf("failed to get TV season for image extraction: %v\n", err)
		return
	}

	// Extract season images
	seasonOpts := ImageExtractionOptions{
		FilePath:  showDir,
		MediaType: images.MediaTypeTVSeason,
		EntityID:  int(season.ID),
		SeasonNum: seasonNumber,
	}
	if err := uc.imageExtractor.Extract(ctx, seasonOpts); err != nil {
		fmt.Printf("failed to extract images for season %d: %v\n", seasonNumber, err)
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
