package mediamodule

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
	"gorm.io/gorm"
)

// Ensure Module implements MediaService through adapter
var _ services.MediaService = (*ServiceAdapter)(nil)

// ServiceAdapter adapts the MediaModule to implement the MediaService interface
type ServiceAdapter struct {
	module *Module
}

// NewServiceAdapter creates a new service adapter for the media module
func NewServiceAdapter(module *Module) services.MediaService {
	return &ServiceAdapter{module: module}
}

// GetFile retrieves a media file by ID
func (s *ServiceAdapter) GetFile(ctx context.Context, id string) (*database.MediaFile, error) {
	if s.module.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var file database.MediaFile
	if err := s.module.db.First(&file, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("media file not found")
		}
		return nil, err
	}

	return &file, nil
}

// GetFileByPath retrieves a media file by path
func (s *ServiceAdapter) GetFileByPath(ctx context.Context, path string) (*database.MediaFile, error) {
	if s.module.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var file database.MediaFile
	if err := s.module.db.First(&file, "path = ?", path).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("media file not found")
		}
		return nil, err
	}

	return &file, nil
}

// ListFiles lists media files based on filter criteria
func (s *ServiceAdapter) ListFiles(ctx context.Context, filter types.MediaFilter) ([]*database.MediaFile, error) {
	if s.module.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := s.module.db.Model(&database.MediaFile{})

	// Apply filters
	if filter.LibraryID != 0 {
		query = query.Where("library_id = ?", filter.LibraryID)
	}
	if filter.MediaType != "" {
		query = query.Where("media_type = ?", filter.MediaType)
	}
	if filter.MinSize > 0 {
		query = query.Where("size_bytes >= ?", filter.MinSize)
	}
	if filter.MaxSize > 0 {
		query = query.Where("size_bytes <= ?", filter.MaxSize)
	}
	if filter.AddedAfter != nil {
		query = query.Where("created_at >= ?", *filter.AddedAfter)
	}
	if filter.AddedBefore != nil {
		query = query.Where("created_at <= ?", *filter.AddedBefore)
	}

	// Apply sorting
	if filter.SortBy != "" {
		order := filter.SortBy
		if filter.SortOrder == "desc" {
			order += " DESC"
		}
		query = query.Order(order)
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var files []*database.MediaFile
	if err := query.Find(&files).Error; err != nil {
		return nil, err
	}

	return files, nil
}

// UpdateFile updates a media file record
func (s *ServiceAdapter) UpdateFile(ctx context.Context, id string, updates map[string]interface{}) error {
	if s.module.db == nil {
		return fmt.Errorf("database not initialized")
	}

	result := s.module.db.Model(&database.MediaFile{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("media file not found")
	}

	return nil
}

// DeleteFile deletes a media file record
func (s *ServiceAdapter) DeleteFile(ctx context.Context, id string) error {
	if s.module.db == nil {
		return fmt.Errorf("database not initialized")
	}

	result := s.module.db.Delete(&database.MediaFile{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("media file not found")
	}

	return nil
}

// GetLibrary retrieves a media library by ID
func (s *ServiceAdapter) GetLibrary(ctx context.Context, id uint32) (*database.MediaLibrary, error) {
	if s.module.libraryManager == nil {
		return nil, fmt.Errorf("library manager not initialized")
	}

	return s.module.libraryManager.GetLibrary(uint(id))
}

// ScanLibrary triggers a scan for a media library
func (s *ServiceAdapter) ScanLibrary(ctx context.Context, libraryID uint32) error {
	if s.module.libraryManager == nil {
		return fmt.Errorf("library manager not initialized")
	}

	// Get the scanner service from the registry
	scannerService, err := services.GetScannerService()
	if err != nil {
		return fmt.Errorf("scanner service not available: %w", err)
	}

	// Start a scan job
	_, err = scannerService.StartScan(ctx, libraryID)
	return err
}

// UpdateMetadata updates metadata for a media file
func (s *ServiceAdapter) UpdateMetadata(ctx context.Context, fileID string, metadata map[string]string) error {
	if s.module.metadataManager == nil {
		return fmt.Errorf("metadata manager not initialized")
	}

	// TODO: Implement UpdateMetadata in MetadataManager
	// For now, update the file metadata directly
	return s.UpdateFile(ctx, fileID, map[string]interface{}{"metadata": metadata})
}

// GetMediaInfo analyzes a media file and returns its characteristics
func (s *ServiceAdapter) GetMediaInfo(ctx context.Context, filePath string) (*types.MediaInfo, error) {
	if s.module.fileProcessor == nil {
		return nil, fmt.Errorf("file processor not initialized")
	}

	// Use the plugin service to get metadata scrapers
	pluginService, err := services.GetPluginService()
	if err == nil {
		scrapers := pluginService.GetMetadataScrapers()
		for _, scraper := range scrapers {
			if scraper.CanHandle(filePath, "") {
				metadata, err := scraper.ExtractMetadata(filePath)
				if err == nil {
					return s.convertMetadataToMediaInfo(metadata), nil
				}
			}
		}
	}

	// Fallback to basic file info
	return s.module.fileProcessor.GetBasicMediaInfo(filePath)
}

// convertMetadataToMediaInfo converts plugin metadata to MediaInfo
func (s *ServiceAdapter) convertMetadataToMediaInfo(metadata map[string]string) *types.MediaInfo {
	info := &types.MediaInfo{
		Container:  metadata["container"],
		Duration:   float64(s.parseDuration(metadata["duration"])),
		VideoCodec: metadata["video_codec"],
		AudioCodec: metadata["audio_codec"],
		Resolution: metadata["resolution"],
		Bitrate:    int64(s.parseBitrate(metadata["video_bitrate"])),
	}

	return info
}

// Helper methods
func (s *ServiceAdapter) parseDuration(duration string) int {
	// Parse duration string to seconds
	// Implementation would parse various formats
	return 0
}

func (s *ServiceAdapter) parseBitrate(bitrate string) int {
	// Parse bitrate string to kbps
	// Implementation would parse various formats
	return 0
}

func (s *ServiceAdapter) parseSampleRate(sampleRate string) int {
	// Parse sample rate string to Hz
	// Implementation would parse various formats
	return 0
}
