package mocks

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// ScanJobRepository is a mock implementation of scanner.ScanJobRepository for testing.
type ScanJobRepository struct {
	t      testing.TB
	mu     sync.RWMutex
	jobs   map[int64]*scanner.ScanJob
	nextID int64

	// Error injection
	CreateErr         error
	GetErr            error
	ListErr           error
	UpdateProgressErr error
	UpdateStatusErr   error
	CompleteErr       error
	DeleteErr         error
}

// NewScanJobRepository creates a new mock scan job repository.
func NewScanJobRepository(t testing.TB) *ScanJobRepository {
	return &ScanJobRepository{
		t:      t,
		jobs:   make(map[int64]*scanner.ScanJob),
		nextID: 1,
	}
}

// WithJobs pre-populates the mock with scan jobs.
func (r *ScanJobRepository) WithJobs(jobs ...*scanner.ScanJob) *ScanJobRepository {
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

// Implementation of scanner.ScanJobRepository interface

func (r *ScanJobRepository) Create(ctx context.Context, job *scanner.ScanJob) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if job.ID == 0 {
		job.ID = r.nextID
		r.nextID++
	}

	// Set timestamps if not already set
	now := time.Now()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}

	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = now
	}

	r.jobs[job.ID] = job
	return nil
}

func (r *ScanJobRepository) GetByID(ctx context.Context, id int64) (*scanner.ScanJob, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	job, exists := r.jobs[id]
	if !exists {
		return nil, scanner.ErrNotFound
	}

	return job, nil
}

func (r *ScanJobRepository) GetLatestByLibrary(ctx context.Context, libraryID int64) (*scanner.ScanJob, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var latest *scanner.ScanJob
	for _, job := range r.jobs {
		if job.LibraryID == libraryID {
			if latest == nil || job.CreatedAt.After(latest.CreatedAt) {
				latest = job
			}
		}
	}

	if latest == nil {
		return nil, scanner.ErrNotFound
	}

	return latest, nil
}

func (r *ScanJobRepository) ListByLibrary(ctx context.Context, libraryID int64, limit int32) ([]*scanner.ScanJob, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var jobs []*scanner.ScanJob
	for _, job := range r.jobs {
		if job.LibraryID == libraryID {
			jobs = append(jobs, job)
		}
	}

	// Apply limit
	if int(limit) < len(jobs) {
		jobs = jobs[:limit]
	}

	return jobs, nil
}

func (r *ScanJobRepository) ListRunning(ctx context.Context) ([]*scanner.ScanJob, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var running []*scanner.ScanJob
	for _, job := range r.jobs {
		if job.Status == scanner.ScanStatusRunning {
			running = append(running, job)
		}
	}

	return running, nil
}

func (r *ScanJobRepository) UpdateProgress(ctx context.Context, id int64, progress *scanner.Progress) error {
	if r.UpdateProgressErr != nil {
		return r.UpdateProgressErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	job, exists := r.jobs[id]
	if !exists {
		return scanner.ErrNotFound
	}

	job.FilesFound = progress.FilesFound
	job.FilesProcessed = progress.FilesProcessed
	job.BytesProcessed = progress.BytesProcessed
	job.ErrorCount = progress.ErrorCount
	job.WarningCount = progress.WarningCount
	job.Phase = progress.Phase
	job.EstimatedTotal = progress.EstimatedTotal
	job.DiscoveryDone = progress.DiscoveryDone

	now := time.Now()
	job.UpdatedAt = now

	return nil
}

func (r *ScanJobRepository) UpdateStatus(ctx context.Context, id int64, status scanner.ScanStatus) error {
	if r.UpdateStatusErr != nil {
		return r.UpdateStatusErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	job, exists := r.jobs[id]
	if !exists {
		return scanner.ErrNotFound
	}

	job.Status = status

	now := time.Now()
	job.UpdatedAt = now

	return nil
}

func (r *ScanJobRepository) Complete(ctx context.Context, job *scanner.ScanJob) error {
	if r.CompleteErr != nil {
		return r.CompleteErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.jobs[job.ID]
	if !exists {
		return scanner.ErrNotFound
	}

	// Update the job
	*existing = *job

	now := time.Now()
	existing.UpdatedAt = now
	existing.CompletedAt = &now

	return nil
}

func (r *ScanJobRepository) Delete(ctx context.Context, id int64) error {
	if r.DeleteErr != nil {
		return r.DeleteErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.jobs[id]; !exists {
		return scanner.ErrNotFound
	}

	delete(r.jobs, id)
	return nil
}

func (r *ScanJobRepository) DeleteOld(ctx context.Context, libraryID int64, retentionMinutes int) error {
	if r.DeleteErr != nil {
		return r.DeleteErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-time.Duration(retentionMinutes) * time.Minute)

	for id, job := range r.jobs {
		if job.LibraryID == libraryID {
			if job.CompletedAt != nil && job.CompletedAt.Before(cutoff) {
				delete(r.jobs, id)
			}
		}
	}

	return nil
}
