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
	"github.com/mantonx/viewra/internal/plugins"
	"gorm.io/gorm"
)

// MetadataManager handles extraction and enrichment of media metadata
type MetadataManager struct {
	db            *gorm.DB
	eventBus      events.EventBus
	pluginManager plugins.Manager
	initialized   bool
	mutex         sync.RWMutex
	
	// Metadata providers
	providers     []MetadataProvider
	providerStats map[string]*ProviderStats
}

// MetadataProvider defines the interface for metadata providers
type MetadataProvider interface {
	ID() string
	Name() string
	SupportedTypes() []string
	FetchMetadata(ctx context.Context, mediaFile *database.MediaFile) (map[string]interface{}, error)
}

// ProviderStats tracks metadata provider statistics
type ProviderStats struct {
	ProviderID    string    `json:"provider_id"`
	ProviderName  string    `json:"provider_name"`
	TotalRequests int       `json:"total_requests"`
	SuccessCount  int       `json:"success_count"`
	FailureCount  int       `json:"failure_count"`
	LastSuccess   time.Time `json:"last_success,omitempty"`
	LastFailure   time.Time `json:"last_failure,omitempty"`
	AvgLatency    int64     `json:"avg_latency_ms"`
}

// MetadataStats represents overall metadata statistics
type MetadataStats struct {
	TotalFiles         int                 `json:"total_files"`
	FilesWithMetadata  int                 `json:"files_with_metadata"`
	MusicFiles         int                 `json:"music_files"`
	VideoFiles         int                 `json:"video_files"`
	ImageFiles         int                 `json:"image_files"`
	ProviderStatistics []*ProviderStats    `json:"provider_statistics"`
}

// NewMetadataManager creates a new metadata manager
func NewMetadataManager(db *gorm.DB, eventBus events.EventBus, pluginManager plugins.Manager) *MetadataManager {
	return &MetadataManager{
		db:            db,
		eventBus:      eventBus,
		pluginManager: pluginManager,
		providerStats: make(map[string]*ProviderStats),
	}
}

// Initialize initializes the metadata manager
func (mm *MetadataManager) Initialize() error {
	log.Println("INFO: Initializing metadata manager")
	
	// Register metadata providers
	mm.registerProviders()
	
	mm.initialized = true
	log.Println("INFO: Metadata manager initialized successfully")
	return nil
}

// registerProviders registers all metadata providers
func (mm *MetadataManager) registerProviders() {
	// Initialize provider stats for each registered provider
	for _, provider := range mm.providers {
		providerID := provider.ID()
		mm.providerStats[providerID] = &ProviderStats{
			ProviderID:   providerID,
			ProviderName: provider.Name(),
		}
		
		log.Printf("INFO: Registered metadata provider: %s", provider.Name())
	}
}

// ExtractMetadata extracts metadata from a media file using appropriate plugins
func (mm *MetadataManager) ExtractMetadata(mediaFile *database.MediaFile) error {
	log.Printf("INFO: Extracting metadata for file: %s", mediaFile.Path)
	
	// Check if file exists
	if _, err := os.Stat(mediaFile.Path); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", mediaFile.Path)
	}
	
	// Check if plugin manager is available
	if mm.pluginManager == nil {
		log.Printf("WARNING: No plugin manager available for file: %s - skipping metadata extraction", mediaFile.Path)
		return nil // Not an error, just no plugins available
	}
	
	// Get file info for plugin matching
	fileInfo, err := os.Stat(mediaFile.Path)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	
	// Get all available file handlers from plugin manager
	handlers := mm.pluginManager.GetFileHandlers()
	
	// Find a matching handler
	var matchingHandler plugins.FileHandlerPlugin
	for _, handler := range handlers {
		if handler.Match(mediaFile.Path, fileInfo) {
			matchingHandler = handler
			break
		}
	}
	
	if matchingHandler == nil {
		log.Printf("WARNING: No plugin handler found for file: %s", mediaFile.Path)
		return nil // Not an error, just no handler available
	}
	
	log.Printf("INFO: Processing file %s with handler: %s", mediaFile.Path, matchingHandler.GetName())
	
	// Create metadata context for plugin
	ctx := plugins.MetadataContext{
		DB:        mm.db,
		MediaFile: mediaFile,
		LibraryID: mediaFile.LibraryID,
		EventBus:  mm.eventBus,
	}
	
	// Delete existing metadata if it exists (clean slate approach)
	if err := mm.db.Where("media_file_id = ?", mediaFile.ID).Delete(&database.MusicMetadata{}).Error; err != nil {
		log.Printf("WARNING: Failed to delete existing metadata: %v", err)
	}
	
	// Process file with the matching handler
	if err := matchingHandler.HandleFile(mediaFile.Path, ctx); err != nil {
		return fmt.Errorf("plugin handler failed: %w", err)
	}
	
	// Publish event
	if mm.eventBus != nil {
		event := events.NewSystemEvent(
			"media.metadata.extracted",
			"Metadata Extracted",
			fmt.Sprintf("Metadata extracted for %s using %s", filepath.Base(mediaFile.Path), matchingHandler.GetName()),
		)
		mm.eventBus.PublishAsync(event)
	}
	
	log.Printf("INFO: Successfully extracted metadata for file: %s", mediaFile.Path)
	return nil
}

