package discovery

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func testDiscoveryLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// =============================================================================
// Tests for CheckWalkStatsErrors
// =============================================================================

func TestCheckWalkStatsErrors(t *testing.T) {
	tests := []struct {
		name             string
		stats            *filesystem.WalkStats
		expectedWarnings int
		wantContains     []string // Substrings that should appear in warnings
	}{
		{
			name:             "nil stats returns no warnings",
			stats:            nil,
			expectedWarnings: 0,
		},
		{
			name: "no errors returns no warnings",
			stats: &filesystem.WalkStats{
				FilesDiscovered: 100,
				DirsScanned:     10,
			},
			expectedWarnings: 0,
		},
		{
			name: "skipped dirs generates warning",
			stats: &filesystem.WalkStats{
				DirsSkipped: 5,
			},
			expectedWarnings: 1,
			wantContains:     []string{"Failed to read 5 directories"},
		},
		{
			name: "skipped files generates warning",
			stats: &filesystem.WalkStats{
				FilesSkipped: 10,
			},
			expectedWarnings: 1,
			wantContains:     []string{"Failed to stat 10 files"},
		},
		{
			name: "permission errors below threshold - no warning",
			stats: &filesystem.WalkStats{
				DirsSkipped:      1, // Needed for HasErrors()
				PermissionErrors: 5, // Below PermissionErrorWarningThreshold (10)
			},
			expectedWarnings: 1, // Only DirsSkipped warning
		},
		{
			name: "permission errors above threshold generates warning",
			stats: &filesystem.WalkStats{
				DirsSkipped:      1, // Needed for HasErrors()
				PermissionErrors: 15,
			},
			expectedWarnings: 2,
			wantContains:     []string{"15 permission errors"},
		},
		{
			name: "network errors generates warning",
			stats: &filesystem.WalkStats{
				FilesSkipped:  1, // Needed for HasErrors()
				NetworkErrors: 3,
			},
			expectedWarnings: 2,
			wantContains:     []string{"3 network/timeout errors"},
		},
		{
			name: "multiple error types",
			stats: &filesystem.WalkStats{
				DirsSkipped:      2,
				FilesSkipped:     5,
				PermissionErrors: 20,
				NetworkErrors:    3,
			},
			expectedWarnings: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := CheckWalkStatsErrors(tt.stats)

			if len(warnings) != tt.expectedWarnings {
				t.Errorf("expected %d warnings, got %d: %v", tt.expectedWarnings, len(warnings), warnings)
			}

			for _, want := range tt.wantContains {
				found := false
				for _, w := range warnings {
					if containsSubstring(w, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected warning containing %q, got: %v", want, warnings)
				}
			}
		})
	}
}

// =============================================================================
// Tests for DetectFileDrop
// =============================================================================

func TestDetectFileDrop(t *testing.T) {
	tests := []struct {
		name          string
		currentCount  int64
		previousCount int64
		wantWarning   bool
		wantContains  string
	}{
		{
			name:          "no drop - same count",
			currentCount:  1000,
			previousCount: 1000,
			wantWarning:   false,
		},
		{
			name:          "small drop below threshold",
			currentCount:  950,
			previousCount: 1000,
			wantWarning:   false, // 5% drop, below 10% threshold
		},
		{
			name:          "drop at threshold boundary",
			currentCount:  900,
			previousCount: 1000,
			wantWarning:   false, // Exactly 10%, not > 10%
		},
		{
			name:          "drop above threshold",
			currentCount:  800,
			previousCount: 1000,
			wantWarning:   true, // 20% drop
			wantContains:  "20% fewer files",
		},
		{
			name:          "significant drop",
			currentCount:  500,
			previousCount: 1000,
			wantWarning:   true, // 50% drop
			wantContains:  "50% fewer files",
		},
		{
			name:          "previous count zero - no warning",
			currentCount:  100,
			previousCount: 0,
			wantWarning:   false,
		},
		{
			name:          "increase in files - no warning",
			currentCount:  1200,
			previousCount: 1000,
			wantWarning:   false, // Negative drop = increase
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warning := DetectFileDrop(tt.currentCount, tt.previousCount)

			if tt.wantWarning && warning == "" {
				t.Error("expected warning but got none")
			}
			if !tt.wantWarning && warning != "" {
				t.Errorf("expected no warning but got: %s", warning)
			}
			if tt.wantContains != "" && !containsSubstring(warning, tt.wantContains) {
				t.Errorf("expected warning to contain %q, got: %s", tt.wantContains, warning)
			}
		})
	}
}

