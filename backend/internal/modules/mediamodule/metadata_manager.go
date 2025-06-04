package mediamodule

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
	"gorm.io/gorm"
)

// MetadataManager handles metadata extraction, validation, and enrichment
type MetadataManager struct {
	db           *gorm.DB
	eventBus     events.EventBus
	pluginModule *pluginmodule.PluginModule // Updated to use new plugin module
	initialized  bool
	mutex        sync.RWMutex

	// Enrichment providers (external APIs, databases, etc.)
	providers map[string]MetadataProvider
}

// MetadataProvider defines the interface for metadata enrichment providers
type MetadataProvider interface {
	GetProviderName() string
	CanEnrich(mediaFile *database.MediaFile) bool
	EnrichMetadata(ctx context.Context, mediaFile *database.MediaFile) (*EnrichmentResult, error)
}

// EnrichmentResult represents the result of metadata enrichment
type EnrichmentResult struct {
	Source         string                 `json:"source"`
	Success        bool                   `json:"success"`
	Title          string                 `json:"title,omitempty"`
	Artist         string                 `json:"artist,omitempty"`
	Album          string                 `json:"album,omitempty"`
	ReleaseDate    string                 `json:"release_date,omitempty"`
	Genre          string                 `json:"genre,omitempty"`
	Duration       int                    `json:"duration,omitempty"`
	TrackNumber    int                    `json:"track_number,omitempty"`
	AdditionalData map[string]interface{} `json:"additional_data,omitempty"`
	Error          string                 `json:"error,omitempty"`
}

// MetadataStats represents metadata manager statistics
type MetadataStats struct {
	TotalFiles     int64     `json:"total_files"`
	ProcessedFiles int64     `json:"processed_files"`
	EnrichedFiles  int64     `json:"enriched_files"`
	ErrorCount     int64     `json:"error_count"`
	LastProcessed  time.Time `json:"last_processed"`
}

// NewMetadataManager creates a new metadata manager
func NewMetadataManager(db *gorm.DB, eventBus events.EventBus, pluginModule *pluginmodule.PluginModule) *MetadataManager {
	return &MetadataManager{
		db:           db,
		eventBus:     eventBus,
		pluginModule: pluginModule, // Updated to use plugin module
		providers:    make(map[string]MetadataProvider),
	}
}

// Initialize initializes the metadata manager
func (mm *MetadataManager) Initialize() error {
	log.Println("INFO: Initializing metadata manager")

	mm.initialized = true
	log.Println("INFO: Metadata manager initialized successfully")
	return nil
}

// ExtractMetadata extracts metadata from a media file using available plugins
func (mm *MetadataManager) ExtractMetadata(mediaFile *database.MediaFile) error {
	log.Printf("INFO: Extracting metadata for file: %s", mediaFile.Path)

	// Check if file exists
	if _, err := os.Stat(mediaFile.Path); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", mediaFile.Path)
	}

	// Check if plugin module is available
	if mm.pluginModule == nil {
		log.Printf("WARNING: No plugin module available for file: %s - skipping metadata extraction", mediaFile.Path)
		return nil // Not an error, just no plugins available
	}

	// Get file info for plugin matching
	fileInfo, err := os.Stat(mediaFile.Path)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Get all available file handlers from plugin module
	handlers := mm.pluginModule.GetEnabledFileHandlers()

	var processedBy []string
	var lastError error

	// Run ALL matching handlers, not just the first one
	for _, handler := range handlers {
		if handler.Match(mediaFile.Path, fileInfo) {
			log.Printf("INFO: Processing file %s with handler: %s", mediaFile.Path, handler.GetName())

			// Create metadata context for plugin
			ctx := &pluginmodule.MetadataContext{
				DB:        mm.db,
				MediaFile: mediaFile,
				LibraryID: uint(mediaFile.LibraryID),
				EventBus:  mm.eventBus,
				PluginID:  handler.GetName(),
			}

			// Process file with the matching handler
			if err := handler.HandleFile(mediaFile.Path, ctx); err != nil {
				log.Printf("WARNING: Plugin handler %s failed for file %s: %v", handler.GetName(), mediaFile.Path, err)
				lastError = err
				continue // Try next handler
			}

			log.Printf("INFO: Successfully processed file %s with handler: %s", mediaFile.Path, handler.GetName())
			processedBy = append(processedBy, handler.GetName())
		}
	}

	if len(processedBy) > 0 {
		log.Printf("INFO: File processed by %d handlers: %s -> %v", len(processedBy), mediaFile.Path, processedBy)
		
		// Publish event
		if mm.eventBus != nil {
			event := events.NewSystemEvent(
				"media.metadata.extracted",
				"Metadata Extracted",
				fmt.Sprintf("Metadata extracted for %s using %d handlers: %v", filepath.Base(mediaFile.Path), len(processedBy), processedBy),
			)
			mm.eventBus.PublishAsync(event)
		}
		
		return nil // Success if at least one handler succeeded
	}

	// If no handlers processed the file and we had errors, return the last error
	if lastError != nil {
		return fmt.Errorf("all plugin handlers failed: %w", lastError)
	}

	// No handlers matched this file
	log.Printf("WARNING: No plugin handler found for file: %s", mediaFile.Path)
	return nil // Not an error, just no handler available
}

