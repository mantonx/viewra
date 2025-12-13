package library

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/infrastructure/system"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestScanLibraryUseCase_isMediaFile(t *testing.T) {
	uc := &ScanLibraryUseCase{}

	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		// Video formats
		{"mp4 video", "mp4", true},
		{"mkv video", "mkv", true},
		{"avi video", "avi", true},
		{"mov video", "mov", true},
		{"wmv video", "wmv", true},
		{"flv video", "flv", true},
		{"webm video", "webm", true},
		{"m4v video", "m4v", true},
		{"mpg video", "mpg", true},
		{"mpeg video", "mpeg", true},
		{"m2ts video", "m2ts", true},
		{"ts video", "ts", true},
		{"vob video", "vob", true},
		{"3gp video", "3gp", true},
		{"mxf video", "mxf", true},

		// Audio formats
		{"mp3 audio", "mp3", true},
		{"flac audio", "flac", true},
		{"m4a audio", "m4a", true},
		{"aac audio", "aac", true},
		{"ogg audio", "ogg", true},
		{"opus audio", "opus", true},
		{"wav audio", "wav", true},
		{"wma audio", "wma", true},

		// Case insensitivity
		{"uppercase MP4", "MP4", true},
		{"mixed case MkV", "MkV", true},

		// With leading dot
		{"with dot .mp4", ".mp4", true},
		{"with dot .mkv", ".mkv", true},

		// Non-media files
		{"text file", "txt", false},
		{"image png", "png", false},
		{"image jpg", "jpg", false},
		{"subtitle srt", "srt", false},
		{"subtitle ass", "ass", false},
		{"nfo file", "nfo", false},
		{"json file", "json", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uc.isMediaFile(tt.ext)
			if result != tt.expected {
				t.Errorf("isMediaFile(%q) = %v, want %v", tt.ext, result, tt.expected)
			}
		})
	}
}

func TestScanLibraryUseCase_calculateProcessingTimeout(t *testing.T) {
	tests := []struct {
		name              string
		fileSize          int64
		isRemote          bool
		baseFileTimeout   time.Duration
		remoteTimeout     time.Duration
		maxExtraTimeout   time.Duration
		expectedMinimum   time.Duration
		expectedMaximum   time.Duration
	}{
		{
			name:            "small local file",
			fileSize:        100 * 1024 * 1024, // 100MB
			isRemote:        false,
			baseFileTimeout: 30 * time.Second,
			remoteTimeout:   60 * time.Second,
			maxExtraTimeout: 120 * time.Second,
			expectedMinimum: 30 * time.Second,
			expectedMaximum: 30 * time.Second,
		},
		{
			name:            "small remote file",
			fileSize:        100 * 1024 * 1024, // 100MB
			isRemote:        true,
			baseFileTimeout: 30 * time.Second,
			remoteTimeout:   60 * time.Second,
			maxExtraTimeout: 120 * time.Second,
			expectedMinimum: 60 * time.Second,
			expectedMaximum: 60 * time.Second,
		},
		{
			name:            "1GB local file",
			fileSize:        1024 * 1024 * 1024, // 1GB
			isRemote:        false,
			baseFileTimeout: 30 * time.Second,
			remoteTimeout:   60 * time.Second,
			maxExtraTimeout: 120 * time.Second,
			expectedMinimum: 31 * time.Second, // 30s base + 1s per GB
			expectedMaximum: 31 * time.Second,
		},
		{
			name:            "50GB local file adds extra time",
			fileSize:        50 * 1024 * 1024 * 1024, // 50GB
			isRemote:        false,
			baseFileTimeout: 30 * time.Second,
			remoteTimeout:   60 * time.Second,
			maxExtraTimeout: 120 * time.Second,
			expectedMinimum: 80 * time.Second, // 30s base + 50s (1s per GB)
			expectedMaximum: 80 * time.Second,
		},
		{
			name:            "150GB local file hits max extra",
			fileSize:        150 * 1024 * 1024 * 1024, // 150GB - larger than maxExtraTimeout
			isRemote:        false,
			baseFileTimeout: 30 * time.Second,
			remoteTimeout:   60 * time.Second,
			maxExtraTimeout: 120 * time.Second,
			expectedMinimum: 150 * time.Second, // 30s base + 120s max extra (capped)
			expectedMaximum: 150 * time.Second,
		},
		{
			name:            "50GB remote file",
			fileSize:        50 * 1024 * 1024 * 1024, // 50GB
			isRemote:        true,
			baseFileTimeout: 30 * time.Second,
			remoteTimeout:   60 * time.Second,
			maxExtraTimeout: 120 * time.Second,
			expectedMinimum: 110 * time.Second, // 60s base + 50s (1s per GB)
			expectedMaximum: 110 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var profile *system.Profile
			if tt.isRemote {
				profile = &system.Profile{
					Storage: system.StorageProfile{
						IsRemote: true,
					},
				}
			}

			uc := &ScanLibraryUseCase{
				config: scan.Config{
					BaseFileTimeout:      tt.baseFileTimeout,
					RemoteStorageTimeout: tt.remoteTimeout,
					MaxExtraTimeout:      tt.maxExtraTimeout,
				},
				systemProfile: profile,
			}

			result := uc.calculateProcessingTimeout(tt.fileSize)
			if result < tt.expectedMinimum || result > tt.expectedMaximum {
				t.Errorf("calculateProcessingTimeout(%d) = %v, want between %v and %v",
					tt.fileSize, result, tt.expectedMinimum, tt.expectedMaximum)
			}
		})
	}
}

