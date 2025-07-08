// Package repository provides data access layer for media operations
package repository

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// MediaRepository handles all database operations for media files
type MediaRepository struct {
	db *gorm.DB
}

// NewMediaRepository creates a new media repository
func NewMediaRepository(db *gorm.DB) *MediaRepository {
	return &MediaRepository{
		db: db,
	}
}

// GetByID retrieves a media file by ID
func (r *MediaRepository) GetByID(ctx context.Context, id string) (*database.MediaFile, error) {
	var file database.MediaFile
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&file).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("media file not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get media file: %w", err)
	}
	return &file, nil
}

// GetByPath retrieves a media file by path
func (r *MediaRepository) GetByPath(ctx context.Context, path string) (*database.MediaFile, error) {
	var file database.MediaFile
	if err := r.db.WithContext(ctx).Where("path = ?", path).First(&file).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("media file not found at path: %s", path)
		}
		return nil, fmt.Errorf("failed to get media file by path: %w", err)
	}
	return &file, nil
}

// List retrieves media files with a query
func (r *MediaRepository) List(ctx context.Context, query *gorm.DB) ([]*database.MediaFile, error) {
	var files []*database.MediaFile
	if err := query.WithContext(ctx).Find(&files).Error; err != nil {
		return nil, fmt.Errorf("failed to list media files: %w", err)
	}
	return files, nil
}

// Update updates a media file
func (r *MediaRepository) Update(ctx context.Context, id string, updates map[string]interface{}) error {
	result := r.db.WithContext(ctx).Model(&database.MediaFile{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update media file: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("media file not found: %s", id)
	}
	return nil
}

// Delete removes a media file
func (r *MediaRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&database.MediaFile{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete media file: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("media file not found: %s", id)
	}
	return nil
}

// GetDB returns the underlying database connection for query building
func (r *MediaRepository) GetDB() *gorm.DB {
	return r.db
}