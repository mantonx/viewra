package library

import (
	"testing"
	"time"

	"github.com/viewra/viewra/internal/domain/scanner"
)

func TestToStartScanResponse(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		job  *scanner.ScanJob
		want StartScanResponse
	}{
		{
			name: "complete scan job",
			job: &scanner.ScanJob{
				ID:        42,
				LibraryID: 1,
				Status:    scanner.ScanStatusRunning,
				StartedAt: now,
			},
			want: StartScanResponse{
				JobID:     42,
				LibraryID: 1,
				Status:    string(scanner.ScanStatusRunning),
				StartedAt: now,
			},
		},
		{
			name: "pending scan job",
			job: &scanner.ScanJob{
				ID:        1,
				LibraryID: 5,
				Status:    scanner.ScanStatusPending,
				StartedAt: now,
			},
			want: StartScanResponse{
				JobID:     1,
				LibraryID: 5,
				Status:    string(scanner.ScanStatusPending),
				StartedAt: now,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToStartScanResponse(tt.job)

			if got.JobID != tt.want.JobID {
				t.Errorf("JobID = %v, want %v", got.JobID, tt.want.JobID)
			}
			if got.LibraryID != tt.want.LibraryID {
				t.Errorf("LibraryID = %v, want %v", got.LibraryID, tt.want.LibraryID)
			}
			if got.Status != tt.want.Status {
				t.Errorf("Status = %v, want %v", got.Status, tt.want.Status)
			}
			if !got.StartedAt.Equal(tt.want.StartedAt) {
				t.Errorf("StartedAt = %v, want %v", got.StartedAt, tt.want.StartedAt)
			}
		})
	}
}

func TestToScanProgressResponse(t *testing.T) {
	now := time.Now()
	completedTime := now.Add(5 * time.Minute)

	tests := []struct {
		name string
		job  *scanner.ScanJob
		want ScanProgressResponse
	}{
		{
			name: "running scan with progress",
			job: &scanner.ScanJob{
				ID:             100,
				LibraryID:      10,
				Status:         scanner.ScanStatusRunning,
				Progress:       45.5,
				FilesFound:     1000,
				FilesProcessed: 455,
				BytesProcessed: 1024000,
				ErrorCount:     5,
				StartedAt:      now,
				CompletedAt:    nil,
				ErrorMessage:   "",
			},
			want: ScanProgressResponse{
				JobID:          100,
				LibraryID:      10,
				Status:         string(scanner.ScanStatusRunning),
				Progress:       45.5,
				FilesFound:     1000,
				FilesProcessed: 455,
				BytesProcessed: 1024000,
				ErrorCount:     5,
				StartedAt:      now,
				CompletedAt:    nil,
				ErrorMessage:   "",
			},
		},
		{
			name: "completed scan",
			job: &scanner.ScanJob{
				ID:             200,
				LibraryID:      20,
				Status:         scanner.ScanStatusCompleted,
				Progress:       100.0,
				FilesFound:     500,
				FilesProcessed: 500,
				BytesProcessed: 5000000,
				ErrorCount:     0,
				StartedAt:      now,
				CompletedAt:    &completedTime,
				ErrorMessage:   "",
			},
			want: ScanProgressResponse{
				JobID:          200,
				LibraryID:      20,
				Status:         string(scanner.ScanStatusCompleted),
				Progress:       100.0,
				FilesFound:     500,
				FilesProcessed: 500,
				BytesProcessed: 5000000,
				ErrorCount:     0,
				StartedAt:      now,
				CompletedAt:    &completedTime,
				ErrorMessage:   "",
			},
		},
		{
			name: "failed scan with error message",
			job: &scanner.ScanJob{
				ID:             300,
				LibraryID:      30,
				Status:         scanner.ScanStatusFailed,
				Progress:       75.0,
				FilesFound:     100,
				FilesProcessed: 75,
				BytesProcessed: 750000,
				ErrorCount:     25,
				StartedAt:      now,
				CompletedAt:    &completedTime,
				ErrorMessage:   "filesystem error: permission denied",
			},
			want: ScanProgressResponse{
				JobID:          300,
				LibraryID:      30,
				Status:         string(scanner.ScanStatusFailed),
				Progress:       75.0,
				FilesFound:     100,
				FilesProcessed: 75,
				BytesProcessed: 750000,
				ErrorCount:     25,
				StartedAt:      now,
				CompletedAt:    &completedTime,
				ErrorMessage:   "filesystem error: permission denied",
			},
		},
		{
			name: "scan with zero progress",
			job: &scanner.ScanJob{
				ID:             400,
				LibraryID:      40,
				Status:         scanner.ScanStatusRunning,
				Progress:       0.0,
				FilesFound:     0,
				FilesProcessed: 0,
				BytesProcessed: 0,
				ErrorCount:     0,
				StartedAt:      now,
				CompletedAt:    nil,
				ErrorMessage:   "",
			},
			want: ScanProgressResponse{
				JobID:          400,
				LibraryID:      40,
				Status:         string(scanner.ScanStatusRunning),
				Progress:       0.0,
				FilesFound:     0,
				FilesProcessed: 0,
				BytesProcessed: 0,
				ErrorCount:     0,
				StartedAt:      now,
				CompletedAt:    nil,
				ErrorMessage:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToScanProgressResponse(tt.job)

			if got.JobID != tt.want.JobID {
				t.Errorf("JobID = %v, want %v", got.JobID, tt.want.JobID)
			}
			if got.LibraryID != tt.want.LibraryID {
				t.Errorf("LibraryID = %v, want %v", got.LibraryID, tt.want.LibraryID)
			}
			if got.Status != tt.want.Status {
				t.Errorf("Status = %v, want %v", got.Status, tt.want.Status)
			}
			if got.Progress != tt.want.Progress {
				t.Errorf("Progress = %v, want %v", got.Progress, tt.want.Progress)
			}
			if got.FilesFound != tt.want.FilesFound {
				t.Errorf("FilesFound = %v, want %v", got.FilesFound, tt.want.FilesFound)
			}
			if got.FilesProcessed != tt.want.FilesProcessed {
				t.Errorf("FilesProcessed = %v, want %v", got.FilesProcessed, tt.want.FilesProcessed)
			}
			if got.BytesProcessed != tt.want.BytesProcessed {
				t.Errorf("BytesProcessed = %v, want %v", got.BytesProcessed, tt.want.BytesProcessed)
			}
			if got.ErrorCount != tt.want.ErrorCount {
				t.Errorf("ErrorCount = %v, want %v", got.ErrorCount, tt.want.ErrorCount)
			}
			if !got.StartedAt.Equal(tt.want.StartedAt) {
				t.Errorf("StartedAt = %v, want %v", got.StartedAt, tt.want.StartedAt)
			}

			// Check CompletedAt (nullable)
			if tt.want.CompletedAt == nil {
				if got.CompletedAt != nil {
					t.Errorf("CompletedAt = %v, want nil", got.CompletedAt)
				}
			} else {
				if got.CompletedAt == nil {
					t.Errorf("CompletedAt = nil, want %v", *tt.want.CompletedAt)
				} else if !got.CompletedAt.Equal(*tt.want.CompletedAt) {
					t.Errorf("CompletedAt = %v, want %v", *got.CompletedAt, *tt.want.CompletedAt)
				}
			}

			if got.ErrorMessage != tt.want.ErrorMessage {
				t.Errorf("ErrorMessage = %v, want %v", got.ErrorMessage, tt.want.ErrorMessage)
			}
		})
	}
}

