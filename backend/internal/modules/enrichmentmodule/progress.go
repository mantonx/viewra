package enrichmentmodule

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// EnrichmentProgressManager tracks progress of metadata and artwork enrichment
type EnrichmentProgressManager struct {
	db *gorm.DB
}

// NewEnrichmentProgressManager creates a new enrichment progress manager
func NewEnrichmentProgressManager(db *gorm.DB) *EnrichmentProgressManager {
	return &EnrichmentProgressManager{db: db}
}

// EnrichmentProgress represents the current enrichment progress for a media type
type EnrichmentProgress struct {
	MediaType           string             `json:"media_type"`
	TotalItems          int                `json:"total_items"`
	EnrichedItems       int                `json:"enriched_items"`
	PendingItems        int                `json:"pending_items"`
	FailedItems         int                `json:"failed_items"`
	ProgressPercentage  float64            `json:"progress_percentage"`
	EstimatedCompletion *time.Time         `json:"estimated_completion,omitempty"`
	LastUpdate          time.Time          `json:"last_update"`
	MediaBreakdown      MediaBreakdown     `json:"media_breakdown"`
	FieldProgress       map[string]float64 `json:"field_progress"` // percentage complete for each field type
	RecentActivity      []EnrichmentItem   `json:"recent_activity"`
}

// MediaBreakdown provides detailed statistics per media category
type MediaBreakdown struct {
	TVShows  *CategoryProgress `json:"tv_shows,omitempty"`
	Movies   *CategoryProgress `json:"movies,omitempty"`
	Music    *CategoryProgress `json:"music,omitempty"`
	Episodes *CategoryProgress `json:"episodes,omitempty"`
}

// CategoryProgress tracks progress for a specific media category
type CategoryProgress struct {
	Total           int                        `json:"total"`
	WithMetadata    int                        `json:"with_metadata"`
	WithArtwork     int                        `json:"with_artwork"`
	FullyEnriched   int                        `json:"fully_enriched"`
	PendingJobs     int                        `json:"pending_jobs"`
	FailedJobs      int                        `json:"failed_jobs"`
	MetadataFields  map[string]FieldProgress   `json:"metadata_fields"`
	ArtworkTypes    map[string]ArtworkProgress `json:"artwork_types"`
	QualityScore    float64                    `json:"quality_score"`
	LastEnrichment  *time.Time                 `json:"last_enrichment,omitempty"`
	EstimatedTime   *time.Duration             `json:"estimated_time,omitempty"`
}

// FieldProgress tracks completion for specific metadata fields
type FieldProgress struct {
	FieldName   string  `json:"field_name"`
	Total       int     `json:"total"`
	Populated   int     `json:"populated"`
	Percentage  float64 `json:"percentage"`
	Quality     string  `json:"quality"` // "excellent", "good", "poor", "missing"
	Sources     []string `json:"sources"` // TMDB, MusicBrainz, etc.
}

// ArtworkProgress tracks completion for specific artwork types
type ArtworkProgress struct {
	ArtworkType string  `json:"artwork_type"` // poster, backdrop, banner, etc.
	Total       int     `json:"total"`
	Available   int     `json:"available"`
	Percentage  float64 `json:"percentage"`
	Resolution  string  `json:"resolution"` // "hd", "sd", "mixed"
}

// EnrichmentItem represents a recently enriched item
type EnrichmentItem struct {
	MediaID     string    `json:"media_id"`
	MediaType   string    `json:"media_type"`
	Title       string    `json:"title"`
	Source      string    `json:"source"`      // TMDB, MusicBrainz, etc.
	Action      string    `json:"action"`      // "metadata_added", "artwork_downloaded", etc.
	Timestamp   time.Time `json:"timestamp"`
	Fields      []string  `json:"fields"`      // Fields that were enriched
	Quality     string    `json:"quality"`     // "high", "medium", "low"
}

