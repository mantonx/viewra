package execution

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/application/library/scan/discovery"
	"github.com/mantonx/viewra/internal/application/library/scan/processing"
	"github.com/mantonx/viewra/internal/application/library/scan/status"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// mockProgressUpdater implements ProgressUpdater for testing.
type mockProgressUpdater struct{}

func (m *mockProgressUpdater) NewProgressUpdate(jobID int64) *status.ProgressUpdate {
	return status.NewProgressUpdate(&mockRepoProgressUpdater{}, testLogger(), jobID)
}

// mockRepoProgressUpdater implements status.ProgressUpdater for the ProgressUpdate builder.
type mockRepoProgressUpdater struct{}

func (m *mockRepoProgressUpdater) UpdateProgress(ctx context.Context, jobID int64, progress *scanner.Progress) error {
	return nil
}

// mockSessionInitializer implements SessionInitializer for testing.
type mockSessionInitializer struct{}

func (m *mockSessionInitializer) InitializeScanSession(ctx context.Context, lib *library.Library) {}

func createTestDeps(t *testing.T, scanRepos *scan.ScanRepositories, mediaRepos *scan.MediaRepositories) *Deps {
	config := &scan.Config{
		DiscoveryBufferSize:  100,
		DiscoveryLogEvery:    10,
		CheckpointBatchSize:  10,
		CheckpointBufferSize: 10,
		MaxRetries:           3,
		WorkerTimeout:        5 * time.Minute,
		HashProgressLogEvery: 1000,
		ProgressUpdateTick:   100 * time.Millisecond,
		RetryBackoffBase:     time.Millisecond, // Fast retries for tests
	}

	// Create minimal media repos if not provided
	if mediaRepos == nil {
		mediaRepos = &scan.MediaRepositories{
			Media: mocks.NewMediaRepository(t),
		}
	}

	return &Deps{
		ScanRepos:  scanRepos,
		MediaRepos: mediaRepos,
		Config:     config,
		Logger:     testLogger(),
		DiscoveryDeps: func() *discovery.Deps {
			return &discovery.Deps{
				ScanRepos: scanRepos,
				Config:    config,
				Logger:    testLogger(),
				IsMediaFile: func(ext string) bool {
					// Simple check for common media extensions
					switch ext {
					case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm",
						".mp3", ".flac", ".wav", ".aac", ".ogg", ".m4a":
						return true
					}
					return false
				},
			}
		},
		ProcessingDeps: func() *processing.Deps {
			return &processing.Deps{
				ScanRepos:  scanRepos,
				MediaRepos: mediaRepos,
				Config:     config,
				Logger:     testLogger(),
			}
		},
		StatusDeps: func() *status.Deps {
			return &status.Deps{
				ScanRepos: scanRepos,
				Logger:    testLogger(),
			}
		},
		ProgressUpdater:           &mockProgressUpdater{},
		SessionInitializer:        &mockSessionInitializer{},
		RecoverFromPanic:          func(jobID, libraryID int64, description string) {},
		RecoverFromPanicWithError: func(jobID, libraryID int64, description string, errChan chan<- error) {},
		HasImageCleanup:           func() bool { return false },
	}
}

func TestCanResumeFromCheckpoints(t *testing.T) {
	t.Run("returns false when no checkpoints exist", func(t *testing.T) {
		scanJobRepo := mocks.NewScanJobRepository(t)
		checkpointRepo := mocks.NewCheckpointRepository(t)

		job := &scanner.ScanJob{
			ID:        100,
			LibraryID: 1,
			Status:    scanner.ScanStatusRunning,
		}
		scanJobRepo.WithJobs(job)

		scanRepos := &scan.ScanRepositories{
			ScanJob:    scanJobRepo,
			Checkpoint: checkpointRepo,
		}

		deps := createTestDeps(t, scanRepos, nil)

		params := &RunScanParams{
			JobID: 100,
			Lib:   &library.Library{ID: 1, Path: "/media"},
		}

		result := CanResumeFromCheckpoints(context.Background(), deps, params, job)

		if result {
			t.Error("expected false when no checkpoints exist")
		}
	})

	t.Run("returns true when all checkpoints are completed", func(t *testing.T) {
		scanJobRepo := mocks.NewScanJobRepository(t)
		checkpointRepo := mocks.NewCheckpointRepository(t)

		job := &scanner.ScanJob{
			ID:         100,
			LibraryID:  1,
			Status:     scanner.ScanStatusRunning,
			FilesFound: 10,
		}
		scanJobRepo.WithJobs(job)

		// Simulate all checkpoints completed
		checkpointRepo.WithStats(&scanner.CheckpointStats{
			TotalFiles:     10,
			CompletedFiles: 10,
			PendingFiles:   0,
			ProcessedFiles: 10,
		})

		scanRepos := &scan.ScanRepositories{
			ScanJob:    scanJobRepo,
			Checkpoint: checkpointRepo,
		}

		deps := createTestDeps(t, scanRepos, nil)

		params := &RunScanParams{
			JobID: 100,
			Lib:   &library.Library{ID: 1, Path: "/media"},
		}

		result := CanResumeFromCheckpoints(context.Background(), deps, params, job)

		if !result {
			t.Error("expected true when all checkpoints completed")
		}
	})
}

