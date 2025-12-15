package mocks

import (
	"context"
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// CheckpointRepository is a mock implementation of scanner.CheckpointRepository for testing.
type CheckpointRepository struct {
	t            testing.TB
	mu           sync.RWMutex
	checkpoints  map[int64]*scanner.ScanCheckpoint // id -> checkpoint
	nextID       int64
	jobIDIndex   map[int64][]*scanner.ScanCheckpoint // jobID -> checkpoints
	overrideStats *scanner.CheckpointStats // If set, GetStats returns this instead of calculating

	// Error injection
	CreateBatchErr      error
	GetPendingBatchErr  error
	UpdateStatusErr     error
	UpdateRetryCountErr error
	GetStatsErr         error
	ListFailedErr       error
	GetByPathErr        error
	ResetFailedErr      error
	DeleteByJobIDErr    error
}

// NewCheckpointRepository creates a new mock checkpoint repository.
func NewCheckpointRepository(t testing.TB) *CheckpointRepository {
	return &CheckpointRepository{
		t:           t,
		checkpoints: make(map[int64]*scanner.ScanCheckpoint),
		jobIDIndex:  make(map[int64][]*scanner.ScanCheckpoint),
		nextID:      1,
	}
}

// WithStats sets override stats that will be returned by GetStats instead of calculating from checkpoints.
func (r *CheckpointRepository) WithStats(stats *scanner.CheckpointStats) *CheckpointRepository {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.overrideStats = stats
	return r
}

// WithCheckpoints pre-populates the mock with checkpoints.
func (r *CheckpointRepository) WithCheckpoints(checkpoints ...*scanner.ScanCheckpoint) *CheckpointRepository {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, cp := range checkpoints {
		if cp.ID == 0 {
			cp.ID = r.nextID
			r.nextID++
		}
		r.checkpoints[cp.ID] = cp
		r.jobIDIndex[cp.ScanJobID] = append(r.jobIDIndex[cp.ScanJobID], cp)
		if cp.ID >= r.nextID {
			r.nextID = cp.ID + 1
		}
	}
	return r
}

// Implementation of scanner.CheckpointRepository interface

func (r *CheckpointRepository) CreateBatch(ctx context.Context, checkpoints []*scanner.ScanCheckpoint) error {
	if r.CreateBatchErr != nil {
		return r.CreateBatchErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, cp := range checkpoints {
		if cp.ID == 0 {
			cp.ID = r.nextID
			r.nextID++
		}
		r.checkpoints[cp.ID] = cp
		r.jobIDIndex[cp.ScanJobID] = append(r.jobIDIndex[cp.ScanJobID], cp)
	}

	return nil
}

func (r *CheckpointRepository) GetPendingBatch(ctx context.Context, jobID int64, limit int) ([]*scanner.ScanCheckpoint, error) {
	if r.GetPendingBatchErr != nil {
		return nil, r.GetPendingBatchErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var pending []*scanner.ScanCheckpoint
	for _, cp := range r.jobIDIndex[jobID] {
		if cp.Status == scanner.CheckpointPending {
			pending = append(pending, cp)
			if len(pending) >= limit {
				break
			}
		}
	}

	return pending, nil
}

func (r *CheckpointRepository) UpdateStatus(ctx context.Context, id int64, status scanner.CheckpointStatus, errorMsg string, errorCategory scanner.ErrorCategory) error {
	if r.UpdateStatusErr != nil {
		return r.UpdateStatusErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	cp, exists := r.checkpoints[id]
	if !exists {
		return scanner.ErrNotFound
	}

	cp.Status = status
	cp.ErrorMessage = errorMsg
	cp.ErrorCategory = errorCategory

	return nil
}

func (r *CheckpointRepository) UpdateRetryCount(ctx context.Context, id int64, retryCount int) error {
	if r.UpdateRetryCountErr != nil {
		return r.UpdateRetryCountErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	cp, exists := r.checkpoints[id]
	if !exists {
		return scanner.ErrNotFound
	}

	cp.RetryCount = retryCount

	return nil
}

func (r *CheckpointRepository) GetStats(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error) {
	if r.GetStatsErr != nil {
		return nil, r.GetStatsErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return override stats if set
	if r.overrideStats != nil {
		return r.overrideStats, nil
	}

	stats := &scanner.CheckpointStats{
		ErrorsByCategory: make(map[scanner.ErrorCategory]int64),
	}

	for _, cp := range r.jobIDIndex[jobID] {
		stats.TotalFiles++
		switch cp.Status {
		case scanner.CheckpointPending:
			stats.PendingFiles++
		case scanner.CheckpointCompleted:
			stats.CompletedFiles++
			stats.ProcessedFiles++
		case scanner.CheckpointFailed:
			stats.FailedFiles++
			stats.ProcessedFiles++
			if cp.ErrorCategory != "" {
				stats.ErrorsByCategory[cp.ErrorCategory]++
			}
		case scanner.CheckpointWarning:
			stats.WarningFiles++
			stats.ProcessedFiles++
		}
	}

	return stats, nil
}

func (r *CheckpointRepository) ListFailed(ctx context.Context, jobID int64, limit int) ([]*scanner.ScanCheckpoint, error) {
	if r.ListFailedErr != nil {
		return nil, r.ListFailedErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var failed []*scanner.ScanCheckpoint
	for _, cp := range r.jobIDIndex[jobID] {
		if cp.Status == scanner.CheckpointFailed {
			failed = append(failed, cp)
			if len(failed) >= limit {
				break
			}
		}
	}

	return failed, nil
}

func (r *CheckpointRepository) GetByPath(ctx context.Context, jobID int64, filePath string) (*scanner.ScanCheckpoint, error) {
	if r.GetByPathErr != nil {
		return nil, r.GetByPathErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, cp := range r.jobIDIndex[jobID] {
		if cp.FilePath == filePath {
			return cp, nil
		}
	}

	return nil, scanner.ErrNotFound
}

func (r *CheckpointRepository) ResetFailed(ctx context.Context, jobID int64) (int64, error) {
	if r.ResetFailedErr != nil {
		return 0, r.ResetFailedErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	var count int64
	for _, cp := range r.jobIDIndex[jobID] {
		if cp.Status == scanner.CheckpointFailed {
			cp.Status = scanner.CheckpointPending
			count++
		}
	}

	return count, nil
}

func (r *CheckpointRepository) DeleteByJobID(ctx context.Context, jobID int64) error {
	if r.DeleteByJobIDErr != nil {
		return r.DeleteByJobIDErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, cp := range r.jobIDIndex[jobID] {
		delete(r.checkpoints, cp.ID)
	}
	delete(r.jobIDIndex, jobID)

	return nil
}

// Helper to get checkpoint count for a job (for testing)
func (r *CheckpointRepository) GetCheckpointCount(jobID int64) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.jobIDIndex[jobID])
}