// GetOverallProgress returns enrichment progress across all media types
func (epm *EnrichmentProgressManager) GetOverallProgress() (*EnrichmentProgress, error) {
	progress := &EnrichmentProgress{
		MediaType:      "all",
		LastUpdate:     time.Now(),
		FieldProgress:  make(map[string]float64),
		RecentActivity: []EnrichmentItem{},
	}

	// Get breakdown for each media type
	tvProgress, err := epm.getTVShowProgress()
	if err != nil {
		return nil, fmt.Errorf("failed to get TV show progress: %w", err)
	}
	progress.MediaBreakdown.TVShows = tvProgress

	movieProgress, err := epm.getMovieProgress()
	if err != nil {
		return nil, fmt.Errorf("failed to get movie progress: %w", err)
	}
	progress.MediaBreakdown.Movies = movieProgress

	musicProgress, err := epm.getMusicProgress()
	if err != nil {
		return nil, fmt.Errorf("failed to get music progress: %w", err)
	}
	progress.MediaBreakdown.Music = musicProgress

	episodeProgress, err := epm.getEpisodeProgress()
	if err != nil {
		return nil, fmt.Errorf("failed to get episode progress: %w", err)
	}
	progress.MediaBreakdown.Episodes = episodeProgress

	// Calculate overall totals
	progress.TotalItems = tvProgress.Total + movieProgress.Total + musicProgress.Total + episodeProgress.Total
	progress.EnrichedItems = tvProgress.FullyEnriched + movieProgress.FullyEnriched + musicProgress.FullyEnriched + episodeProgress.FullyEnriched
	progress.PendingItems = tvProgress.PendingJobs + movieProgress.PendingJobs + musicProgress.PendingJobs + episodeProgress.PendingJobs
	progress.FailedItems = tvProgress.FailedJobs + movieProgress.FailedJobs + musicProgress.FailedJobs + episodeProgress.FailedJobs

	if progress.TotalItems > 0 {
		progress.ProgressPercentage = float64(progress.EnrichedItems) / float64(progress.TotalItems) * 100
	}

	// Get recent activity
	progress.RecentActivity, err = epm.getRecentActivity(20)
	if err != nil {
		log.Printf("WARN: Failed to get recent enrichment activity: %v", err)
	}

	// Estimate completion time based on current rate
	if progress.PendingItems > 0 {
		avgRate := epm.calculateEnrichmentRate()
		if avgRate > 0 {
			remainingHours := float64(progress.PendingItems) / avgRate
			estimatedCompletion := time.Now().Add(time.Duration(remainingHours) * time.Hour)
			progress.EstimatedCompletion = &estimatedCompletion
		}
	}

	return progress, nil
}

// GetTVShowProgress returns enrichment progress specifically for TV shows
func (epm *EnrichmentProgressManager) GetTVShowProgress() (*EnrichmentProgress, error) {
	tvProgress, err := epm.getTVShowProgress()
	if err != nil {
		return nil, err
	}

	progress := &EnrichmentProgress{
		MediaType:          "tv_shows",
		TotalItems:         tvProgress.Total,
		EnrichedItems:      tvProgress.FullyEnriched,
		PendingItems:       tvProgress.PendingJobs,
		FailedItems:        tvProgress.FailedJobs,
		ProgressPercentage: float64(tvProgress.FullyEnriched) / float64(tvProgress.Total) * 100,
		LastUpdate:         time.Now(),
		FieldProgress:      make(map[string]float64),
	}

	// Calculate field-specific progress percentages
	for fieldName, fieldProgress := range tvProgress.MetadataFields {
		progress.FieldProgress[fieldName] = fieldProgress.Percentage
	}

	// Add artwork progress to field progress
	for artworkType, artworkProgress := range tvProgress.ArtworkTypes {
		progress.FieldProgress[artworkType] = artworkProgress.Percentage
	}

	progress.MediaBreakdown.TVShows = tvProgress

	// Get recent TV show activity
	var err2 error
	progress.RecentActivity, err2 = epm.getRecentActivityForType("tv_show", 10)
	if err2 != nil {
		log.Printf("WARN: Failed to get recent TV show activity: %v", err2)
	}

	// Estimate completion time
	if tvProgress.PendingJobs > 0 {
		avgRate := epm.calculateEnrichmentRateForType("tv_show")
		if avgRate > 0 {
			remainingHours := float64(tvProgress.PendingJobs) / avgRate
			estimatedCompletion := time.Now().Add(time.Duration(remainingHours) * time.Hour)
			progress.EstimatedCompletion = &estimatedCompletion
		}
	}

	return progress, nil
}