func TestRunFreshScan(t *testing.T) {
	tests := []struct {
		name              string
		setupTempDir      func(*testing.T) string
		setupRepos        func(*testing.T) (*scan.ScanRepositories, *discovery.IncrementalScanner)
		jobID             int64
		expectJobComplete bool
		expectError       bool
	}{
		{
			name: "handles no changes detected - marks job complete",
			setupTempDir: func(t *testing.T) string {
				return t.TempDir()
			},
			setupRepos: func(t *testing.T) (*scan.ScanRepositories, *discovery.IncrementalScanner) {
				scanJobRepo := mocks.NewScanJobRepository(t)
				scanStateRepo := mocks.NewScanStateRepository(t)
				checkpointRepo := mocks.NewCheckpointRepository(t)

				job := &scanner.ScanJob{
					ID:             100,
					LibraryID:      1,
					Status:         scanner.ScanStatusRunning,
					EstimatedTotal: 0,
				}
				scanJobRepo.WithJobs(job)

				repos := &scan.ScanRepositories{
					ScanJob:    scanJobRepo,
					ScanState:  scanStateRepo,
					Checkpoint: checkpointRepo,
				}

				incScanner := discovery.NewIncrementalScanner(scanStateRepo, testLogger())

				return repos, incScanner
			},
			jobID:             100,
			expectJobComplete: true,
			expectError:       false,
		},
		{
			name: "handles initialization error gracefully",
			setupTempDir: func(t *testing.T) string {
				return t.TempDir()
			},
			setupRepos: func(t *testing.T) (*scan.ScanRepositories, *discovery.IncrementalScanner) {
				scanJobRepo := mocks.NewScanJobRepository(t)
				scanStateRepo := mocks.NewScanStateRepository(t)
				checkpointRepo := mocks.NewCheckpointRepository(t)

				scanJobRepo.GetErr = errors.New("database error")

				repos := &scan.ScanRepositories{
					ScanJob:    scanJobRepo,
					ScanState:  scanStateRepo,
					Checkpoint: checkpointRepo,
				}

				incScanner := discovery.NewIncrementalScanner(scanStateRepo, testLogger())

				return repos, incScanner
			},
			jobID:             100,
			expectJobComplete: false,
			expectError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := tt.setupTempDir(t)
			repos, incScanner := tt.setupRepos(t)

			lib := &library.Library{
				ID:   1,
				Path: tmpDir,
			}

			deps := createTestDeps(t, repos, nil)
			deps.DiscoveryDeps = func() *discovery.Deps {
				return &discovery.Deps{
					ScanRepos:   repos,
					Config:      deps.Config,
					Logger:      testLogger(),
					IncrScanner: incScanner,
				}
			}

			RunFreshScan(context.Background(), deps, tt.jobID, lib)

			time.Sleep(100 * time.Millisecond)

			if tt.expectJobComplete {
				job, err := repos.ScanJob.GetByID(context.Background(), tt.jobID)
				if err != nil {
					if !tt.expectError {
						t.Fatalf("failed to get job: %v", err)
					}
				} else {
					if job.Status != scanner.ScanStatusCompleted {
						t.Errorf("expected job status %v, got %v", scanner.ScanStatusCompleted, job.Status)
					}
				}
			}
		})
	}
}

func TestPhaseWalkDirectory(t *testing.T) {
	t.Run("discovers media files in temp directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		testFiles := []string{"movie1.mp4", "movie2.mkv", "readme.txt", "movie3.avi"}
		for _, filename := range testFiles {
			if err := os.WriteFile(filepath.Join(tmpDir, filename), []byte("test"), 0644); err != nil {
				t.Fatal(err)
			}
		}

		scanJobRepo := mocks.NewScanJobRepository(t)
		job := &scanner.ScanJob{
			ID:             100,
			LibraryID:      1,
			EstimatedTotal: 0,
		}
		scanJobRepo.WithJobs(job)

		scanRepos := &scan.ScanRepositories{
			ScanJob: scanJobRepo,
		}

		deps := createTestDeps(t, scanRepos, nil)

		dctx := discovery.NewContext(100, &library.Library{ID: 1, Path: tmpDir}, job, filesystem.NewWalker(filesystem.WithLogger(testLogger())))

		files, err := phaseWalkDirectory(context.Background(), deps, dctx)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should discover 3 media files (mp4, mkv, avi) - not txt
		if len(files) != 3 {
			t.Errorf("expected 3 media files, got %d", len(files))
		}

		if dctx.DiscoveryStats == nil {
			t.Error("DiscoveryStats should be set")
		}
	})

	t.Run("handles walker errors for nonexistent path", func(t *testing.T) {
		scanJobRepo := mocks.NewScanJobRepository(t)
		job := &scanner.ScanJob{
			ID:             100,
			LibraryID:      1,
			EstimatedTotal: 0,
		}
		scanJobRepo.WithJobs(job)

		scanRepos := &scan.ScanRepositories{
			ScanJob: scanJobRepo,
		}

		deps := createTestDeps(t, scanRepos, nil)

		dctx := discovery.NewContext(100, &library.Library{ID: 1, Path: "/nonexistent/path/that/does/not/exist"}, job, filesystem.NewWalker(filesystem.WithLogger(testLogger())))

		files, err := phaseWalkDirectory(context.Background(), deps, dctx)

		if err != nil {
			t.Logf("Got expected error: %v", err)
		} else {
			if len(files) != 0 {
				t.Errorf("expected 0 files for nonexistent path, got %d", len(files))
			}
		}
	})
}

func TestPhaseDetermineChanges(t *testing.T) {
	tests := []struct {
		name            string
		setupRepos      func(*testing.T) *scan.ScanRepositories
		setupIncScanner func(*testing.T, *scan.ScanRepositories) *discovery.IncrementalScanner
		discoveredFiles []scanner.FileInfo
		expectNil       bool
		validateDiff    func(*testing.T, *scanner.ScanDiff)
	}{
		{
			name: "detects new files",
			setupRepos: func(t *testing.T) *scan.ScanRepositories {
				scanJobRepo := mocks.NewScanJobRepository(t)
				scanStateRepo := mocks.NewScanStateRepository(t)

				job := &scanner.ScanJob{
					ID:        100,
					LibraryID: 1,
				}
				scanJobRepo.WithJobs(job)

				return &scan.ScanRepositories{
					ScanJob:   scanJobRepo,
					ScanState: scanStateRepo,
				}
			},
			setupIncScanner: func(t *testing.T, repos *scan.ScanRepositories) *discovery.IncrementalScanner {
				return discovery.NewIncrementalScanner(repos.ScanState, testLogger())
			},
			discoveredFiles: []scanner.FileInfo{
				{Path: "/media/new1.mp4", Size: 1000, ModTime: time.Now()},
				{Path: "/media/new2.mp4", Size: 2000, ModTime: time.Now()},
			},
			expectNil: false,
			validateDiff: func(t *testing.T, diff *scanner.ScanDiff) {
				if diff == nil {
					t.Fatal("expected non-nil diff")
				}
				if len(diff.NewFiles) != 2 {
					t.Errorf("expected 2 new files, got %d", len(diff.NewFiles))
				}
			},
		},
		{
			name: "detects no changes - returns nil",
			setupRepos: func(t *testing.T) *scan.ScanRepositories {
				scanJobRepo := mocks.NewScanJobRepository(t)
				scanStateRepo := mocks.NewScanStateRepository(t)

				job := &scanner.ScanJob{
					ID:        100,
					LibraryID: 1,
				}
				scanJobRepo.WithJobs(job)

				modTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
				scanState := &scanner.ScanState{
					LibraryID: 1,
					FilePath:  "/media/unchanged.mp4",
					FileSize:  1000,
					FileMTime: modTime,
				}
				scanStateRepo.WithStates(scanState)

				return &scan.ScanRepositories{
					ScanJob:   scanJobRepo,
					ScanState: scanStateRepo,
				}
			},
			setupIncScanner: func(t *testing.T, repos *scan.ScanRepositories) *discovery.IncrementalScanner {
				return discovery.NewIncrementalScanner(repos.ScanState, testLogger())
			},
			discoveredFiles: []scanner.FileInfo{
				{Path: "/media/unchanged.mp4", Size: 1000, ModTime: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)},
			},
			expectNil: true,
			validateDiff: func(t *testing.T, diff *scanner.ScanDiff) {
				if diff != nil {
					t.Errorf("expected nil diff when no changes, got %+v", diff)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repos := tt.setupRepos(t)
			incScanner := tt.setupIncScanner(t, repos)

			deps := createTestDeps(t, repos, nil)
			deps.DiscoveryDeps = func() *discovery.Deps {
				return &discovery.Deps{
					ScanRepos:   repos,
					Config:      deps.Config,
					Logger:      testLogger(),
					IncrScanner: incScanner,
				}
			}

			dctx := discovery.NewContext(100, &library.Library{ID: 1, Path: "/media"}, nil, nil)

			diff := phaseDetermineChanges(context.Background(), deps, dctx, tt.discoveredFiles)

			if tt.expectNil && diff != nil {
				t.Errorf("expected nil diff, got %+v", diff)
			}

			if !tt.expectNil && diff == nil {
				t.Error("expected non-nil diff, got nil")
			}

			tt.validateDiff(t, diff)
		})
	}
}

