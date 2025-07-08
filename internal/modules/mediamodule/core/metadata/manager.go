package metadata

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// Manager handles metadata operations for media files
type Manager struct {
	db *gorm.DB
}

// NewManager creates a new metadata manager
func NewManager(db *gorm.DB) *Manager {
	return &Manager{
		db: db,
	}
}

// UpdateMetadata updates metadata for a media file
func (m *Manager) UpdateMetadata(ctx context.Context, fileID string, metadata map[string]string) error {
	// First, get the media file to determine its type
	var mediaFile database.MediaFile
	if err := m.db.WithContext(ctx).Where("id = ?", fileID).First(&mediaFile).Error; err != nil {
		return fmt.Errorf("failed to get media file: %w", err)
	}

	// Update based on media type
	switch mediaFile.MediaType {
	case database.MediaTypeMovie:
		return m.updateMovieMetadata(ctx, mediaFile.MediaID, metadata)
	case database.MediaTypeEpisode:
		return m.updateEpisodeMetadata(ctx, mediaFile.MediaID, metadata)
	case database.MediaTypeTrack:
		return m.updateTrackMetadata(ctx, mediaFile.MediaID, metadata)
	default:
		// For generic files, update the MediaFile itself
		return m.updateGenericMetadata(ctx, fileID, metadata)
	}
}

// updateMovieMetadata updates metadata for a movie
func (m *Manager) updateMovieMetadata(ctx context.Context, movieID string, metadata map[string]string) error {
	updates := make(map[string]interface{})
	
	// Map metadata fields to database columns
	if title, ok := metadata["title"]; ok {
		updates["title"] = title
	}
	// Note: Movie model doesn't have Year field, use ReleaseDate instead
	if year, ok := metadata["year"]; ok {
		updates["release_date"] = year
	}
	if overview, ok := metadata["overview"]; ok {
		updates["overview"] = overview
	}
	if imdbID, ok := metadata["imdb_id"]; ok {
		updates["imdb_id"] = imdbID
	}
	if tmdbID, ok := metadata["tmdb_id"]; ok {
		updates["tmdb_id"] = tmdbID
	}
	
	if len(updates) == 0 {
		return nil // Nothing to update
	}
	
	result := m.db.WithContext(ctx).Model(&database.Movie{}).Where("id = ?", movieID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update movie metadata: %w", result.Error)
	}
	
	return nil
}

// updateEpisodeMetadata updates metadata for an episode
func (m *Manager) updateEpisodeMetadata(ctx context.Context, episodeID string, metadata map[string]string) error {
	updates := make(map[string]interface{})
	
	// Map metadata fields to database columns
	if title, ok := metadata["title"]; ok {
		updates["title"] = title
	}
	if description, ok := metadata["description"]; ok {
		updates["description"] = description
	}
	if airDate, ok := metadata["air_date"]; ok {
		updates["air_date"] = airDate
	}
	
	if len(updates) == 0 {
		return nil // Nothing to update
	}
	
	result := m.db.WithContext(ctx).Model(&database.Episode{}).Where("id = ?", episodeID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update episode metadata: %w", result.Error)
	}
	
	return nil
}

// updateTrackMetadata updates metadata for a music track
func (m *Manager) updateTrackMetadata(ctx context.Context, trackID string, metadata map[string]string) error {
	updates := make(map[string]interface{})
	
	// Map metadata fields to database columns
	if title, ok := metadata["title"]; ok {
		updates["title"] = title
	}
	// Note: Track model uses foreign keys for artist relationships
	// These fields would need to be handled differently in a real implementation
	if duration, ok := metadata["duration"]; ok {
		updates["duration"] = duration
	}
	
	if len(updates) == 0 {
		return nil // Nothing to update
	}
	
	result := m.db.WithContext(ctx).Model(&database.Track{}).Where("id = ?", trackID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update track metadata: %w", result.Error)
	}
	
	return nil
}

// updateGenericMetadata updates metadata for generic media files
func (m *Manager) updateGenericMetadata(ctx context.Context, fileID string, metadata map[string]string) error {
	// For now, we can store some basic metadata in the MediaFile itself
	// In the future, this could be expanded to a separate metadata table
	
	updates := make(map[string]interface{})
	
	// Only update specific allowed fields
	if duration, ok := metadata["duration"]; ok {
		updates["duration"] = duration
	}
	
	if len(updates) == 0 {
		return nil // Nothing to update
	}
	
	result := m.db.WithContext(ctx).Model(&database.MediaFile{}).Where("id = ?", fileID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update generic metadata: %w", result.Error)
	}
	
	return nil
}

// GetMetadata retrieves metadata for a media file
func (m *Manager) GetMetadata(ctx context.Context, fileID string) (map[string]interface{}, error) {
	var mediaFile database.MediaFile
	if err := m.db.WithContext(ctx).Where("id = ?", fileID).First(&mediaFile).Error; err != nil {
		return nil, fmt.Errorf("failed to get media file: %w", err)
	}
	
	metadata := make(map[string]interface{})
	metadata["file_id"] = mediaFile.ID
	metadata["path"] = mediaFile.Path
	metadata["media_type"] = mediaFile.MediaType
	metadata["size_bytes"] = mediaFile.SizeBytes
	
	// Get type-specific metadata
	switch mediaFile.MediaType {
	case database.MediaTypeMovie:
		movie := &database.Movie{}
		if err := m.db.WithContext(ctx).Where("id = ?", mediaFile.MediaID).First(movie).Error; err == nil {
			metadata["title"] = movie.Title
			metadata["release_date"] = movie.ReleaseDate
			metadata["overview"] = movie.Overview
			metadata["imdb_id"] = movie.ImdbID
			metadata["tmdb_id"] = movie.TmdbID
		}
	case database.MediaTypeEpisode:
		episode := &database.Episode{}
		if err := m.db.WithContext(ctx).Where("id = ?", mediaFile.MediaID).First(episode).Error; err == nil {
			metadata["title"] = episode.Title
			metadata["description"] = episode.Description
			metadata["air_date"] = episode.AirDate
			// Note: season_number would need to be accessed through episode.Season.SeasonNumber
			metadata["episode_number"] = episode.EpisodeNumber
		}
	case database.MediaTypeTrack:
		track := &database.Track{}
		if err := m.db.WithContext(ctx).Where("id = ?", mediaFile.MediaID).First(track).Error; err == nil {
			metadata["title"] = track.Title
			// Note: Artist info would need to be accessed through track.Artist.Name
			metadata["duration"] = track.Duration
			metadata["track_number"] = track.TrackNumber
		}
	}
	
	return metadata, nil
}