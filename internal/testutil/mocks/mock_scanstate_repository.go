package mocks

import (
	"context"
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// ScanStateRepository is a mock implementation of scanner.ScanStateRepository for testing.
type ScanStateRepository struct {
	t      testing.TB
	mu     sync.RWMutex
	states map[string]*scanner.ScanState // key: libraryID:filePath

	// Error injection
	GetLibraryStateErr error
	UpsertErr          error
	UpsertBatchErr     error
	DeleteByPathErr    error
	DeleteByPathsErr   error
	DeleteByLibraryErr error
	GetByPathErr       error
	CountByLibraryErr  error
	SetErrorErr        error
	SetWarningErr      error
}

// NewScanStateRepository creates a new mock scan state repository.
func NewScanStateRepository(t testing.TB) *ScanStateRepository {
	return &ScanStateRepository{
		t:      t,
		states: make(map[string]*scanner.ScanState),
	}
}

// WithStates pre-populates the mock with scan states.
func (r *ScanStateRepository) WithStates(states ...*scanner.ScanState) *ScanStateRepository {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, state := range states {
		key := r.makeKey(state.LibraryID, state.FilePath)
		r.states[key] = state
	}
	return r
}

func (r *ScanStateRepository) makeKey(libraryID int64, filePath string) string {
	return string(rune(libraryID)) + ":" + filePath
}

// Implementation of scanner.ScanStateRepository interface

func (r *ScanStateRepository) GetLibraryState(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	if r.GetLibraryStateErr != nil {
		return nil, r.GetLibraryStateErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var states []*scanner.ScanState
	for _, state := range r.states {
		if state.LibraryID == libraryID {
			states = append(states, state)
		}
	}

	return states, nil
}

func (r *ScanStateRepository) Upsert(ctx context.Context, state *scanner.ScanState) error {
	if r.UpsertErr != nil {
		return r.UpsertErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.makeKey(state.LibraryID, state.FilePath)
	r.states[key] = state
	return nil
}

func (r *ScanStateRepository) UpsertBatch(ctx context.Context, states []*scanner.ScanState) error {
	if r.UpsertBatchErr != nil {
		return r.UpsertBatchErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, state := range states {
		key := r.makeKey(state.LibraryID, state.FilePath)
		r.states[key] = state
	}
	return nil
}

func (r *ScanStateRepository) DeleteByPath(ctx context.Context, libraryID int64, filePath string) error {
	if r.DeleteByPathErr != nil {
		return r.DeleteByPathErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.makeKey(libraryID, filePath)
	delete(r.states, key)
	return nil
}

func (r *ScanStateRepository) DeleteByPaths(ctx context.Context, libraryID int64, filePaths []string) error {
	if r.DeleteByPathsErr != nil {
		return r.DeleteByPathsErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, filePath := range filePaths {
		key := r.makeKey(libraryID, filePath)
		delete(r.states, key)
	}
	return nil
}

func (r *ScanStateRepository) DeleteByLibrary(ctx context.Context, libraryID int64) error {
	if r.DeleteByLibraryErr != nil {
		return r.DeleteByLibraryErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for key, state := range r.states {
		if state.LibraryID == libraryID {
			delete(r.states, key)
		}
	}
	return nil
}

func (r *ScanStateRepository) GetByPath(ctx context.Context, libraryID int64, filePath string) (*scanner.ScanState, error) {
	if r.GetByPathErr != nil {
		return nil, r.GetByPathErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	key := r.makeKey(libraryID, filePath)
	state, exists := r.states[key]
	if !exists {
		return nil, scanner.ErrNotFound
	}

	return state, nil
}

func (r *ScanStateRepository) CountByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	if r.CountByLibraryErr != nil {
		return 0, r.CountByLibraryErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	count := int64(0)
	for _, state := range r.states {
		if state.LibraryID == libraryID {
			count++
		}
	}

	return count, nil
}

func (r *ScanStateRepository) GetLibraryWarnings(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var states []*scanner.ScanState
	for _, state := range r.states {
		if state.LibraryID == libraryID && state.HasWarning {
			states = append(states, state)
		}
	}

	return states, nil
}

func (r *ScanStateRepository) GetLibraryErrors(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var states []*scanner.ScanState
	for _, state := range r.states {
		if state.LibraryID == libraryID && state.HasError {
			states = append(states, state)
		}
	}

	return states, nil
}

func (r *ScanStateRepository) GetLibraryIssues(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var states []*scanner.ScanState
	for _, state := range r.states {
		if state.LibraryID == libraryID && (state.HasWarning || state.HasError) {
			states = append(states, state)
		}
	}

	return states, nil
}

func (r *ScanStateRepository) CountLibraryIssues(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	counts := &scanner.LibraryIssueCounts{}
	for _, state := range r.states {
		if state.LibraryID == libraryID {
			if state.HasError {
				counts.ErrorCount++
			}
			if state.HasWarning {
				counts.WarningCount++
			}
		}
	}

	return counts, nil
}

func (r *ScanStateRepository) SetWarning(ctx context.Context, libraryID int64, filePath, message, category string) error {
	if r.SetWarningErr != nil {
		return r.SetWarningErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.makeKey(libraryID, filePath)
	state, exists := r.states[key]
	if !exists {
		return scanner.ErrNotFound
	}

	state.HasWarning = true
	state.WarningMessage = message
	state.WarningCategory = category
	return nil
}

func (r *ScanStateRepository) SetError(ctx context.Context, libraryID int64, filePath, message, category string) error {
	if r.SetErrorErr != nil {
		return r.SetErrorErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.makeKey(libraryID, filePath)
	state, exists := r.states[key]
	if !exists {
		return scanner.ErrNotFound
	}

	state.HasError = true
	state.ErrorMessage = message
	state.ErrorCategory = category
	return nil
}

func (r *ScanStateRepository) ClearWarning(ctx context.Context, libraryID int64, filePath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.makeKey(libraryID, filePath)
	state, exists := r.states[key]
	if !exists {
		return scanner.ErrNotFound
	}

	state.HasWarning = false
	state.WarningMessage = ""
	state.WarningCategory = ""
	return nil
}

func (r *ScanStateRepository) ClearError(ctx context.Context, libraryID int64, filePath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.makeKey(libraryID, filePath)
	state, exists := r.states[key]
	if !exists {
		return scanner.ErrNotFound
	}

	state.HasError = false
	state.ErrorMessage = ""
	state.ErrorCategory = ""
	return nil
}

// GetStates returns all scan states (for testing)
func (r *ScanStateRepository) GetStates() []*scanner.ScanState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var states []*scanner.ScanState
	for _, state := range r.states {
		states = append(states, state)
	}
	return states
}
