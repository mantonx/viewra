package mediamodule

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// UploadHandler manages file uploads and integrates with media libraries
type UploadHandler struct {
	db          *gorm.DB
	eventBus    events.EventBus
	initialized bool
	mutex       sync.RWMutex
	
	// Upload configuration
	maxFileSize      int64
	tempUploadDir    string
	allowedMimeTypes map[string]bool
	fileCounter      uint64
	fileCounterMutex sync.Mutex
}

// UploadRequest represents a file upload request
type UploadRequest struct {
	File      *multipart.FileHeader `form:"file" binding:"required"`
	LibraryID uint                  `form:"libraryId"`
	Title     string                `form:"title"`
	Album     string                `form:"album"`
	Artist    string                `form:"artist"`
}

// UploadResult represents the result of a file upload
type UploadResult struct {
	MediaFileID  string    `json:"media_file_id"`
	FileName     string    `json:"file_name"`
	OriginalName string    `json:"original_name"`
	Size         int64     `json:"size"`
	Path         string    `json:"path"`
	LibraryID    uint      `json:"library_id"`
	MimeType     string    `json:"mime_type"`
	UploadedAt   time.Time `json:"uploaded_at"`
	MetadataID   uint      `json:"metadata_id,omitempty"`
}

// UploadStats represents upload statistics
type UploadStats struct {
	TotalUploads   int       `json:"total_uploads"`
	TotalFileSize  int64     `json:"total_file_size"`
	LastUpload     time.Time `json:"last_upload"`
	UploadsByType  map[string]int `json:"uploads_by_type"`
}

// NewUploadHandler creates a new upload handler
func NewUploadHandler(db *gorm.DB, eventBus events.EventBus) *UploadHandler {
	return &UploadHandler{
		db:               db,
		eventBus:         eventBus,
		maxFileSize:      500 * 1024 * 1024, // Default 500 MB max file size
		tempUploadDir:    os.TempDir(),
		allowedMimeTypes: getDefaultAllowedMimeTypes(),
	}
}

// Initialize initializes the upload handler
func (uh *UploadHandler) Initialize() error {
	log.Println("INFO: Initializing upload handler")
	
	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(uh.tempUploadDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp upload directory: %w", err)
	}
	
	uh.initialized = true
	log.Println("INFO: Upload handler initialized successfully")
	return nil
}

