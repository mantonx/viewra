package library

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/domain/scanner/parsers"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/nfo"
)

// processTVEpisode creates or updates a TV episode entry
func (uc *ScanLibraryUseCase) processTVEpisode(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, existingMediaCache *sync.Map) (*int64, error) {
	// Handle nil checkpoint (legacy code path or tests)
	if checkpoint == nil {
		checkpoint = &scanner.ScanCheckpoint{
			FilePath: result.FilePath,
			FileHash: "",
			FileSize: 0,
		}
	}

	// Skip audio files in TV libraries - they can't be episodes (e.g., theme.mp3, soundtrack files)
	// Audio files should only be processed in Music libraries
	ext := strings.ToLower(strings.TrimPrefix(result.FilePath[strings.LastIndex(result.FilePath, "."):], "."))
	if audioExtensions[ext] { // Uses package-level var from scan_utils.go
		// Return nil media ID but no error - this file is intentionally skipped
		return nil, nil
	}

	// Use coordinator's parsed data directly (no duplicate parsing needed!)
	showTitle := result.ShowName
	season := 0
	episodeNumber := 0
	episodeTitle := result.Title

	// Use coordinator's parsed season/episode numbers
	if result.SeasonNumber != nil {
		season = *result.SeasonNumber
	}

	if result.EpisodeNumber != nil {
		episodeNumber = *result.EpisodeNumber
	}

	// Fallback: If coordinator didn't populate show name, parse as last resort
	// This should rarely happen, but handles edge cases
	if showTitle == "" {
		parser := parsers.NewDefaultParser()
		tvInfo, err := parser.ParseTVEpisode(result.FilePath)
		if err != nil || tvInfo == nil {
			return nil, fmt.Errorf("failed to parse TV episode filename: %w", err)
		}
		showTitle = tvInfo.ShowName
		if season == 0 {
			season = tvInfo.Season
		}
		if episodeNumber == 0 {
			episodeNumber = tvInfo.Episode
		}
		if episodeTitle == "" {
			episodeTitle = tvInfo.EpisodeTitle
		}
	}

	episode := &media.TVEpisode{
		Media: media.Media{
			LibraryID:       libraryID,
			Title:           episodeTitle,
			FilePath:        result.FilePath,
			FileSize:        checkpoint.FileSize,
			FileHash:        checkpoint.FileHash,
			Duration:        int(result.Duration),
			IsExtra:         isExtra(result.FilePath),
			Width:           result.Width,
			Height:          result.Height,
			VideoCodec:      result.VideoCodec,
			AudioCodec:      result.AudioCodec,
			Bitrate:         result.Bitrate,
			FrameRate:       result.FrameRate,
			ContainerFormat: result.ContainerFormat,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		ShowTitle:    showTitle,
		Season:       season,
		Episode:      episodeNumber,
		EpisodeTitle: episodeTitle,
	}

	// Try to enhance metadata from NFO file
	nfoPath, err := nfo.FindEpisodeNFO(result.FilePath)
	if err == nil && nfoPath != "" {
		nfoMetadata, err := nfo.ParseEpisodeNFO(nfoPath)
		if err == nil && nfoMetadata != nil {
			// Populate episode metadata from NFO
			if nfoMetadata.Title != "" {
				episode.EpisodeTitle = nfoMetadata.Title
				episode.Media.Title = nfoMetadata.Title
			}
			if nfoMetadata.ShowTitle != "" {
				episode.ShowTitle = nfoMetadata.ShowTitle
			}
			if nfoMetadata.Season > 0 {
				episode.Season = nfoMetadata.Season
			}
			if nfoMetadata.Episode > 0 {
				episode.Episode = nfoMetadata.Episode
			}
			if nfoMetadata.Plot != "" {
				episode.Description = nfoMetadata.Plot
			}
			if !nfoMetadata.AirDate.IsZero() {
				episode.AirDate = nfoMetadata.AirDate.Format("2006-01-02")
			}
			if nfoMetadata.RuntimeMinutes > 0 {
				episode.Media.Duration = nfoMetadata.RuntimeMinutes * 60
			}
			episode.IMDbID = nfoMetadata.IMDbID
			episode.TVDbID = nfoMetadata.TVDbID
		}
	}

	// Check if episode already exists using in-memory cache (major performance optimization!)
	// This eliminates individual database SELECTs for every file
	if value, found := existingMediaCache.Load(result.FilePath); found {
		// Update existing entry
		episode.Media.ID = value.(int64)
		episode.Media.Type = "tv_episode"
		if err := uc.mediaRepos.Media.Update(ctx, &episode.Media); err != nil {
			return nil, fmt.Errorf("failed to update base media record: %w", err)
		}
		if err := uc.mediaRepos.TV.UpdateTVEpisode(ctx, episode); err != nil {
			return nil, fmt.Errorf("failed to update TV episode metadata: %w", err)
		}
		// Extract and catalog images (even for existing episodes to populate cache)
		uc.extractImagesForEpisode(ctx, episode, result.FilePath, libraryID)
		return &episode.Media.ID, nil
	}

	// Create new entry - let TV repository handle both media and episode records
	episode.Media.Type = "tv_episode"
	if err := uc.mediaRepos.TV.CreateTVEpisode(ctx, episode); err != nil {
		// Handle race condition: Another worker may have created this episode between our check and insert
		// TV episodes have a UNIQUE constraint on (show_id, season_number, episode_number)
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "duplicate key") {
			// Check cache again (another worker may have just added it)
			if value, found := existingMediaCache.Load(result.FilePath); found {
				// Update the existing record
				episode.Media.ID = value.(int64)
			} else {
				// Cache miss - fetch from database (race condition: created after our initial cache load)
				existing, fetchErr := uc.mediaRepos.Media.GetByFilePath(ctx, libraryID, result.FilePath)
				if fetchErr != nil || existing == nil {
					return nil, fmt.Errorf("failed to fetch existing media after collision: %w", fetchErr)
				}
				episode.Media.ID = existing.ID
				// Add to cache for future lookups
				existingMediaCache.Store(result.FilePath, existing.ID)
			}

			// Update the existing record
			episode.Media.Type = "tv_episode"
			if updateErr := uc.mediaRepos.Media.Update(ctx, &episode.Media); updateErr != nil {
				return nil, fmt.Errorf("failed to update base media record after collision: %w", updateErr)
			}
			if updateErr := uc.mediaRepos.TV.UpdateTVEpisode(ctx, episode); updateErr != nil {
				return nil, fmt.Errorf("failed to update TV episode metadata after collision: %w", updateErr)
			}
			uc.extractImagesForEpisode(ctx, episode, result.FilePath, libraryID)
			return &episode.Media.ID, nil
		}
		return nil, fmt.Errorf("failed to create base media record: %w", err)
	}

	// Add newly created media to cache so other workers don't try to create it again
	existingMediaCache.Store(result.FilePath, episode.Media.ID)

	// Extract and catalog images for the episode, show, and season
	// NOTE: Image extraction also triggers show metadata enrichment from NFO files
	uc.extractImagesForEpisode(ctx, episode, result.FilePath, libraryID)
	return &episode.Media.ID, nil
}