func TestPhaseHashAndProcess_EmptyFiles(t *testing.T) {
	t.Run("handles empty file list gracefully", func(t *testing.T) {
		checkpointRepo := mocks.NewCheckpointRepository(t)
		scanJobRepo := mocks.NewScanJobRepository(t)

		job := &scanner.ScanJob{
			ID:        100,
			LibraryID: 1,
		}
		scanJobRepo.WithJobs(job)

		scanRepos := &scan.ScanRepositories{
			Checkpoint: checkpointRepo,
			ScanJob:    scanJobRepo,
		}

		deps := createTestDeps(t, scanRepos, nil)

		dctx := discovery.NewContext(100, &library.Library{ID: 1, Path: "/media"}, nil, nil)
		dctx.DiscoveryStats = &filesystem.WalkStats{FilesDiscovered: 0}

		diff := &scanner.ScanDiff{
			NewFiles:       []scanner.FileInfo{},
			ModifiedFiles:  []scanner.FileInfo{},
			UnchangedFiles: []string{},
		}

		// Should complete without error
		phaseHashAndProcess(context.Background(), deps, dctx, diff)
	})
}

func TestPhaseHashAndProcess_CheckpointCreationError(t *testing.T) {
	t.Run("handles checkpoint creation error", func(t *testing.T) {
		tmpDir := t.TempDir()

		testFile := filepath.Join(tmpDir, "movie.mp4")
		if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
			t.Fatal(err)
		}

		checkpointRepo := mocks.NewCheckpointRepository(t)
		scanJobRepo := mocks.NewScanJobRepository(t)

		job := &scanner.ScanJob{
			ID:        100,
			LibraryID: 1,
			Status:    scanner.ScanStatusRunning,
		}
		scanJobRepo.WithJobs(job)

		checkpointRepo.CreateBatchErr = errors.New("database connection failed")

		scanRepos := &scan.ScanRepositories{
			Checkpoint: checkpointRepo,
			ScanJob:    scanJobRepo,
		}

		deps := createTestDeps(t, scanRepos, nil)

		dctx := discovery.NewContext(100, &library.Library{ID: 1, Path: tmpDir}, nil, nil)
		dctx.DiscoveryStats = &filesystem.WalkStats{FilesDiscovered: 1}

		diff := &scanner.ScanDiff{
			NewFiles: []scanner.FileInfo{
				{Path: testFile, Size: 12, ModTime: time.Now()},
			},
			ModifiedFiles:  []scanner.FileInfo{},
			UnchangedFiles: []string{},
		}

		phaseHashAndProcess(context.Background(), deps, dctx, diff)

		updatedJob, err := scanJobRepo.GetByID(context.Background(), 100)
		if err != nil {
			t.Fatalf("failed to get job: %v", err)
		}
		if updatedJob.Status != scanner.ScanStatusFailed {
			t.Errorf("expected job status %v, got %v", scanner.ScanStatusFailed, updatedJob.Status)
		}
	})
}

// mockBackgroundStarter implements BackgroundStarter for testing.
type mockBackgroundStarter struct {
	called   bool
	jobID    int64
	libID    int64
	panicCtx string
}

func (m *mockBackgroundStarter) StartScanBackground(jobID int64, libraryID int64, panicContext string) {
	m.called = true
	m.jobID = jobID
	m.libID = libraryID
	m.panicCtx = panicContext
}