// EnrichMetadata enriches media metadata using registered providers
func (mm *MetadataManager) EnrichMetadata(mediaFileID string) error {
	if !mm.initialized {
		return fmt.Errorf("metadata manager not initialized")
	}

	// Get file from database with its metadata
	var mediaFile database.MediaFile
	if err := mm.db.Where("id = ?", mediaFileID).First(&mediaFile).Error; err != nil {
		return fmt.Errorf("failed to find media file: %w", err)
	}

	// Create timeout context for enrichment operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try each provider and collect results
	var enrichmentError error
	for _, provider := range mm.providers {
		// Check if provider can enrich this file
		if !provider.CanEnrich(&mediaFile) {
			continue
		}

		// Attempt to enrich metadata from this provider
		enrichmentResult, err := provider.EnrichMetadata(ctx, &mediaFile)

		if err != nil {
			enrichmentError = fmt.Errorf("provider %s failed: %w", provider.GetProviderName(), err)
			log.Printf("WARNING: Failed to enrich metadata with provider %s: %v", provider.GetProviderName(), err)
			continue
		}

		// Process enriched metadata
		if err := mm.updateMediaWithEnrichedData(mediaFileID, provider.GetProviderName(), enrichmentResult); err != nil {
			log.Printf("WARNING: Failed to update media with enriched data: %v", err)
			continue
		}

		// Success - publish event
		if mm.eventBus != nil {
			event := events.NewSystemEvent(
				"media.metadata.enriched",
				"Media Metadata Enriched",
				fmt.Sprintf("Enriched metadata for media file %s using provider %s", mediaFileID, provider.GetProviderName()),
			)
			event.Data = map[string]interface{}{
				"mediaFileID":  mediaFileID,
				"providerName": provider.GetProviderName(),
			}
			mm.eventBus.PublishAsync(event)
		}

		log.Printf("INFO: Successfully enriched metadata for file %s using provider %s", mediaFileID, provider.GetProviderName())
		break // Successfully enriched, no need to try other providers
	}

	return enrichmentError
}

// updateMediaWithEnrichedData updates the media record with enriched metadata
func (mm *MetadataManager) updateMediaWithEnrichedData(mediaFileID string, providerName string, data *EnrichmentResult) error {
	// Implementation depends on what type of metadata we're enriching
	// For now, just log the enrichment
	log.Printf("INFO: Enriched media file %s with provider %s", mediaFileID, providerName)
	return nil
}

// GetStats returns metadata manager statistics
func (mm *MetadataManager) GetStats() *MetadataStats {
	stats := &MetadataStats{}

	// Count total files
	var totalFiles int64
	mm.db.Model(&database.MediaFile{}).Count(&totalFiles)
	stats.TotalFiles = totalFiles

	// For now, use simplified statistics
	stats.ProcessedFiles = totalFiles // Assume all files have been processed
	stats.EnrichedFiles = 0           // No enrichment providers yet
	stats.ErrorCount = 0              // No error tracking yet
	stats.LastProcessed = time.Now()

	return stats
}

// RegisterProvider registers a metadata enrichment provider
func (mm *MetadataManager) RegisterProvider(provider MetadataProvider) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	mm.providers[provider.GetProviderName()] = provider
	log.Printf("INFO: Registered metadata provider: %s", provider.GetProviderName())
}

// Shutdown gracefully shuts down the metadata manager
func (mm *MetadataManager) Shutdown(ctx context.Context) error {
	log.Println("INFO: Shutting down metadata manager")

	mm.initialized = false
	log.Println("INFO: Metadata manager shutdown complete")
	return nil
}
