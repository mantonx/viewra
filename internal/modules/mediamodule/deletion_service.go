package mediamodule

import (
	"fmt"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"gorm.io/gorm"
)

// LibraryDeletionService handles comprehensive library deletion
type LibraryDeletionService struct {
	db             *gorm.DB
	eventBus       events.EventBus
	scannerManager ScannerManagerInterface // Changed from interface{} to proper type
}

// DeletionResult contains the results of a library deletion operation
type DeletionResult struct {
	LibraryID           uint32            `json:"library_id"`
	LibraryPath         string            `json:"library_path"`
	LibraryType         string            `json:"library_type"`
	Success             bool              `json:"success"`
	Message             string            `json:"message"`
	Error               error             `json:"error,omitempty"`
	CleanupStats        *CleanupStats     `json:"cleanup_stats"`
	Duration            time.Duration     `json:"duration"`
	Timestamp           time.Time         `json:"timestamp"`
}

// CleanupStats tracks what was cleaned up during deletion
type CleanupStats struct {
	MediaFilesDeleted      int64 `json:"media_files_deleted"`
	TracksDeleted          int64 `json:"tracks_deleted"`
	AlbumsDeleted          int64 `json:"albums_deleted"`
	ArtistsDeleted         int64 `json:"artists_deleted"`
	EpisodesDeleted        int64 `json:"episodes_deleted"`
	SeasonsDeleted         int64 `json:"seasons_deleted"`
	TVShowsDeleted         int64 `json:"tv_shows_deleted"`
	MoviesDeleted          int64 `json:"movies_deleted"`
	AssetsDeleted          int64 `json:"assets_deleted"`
	EnrichmentsDeleted     int64 `json:"enrichments_deleted"`
	ExternalIDsDeleted     int64 `json:"external_ids_deleted"`
	RolesDeleted           int64 `json:"roles_deleted"`
	PeopleDeleted          int64 `json:"people_deleted"`
	ScanJobsDeleted        int64 `json:"scan_jobs_deleted"`
	AssetFilesRemoved      int64 `json:"asset_files_removed"`
	OrphanedFilesRemoved   int64 `json:"orphaned_files_removed"`
}

// ScannerManagerInterface defines the interface for scanner operations
type ScannerManagerInterface interface {
	GetAllScans() ([]database.ScanJob, error)
	TerminateScan(jobID uint32) error
	CleanupJobsByLibrary(libraryID uint32) (int64, error)
}

// NewLibraryDeletionService creates a new library deletion service
func NewLibraryDeletionService(db *gorm.DB, eventBus events.EventBus) *LibraryDeletionService {
	return &LibraryDeletionService{
		db:       db,
		eventBus: eventBus,
	}
}

// SetScannerManager sets the scanner manager for scan-related cleanup
func (lds *LibraryDeletionService) SetScannerManager(scannerManager ScannerManagerInterface) {
	lds.scannerManager = scannerManager
}

