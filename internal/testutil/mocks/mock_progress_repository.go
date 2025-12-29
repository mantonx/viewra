package mocks

import (
	"context"
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/domain/progress"
)

// ProgressRepository is a mock implementation of progress.Repository for testing.
type ProgressRepository struct {
	t         testing.TB
	mu        sync.RWMutex
	progress  map[int64]*progress.WatchProgress // key: progress ID
	byMediaID map[int64]*progress.WatchProgress // key: media ID
	nextID    int64

	// Error injection
	CreateErr          error
	UpdateErr          error
	GetErr             error
	GetBatchErr        error
	ListErr            error
	DeleteErr          error
	DeleteByMediaIDErr error
	UpsertErr          error
}

// NewProgressRepository creates a new mock progress repository.
func NewProgressRepository(t testing.TB) *ProgressRepository {
	return &ProgressRepository{
		t:         t,
		progress:  make(map[int64]*progress.WatchProgress),
		byMediaID: make(map[int64]*progress.WatchProgress),
		nextID:    1,
	}
}

// WithProgress pre-populates the mock with progress records.
func (r *ProgressRepository) WithProgress(items ...*progress.WatchProgress) *ProgressRepository {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, item := range items {
		r.progress[item.ID] = item
		r.byMediaID[item.MediaID] = item
		if item.ID >= r.nextID {
			r.nextID = item.ID + 1
		}
	}
	return r
}

// WithCreateError injects an error for Create operations.
func (r *ProgressRepository) WithCreateError(err error) *ProgressRepository {
	r.CreateErr = err
	return r
}

// WithUpsertError injects an error for Upsert operations.
func (r *ProgressRepository) WithUpsertError(err error) *ProgressRepository {
	r.UpsertErr = err
	return r
}

// Implementation of progress.Repository interface

func (r *ProgressRepository) Create(ctx context.Context, p *progress.WatchProgress) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already exists
	if _, exists := r.byMediaID[p.MediaID]; exists {
		return progress.ErrProgressAlreadyExists
	}

	if p.ID == 0 {
		p.ID = r.nextID
		r.nextID++
	}

	r.progress[p.ID] = p
	r.byMediaID[p.MediaID] = p
	return nil
}

func (r *ProgressRepository) Update(ctx context.Context, p *progress.WatchProgress) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.progress[p.ID]; !exists {
		return progress.ErrProgressNotFound
	}

	r.progress[p.ID] = p
	r.byMediaID[p.MediaID] = p
	return nil
}

func (r *ProgressRepository) GetByMediaID(ctx context.Context, mediaID int64) (*progress.WatchProgress, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.byMediaID[mediaID]
	if !exists {
		return nil, progress.ErrProgressNotFound
	}

	return p, nil
}

func (r *ProgressRepository) GetByMediaIDAndUserID(ctx context.Context, mediaID, userID int64) (*progress.WatchProgress, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.byMediaID[mediaID]
	if !exists || p.UserID != userID {
		return nil, progress.ErrProgressNotFound
	}

	return p, nil
}

func (r *ProgressRepository) GetBatchByMediaIDs(ctx context.Context, mediaIDs []int64, userID int64) (map[int64]*progress.WatchProgress, error) {
	if r.GetBatchErr != nil {
		return nil, r.GetBatchErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[int64]*progress.WatchProgress)
	for _, mediaID := range mediaIDs {
		if p, exists := r.byMediaID[mediaID]; exists && p.UserID == userID {
			result[mediaID] = p
		}
	}

	return result, nil
}

func (r *ProgressRepository) ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]*progress.WatchProgress, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []*progress.WatchProgress
	for _, p := range r.progress {
		if p.UserID == userID {
			all = append(all, p)
		}
	}

	// Apply pagination
	if offset >= len(all) {
		return []*progress.WatchProgress{}, nil
	}

	end := offset + limit
	if end > len(all) {
		end = len(all)
	}

	return all[offset:end], nil
}

func (r *ProgressRepository) ListWatchedByUserID(ctx context.Context, userID int64, limit, offset int) ([]*progress.WatchProgress, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []*progress.WatchProgress
	for _, p := range r.progress {
		if p.UserID == userID && p.IsWatched {
			all = append(all, p)
		}
	}

	// Apply pagination
	if offset >= len(all) {
		return []*progress.WatchProgress{}, nil
	}

	end := offset + limit
	if end > len(all) {
		end = len(all)
	}

	return all[offset:end], nil
}

func (r *ProgressRepository) ListInProgressByUserID(ctx context.Context, userID int64, limit, offset int) ([]*progress.WatchProgress, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []*progress.WatchProgress
	for _, p := range r.progress {
		if p.UserID == userID && !p.IsWatched && p.ProgressSeconds > 0 {
			all = append(all, p)
		}
	}

	// Apply pagination
	if offset >= len(all) {
		return []*progress.WatchProgress{}, nil
	}

	end := offset + limit
	if end > len(all) {
		end = len(all)
	}

	return all[offset:end], nil
}

func (r *ProgressRepository) Delete(ctx context.Context, id int64) error {
	if r.DeleteErr != nil {
		return r.DeleteErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	p, exists := r.progress[id]
	if !exists {
		return progress.ErrProgressNotFound
	}

	delete(r.progress, id)
	delete(r.byMediaID, p.MediaID)
	return nil
}

func (r *ProgressRepository) DeleteByMediaID(ctx context.Context, mediaID int64) error {
	if r.DeleteByMediaIDErr != nil {
		return r.DeleteByMediaIDErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	p, exists := r.byMediaID[mediaID]
	if !exists {
		return progress.ErrProgressNotFound
	}

	delete(r.progress, p.ID)
	delete(r.byMediaID, mediaID)
	return nil
}

func (r *ProgressRepository) Upsert(ctx context.Context, p *progress.WatchProgress) error {
	if r.UpsertErr != nil {
		return r.UpsertErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if exists
	if existing, exists := r.byMediaID[p.MediaID]; exists {
		// Update existing
		p.ID = existing.ID
		r.progress[p.ID] = p
		r.byMediaID[p.MediaID] = p
	} else {
		// Create new
		if p.ID == 0 {
			p.ID = r.nextID
			r.nextID++
		}
		r.progress[p.ID] = p
		r.byMediaID[p.MediaID] = p
	}

	return nil
}