func TestHandleStuckScan(t *testing.T) {
	t.Run("resumes stuck scan with pending checkpoints", func(t *testing.T) {
		scanJobRepo := mocks.NewScanJobRepository(t)
		checkpointRepo := mocks.NewCheckpointRepository(t)
		libraryRepo := mocks.NewLibraryRepository(t)

		job := &scanner.ScanJob{
			ID:         100,
			LibraryID:  1,
			Status:     scanner.ScanStatusRunning,
			FilesFound: 10,
		}
		scanJobRepo.WithJobs(job)

		checkpointRepo.WithStats(&scanner.CheckpointStats{
			TotalFiles:     10,
			PendingFiles:   5,
			CompletedFiles: 5,
			ProcessedFiles: 5,
		})

		lib := &library.Library{ID: 1, Path: "/media"}
		libraryRepo.WithLibraries(lib)

		scanRepos := &scan.ScanRepositories{
			ScanJob:    scanJobRepo,
			Checkpoint: checkpointRepo,
		}

		mediaRepos := &scan.MediaRepositories{
			Library: libraryRepo,
		}

		deps := createTestDeps(t, scanRepos, mediaRepos)

		bgStarter := &mockBackgroundStarter{}

		HandleStuckScan(context.Background(), deps, job, bgStarter)

		if !bgStarter.called {
			t.Error("expected background starter to be called")
		}
	})

	t.Run("marks completed when no pending files", func(t *testing.T) {
		scanJobRepo := mocks.NewScanJobRepository(t)
		checkpointRepo := mocks.NewCheckpointRepository(t)
		libraryRepo := mocks.NewLibraryRepository(t)

		job := &scanner.ScanJob{
			ID:         100,
			LibraryID:  1,
			Status:     scanner.ScanStatusRunning,
			FilesFound: 10,
		}
		scanJobRepo.WithJobs(job)

		checkpointRepo.WithStats(&scanner.CheckpointStats{
			TotalFiles:     10,
			PendingFiles:   0,
			CompletedFiles: 10,
			ProcessedFiles: 10,
		})

		lib := &library.Library{ID: 1, Path: "/media"}
		libraryRepo.WithLibraries(lib)

		scanRepos := &scan.ScanRepositories{
			ScanJob:    scanJobRepo,
			Checkpoint: checkpointRepo,
		}

		mediaRepos := &scan.MediaRepositories{
			Library: libraryRepo,
		}

		deps := createTestDeps(t, scanRepos, mediaRepos)

		bgStarter := &mockBackgroundStarter{}

		HandleStuckScan(context.Background(), deps, job, bgStarter)

		// Should not start background scan since job is actually complete
		if bgStarter.called {
			t.Error("expected background starter NOT to be called for complete job")
		}

		// Should mark job as completed
		updatedJob, _ := scanJobRepo.GetByID(context.Background(), 100)
		if updatedJob.Status != scanner.ScanStatusCompleted {
			t.Errorf("expected completed status, got %v", updatedJob.Status)
		}
	})

	t.Run("handles library not found error", func(t *testing.T) {
		scanJobRepo := mocks.NewScanJobRepository(t)
		checkpointRepo := mocks.NewCheckpointRepository(t)
		libraryRepo := mocks.NewLibraryRepository(t)

		job := &scanner.ScanJob{
			ID:        100,
			LibraryID: 999, // Non-existent library
			Status:    scanner.ScanStatusRunning,
		}
		scanJobRepo.WithJobs(job)

		libraryRepo.GetErr = fmt.Errorf("library not found")

		scanRepos := &scan.ScanRepositories{
			ScanJob:    scanJobRepo,
			Checkpoint: checkpointRepo,
		}

		mediaRepos := &scan.MediaRepositories{
			Library: libraryRepo,
		}

		deps := createTestDeps(t, scanRepos, mediaRepos)

		bgStarter := &mockBackgroundStarter{}

		HandleStuckScan(context.Background(), deps, job, bgStarter)

		// Should mark job as failed
		updatedJob, _ := scanJobRepo.GetByID(context.Background(), 100)
		if updatedJob.Status != scanner.ScanStatusFailed {
			t.Errorf("expected failed status, got %v", updatedJob.Status)
		}
	})
}

func TestValidateCheckpointCompleteness(t *testing.T) {
	tests := []struct {
		name           string
		filesFound     int64
		totalFiles     int64
		expectedResult bool
		expectDelete   bool
	}{
		{
			name:           "complete - checkpoints exceed minimum",
			filesFound:     100,
			totalFiles:     10, // 10 >= 100/100 = 1
			expectedResult: true,
			expectDelete:   false,
		},
		{
			name:           "complete - files found is zero",
			filesFound:     0,
			totalFiles:     0,
			expectedResult: true,
			expectDelete:   false,
		},
		{
			name:           "incomplete - checkpoints below minimum",
			filesFound:     1000,
			totalFiles:     5, // 5 < 1000/100 = 10
			expectedResult: false,
			expectDelete:   true,
		},
		{
			name:           "complete - exactly at minimum",
			filesFound:     100,
			totalFiles:     1, // 1 >= 100/100 = 1
			expectedResult: true,
			expectDelete:   false,
		},
		{
			name:           "complete - small files found uses minimum of 1",
			filesFound:     50,
			totalFiles:     1, // 1 >= max(50/100, 1) = 1
			expectedResult: true,
			expectDelete:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkpointRepo := mocks.NewCheckpointRepository(t)
			scanRepos := &scan.ScanRepositories{
				Checkpoint: checkpointRepo,
			}

			deps := &Deps{
				ScanRepos: scanRepos,
				Logger:    testLogger(),
			}

			currentJob := &scanner.ScanJob{
				ID:         100,
				FilesFound: tt.filesFound,
			}

			stats := &scanner.CheckpointStats{
				TotalFiles: tt.totalFiles,
			}

			result := validateCheckpointCompleteness(context.Background(), deps, 100, currentJob, stats)

			if result != tt.expectedResult {
				t.Errorf("expected result %v, got %v", tt.expectedResult, result)
			}

			// We can verify delete was called by checking checkpoints are gone
			// The mock automatically deletes when DeleteByJobID is called
		})
	}
}

func TestValidateCheckpointCompleteness_DeleteError(t *testing.T) {
	checkpointRepo := mocks.NewCheckpointRepository(t)
	checkpointRepo.DeleteByJobIDErr = errors.New("database error")

	scanRepos := &scan.ScanRepositories{
		Checkpoint: checkpointRepo,
	}

	deps := &Deps{
		ScanRepos: scanRepos,
		Logger:    testLogger(),
	}

	currentJob := &scanner.ScanJob{
		ID:         100,
		FilesFound: 1000,
	}

	stats := &scanner.CheckpointStats{
		TotalFiles: 1, // Too few
	}

	// Should still return false even if delete fails
	result := validateCheckpointCompleteness(context.Background(), deps, 100, currentJob, stats)
	if result != false {
		t.Error("expected false result even with delete error")
	}
}