// DeleteLibrary performs comprehensive library deletion with all related data cleanup
func (lds *LibraryDeletionService) DeleteLibrary(libraryID uint32) *DeletionResult {
	startTime := time.Now()
	result := &DeletionResult{
		LibraryID:    libraryID,
		Success:      false,
		CleanupStats: &CleanupStats{},
		Timestamp:    startTime,
	}

	// Check if library exists and get details
	var library database.MediaLibrary
	if err := lds.db.First(&library, libraryID).Error; err != nil {
		result.Error = fmt.Errorf("library not found: %w", err)
		result.Message = "Library not found"
		result.Duration = time.Since(startTime)
		return result
	}

	result.LibraryPath = library.Path
	result.LibraryType = library.Type

	logger.Info("Starting comprehensive library deletion", "library_id", libraryID, "path", library.Path, "type", library.Type)

	// Step 1: Stop and cleanup active scans
	if err := lds.stopActiveScanJobs(libraryID, result.CleanupStats); err != nil {
		result.Error = err
		result.Message = "Failed to stop active scan jobs"
		result.Duration = time.Since(startTime)
		return result
	}

	// Step 2: Get all media file IDs for this library (for cleanup reference)
	var mediaFileIDs []string
	if err := lds.db.Model(&database.MediaFile{}).Where("library_id = ?", libraryID).Pluck("id", &mediaFileIDs).Error; err != nil {
		result.Error = fmt.Errorf("failed to get media file IDs: %w", err)
		result.Message = "Failed to get media file IDs for cleanup"
		result.Duration = time.Since(startTime)
		return result
	}

	logger.Info("Found media files to clean up", "library_id", libraryID, "file_count", len(mediaFileIDs))

	// Step 3: Comprehensive metadata cleanup
	if len(mediaFileIDs) > 0 {
		if err := lds.cleanupMetadata(libraryID, mediaFileIDs, result.CleanupStats); err != nil {
			result.Error = err
			result.Message = "Failed during metadata cleanup"
			result.Duration = time.Since(startTime)
			return result
		}
	}

	// Step 4: Delete all media files for this library
	if err := lds.cleanupMediaFiles(libraryID, result.CleanupStats); err != nil {
		result.Error = err
		result.Message = "Failed to cleanup media files"
		result.Duration = time.Since(startTime)
		return result
	}

	// Step 5: Cleanup orphaned assets and files
	if err := lds.cleanupOrphanedData(result.CleanupStats); err != nil {
		logger.Warn("Failed to cleanup some orphaned data", "error", err)
		// Don't fail the whole operation for orphaned cleanup issues
	}

	// Step 6: Delete the library record itself
	if err := lds.db.Delete(&library).Error; err != nil {
		result.Error = fmt.Errorf("failed to delete library record: %w", err)
		result.Message = "Failed to delete library record"
		result.Duration = time.Since(startTime)
		return result
	}

	// Success!
	result.Success = true
	result.Message = "Library deleted successfully"
	result.Duration = time.Since(startTime)

	logger.Info("Library deletion completed successfully", 
		"library_id", libraryID, 
		"duration", result.Duration,
		"media_files_deleted", result.CleanupStats.MediaFilesDeleted,
		"tracks_deleted", result.CleanupStats.TracksDeleted,
		"albums_deleted", result.CleanupStats.AlbumsDeleted,
		"artists_deleted", result.CleanupStats.ArtistsDeleted)

	// Publish deletion event
	lds.publishDeletionEvent(result)

	return result
}

// stopActiveScanJobs stops any active scan jobs for the library
func (lds *LibraryDeletionService) stopActiveScanJobs(libraryID uint32, stats *CleanupStats) error {
	if lds.scannerManager == nil {
		logger.Warn("Scanner manager not available, skipping scan job cleanup")
		return nil
	}

	// Get all active jobs for this library
	allJobs, err := lds.scannerManager.GetAllScans()
	if err != nil {
		return fmt.Errorf("failed to get scan jobs: %w", err)
	}

	var activeJobsForLibrary []database.ScanJob
	for _, job := range allJobs {
		if job.LibraryID == libraryID && (job.Status == "running" || job.Status == "paused") {
			activeJobsForLibrary = append(activeJobsForLibrary, job)
		}
	}

	// Stop active jobs
	if len(activeJobsForLibrary) > 0 {
		logger.Info("Stopping active scan jobs", "library_id", libraryID, "job_count", len(activeJobsForLibrary))

		for _, job := range activeJobsForLibrary {
			logger.Info("Terminating scan job", "job_id", job.ID, "status", job.Status)
			if err := lds.scannerManager.TerminateScan(job.ID); err != nil {
				logger.Warn("Failed to terminate scan job", "job_id", job.ID, "error", err)
			}
		}

		// Wait for scans to stop
		maxWaitTime := 10 * time.Second
		checkInterval := 500 * time.Millisecond
		waited := time.Duration(0)

		for waited < maxWaitTime {
			time.Sleep(checkInterval)
			waited += checkInterval

			stillActive := false
			currentJobs, checkErr := lds.scannerManager.GetAllScans()
			if checkErr == nil {
				for _, job := range currentJobs {
					if job.LibraryID == libraryID && job.Status == "running" {
						stillActive = true
						break
					}
				}
			}

			if !stillActive {
				logger.Info("All scans stopped successfully", "library_id", libraryID, "waited", waited)
				break
			}
		}

		if waited >= maxWaitTime {
			logger.Warn("Timeout waiting for scans to stop, proceeding with cleanup", "library_id", libraryID)
		}
	}

	// Cleanup scan jobs for this library
	jobsDeleted, err := lds.scannerManager.CleanupJobsByLibrary(libraryID)
	if err != nil {
		return fmt.Errorf("failed to cleanup scan jobs: %w", err)
	}
	stats.ScanJobsDeleted = jobsDeleted

	if jobsDeleted > 0 {
		logger.Info("Cleaned up scan jobs", "library_id", libraryID, "jobs_deleted", jobsDeleted)
	}

	return nil
}

