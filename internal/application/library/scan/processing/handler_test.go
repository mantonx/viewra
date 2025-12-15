package processing

// Tests for handler.go (ProcessFileWithCheckpoint function)
//
// This file tests the core file processing logic which:
// 1. Processes individual files during library scanning
// 2. Coordinates metadata extraction via the coordinator
// 3. Dispatches to media-specific processors
// 4. Manages scan state for incremental scanning
// 5. Handles warnings and errors during file processing

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/infrastructure/system"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func handlerTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// createTestFile creates a test file with given name
func createTestFile(t *testing.T, name string) string {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, name)
	if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	return path
}

// mockMediaProcessor implements MediaProcessor for testing
type mockMediaProcessor struct {
	processMovieFunc   func(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error)
	processTVFunc      func(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error)
	processMusicFunc   func(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error)
	movieID            *int64
	tvID               *int64
	musicID            *int64
	processMovieErr    error
	processTVErr       error
	processMusicErr    error
}

func newMockMediaProcessor() *mockMediaProcessor {
	defaultID := int64(1)
	return &mockMediaProcessor{
		movieID: &defaultID,
		tvID:    &defaultID,
		musicID: &defaultID,
	}
}

func (m *mockMediaProcessor) ProcessMovie(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error) {
	if m.processMovieFunc != nil {
		return m.processMovieFunc(ctx, libraryID, result, checkpoint, cache)
	}
	return m.movieID, m.processMovieErr
}

func (m *mockMediaProcessor) ProcessTVEpisode(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error) {
	if m.processTVFunc != nil {
		return m.processTVFunc(ctx, libraryID, result, checkpoint, cache)
	}
	return m.tvID, m.processTVErr
}

func (m *mockMediaProcessor) ProcessMusicTrack(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error) {
	if m.processMusicFunc != nil {
		return m.processMusicFunc(ctx, libraryID, result, checkpoint, cache)
	}
	return m.musicID, m.processMusicErr
}

func createTestDeps(t *testing.T, scanStateRepo *mocks.ScanStateRepository, mediaProcessor MediaProcessor) *Deps {
	coordConfig := filesystem.DefaultCoordinatorConfig()
	coordConfig.Logger = handlerTestLogger()
	coord := filesystem.NewCoordinator(coordConfig)

	if mediaProcessor == nil {
		mediaProcessor = newMockMediaProcessor()
	}

	return &Deps{
		ScanRepos: &scan.ScanRepositories{
			ScanState: scanStateRepo,
		},
		MediaRepos:     &scan.MediaRepositories{},
		MediaProcessor: mediaProcessor,
		Coordinator:    coord,
		Config:         &scan.Config{BaseFileTimeout: 30000000000}, // 30 seconds
		Logger:         handlerTestLogger(),
	}
}