func TestScanLibraryUseCase_statWithTimeout(t *testing.T) {
	// Create a temp file for testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	uc := &ScanLibraryUseCase{}

	t.Run("successful stat", func(t *testing.T) {
		ctx := context.Background()
		info, err := uc.statWithTimeout(ctx, tmpFile, 5*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if info == nil {
			t.Error("expected FileInfo, got nil")
		}
		if info != nil && info.Size() != 12 {
			t.Errorf("Size() = %d, want 12", info.Size())
		}
	})

	t.Run("file not found", func(t *testing.T) {
		ctx := context.Background()
		_, err := uc.statWithTimeout(ctx, "/nonexistent/path/file.txt", 5*time.Second)
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		_, err := uc.statWithTimeout(ctx, tmpFile, 5*time.Second)
		// Either context.Canceled error or successful completion due to race
		// is acceptable - the stat goroutine might finish before cancellation is detected
		_ = err // No assertion needed - just verify no panic
	})
}

func TestScanLibraryUseCase_validateDiscovery(t *testing.T) {
	tests := []struct {
		name             string
		filesDiscovered  int64
		stats            *filesystem.WalkStats
		previousJobs     []*scanner.ScanJob
		expectedWarnings int
	}{
		{
			name:             "clean discovery no issues",
			filesDiscovered:  1000,
			stats:            &filesystem.WalkStats{FilesDiscovered: 1000, DirsScanned: 50},
			previousJobs:     nil,
			expectedWarnings: 0,
		},
		{
			name:            "discovery with skipped dirs",
			filesDiscovered: 950,
			stats: &filesystem.WalkStats{
				FilesDiscovered: 950,
				DirsScanned:     50,
				DirsSkipped:     5,
			},
			previousJobs:     nil,
			expectedWarnings: 1,
		},
		{
			name:            "discovery with skipped files",
			filesDiscovered: 950,
			stats: &filesystem.WalkStats{
				FilesDiscovered: 950,
				DirsScanned:     50,
				FilesSkipped:    10,
			},
			previousJobs:     nil,
			expectedWarnings: 1,
		},
		{
			name:            "discovery with permission errors",
			filesDiscovered: 950,
			stats: &filesystem.WalkStats{
				FilesDiscovered:  950,
				DirsScanned:      50,
				DirsSkipped:      1, // HasErrors() requires DirsSkipped > 0 or FilesSkipped > 0
				PermissionErrors: 15,
			},
			previousJobs:     nil,
			expectedWarnings: 2, // DirsSkipped warning + PermissionErrors warning
		},
		{
			name:            "discovery with network errors",
			filesDiscovered: 950,
			stats: &filesystem.WalkStats{
				FilesDiscovered: 950,
				DirsScanned:     50,
				FilesSkipped:    1, // HasErrors() requires DirsSkipped > 0 or FilesSkipped > 0
				NetworkErrors:   5,
			},
			previousJobs:     nil,
			expectedWarnings: 2, // FilesSkipped warning + NetworkErrors warning
		},
		{
			name:            "significant file count drop",
			filesDiscovered: 500,
			stats:           &filesystem.WalkStats{FilesDiscovered: 500, DirsScanned: 50},
			previousJobs: []*scanner.ScanJob{
				{ID: 2, LibraryID: 1, Status: scanner.ScanStatusRunning, FilesFound: 0},   // Current (running) - needed because ListByLibrary returns all
				{ID: 1, LibraryID: 1, Status: scanner.ScanStatusCompleted, FilesFound: 1000}, // Previous completed scan
			},
			expectedWarnings: 1, // 50% drop
		},
		{
			name:            "repeated discovery errors",
			filesDiscovered: 900,
			stats: &filesystem.WalkStats{
				FilesDiscovered: 900,
				DirsScanned:     50,
				DirsSkipped:     5,
			},
			previousJobs: []*scanner.ScanJob{
				{ID: 2, LibraryID: 1, Status: scanner.ScanStatusRunning, FilesFound: 0},       // Current (running)
				{ID: 1, LibraryID: 1, Status: scanner.ScanStatusCompleted, FilesFound: 950, DirsSkipped: 3}, // Previous completed
			},
			expectedWarnings: 2, // dirs skipped + repeated errors
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanJobRepo := mocks.NewScanJobRepository(t)
			if tt.previousJobs != nil {
				scanJobRepo.WithJobs(tt.previousJobs...)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &scan.ScanRepositories{
					ScanJob: scanJobRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			ctx := context.Background()
			warnings := uc.validateDiscovery(ctx, 1, tt.filesDiscovered, tt.stats)

			if len(warnings) != tt.expectedWarnings {
				t.Errorf("validateDiscovery() returned %d warnings, want %d", len(warnings), tt.expectedWarnings)
				for i, w := range warnings {
					t.Logf("  warning[%d]: %s", i, w)
				}
			}
		})
	}
}

// Note: Tests for scanutil.IsExtra, scanutil.IsMediaFile, scanutil.IsAudioFile,
// and scanmedia.IsConstraintError live in their respective sub-packages:
// - scan/scanutil/utils_test.go
// - scan/media/upsert_test.go

func TestProgressUpdate_Builder(t *testing.T) {
	t.Run("builds progress with all fields", func(t *testing.T) {
		// Create mock repository
		mockScanJob := mocks.NewScanJobRepository(t)
		// Pre-populate with a job so UpdateProgress finds it
		mockScanJob.WithJobs(&scanner.ScanJob{ID: 123})

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{ScanJob: mockScanJob},
			logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		err := uc.NewProgressUpdate(123).
			Phase(scanner.ScanPhaseProcessing).
			FilesFound(100).
			FilesProcessed(50).
			Errors(5).
			Warnings(10).
			EstimatedTotal(200).
			DiscoveryDone().
			Update(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify the job was updated
		job, _ := mockScanJob.GetByID(context.Background(), 123)
		if job.Phase != scanner.ScanPhaseProcessing {
			t.Errorf("expected phase %v, got %v", scanner.ScanPhaseProcessing, job.Phase)
		}
		if job.FilesFound != 100 {
			t.Errorf("expected FilesFound 100, got %d", job.FilesFound)
		}
		if job.FilesProcessed != 50 {
			t.Errorf("expected FilesProcessed 50, got %d", job.FilesProcessed)
		}
		if job.ErrorCount != 5 {
			t.Errorf("expected ErrorCount 5, got %d", job.ErrorCount)
		}
		if job.WarningCount != 10 {
			t.Errorf("expected WarningCount 10, got %d", job.WarningCount)
		}
		if job.EstimatedTotal != 200 {
			t.Errorf("expected EstimatedTotal 200, got %d", job.EstimatedTotal)
		}
		if !job.DiscoveryDone {
			t.Error("expected DiscoveryDone to be true")
		}
	})

	t.Run("FromCheckpointStats copies stats", func(t *testing.T) {
		mockScanJob := mocks.NewScanJobRepository(t)
		mockScanJob.WithJobs(&scanner.ScanJob{ID: 123})

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{ScanJob: mockScanJob},
			logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		stats := &scanner.CheckpointStats{
			ProcessedFiles: 75,
			FailedFiles:    3,
			WarningFiles:   7,
		}

		err := uc.NewProgressUpdate(123).
			FromCheckpointStats(stats).
			Update(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		job, _ := mockScanJob.GetByID(context.Background(), 123)
		if job.FilesProcessed != 75 {
			t.Errorf("expected FilesProcessed 75, got %d", job.FilesProcessed)
		}
		if job.ErrorCount != 3 {
			t.Errorf("expected ErrorCount 3, got %d", job.ErrorCount)
		}
		if job.WarningCount != 7 {
			t.Errorf("expected WarningCount 7, got %d", job.WarningCount)
		}
	})

	t.Run("FromJob copies job fields", func(t *testing.T) {
		mockScanJob := mocks.NewScanJobRepository(t)
		mockScanJob.WithJobs(&scanner.ScanJob{ID: 123})

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{ScanJob: mockScanJob},
			logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		sourceJob := &scanner.ScanJob{
			FilesFound:     500,
			Phase:          scanner.ScanPhaseCompleted,
			EstimatedTotal: 600,
			DiscoveryDone:  true,
		}

		err := uc.NewProgressUpdate(123).
			FromJob(sourceJob).
			Update(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		job, _ := mockScanJob.GetByID(context.Background(), 123)
		if job.FilesFound != 500 {
			t.Errorf("expected FilesFound 500, got %d", job.FilesFound)
		}
		if job.Phase != scanner.ScanPhaseCompleted {
			t.Errorf("expected phase %v, got %v", scanner.ScanPhaseCompleted, job.Phase)
		}
		if job.EstimatedTotal != 600 {
			t.Errorf("expected EstimatedTotal 600, got %d", job.EstimatedTotal)
		}
		if !job.DiscoveryDone {
			t.Error("expected DiscoveryDone to be true")
		}
	})

	t.Run("handles nil stats gracefully", func(t *testing.T) {
		mockScanJob := mocks.NewScanJobRepository(t)
		mockScanJob.WithJobs(&scanner.ScanJob{ID: 123})

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{ScanJob: mockScanJob},
			logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		err := uc.NewProgressUpdate(123).
			FromCheckpointStats(nil).
			FilesFound(100).
			Update(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		job, _ := mockScanJob.GetByID(context.Background(), 123)
		// Should have FilesFound but zero for stats fields
		if job.FilesFound != 100 {
			t.Errorf("expected FilesFound 100, got %d", job.FilesFound)
		}
		if job.FilesProcessed != 0 {
			t.Errorf("expected FilesProcessed 0, got %d", job.FilesProcessed)
		}
	})

	t.Run("handles nil job gracefully", func(t *testing.T) {
		mockScanJob := mocks.NewScanJobRepository(t)
		mockScanJob.WithJobs(&scanner.ScanJob{ID: 123})

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{ScanJob: mockScanJob},
			logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		err := uc.NewProgressUpdate(123).
			FromJob(nil).
			FilesProcessed(50).
			Update(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		job, _ := mockScanJob.GetByID(context.Background(), 123)
		// Should have FilesProcessed but zero for job fields
		if job.FilesProcessed != 50 {
			t.Errorf("expected FilesProcessed 50, got %d", job.FilesProcessed)
		}
		if job.FilesFound != 0 {
			t.Errorf("expected FilesFound 0, got %d", job.FilesFound)
		}
	})
}

func TestCompleteJobSafely(t *testing.T) {
	t.Run("successful completion", func(t *testing.T) {
		// Pre-populate with a running job
		existingJob := &scanner.ScanJob{
			ID:     1,
			Status: scanner.ScanStatusRunning,
		}
		scanJobRepo := mocks.NewScanJobRepository(t).WithJobs(existingJob)
		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{
				ScanJob: scanJobRepo,
			},
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		job := &scanner.ScanJob{
			ID:     1,
			Status: scanner.ScanStatusCompleted,
		}

		// Should not panic and should call Complete
		uc.completeJobSafely(context.Background(), job)

		// Verify the job was updated to completed
		stored, err := scanJobRepo.GetByID(context.Background(), 1)
		if err != nil {
			t.Fatalf("expected job to be stored: %v", err)
		}
		if stored.Status != scanner.ScanStatusCompleted {
			t.Errorf("expected status %s, got %s", scanner.ScanStatusCompleted, stored.Status)
		}
	})

	t.Run("error is logged not returned", func(t *testing.T) {
		scanJobRepo := mocks.NewScanJobRepository(t)
		scanJobRepo.CompleteErr = fmt.Errorf("database error")

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{
				ScanJob: scanJobRepo,
			},
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		job := &scanner.ScanJob{
			ID:     1,
			Status: scanner.ScanStatusFailed,
		}

		// Should not panic - error is logged, not returned
		uc.completeJobSafely(context.Background(), job)
	})

	t.Run("deleted job is handled gracefully", func(t *testing.T) {
		scanJobRepo := mocks.NewScanJobRepository(t)
		// Error message matches deletedPatterns in scanner package
		scanJobRepo.CompleteErr = fmt.Errorf("foreign key constraint violation")

		uc := &ScanLibraryUseCase{
			scanRepos: &scan.ScanRepositories{
				ScanJob: scanJobRepo,
			},
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		job := &scanner.ScanJob{
			ID:     1,
			Status: scanner.ScanStatusCompleted,
		}

		// Should not panic - deleted jobs are expected
		uc.completeJobSafely(context.Background(), job)
	})
}

func TestIsScanDeleted(t *testing.T) {
	t.Run("returns true for foreign key constraint error", func(t *testing.T) {
		err := fmt.Errorf("foreign key constraint violation")
		if !scanner.IsScanJobDeleted(err) {
			t.Error("expected true for foreign key constraint error")
		}
	})

	t.Run("returns true for no rows error", func(t *testing.T) {
		err := fmt.Errorf("no rows in result set")
		if !scanner.IsScanJobDeleted(err) {
			t.Error("expected true for no rows error")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		if scanner.IsScanJobDeleted(fmt.Errorf("some other error")) {
			t.Error("expected false for other errors")
		}
	})

	t.Run("returns false for nil", func(t *testing.T) {
		if scanner.IsScanJobDeleted(nil) {
			t.Error("expected false for nil")
		}
	})
}