// cleanupMetadata performs comprehensive cleanup of all metadata and entities
func (lds *LibraryDeletionService) cleanupMetadata(libraryID uint32, mediaFileIDs []string, stats *CleanupStats) error {
	logger.Info("Starting comprehensive metadata cleanup", "library_id", libraryID)

	// Get all media IDs for different media types from this library
	var episodeIDs, trackIDs, movieIDs []string

	// Get episode IDs from this library
	if err := lds.db.Model(&database.MediaFile{}).Where("library_id = ? AND media_type = ?", libraryID, "episode").Pluck("media_id", &episodeIDs).Error; err != nil {
		logger.Warn("Failed to get episode IDs", "error", err)
	} else {
		logger.Info("Found episodes to clean up", "count", len(episodeIDs))
	}

	// Get track IDs from this library
	if err := lds.db.Model(&database.MediaFile{}).Where("library_id = ? AND media_type = ?", libraryID, "track").Pluck("media_id", &trackIDs).Error; err != nil {
		logger.Warn("Failed to get track IDs", "error", err)
	} else {
		logger.Info("Found tracks to clean up", "count", len(trackIDs))
	}

	// Get movie IDs from this library
	if err := lds.db.Model(&database.MediaFile{}).Where("library_id = ? AND media_type = ?", libraryID, "movie").Pluck("media_id", &movieIDs).Error; err != nil {
		logger.Warn("Failed to get movie IDs", "error", err)
	} else {
		logger.Info("Found movies to clean up", "count", len(movieIDs))
	}

	allMediaIDs := append(append(episodeIDs, trackIDs...), movieIDs...)
	logger.Info("Total media entities to clean up", "episodes", len(episodeIDs), "tracks", len(trackIDs), "movies", len(movieIDs), "total", len(allMediaIDs))

	// Delete enrichments for all media in this library
	if len(allMediaIDs) > 0 {
		if enrichResult := lds.db.Where("media_id IN ?", allMediaIDs).Delete(&database.MediaEnrichment{}); enrichResult.Error != nil {
			logger.Warn("Failed to delete media enrichments", "error", enrichResult.Error)
		} else {
			stats.EnrichmentsDeleted = enrichResult.RowsAffected
			if enrichResult.RowsAffected > 0 {
				logger.Info("Deleted media enrichments", "count", enrichResult.RowsAffected)
			}
		}
	}

	// Delete external IDs for all media in this library
	if len(allMediaIDs) > 0 {
		if extIDResult := lds.db.Where("media_id IN ?", allMediaIDs).Delete(&database.MediaExternalIDs{}); extIDResult.Error != nil {
			logger.Warn("Failed to delete external IDs", "error", extIDResult.Error)
		} else {
			stats.ExternalIDsDeleted = extIDResult.RowsAffected
			if extIDResult.RowsAffected > 0 {
				logger.Info("Deleted external IDs", "count", extIDResult.RowsAffected)
			}
		}
	}

	// Delete media assets for all media in this library
	if len(allMediaIDs) > 0 {
		if assetResult := lds.db.Where("media_id IN ?", allMediaIDs).Delete(&database.MediaAsset{}); assetResult.Error != nil {
			logger.Warn("Failed to delete media assets", "error", assetResult.Error)
		} else {
			stats.AssetsDeleted = assetResult.RowsAffected
			if assetResult.RowsAffected > 0 {
				logger.Info("Deleted media assets", "count", assetResult.RowsAffected)
			}
		}
	}

	// Delete people roles for all media in this library
	if len(allMediaIDs) > 0 {
		if roleResult := lds.db.Where("media_id IN ?", allMediaIDs).Delete(&database.Roles{}); roleResult.Error != nil {
			logger.Warn("Failed to delete roles", "error", roleResult.Error)
		} else {
			stats.RolesDeleted = roleResult.RowsAffected
			if roleResult.RowsAffected > 0 {
				logger.Info("Deleted roles", "count", roleResult.RowsAffected)
			}
		}
	}

	// Cleanup TV show hierarchy
	if len(episodeIDs) > 0 {
		logger.Info("Cleaning up TV show hierarchy")
		lds.cleanupTVHierarchy(episodeIDs, stats)
	}

	// Cleanup music hierarchy
	if len(trackIDs) > 0 {
		logger.Info("Cleaning up music hierarchy")
		lds.cleanupMusicHierarchy(trackIDs, stats)
	}

	// Delete movies and their assets
	if len(movieIDs) > 0 {
		logger.Info("Cleaning up movies")
		
		// Delete movie assets first
		if movieAssetResult := lds.db.Where("media_id IN ? AND entity_type = ?", movieIDs, "movie").Delete(&database.MediaAsset{}); movieAssetResult.Error != nil {
			logger.Warn("Failed to delete movie assets", "error", movieAssetResult.Error)
		} else if movieAssetResult.RowsAffected > 0 {
			logger.Info("Deleted movie assets", "count", movieAssetResult.RowsAffected)
		}

		// Delete movies
		if movieResult := lds.db.Where("id IN ?", movieIDs).Delete(&database.Movie{}); movieResult.Error != nil {
			logger.Warn("Failed to delete movies", "error", movieResult.Error)
		} else {
			stats.MoviesDeleted = movieResult.RowsAffected
			if movieResult.RowsAffected > 0 {
				logger.Info("Deleted movies", "count", movieResult.RowsAffected)
			}
		}
	}

	// Cleanup orphaned people (people with no remaining roles)
	lds.cleanupOrphanedPeople(stats)

	logger.Info("Metadata cleanup completed", "library_id", libraryID)
	return nil
}

