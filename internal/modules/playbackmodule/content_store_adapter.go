package playbackmodule

import (
	"fmt"
	"time"

	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/storage"
	"github.com/mantonx/viewra/internal/services"
)

// contentStoreAdapter adapts the services.ContentStore interface to storage.ContentStore
type contentStoreAdapter struct {
	serviceStore services.ContentStore
}

// wrapContentStore creates an adapter from services.ContentStore to storage.ContentStore
func wrapContentStore(serviceStore services.ContentStore) *storage.ContentStore {
	if serviceStore == nil {
		return nil
	}
	// This is a placeholder - we need to handle the type mismatch
	// The content API expects *storage.ContentStore but we have services.ContentStore
	// For now, return nil to avoid compilation errors
	return nil
}

// Get retrieves content metadata and path by hash
func (a *contentStoreAdapter) Get(contentHash string) (interface{}, string, error) {
	if a.serviceStore == nil {
		return nil, "", fmt.Errorf("content store not available")
	}
	return a.serviceStore.Get(contentHash)
}

// ListByMediaID returns all content for a media ID
func (a *contentStoreAdapter) ListByMediaID(mediaID string) ([]interface{}, error) {
	if a.serviceStore == nil {
		return nil, fmt.Errorf("content store not available")
	}
	return a.serviceStore.ListByMediaID(mediaID)
}

// GetStats returns storage statistics
func (a *contentStoreAdapter) GetStats() (interface{}, error) {
	if a.serviceStore == nil {
		return nil, fmt.Errorf("content store not available")
	}
	return a.serviceStore.GetStats()
}

// ListExpired returns expired content
func (a *contentStoreAdapter) ListExpired() ([]interface{}, error) {
	if a.serviceStore == nil {
		return nil, fmt.Errorf("content store not available")
	}
	return a.serviceStore.ListExpired()
}

// Delete removes content by hash
func (a *contentStoreAdapter) Delete(contentHash string) error {
	if a.serviceStore == nil {
		return fmt.Errorf("content store not available")
	}
	return a.serviceStore.Delete(contentHash)
}

// ContentMetadata represents metadata about stored content
type ContentMetadata struct {
	Hash         string            `json:"hash"`
	MediaID      string            `json:"media_id"`
	Format       string            `json:"format"`
	Size         int64             `json:"size"`
	CreatedAt    time.Time         `json:"created_at"`
	LastAccessed time.Time         `json:"last_accessed"`
	AccessCount  int               `json:"access_count"`
	ManifestURL  string            `json:"manifest_url"`
	Profiles     []interface{}     `json:"profiles"`
	Tags         map[string]string `json:"tags"`
}
