// Package types - Internal interfaces
package types

import (
	"context"

	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// Repository defines the interface for media data access
type Repository interface {
	// Basic CRUD operations
	GetByID(ctx context.Context, id string) (*database.MediaFile, error)
	GetByPath(ctx context.Context, path string) (*database.MediaFile, error)
	List(ctx context.Context, query *gorm.DB) ([]*database.MediaFile, error)
	Update(ctx context.Context, id string, updates map[string]interface{}) error
	Delete(ctx context.Context, id string) error
}

// Filter defines the interface for media filtering logic
type Filter interface {
	// ApplyFilter applies filter criteria to a database query
	ApplyFilter(query *gorm.DB, filter MediaQuery) *gorm.DB
	
	// GetPlaybackMethod determines the playback method for a media file
	GetPlaybackMethod(file *database.MediaFile) PlaybackMethod
}

// LibraryManager defines the interface for library management
type LibraryManager interface {
	// Library operations
	GetLibrary(ctx context.Context, id uint32) (*database.MediaLibrary, error)
	CreateLibrary(ctx context.Context, lib *database.MediaLibrary) error
	UpdateLibrary(ctx context.Context, id uint32, updates map[string]interface{}) error
	DeleteLibrary(ctx context.Context, id uint32) error
	
	// Library statistics
	GetLibraryStats(ctx context.Context, id uint32) (*LibraryStats, error)
}

// MetadataManager defines the interface for metadata operations
type MetadataManager interface {
	// Metadata operations
	UpdateMetadata(ctx context.Context, fileID string, metadata map[string]string) error
	GetMetadata(ctx context.Context, fileID string) (map[string]string, error)
	
	// Enrichment operations
	EnrichMedia(ctx context.Context, req EnrichmentRequest) ([]EnrichmentResult, error)
}

// MediaAnalyzer defines the interface for media file analysis
type MediaAnalyzer interface {
	// AnalyzeFile analyzes a media file and returns detailed information
	AnalyzeFile(ctx context.Context, filePath string) (*MediaAnalysis, error)
	
	// ProbeFile performs a quick probe of a media file
	ProbeFile(ctx context.Context, filePath string) (*MediaFileInfo, error)
}