func TestToScanHistoryResponse(t *testing.T) {
	now := time.Now()
	completed1 := now.Add(5 * time.Minute)
	completed2 := now.Add(10 * time.Minute)

	tests := []struct {
		name string
		jobs []*scanner.ScanJob
		want ScanHistoryResponse
	}{
		{
			name: "multiple scan jobs",
			jobs: []*scanner.ScanJob{
				{
					ID:             1,
					LibraryID:      10,
					Status:         scanner.ScanStatusCompleted,
					Progress:       100.0,
					FilesFound:     100,
					FilesProcessed: 100,
					StartedAt:      now,
					CompletedAt:    &completed1,
				},
				{
					ID:             2,
					LibraryID:      10,
					Status:         scanner.ScanStatusFailed,
					Progress:       50.0,
					FilesFound:     200,
					FilesProcessed: 100,
					ErrorCount:     10,
					StartedAt:      now,
					CompletedAt:    &completed2,
					ErrorMessage:   "scan error",
				},
			},
			want: ScanHistoryResponse{
				Jobs: []ScanProgressResponse{
					{
						JobID:          1,
						LibraryID:      10,
						Status:         string(scanner.ScanStatusCompleted),
						Progress:       100.0,
						FilesFound:     100,
						FilesProcessed: 100,
						StartedAt:      now,
						CompletedAt:    &completed1,
					},
					{
						JobID:          2,
						LibraryID:      10,
						Status:         string(scanner.ScanStatusFailed),
						Progress:       50.0,
						FilesFound:     200,
						FilesProcessed: 100,
						ErrorCount:     10,
						StartedAt:      now,
						CompletedAt:    &completed2,
						ErrorMessage:   "scan error",
					},
				},
				Total: 2,
			},
		},
		{
			name: "empty scan history",
			jobs: []*scanner.ScanJob{},
			want: ScanHistoryResponse{
				Jobs:  []ScanProgressResponse{},
				Total: 0,
			},
		},
		{
			name: "single scan job",
			jobs: []*scanner.ScanJob{
				{
					ID:             100,
					LibraryID:      50,
					Status:         scanner.ScanStatusRunning,
					Progress:       25.0,
					FilesFound:     400,
					FilesProcessed: 100,
					StartedAt:      now,
					CompletedAt:    nil,
				},
			},
			want: ScanHistoryResponse{
				Jobs: []ScanProgressResponse{
					{
						JobID:          100,
						LibraryID:      50,
						Status:         string(scanner.ScanStatusRunning),
						Progress:       25.0,
						FilesFound:     400,
						FilesProcessed: 100,
						StartedAt:      now,
						CompletedAt:    nil,
					},
				},
				Total: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToScanHistoryResponse(tt.jobs)

			if got.Total != tt.want.Total {
				t.Errorf("Total = %v, want %v", got.Total, tt.want.Total)
			}

			if len(got.Jobs) != len(tt.want.Jobs) {
				t.Fatalf("Jobs length = %v, want %v", len(got.Jobs), len(tt.want.Jobs))
			}

			for i := range got.Jobs {
				if got.Jobs[i].JobID != tt.want.Jobs[i].JobID {
					t.Errorf("Jobs[%d].JobID = %v, want %v", i, got.Jobs[i].JobID, tt.want.Jobs[i].JobID)
				}
				if got.Jobs[i].Status != tt.want.Jobs[i].Status {
					t.Errorf("Jobs[%d].Status = %v, want %v", i, got.Jobs[i].Status, tt.want.Jobs[i].Status)
				}
				if got.Jobs[i].Progress != tt.want.Jobs[i].Progress {
					t.Errorf("Jobs[%d].Progress = %v, want %v", i, got.Jobs[i].Progress, tt.want.Jobs[i].Progress)
				}
			}
		})
	}
}
