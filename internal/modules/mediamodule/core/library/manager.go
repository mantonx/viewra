package library

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// Manager handles library operations
type Manager struct {
	db *gorm.DB
}

// NewManager creates a new library manager
func NewManager(db *gorm.DB) *Manager {
	return &Manager{
		db: db,
	}
}

// GetLibrary retrieves a library by ID
func (m *Manager) GetLibrary(ctx context.Context, id uint32) (*database.MediaLibrary, error) {
	var library database.MediaLibrary
	if err := m.db.WithContext(ctx).Where("id = ?", id).First(&library).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("library not found: %d", id)
		}
		return nil, fmt.Errorf("failed to get library: %w", err)
	}
	return &library, nil
}

// CreateLibrary creates a new media library
func (m *Manager) CreateLibrary(ctx context.Context, library *database.MediaLibrary) error {
	if err := m.db.WithContext(ctx).Create(library).Error; err != nil {
		return fmt.Errorf("failed to create library: %w", err)
	}
	return nil
}

// UpdateLibrary updates a media library
func (m *Manager) UpdateLibrary(ctx context.Context, id uint32, updates map[string]interface{}) error {
	result := m.db.WithContext(ctx).Model(&database.MediaLibrary{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update library: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("library not found: %d", id)
	}
	return nil
}

// DeleteLibrary deletes a media library
func (m *Manager) DeleteLibrary(ctx context.Context, id uint32) error {
	result := m.db.WithContext(ctx).Where("id = ?", id).Delete(&database.MediaLibrary{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete library: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("library not found: %d", id)
	}
	return nil
}

// ListLibraries lists all media libraries
func (m *Manager) ListLibraries(ctx context.Context) ([]*database.MediaLibrary, error) {
	var libraries []*database.MediaLibrary
	if err := m.db.WithContext(ctx).Find(&libraries).Error; err != nil {
		return nil, fmt.Errorf("failed to list libraries: %w", err)
	}
	return libraries, nil
}

// GetLibraryByPath retrieves a library by path
func (m *Manager) GetLibraryByPath(ctx context.Context, path string) (*database.MediaLibrary, error) {
	var library database.MediaLibrary
	if err := m.db.WithContext(ctx).Where("path = ?", path).First(&library).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("library not found at path: %s", path)
		}
		return nil, fmt.Errorf("failed to get library by path: %w", err)
	}
	return &library, nil
}