package library

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// mockScanJobRepository is a mock implementation of scanner.ScanJobRepository
type mockScanJobRepository struct {
	jobs      map[int64]*scanner.ScanJob
	nextID    int64
	getByIDErr       error
	getLatestErr     error
	listByLibraryErr error
}

func newMockScanJobRepository() *mockScanJobRepository {
	return &mockScanJobRepository{
		jobs:   make(map[int64]*scanner.ScanJob),
		nextID: 1,
	}
}

func (m *mockScanJobRepository) Create(ctx context.Context, job *scanner.ScanJob) error {
	job.ID = m.nextID
	m.nextID++
	m.jobs[job.ID] = job
	return nil
}

func (m *mockScanJobRepository) GetByID(ctx context.Context, id int64) (*scanner.ScanJob, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	job, ok := m.jobs[id]
	if !ok {
		return nil, scanner.ErrNotFound
	}
	return job, nil
}

func (m *mockScanJobRepository) GetLatestByLibrary(ctx context.Context, libraryID int64) (*scanner.ScanJob, error) {
	if m.getLatestErr != nil {
		return nil, m.getLatestErr
	}

	var latest *scanner.ScanJob
	for _, job := range m.jobs {
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

func (m *mockScanJobRepository) ListByLibrary(ctx context.Context, libraryID int64, limit int32) ([]*scanner.ScanJob, error) {
	if m.listByLibraryErr != nil {
		return nil, m.listByLibraryErr
	}

	var jobs []*scanner.ScanJob
	for _, job := range m.jobs {
		if job.LibraryID == libraryID {
			jobs = append(jobs, job)
		}
	}

	// Apply limit
	if limit > 0 && len(jobs) > int(limit) {
		jobs = jobs[:limit]
	}

	return jobs, nil
}

func (m *mockScanJobRepository) ListRunning(ctx context.Context) ([]*scanner.ScanJob, error) {
	var running []*scanner.ScanJob
	for _, job := range m.jobs {
		if job.Status == scanner.ScanStatusRunning {
			running = append(running, job)
		}
	}
	return running, nil
}

func (m *mockScanJobRepository) UpdateProgress(ctx context.Context, id int64, progress *scanner.Progress) error {
	job, ok := m.jobs[id]
	if !ok {
		return scanner.ErrNotFound
	}
	job.FilesFound = progress.FilesFound
	job.FilesProcessed = progress.FilesProcessed
	job.BytesProcessed = progress.BytesProcessed
	job.ErrorCount = progress.ErrorCount
	job.Progress = progress.GetPercentage()
	return nil
}

func (m *mockScanJobRepository) UpdateStatus(ctx context.Context, id int64, status scanner.ScanStatus) error {
	job, ok := m.jobs[id]
	if !ok {
		return scanner.ErrNotFound
	}
	job.Status = status
	return nil
}

func (m *mockScanJobRepository) Complete(ctx context.Context, job *scanner.ScanJob) error {
	existing, ok := m.jobs[job.ID]
	if !ok {
		return scanner.ErrNotFound
	}
	existing.Status = job.Status
	existing.Progress = job.Progress
	existing.FilesFound = job.FilesFound
	existing.FilesProcessed = job.FilesProcessed
	existing.BytesProcessed = job.BytesProcessed
	existing.ErrorCount = job.ErrorCount
	existing.CompletedAt = job.CompletedAt
	existing.ErrorMessage = job.ErrorMessage
	return nil
}

func (m *mockScanJobRepository) Delete(ctx context.Context, id int64) error {
	delete(m.jobs, id)
	return nil
}

func (m *mockScanJobRepository) DeleteOld(ctx context.Context, libraryID int64, retentionDays int) error {
	return nil
}

func TestScanLibraryUseCase_GetProgress(t *testing.T) {
	now := time.Now()
	completedTime := now.Add(5 * time.Minute)

	tests := []struct {
		name      string
		jobID     int64
		setupMock func(*mockScanJobRepository)
		wantErr   bool
		checkResp func(*testing.T, ScanProgressResponse)
	}{
		{
			name:  "get progress for running scan",
			jobID: 1,
			setupMock: func(repo *mockScanJobRepository) {
				repo.jobs[1] = &scanner.ScanJob{
					ID:             1,
					LibraryID:      10,
					Status:         scanner.ScanStatusRunning,
					Progress:       45.5,
					FilesFound:     1000,
					FilesProcessed: 455,
					BytesProcessed: 1024000,
					ErrorCount:     5,
					StartedAt:      now,
					CreatedAt:      now,
					UpdatedAt:      now,
				}
			},
			wantErr: false,
			checkResp: func(t *testing.T, resp ScanProgressResponse) {
				if resp.JobID != 1 {
					t.Errorf("JobID = %v, want 1", resp.JobID)
				}
				if resp.Status != string(scanner.ScanStatusRunning) {
					t.Errorf("Status = %v, want %v", resp.Status, scanner.ScanStatusRunning)
				}
				if resp.Progress != 45.5 {
					t.Errorf("Progress = %v, want 45.5", resp.Progress)
				}
				if resp.FilesProcessed != 455 {
					t.Errorf("FilesProcessed = %v, want 455", resp.FilesProcessed)
				}
			},
		},
		{
			name:  "get progress for completed scan",
			jobID: 2,
			setupMock: func(repo *mockScanJobRepository) {
				repo.jobs[2] = &scanner.ScanJob{
					ID:             2,
					LibraryID:      20,
					Status:         scanner.ScanStatusCompleted,
					Progress:       100.0,
					FilesFound:     500,
					FilesProcessed: 500,
					BytesProcessed: 5000000,
					ErrorCount:     0,
					StartedAt:      now,
					CompletedAt:    &completedTime,
					CreatedAt:      now,
					UpdatedAt:      now,
				}
			},
			wantErr: false,
			checkResp: func(t *testing.T, resp ScanProgressResponse) {
				if resp.JobID != 2 {
					t.Errorf("JobID = %v, want 2", resp.JobID)
				}
				if resp.Status != string(scanner.ScanStatusCompleted) {
					t.Errorf("Status = %v, want %v", resp.Status, scanner.ScanStatusCompleted)
				}
				if resp.Progress != 100.0 {
					t.Errorf("Progress = %v, want 100.0", resp.Progress)
				}
				if resp.CompletedAt == nil {
					t.Error("CompletedAt should not be nil for completed scan")
				}
			},
		},
		{
			name:  "job not found",
			jobID: 999,
			setupMock: func(repo *mockScanJobRepository) {
				// No job with ID 999
			},
			wantErr: true,
		},
		{
			name:  "repository error",
			jobID: 3,
			setupMock: func(repo *mockScanJobRepository) {
				repo.getByIDErr = errors.New("database connection error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := newMockScanJobRepository()
			if tt.setupMock != nil {
				tt.setupMock(mockRepo)
			}

			uc := &ScanLibraryUseCase{
				scanJobRepo: mockRepo,
			}

			resp, err := uc.GetProgress(context.Background(), tt.jobID)

			if tt.wantErr {
				if err == nil {
					t.Error("GetProgress() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetProgress() unexpected error = %v", err)
				return
			}

			if tt.checkResp != nil {
				tt.checkResp(t, resp)
			}
		})
	}
}

func TestScanLibraryUseCase_GetLatestScan(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-1 * time.Hour)

	tests := []struct {
		name      string
		libraryID int64
		setupMock func(*mockScanJobRepository)
		wantErr   bool
		checkResp func(*testing.T, ScanProgressResponse)
	}{
		{
			name:      "get latest scan for library with multiple scans",
			libraryID: 1,
			setupMock: func(repo *mockScanJobRepository) {
				// Older scan
				repo.jobs[1] = &scanner.ScanJob{
					ID:        1,
					LibraryID: 1,
					Status:    scanner.ScanStatusCompleted,
					Progress:  100.0,
					StartedAt: earlier,
					CreatedAt: earlier,
					UpdatedAt: earlier,
				}
				// Newer scan (should be returned)
				repo.jobs[2] = &scanner.ScanJob{
					ID:        2,
					LibraryID: 1,
					Status:    scanner.ScanStatusRunning,
					Progress:  50.0,
					StartedAt: now,
					CreatedAt: now,
					UpdatedAt: now,
				}
			},
			wantErr: false,
			checkResp: func(t *testing.T, resp ScanProgressResponse) {
				if resp.JobID != 2 {
					t.Errorf("JobID = %v, want 2 (latest)", resp.JobID)
				}
			},
		},
		{
			name:      "no scans for library",
			libraryID: 999,
			setupMock: func(repo *mockScanJobRepository) {
				// Library 1 has scans, but not library 999
				repo.jobs[1] = &scanner.ScanJob{
					ID:        1,
					LibraryID: 1,
					Status:    scanner.ScanStatusCompleted,
					CreatedAt: now,
				}
			},
			wantErr: true,
		},
		{
			name:      "repository error",
			libraryID: 1,
			setupMock: func(repo *mockScanJobRepository) {
				repo.getLatestErr = errors.New("database error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := newMockScanJobRepository()
			if tt.setupMock != nil {
				tt.setupMock(mockRepo)
			}

			uc := &ScanLibraryUseCase{
				scanJobRepo: mockRepo,
			}

			resp, err := uc.GetLatestScan(context.Background(), tt.libraryID)

			if tt.wantErr {
				if err == nil {
					t.Error("GetLatestScan() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetLatestScan() unexpected error = %v", err)
				return
			}

			if tt.checkResp != nil {
				tt.checkResp(t, resp)
			}
		})
	}
}

func TestScanLibraryUseCase_GetScanHistory(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		libraryID int64
		limit     int32
		setupMock func(*mockScanJobRepository)
		wantErr   bool
		checkResp func(*testing.T, ScanHistoryResponse)
	}{
		{
			name:      "get history with multiple scans",
			libraryID: 1,
			limit:     10,
			setupMock: func(repo *mockScanJobRepository) {
				for i := 1; i <= 3; i++ {
					repo.jobs[int64(i)] = &scanner.ScanJob{
						ID:        int64(i),
						LibraryID: 1,
						Status:    scanner.ScanStatusCompleted,
						Progress:  100.0,
						CreatedAt: now.Add(time.Duration(i) * time.Minute),
					}
				}
			},
			wantErr: false,
			checkResp: func(t *testing.T, resp ScanHistoryResponse) {
				if resp.Total != 3 {
					t.Errorf("Total = %v, want 3", resp.Total)
				}
				if len(resp.Jobs) != 3 {
					t.Errorf("Jobs length = %v, want 3", len(resp.Jobs))
				}
			},
		},
		{
			name:      "empty history",
			libraryID: 999,
			limit:     10,
			setupMock: func(repo *mockScanJobRepository) {
				// Library 1 has scans, but not library 999
				repo.jobs[1] = &scanner.ScanJob{
					ID:        1,
					LibraryID: 1,
					Status:    scanner.ScanStatusCompleted,
					CreatedAt: now,
				}
			},
			wantErr: false,
			checkResp: func(t *testing.T, resp ScanHistoryResponse) {
				if resp.Total != 0 {
					t.Errorf("Total = %v, want 0", resp.Total)
				}
				if len(resp.Jobs) != 0 {
					t.Errorf("Jobs length = %v, want 0", len(resp.Jobs))
				}
			},
		},
		{
			name:      "apply limit",
			libraryID: 1,
			limit:     2,
			setupMock: func(repo *mockScanJobRepository) {
				for i := 1; i <= 5; i++ {
					repo.jobs[int64(i)] = &scanner.ScanJob{
						ID:        int64(i),
						LibraryID: 1,
						Status:    scanner.ScanStatusCompleted,
						CreatedAt: now,
					}
				}
			},
			wantErr: false,
			checkResp: func(t *testing.T, resp ScanHistoryResponse) {
				if len(resp.Jobs) != 2 {
					t.Errorf("Jobs length = %v, want 2 (limited)", len(resp.Jobs))
				}
				if resp.Total != 2 {
					t.Errorf("Total = %v, want 2", resp.Total)
				}
			},
		},
		{
			name:      "zero limit returns all",
			libraryID: 1,
			limit:     0,
			setupMock: func(repo *mockScanJobRepository) {
				for i := 1; i <= 3; i++ {
					repo.jobs[int64(i)] = &scanner.ScanJob{
						ID:        int64(i),
						LibraryID: 1,
						Status:    scanner.ScanStatusCompleted,
						CreatedAt: now,
					}
				}
			},
			wantErr: false,
			checkResp: func(t *testing.T, resp ScanHistoryResponse) {
				if len(resp.Jobs) != 3 {
					t.Errorf("Jobs length = %v, want 3 (no limit)", len(resp.Jobs))
				}
			},
		},
		{
			name:      "repository error",
			libraryID: 1,
			limit:     10,
			setupMock: func(repo *mockScanJobRepository) {
				repo.listByLibraryErr = errors.New("database error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := newMockScanJobRepository()
			if tt.setupMock != nil {
				tt.setupMock(mockRepo)
			}

			uc := &ScanLibraryUseCase{
				scanJobRepo: mockRepo,
			}

			resp, err := uc.GetScanHistory(context.Background(), tt.libraryID, tt.limit)

			if tt.wantErr {
				if err == nil {
					t.Error("GetScanHistory() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetScanHistory() unexpected error = %v", err)
				return
			}

			if tt.checkResp != nil {
				tt.checkResp(t, resp)
			}
		})
	}
}
