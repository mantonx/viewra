// Package core provides the core business logic for the media module
package core

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/mediamodule/core/filters"
	"github.com/mantonx/viewra/internal/modules/mediamodule/core/library"
	"github.com/mantonx/viewra/internal/modules/mediamodule/core/metadata"
	"github.com/mantonx/viewra/internal/modules/mediamodule/core/repository"
	"github.com/mantonx/viewra/internal/modules/mediamodule/utils"
	"github.com/mantonx/viewra/internal/types"
	"gorm.io/gorm"
)

// MediaManager orchestrates all media-related operations
type MediaManager struct {
	repo            *repository.MediaRepository
	filter          *filters.MediaFilter
	libraryManager  *library.Manager
	metadataManager *metadata.Manager
}

// NewMediaManager creates a new media manager
func NewMediaManager(
	db *gorm.DB,
	libraryManager *library.Manager,
	metadataManager *metadata.Manager,
) *MediaManager {
	return &MediaManager{
		repo:            repository.NewMediaRepository(db),
		filter:          filters.NewMediaFilter(),
		libraryManager:  libraryManager,
		metadataManager: metadataManager,
	}
}

// GetFile retrieves a media file by ID
func (m *MediaManager) GetFile(ctx context.Context, id string) (*database.MediaFile, error) {
	return m.repo.GetByID(ctx, id)
}

// GetFileByPath retrieves a media file by path
func (m *MediaManager) GetFileByPath(ctx context.Context, path string) (*database.MediaFile, error) {
	return m.repo.GetByPath(ctx, path)
}

// ListFiles lists media files with filtering
func (m *MediaManager) ListFiles(ctx context.Context, filter types.MediaFilter) ([]*database.MediaFile, error) {
	// Start with base query
	query := m.repo.GetDB()
	
	// Apply all filters using the filter logic
	query = m.filter.ApplyFilter(query, filter)
	
	// Execute query through repository
	return m.repo.List(ctx, query)
}

// UpdateFile updates a media file
func (m *MediaManager) UpdateFile(ctx context.Context, id string, updates map[string]interface{}) error {
	return m.repo.Update(ctx, id, updates)
}

// DeleteFile deletes a media file
func (m *MediaManager) DeleteFile(ctx context.Context, id string) error {
	return m.repo.Delete(ctx, id)
}

// GetLibrary retrieves a library by ID
func (m *MediaManager) GetLibrary(ctx context.Context, id uint32) (*database.MediaLibrary, error) {
	return m.libraryManager.GetLibrary(ctx, id)
}

// UpdateMetadata updates metadata for a media file
func (m *MediaManager) UpdateMetadata(ctx context.Context, fileID string, metadata map[string]string) error {
	return m.metadataManager.UpdateMetadata(ctx, fileID, metadata)
}

// GetMediaInfo analyzes a media file and returns its information
func (m *MediaManager) GetMediaInfo(ctx context.Context, filePath string) (*types.MediaInfo, error) {
	return utils.ProbeMediaFile(filePath)
}

// GetPlaybackMethod determines the playback method for a media file
func (m *MediaManager) GetPlaybackMethod(ctx context.Context, fileID string) (string, error) {
	file, err := m.GetFile(ctx, fileID)
	if err != nil {
		return "", fmt.Errorf("failed to get media file: %w", err)
	}
	
	return string(m.filter.GetPlaybackMethod(file)), nil
}