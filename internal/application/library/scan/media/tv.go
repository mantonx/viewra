package media

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan/scanutil"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/domain/scanner/parsers"
)

// ProcessTVEpisode creates or updates a TV episode entry.
// For multi-episode files (e.g., S01E01-E02), it creates records for all episodes in the range.
func ProcessTVEpisode(
	ctx context.Context,
	deps *Deps,
	libraryID int64,
	result *scanner.ScanResult,
	checkpoint *scanner.ScanCheckpoint,
	existingMediaCache *sync.Map,
) (*int64, error) {
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
	if scanutil.IsAudioFile(ext) {
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
		return ProcessMultiEpisodeFile(ctx, deps, libraryID, result, checkpoint, existingMediaCache,
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
			IsExtra:         scanutil.IsExtra(result.FilePath),
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

	// NOTE: NFO metadata enrichment is now handled asynchronously by the enrichment pipeline.
	// The scanner saves minimal records; the NFO enricher populates rich metadata.

	// Use shared cache-based upsert pattern with race condition handling
	episode.Media.Type = "tv_episode"
	return ProcessMediaWithCache(ctx, deps, libraryID, result.FilePath, existingMediaCache, UpsertCallbacks{
		GetMediaID: func() int64 { return episode.Media.ID },
		SetMediaID: func(id int64) { episode.Media.ID = id },
		Update: func(ctx context.Context) error {
			if err := deps.MediaRepos.Media.Update(ctx, &episode.Media); err != nil {
				return fmt.Errorf("failed to update base media record: %w", err)
			}
			if err := deps.MediaRepos.TV.UpdateTVEpisode(ctx, episode); err != nil {
				return fmt.Errorf("failed to update TV episode metadata: %w", err)
			}
			return nil
		},
		Create: func(ctx context.Context) error {
			if err := deps.MediaRepos.TV.CreateTVEpisode(ctx, episode); err != nil {
				return fmt.Errorf("failed to create TV episode: %w", err)
			}
			return nil
		},
		PostSave: func(ctx context.Context) {
			// Extract and catalog images for the episode, show, and season
			// NOTE: Image extraction also triggers show metadata enrichment from NFO files
			ExtractImagesForEpisode(ctx, deps, episode, result.FilePath, libraryID)
			PersistMediaTracks(ctx, deps, episode.Media.ID, result)
			// Enqueue for enrichment if pipeline is configured
			enqueueForEnrichment(ctx, deps, episode.Media.ID, enrichment.MediaTypeTV)
		},
	})
}

// ProcessMultiEpisodeFile handles files that contain multiple episodes (e.g., S01E01-E02).
// It creates a media record for each episode in the range using virtual file paths
// (e.g., "/path/to/file.mkv#ep2" for the second episode).
// This allows the UI to display each episode separately while they all point to the same physical file.
func ProcessMultiEpisodeFile(
	ctx context.Context,
	deps *Deps,
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
				IsExtra:         scanutil.IsExtra(result.FilePath),
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
			if err := deps.MediaRepos.Media.Update(ctx, &episode.Media); err != nil {
				deps.Logger.Warn("failed to update multi-episode media record",
					"file_path", filePath,
					"episode", epNum,
					"error", err)
				continue
			}
			if err := deps.MediaRepos.TV.UpdateTVEpisode(ctx, episode); err != nil {
				deps.Logger.Warn("failed to update multi-episode TV episode metadata",
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
		if err := deps.MediaRepos.TV.CreateTVEpisode(ctx, episode); err != nil {
			// Handle UNIQUE constraint - episode may already exist
			if IsConstraintError(err) {
				deps.Logger.Debug("multi-episode already exists, skipping",
					"file_path", filePath,
					"season", season,
					"episode", epNum)
				continue
			}
			deps.Logger.Warn("failed to create multi-episode record",
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
			ExtractImagesForEpisode(ctx, deps, episode, result.FilePath, libraryID)
		}
	}

	return firstMediaID, nil
}