// getTVShowProgress calculates detailed TV show enrichment progress
func (epm *EnrichmentProgressManager) getTVShowProgress() (*CategoryProgress, error) {
	progress := &CategoryProgress{
		MetadataFields: make(map[string]FieldProgress),
		ArtworkTypes:   make(map[string]ArtworkProgress),
	}

	// Get total TV shows count
	var totalShows int64
	if err := epm.db.Model(&database.TVShow{}).Count(&totalShows).Error; err != nil {
		return nil, fmt.Errorf("failed to count TV shows: %w", err)
	}
	progress.Total = int(totalShows)

	if progress.Total == 0 {
		return progress, nil
	}

	// Count shows with metadata fields
	progress.MetadataFields["title"] = epm.getFieldProgress(&database.TVShow{}, "title", "title != ''")
	progress.MetadataFields["description"] = epm.getFieldProgress(&database.TVShow{}, "description", "description != ''")
	progress.MetadataFields["first_air_date"] = epm.getFieldProgress(&database.TVShow{}, "first_air_date", "first_air_date IS NOT NULL")
	progress.MetadataFields["tmdb_id"] = epm.getFieldProgress(&database.TVShow{}, "tmdb_id", "tmdb_id != ''")
	progress.MetadataFields["status"] = epm.getFieldProgress(&database.TVShow{}, "status", "status != '' AND status != 'Unknown'")

	// Count shows with artwork
	progress.ArtworkTypes["poster"] = epm.getArtworkProgress(&database.TVShow{}, "poster", "poster != ''")
	progress.ArtworkTypes["backdrop"] = epm.getArtworkProgress(&database.TVShow{}, "backdrop", "backdrop != ''")

	// Count shows with metadata (any enrichment data)
	var withMetadata int64
	epm.db.Model(&database.TVShow{}).
		Joins("JOIN media_enrichments ON media_enrichments.media_id = tv_shows.id AND media_enrichments.media_type = 'tv_show'").
		Distinct("tv_shows.id").
		Count(&withMetadata)
	progress.WithMetadata = int(withMetadata)

	// Count shows with artwork (from assets)
	var withArtwork int64
	epm.db.Table("tv_shows").
		Joins("JOIN media_assets ON media_assets.entity_id = tv_shows.id").
		Where("media_assets.entity_type = ? AND media_assets.asset_type IN ('poster', 'backdrop')", "tv_show").
		Distinct("tv_shows.id").
		Count(&withArtwork)
	progress.WithArtwork = int(withArtwork)

	// Count fully enriched shows (with both metadata and artwork)
	progress.FullyEnriched = int(math.Min(float64(progress.WithMetadata), float64(progress.WithArtwork)))

	// Count pending enrichment jobs
	var pendingJobs int64
	epm.db.Model(&database.MediaFile{}).
		Joins("JOIN enrichment_jobs ON enrichment_jobs.media_file_id = media_files.id").
		Where("media_files.media_type = 'episode' AND enrichment_jobs.status = 'pending'").
		Count(&pendingJobs)
	progress.PendingJobs = int(pendingJobs)

	// Count failed enrichment jobs
	var failedJobs int64
	epm.db.Model(&database.MediaFile{}).
		Joins("JOIN enrichment_jobs ON enrichment_jobs.media_file_id = media_files.id").
		Where("media_files.media_type = 'episode' AND enrichment_jobs.status = 'failed'").
		Count(&failedJobs)
	progress.FailedJobs = int(failedJobs)

	// Calculate quality score (weighted average of field completeness)
	totalFields := len(progress.MetadataFields) + len(progress.ArtworkTypes)
	if totalFields > 0 {
		scoreSum := 0.0
		for _, field := range progress.MetadataFields {
			scoreSum += field.Percentage
		}
		for _, artwork := range progress.ArtworkTypes {
			scoreSum += artwork.Percentage
		}
		progress.QualityScore = scoreSum / float64(totalFields)
	}

	// Get last enrichment timestamp
	var lastEnrichment time.Time
	err := epm.db.Model(&database.MediaEnrichment{}).
		Where("media_type = 'tv_show'").
		Order("updated_at DESC").
		Limit(1).
		Select("updated_at").
		Scan(&lastEnrichment).Error
	if err == nil && !lastEnrichment.IsZero() {
		progress.LastEnrichment = &lastEnrichment
	}

	return progress, nil
}