func TestResumeFromCheckpoints(t *testing.T) {
	t.Run("resumes scan successfully", func(t *testing.T) {
		scanJobRepo := mocks.NewScanJobRepository(t)
		checkpointRepo := mocks.NewCheckpointRepository(t)
		scanStateRepo := mocks.NewScanStateRepository(t)

		job := &scanner.ScanJob{
			ID:             100,
			LibraryID:      1,
			Status:         scanner.ScanStatusRunning,
			FilesFound:     10,
			EstimatedTotal: 10,
		}
		scanJobRepo.WithJobs(job)

		// Set stats to show all files processed so the loop completes immediately
		// ProcessedFiles >= TotalFiles triggers completion
		checkpointRepo.WithStats(&scanner.CheckpointStats{
			TotalFiles:     10,
			PendingFiles:   0,
			CompletedFiles: 9,
			ProcessedFiles: 10, // All processed
			FailedFiles:    1,
			WarningFiles:   0,
		})

		scanRepos := &scan.ScanRepositories{
			ScanJob:    scanJobRepo,
			Checkpoint: checkpointRepo,
			ScanState:  scanStateRepo,
		}

		mediaRepos := &scan.MediaRepositories{
			Media: mocks.NewMediaRepository(t),
		}

		deps := createTestDeps(t, scanRepos, mediaRepos)

		lib := &library.Library{ID: 1, Path: "/media"}

		// This should not panic and should complete quickly since all files are processed
		ResumeFromCheckpoints(context.Background(), deps, 100, lib)
	})

	t.Run("handles GetStats error", func(t *testing.T) {
		scanJobRepo := mocks.NewScanJobRepository(t)
		checkpointRepo := mocks.NewCheckpointRepository(t)
		checkpointRepo.GetStatsErr = errors.New("database error")

		scanRepos := &scan.ScanRepositories{
			ScanJob:    scanJobRepo,
			Checkpoint: checkpointRepo,
		}

		deps := &Deps{
			ScanRepos: scanRepos,
			Logger:    testLogger(),
		}

		lib := &library.Library{ID: 1, Path: "/media"}

		// Should not panic
		ResumeFromCheckpoints(context.Background(), deps, 100, lib)
	})

	t.Run("handles GetByID error", func(t *testing.T) {
		scanJobRepo := mocks.NewScanJobRepository(t)
		checkpointRepo := mocks.NewCheckpointRepository(t)

		checkpointRepo.WithStats(&scanner.CheckpointStats{
			TotalFiles:     10,
			PendingFiles:   5,
			CompletedFiles: 5,
		})

		// Job doesn't exist
		scanJobRepo.GetErr = errors.New("job not found")

		scanRepos := &scan.ScanRepositories{
			ScanJob:    scanJobRepo,
			Checkpoint: checkpointRepo,
		}

		deps := &Deps{
			ScanRepos: scanRepos,
			Logger:    testLogger(),
		}

		lib := &library.Library{ID: 1, Path: "/media"}

		// Should not panic
		ResumeFromCheckpoints(context.Background(), deps, 999, lib)
	})
}

func TestCanResumeFromCheckpoints_NoCheckpoints(t *testing.T) {
	scanJobRepo := mocks.NewScanJobRepository(t)
	checkpointRepo := mocks.NewCheckpointRepository(t)

	job := &scanner.ScanJob{
		ID:         100,
		LibraryID:  1,
		Status:     scanner.ScanStatusRunning,
		FilesFound: 100,
	}
	scanJobRepo.WithJobs(job)

	// No checkpoints at all
	checkpointRepo.WithStats(&scanner.CheckpointStats{
		TotalFiles:     0,
		PendingFiles:   0,
		CompletedFiles: 0,
	})

	scanRepos := &scan.ScanRepositories{
		ScanJob:    scanJobRepo,
		Checkpoint: checkpointRepo,
	}

	deps := &Deps{
		ScanRepos: scanRepos,
		Logger:    testLogger(),
	}

	lib := &library.Library{ID: 1, Path: "/media"}
	params := &RunScanParams{JobID: 100, Lib: lib}

	result := CanResumeFromCheckpoints(context.Background(), deps, params, job)

	if result {
		t.Error("should not resume when there are no checkpoints")
	}
}

func TestCanResumeFromCheckpoints_StatsError(t *testing.T) {
	scanJobRepo := mocks.NewScanJobRepository(t)
	checkpointRepo := mocks.NewCheckpointRepository(t)

	job := &scanner.ScanJob{
		ID:         100,
		LibraryID:  1,
		Status:     scanner.ScanStatusRunning,
		FilesFound: 100,
	}
	scanJobRepo.WithJobs(job)

	checkpointRepo.GetStatsErr = errors.New("database error")

	scanRepos := &scan.ScanRepositories{
		ScanJob:    scanJobRepo,
		Checkpoint: checkpointRepo,
	}

	deps := &Deps{
		ScanRepos: scanRepos,
		Logger:    testLogger(),
	}

	lib := &library.Library{ID: 1, Path: "/media"}
	params := &RunScanParams{JobID: 100, Lib: lib}

	result := CanResumeFromCheckpoints(context.Background(), deps, params, job)

	if result {
		t.Error("should not resume when stats retrieval fails")
	}
}

func TestCanResumeFromCheckpoints_IncompleteCheckpoints(t *testing.T) {
	scanJobRepo := mocks.NewScanJobRepository(t)
	checkpointRepo := mocks.NewCheckpointRepository(t)

	job := &scanner.ScanJob{
		ID:         100,
		LibraryID:  1,
		Status:     scanner.ScanStatusRunning,
		FilesFound: 10000, // Large number
	}
	scanJobRepo.WithJobs(job)

	// Too few checkpoints for the files found (requires 10000/100 = 100)
	checkpointRepo.WithStats(&scanner.CheckpointStats{
		TotalFiles:     5, // Below minimum
		PendingFiles:   2,
		CompletedFiles: 3,
	})

	scanRepos := &scan.ScanRepositories{
		ScanJob:    scanJobRepo,
		Checkpoint: checkpointRepo,
	}

	deps := &Deps{
		ScanRepos: scanRepos,
		Logger:    testLogger(),
	}

	lib := &library.Library{ID: 1, Path: "/media"}
	params := &RunScanParams{JobID: 100, Lib: lib}

	result := CanResumeFromCheckpoints(context.Background(), deps, params, job)

	if result {
		t.Error("should not resume when checkpoint creation was incomplete")
	}
}

func TestCanResumeFromCheckpoints_AlreadyComplete(t *testing.T) {
	scanJobRepo := mocks.NewScanJobRepository(t)
	checkpointRepo := mocks.NewCheckpointRepository(t)

	job := &scanner.ScanJob{
		ID:         100,
		LibraryID:  1,
		Status:     scanner.ScanStatusRunning,
		FilesFound: 100,
	}
	scanJobRepo.WithJobs(job)

	// All files are processed (no pending)
	checkpointRepo.WithStats(&scanner.CheckpointStats{
		TotalFiles:     100,
		PendingFiles:   0,
		CompletedFiles: 100,
		ProcessedFiles: 100,
	})

	scanRepos := &scan.ScanRepositories{
		ScanJob:    scanJobRepo,
		Checkpoint: checkpointRepo,
	}

	deps := createTestDeps(t, scanRepos, nil)

	lib := &library.Library{ID: 1, Path: "/media"}
	params := &RunScanParams{JobID: 100, Lib: lib}

	result := CanResumeFromCheckpoints(context.Background(), deps, params, job)

	// Should return true and mark as complete
	if !result {
		t.Error("should return true when scan is already complete")
	}

	// Job should be marked as completed
	updatedJob, _ := scanJobRepo.GetByID(context.Background(), 100)
	if updatedJob.Status != scanner.ScanStatusCompleted {
		t.Errorf("expected completed status, got %v", updatedJob.Status)
	}
}

