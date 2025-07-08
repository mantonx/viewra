// Package storage manages content storage for transcoded files
package storage

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
)

// Manager orchestrates all storage operations
type Manager struct {
	logger       hclog.Logger
	contentStore *ContentStore
}

// NewManager creates a new storage manager
func NewManager(logger hclog.Logger, baseDir string) (*Manager, error) {
	contentStore, err := NewContentStore(baseDir, logger.Named("content-store"))
	if err != nil {
		return nil, fmt.Errorf("failed to create content store: %w", err)
	}
	
	return &Manager{
		logger:       logger,
		contentStore: contentStore,
	}, nil
}

// GetContentStore returns the content store
func (m *Manager) GetContentStore() *ContentStore {
	return m.contentStore
}

// StoreContent stores content with the given hash
func (m *Manager) StoreContent(ctx context.Context, contentHash string, sourceDir string, metadata ContentMetadata) error {
	return m.contentStore.Store(contentHash, sourceDir, metadata)
}

// GetContent retrieves content metadata and path by hash
func (m *Manager) GetContent(ctx context.Context, contentHash string) (*ContentMetadata, string, error) {
	data, path, err := m.contentStore.Get(contentHash)
	if err != nil {
		return nil, "", err
	}
	metadata := data.(*ContentMetadata)
	return metadata, path, nil
}

// Cleanup performs storage cleanup operations
func (m *Manager) Cleanup(ctx context.Context) error {
	// TODO: Implement cleanup logic
	// - Remove orphaned files
	// - Clean up old transcodes
	// - Enforce storage quotas
	return nil
}