// getMovieProgress calculates movie enrichment progress
func (epm *EnrichmentProgressManager) getMovieProgress() (*CategoryProgress, error) {
	progress := &CategoryProgress{
		MetadataFields: make(map[string]FieldProgress),
		ArtworkTypes:   make(map[string]ArtworkProgress),
	}

	var totalMovies int64
	if err := epm.db.Model(&database.Movie{}).Count(&totalMovies).Error; err != nil {
		return nil, fmt.Errorf("failed to count movies: %w", err)
	}
	progress.Total = int(totalMovies)

	if progress.Total == 0 {
		return progress, nil
	}

	// Basic metadata fields for movies
	progress.MetadataFields["title"] = epm.getFieldProgress(&database.Movie{}, "title", "title != ''")
	progress.MetadataFields["release_date"] = epm.getFieldProgress(&database.Movie{}, "release_date", "release_date IS NOT NULL")

	return progress, nil
}

// getMusicProgress calculates music enrichment progress
func (epm *EnrichmentProgressManager) getMusicProgress() (*CategoryProgress, error) {
	progress := &CategoryProgress{
		MetadataFields: make(map[string]FieldProgress),
		ArtworkTypes:   make(map[string]ArtworkProgress),
	}

	var totalTracks int64
	if err := epm.db.Model(&database.Track{}).Count(&totalTracks).Error; err != nil {
		return nil, fmt.Errorf("failed to count tracks: %w", err)
	}
	progress.Total = int(totalTracks)

	if progress.Total == 0 {
		return progress, nil
	}

	// Basic metadata fields for music
	progress.MetadataFields["title"] = epm.getFieldProgress(&database.Track{}, "title", "title != ''")
	progress.MetadataFields["artist"] = epm.getFieldProgress(&database.Track{}, "artist_id", "artist_id IS NOT NULL")
	progress.MetadataFields["album"] = epm.getFieldProgress(&database.Track{}, "album_id", "album_id IS NOT NULL")

	return progress, nil
}

// getEpisodeProgress calculates episode enrichment progress  
func (epm *EnrichmentProgressManager) getEpisodeProgress() (*CategoryProgress, error) {
	progress := &CategoryProgress{
		MetadataFields: make(map[string]FieldProgress),
		ArtworkTypes:   make(map[string]ArtworkProgress),
	}

	var totalEpisodes int64
	if err := epm.db.Model(&database.Episode{}).Count(&totalEpisodes).Error; err != nil {
		return nil, fmt.Errorf("failed to count episodes: %w", err)
	}
	progress.Total = int(totalEpisodes)

	if progress.Total == 0 {
		return progress, nil
	}

	// Basic metadata fields for episodes
	progress.MetadataFields["title"] = epm.getFieldProgress(&database.Episode{}, "title", "title != ''")
	progress.MetadataFields["description"] = epm.getFieldProgress(&database.Episode{}, "description", "description != ''")
	progress.MetadataFields["air_date"] = epm.getFieldProgress(&database.Episode{}, "air_date", "air_date IS NOT NULL")

	return progress, nil
}

// getFieldProgress calculates progress for a specific field
func (epm *EnrichmentProgressManager) getFieldProgress(model interface{}, fieldName, whereClause string) FieldProgress {
	var total, populated int64

	// Count total records
	epm.db.Model(model).Count(&total)
	
	// Count populated records
	if whereClause != "" {
		epm.db.Model(model).Where(whereClause).Count(&populated)
	}

	percentage := 0.0
	if total > 0 {
		percentage = float64(populated) / float64(total) * 100
	}

	quality := "missing"
	if percentage >= 90 {
		quality = "excellent"
	} else if percentage >= 70 {
		quality = "good"
	} else if percentage >= 30 {
		quality = "poor"
	}

	return FieldProgress{
		FieldName:  fieldName,
		Total:      int(total),
		Populated:  int(populated),
		Percentage: percentage,
		Quality:    quality,
	}
}