// cleanupTVHierarchy cleans up TV show hierarchy (episodes -> seasons -> shows)
func (lds *LibraryDeletionService) cleanupTVHierarchy(episodeIDs []string, stats *CleanupStats) {
	logger.Info("Cleaning up TV show hierarchy", "episode_count", len(episodeIDs))

	// Get season IDs from episodes first
	var seasonIDs []string
	if err := lds.db.Model(&database.Episode{}).Where("id IN ?", episodeIDs).Distinct("season_id").Pluck("season_id", &seasonIDs).Error; err != nil {
		logger.Warn("Failed to get season IDs from episodes", "error", err)
		// Try to continue with what we have
	}

	// Get TV show IDs from those seasons
	var tvShowIDs []string
	if len(seasonIDs) > 0 {
		if err := lds.db.Model(&database.Season{}).Where("id IN ?", seasonIDs).Distinct("tv_show_id").Pluck("tv_show_id", &tvShowIDs).Error; err != nil {
			logger.Warn("Failed to get TV show IDs from seasons", "error", err)
		}
	}

	// Delete episodes first (this should cascade if foreign keys are set up)
	if episodeResult := lds.db.Where("id IN ?", episodeIDs).Delete(&database.Episode{}); episodeResult.Error != nil {
		logger.Warn("Failed to delete episodes", "error", episodeResult.Error)
	} else {
		stats.EpisodesDeleted = episodeResult.RowsAffected
		logger.Info("Deleted episodes", "count", episodeResult.RowsAffected)
	}

	// Check for orphaned seasons and delete them
	if len(seasonIDs) > 0 {
		var seasonsToDelete []string
		for _, seasonID := range seasonIDs {
			var remainingEpisodes int64
			if err := lds.db.Model(&database.Episode{}).Where("season_id = ?", seasonID).Count(&remainingEpisodes).Error; err == nil && remainingEpisodes == 0 {
				seasonsToDelete = append(seasonsToDelete, seasonID)
			}
		}

		if len(seasonsToDelete) > 0 {
			if seasonResult := lds.db.Where("id IN ?", seasonsToDelete).Delete(&database.Season{}); seasonResult.Error != nil {
				logger.Warn("Failed to delete orphaned seasons", "error", seasonResult.Error)
			} else {
				stats.SeasonsDeleted = seasonResult.RowsAffected
				logger.Info("Deleted orphaned seasons", "count", seasonResult.RowsAffected)
			}
		}
	}

	// Check for orphaned TV shows and delete them
	if len(tvShowIDs) > 0 {
		var tvShowsToDelete []string
		for _, tvShowID := range tvShowIDs {
			var remainingSeasons int64
			if err := lds.db.Model(&database.Season{}).Where("tv_show_id = ?", tvShowID).Count(&remainingSeasons).Error; err == nil && remainingSeasons == 0 {
				tvShowsToDelete = append(tvShowsToDelete, tvShowID)
			}
		}

		if len(tvShowsToDelete) > 0 {
			// Before deleting TV shows, clean up any remaining assets
			if assetResult := lds.db.Where("media_id IN ? AND entity_type = ?", tvShowsToDelete, "tv_show").Delete(&database.MediaAsset{}); assetResult.Error != nil {
				logger.Warn("Failed to delete TV show assets before show deletion", "error", assetResult.Error)
			} else if assetResult.RowsAffected > 0 {
				logger.Info("Deleted TV show assets", "count", assetResult.RowsAffected)
			}

			if tvShowResult := lds.db.Where("id IN ?", tvShowsToDelete).Delete(&database.TVShow{}); tvShowResult.Error != nil {
				logger.Warn("Failed to delete orphaned TV shows", "error", tvShowResult.Error)
			} else {
				stats.TVShowsDeleted = tvShowResult.RowsAffected
				logger.Info("Deleted orphaned TV shows", "count", tvShowResult.RowsAffected)
			}
		}
	}
}