// =============================================================================
// Tests for DetectRepeatedErrors
// =============================================================================

func TestDetectRepeatedErrors(t *testing.T) {
	tests := []struct {
		name        string
		stats       *filesystem.WalkStats
		prevJob     *scanner.ScanJob
		wantWarning bool
	}{
		{
			name:        "nil stats - no warning",
			stats:       nil,
			prevJob:     &scanner.ScanJob{DirsSkipped: 5},
			wantWarning: false,
		},
		{
			name:        "current has no skipped dirs - no warning",
			stats:       &filesystem.WalkStats{DirsSkipped: 0},
			prevJob:     &scanner.ScanJob{DirsSkipped: 5},
			wantWarning: false,
		},
		{
			name:        "previous had no skipped dirs - no warning",
			stats:       &filesystem.WalkStats{DirsSkipped: 3},
			prevJob:     &scanner.ScanJob{DirsSkipped: 0},
			wantWarning: false,
		},
		{
			name:        "both have skipped dirs - warning",
			stats:       &filesystem.WalkStats{DirsSkipped: 3},
			prevJob:     &scanner.ScanJob{DirsSkipped: 5},
			wantWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warning := DetectRepeatedErrors(tt.stats, tt.prevJob)

			if tt.wantWarning && warning == "" {
				t.Error("expected warning but got none")
			}
			if !tt.wantWarning && warning != "" {
				t.Errorf("expected no warning but got: %s", warning)
			}
		})
	}
}

// =============================================================================
// Tests for ValidateDiscovery
// =============================================================================

func TestValidateDiscovery(t *testing.T) {
	tests := []struct {
		name             string
		filesDiscovered  int64
		stats            *filesystem.WalkStats
		setupMocks       func(*mocks.ScanJobRepository)
		expectedWarnings int
	}{
		{
			name:            "no errors no previous scans",
			filesDiscovered: 100,
			stats:           &filesystem.WalkStats{FilesDiscovered: 100, DirsScanned: 10},
			setupMocks: func(repo *mocks.ScanJobRepository) {
				// No previous jobs
			},
			expectedWarnings: 0,
		},
		{
			name:            "walk stats errors generate warnings",
			filesDiscovered: 100,
			stats: &filesystem.WalkStats{
				FilesDiscovered: 100,
				DirsSkipped:     5,
				FilesSkipped:    10,
			},
			setupMocks:       func(repo *mocks.ScanJobRepository) {},
			expectedWarnings: 2, // dirs skipped + files skipped
		},
		{
			name:            "significant drop from previous scan",
			filesDiscovered: 50,
			stats:           &filesystem.WalkStats{FilesDiscovered: 50},
			setupMocks: func(repo *mocks.ScanJobRepository) {
				now := time.Now()
				repo.WithJobs(
					&scanner.ScanJob{
						ID:        1,
						LibraryID: 1,
						Status:    scanner.ScanStatusRunning, // Current job
						CreatedAt: now,
						UpdatedAt: now,
					},
					&scanner.ScanJob{
						ID:         2,
						LibraryID:  1,
						Status:     scanner.ScanStatusCompleted,
						FilesFound: 100, // 50% drop
						CreatedAt:  now.Add(-1 * time.Hour),
						UpdatedAt:  now.Add(-1 * time.Hour),
					},
				)
			},
			expectedWarnings: 1, // File drop warning
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobRepo := mocks.NewScanJobRepository(t)
			if tt.setupMocks != nil {
				tt.setupMocks(jobRepo)
			}

			deps := &Deps{
				ScanRepos: &scan.ScanRepositories{
					ScanJob: jobRepo,
				},
				Logger: testDiscoveryLogger(),
			}

			warnings := ValidateDiscovery(context.Background(), deps, 1, tt.filesDiscovered, tt.stats)

			if len(warnings) != tt.expectedWarnings {
				t.Errorf("expected %d warnings, got %d: %v", tt.expectedWarnings, len(warnings), warnings)
			}
		})
	}
}

