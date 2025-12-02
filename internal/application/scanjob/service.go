package scanjob

import (
	"context"
	"log/slog"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// Repository defines the interface for scan job data access.
type Repository interface {
	GetLatestByLibrary(ctx context.Context, libraryID int64) (*scanner.ScanJob, error)
	ListByLibrary(ctx context.Context, libraryID int64, limit int32) ([]*scanner.ScanJob, error)
	GetByID(ctx context.Context, jobID int64) (*scanner.ScanJob, error)
	UpdateStatus(ctx context.Context, jobID int64, status scanner.ScanStatus) error
	DeleteOld(ctx context.Context, libraryID int64, retentionMinutes int) error
}

// CheckpointRepository defines the interface for checkpoint data access.
type CheckpointRepository interface {
	GetStats(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error)
	ListFailed(ctx context.Context, jobID int64, limit int) ([]*scanner.ScanCheckpoint, error)
	ResetFailed(ctx context.Context, jobID int64) (int64, error)
}

// ScanStateRepository defines the interface for scan state data access.
type ScanStateRepository interface {
	GetLibraryIssues(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error)
	CountLibraryIssues(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error)
	CountByLibrary(ctx context.Context, libraryID int64) (int64, error)
}

// Service provides all scan job-related operations.
// Consolidates scan job, checkpoint, and scan state queries.
type Service struct {
	jobRepo        Repository
	checkpointRepo CheckpointRepository
	scanStateRepo  ScanStateRepository
	logger         *slog.Logger
}

// NewService creates a new scan job service.
func NewService(
	jobRepo Repository,
	checkpointRepo CheckpointRepository,
	scanStateRepo ScanStateRepository,
	logger *slog.Logger,
) *Service {
	return &Service{
		jobRepo:        jobRepo,
		checkpointRepo: checkpointRepo,
		scanStateRepo:  scanStateRepo,
		logger:         logger,
	}
}

// GetLatestByLibrary returns the most recent scan job for a library.
func (s *Service) GetLatestByLibrary(ctx context.Context, libraryID int64) (*scanner.ScanJob, error) {
	return s.jobRepo.GetLatestByLibrary(ctx, libraryID)
}

// ListByLibrary returns scan job history for a library.
func (s *Service) ListByLibrary(ctx context.Context, libraryID int64, limit int32) ([]*scanner.ScanJob, error) {
	return s.jobRepo.ListByLibrary(ctx, libraryID, limit)
}

// GetByID returns a scan job by ID.
func (s *Service) GetByID(ctx context.Context, jobID int64) (*scanner.ScanJob, error) {
	return s.jobRepo.GetByID(ctx, jobID)
}

// UpdateStatus updates the status of a scan job.
func (s *Service) UpdateStatus(ctx context.Context, jobID int64, status scanner.ScanStatus) error {
	return s.jobRepo.UpdateStatus(ctx, jobID, status)
}

// DeleteOld deletes old scan jobs for a library based on retention policy.
func (s *Service) DeleteOld(ctx context.Context, libraryID int64, retentionMinutes int) error {
	return s.jobRepo.DeleteOld(ctx, libraryID, retentionMinutes)
}

// GetCheckpointStats returns checkpoint statistics for a scan job.
func (s *Service) GetStats(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error) {
	return s.checkpointRepo.GetStats(ctx, jobID)
}

// ListFailed returns failed checkpoints for a scan job.
func (s *Service) ListFailed(ctx context.Context, jobID int64, limit int) ([]*scanner.ScanCheckpoint, error) {
	return s.checkpointRepo.ListFailed(ctx, jobID, limit)
}

// ResetFailed resets failed checkpoints to pending status.
func (s *Service) ResetFailed(ctx context.Context, jobID int64) (int64, error) {
	return s.checkpointRepo.ResetFailed(ctx, jobID)
}

// GetLibraryIssues returns all files with warnings or errors for a library.
func (s *Service) GetLibraryIssues(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	return s.scanStateRepo.GetLibraryIssues(ctx, libraryID)
}

// CountLibraryIssues returns issue counts for a library.
func (s *Service) CountLibraryIssues(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error) {
	return s.scanStateRepo.CountLibraryIssues(ctx, libraryID)
}

// CountByLibrary returns the total scan state count for a library.
func (s *Service) CountByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	return s.scanStateRepo.CountByLibrary(ctx, libraryID)
}
