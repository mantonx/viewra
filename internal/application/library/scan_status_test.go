package library

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/application/library/scan/status"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// Mock repositories for scan status tests

type mockScanStateRepositoryForStatus struct {
	countLibraryIssuesFunc func(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error)
	countByLibraryFunc     func(ctx context.Context, libraryID int64) (int64, error)
}

func (m *mockScanStateRepositoryForStatus) GetLibraryState(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	return nil, nil
}

func (m *mockScanStateRepositoryForStatus) Upsert(ctx context.Context, state *scanner.ScanState) error {
	return nil
}

func (m *mockScanStateRepositoryForStatus) UpsertBatch(ctx context.Context, states []*scanner.ScanState) error {
	return nil
}

func (m *mockScanStateRepositoryForStatus) DeleteByPath(ctx context.Context, libraryID int64, filePath string) error {
	return nil
}

func (m *mockScanStateRepositoryForStatus) DeleteByPaths(ctx context.Context, libraryID int64, filePaths []string) error {
	return nil
}

func (m *mockScanStateRepositoryForStatus) DeleteByLibrary(ctx context.Context, libraryID int64) error {
	return nil
}

func (m *mockScanStateRepositoryForStatus) GetByPath(ctx context.Context, libraryID int64, filePath string) (*scanner.ScanState, error) {
	return nil, nil
}

func (m *mockScanStateRepositoryForStatus) CountByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	if m.countByLibraryFunc != nil {
		return m.countByLibraryFunc(ctx, libraryID)
	}
	return 0, nil
}

func (m *mockScanStateRepositoryForStatus) GetLibraryWarnings(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	return nil, nil
}

func (m *mockScanStateRepositoryForStatus) GetLibraryErrors(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	return nil, nil
}

func (m *mockScanStateRepositoryForStatus) GetLibraryIssues(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	return nil, nil
}

func (m *mockScanStateRepositoryForStatus) CountLibraryIssues(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error) {
	if m.countLibraryIssuesFunc != nil {
		return m.countLibraryIssuesFunc(ctx, libraryID)
	}
	return nil, nil
}

func (m *mockScanStateRepositoryForStatus) SetWarning(ctx context.Context, libraryID int64, filePath, message, category string) error {
	return nil
}

func (m *mockScanStateRepositoryForStatus) SetError(ctx context.Context, libraryID int64, filePath, message, category string) error {
	return nil
}

func (m *mockScanStateRepositoryForStatus) ClearWarning(ctx context.Context, libraryID int64, filePath string) error {
	return nil
}

func (m *mockScanStateRepositoryForStatus) ClearError(ctx context.Context, libraryID int64, filePath string) error {
	return nil
}


type mockCheckpointRepositoryForStatus struct {
	getStatsFunc func(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error)
}

func (m *mockCheckpointRepositoryForStatus) CreateBatch(ctx context.Context, checkpoints []*scanner.ScanCheckpoint) error {
	return nil
}

func (m *mockCheckpointRepositoryForStatus) GetPendingBatch(ctx context.Context, jobID int64, limit int) ([]*scanner.ScanCheckpoint, error) {
	return nil, nil
}

func (m *mockCheckpointRepositoryForStatus) UpdateStatus(ctx context.Context, id int64, status scanner.CheckpointStatus, errorMsg string, errorCategory scanner.ErrorCategory) error {
	return nil
}

func (m *mockCheckpointRepositoryForStatus) UpdateRetryCount(ctx context.Context, id int64, retryCount int) error {
	return nil
}

func (m *mockCheckpointRepositoryForStatus) GetStats(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error) {
	if m.getStatsFunc != nil {
		return m.getStatsFunc(ctx, jobID)
	}
	return nil, nil
}

func (m *mockCheckpointRepositoryForStatus) ListFailed(ctx context.Context, jobID int64, limit int) ([]*scanner.ScanCheckpoint, error) {
	return nil, nil
}

