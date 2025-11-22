package mocks

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/domain/media"
)

// MediaRepository is a mock implementation of media.Repository for testing.
type MediaRepository struct {
	t      testing.TB
	mu     sync.RWMutex
	media  map[int64]*media.Media
	nextID int64

	// Error injection
	CreateErr           error
	GetErr              error
	GetByFilePathErr    error
	ListErr             error
	UpdateErr           error
	DeleteErr           error
	ExistsInLibraryErr  error
	CountErr            error
	DeleteWithTxErr     error
	ListByLibraryWithTxErr error
}

// NewMediaRepository creates a new mock media repository.
func NewMediaRepository(t testing.TB) *MediaRepository {
	return &MediaRepository{
		t:      t,
		media:  make(map[int64]*media.Media),
		nextID: 1,
	}
}

// WithMedia pre-populates the mock with media items.
func (m *MediaRepository) WithMedia(items ...*media.Media) *MediaRepository {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, item := range items {
		m.media[item.ID] = item
		if item.ID >= m.nextID {
			m.nextID = item.ID + 1
		}
	}
	return m
}

// WithCreateError injects an error for Create operations.
func (m *MediaRepository) WithCreateError(err error) *MediaRepository {
	m.CreateErr = err
	return m
}

// WithGetError injects an error for GetByID operations.
func (m *MediaRepository) WithGetError(err error) *MediaRepository {
	m.GetErr = err
	return m
}

// WithDeleteError injects an error for Delete operations.
func (m *MediaRepository) WithDeleteError(err error) *MediaRepository {
	m.DeleteErr = err
	return m
}

// AssertMediaCount verifies the number of media items in the repository.
func (m *MediaRepository) AssertMediaCount(expected int) {
	m.t.Helper()
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.media) != expected {
		m.t.Errorf("expected %d media items, got %d", expected, len(m.media))
	}
}

// MediaExists checks if a media item exists by ID.
func (m *MediaRepository) MediaExists(id int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.media[id]
	return exists
}

// Implementation of media.Repository interface

func (m *MediaRepository) Create(ctx context.Context, item *media.Media) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if item.ID == 0 {
		item.ID = m.nextID
		m.nextID++
	}

	m.media[item.ID] = item
	return nil
}

func (m *MediaRepository) GetByID(ctx context.Context, id int64) (*media.Media, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.media[id]
	if !exists {
		return nil, sql.ErrNoRows
	}

	return item, nil
}

func (m *MediaRepository) GetByFilePath(ctx context.Context, libraryID int64, filePath string) (*media.Media, error) {
	if m.GetByFilePathErr != nil {
		return nil, m.GetByFilePathErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, item := range m.media {
		if item.LibraryID == libraryID && item.FilePath == filePath {
			return item, nil
		}
	}

	return nil, sql.ErrNoRows
}

func (m *MediaRepository) ListAll(ctx context.Context) ([]*media.Media, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*media.Media, 0, len(m.media))
	for _, item := range m.media {
		result = append(result, item)
	}

	return result, nil
}

func (m *MediaRepository) ListByLibrary(ctx context.Context, libraryID int64) ([]*media.Media, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*media.Media
	for _, item := range m.media {
		if item.LibraryID == libraryID {
			result = append(result, item)
		}
	}

	return result, nil
}

func (m *MediaRepository) ListByType(ctx context.Context, libraryID int64, mediaType media.MediaType) ([]*media.Media, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*media.Media
	for _, item := range m.media {
		if item.LibraryID == libraryID && item.Type == string(mediaType) {
			result = append(result, item)
		}
	}

	return result, nil
}

func (m *MediaRepository) Update(ctx context.Context, item *media.Media) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.media[item.ID]; !exists {
		return sql.ErrNoRows
	}

	m.media[item.ID] = item
	return nil
}

func (m *MediaRepository) Delete(ctx context.Context, id int64) error {
	if m.DeleteErr != nil {
		return m.DeleteErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.media[id]; !exists {
		return sql.ErrNoRows
	}

	delete(m.media, id)
	return nil
}

func (m *MediaRepository) ExistsInLibrary(ctx context.Context, libraryID int64, filePath string) (bool, error) {
	if m.ExistsInLibraryErr != nil {
		return false, m.ExistsInLibraryErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, item := range m.media {
		if item.LibraryID == libraryID && item.FilePath == filePath {
			return true, nil
		}
	}

	return false, nil
}

func (m *MediaRepository) Count(ctx context.Context, libraryID int64) (int64, error) {
	if m.CountErr != nil {
		return 0, m.CountErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var count int64
	for _, item := range m.media {
		if item.LibraryID == libraryID {
			count++
		}
	}

	return count, nil
}

func (m *MediaRepository) CountByType(ctx context.Context, libraryID int64, mediaType media.MediaType) (int64, error) {
	if m.CountErr != nil {
		return 0, m.CountErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var count int64
	for _, item := range m.media {
		if item.LibraryID == libraryID && item.Type == string(mediaType) {
			count++
		}
	}

	return count, nil
}

func (m *MediaRepository) DeleteWithTx(ctx context.Context, tx *sql.Tx, id int64) error {
	if m.DeleteWithTxErr != nil {
		return m.DeleteWithTxErr
	}

	// In mock, just delegate to regular Delete
	return m.Delete(ctx, id)
}

func (m *MediaRepository) ListByLibraryWithTx(ctx context.Context, tx *sql.Tx, libraryID int64) ([]*media.Media, error) {
	if m.ListByLibraryWithTxErr != nil {
		return nil, m.ListByLibraryWithTxErr
	}

	// In mock, just delegate to regular ListByLibrary
	return m.ListByLibrary(ctx, libraryID)
}
