package status

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// mockScanStateRepository implements scanner.ScanStateRepository for testing
type mockScanStateRepository struct {
	countLibraryIssuesFunc func(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error)
	countByLibraryFunc     func(ctx context.Context, libraryID int64) (int64, error)
}

func (m *mockScanStateRepository) GetLibraryState(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	return nil, nil
}
func (m *mockScanStateRepository) Upsert(ctx context.Context, state *scanner.ScanState) error {
	return nil
}
func (m *mockScanStateRepository) UpsertBatch(ctx context.Context, states []*scanner.ScanState) error {
	return nil
}
func (m *mockScanStateRepository) DeleteByPath(ctx context.Context, libraryID int64, filePath string) error {
	return nil
}
func (m *mockScanStateRepository) DeleteByPaths(ctx context.Context, libraryID int64, filePaths []string) error {
	return nil
}
func (m *mockScanStateRepository) DeleteByLibrary(ctx context.Context, libraryID int64) error {
	return nil
}
func (m *mockScanStateRepository) GetByPath(ctx context.Context, libraryID int64, filePath string) (*scanner.ScanState, error) {
	return nil, nil
}
func (m *mockScanStateRepository) CountByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	if m.countByLibraryFunc != nil {
		return m.countByLibraryFunc(ctx, libraryID)
	}
	return 0, nil
}
func (m *mockScanStateRepository) GetLibraryWarnings(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	return nil, nil
}
func (m *mockScanStateRepository) GetLibraryErrors(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	return nil, nil
}
func (m *mockScanStateRepository) GetLibraryIssues(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	return nil, nil
}
func (m *mockScanStateRepository) CountLibraryIssues(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error) {
	if m.countLibraryIssuesFunc != nil {
		return m.countLibraryIssuesFunc(ctx, libraryID)
	}
	return nil, nil
}
func (m *mockScanStateRepository) SetWarning(ctx context.Context, libraryID int64, filePath, message, category string) error {
	return nil
}
func (m *mockScanStateRepository) SetError(ctx context.Context, libraryID int64, filePath, message, category string) error {
	return nil
}
func (m *mockScanStateRepository) ClearWarning(ctx context.Context, libraryID int64, filePath string) error {
	return nil
}
func (m *mockScanStateRepository) ClearError(ctx context.Context, libraryID int64, filePath string) error {
	return nil
}

// checkpointRepoForStatus wraps the generated mock to add custom behavior
type checkpointRepoForStatus struct {
	*mocks.CheckpointRepository
	getStatsFunc func(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error)
}

func newCheckpointRepoForStatus(t *testing.T) *checkpointRepoForStatus {
	return &checkpointRepoForStatus{
		CheckpointRepository: mocks.NewCheckpointRepository(t),
	}
}