// getArtworkProgress calculates artwork progress for a specific media type
func (epm *EnrichmentProgressManager) getArtworkProgress(model interface{}, fieldName, whereClause string) ArtworkProgress {
	var total, available int64

	// Count total records
	epm.db.Model(model).Count(&total)
	
	// Count available records
	if whereClause != "" {
		epm.db.Model(model).Where(whereClause).Count(&available)
	}

	percentage := 0.0
	if total > 0 {
		percentage = float64(available) / float64(total) * 100
	}

	resolution := "mixed"
	if percentage >= 90 {
		resolution = "hd"
	} else if percentage >= 70 {
		resolution = "sd"
	}

	return ArtworkProgress{
		ArtworkType: fieldName,
		Total:       int(total),
		Available:   int(available),
		Percentage:  percentage,
		Resolution:  resolution,
	}
}

// getRecentActivity returns recent enrichment activity
func (epm *EnrichmentProgressManager) getRecentActivity(limit int) ([]EnrichmentItem, error) {
	var items []EnrichmentItem

	// Get recent completed enrichment jobs
	var jobs []EnrichmentJob
	err := epm.db.Where("status = 'completed'").
		Order("updated_at DESC").
		Limit(limit).
		Find(&jobs).Error
	if err != nil {
		return nil, err
	}

	for _, job := range jobs {
		// Get media file info
		var mediaFile database.MediaFile
		if err := epm.db.Where("id = ?", job.MediaFileID).First(&mediaFile).Error; err != nil {
			continue
		}

		item := EnrichmentItem{
			MediaID:   mediaFile.MediaID,
			MediaType: string(mediaFile.MediaType),
			Action:    "enrichment_completed",
			Timestamp: job.UpdatedAt,
			Quality:   "medium", // Could be calculated based on results
		}

		// Try to get title based on media type
		switch mediaFile.MediaType {
		case "episode":
			var episode database.Episode
			if err := epm.db.Where("id = ?", mediaFile.MediaID).First(&episode).Error; err == nil {
				item.Title = episode.Title
			}
		case "track":
			var track database.Track
			if err := epm.db.Where("id = ?", mediaFile.MediaID).First(&track).Error; err == nil {
				item.Title = track.Title
			}
		case "movie":
			var movie database.Movie
			if err := epm.db.Where("id = ?", mediaFile.MediaID).First(&movie).Error; err == nil {
				item.Title = movie.Title
			}
		}

		items = append(items, item)
	}

	return items, nil
}

// getRecentActivityForType returns recent activity for a specific media type
func (epm *EnrichmentProgressManager) getRecentActivityForType(mediaType string, limit int) ([]EnrichmentItem, error) {
	items, err := epm.getRecentActivity(limit * 2) // Get more then filter
	if err != nil {
		return nil, err
	}

	var filtered []EnrichmentItem
	for _, item := range items {
		if item.MediaType == mediaType && len(filtered) < limit {
			filtered = append(filtered, item)
		}
	}

	return filtered, nil
}

// calculateEnrichmentRate calculates the average enrichment rate (items per hour)
func (epm *EnrichmentProgressManager) calculateEnrichmentRate() float64 {
	// Look at completed jobs from the last 24 hours
	since := time.Now().Add(-24 * time.Hour)
	
	var count int64
	epm.db.Model(&EnrichmentJob{}).
		Where("status = 'completed' AND updated_at > ?", since).
		Count(&count)

	if count > 0 {
		return float64(count) / 24.0 // items per hour
	}
	return 0
}

// calculateEnrichmentRateForType calculates the enrichment rate for a specific media type
func (epm *EnrichmentProgressManager) calculateEnrichmentRateForType(mediaType string) float64 {
	since := time.Now().Add(-24 * time.Hour)
	
	var count int64
	epm.db.Model(&EnrichmentJob{}).
		Joins("JOIN media_files ON media_files.id = enrichment_jobs.media_file_id").
		Where("enrichment_jobs.status = 'completed' AND enrichment_jobs.updated_at > ? AND media_files.media_type = ?", since, mediaType).
		Count(&count)

	if count > 0 {
		return float64(count) / 24.0 // items per hour
	}
	return 0
} 