func TestCheckAgainstPreviousScan(t *testing.T) {
	tests := []struct {
		name             string
		filesDiscovered  int64
		stats            *filesystem.WalkStats
		setupMocks       func(*mocks.ScanJobRepository)
		expectedWarnings int
	}{
		{
			name:            "no previous jobs",
			filesDiscovered: 100,
			stats:           &filesystem.WalkStats{FilesDiscovered: 100},
			setupMocks:      func(repo *mocks.ScanJobRepository) {},
			expectedWarnings: 0,
		},
		{
			name:            "only one job (current)",
			filesDiscovered: 100,
			stats:           &filesystem.WalkStats{FilesDiscovered: 100},
			setupMocks: func(repo *mocks.ScanJobRepository) {
				now := time.Now()
				repo.WithJobs(&scanner.ScanJob{
					ID:        1,
					LibraryID: 1,
					Status:    scanner.ScanStatusRunning,
					CreatedAt: now,
					UpdatedAt: now,
				})
			},
			expectedWarnings: 0,
		},
		{
			name:            "previous job not completed",
			filesDiscovered: 50,
			stats:           &filesystem.WalkStats{FilesDiscovered: 50},
			setupMocks: func(repo *mocks.ScanJobRepository) {
				now := time.Now()
				repo.WithJobs(
					&scanner.ScanJob{
						ID:        1,
						LibraryID: 1,
						Status:    scanner.ScanStatusRunning,
						CreatedAt: now,
						UpdatedAt: now,
					},
					&scanner.ScanJob{
						ID:         2,
						LibraryID:  1,
						Status:     scanner.ScanStatusFailed, // Not completed
						FilesFound: 100,
						CreatedAt:  now.Add(-1 * time.Hour),
						UpdatedAt:  now.Add(-1 * time.Hour),
					},
				)
			},
			expectedWarnings: 0, // Skipped because previous not completed
		},
		{
			name:            "previous job has zero files",
			filesDiscovered: 50,
			stats:           &filesystem.WalkStats{FilesDiscovered: 50},
			setupMocks: func(repo *mocks.ScanJobRepository) {
				now := time.Now()
				repo.WithJobs(
					&scanner.ScanJob{
						ID:        1,
						LibraryID: 1,
						Status:    scanner.ScanStatusRunning,
						CreatedAt: now,
						UpdatedAt: now,
					},
					&scanner.ScanJob{
						ID:         2,
						LibraryID:  1,
						Status:     scanner.ScanStatusCompleted,
						FilesFound: 0, // Zero files
						CreatedAt:  now.Add(-1 * time.Hour),
						UpdatedAt:  now.Add(-1 * time.Hour),
					},
				)
			},
			expectedWarnings: 0, // Skipped because previous has zero files
		},
		{
			name:            "repeated directory errors",
			filesDiscovered: 100,
			stats:           &filesystem.WalkStats{FilesDiscovered: 100, DirsSkipped: 5},
			setupMocks: func(repo *mocks.ScanJobRepository) {
				now := time.Now()
				repo.WithJobs(
					&scanner.ScanJob{
						ID:        1,
						LibraryID: 1,
						Status:    scanner.ScanStatusRunning,
						CreatedAt: now,
						UpdatedAt: now,
					},
					&scanner.ScanJob{
						ID:          2,
						LibraryID:   1,
						Status:      scanner.ScanStatusCompleted,
						FilesFound:  100,
						DirsSkipped: 3, // Previous also had skipped dirs
						CreatedAt:   now.Add(-1 * time.Hour),
						UpdatedAt:   now.Add(-1 * time.Hour),
					},
				)
			},
			expectedWarnings: 1, // Repeated errors warning
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobRepo := mocks.NewScanJobRepository(t)
			if tt.setupMocks != nil {
				tt.setupMocks(jobRepo)
			}

			deps := &Deps{
				ScanRepos: &scan.ScanRepositories{
					ScanJob: jobRepo,
				},
				Logger: testDiscoveryLogger(),
			}

			warnings := checkAgainstPreviousScan(context.Background(), deps, 1, tt.filesDiscovered, tt.stats)

			if len(warnings) != tt.expectedWarnings {
				t.Errorf("expected %d warnings, got %d: %v", tt.expectedWarnings, len(warnings), warnings)
			}
		})
	}
}

// =============================================================================
// Tests for NewContext
// =============================================================================

func TestNewContext(t *testing.T) {
	lib := &library.Library{ID: 1, Name: "Test", Path: "/test"}
	job := &scanner.ScanJob{ID: 100, LibraryID: 1}

	ctx := NewContext(100, lib, job, nil)

	if ctx.JobID != 100 {
		t.Errorf("JobID = %d, want 100", ctx.JobID)
	}
	if ctx.Lib != lib {
		t.Error("Lib should be set")
	}
	if ctx.CurrentJob != job {
		t.Error("CurrentJob should be set")
	}
	if ctx.Walker != nil {
		t.Error("Walker should be nil when not provided")
	}
	if ctx.DiscoveryStats != nil {
		t.Error("DiscoveryStats should be nil initially")
	}
}

// =============================================================================
// Helper
// =============================================================================

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
