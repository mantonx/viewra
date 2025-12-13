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
// For multi-episode files (e.g., S01E01-E02), it creates records for all episodes in the range
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
	episodeEndNumber := 0
	episodeTitle := result.Title

	// Use coordinator's parsed season/episode numbers
	if result.SeasonNumber != nil {
		season = *result.SeasonNumber
	}

	if result.EpisodeNumber != nil {
		episodeNumber = *result.EpisodeNumber
	}

	if result.EpisodeEndNumber != nil {
		episodeEndNumber = *result.EpisodeEndNumber
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
		if episodeEndNumber == 0 && tvInfo.EpisodeEnd > 0 {
			episodeEndNumber = tvInfo.EpisodeEnd
		}
		if episodeTitle == "" {
			episodeTitle = tvInfo.EpisodeTitle
		}
	}

	// Handle multi-episode files (e.g., S01E01-E02)
	// Create episode records for all episodes in the range
	if episodeEndNumber > episodeNumber {
		return uc.processMultiEpisodeFile(ctx, libraryID, result, checkpoint, existingMediaCache,
			showTitle, season, episodeNumber, episodeEndNumber, episodeTitle)
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

	// Use shared cache-based upsert pattern with race condition handling
	episode.Media.Type = "tv_episode"
	return uc.processMediaWithCache(ctx, libraryID, result.FilePath, existingMediaCache, MediaUpsertCallbacks{
		GetMediaID: func() int64 { return episode.Media.ID },
		SetMediaID: func(id int64) { episode.Media.ID = id },
		Update: func(ctx context.Context) error {
			if err := uc.mediaRepos.Media.Update(ctx, &episode.Media); err != nil {
				return fmt.Errorf("failed to update base media record: %w", err)
			}
			if err := uc.mediaRepos.TV.UpdateTVEpisode(ctx, episode); err != nil {
				return fmt.Errorf("failed to update TV episode metadata: %w", err)
			}
			return nil
		},
		Create: func(ctx context.Context) error {
			if err := uc.mediaRepos.TV.CreateTVEpisode(ctx, episode); err != nil {
				return fmt.Errorf("failed to create TV episode: %w", err)
			}
			return nil
		},
		PostSave: func(ctx context.Context) {
			// Extract and catalog images for the episode, show, and season
			// NOTE: Image extraction also triggers show metadata enrichment from NFO files
			uc.extractImagesForEpisode(ctx, episode, result.FilePath, libraryID)
			uc.persistMediaTracks(ctx, episode.Media.ID, result)
		},
	})
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

// processMultiEpisodeFile handles files that contain multiple episodes (e.g., S01E01-E02)
// It creates a media record for each episode in the range using virtual file paths
// (e.g., "/path/to/file.mkv#ep2" for the second episode)
// This allows the UI to display each episode separately while they all point to the same physical file
func (uc *ScanLibraryUseCase) processMultiEpisodeFile(
	ctx context.Context,
	libraryID int64,
	result *scanner.ScanResult,
	checkpoint *scanner.ScanCheckpoint,
	existingMediaCache *sync.Map,
	showTitle string,
	season int,
	episodeStart int,
	episodeEnd int,
	episodeTitle string,
) (*int64, error) {
	var firstMediaID *int64

	// Create an episode record for each episode in the range
	for epNum := episodeStart; epNum <= episodeEnd; epNum++ {
		// Generate a unique file path for each episode
		// First episode uses the real path, subsequent episodes use virtual paths
		filePath := result.FilePath
		if epNum > episodeStart {
			// Use a virtual path with episode marker (e.g., "/path/file.mkv#ep2")
			// This satisfies the UNIQUE constraint on file_path while clearly indicating
			// this is part of a multi-episode file
			filePath = fmt.Sprintf("%s#ep%d", result.FilePath, epNum)
		}

		// Generate episode title with part number for multi-episode files
		epTitle := episodeTitle
		if episodeTitle != "" && episodeEnd > episodeStart {
			partNum := epNum - episodeStart + 1
			epTitle = fmt.Sprintf("%s (Part %d)", episodeTitle, partNum)
		}

		episode := &media.TVEpisode{
			Media: media.Media{
				LibraryID:       libraryID,
				Title:           epTitle,
				FilePath:        filePath,
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
			Episode:      epNum,
			EpisodeTitle: epTitle,
		}

		// Check if this episode already exists in cache
		if value, found := existingMediaCache.Load(filePath); found {
			episode.Media.ID = value.(int64)
			episode.Media.Type = "tv_episode"
			if err := uc.mediaRepos.Media.Update(ctx, &episode.Media); err != nil {
				uc.logger.Warn("failed to update multi-episode media record",
					"file_path", filePath,
					"episode", epNum,
					"error", err)
				continue
			}
			if err := uc.mediaRepos.TV.UpdateTVEpisode(ctx, episode); err != nil {
				uc.logger.Warn("failed to update multi-episode TV episode metadata",
					"file_path", filePath,
					"episode", epNum,
					"error", err)
				continue
			}
			if firstMediaID == nil {
				firstMediaID = &episode.Media.ID
			}
			continue
		}

		// Create new episode record
		episode.Media.Type = "tv_episode"
		if err := uc.mediaRepos.TV.CreateTVEpisode(ctx, episode); err != nil {
			// Handle UNIQUE constraint - episode may already exist
			if isConstraintError(err) {
				uc.logger.Debug("multi-episode already exists, skipping",
					"file_path", filePath,
					"season", season,
					"episode", epNum)
				continue
			}
			uc.logger.Warn("failed to create multi-episode record",
				"file_path", filePath,
				"episode", epNum,
				"error", err)
			continue
		}

		// Cache the newly created episode
		existingMediaCache.Store(filePath, episode.Media.ID)

		if firstMediaID == nil {
			firstMediaID = &episode.Media.ID
			// Extract images only for the first episode (they're shared anyway)
			uc.extractImagesForEpisode(ctx, episode, result.FilePath, libraryID)
		}
	}

	return firstMediaID, nil
}