// cleanupMusicHierarchy cleans up music hierarchy (tracks -> albums -> artists)
func (lds *LibraryDeletionService) cleanupMusicHierarchy(trackIDs []string, stats *CleanupStats) {
	logger.Info("Cleaning up music hierarchy", "track_count", len(trackIDs))

	// Get album and artist IDs from tracks
	var albumIDs, artistIDs []string
	if err := lds.db.Model(&database.Track{}).Where("id IN ?", trackIDs).Pluck("album_id", &albumIDs).Error; err != nil {
		logger.Warn("Failed to get album IDs", "error", err)
	}
	if err := lds.db.Model(&database.Track{}).Where("id IN ?", trackIDs).Pluck("artist_id", &artistIDs).Error; err != nil {
		logger.Warn("Failed to get artist IDs", "error", err)
	}

	// Delete tracks first
	if trackResult := lds.db.Where("id IN ?", trackIDs).Delete(&database.Track{}); trackResult.Error != nil {
		logger.Warn("Failed to delete tracks", "error", trackResult.Error)
	} else {
		stats.TracksDeleted = trackResult.RowsAffected
	}

	// Check and delete orphaned albums
	var albumsToDelete []string
	for _, albumID := range albumIDs {
		var remainingTracks int64
		if err := lds.db.Model(&database.Track{}).Where("album_id = ?", albumID).Count(&remainingTracks).Error; err == nil && remainingTracks == 0 {
			albumsToDelete = append(albumsToDelete, albumID)
		}
	}

	if len(albumsToDelete) > 0 {
		if albumResult := lds.db.Where("id IN ?", albumsToDelete).Delete(&database.Album{}); albumResult.Error != nil {
			logger.Warn("Failed to delete orphaned albums", "error", albumResult.Error)
		} else {
			stats.AlbumsDeleted = albumResult.RowsAffected
		}
	}

	// Check and delete orphaned artists
	var artistsToDelete []string
	for _, artistID := range artistIDs {
		var remainingTracks, remainingAlbums int64
		if err := lds.db.Model(&database.Track{}).Where("artist_id = ?", artistID).Count(&remainingTracks).Error; err == nil && remainingTracks == 0 {
			if err := lds.db.Model(&database.Album{}).Where("artist_id = ?", artistID).Count(&remainingAlbums).Error; err == nil && remainingAlbums == 0 {
				artistsToDelete = append(artistsToDelete, artistID)
			}
		}
	}

	if len(artistsToDelete) > 0 {
		if artistResult := lds.db.Where("id IN ?", artistsToDelete).Delete(&database.Artist{}); artistResult.Error != nil {
			logger.Warn("Failed to delete orphaned artists", "error", artistResult.Error)
		} else {
			stats.ArtistsDeleted = artistResult.RowsAffected
		}
	}
}

