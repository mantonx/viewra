// Package storage provides content-addressable storage for transcoded media files.
// This file provides a wrapper to adapt the concrete ContentStore to the services interface.
package storage

import (
	"fmt"

	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/utils"
)

// ContentStoreWrapper adapts the concrete ContentStore to the services.ContentStore interface
type ContentStoreWrapper struct {
	store *ContentStore
}

// NewContentStoreWrapper creates a new wrapper around the concrete store
func NewContentStoreWrapper(store *ContentStore) services.ContentStore {
	return &ContentStoreWrapper{store: store}
}

// Get retrieves content metadata and path by hash
func (w *ContentStoreWrapper) Get(contentHash string) (metadata interface{}, contentPath string, err error) {
	meta, path, err := w.store.Get(contentHash)
	if err != nil {
		return nil, "", err
	}
	return meta, path, nil
}

// ListByMediaID returns all content versions for a media ID
func (w *ContentStoreWrapper) ListByMediaID(mediaID string) ([]interface{}, error) {
	metadataList, err := w.store.ListByMediaID(mediaID)
	if err != nil {
		return nil, err
	}

	// Convert to interface slice
	result := make([]interface{}, len(metadataList))
	for i, meta := range metadataList {
		result[i] = meta
	}

	return result, nil
}

// Delete removes content by hash
func (w *ContentStoreWrapper) Delete(contentHash string) error {
	return w.store.Delete(contentHash)
}

// Store saves content
func (w *ContentStoreWrapper) Store(contentHash string, sourceDir string, metadata interface{}) error {
	meta, ok := metadata.(ContentMetadata)
	if !ok {
		return fmt.Errorf("invalid metadata type")
	}
	return w.store.Store(contentHash, sourceDir, meta)
}

// Exists checks if content exists
func (w *ContentStoreWrapper) Exists(contentHash string) bool {
	return w.store.Exists(contentHash)
}

// GenerateContentHash creates a hash for content
func (w *ContentStoreWrapper) GenerateContentHash(mediaID string, profiles interface{}, formats interface{}) string {
	// For now, use a simple hash based on media ID and first format
	// This can be extended later to include encoding profiles
	container := "mp4" // default
	if formatList, ok := formats.([]string); ok && len(formatList) > 0 {
		container = formatList[0]
	}
	
	// Use the centralized hash utility
	return utils.GenerateContentHash(mediaID, container, 0, nil)
}

// GetStats returns storage statistics
func (w *ContentStoreWrapper) GetStats() (interface{}, error) {
	return w.store.GetStats()
}

// ListExpired returns content that has expired
func (w *ContentStoreWrapper) ListExpired() ([]interface{}, error) {
	expiredList, err := w.store.ListExpired()
	if err != nil {
		return nil, err
	}

	// Convert to interface slice
	result := make([]interface{}, len(expiredList))
	for i, meta := range expiredList {
		result[i] = meta
	}

	return result, nil
}
