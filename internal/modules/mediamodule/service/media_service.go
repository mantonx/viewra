package service

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/mediamodule/core/library"
	"github.com/mantonx/viewra/internal/modules/mediamodule/core/metadata"

	utils "github.com/mantonx/viewra/internal/modules/mediamodule/utils"
	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
	"gorm.io/gorm"
)

// mediaServiceImpl implements the MediaService interface
type mediaServiceImpl struct {
	db              *gorm.DB
	libraryManager  *library.Manager
	metadataManager *metadata.Manager
}

// NewMediaService creates a new media service implementation
func NewMediaService(
	db *gorm.DB,
	libraryManager *library.Manager,
	metadataManager *metadata.Manager,
) services.MediaService {
	return &mediaServiceImpl{
		db:              db,
		libraryManager:  libraryManager,
		metadataManager: metadataManager,
	}
}

// GetFile retrieves a media file by ID
func (s *mediaServiceImpl) GetFile(ctx context.Context, id string) (*database.MediaFile, error) {
	var file database.MediaFile
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&file).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("media file not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get media file: %w", err)
	}
	return &file, nil
}

// GetFileByPath retrieves a media file by path
func (s *mediaServiceImpl) GetFileByPath(ctx context.Context, path string) (*database.MediaFile, error) {
	var file database.MediaFile
	if err := s.db.WithContext(ctx).Where("path = ?", path).First(&file).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("media file not found at path: %s", path)
		}
		return nil, fmt.Errorf("failed to get media file by path: %w", err)
	}
	return &file, nil
}

// ListFiles lists media files with optional filtering
func (s *mediaServiceImpl) ListFiles(ctx context.Context, filter types.MediaFilter) ([]*database.MediaFile, error) {
	query := s.db.WithContext(ctx).Model(&database.MediaFile{})
	
	// Apply filters
	if filter.LibraryID != nil {
		query = query.Where("library_id = ?", *filter.LibraryID)
	}
	
	if filter.MediaType != "" {
		query = query.Where("media_type = ?", filter.MediaType)
	}
	
	if filter.Search != "" {
		query = query.Where("path LIKE ?", "%"+filter.Search+"%")
	}
	
	// Media format filters
	if filter.VideoCodec != "" {
		query = query.Where("LOWER(video_codec) LIKE ?", "%"+filter.VideoCodec+"%")
	}
	
	if filter.AudioCodec != "" {
		query = query.Where("LOWER(audio_codec) LIKE ?", "%"+filter.AudioCodec+"%")
	}
	
	if filter.Container != "" {
		query = query.Where("LOWER(container) = ?", filter.Container)
	}
	
	if filter.Resolution != "" {
		// Handle resolution based on height dimension (e.g., "1920x1080" -> 1080p)
		switch filter.Resolution {
		case "4k":
			query = query.Where("(resolution LIKE ? OR resolution LIKE ? OR video_height >= ?)", "%2160%", "%4K%", 2160)
		case "1080p":
			query = query.Where("(resolution LIKE ? OR (video_height >= ? AND video_height < ?))", "%1080%", 1080, 1440)
		case "720p":
			query = query.Where("(resolution LIKE ? OR (video_height >= ? AND video_height < ?))", "%720%", 720, 1080)
		case "480p":
			query = query.Where("(resolution LIKE ? OR resolution LIKE ? OR (video_height >= ? AND video_height < ?))", "%480%", "%SDTV%", 480, 720)
		default:
			query = query.Where("resolution LIKE ?", "%"+filter.Resolution+"%")
		}
	}
	
	// Playback method filter - filter based on browser compatibility
	if filter.PlaybackMethod != "" {
		switch filter.PlaybackMethod {
		case "direct":
			// Direct play: H264 video, AAC/MP3 audio, MP4/WebM/MOV containers
			query = query.Where(
				"(LOWER(container) IN (?, ?, ?) AND LOWER(video_codec) = ? AND LOWER(audio_codec) IN (?, ?))",
				"mp4", "webm", "mov", "h264", "aac", "mp3",
			)
		case "remux":
			// Remux: Good codecs but wrong container (e.g., H264 in MKV)
			query = query.Where(
				"(LOWER(container) NOT IN (?, ?, ?) AND LOWER(video_codec) = ? AND LOWER(audio_codec) IN (?, ?, ?, ?))",
				"mp4", "webm", "mov", "h264", "aac", "mp3", "ac3", "eac3",
			)
		case "transcode":
			// Transcode: Incompatible codecs (HEVC, VP9, etc.)
			query = query.Where(
				"(LOWER(video_codec) NOT IN (?) OR LOWER(audio_codec) NOT IN (?, ?, ?, ?))",
				"h264", "aac", "mp3", "ac3", "eac3",
			)
		}
	}
	
	// Apply sorting
	if filter.SortBy != "" {
		order := "ASC"
		if filter.SortDesc {
			order = "DESC"
		}
		query = query.Order(fmt.Sprintf("%s %s", filter.SortBy, order))
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
		return nil, fmt.Errorf("failed to list media files: %w", err)
	}
	
	return files, nil
}

// UpdateFile updates a media file
func (s *mediaServiceImpl) UpdateFile(ctx context.Context, id string, updates map[string]interface{}) error {
	result := s.db.WithContext(ctx).Model(&database.MediaFile{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update media file: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("media file not found: %s", id)
	}
	return nil
}

// DeleteFile deletes a media file
func (s *mediaServiceImpl) DeleteFile(ctx context.Context, id string) error {
	result := s.db.WithContext(ctx).Where("id = ?", id).Delete(&database.MediaFile{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete media file: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("media file not found: %s", id)
	}
	return nil
}

// GetLibrary retrieves a media library by ID
func (s *mediaServiceImpl) GetLibrary(ctx context.Context, id uint32) (*database.MediaLibrary, error) {
	return s.libraryManager.GetLibrary(ctx, id)
}

// ScanLibrary triggers a scan of a media library
func (s *mediaServiceImpl) ScanLibrary(ctx context.Context, libraryID uint32) error {
	// Get scanner service and delegate to it
	scannerService, err := services.GetScannerService()
	if err != nil {
		return fmt.Errorf("scanner service not available: %w", err)
	}
	
	_, err = scannerService.StartScan(ctx, libraryID)
	return err
}

// UpdateMetadata updates metadata for a media file
func (s *mediaServiceImpl) UpdateMetadata(ctx context.Context, fileID string, metadata map[string]string) error {
	return s.metadataManager.UpdateMetadata(ctx, fileID, metadata)
}

// GetMediaInfo analyzes a media file and returns its information
func (s *mediaServiceImpl) GetMediaInfo(ctx context.Context, filePath string) (*types.MediaInfo, error) {
	// Use FFmpeg probe to get media info
	return utils.ProbeMediaFile(filePath)
}