func (m *checkpointRepoForStatus) GetStats(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error) {
	if m.getStatsFunc != nil {
		return m.getStatsFunc(ctx, jobID)
	}
	return m.CheckpointRepository.GetStats(ctx, jobID)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestGetScanStatus(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(5 * time.Minute)

	tests := []struct {
		name        string
		libraryID   int64
		setupMocks  func(*mocks.ScanJobRepository, *mockScanStateRepository, *checkpointRepoForStatus)
		wantErr     bool
		checkResult func(*testing.T, *Result)
	}{
		{
			name:      "running scan with progress",
			libraryID: 1,
			setupMocks: func(jobRepo *mocks.ScanJobRepository, stateRepo *mockScanStateRepository, checkRepo *checkpointRepoForStatus) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:             1,
					LibraryID:      1,
					Status:         scanner.ScanStatusRunning,
					Phase:          scanner.ScanPhaseProcessing,
					StartedAt:      now,
					FilesFound:     100,
					FilesProcessed: 50,
					Progress:       50.0,
					CreatedAt:      now,
					UpdatedAt:      now,
				})
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				if result.Status != scanner.ScanStatusRunning {
					t.Errorf("Status = %v, want %v", result.Status, scanner.ScanStatusRunning)
				}
				if result.FilesFound != 100 {
					t.Errorf("FilesFound = %d, want 100", result.FilesFound)
				}
			},
		},
		{
			name:      "completed scan",
			libraryID: 1,
			setupMocks: func(jobRepo *mocks.ScanJobRepository, stateRepo *mockScanStateRepository, checkRepo *checkpointRepoForStatus) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:             1,
					LibraryID:      1,
					Status:         scanner.ScanStatusCompleted,
					Phase:          scanner.ScanPhaseCompleted,
					StartedAt:      now.Add(-1 * time.Hour),
					CompletedAt:    &completedAt,
					FilesFound:     100,
					FilesProcessed: 100,
					Progress:       100.0,
					CreatedAt:      now.Add(-2 * time.Hour),
					UpdatedAt:      completedAt,
				})

				stateRepo.countByLibraryFunc = func(ctx context.Context, libraryID int64) (int64, error) {
					return 100, nil
				}
				stateRepo.countLibraryIssuesFunc = func(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error) {
					return &scanner.LibraryIssueCounts{ErrorCount: 0, WarningCount: 0}, nil
				}
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				if result.Status != scanner.ScanStatusCompleted {
					t.Errorf("Status = %v, want %v", result.Status, scanner.ScanStatusCompleted)
				}
				if result.CompletedAt == nil {
					t.Error("CompletedAt should not be nil for completed scan")
				}
				if result.Progress != 100.0 {
					t.Errorf("Progress = %f, want 100.0", result.Progress)
				}
			},
		},
		{
			name:      "no scan job found",
			libraryID: 999,
			setupMocks: func(jobRepo *mocks.ScanJobRepository, stateRepo *mockScanStateRepository, checkRepo *checkpointRepoForStatus) {
				// No jobs in repository
			},
			wantErr: true,
		},
		{
			name:      "scan state enrichment with counts",
			libraryID: 1,
			setupMocks: func(jobRepo *mocks.ScanJobRepository, stateRepo *mockScanStateRepository, checkRepo *checkpointRepoForStatus) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:             1,
					LibraryID:      1,
					Status:         scanner.ScanStatusRunning,
					Phase:          scanner.ScanPhaseProcessing,
					StartedAt:      now,
					FilesFound:     100,
					FilesProcessed: 50,
					ErrorCount:     2,
					WarningCount:   5,
					CreatedAt:      now,
					UpdatedAt:      now,
				})

				stateRepo.countLibraryIssuesFunc = func(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error) {
					return &scanner.LibraryIssueCounts{
						ErrorCount:   10,
						WarningCount: 15,
					}, nil
				}
				stateRepo.countByLibraryFunc = func(ctx context.Context, libraryID int64) (int64, error) {
					return 75, nil
				}
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *Result) {
				// Should use scan_state counts, not job counts
				if result.ErrorCount != 10 {
					t.Errorf("ErrorCount = %d, want 10 (from scan_state)", result.ErrorCount)
				}
				if result.WarningCount != 15 {
					t.Errorf("WarningCount = %d, want 15 (from scan_state)", result.WarningCount)
				}
				if result.FilesProcessed != 75 {
					t.Errorf("FilesProcessed = %d, want 75 (from scan_state)", result.FilesProcessed)
				}
				// Progress should be recalculated: 75/100 * 100 = 75%
				if result.Progress != 75.0 {
					t.Errorf("Progress = %f, want 75.0", result.Progress)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobRepo := mocks.NewScanJobRepository(t)
			stateRepo := &mockScanStateRepository{}
			checkRepo := newCheckpointRepoForStatus(t)

			if tt.setupMocks != nil {
				tt.setupMocks(jobRepo, stateRepo, checkRepo)
			}

			deps := &Deps{
				ScanRepos: &scan.ScanRepositories{
					ScanJob:    jobRepo,
					ScanState:  stateRepo,
					Checkpoint: checkRepo,
				},
				Logger: discardLogger(),
			}

			result, err := GetScanStatus(context.Background(), deps, tt.libraryID)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

func TestEnrichWithScanState(t *testing.T) {
	tests := []struct {
		name        string
		libraryID   int64
		job         *scanner.ScanJob
		status      *Result
		setupMocks  func(*mockScanStateRepository)
		checkResult func(*testing.T, *Result)
	}{
		{
			name:      "successful enrichment with all data",
			libraryID: 1,
			job: &scanner.ScanJob{
				ID:             1,
				LibraryID:      1,
				FilesFound:     100,
				FilesProcessed: 40,
				ErrorCount:     1,
				WarningCount:   2,
			},
			status: &Result{
				FilesFound:     100,
				FilesProcessed: 40,
				ErrorCount:     1,
				WarningCount:   2,
				Progress:       40.0,
			},
			setupMocks: func(repo *mockScanStateRepository) {
				repo.countLibraryIssuesFunc = func(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error) {
					return &scanner.LibraryIssueCounts{
						ErrorCount:   5,
						WarningCount: 10,
					}, nil
				}
				repo.countByLibraryFunc = func(ctx context.Context, libraryID int64) (int64, error) {
					return 60, nil
				}
			},
			checkResult: func(t *testing.T, result *Result) {
				if result.ErrorCount != 5 {
					t.Errorf("ErrorCount = %d, want 5", result.ErrorCount)
				}
				if result.WarningCount != 10 {
					t.Errorf("WarningCount = %d, want 10", result.WarningCount)
				}
				if result.FilesProcessed != 60 {
					t.Errorf("FilesProcessed = %d, want 60", result.FilesProcessed)
				}
				// Progress should be recalculated: 60/100 * 100 = 60%
				if result.Progress != 60.0 {
					t.Errorf("Progress = %f, want 60.0", result.Progress)
				}
			},
		},
		{
			name:      "error getting issue counts - keeps original values",
			libraryID: 1,
			job: &scanner.ScanJob{
				ID:             1,
				LibraryID:      1,
				FilesFound:     100,
				FilesProcessed: 40,
				ErrorCount:     3,
				WarningCount:   7,
			},
			status: &Result{
				FilesFound:     100,
				FilesProcessed: 40,
				ErrorCount:     3,
				WarningCount:   7,
			},
			setupMocks: func(repo *mockScanStateRepository) {
				repo.countLibraryIssuesFunc = func(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error) {
					return nil, errors.New("database error")
				}
			},
			checkResult: func(t *testing.T, result *Result) {
				// Should keep original job counts on error
				if result.ErrorCount != 3 {
					t.Errorf("ErrorCount = %d, want 3 (job count)", result.ErrorCount)
				}
				if result.WarningCount != 7 {
					t.Errorf("WarningCount = %d, want 7 (job count)", result.WarningCount)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRepo := &mockScanStateRepository{}
			if tt.setupMocks != nil {
				tt.setupMocks(stateRepo)
			}

			deps := &Deps{
				ScanRepos: &scan.ScanRepositories{
					ScanState: stateRepo,
				},
				Logger: discardLogger(),
			}

			EnrichWithScanState(context.Background(), deps, tt.libraryID, tt.job, tt.status)

			if tt.checkResult != nil {
				tt.checkResult(t, tt.status)
			}
		})
	}
}

func TestEnrichWithETA(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		jobID       int64
		filesFound  int64
		status      *Result
		setupMocks  func(*checkpointRepoForStatus)
		checkResult func(*testing.T, *Result)
	}{
		{
			name:       "calculates ETA with valid stats",
			jobID:      1,
			filesFound: 100,
			status:     &Result{},
			setupMocks: func(repo *checkpointRepoForStatus) {
				firstProcessed := now.Add(-30 * time.Second)
				repo.getStatsFunc = func(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error) {
					return &scanner.CheckpointStats{
						TotalFiles:       100,
						ProcessedFiles:   50,
						CompletedFiles:   45,
						FailedFiles:      5,
						FirstProcessedAt: &firstProcessed,
					}, nil
				}
			},
			checkResult: func(t *testing.T, result *Result) {
				if result.ETASeconds == nil {
					t.Error("ETASeconds should not be nil")
				} else {
					// With 50 files processed in 30 seconds, rate ~= 1.67 files/sec
					// Remaining = 50 files, ETA ~= 30 seconds
					if *result.ETASeconds < 25 || *result.ETASeconds > 35 {
						t.Errorf("ETASeconds = %d, expected around 30", *result.ETASeconds)
					}
				}
			},
		},
		{
			name:       "no ETA when stats unavailable",
			jobID:      1,
			filesFound: 100,
			status:     &Result{},
			setupMocks: func(repo *checkpointRepoForStatus) {
				repo.getStatsFunc = func(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error) {
					return nil, errors.New("no stats")
				}
			},
			checkResult: func(t *testing.T, result *Result) {
				if result.ETASeconds != nil {
					t.Error("ETASeconds should be nil when stats unavailable")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkRepo := newCheckpointRepoForStatus(t)
			if tt.setupMocks != nil {
				tt.setupMocks(checkRepo)
			}

			deps := &Deps{
				ScanRepos: &scan.ScanRepositories{
					Checkpoint: checkRepo,
				},
				Logger: discardLogger(),
			}

			EnrichWithETA(context.Background(), deps, tt.jobID, tt.filesFound, tt.status)

			if tt.checkResult != nil {
				tt.checkResult(t, tt.status)
			}
		})
	}
}

// =============================================================================
// Tests for completion.go
// =============================================================================

func TestCompleteFromStats(t *testing.T) {
	now := time.Now()
	jobRepo := mocks.NewScanJobRepository(t)
	// Pre-populate with a running job that will be completed
	jobRepo.WithJobs(&scanner.ScanJob{
		ID:        123,
		LibraryID: 1,
		Status:    scanner.ScanStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	})

	deps := &Deps{
		ScanRepos: &scan.ScanRepositories{
			ScanJob: jobRepo,
		},
		Logger: discardLogger(),
	}

	stats := &scanner.CheckpointStats{
		TotalFiles:     100,
		ProcessedFiles: 95,
		CompletedFiles: 90,
		FailedFiles:    5,
		WarningFiles:   10,
	}

	CompleteFromStats(context.Background(), deps, 123, 100, stats)

	// Verify job was completed by fetching it
	job, err := jobRepo.GetByID(context.Background(), 123)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if job.Status != scanner.ScanStatusCompleted {
		t.Errorf("Status = %v, want %v", job.Status, scanner.ScanStatusCompleted)
	}
	if job.FilesProcessed != 95 {
		t.Errorf("FilesProcessed = %d, want 95", job.FilesProcessed)
	}
	if job.ErrorCount != 5 {
		t.Errorf("ErrorCount = %d, want 5", job.ErrorCount)
	}
	if job.WarningCount != 10 {
		t.Errorf("WarningCount = %d, want 10", job.WarningCount)
	}
	if job.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
	if job.Phase != scanner.ScanPhaseCompleted {
		t.Errorf("Phase = %v, want %v", job.Phase, scanner.ScanPhaseCompleted)
	}
}

func TestCompleteWithError(t *testing.T) {
	now := time.Now()
	jobRepo := mocks.NewScanJobRepository(t)
	jobRepo.WithJobs(&scanner.ScanJob{
		ID:        456,
		LibraryID: 1,
		Status:    scanner.ScanStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	})

	deps := &Deps{
		ScanRepos: &scan.ScanRepositories{
			ScanJob: jobRepo,
		},
		Logger: discardLogger(),
	}

	testErr := errors.New("scan failed: database error")
	CompleteWithError(context.Background(), deps, 456, testErr)

	job, err := jobRepo.GetByID(context.Background(), 456)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if job.Status != scanner.ScanStatusFailed {
		t.Errorf("Status = %v, want %v", job.Status, scanner.ScanStatusFailed)
	}
	if job.ErrorMessage != "scan failed: database error" {
		t.Errorf("ErrorMessage = %q, want %q", job.ErrorMessage, "scan failed: database error")
	}
	if job.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
}

func TestCompleteSafely_Success(t *testing.T) {
	now := time.Now()
	jobRepo := mocks.NewScanJobRepository(t)
	jobRepo.WithJobs(&scanner.ScanJob{
		ID:        789,
		LibraryID: 1,
		Status:    scanner.ScanStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	})

	deps := &Deps{
		ScanRepos: &scan.ScanRepositories{
			ScanJob: jobRepo,
		},
		Logger: discardLogger(),
	}

	job := &scanner.ScanJob{
		ID:     789,
		Status: scanner.ScanStatusCompleted,
	}

	CompleteSafely(context.Background(), deps, job)

	// Verify the job was updated
	updatedJob, err := jobRepo.GetByID(context.Background(), 789)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}
	if updatedJob.Status != scanner.ScanStatusCompleted {
		t.Errorf("Status = %v, want %v", updatedJob.Status, scanner.ScanStatusCompleted)
	}
}

func TestCompleteSafely_DeletedJob(t *testing.T) {
	jobRepo := mocks.NewScanJobRepository(t)
	// Set error that matches IsScanJobDeleted pattern
	jobRepo.CompleteErr = errors.New("scan job not found")

	deps := &Deps{
		ScanRepos: &scan.ScanRepositories{
			ScanJob: jobRepo,
		},
		Logger: discardLogger(),
	}

	job := &scanner.ScanJob{
		ID:     999,
		Status: scanner.ScanStatusCompleted,
	}

	// Should not panic when job was deleted
	CompleteSafely(context.Background(), deps, job)
}

func TestCompleteSafely_OtherError(t *testing.T) {
	jobRepo := mocks.NewScanJobRepository(t)
	jobRepo.CompleteErr = errors.New("database connection lost")

	deps := &Deps{
		ScanRepos: &scan.ScanRepositories{
			ScanJob: jobRepo,
		},
		Logger: discardLogger(),
	}

	job := &scanner.ScanJob{
		ID:     123,
		Status: scanner.ScanStatusFailed,
	}

	// Should not panic on database error
	CompleteSafely(context.Background(), deps, job)
}

func TestMarkFailed(t *testing.T) {
	now := time.Now()
	jobRepo := mocks.NewScanJobRepository(t)
	jobRepo.WithJobs(&scanner.ScanJob{
		ID:        111,
		LibraryID: 1,
		Status:    scanner.ScanStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	})

	deps := &Deps{
		ScanRepos: &scan.ScanRepositories{
			ScanJob: jobRepo,
		},
		Logger: discardLogger(),
	}

	MarkFailed(context.Background(), deps, 111, "processing failed", discardLogger(), "extra", "info")

	job, err := jobRepo.GetByID(context.Background(), 111)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if job.Status != scanner.ScanStatusFailed {
		t.Errorf("Status = %v, want %v", job.Status, scanner.ScanStatusFailed)
	}
	if job.ErrorMessage != "processing failed" {
		t.Errorf("ErrorMessage = %q, want %q", job.ErrorMessage, "processing failed")
	}
}

func TestMarkFailed_NilLogger(t *testing.T) {
	now := time.Now()
	jobRepo := mocks.NewScanJobRepository(t)
	jobRepo.WithJobs(&scanner.ScanJob{
		ID:        222,
		LibraryID: 1,
		Status:    scanner.ScanStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	})

	deps := &Deps{
		ScanRepos: &scan.ScanRepositories{
			ScanJob: jobRepo,
		},
		Logger: discardLogger(),
	}

	// Should not panic with nil logger
	MarkFailed(context.Background(), deps, 222, "error message", nil)

	job, err := jobRepo.GetByID(context.Background(), 222)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}
	if job.Status != scanner.ScanStatusFailed {
		t.Errorf("Status = %v, want %v", job.Status, scanner.ScanStatusFailed)
	}
}

func TestPtrTime(t *testing.T) {
	now := time.Now()
	ptr := ptrTime(now)

	if ptr == nil {
		t.Fatal("ptrTime returned nil")
	}
	if !ptr.Equal(now) {
		t.Errorf("ptrTime returned %v, want %v", *ptr, now)
	}
}

// =============================================================================
// Tests for progress.go
// =============================================================================

// mockProgressUpdater implements ProgressUpdater for testing
type mockProgressUpdater struct {
	updateCalled   int
	lastJobID      int64
	lastProgress   *scanner.Progress
	updateErr      error
}

func (m *mockProgressUpdater) UpdateProgress(ctx context.Context, jobID int64, progress *scanner.Progress) error {
	m.updateCalled++
	m.lastJobID = jobID
	m.lastProgress = progress
	return m.updateErr
}

func TestNewProgressUpdate(t *testing.T) {
	updater := &mockProgressUpdater{}
	logger := discardLogger()

	pu := NewProgressUpdate(updater, logger, 123)

	if pu == nil {
		t.Fatal("NewProgressUpdate returned nil")
	}
	if pu.jobID != 123 {
		t.Errorf("jobID = %d, want 123", pu.jobID)
	}
}

func TestProgressUpdate_BuilderChain(t *testing.T) {
	updater := &mockProgressUpdater{}
	logger := discardLogger()

	pu := NewProgressUpdate(updater, logger, 100).
		Phase(scanner.ScanPhaseProcessing).
		FilesFound(500).
		FilesProcessed(250).
		Errors(5).
		Warnings(10).
		EstimatedTotal(600).
		DiscoveryDone()

	if pu.phase != scanner.ScanPhaseProcessing {
		t.Errorf("phase = %v, want %v", pu.phase, scanner.ScanPhaseProcessing)
	}
	if pu.filesFound != 500 {
		t.Errorf("filesFound = %d, want 500", pu.filesFound)
	}
	if pu.filesProcessed != 250 {
		t.Errorf("filesProcessed = %d, want 250", pu.filesProcessed)
	}
	if pu.errorCount != 5 {
		t.Errorf("errorCount = %d, want 5", pu.errorCount)
	}
	if pu.warningCount != 10 {
		t.Errorf("warningCount = %d, want 10", pu.warningCount)
	}
	if pu.estimatedTotal != 600 {
		t.Errorf("estimatedTotal = %d, want 600", pu.estimatedTotal)
	}
	if !pu.discoveryDone {
		t.Error("discoveryDone should be true")
	}
}

func TestProgressUpdate_FromCheckpointStats(t *testing.T) {
	updater := &mockProgressUpdater{}
	logger := discardLogger()

	stats := &scanner.CheckpointStats{
		ProcessedFiles: 100,
		FailedFiles:    5,
		WarningFiles:   8,
	}

	pu := NewProgressUpdate(updater, logger, 1).FromCheckpointStats(stats)

	if pu.filesProcessed != 100 {
		t.Errorf("filesProcessed = %d, want 100", pu.filesProcessed)
	}
	if pu.errorCount != 5 {
		t.Errorf("errorCount = %d, want 5", pu.errorCount)
	}
	if pu.warningCount != 8 {
		t.Errorf("warningCount = %d, want 8", pu.warningCount)
	}
}

func TestProgressUpdate_FromCheckpointStats_Nil(t *testing.T) {
	updater := &mockProgressUpdater{}
	logger := discardLogger()

	// Should not panic with nil stats
	pu := NewProgressUpdate(updater, logger, 1).FromCheckpointStats(nil)

	if pu.filesProcessed != 0 {
		t.Errorf("filesProcessed = %d, want 0", pu.filesProcessed)
	}
}

func TestProgressUpdate_FromJob(t *testing.T) {
	updater := &mockProgressUpdater{}
	logger := discardLogger()

	job := &scanner.ScanJob{
		FilesFound:     200,
		Phase:          scanner.ScanPhaseProcessing,
		EstimatedTotal: 500,
		DiscoveryDone:  true,
	}

	pu := NewProgressUpdate(updater, logger, 1).FromJob(job)

	if pu.filesFound != 200 {
		t.Errorf("filesFound = %d, want 200", pu.filesFound)
	}
	if pu.phase != scanner.ScanPhaseProcessing {
		t.Errorf("phase = %v, want %v", pu.phase, scanner.ScanPhaseProcessing)
	}
	if pu.estimatedTotal != 500 {
		t.Errorf("estimatedTotal = %d, want 500", pu.estimatedTotal)
	}
	if !pu.discoveryDone {
		t.Error("discoveryDone should be true")
	}
}

func TestProgressUpdate_FromJob_Nil(t *testing.T) {
	updater := &mockProgressUpdater{}
	logger := discardLogger()

	// Should not panic with nil job
	pu := NewProgressUpdate(updater, logger, 1).FromJob(nil)

	if pu.filesFound != 0 {
		t.Errorf("filesFound = %d, want 0", pu.filesFound)
	}
}

func TestProgressUpdate_Update(t *testing.T) {
	updater := &mockProgressUpdater{}
	logger := discardLogger()

	err := NewProgressUpdate(updater, logger, 123).
		Phase(scanner.ScanPhaseProcessing).
		FilesFound(100).
		FilesProcessed(50).
		Errors(2).
		Warnings(5).
		EstimatedTotal(150).
		DiscoveryDone().
		Update(context.Background())

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if updater.updateCalled != 1 {
		t.Errorf("UpdateProgress called %d times, want 1", updater.updateCalled)
	}
	if updater.lastJobID != 123 {
		t.Errorf("jobID = %d, want 123", updater.lastJobID)
	}
	if updater.lastProgress == nil {
		t.Fatal("lastProgress should not be nil")
	}
	if updater.lastProgress.Phase != scanner.ScanPhaseProcessing {
		t.Errorf("Phase = %v, want %v", updater.lastProgress.Phase, scanner.ScanPhaseProcessing)
	}
	if updater.lastProgress.FilesFound != 100 {
		t.Errorf("FilesFound = %d, want 100", updater.lastProgress.FilesFound)
	}
	if updater.lastProgress.FilesProcessed != 50 {
		t.Errorf("FilesProcessed = %d, want 50", updater.lastProgress.FilesProcessed)
	}
	if updater.lastProgress.ErrorCount != 2 {
		t.Errorf("ErrorCount = %d, want 2", updater.lastProgress.ErrorCount)
	}
	if updater.lastProgress.WarningCount != 5 {
		t.Errorf("WarningCount = %d, want 5", updater.lastProgress.WarningCount)
	}
	if updater.lastProgress.EstimatedTotal != 150 {
		t.Errorf("EstimatedTotal = %d, want 150", updater.lastProgress.EstimatedTotal)
	}
	if !updater.lastProgress.DiscoveryDone {
		t.Error("DiscoveryDone should be true")
	}
}

func TestProgressUpdate_Update_Error(t *testing.T) {
	updater := &mockProgressUpdater{
		updateErr: errors.New("database error"),
	}
	logger := discardLogger()

	err := NewProgressUpdate(updater, logger, 123).
		FilesFound(100).
		Update(context.Background())

	if err == nil {
		t.Error("Expected error, got nil")
	}
	if err.Error() != "database error" {
		t.Errorf("Error = %q, want %q", err.Error(), "database error")
	}
}

func TestProgressUpdate_UpdateAsync(t *testing.T) {
	updater := &mockProgressUpdater{}
	logger := discardLogger()

	pu := NewProgressUpdate(updater, logger, 123).
		FilesFound(100).
		FilesProcessed(50)

	pu.UpdateAsync(context.Background())

	// Give the goroutine time to complete
	time.Sleep(50 * time.Millisecond)

	if updater.updateCalled != 1 {
		t.Errorf("UpdateProgress called %d times, want 1", updater.updateCalled)
	}
}

func TestProgressUpdate_UpdateAsync_Error(t *testing.T) {
	updater := &mockProgressUpdater{
		updateErr: errors.New("async error"),
	}
	logger := discardLogger()

	pu := NewProgressUpdate(updater, logger, 123).
		FilesFound(100)

	// Should not panic on async error
	pu.UpdateAsync(context.Background())

	// Give the goroutine time to complete
	time.Sleep(50 * time.Millisecond)

	if updater.updateCalled != 1 {
		t.Errorf("UpdateProgress called %d times, want 1", updater.updateCalled)
	}
}