// EnrichMetadata enriches media metadata using registered providers
func (mm *MetadataManager) EnrichMetadata(mediaFileID uint) error {
	if !mm.initialized {
		return fmt.Errorf("metadata manager not initialized")
	}
	
	// Get file from database with its metadata
	var mediaFile database.MediaFile
	if err := mm.db.First(&mediaFile, mediaFileID).Error; err != nil {
		return fmt.Errorf("failed to find media file: %w", err)
	}
	
	// Create timeout context for enrichment operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Try each provider and collect results
	var enrichmentError error
	for _, provider := range mm.providers {
		stats := mm.providerStats[provider.ID()]
		
		startTime := time.Now()
		stats.TotalRequests++
		
		// Attempt to fetch metadata from this provider
		metadataMap, err := provider.FetchMetadata(ctx, &mediaFile)
		
		if err != nil {
			stats.FailureCount++
			stats.LastFailure = time.Now()
			enrichmentError = fmt.Errorf("provider %s failed: %w", provider.Name(), err)
			log.Printf("WARNING: Failed to enrich metadata with provider %s: %v", provider.Name(), err)
			continue
		}
		
		// Update provider stats
		stats.SuccessCount++
		stats.LastSuccess = time.Now()
		latency := time.Since(startTime).Milliseconds()
		if stats.AvgLatency == 0 {
			stats.AvgLatency = latency
		} else {
			stats.AvgLatency = (stats.AvgLatency + latency) / 2
		}
		
		// Process enriched metadata
		if err := mm.updateMediaWithEnrichedData(mediaFileID, provider.ID(), metadataMap); err != nil {
			log.Printf("WARNING: Failed to update media with enriched data: %v", err)
			continue
		}
		
		// Publish metadata enriched event
		if mm.eventBus != nil {
			event := events.NewSystemEvent(
				"media.metadata.enriched",
				"Media Metadata Enriched",
				fmt.Sprintf("Enriched metadata for media file %d using provider %s", mediaFileID, provider.Name()),
			)
			event.Data = map[string]interface{}{
				"mediaFileID":  mediaFileID,
				"providerID":   provider.ID(),
				"providerName": provider.Name(),
				"enrichedKeys": getMapKeys(metadataMap),
			}
			mm.eventBus.PublishAsync(event)
		}
		
		// If we got here, we successfully enriched the metadata
		return nil
	}
	
	// If we got here, all providers failed
	return enrichmentError
}

// updateMediaWithEnrichedData updates the media record with enriched metadata
func (mm *MetadataManager) updateMediaWithEnrichedData(mediaFileID uint, providerID string, data map[string]interface{}) error {
	// Implementation depends on what type of metadata we're enriching
	// For now, just log the enrichment
	log.Printf("INFO: Enriched media file %d with provider %s: %v", mediaFileID, providerID, getMapKeys(data))
	return nil
}

// GetStats returns statistics about metadata in the system
func (mm *MetadataManager) GetStats() *MetadataStats {
	stats := &MetadataStats{}
	
	// Count total files
	var totalFiles int64
	mm.db.Model(&database.MediaFile{}).Count(&totalFiles)
	stats.TotalFiles = int(totalFiles)
	
	// Count music files with metadata
	var musicFiles int64
	mm.db.Model(&database.MusicMetadata{}).Count(&musicFiles)
	stats.MusicFiles = int(musicFiles)
	
	// Files with metadata is the sum of all specific types
	stats.FilesWithMetadata = stats.MusicFiles + stats.VideoFiles + stats.ImageFiles
	
	// Add provider statistics
	for _, providerStat := range mm.providerStats {
		stats.ProviderStatistics = append(stats.ProviderStatistics, providerStat)
	}
	
	return stats
}

// Shutdown gracefully shuts down the metadata manager
func (mm *MetadataManager) Shutdown(ctx context.Context) error {
	log.Println("INFO: Shutting down metadata manager")
	
	// Nothing specific to do for shutdown yet
	
	mm.initialized = false
	log.Println("INFO: Metadata manager shutdown complete")
	return nil
}

// Helper function to get keys from a map
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}