// enrichTVShowMetadataFromNFO attempts to load and update TV show metadata from tvshow.nfo files
// This is called after the show is created/found to populate rich metadata (year, genre, plot, IMDb/TMDb IDs, etc.)
func (uc *ScanLibraryUseCase) enrichTVShowMetadataFromNFO(ctx context.Context, showID int64, episodeFilePath string) {
	// Determine the show directory from the episode file path
	// TV shows can have two structures:
	// 1. With season subdirs: /path/to/show/Season 01/episode.mkv -> /path/to/show
	// 2. Without season subdirs: /path/to/show/episode.mkv -> /path/to/show
	showDir := nfo.DetermineShowDirectory(episodeFilePath)
	if showDir == "" {
		return // Unable to determine show directory
	}

	// Try to find tvshow.nfo in the show directory
	nfoPath, err := nfo.FindTVShowNFO(showDir)
	if err != nil || nfoPath == "" {
		// No NFO file found - this is fine, not all shows have metadata files
		return
	}

	// Parse the NFO file
	nfoMetadata, err := nfo.ParseTVShowNFO(nfoPath)
	if err != nil {
		uc.logger.Warn("failed to parse tvshow.nfo",
			"nfo_path", nfoPath,
			"show_id", showID,
			"error", err)
		return
	}

	// Get the current show to preserve fields we're not updating
	show, err := uc.mediaRepos.TV.GetTVShowByID(ctx, showID)
	if err != nil {
		uc.logger.Warn("failed to get TV show for metadata enrichment",
			"show_id", showID,
			"error", err)
		return
	}

	// Populate show metadata from NFO (only fields that exist in domain.TVShow)
	if nfoMetadata.Year > 0 {
		show.Year = nfoMetadata.Year
	}
	if len(nfoMetadata.Genre) > 0 {
		show.Genre = nfoMetadata.Genre
	}
	if nfoMetadata.Plot != "" {
		show.Plot = nfoMetadata.Plot
	}
	if nfoMetadata.ContentRating != "" {
		show.ContentRating = nfoMetadata.ContentRating
	}
	if nfoMetadata.IMDbID != "" {
		show.IMDbID = nfoMetadata.IMDbID
	}
	if nfoMetadata.TMDbID > 0 {
		show.TMDbID = nfoMetadata.TMDbID
	}

	// Update the show in the database
	if err := uc.mediaRepos.TV.UpdateTVShow(ctx, show); err != nil {
		uc.logger.Warn("failed to update TV show with NFO metadata",
			"show_id", showID,
			"nfo_path", nfoPath,
			"error", err)
	}
}