func (m *mockCheckpointRepositoryForStatus) GetByPath(ctx context.Context, jobID int64, filePath string) (*scanner.ScanCheckpoint, error) {
	return nil, nil
}

func (m *mockCheckpointRepositoryForStatus) ResetFailed(ctx context.Context, jobID int64) (int64, error) {
	return 0, nil
}

func (m *mockCheckpointRepositoryForStatus) DeleteByJobID(ctx context.Context, jobID int64) error {
	return nil
}

func TestScanLibraryUseCase_GetScanStatus(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(-1 * time.Hour)

	tests := []struct {
		name       string
		libraryID  int64
		setupMocks func(*mocks.ScanJobRepository, *mockScanStateRepositoryForStatus, *mockCheckpointRepositoryForStatus)
		wantErr    bool
		checkErr   func(*testing.T, error)
		checkResult func(*testing.T, *status.Result)
	}{
		{
			name:      "successful status retrieval with running scan",
			libraryID: 1,
			setupMocks: func(jobRepo *mocks.ScanJobRepository, stateRepo *mockScanStateRepositoryForStatus, checkRepo *mockCheckpointRepositoryForStatus) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:        1,
					LibraryID: 1,
					Status:    scanner.ScanStatusRunning,
					Phase:     scanner.ScanPhaseProcessing,
					StartedAt: now,
					FilesFound: 100,
					FilesProcessed: 50,
					BytesProcessed: 1024000,
					Progress:       50.0,
					ErrorCount:   2,
					WarningCount: 5,
					EstimatedTotal: 100,
					DiscoveryDone: true,
					CreatedAt: now,
					UpdatedAt: now,
				})
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *status.Result) {
				if result.JobID != 1 {
					t.Errorf("JobID = %d, want 1", result.JobID)
				}
				if result.LibraryID != 1 {
					t.Errorf("LibraryID = %d, want 1", result.LibraryID)
				}
				if result.Status != scanner.ScanStatusRunning {
					t.Errorf("Status = %v, want %v", result.Status, scanner.ScanStatusRunning)
				}
				if result.Phase != scanner.ScanPhaseProcessing {
					t.Errorf("Phase = %v, want %v", result.Phase, scanner.ScanPhaseProcessing)
				}
				if result.FilesFound != 100 {
					t.Errorf("FilesFound = %d, want 100", result.FilesFound)
				}
			},
		},
		{
			name:      "successful status retrieval with completed scan",
			libraryID: 1,
			setupMocks: func(jobRepo *mocks.ScanJobRepository, stateRepo *mockScanStateRepositoryForStatus, checkRepo *mockCheckpointRepositoryForStatus) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:          1,
					LibraryID:   1,
					Status:      scanner.ScanStatusCompleted,
					Phase:       scanner.ScanPhaseCompleted,
					StartedAt:   now.Add(-2 * time.Hour),
					CompletedAt: &completedAt,
					FilesFound: 100,
					FilesProcessed: 100,
					BytesProcessed: 2048000,
					Progress:       100.0,
					ErrorCount:   0,
					WarningCount: 0,
					EstimatedTotal: 100,
					DiscoveryDone: true,
					CreatedAt: now.Add(-2 * time.Hour),
					UpdatedAt: completedAt,
				})

				// Mock scan state to return 100 processed files so progress stays at 100%
				stateRepo.countByLibraryFunc = func(ctx context.Context, libraryID int64) (int64, error) {
					return 100, nil
				}
				stateRepo.countLibraryIssuesFunc = func(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error) {
					return &scanner.LibraryIssueCounts{
						ErrorCount:   0,
						WarningCount: 0,
					}, nil
				}
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *status.Result) {
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
			setupMocks: func(jobRepo *mocks.ScanJobRepository, stateRepo *mockScanStateRepositoryForStatus, checkRepo *mockCheckpointRepositoryForStatus) {
				// No jobs in repository
			},
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				if !errors.Is(err, scanner.ErrNotFound) {
					t.Errorf("Expected ErrNotFound, got %v", err)
				}
			},
		},
		{
			name:      "scan state enrichment with counts",
			libraryID: 1,
			setupMocks: func(jobRepo *mocks.ScanJobRepository, stateRepo *mockScanStateRepositoryForStatus, checkRepo *mockCheckpointRepositoryForStatus) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:        1,
					LibraryID: 1,
					Status:    scanner.ScanStatusRunning,
					Phase:     scanner.ScanPhaseProcessing,
					StartedAt: now,
					FilesFound: 100,
					FilesProcessed: 50,
					ErrorCount:   2,
					WarningCount: 5,
					CreatedAt: now,
					UpdatedAt: now,
				})

				// Mock scan state counts
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
			checkResult: func(t *testing.T, result *status.Result) {
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
		{
			name:      "ETA calculation for running scan",
			libraryID: 1,
			setupMocks: func(jobRepo *mocks.ScanJobRepository, stateRepo *mockScanStateRepositoryForStatus, checkRepo *mockCheckpointRepositoryForStatus) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:        1,
					LibraryID: 1,
					Status:    scanner.ScanStatusRunning,
					Phase:     scanner.ScanPhaseProcessing,
					StartedAt: now,
					FilesFound: 100,
					FilesProcessed: 50,
					CreatedAt: now,
					UpdatedAt: now,
				})

				firstProcessed := now.Add(-30 * time.Second)
				checkRepo.getStatsFunc = func(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error) {
					return &scanner.CheckpointStats{
						TotalFiles:       100,
						ProcessedFiles:   50,
						CompletedFiles:   45,
						FailedFiles:      5,
						FirstProcessedAt: &firstProcessed,
					}, nil
				}
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *status.Result) {
				if result.ETASeconds == nil {
					t.Error("ETASeconds should not be nil for running scan with stats")
				} else {
					// With 50 files processed in 30 seconds, rate = 50/30 = 1.67 files/sec
					// Remaining = 100 - 50 = 50 files
					// ETA = 50 / 1.67 = ~30 seconds
					if *result.ETASeconds < 25 || *result.ETASeconds > 35 {
						t.Errorf("ETASeconds = %d, expected around 30", *result.ETASeconds)
					}
				}
			},
		},
		{
			name:      "no ETA for completed scan",
			libraryID: 1,
			setupMocks: func(jobRepo *mocks.ScanJobRepository, stateRepo *mockScanStateRepositoryForStatus, checkRepo *mockCheckpointRepositoryForStatus) {
				jobRepo.WithJobs(&scanner.ScanJob{
					ID:          1,
					LibraryID:   1,
					Status:      scanner.ScanStatusCompleted,
					Phase:       scanner.ScanPhaseCompleted,
					StartedAt:   now.Add(-1 * time.Hour),
					CompletedAt: &completedAt,
					FilesFound: 100,
					FilesProcessed: 100,
					CreatedAt: now.Add(-1 * time.Hour),
					UpdatedAt: completedAt,
				})
			},
			wantErr: false,
			checkResult: func(t *testing.T, result *status.Result) {
				if result.ETASeconds != nil {
					t.Errorf("ETASeconds should be nil for completed scan, got %v", *result.ETASeconds)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			jobRepo := mocks.NewScanJobRepository(t)
			stateRepo := &mockScanStateRepositoryForStatus{}
			checkRepo := &mockCheckpointRepositoryForStatus{}

			// Setup mocks
			if tt.setupMocks != nil {
				tt.setupMocks(jobRepo, stateRepo, checkRepo)
			}

			// Create use case
			uc := &ScanLibraryUseCase{
				scanRepos: &scan.ScanRepositories{
					ScanJob:    jobRepo,
					ScanState:  stateRepo,
					Checkpoint: checkRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			// Execute
			result, err := uc.GetScanStatus(context.Background(), tt.libraryID)

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
					return
				}
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
				return
			}

			// Check success expectations
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

func TestScanLibraryUseCase_enrichWithScanState(t *testing.T) {
	tests := []struct {
		name       string
		libraryID  int64
		job        *scanner.ScanJob
		status     *status.Result
		setupMocks func(*mockScanStateRepositoryForStatus)
		checkResult func(*testing.T, *status.Result)
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
			status: &status.Result{
				FilesFound:     100,
				FilesProcessed: 40,
				ErrorCount:     1,
				WarningCount:   2,
				Progress:       40.0,
			},
			setupMocks: func(repo *mockScanStateRepositoryForStatus) {
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
			checkResult: func(t *testing.T, result *status.Result) {
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
			name:      "error getting issue counts - uses job counts",
			libraryID: 1,
			job: &scanner.ScanJob{
				ID:             1,
				LibraryID:      1,
				FilesFound:     100,
				FilesProcessed: 40,
				ErrorCount:     3,
				WarningCount:   7,
			},
			status: &status.Result{
				FilesFound:     100,
				FilesProcessed: 40,
				ErrorCount:     3,
				WarningCount:   7,
			},
			setupMocks: func(repo *mockScanStateRepositoryForStatus) {
				repo.countLibraryIssuesFunc = func(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error) {
					return nil, errors.New("database error")
				}
			},
			checkResult: func(t *testing.T, result *status.Result) {
				// Should keep original job counts on error
				if result.ErrorCount != 3 {
					t.Errorf("ErrorCount = %d, want 3 (job count)", result.ErrorCount)
				}
				if result.WarningCount != 7 {
					t.Errorf("WarningCount = %d, want 7 (job count)", result.WarningCount)
				}
			},
		},
		{
			name:      "error getting processed count - uses job count",
			libraryID: 1,
			job: &scanner.ScanJob{
				ID:             1,
				LibraryID:      1,
				FilesFound:     100,
				FilesProcessed: 40,
				Progress:       40.0,
			},
			status: &status.Result{
				FilesFound:     100,
				FilesProcessed: 40,
				Progress:       40.0,
			},
			setupMocks: func(repo *mockScanStateRepositoryForStatus) {
				repo.countByLibraryFunc = func(ctx context.Context, libraryID int64) (int64, error) {
					return 0, errors.New("database error")
				}
			},
			checkResult: func(t *testing.T, result *status.Result) {
				// Should keep original job values on error
				if result.FilesProcessed != 40 {
					t.Errorf("FilesProcessed = %d, want 40 (job count)", result.FilesProcessed)
				}
				if result.Progress != 40.0 {
					t.Errorf("Progress = %f, want 40.0 (job progress)", result.Progress)
				}
			},
		},
		{
			name:      "zero files found - no division by zero",
			libraryID: 1,
			job: &scanner.ScanJob{
				ID:             1,
				LibraryID:      1,
				FilesFound:     0,
				FilesProcessed: 0,
				Progress:       0,
			},
			status: &status.Result{
				FilesFound:     0,
				FilesProcessed: 0,
				Progress:       0,
			},
			setupMocks: func(repo *mockScanStateRepositoryForStatus) {
				repo.countByLibraryFunc = func(ctx context.Context, libraryID int64) (int64, error) {
					return 5, nil
				}
			},
			checkResult: func(t *testing.T, result *status.Result) {
				// Progress should remain 0, not cause division by zero
				if result.Progress != 0 {
					t.Errorf("Progress = %f, want 0 (no files found)", result.Progress)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			repo := &mockScanStateRepositoryForStatus{}

			// Setup mocks
			if tt.setupMocks != nil {
				tt.setupMocks(repo)
			}

			// Create use case
			uc := &ScanLibraryUseCase{
				scanRepos: &scan.ScanRepositories{
					ScanState: repo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			// Execute
			uc.enrichWithScanState(context.Background(), tt.libraryID, tt.job, tt.status)

			// Check results
			if tt.checkResult != nil {
				tt.checkResult(t, tt.status)
			}
		})
	}
}

func TestScanLibraryUseCase_enrichWithETA(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		jobID       int64
		filesFound  int64
		status      *status.Result
		setupMocks  func(*mockCheckpointRepositoryForStatus)
		checkResult func(*testing.T, *status.Result)
	}{
		{
			name:       "successful ETA calculation",
			jobID:      1,
			filesFound: 100,
			status:     &status.Result{},
			setupMocks: func(repo *mockCheckpointRepositoryForStatus) {
				firstProcessed := now.Add(-60 * time.Second)
				repo.getStatsFunc = func(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error) {
					return &scanner.CheckpointStats{
						TotalFiles:       100,
						ProcessedFiles:   30,
						CompletedFiles:   28,
						FailedFiles:      2,
						FirstProcessedAt: &firstProcessed,
					}, nil
				}
			},
			checkResult: func(t *testing.T, result *status.Result) {
				if result.ETASeconds == nil {
					t.Error("ETASeconds should not be nil")
				} else {
					// 30 files in 60 seconds = 0.5 files/sec
					// Remaining: 100 - 30 = 70 files
					// ETA: 70 / 0.5 = 140 seconds
					if *result.ETASeconds < 130 || *result.ETASeconds > 150 {
						t.Errorf("ETASeconds = %d, expected around 140", *result.ETASeconds)
					}
				}
			},
		},
		{
			name:       "error getting stats - no ETA",
			jobID:      1,
			filesFound: 100,
			status:     &status.Result{},
			setupMocks: func(repo *mockCheckpointRepositoryForStatus) {
				repo.getStatsFunc = func(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error) {
					return nil, errors.New("database error")
				}
			},
			checkResult: func(t *testing.T, result *status.Result) {
				if result.ETASeconds != nil {
					t.Errorf("ETASeconds should be nil on error, got %v", *result.ETASeconds)
				}
			},
		},
		{
			name:       "nil stats - no ETA",
			jobID:      1,
			filesFound: 100,
			status:     &status.Result{},
			setupMocks: func(repo *mockCheckpointRepositoryForStatus) {
				repo.getStatsFunc = func(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error) {
					return nil, nil
				}
			},
			checkResult: func(t *testing.T, result *status.Result) {
				if result.ETASeconds != nil {
					t.Errorf("ETASeconds should be nil when stats are nil, got %v", *result.ETASeconds)
				}
			},
		},
		{
			name:       "stats with nil EstimateRemainingSeconds - no ETA",
			jobID:      1,
			filesFound: 100,
			status:     &status.Result{},
			setupMocks: func(repo *mockCheckpointRepositoryForStatus) {
				repo.getStatsFunc = func(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error) {
					// No FirstProcessedAt, so EstimateRemainingSeconds will return nil
					return &scanner.CheckpointStats{
						TotalFiles:       100,
						ProcessedFiles:   30,
						FirstProcessedAt: nil,
					}, nil
				}
			},
			checkResult: func(t *testing.T, result *status.Result) {
				if result.ETASeconds != nil {
					t.Errorf("ETASeconds should be nil when EstimateRemainingSeconds returns nil, got %v", *result.ETASeconds)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			repo := &mockCheckpointRepositoryForStatus{}

			// Setup mocks
			if tt.setupMocks != nil {
				tt.setupMocks(repo)
			}

			// Create use case
			uc := &ScanLibraryUseCase{
				scanRepos: &scan.ScanRepositories{
					Checkpoint: repo,
				},
			}

			// Execute
			uc.enrichWithETA(context.Background(), tt.jobID, tt.filesFound, tt.status)

			// Check results
			if tt.checkResult != nil {
				tt.checkResult(t, tt.status)
			}
		})
	}
}