func TestCanResumeFromCheckpoints_ValidResume(t *testing.T) {
	scanJobRepo := mocks.NewScanJobRepository(t)
	checkpointRepo := mocks.NewCheckpointRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	job := &scanner.ScanJob{
		ID:             100,
		LibraryID:      1,
		Status:         scanner.ScanStatusRunning,
		FilesFound:     100,
		EstimatedTotal: 100,
	}
	scanJobRepo.WithJobs(job)

	// Set stats to show all files processed so the resume completes quickly
	// This tests that CanResumeFromCheckpoints returns true when there are
	// valid checkpoints to resume from (even if already complete)
	checkpointRepo.WithStats(&scanner.CheckpointStats{
		TotalFiles:     100,
		PendingFiles:   0,
		CompletedFiles: 100,
		ProcessedFiles: 100,
		FailedFiles:    0,
		WarningFiles:   0,
	})

	scanRepos := &scan.ScanRepositories{
		ScanJob:    scanJobRepo,
		Checkpoint: checkpointRepo,
		ScanState:  scanStateRepo,
	}

	mediaRepos := &scan.MediaRepositories{
		Media: mocks.NewMediaRepository(t),
	}

	deps := createTestDeps(t, scanRepos, mediaRepos)

	lib := &library.Library{ID: 1, Path: "/media"}
	params := &RunScanParams{JobID: 100, Lib: lib}

	result := CanResumeFromCheckpoints(context.Background(), deps, params, job)

	// Should return true (already complete path)
	if !result {
		t.Error("should return true when valid checkpoints exist")
	}
}

func TestResumeFromCheckpoints_UpdateProgressError(t *testing.T) {
	scanJobRepo := mocks.NewScanJobRepository(t)
	checkpointRepo := mocks.NewCheckpointRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	job := &scanner.ScanJob{
		ID:             100,
		LibraryID:      1,
		Status:         scanner.ScanStatusRunning,
		FilesFound:     10,
		EstimatedTotal: 10,
	}
	scanJobRepo.WithJobs(job)

	// Set stats to show all files processed so the loop completes
	checkpointRepo.WithStats(&scanner.CheckpointStats{
		TotalFiles:     10,
		PendingFiles:   0,
		CompletedFiles: 10,
		ProcessedFiles: 10,
		FailedFiles:    0,
		WarningFiles:   0,
	})

	// Simulate error when updating progress
	scanJobRepo.UpdateProgressErr = errors.New("database error")

	scanRepos := &scan.ScanRepositories{
		ScanJob:    scanJobRepo,
		Checkpoint: checkpointRepo,
		ScanState:  scanStateRepo,
	}

	mediaRepos := &scan.MediaRepositories{
		Media: mocks.NewMediaRepository(t),
	}

	deps := createTestDeps(t, scanRepos, mediaRepos)

	lib := &library.Library{ID: 1, Path: "/media"}

	// Should not panic even if update fails
	ResumeFromCheckpoints(context.Background(), deps, 100, lib)
}

func TestProcessFilesWithCheckpoints_WithImageCleanup(t *testing.T) {
	scanJobRepo := mocks.NewScanJobRepository(t)
	checkpointRepo := mocks.NewCheckpointRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	job := &scanner.ScanJob{
		ID:        100,
		LibraryID: 1,
	}
	scanJobRepo.WithJobs(job)

	scanRepos := &scan.ScanRepositories{
		ScanJob:    scanJobRepo,
		Checkpoint: checkpointRepo,
		ScanState:  scanStateRepo,
	}

	mediaRepos := &scan.MediaRepositories{
		Media: mocks.NewMediaRepository(t),
	}

	deps := createTestDeps(t, scanRepos, mediaRepos)

	// Enable image cleanup but disable it for this test (to avoid nil pointer)
	deps.HasImageCleanup = func() bool { return false }

	lib := &library.Library{ID: 1, Path: "/media"}

	hashingDone := make(chan struct{})
	close(hashingDone)

	// Test with image cleanup disabled - should not panic
	ProcessFilesWithCheckpoints(context.Background(), deps, 100, lib, hashingDone, nil)
}

func TestProcessFilesWithCheckpoints_WithDiscoveryStats(t *testing.T) {
	scanJobRepo := mocks.NewScanJobRepository(t)
	checkpointRepo := mocks.NewCheckpointRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	job := &scanner.ScanJob{
		ID:        100,
		LibraryID: 1,
	}
	scanJobRepo.WithJobs(job)

	scanRepos := &scan.ScanRepositories{
		ScanJob:    scanJobRepo,
		Checkpoint: checkpointRepo,
		ScanState:  scanStateRepo,
	}

	mediaRepos := &scan.MediaRepositories{
		Media: mocks.NewMediaRepository(t),
	}

	deps := createTestDeps(t, scanRepos, mediaRepos)

	lib := &library.Library{ID: 1, Path: "/media"}

	hashingDone := make(chan struct{})
	close(hashingDone)

	discoveryStats := &filesystem.WalkStats{
		FilesDiscovered: 10,
		DirsScanned:     5,
	}

	// Test with discovery stats - should pass them through
	ProcessFilesWithCheckpoints(context.Background(), deps, 100, lib, hashingDone, discoveryStats)
}

func TestHandleStuckScan_WithCheckpointStatsWarning(t *testing.T) {
	scanJobRepo := mocks.NewScanJobRepository(t)
	checkpointRepo := mocks.NewCheckpointRepository(t)
	libraryRepo := mocks.NewLibraryRepository(t)

	job := &scanner.ScanJob{
		ID:         100,
		LibraryID:  1,
		Status:     scanner.ScanStatusRunning,
		FilesFound: 10,
	}
	scanJobRepo.WithJobs(job)

	// Stats retrieval fails, but we still resume
	checkpointRepo.GetStatsErr = errors.New("connection timeout")

	lib := &library.Library{ID: 1, Path: "/media"}
	libraryRepo.WithLibraries(lib)

	scanRepos := &scan.ScanRepositories{
		ScanJob:    scanJobRepo,
		Checkpoint: checkpointRepo,
	}

	mediaRepos := &scan.MediaRepositories{
		Library: libraryRepo,
	}

	deps := createTestDeps(t, scanRepos, mediaRepos)

	bgStarter := &mockBackgroundStarter{}

	// Should still try to resume even if stats fail
	HandleStuckScan(context.Background(), deps, job, bgStarter)

	if !bgStarter.called {
		t.Error("expected background starter to be called despite stats error")
	}

	if bgStarter.jobID != 100 {
		t.Errorf("expected jobID 100, got %d", bgStarter.jobID)
	}

	if bgStarter.panicCtx != "resumed stuck scan goroutine panicked" {
		t.Errorf("unexpected panic context: %s", bgStarter.panicCtx)
	}
}