// ProcessUpload handles a file upload
func (uh *UploadHandler) ProcessUpload(file multipart.File, header *multipart.FileHeader, libraryID uint) (*UploadResult, error) {
	if !uh.initialized {
		return nil, fmt.Errorf("upload handler not initialized")
	}
	
	// Check file size
	if header.Size > uh.maxFileSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size of %d bytes", uh.maxFileSize)
	}
	
	// Check if the library exists if provided
	var library database.MediaLibrary
	if libraryID > 0 {
		if err := uh.db.First(&library, libraryID).Error; err != nil {
			return nil, fmt.Errorf("library not found: %w", err)
		}
	}
	
	// Generate upload path
	var uploadPath string
	if libraryID > 0 {
		// Use library path for uploads
		uploadPath = filepath.Join(library.Path, "uploads")
	} else {
		// Use default temp path
		uploadPath = filepath.Join(uh.tempUploadDir, "uploads")
	}
	
	// Ensure upload directory exists
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}
	
	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	base := strings.TrimSuffix(header.Filename, ext)
	timestamp := time.Now().Format("20060102_150405")
	
	// Add a counter to ensure uniqueness
	uh.fileCounterMutex.Lock()
	uh.fileCounter++
	counter := uh.fileCounter
	uh.fileCounterMutex.Unlock()
	
	uniqueFilename := fmt.Sprintf("%s_%s_%d%s", base, timestamp, counter, ext)
	filePath := filepath.Join(uploadPath, uniqueFilename)
	
	// Create the output file
	output, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	defer output.Close()
	
	// Copy the file
	_, err = io.Copy(output, file)
	if err != nil {
		os.Remove(filePath) // Clean up on error
		return nil, fmt.Errorf("failed to save uploaded file: %w", err)
	}
	
	// Calculate file hash
	fileHash, err := utils.CalculateFileHash(filePath)
	if err != nil {
		os.Remove(filePath) // Clean up on error
		return nil, fmt.Errorf("failed to calculate file hash: %w", err)
	}
	
	// Create media file record
	mediaFile := database.MediaFile{
		Path:      filePath,
		SizeBytes: header.Size,
		Hash:      fileHash,
		LibraryID: uint32(libraryID),
		LastSeen:  time.Now(),
	}
	
	if err := uh.db.Create(&mediaFile).Error; err != nil {
		os.Remove(filePath) // Clean up on error
		return nil, fmt.Errorf("failed to create media file record: %w", err)
	}
	
	// Create upload result
	result := &UploadResult{
		MediaFileID:  mediaFile.ID,
		FileName:     uniqueFilename,
		OriginalName: header.Filename,
		Size:         header.Size,
		Path:         filePath,
		LibraryID:    libraryID,
		MimeType:     getMimeTypeForFile(header.Filename),
		UploadedAt:   time.Now(),
	}
	
	// Publish upload event
	if uh.eventBus != nil {
		event := events.NewSystemEvent(
			"media.file.uploaded",
			"Media File Uploaded",
			fmt.Sprintf("File uploaded: %s (%.2f MB)", uniqueFilename, float64(header.Size)/(1024*1024)),
		)
		event.Data = map[string]interface{}{
			"mediaFileID":  mediaFile.ID,
			"libraryID":    libraryID,
			"originalName": header.Filename,
			"size":         header.Size,
			"path":         filePath,
		}
		uh.eventBus.PublishAsync(event)
	}
	
	log.Printf("INFO: File uploaded successfully: %s (%.2f MB)", uniqueFilename, float64(header.Size)/(1024*1024))
	return result, nil
}

// GetStats returns statistics about uploads
func (uh *UploadHandler) GetStats() *UploadStats {
	stats := &UploadStats{
		UploadsByType: make(map[string]int),
	}
	
	var count int64
	uh.db.Model(&database.MediaFile{}).Count(&count)
	stats.TotalUploads = int(count)
	
	var totalSize sql.NullInt64
	uh.db.Model(&database.MediaFile{}).Select("SUM(size)").Scan(&totalSize)
	if totalSize.Valid {
		stats.TotalFileSize = totalSize.Int64
	}
	
	// Get last upload time
	var lastFile database.MediaFile
	if err := uh.db.Order("created_at DESC").First(&lastFile).Error; err == nil {
		stats.LastUpload = lastFile.CreatedAt
	}
	
	return stats
}

// Shutdown gracefully shuts down the upload handler
func (uh *UploadHandler) Shutdown(ctx context.Context) error {
	log.Println("INFO: Shutting down upload handler")
	
	// Nothing specific to do for shutdown yet
	
	uh.initialized = false
	log.Println("INFO: Upload handler shutdown complete")
	return nil
}

// Helper function to get default allowed MIME types
func getDefaultAllowedMimeTypes() map[string]bool {
	return map[string]bool{
		"audio/mpeg":        true, // MP3
		"audio/mp4":         true, // AAC, M4A
		"audio/flac":        true, // FLAC
		"audio/ogg":         true, // OGG
		"audio/wav":         true, // WAV
		"audio/x-wav":       true, // WAV
		"video/mp4":         true, // MP4
		"video/quicktime":   true, // MOV
		"video/x-matroska":  true, // MKV
		"image/jpeg":        true, // JPG, JPEG
		"image/png":         true, // PNG
		"image/gif":         true, // GIF
	}
}

// Helper function to determine MIME type for a file
func getMimeTypeForFile(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".m4a", ".aac":
		return "audio/mp4"
	case ".flac":
		return "audio/flac"
	case ".ogg":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".mkv":
		return "video/x-matroska"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}