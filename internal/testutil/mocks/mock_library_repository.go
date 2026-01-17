package mocks

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/library"
)

// LibraryRepository is a mock implementation of library.Repository for testing.
type LibraryRepository struct {
	t         testing.TB
	mu        sync.RWMutex
	libraries map[int64]*library.Library
	nextID    int64

	// Error injection
	CreateErr           error
	GetErr              error
	ListErr             error
	UpdateErr           error
	DeleteErr           error
	ExistsErr           error
	CreateWithTxErr     error
	GetByIDWithTxErr    error
	DeleteWithTxErr     error
	ExistsWithTxErr     error
	UpdateMonitoringErr    error
	ListMonitoredErr       error
	UpdateLastScannedAtErr error
}

// NewLibraryRepository creates a new mock library repository.
func NewLibraryRepository(t testing.TB) *LibraryRepository {
	return &LibraryRepository{
		t:         t,
		libraries: make(map[int64]*library.Library),
		nextID:    1,
	}
}

// WithLibraries pre-populates the mock with library items.
func (r *LibraryRepository) WithLibraries(items ...*library.Library) *LibraryRepository {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, item := range items {
		r.libraries[item.ID] = item
		if item.ID >= r.nextID {
			r.nextID = item.ID + 1
		}
	}
	return r
}

// WithCreateError injects an error for Create operations.
func (r *LibraryRepository) WithCreateError(err error) *LibraryRepository {
	r.CreateErr = err
	return r
}

// WithGetError injects an error for GetByID operations.
func (r *LibraryRepository) WithGetError(err error) *LibraryRepository {
	r.GetErr = err
	return r
}

// AssertLibraryCount verifies the number of libraries in the repository.
func (r *LibraryRepository) AssertLibraryCount(expected int) {
	r.t.Helper()
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.libraries) != expected {
		r.t.Errorf("expected %d libraries, got %d", expected, len(r.libraries))
	}
}

// LibraryExists checks if a library exists by ID.
func (r *LibraryRepository) LibraryExists(id int64) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.libraries[id]
	return exists
}

// Implementation of library.Repository interface

func (r *LibraryRepository) Create(ctx context.Context, lib *library.Library) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if lib.ID == 0 {
		lib.ID = r.nextID
		r.nextID++
	}

	r.libraries[lib.ID] = lib
	return nil
}

func (r *LibraryRepository) GetByID(ctx context.Context, id int64) (*library.Library, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	lib, exists := r.libraries[id]
	if !exists {
		return nil, library.ErrLibraryNotFound
	}

	return lib, nil
}

func (r *LibraryRepository) GetByPath(ctx context.Context, path string) (*library.Library, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, lib := range r.libraries {
		if lib.Path == path {
			return lib, nil
		}
	}

	return nil, sql.ErrNoRows
}

func (r *LibraryRepository) List(ctx context.Context) ([]*library.Library, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*library.Library, 0, len(r.libraries))
	for _, lib := range r.libraries {
		result = append(result, lib)
	}

	return result, nil
}

func (r *LibraryRepository) Update(ctx context.Context, lib *library.Library) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.libraries[lib.ID]; !exists {
		return sql.ErrNoRows
	}

	r.libraries[lib.ID] = lib
	return nil
}

func (r *LibraryRepository) Delete(ctx context.Context, id int64) error {
	if r.DeleteErr != nil {
		return r.DeleteErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.libraries[id]; !exists {
		return sql.ErrNoRows
	}

	delete(r.libraries, id)
	return nil
}

func (r *LibraryRepository) Exists(ctx context.Context, path string) (bool, error) {
	if r.ExistsErr != nil {
		return false, r.ExistsErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, lib := range r.libraries {
		if lib.Path == path {
			return true, nil
		}
	}

	return false, nil
}

// Transaction-aware methods

func (r *LibraryRepository) CreateWithTx(ctx context.Context, tx common.Transaction, lib *library.Library) error {
	if r.CreateWithTxErr != nil {
		return r.CreateWithTxErr
	}

	// In mock, delegate to regular Create
	return r.Create(ctx, lib)
}

func (r *LibraryRepository) GetByIDWithTx(ctx context.Context, tx common.Transaction, id int64) (*library.Library, error) {
	if r.GetByIDWithTxErr != nil {
		return nil, r.GetByIDWithTxErr
	}

	// In mock, delegate to regular GetByID
	return r.GetByID(ctx, id)
}

func (r *LibraryRepository) DeleteWithTx(ctx context.Context, tx common.Transaction, id int64) error {
	if r.DeleteWithTxErr != nil {
		return r.DeleteWithTxErr
	}

	// In mock, delegate to regular Delete
	return r.Delete(ctx, id)
}

func (r *LibraryRepository) ExistsWithTx(ctx context.Context, tx common.Transaction, path string) (bool, error) {
	if r.ExistsWithTxErr != nil {
		return false, r.ExistsWithTxErr
	}

	// In mock, delegate to regular Exists
	return r.Exists(ctx, path)
}

// Monitoring methods

func (r *LibraryRepository) UpdateMonitoring(ctx context.Context, id int64, enabled bool, config *library.MonitoringConfig) error {
	if r.UpdateMonitoringErr != nil {
		return r.UpdateMonitoringErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	lib, exists := r.libraries[id]
	if !exists {
		return library.ErrLibraryNotFound
	}

	lib.MonitoringEnabled = enabled
	lib.MonitoringConfig = config
	return nil
}

func (r *LibraryRepository) ListMonitored(ctx context.Context) ([]*library.Library, error) {
	if r.ListMonitoredErr != nil {
		return nil, r.ListMonitoredErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*library.Library, 0)
	for _, lib := range r.libraries {
		if lib.MonitoringEnabled {
			result = append(result, lib)
		}
	}

	return result, nil
}

// Scan tracking

func (r *LibraryRepository) UpdateLastScannedAt(ctx context.Context, id int64) error {
	if r.UpdateLastScannedAtErr != nil {
		return r.UpdateLastScannedAtErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.libraries[id]; !exists {
		return library.ErrLibraryNotFound
	}

	// In mock, we don't need to actually update the timestamp
	return nil
}