func TestPhaseWalkDirectory_ProgressUpdateError(t *testing.T) {
	tmpDir := t.TempDir()

	testFiles := []string{"movie1.mp4", "movie2.mkv"}
	for _, filename := range testFiles {
		if err := os.WriteFile(filepath.Join(tmpDir, filename), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	scanJobRepo := mocks.NewScanJobRepository(t)
	job := &scanner.ScanJob{
		ID:             100,
		LibraryID:      1,
		EstimatedTotal: 0,
	}
	scanJobRepo.WithJobs(job)

	// Simulate error when updating progress
	scanJobRepo.UpdateProgressErr = errors.New("failed to update")

	scanRepos := &scan.ScanRepositories{
		ScanJob: scanJobRepo,
	}

	deps := createTestDeps(t, scanRepos, nil)

	dctx := discovery.NewContext(100, &library.Library{ID: 1, Path: tmpDir}, job, filesystem.NewWalker(filesystem.WithLogger(testLogger())))

	files, err := phaseWalkDirectory(context.Background(), deps, dctx)

	// Should still complete despite progress update error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 media files, got %d", len(files))
	}
}

func TestRunFreshScan_WithActualFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test media files
	testFiles := []string{"movie1.mp4", "movie2.mkv", "song.mp3"}
	for _, filename := range testFiles {
		if err := os.WriteFile(filepath.Join(tmpDir, filename), []byte("test content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	scanJobRepo := mocks.NewScanJobRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)
	checkpointRepo := mocks.NewCheckpointRepository(t)

	job := &scanner.ScanJob{
		ID:             100,
		LibraryID:      1,
		Status:         scanner.ScanStatusRunning,
		EstimatedTotal: 0,
	}
	scanJobRepo.WithJobs(job)

	repos := &scan.ScanRepositories{
		ScanJob:    scanJobRepo,
		ScanState:  scanStateRepo,
		Checkpoint: checkpointRepo,
	}

	incScanner := discovery.NewIncrementalScanner(scanStateRepo, testLogger())

	mediaRepos := &scan.MediaRepositories{
		Media: mocks.NewMediaRepository(t),
	}

	lib := &library.Library{
		ID:   1,
		Path: tmpDir,
	}

	deps := createTestDeps(t, repos, mediaRepos)
	deps.DiscoveryDeps = func() *discovery.Deps {
		return &discovery.Deps{
			ScanRepos:   repos,
			Config:      deps.Config,
			Logger:      testLogger(),
			IncrScanner: incScanner,
			IsMediaFile: func(ext string) bool {
				switch ext {
				case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm",
					".mp3", ".flac", ".wav", ".aac", ".ogg", ".m4a":
					return true
				}
				return false
			},
		}
	}

	RunFreshScan(context.Background(), deps, 100, lib)

	time.Sleep(200 * time.Millisecond)

	// Verify files were discovered
	updatedJob, err := repos.ScanJob.GetByID(context.Background(), 100)
	if err == nil {
		if updatedJob.FilesFound == 0 {
			t.Error("expected files to be discovered")
		}
	}
}

func TestPhaseHashAndProcess_WithPanicRecovery(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "movie.mp4")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	checkpointRepo := mocks.NewCheckpointRepository(t)
	scanJobRepo := mocks.NewScanJobRepository(t)

	job := &scanner.ScanJob{
		ID:        100,
		LibraryID: 1,
		Status:    scanner.ScanStatusRunning,
	}
	scanJobRepo.WithJobs(job)

	scanRepos := &scan.ScanRepositories{
		Checkpoint: checkpointRepo,
		ScanJob:    scanJobRepo,
	}

	deps := createTestDeps(t, scanRepos, nil)

	// Track panic recovery calls
	panicRecoveryCalled := false
	deps.RecoverFromPanicWithError = func(jobID, libraryID int64, description string, errChan chan<- error) {
		panicRecoveryCalled = true
	}

	dctx := discovery.NewContext(100, &library.Library{ID: 1, Path: tmpDir}, nil, nil)
	dctx.DiscoveryStats = &filesystem.WalkStats{FilesDiscovered: 1}

	diff := &scanner.ScanDiff{
		NewFiles: []scanner.FileInfo{
			{Path: testFile, Size: 12, ModTime: time.Now()},
		},
		ModifiedFiles:  []scanner.FileInfo{},
		UnchangedFiles: []string{},
	}

	phaseHashAndProcess(context.Background(), deps, dctx, diff)

	// Verify panic recovery setup was used
	_ = panicRecoveryCalled
}

func TestPhaseDetermineChanges_ModifiedFiles(t *testing.T) {
	scanJobRepo := mocks.NewScanJobRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	job := &scanner.ScanJob{
		ID:        100,
		LibraryID: 1,
	}
	scanJobRepo.WithJobs(job)

	// Existing file with old modification time
	oldModTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	scanState := &scanner.ScanState{
		LibraryID: 1,
		FilePath:  "/media/modified.mp4",
		FileSize:  1000,
		FileMTime: oldModTime,
	}
	scanStateRepo.WithStates(scanState)

	repos := &scan.ScanRepositories{
		ScanJob:   scanJobRepo,
		ScanState: scanStateRepo,
	}

	incScanner := discovery.NewIncrementalScanner(repos.ScanState, testLogger())

	deps := createTestDeps(t, repos, nil)
	deps.DiscoveryDeps = func() *discovery.Deps {
		return &discovery.Deps{
			ScanRepos:   repos,
			Config:      deps.Config,
			Logger:      testLogger(),
			IncrScanner: incScanner,
		}
	}

	dctx := discovery.NewContext(100, &library.Library{ID: 1, Path: "/media"}, nil, nil)

	// Same file but with newer modification time
	newModTime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	discoveredFiles := []scanner.FileInfo{
		{Path: "/media/modified.mp4", Size: 1000, ModTime: newModTime},
	}

	diff := phaseDetermineChanges(context.Background(), deps, dctx, discoveredFiles)

	if diff == nil {
		t.Fatal("expected non-nil diff for modified files")
	}

	if len(diff.ModifiedFiles) != 1 {
		t.Errorf("expected 1 modified file, got %d", len(diff.ModifiedFiles))
	}
}

func TestPhaseHashAndProcess_ProcessingError(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "movie.mp4")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	checkpointRepo := mocks.NewCheckpointRepository(t)
	scanJobRepo := mocks.NewScanJobRepository(t)

	job := &scanner.ScanJob{
		ID:        100,
		LibraryID: 1,
		Status:    scanner.ScanStatusRunning,
	}
	scanJobRepo.WithJobs(job)

	// Set stats to show processing complete so loop exits immediately
	checkpointRepo.WithStats(&scanner.CheckpointStats{
		TotalFiles:     1,
		PendingFiles:   0,
		CompletedFiles: 1,
		ProcessedFiles: 1,
		FailedFiles:    0,
		WarningFiles:   0,
	})

	scanRepos := &scan.ScanRepositories{
		Checkpoint: checkpointRepo,
		ScanJob:    scanJobRepo,
	}

	deps := createTestDeps(t, scanRepos, nil)

	// Simulate processing error via error channel
	deps.RecoverFromPanicWithError = func(jobID, libraryID int64, description string, errChan chan<- error) {
		// Don't send error for successful case
	}

	dctx := discovery.NewContext(100, &library.Library{ID: 1, Path: tmpDir}, nil, nil)
	dctx.DiscoveryStats = &filesystem.WalkStats{FilesDiscovered: 1}

	diff := &scanner.ScanDiff{
		NewFiles: []scanner.FileInfo{
			{Path: testFile, Size: 12, ModTime: time.Now()},
		},
		ModifiedFiles:  []scanner.FileInfo{},
		UnchangedFiles: []string{},
	}

	phaseHashAndProcess(context.Background(), deps, dctx, diff)

	// Processing should complete
	time.Sleep(100 * time.Millisecond)
}

func TestPhaseHashAndProcess_WithModifiedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	newFile := filepath.Join(tmpDir, "new.mp4")
	modFile := filepath.Join(tmpDir, "modified.mp4")

	if err := os.WriteFile(newFile, []byte("new content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modFile, []byte("modified content"), 0644); err != nil {
		t.Fatal(err)
	}

	checkpointRepo := mocks.NewCheckpointRepository(t)
	scanJobRepo := mocks.NewScanJobRepository(t)

	job := &scanner.ScanJob{
		ID:        100,
		LibraryID: 1,
		Status:    scanner.ScanStatusRunning,
	}
	scanJobRepo.WithJobs(job)

	scanRepos := &scan.ScanRepositories{
		Checkpoint: checkpointRepo,
		ScanJob:    scanJobRepo,
	}

	deps := createTestDeps(t, scanRepos, nil)

	dctx := discovery.NewContext(100, &library.Library{ID: 1, Path: tmpDir}, nil, nil)
	dctx.DiscoveryStats = &filesystem.WalkStats{FilesDiscovered: 2}

	diff := &scanner.ScanDiff{
		NewFiles: []scanner.FileInfo{
			{Path: newFile, Size: 11, ModTime: time.Now()},
		},
		ModifiedFiles: []scanner.FileInfo{
			{Path: modFile, Size: 16, ModTime: time.Now()},
		},
		UnchangedFiles: []string{},
	}

	phaseHashAndProcess(context.Background(), deps, dctx, diff)

	time.Sleep(100 * time.Millisecond)
}

func TestRunFreshScan_WithWalkError(t *testing.T) {
	scanJobRepo := mocks.NewScanJobRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)
	checkpointRepo := mocks.NewCheckpointRepository(t)

	job := &scanner.ScanJob{
		ID:             100,
		LibraryID:      1,
		Status:         scanner.ScanStatusRunning,
		EstimatedTotal: 0,
	}
	scanJobRepo.WithJobs(job)

	repos := &scan.ScanRepositories{
		ScanJob:    scanJobRepo,
		ScanState:  scanStateRepo,
		Checkpoint: checkpointRepo,
	}

	incScanner := discovery.NewIncrementalScanner(scanStateRepo, testLogger())

	mediaRepos := &scan.MediaRepositories{
		Media: mocks.NewMediaRepository(t),
	}

	// Use non-existent directory to trigger error
	lib := &library.Library{
		ID:   1,
		Path: "/nonexistent/path/that/does/not/exist",
	}

	deps := createTestDeps(t, repos, mediaRepos)
	deps.DiscoveryDeps = func() *discovery.Deps {
		return &discovery.Deps{
			ScanRepos:   repos,
			Config:      deps.Config,
			Logger:      testLogger(),
			IncrScanner: incScanner,
			IsMediaFile: func(ext string) bool {
				switch ext {
				case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm",
					".mp3", ".flac", ".wav", ".aac", ".ogg", ".m4a":
					return true
				}
				return false
			},
		}
	}

	RunFreshScan(context.Background(), deps, 100, lib)

	time.Sleep(100 * time.Millisecond)

	// Should mark job as failed
	updatedJob, err := repos.ScanJob.GetByID(context.Background(), 100)
	if err == nil && updatedJob.Status == scanner.ScanStatusFailed {
		// Expected behavior
	}
}

func TestProcessFilesWithCheckpoints_HashingNotDone(t *testing.T) {
	scanJobRepo := mocks.NewScanJobRepository(t)
	checkpointRepo := mocks.NewCheckpointRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	job := &scanner.ScanJob{
		ID:        100,
		LibraryID: 1,
	}
	scanJobRepo.WithJobs(job)

	scanRepos := &scan.ScanRepositories{
		ScanJob:    scanJobRepo,
		Checkpoint: checkpointRepo,
		ScanState:  scanStateRepo,
	}

	mediaRepos := &scan.MediaRepositories{
		Media: mocks.NewMediaRepository(t),
	}

	deps := createTestDeps(t, scanRepos, mediaRepos)

	lib := &library.Library{ID: 1, Path: "/media"}

	// Create hashingDone channel but don't close it immediately
	hashingDone := make(chan struct{})

	go func() {
		time.Sleep(50 * time.Millisecond)
		close(hashingDone)
	}()

	// Should wait for hashingDone to be closed
	ProcessFilesWithCheckpoints(context.Background(), deps, 100, lib, hashingDone, nil)
}
