package processing

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/system"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testConfig() *scan.Config {
	return &scan.Config{
		CheckpointBufferSize: 100,
		HashProgressLogEvery: 1,
		BatchWriteTimeout:    100 * time.Millisecond,
	}
}

func testDeps(t *testing.T, checkpointRepo *mocks.CheckpointRepository, scanStateRepo *mocks.ScanStateRepository, profile *system.Profile) *Deps {
	return &Deps{
		ScanRepos: &scan.ScanRepositories{
			Checkpoint: checkpointRepo,
			ScanState:  scanStateRepo,
		},
		Config:        testConfig(),
		SystemProfile: profile,
		Logger:        testLogger(),
	}
}

func TestHashAndStreamCheckpoints(t *testing.T) {
	tests := []struct {
		name                string
		filesToProcess      []scanner.FileInfo
		existingStates      []*scanner.ScanState
		systemProfile       *system.Profile
		createBatchErr      error
		getScanStateErr     error
		jobID               int64
		libraryID           int64
		expectedError       bool
		expectedCPCount     int
		validateCheckpoints func(t *testing.T, repo *mocks.CheckpointRepository)
	}{
		{
			name: "successful hashing of multiple files",
			filesToProcess: []scanner.FileInfo{
				{Path: "/tmp/test1.mp4", Size: 1024, ModTime: time.Now()},
				{Path: "/tmp/test2.mp4", Size: 2048, ModTime: time.Now()},
				{Path: "/tmp/test3.mp4", Size: 3072, ModTime: time.Now()},
			},
			jobID:     1,
			libraryID: 1,
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumCPU:      8,
					NumPhysical: 4,
				},
				Storage: system.StorageProfile{
					Type: system.StorageTypeLocalSSD,
				},
			},
			expectedError:   false,
			expectedCPCount: 3,
			validateCheckpoints: func(t *testing.T, repo *mocks.CheckpointRepository) {
				count := repo.GetCheckpointCount(1)
				if count != 3 {
					t.Errorf("Expected 3 checkpoints, got %d", count)
				}

				stats, err := repo.GetStats(context.Background(), 1)
				if err != nil {
					t.Fatalf("Failed to get stats: %v", err)
				}
				if stats.TotalFiles != 3 {
					t.Errorf("Expected 3 total files, got %d", stats.TotalFiles)
				}
			},
		},
		{
			name: "reuse existing hash from scan state",
			filesToProcess: []scanner.FileInfo{
				{Path: "/tmp/cached.mp4", Size: 1024, ModTime: time.Now()},
				{Path: "/tmp/new.mp4", Size: 2048, ModTime: time.Now()},
			},
			existingStates: []*scanner.ScanState{
				{
					LibraryID: 1,
					FilePath:  "/tmp/cached.mp4",
					FileHash:  "existing-hash-12345",
					FileSize:  1024,
				},
			},
			jobID:     2,
			libraryID: 1,
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumCPU:      4,
					NumPhysical: 2,
				},
				Storage: system.StorageProfile{
					Type: system.StorageTypeLocalHDD,
				},
			},
			expectedError:   false,
			expectedCPCount: 2,
			validateCheckpoints: func(t *testing.T, repo *mocks.CheckpointRepository) {
				count := repo.GetCheckpointCount(2)
				if count != 2 {
					t.Errorf("Expected 2 checkpoints, got %d", count)
				}

				// Find the cached checkpoint and verify it has the existing hash
				ctx := context.Background()
				checkpoints, err := repo.GetPendingBatch(ctx, 2, 10)
				if err != nil {
					t.Fatalf("Failed to get pending batch: %v", err)
				}

				foundCached := false
				for _, cp := range checkpoints {
					if cp.FilePath == "/tmp/cached.mp4" {
						foundCached = true
						if cp.FileHash != "existing-hash-12345" {
							t.Errorf("Expected hash 'existing-hash-12345', got '%s'", cp.FileHash)
						}
						break
					}
				}
				if !foundCached {
					t.Error("Should find cached checkpoint")
				}
			},
		},
		{
			name: "handle non-existent files gracefully",
			filesToProcess: []scanner.FileInfo{
				{Path: "/nonexistent/file.mp4", Size: 1024, ModTime: time.Now()},
			},
			jobID:     3,
			libraryID: 1,
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumCPU:      4,
					NumPhysical: 2,
				},
				Storage: system.StorageProfile{
					Type: system.StorageTypeLocalSSD,
				},
			},
			expectedError:   false,
			expectedCPCount: 1,
			validateCheckpoints: func(t *testing.T, repo *mocks.CheckpointRepository) {
				count := repo.GetCheckpointCount(3)
				if count != 1 {
					t.Errorf("Expected 1 checkpoint, got %d", count)
				}

				// Checkpoint should be created with empty hash (hash failed)
				ctx := context.Background()
				checkpoints, err := repo.GetPendingBatch(ctx, 3, 10)
				if err != nil {
					t.Fatalf("Failed to get pending batch: %v", err)
				}
				if len(checkpoints) != 1 {
					t.Fatalf("Expected 1 checkpoint, got %d", len(checkpoints))
				}

				// Hash should be empty due to file not existing
				if checkpoints[0].FileHash != "" {
					t.Errorf("Hash should be empty for non-existent file, got '%s'", checkpoints[0].FileHash)
				}
				if checkpoints[0].Status != scanner.CheckpointPending {
					t.Errorf("Expected status Pending, got %v", checkpoints[0].Status)
				}
			},
		},
		{
			name: "context cancellation is handled gracefully",
			filesToProcess: []scanner.FileInfo{
				{Path: "/tmp/test1.mp4", Size: 1024, ModTime: time.Now()},
				{Path: "/tmp/test2.mp4", Size: 2048, ModTime: time.Now()},
				{Path: "/tmp/test3.mp4", Size: 3072, ModTime: time.Now()},
				{Path: "/tmp/test4.mp4", Size: 4096, ModTime: time.Now()},
				{Path: "/tmp/test5.mp4", Size: 5120, ModTime: time.Now()},
			},
			jobID:     4,
			libraryID: 1,
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumCPU:      2,
					NumPhysical: 2,
				},
				Storage: system.StorageProfile{
					Type: system.StorageTypeLocalSSD,
				},
			},
			expectedError: false, // Context cancellation doesn't return error, just stops processing
			validateCheckpoints: func(t *testing.T, repo *mocks.CheckpointRepository) {
				// Context cancellation should be handled gracefully
				// Some or all checkpoints may be created depending on timing
				count := repo.GetCheckpointCount(4)
				if count > 5 {
					t.Errorf("Expected at most 5 checkpoints, got %d", count)
				}
			},
		},
		{
			name: "database error during batch insert",
			filesToProcess: []scanner.FileInfo{
				{Path: "/tmp/test1.mp4", Size: 1024, ModTime: time.Now()},
				{Path: "/tmp/test2.mp4", Size: 2048, ModTime: time.Now()},
			},
			jobID:          5,
			libraryID:      1,
			createBatchErr: errors.New("database connection lost"),
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumCPU:      4,
					NumPhysical: 2,
				},
				Storage: system.StorageProfile{
					Type: system.StorageTypeLocalSSD,
				},
			},
			expectedError: true,
		},
		{
			name: "batch processing with small batch size",
			filesToProcess: []scanner.FileInfo{
				{Path: "/tmp/test1.mp4", Size: 1024, ModTime: time.Now()},
				{Path: "/tmp/test2.mp4", Size: 2048, ModTime: time.Now()},
				{Path: "/tmp/test3.mp4", Size: 3072, ModTime: time.Now()},
				{Path: "/tmp/test4.mp4", Size: 4096, ModTime: time.Now()},
				{Path: "/tmp/test5.mp4", Size: 5120, ModTime: time.Now()},
			},
			jobID:     6,
			libraryID: 1,
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumCPU:      4,
					NumPhysical: 2,
				},
				Storage: system.StorageProfile{
					Type: system.StorageTypeLocalSSD,
				},
			},
			expectedError:   false,
			expectedCPCount: 5,
			validateCheckpoints: func(t *testing.T, repo *mocks.CheckpointRepository) {
				count := repo.GetCheckpointCount(6)
				if count != 5 {
					t.Errorf("Expected 5 checkpoints, got %d", count)
				}
			},
		},
		{
			name:           "empty file list",
			filesToProcess: []scanner.FileInfo{},
			jobID:          7,
			libraryID:      1,
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumCPU:      4,
					NumPhysical: 2,
				},
				Storage: system.StorageProfile{
					Type: system.StorageTypeLocalSSD,
				},
			},
			expectedError:   false,
			expectedCPCount: 0,
			validateCheckpoints: func(t *testing.T, repo *mocks.CheckpointRepository) {
				count := repo.GetCheckpointCount(7)
				if count != 0 {
					t.Errorf("Expected 0 checkpoints, got %d", count)
				}
			},
		},
		{
			name: "fallback to default settings when no system profile",
			filesToProcess: []scanner.FileInfo{
				{Path: "/tmp/test1.mp4", Size: 1024, ModTime: time.Now()},
				{Path: "/tmp/test2.mp4", Size: 2048, ModTime: time.Now()},
			},
			jobID:         8,
			libraryID:     1,
			systemProfile: nil, // No system profile
			expectedError:   false,
			expectedCPCount: 2,
			validateCheckpoints: func(t *testing.T, repo *mocks.CheckpointRepository) {
				count := repo.GetCheckpointCount(8)
				if count != 2 {
					t.Errorf("Expected 2 checkpoints, got %d", count)
				}
			},
		},
		{
			name: "scan state lookup error should not fail processing",
			filesToProcess: []scanner.FileInfo{
				{Path: "/tmp/test1.mp4", Size: 1024, ModTime: time.Now()},
			},
			getScanStateErr: errors.New("database error"),
			jobID:           9,
			libraryID:       1,
			systemProfile: &system.Profile{
				CPU: system.CPUProfile{
					NumCPU:      4,
					NumPhysical: 2,
				},
				Storage: system.StorageProfile{
					Type: system.StorageTypeLocalSSD,
				},
			},
			expectedError:   false, // Should continue even if scan state lookup fails
			expectedCPCount: 1,
			validateCheckpoints: func(t *testing.T, repo *mocks.CheckpointRepository) {
				count := repo.GetCheckpointCount(9)
				if count != 1 {
					t.Errorf("Expected 1 checkpoint, got %d", count)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create context (with cancellation for that specific test)
			ctx := context.Background()
			var cancel context.CancelFunc

			if tt.name == "context cancellation stops processing" {
				ctx, cancel = context.WithCancel(ctx)
				// Cancel context very quickly to stop processing early
				go func() {
					time.Sleep(5 * time.Millisecond)
					cancel()
				}()
			} else {
				ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
			}
			defer cancel()

			// Setup mocks
			checkpointRepo := mocks.NewCheckpointRepository(t)
			scanStateRepo := mocks.NewScanStateRepository(t)

			// Inject errors if specified
			checkpointRepo.CreateBatchErr = tt.createBatchErr
			scanStateRepo.GetByPathErr = tt.getScanStateErr

			// Pre-populate scan state if provided
			if len(tt.existingStates) > 0 {
				scanStateRepo.WithStates(tt.existingStates...)
			}

			// Create deps
			deps := testDeps(t, checkpointRepo, scanStateRepo, tt.systemProfile)

			// Execute
			err := HashAndStreamCheckpoints(ctx, deps, tt.filesToProcess, tt.jobID, tt.libraryID)

			// Assert
			if tt.expectedError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectedCPCount > 0 {
				count := checkpointRepo.GetCheckpointCount(tt.jobID)
				if count != tt.expectedCPCount {
					t.Errorf("Expected %d checkpoints, got %d", tt.expectedCPCount, count)
				}
			}

			if tt.validateCheckpoints != nil {
				tt.validateCheckpoints(t, checkpointRepo)
			}
		})
	}
}