// cleanupOrphanedPeople removes people with no remaining roles
func (lds *LibraryDeletionService) cleanupOrphanedPeople(stats *CleanupStats) {
	var orphanedPeople []string
	if err := lds.db.Raw(`
		SELECT p.id FROM people p 
		LEFT JOIN roles r ON p.id = r.person_id 
		WHERE r.person_id IS NULL
	`).Pluck("id", &orphanedPeople).Error; err != nil {
		logger.Warn("Failed to find orphaned people", "error", err)
		return
	}

	if len(orphanedPeople) > 0 {
		if peopleResult := lds.db.Where("id IN ?", orphanedPeople).Delete(&database.People{}); peopleResult.Error != nil {
			logger.Warn("Failed to delete orphaned people", "error", peopleResult.Error)
		} else {
			stats.PeopleDeleted = peopleResult.RowsAffected
		}
	}
}

// cleanupMediaFiles deletes all media files for the library
func (lds *LibraryDeletionService) cleanupMediaFiles(libraryID uint32, stats *CleanupStats) error {
	logger.Info("Deleting media files", "library_id", libraryID)

	mediaFilesResult := lds.db.Where("library_id = ?", libraryID).Delete(&database.MediaFile{})
	if mediaFilesResult.Error != nil {
		return fmt.Errorf("failed to delete media files: %w", mediaFilesResult.Error)
	}

	stats.MediaFilesDeleted = mediaFilesResult.RowsAffected
	if mediaFilesResult.RowsAffected > 0 {
		logger.Info("Deleted media files", "library_id", libraryID, "files_deleted", mediaFilesResult.RowsAffected)
	}

	return nil
}

// cleanupOrphanedData cleans up orphaned assets and files
func (lds *LibraryDeletionService) cleanupOrphanedData(stats *CleanupStats) error {
	if lds.scannerManager == nil {
		logger.Warn("Scanner manager not available, skipping orphaned data cleanup")
		return nil
	}

	logger.Info("Cleaning up orphaned assets and files")

	logger.Info("Orphaned data cleanup skipped - deprecated cleanup methods were removed")
	return nil
}

// publishDeletionEvent publishes a library deletion event
func (lds *LibraryDeletionService) publishDeletionEvent(result *DeletionResult) {
	if lds.eventBus == nil {
		return
	}

	eventType := events.EventInfo
	if !result.Success {
		eventType = events.EventScanFailed
	}

	deleteEvent := events.NewSystemEvent(
		eventType,
		"Media Library Deleted",
		fmt.Sprintf("%s media library at path %s has been removed", result.LibraryType, result.LibraryPath),
	)
	deleteEvent.Data = map[string]interface{}{
		"libraryId":      result.LibraryID,
		"path":           result.LibraryPath,
		"type":           result.LibraryType,
		"success":        result.Success,
		"duration":       result.Duration.String(),
		"cleanup_stats":  result.CleanupStats,
	}
	lds.eventBus.PublishAsync(deleteEvent)
} 