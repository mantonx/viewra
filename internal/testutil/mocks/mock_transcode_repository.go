package mocks

import (
	"context"
	"database/sql"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/transcode"
)

// TranscodeRepository is a mock implementation of transcode.Repository for testing.
type TranscodeRepository struct {
	t      testing.TB
	mu     sync.RWMutex
	jobs   map[int64]*transcode.TranscodeJob
	nextID int64

	// Error injection
	CreateErr            error
	UpdateErr            error
	GetErr               error
	ListErr              error
	CountErr             error
	DeleteErr            error
	DeleteByMediaIDErr   error
	UpdateAccessErr      error
	GetTotalSizeErr      error
}

// NewTranscodeRepository creates a new mock transcode repository.
func NewTranscodeRepository(t testing.TB) *TranscodeRepository {
	return &TranscodeRepository{
		t:      t,
		jobs:   make(map[int64]*transcode.TranscodeJob),
		nextID: 1,
	}
}

// WithJobs pre-populates the mock with transcode jobs.
func (r *TranscodeRepository) WithJobs(jobs ...*transcode.TranscodeJob) *TranscodeRepository {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, job := range jobs {
		r.jobs[job.ID] = job
		if job.ID >= r.nextID {
			r.nextID = job.ID + 1
		}
	}
	return r
}

// Implementation of transcode.Repository interface

func (r *TranscodeRepository) Create(ctx context.Context, job *transcode.TranscodeJob) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if job.ID == 0 {
		job.ID = r.nextID
		r.nextID++
	}

	r.jobs[job.ID] = job
	return nil
}

func (r *TranscodeRepository) Update(ctx context.Context, job *transcode.TranscodeJob) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.jobs[job.ID]; !exists {
		return sql.ErrNoRows
	}

	r.jobs[job.ID] = job
	return nil
}

func (r *TranscodeRepository) GetByID(ctx context.Context, id int64) (*transcode.TranscodeJob, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	job, exists := r.jobs[id]
	if !exists {
		return nil, sql.ErrNoRows
	}

	return job, nil
}

func (r *TranscodeRepository) GetByMediaIDAndQuality(ctx context.Context, mediaID int64, quality string) (*transcode.TranscodeJob, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, job := range r.jobs {
		if job.MediaID == mediaID && job.Quality == quality {
			return job, nil
		}
	}

	return nil, sql.ErrNoRows
}

func (r *TranscodeRepository) ListByStatus(ctx context.Context, status string) ([]*transcode.TranscodeJob, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*transcode.TranscodeJob
	for _, job := range r.jobs {
		if job.Status == status {
			result = append(result, job)
		}
	}

	return result, nil
}

func (r *TranscodeRepository) ListQueuedJobs(ctx context.Context, limit int) ([]*transcode.TranscodeJob, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var queued []*transcode.TranscodeJob
	for _, job := range r.jobs {
		if job.Status == "queued" {
			queued = append(queued, job)
		}
	}

	// Sort by creation time (oldest first)
	sort.Slice(queued, func(i, j int) bool {
		return queued[i].CreatedAt.Before(queued[j].CreatedAt)
	})

	// Apply limit
	if limit > 0 && len(queued) > limit {
		queued = queued[:limit]
	}

	return queued, nil
}

func (r *TranscodeRepository) ListProcessingJobs(ctx context.Context) ([]*transcode.TranscodeJob, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var processing []*transcode.TranscodeJob
	for _, job := range r.jobs {
		if job.Status == "processing" {
			processing = append(processing, job)
		}
	}

	return processing, nil
}

func (r *TranscodeRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	if r.CountErr != nil {
		return 0, r.CountErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int64
	for _, job := range r.jobs {
		if job.Status == status {
			count++
		}
	}

	return count, nil
}

func (r *TranscodeRepository) Delete(ctx context.Context, id int64) error {
	if r.DeleteErr != nil {
		return r.DeleteErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.jobs[id]; !exists {
		return sql.ErrNoRows
	}

	delete(r.jobs, id)
	return nil
}

func (r *TranscodeRepository) DeleteByMediaID(ctx context.Context, mediaID int64) error {
	if r.DeleteByMediaIDErr != nil {
		return r.DeleteByMediaIDErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for id, job := range r.jobs {
		if job.MediaID == mediaID {
			delete(r.jobs, id)
		}
	}

	return nil
}

func (r *TranscodeRepository) ListAll(ctx context.Context) ([]*transcode.TranscodeJob, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*transcode.TranscodeJob, 0, len(r.jobs))
	for _, job := range r.jobs {
		result = append(result, job)
	}

	return result, nil
}

func (r *TranscodeRepository) UpdateAccess(ctx context.Context, mediaID int64, quality string) error {
	if r.UpdateAccessErr != nil {
		return r.UpdateAccessErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, job := range r.jobs {
		if job.MediaID == mediaID && job.Quality == quality {
			now := time.Now()
			job.LastAccessedAt = now
			job.AccessCount++
			return nil
		}
	}

	return sql.ErrNoRows
}

func (r *TranscodeRepository) GetTotalSize(ctx context.Context) (int64, error) {
	if r.GetTotalSizeErr != nil {
		return 0, r.GetTotalSizeErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var total int64
	for _, job := range r.jobs {
		if job.Status == "completed" {
			total += job.FileSizeBytes
		}
	}

	return total, nil
}

func (r *TranscodeRepository) ListByLRU(ctx context.Context, limit int) ([]*transcode.TranscodeJob, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect all jobs
	allJobs := make([]*transcode.TranscodeJob, 0, len(r.jobs))
	for _, job := range r.jobs {
		allJobs = append(allJobs, job)
	}

	// Sort by last accessed time (least recently used first)
	sort.Slice(allJobs, func(i, j int) bool {
		if allJobs[i].LastAccessedAt.IsZero() {
			return true
		}
		if allJobs[j].LastAccessedAt.IsZero() {
			return false
		}
		return allJobs[i].LastAccessedAt.Before(allJobs[j].LastAccessedAt)
	})

	// Apply limit
	if limit > 0 && len(allJobs) > limit {
		allJobs = allJobs[:limit]
	}

	return allJobs, nil
}