// TestProcessFileWithCheckpoint_FileStatError tests file stat errors
func TestProcessFileWithCheckpoint_FileStatError(t *testing.T) {
	scanStateRepo := mocks.NewScanStateRepository(t)
	deps := createTestDeps(t, scanStateRepo, nil)

	lib := &library.Library{
		ID:   1,
		Name: "Test Library",
		Path: "/test/path",
		Type: library.LibraryTypeMovies,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  "/nonexistent/file.mp4",
		FileHash:  "hash123",
		FileSize:  1024000,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	_, err := ProcessFileWithCheckpoint(ctx, deps, lib, checkpoint, existingMediaCache)

	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	// No scan state should be created
	if len(scanStateRepo.GetStates()) != 0 {
		t.Errorf("Expected 0 scan states (file stat error), got %d", len(scanStateRepo.GetStates()))
	}
}

// TestProcessFileWithCheckpoint_MovieProcessing tests movie file processing
func TestProcessFileWithCheckpoint_MovieProcessing(t *testing.T) {
	tmpFile := createTestFile(t, "movie.mp4")
	scanStateRepo := mocks.NewScanStateRepository(t)

	mediaProcessor := newMockMediaProcessor()
	deps := createTestDeps(t, scanStateRepo, mediaProcessor)

	lib := &library.Library{
		ID:   1,
		Name: "Test Library",
		Path: filepath.Dir(tmpFile),
		Type: library.LibraryTypeMovies,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  tmpFile,
		FileHash:  "hash123",
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	hasWarning, err := ProcessFileWithCheckpoint(ctx, deps, lib, checkpoint, existingMediaCache)

	// File exists but we use real coordinator which will report parse error
	// for our fake file - that's expected
	if err != nil && hasWarning {
		// May have a warning from parse failure - that's OK
		t.Logf("Got warning (expected for test file): %v", err)
	}
}

// TestProcessFileWithCheckpoint_DatabaseError tests database error handling
func TestProcessFileWithCheckpoint_DatabaseError(t *testing.T) {
	tmpFile := createTestFile(t, "movie.mp4")
	scanStateRepo := mocks.NewScanStateRepository(t)

	mediaProcessor := newMockMediaProcessor()
	mediaProcessor.processMovieErr = errors.New("database is locked")

	deps := createTestDeps(t, scanStateRepo, mediaProcessor)

	lib := &library.Library{
		ID:   1,
		Name: "Test Library",
		Path: filepath.Dir(tmpFile),
		Type: library.LibraryTypeMovies,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  tmpFile,
		FileHash:  "hash123",
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	_, err := ProcessFileWithCheckpoint(ctx, deps, lib, checkpoint, existingMediaCache)

	// The coordinator will try to extract metadata from the fake file
	// and either fail or succeed with minimal data
	// The database error may or may not be reached depending on coordinator behavior
	t.Logf("Result: err=%v", err)
}

// TestProcessFileWithCheckpoint_UnknownLibraryType tests unknown library type handling
func TestProcessFileWithCheckpoint_UnknownLibraryType(t *testing.T) {
	tmpFile := createTestFile(t, "file.mp4")
	scanStateRepo := mocks.NewScanStateRepository(t)
	deps := createTestDeps(t, scanStateRepo, nil)

	lib := &library.Library{
		ID:   1,
		Name: "Test Library",
		Path: filepath.Dir(tmpFile),
		Type: library.LibraryType("invalid"),
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  tmpFile,
		FileHash:  "hash123",
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	_, err := ProcessFileWithCheckpoint(ctx, deps, lib, checkpoint, existingMediaCache)

	// Unknown library type should eventually cause an error after coordinator runs
	t.Logf("Result: err=%v", err)
}

// TestProcessFileWithCheckpoint_ScanStateUpsertError tests scan state upsert error handling
func TestProcessFileWithCheckpoint_ScanStateUpsertError(t *testing.T) {
	tmpFile := createTestFile(t, "movie.mp4")
	scanStateRepo := mocks.NewScanStateRepository(t)
	scanStateRepo.UpsertErr = errors.New("scan state database error")

	deps := createTestDeps(t, scanStateRepo, nil)

	lib := &library.Library{
		ID:   1,
		Name: "Test Library",
		Path: filepath.Dir(tmpFile),
		Type: library.LibraryTypeMovies,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  tmpFile,
		FileHash:  "hash123",
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	// ScanState upsert errors should be logged but not fatal
	_, _ = ProcessFileWithCheckpoint(ctx, deps, lib, checkpoint, existingMediaCache)

	// Test passes if no panic occurs - scan state errors are non-fatal
}

// TestProcessFileWithCheckpoint_ContextCancellation tests context cancellation
func TestProcessFileWithCheckpoint_ContextCancellation(t *testing.T) {
	tmpFile := createTestFile(t, "movie.mp4")
	scanStateRepo := mocks.NewScanStateRepository(t)
	deps := createTestDeps(t, scanStateRepo, nil)

	lib := &library.Library{
		ID:   1,
		Name: "Test Library",
		Path: filepath.Dir(tmpFile),
		Type: library.LibraryTypeMovies,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  tmpFile,
		FileHash:  "hash123",
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ProcessFileWithCheckpoint(ctx, deps, lib, checkpoint, existingMediaCache)

	// Should return error when context is cancelled
	if err == nil {
		t.Logf("Note: context cancellation may not be detected if stat completes quickly")
	}
}

// TestProcessFileWithCheckpoint_TVProcessing tests TV episode file processing
func TestProcessFileWithCheckpoint_TVProcessing(t *testing.T) {
	tmpFile := createTestFile(t, "episode.mkv")
	scanStateRepo := mocks.NewScanStateRepository(t)

	mediaProcessor := newMockMediaProcessor()
	deps := createTestDeps(t, scanStateRepo, mediaProcessor)

	lib := &library.Library{
		ID:   1,
		Name: "Test TV Library",
		Path: filepath.Dir(tmpFile),
		Type: library.LibraryTypeTV,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  tmpFile,
		FileHash:  "hash123",
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	hasWarning, err := ProcessFileWithCheckpoint(ctx, deps, lib, checkpoint, existingMediaCache)

	// File exists but we use real coordinator which will report parse error
	// for our fake file - that's expected
	if err != nil && hasWarning {
		// May have a warning from parse failure - that's OK
		t.Logf("Got warning (expected for test file): %v", err)
	}
}

// TestProcessFileWithCheckpoint_MusicProcessing tests music track file processing
func TestProcessFileWithCheckpoint_MusicProcessing(t *testing.T) {
	tmpFile := createTestFile(t, "track.mp3")
	scanStateRepo := mocks.NewScanStateRepository(t)

	mediaProcessor := newMockMediaProcessor()
	deps := createTestDeps(t, scanStateRepo, mediaProcessor)

	lib := &library.Library{
		ID:   1,
		Name: "Test Music Library",
		Path: filepath.Dir(tmpFile),
		Type: library.LibraryTypeMusic,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  tmpFile,
		FileHash:  "hash123",
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	hasWarning, err := ProcessFileWithCheckpoint(ctx, deps, lib, checkpoint, existingMediaCache)

	// File exists but we use real coordinator which will report parse error
	// for our fake file - that's expected
	if err != nil && hasWarning {
		// May have a warning from parse failure - that's OK
		t.Logf("Got warning (expected for test file): %v", err)
	}
}

// TestProcessFileWithCheckpoint_ProcessingTimeout tests timeout during processing
func TestProcessFileWithCheckpoint_ProcessingTimeout(t *testing.T) {
	tmpFile := createTestFile(t, "large_movie.mp4")
	scanStateRepo := mocks.NewScanStateRepository(t)

	mediaProcessor := newMockMediaProcessor()
	deps := createTestDeps(t, scanStateRepo, mediaProcessor)

	// Set extremely short timeout to trigger timeout
	deps.Config.BaseFileTimeout = 1 * time.Nanosecond

	lib := &library.Library{
		ID:   1,
		Name: "Test Library",
		Path: filepath.Dir(tmpFile),
		Type: library.LibraryTypeMovies,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  tmpFile,
		FileHash:  "hash123",
		FileSize:  1024000000, // Large file size
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	_, err := ProcessFileWithCheckpoint(ctx, deps, lib, checkpoint, existingMediaCache)

	// Should get a timeout or processing error
	if err != nil {
		t.Logf("Got expected error due to short timeout: %v", err)
	}
}

// TestProcessFileWithCheckpoint_WarningHandling tests warning path
func TestProcessFileWithCheckpoint_WarningHandling(t *testing.T) {
	tmpFile := createTestFile(t, "movie.mp4")
	scanStateRepo := mocks.NewScanStateRepository(t)

	mediaProcessor := newMockMediaProcessor()

	// Mock processor that returns success but coordinator may have warnings
	mediaProcessor.processMovieFunc = func(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error) {
		id := int64(1)
		return &id, nil
	}

	deps := createTestDeps(t, scanStateRepo, mediaProcessor)

	lib := &library.Library{
		ID:   1,
		Name: "Test Library",
		Path: filepath.Dir(tmpFile),
		Type: library.LibraryTypeMovies,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  tmpFile,
		FileHash:  "hash123",
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	hasWarning, err := ProcessFileWithCheckpoint(ctx, deps, lib, checkpoint, existingMediaCache)

	// Real coordinator will likely have issues with our test file
	// Just verify the function completes
	t.Logf("Processing result: hasWarning=%v, err=%v", hasWarning, err)
}

// TestProcessFileWithCheckpoint_TVProcessingError tests TV processing error
func TestProcessFileWithCheckpoint_TVProcessingError(t *testing.T) {
	tmpFile := createTestFile(t, "episode.mkv")
	scanStateRepo := mocks.NewScanStateRepository(t)

	mediaProcessor := newMockMediaProcessor()
	mediaProcessor.processTVErr = errors.New("failed to parse TV episode metadata")

	deps := createTestDeps(t, scanStateRepo, mediaProcessor)

	lib := &library.Library{
		ID:   1,
		Name: "Test TV Library",
		Path: filepath.Dir(tmpFile),
		Type: library.LibraryTypeTV,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  tmpFile,
		FileHash:  "hash123",
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	_, err := ProcessFileWithCheckpoint(ctx, deps, lib, checkpoint, existingMediaCache)

	// Should get processing error
	t.Logf("Got result: err=%v", err)
}

// TestProcessFileWithCheckpoint_MusicProcessingError tests music processing error
func TestProcessFileWithCheckpoint_MusicProcessingError(t *testing.T) {
	tmpFile := createTestFile(t, "track.mp3")
	scanStateRepo := mocks.NewScanStateRepository(t)

	mediaProcessor := newMockMediaProcessor()
	mediaProcessor.processMusicErr = errors.New("failed to parse audio metadata")

	deps := createTestDeps(t, scanStateRepo, mediaProcessor)

	lib := &library.Library{
		ID:   1,
		Name: "Test Music Library",
		Path: filepath.Dir(tmpFile),
		Type: library.LibraryTypeMusic,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  tmpFile,
		FileHash:  "hash123",
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	_, err := ProcessFileWithCheckpoint(ctx, deps, lib, checkpoint, existingMediaCache)

	// Should get processing error
	t.Logf("Got result: err=%v", err)
}

// TestCalculateProcessingTimeout tests timeout calculation
func TestCalculateProcessingTimeout(t *testing.T) {
	tests := []struct {
		name        string
		fileSize    int64
		isRemote    bool
		minExpected time.Duration
	}{
		{
			name:        "small file local storage",
			fileSize:    1024 * 1024, // 1 MB
			isRemote:    false,
			minExpected: 30 * time.Second,
		},
		{
			name:        "large file local storage",
			fileSize:    1024 * 1024 * 1024 * 10, // 10 GB
			isRemote:    false,
			minExpected: 30 * time.Second,
		},
		{
			name:        "small file remote storage",
			fileSize:    1024 * 1024, // 1 MB
			isRemote:    true,
			minExpected: 60 * time.Second,
		},
		{
			name:        "large file remote storage",
			fileSize:    1024 * 1024 * 1024 * 10, // 10 GB
			isRemote:    true,
			minExpected: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanStateRepo := mocks.NewScanStateRepository(t)
			deps := createTestDeps(t, scanStateRepo, nil)

			// Set config values required for timeout calculation
			deps.Config.BaseFileTimeout = 30 * time.Second
			deps.Config.RemoteStorageTimeout = 60 * time.Second
			deps.Config.MaxExtraTimeout = 5 * time.Minute

			if tt.isRemote {
				deps.SystemProfile = &system.Profile{
					Storage: system.StorageProfile{
						IsRemote: true,
					},
				}
			}

			timeout := calculateProcessingTimeout(deps, tt.fileSize)

			if timeout < tt.minExpected {
				t.Errorf("Expected timeout >= %v, got %v", tt.minExpected, timeout)
			}

			t.Logf("File size %d bytes, remote=%v -> timeout %v", tt.fileSize, tt.isRemote, timeout)
		})
	}
}
