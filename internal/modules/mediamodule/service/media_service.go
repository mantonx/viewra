package service

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/mediamodule/core"
	"github.com/mantonx/viewra/internal/modules/mediamodule/core/library"
	"github.com/mantonx/viewra/internal/modules/mediamodule/core/metadata"
	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
	"gorm.io/gorm"
)

// mediaServiceImpl implements the MediaService interface as a thin wrapper
// All business logic is delegated to the core MediaManager
type mediaServiceImpl struct {
	mediaManager *core.MediaManager
}

// NewMediaService creates a new media service implementation
func NewMediaService(
	db *gorm.DB,
	libraryManager *library.Manager,
	metadataManager *metadata.Manager,
) services.MediaService {
	return &mediaServiceImpl{
		mediaManager: core.NewMediaManager(db, libraryManager, metadataManager),
	}
}

// GetFile retrieves a media file by ID
func (s *mediaServiceImpl) GetFile(ctx context.Context, id string) (*database.MediaFile, error) {
	return s.mediaManager.GetFile(ctx, id)
}

// GetFileByPath retrieves a media file by path
func (s *mediaServiceImpl) GetFileByPath(ctx context.Context, path string) (*database.MediaFile, error) {
	return s.mediaManager.GetFileByPath(ctx, path)
}

// ListFiles lists media files with optional filtering
func (s *mediaServiceImpl) ListFiles(ctx context.Context, filter types.MediaFilter) ([]*database.MediaFile, error) {
	return s.mediaManager.ListFiles(ctx, filter)
}

// UpdateFile updates a media file
func (s *mediaServiceImpl) UpdateFile(ctx context.Context, id string, updates map[string]interface{}) error {
	return s.mediaManager.UpdateFile(ctx, id, updates)
}

// DeleteFile deletes a media file
func (s *mediaServiceImpl) DeleteFile(ctx context.Context, id string) error {
	return s.mediaManager.DeleteFile(ctx, id)
}

// GetLibrary retrieves a media library by ID
func (s *mediaServiceImpl) GetLibrary(ctx context.Context, id uint32) (*database.MediaLibrary, error) {
	return s.mediaManager.GetLibrary(ctx, id)
}

// ScanLibrary triggers a scan of a media library
func (s *mediaServiceImpl) ScanLibrary(ctx context.Context, libraryID uint32) error {
	// Get scanner service from registry and delegate to it
	scannerService, err := services.GetScannerService()
	if err != nil {
		return fmt.Errorf("scanner service not available: %w", err)
	}
	
	_, err = scannerService.StartScan(ctx, libraryID)
	return err
}

// UpdateMetadata updates metadata for a media file
func (s *mediaServiceImpl) UpdateMetadata(ctx context.Context, fileID string, metadata map[string]string) error {
	return s.mediaManager.UpdateMetadata(ctx, fileID, metadata)
}

// GetMediaInfo analyzes a media file and returns its information
func (s *mediaServiceImpl) GetMediaInfo(ctx context.Context, filePath string) (*types.MediaInfo, error) {
	return s.mediaManager.GetMediaInfo(ctx, filePath)
}