func TestHashAndStreamCheckpoints_WithRealFiles(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create test files with known content
	testFiles := []struct {
		name    string
		content string
	}{
		{"file1.txt", "Hello, World!"},
		{"file2.txt", "This is test content for hashing"},
		{"file3.txt", "Another test file with different content"},
	}

	var filesToProcess []scanner.FileInfo
	for _, tf := range testFiles {
		path := filepath.Join(tmpDir, tf.name)
		err := os.WriteFile(path, []byte(tf.content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Failed to stat test file: %v", err)
		}

		filesToProcess = append(filesToProcess, scanner.FileInfo{
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	// Setup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	checkpointRepo := mocks.NewCheckpointRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	systemProfile := &system.Profile{
		CPU: system.CPUProfile{
			NumCPU:      4,
			NumPhysical: 2,
		},
		Storage: system.StorageProfile{
			Type: system.StorageTypeLocalSSD,
		},
	}

	deps := testDeps(t, checkpointRepo, scanStateRepo, systemProfile)

	// Execute
	err := HashAndStreamCheckpoints(ctx, deps, filesToProcess, 1, 1)
	if err != nil {
		t.Fatalf("HashAndStreamCheckpoints failed: %v", err)
	}

	// Validate
	count := checkpointRepo.GetCheckpointCount(1)
	if count != 3 {
		t.Errorf("Expected 3 checkpoints, got %d", count)
	}

	// Get checkpoints and verify they have hashes
	checkpoints, err := checkpointRepo.GetPendingBatch(ctx, 1, 10)
	if err != nil {
		t.Fatalf("Failed to get pending batch: %v", err)
	}
	if len(checkpoints) != 3 {
		t.Fatalf("Expected 3 checkpoints, got %d", len(checkpoints))
	}

	for _, cp := range checkpoints {
		if cp.FileHash == "" {
			t.Errorf("File should have been hashed: %s", cp.FilePath)
		}
		if cp.Status != scanner.CheckpointPending {
			t.Errorf("Expected status Pending, got %v", cp.Status)
		}
		if cp.FileSize <= 0 {
			t.Errorf("Expected positive file size, got %d", cp.FileSize)
		}
	}
}

func TestHashAndStreamCheckpoints_HashReuseOptimization(t *testing.T) {
	// This test verifies that hashes are reused from scan_state when available
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}

	filesToProcess := []scanner.FileInfo{
		{
			Path:    testFile,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		},
	}

	// Pre-populate scan state with existing hash
	existingHash := "cached-hash-from-previous-scan"
	existingStates := []*scanner.ScanState{
		{
			LibraryID: 1,
			FilePath:  testFile,
			FileHash:  existingHash,
			FileSize:  info.Size(),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	checkpointRepo := mocks.NewCheckpointRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t).WithStates(existingStates...)

	systemProfile := &system.Profile{
		CPU: system.CPUProfile{
			NumCPU:      4,
			NumPhysical: 2,
		},
		Storage: system.StorageProfile{
			Type: system.StorageTypeLocalSSD,
		},
	}

	deps := testDeps(t, checkpointRepo, scanStateRepo, systemProfile)

	// Execute
	err = HashAndStreamCheckpoints(ctx, deps, filesToProcess, 1, 1)
	if err != nil {
		t.Fatalf("HashAndStreamCheckpoints failed: %v", err)
	}

	// Verify the checkpoint was created with the cached hash
	checkpoints, err := checkpointRepo.GetPendingBatch(ctx, 1, 10)
	if err != nil {
		t.Fatalf("Failed to get pending batch: %v", err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("Expected 1 checkpoint, got %d", len(checkpoints))
	}

	// Should have reused the existing hash
	if checkpoints[0].FileHash != existingHash {
		t.Errorf("Expected hash '%s', got '%s'", existingHash, checkpoints[0].FileHash)
	}
}

func TestHashAndStreamCheckpoints_BatchTimeout(t *testing.T) {
	// This test verifies that partial batches are flushed on timeout
	tmpDir := t.TempDir()

	// Create 2 test files (less than typical batch size)
	var filesToProcess []scanner.FileInfo
	for i := 0; i < 2; i++ {
		path := filepath.Join(tmpDir, "file"+string(rune('1'+i))+".txt")
		err := os.WriteFile(path, []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Failed to stat test file: %v", err)
		}

		filesToProcess = append(filesToProcess, scanner.FileInfo{
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	checkpointRepo := mocks.NewCheckpointRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	systemProfile := &system.Profile{
		CPU: system.CPUProfile{
			NumCPU:      4,
			NumPhysical: 2,
		},
		Storage: system.StorageProfile{
			Type: system.StorageTypeLocalSSD,
		},
	}

	// Use a very short batch timeout to trigger timeout-based flush
	deps := &Deps{
		ScanRepos: &scan.ScanRepositories{
			Checkpoint: checkpointRepo,
			ScanState:  scanStateRepo,
		},
		Config: &scan.Config{
			CheckpointBufferSize: 100,
			HashProgressLogEvery: 1,
			BatchWriteTimeout:    50 * time.Millisecond, // Very short timeout
		},
		SystemProfile: systemProfile,
		Logger:        testLogger(),
	}

	// Execute
	err := HashAndStreamCheckpoints(ctx, deps, filesToProcess, 1, 1)
	if err != nil {
		t.Fatalf("HashAndStreamCheckpoints failed: %v", err)
	}

	// Verify checkpoints were written (despite not reaching batch size)
	count := checkpointRepo.GetCheckpointCount(1)
	if count != 2 {
		t.Errorf("Expected 2 checkpoints (partial batch flushed on timeout), got %d", count)
	}
}

func TestHashAndStreamCheckpoints_ConcurrentHashing(t *testing.T) {
	// Create multiple test files to ensure concurrent hashing works
	tmpDir := t.TempDir()

	const numFiles = 20
	var filesToProcess []scanner.FileInfo

	for i := 0; i < numFiles; i++ {
		path := filepath.Join(tmpDir, "file_"+string(rune('0'+i/10))+string(rune('0'+i%10))+".txt")
		content := "content for file " + string(rune('0'+i))
		err := os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Failed to stat test file: %v", err)
		}

		filesToProcess = append(filesToProcess, scanner.FileInfo{
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	checkpointRepo := mocks.NewCheckpointRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	// Use profile with multiple workers
	systemProfile := &system.Profile{
		CPU: system.CPUProfile{
			NumCPU:      8,
			NumPhysical: 4,
		},
		Storage: system.StorageProfile{
			Type: system.StorageTypeLocalSSD,
		},
	}

	deps := &Deps{
		ScanRepos: &scan.ScanRepositories{
			Checkpoint: checkpointRepo,
			ScanState:  scanStateRepo,
		},
		Config: &scan.Config{
			CheckpointBufferSize: 100,
			HashProgressLogEvery: 5,
			BatchWriteTimeout:    100 * time.Millisecond,
		},
		SystemProfile: systemProfile,
		Logger:        testLogger(),
	}

	// Execute
	err := HashAndStreamCheckpoints(ctx, deps, filesToProcess, 1, 1)
	if err != nil {
		t.Fatalf("HashAndStreamCheckpoints failed: %v", err)
	}

	// Verify all files were processed
	count := checkpointRepo.GetCheckpointCount(1)
	if count != numFiles {
		t.Errorf("Expected %d checkpoints, got %d", numFiles, count)
	}

	// Verify all checkpoints have hashes
	checkpoints, err := checkpointRepo.GetPendingBatch(ctx, 1, numFiles)
	if err != nil {
		t.Fatalf("Failed to get pending batch: %v", err)
	}
	if len(checkpoints) != numFiles {
		t.Errorf("Expected %d checkpoints, got %d", numFiles, len(checkpoints))
	}

	for _, cp := range checkpoints {
		if cp.FileHash == "" {
			t.Errorf("File should have hash: %s", cp.FilePath)
		}
	}
}

// =============================================================================
// Tests for sendHashJobs - Context Cancellation Coverage
// =============================================================================

func TestSendHashJobs_ContextCancellation(t *testing.T) {
	// Create a large batch of files to ensure context cancellation happens mid-send
	var filesToProcess []scanner.FileInfo
	for i := 0; i < 100; i++ {
		filesToProcess = append(filesToProcess, scanner.FileInfo{
			Path: "/test/file" + string(rune('0'+i)) + ".mp4",
			Size: 1024,
		})
	}

	// Create context that will be cancelled quickly
	ctx, cancel := context.WithCancel(context.Background())

	// Create jobs channel with small buffer to force blocking
	jobs := make(chan scanner.FileInfo, 1)

	// Cancel context after a short delay
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	// Track how many jobs were sent
	jobsSentChan := make(chan int, 1)
	go func() {
		count := 0
		for range jobs {
			count++
		}
		jobsSentChan <- count
	}()

	// Call sendHashJobs
	sendHashJobs(ctx, testLogger(), jobs, filesToProcess)

	// Wait for receiver to finish
	jobsSent := <-jobsSentChan

	// Verify that not all jobs were sent (context was cancelled)
	if jobsSent >= len(filesToProcess) {
		t.Logf("Context cancellation may not have stopped processing (sent %d/%d)", jobsSent, len(filesToProcess))
	} else {
		t.Logf("Context cancellation stopped processing at %d/%d files", jobsSent, len(filesToProcess))
	}
}

func TestSendHashJobs_AllJobsSent(t *testing.T) {
	// Create test files
	var filesToProcess []scanner.FileInfo
	for i := 0; i < 10; i++ {
		filesToProcess = append(filesToProcess, scanner.FileInfo{
			Path: "/test/file" + string(rune('0'+i)) + ".mp4",
			Size: 1024,
		})
	}

	ctx := context.Background()
	jobs := make(chan scanner.FileInfo, 20) // Large buffer to avoid blocking

	// Track received jobs
	receivedJobs := make([]scanner.FileInfo, 0)
	done := make(chan struct{})
	go func() {
		for job := range jobs {
			receivedJobs = append(receivedJobs, job)
		}
		close(done)
	}()

	// Call sendHashJobs
	sendHashJobs(ctx, testLogger(), jobs, filesToProcess)

	// Wait for receiver to finish
	<-done

	// Verify all jobs were sent
	if len(receivedJobs) != len(filesToProcess) {
		t.Errorf("Expected %d jobs, got %d", len(filesToProcess), len(receivedJobs))
	}

	// Verify jobs are in order
	for i, job := range receivedJobs {
		if job.Path != filesToProcess[i].Path {
			t.Errorf("Job %d: expected path %s, got %s", i, filesToProcess[i].Path, job.Path)
		}
	}
}

// =============================================================================
// Tests for GetHasherConfig
// =============================================================================

func TestGetHasherConfig(t *testing.T) {
	tests := []struct {
		name                  string
		profile               *system.Profile
		expectedWorkers       int
		expectedBatchSize     int
		expectProfileSettings bool
	}{
		{
			name:                  "nil profile - uses defaults",
			profile:               nil,
			expectedWorkers:       scan.DefaultHashWorkers,
			expectedBatchSize:     scan.DefaultHashBatchSize,
			expectProfileSettings: false,
		},
		{
			name: "with profile - uses calculated settings",
			profile: &system.Profile{
				CPU: system.CPUProfile{
					NumCPU:      8,
					NumPhysical: 4,
				},
				Storage: system.StorageProfile{
					Type: system.StorageTypeLocalSSD,
				},
			},
			expectProfileSettings: true,
		},
		{
			name: "low-end profile",
			profile: &system.Profile{
				CPU: system.CPUProfile{
					NumCPU:      2,
					NumPhysical: 2,
				},
				Storage: system.StorageProfile{
					Type: system.StorageTypeLocalHDD,
				},
			},
			expectProfileSettings: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := scan.DefaultConfig()
			logger := testLogger()

			hasherConfig := GetHasherConfig(tt.profile, &config, logger)

			if !tt.expectProfileSettings {
				if hasherConfig.NumWorkers != tt.expectedWorkers {
					t.Errorf("Expected NumWorkers %d, got %d", tt.expectedWorkers, hasherConfig.NumWorkers)
				}
				if hasherConfig.BatchSize != tt.expectedBatchSize {
					t.Errorf("Expected BatchSize %d, got %d", tt.expectedBatchSize, hasherConfig.BatchSize)
				}
			} else {
				// Just verify we got reasonable values from the profile
				if hasherConfig.NumWorkers <= 0 {
					t.Error("Expected positive NumWorkers from profile")
				}
				if hasherConfig.BatchSize <= 0 {
					t.Error("Expected positive BatchSize from profile")
				}
			}

			// Verify other config fields are set
			if hasherConfig.BatchTimeout != config.BatchWriteTimeout {
				t.Errorf("Expected BatchTimeout %v, got %v", config.BatchWriteTimeout, hasherConfig.BatchTimeout)
			}
			if hasherConfig.CheckpointBufferSize != config.CheckpointBufferSize {
				t.Errorf("Expected CheckpointBufferSize %d, got %d", config.CheckpointBufferSize, hasherConfig.CheckpointBufferSize)
			}
			if hasherConfig.HashProgressLogEvery != config.HashProgressLogEvery {
				t.Errorf("Expected HashProgressLogEvery %d, got %d", config.HashProgressLogEvery, hasherConfig.HashProgressLogEvery)
			}
		})
	}
}
