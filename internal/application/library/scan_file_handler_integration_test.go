package library

// Integration tests for scan_file_handler.go
//
// These tests actually call the real processFileWithCheckpoint function
// using real files and the real filesystem.Coordinator.
//
// This approach achieves higher test coverage than mocking, though it's
// slower and requires real file operations.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// createUseCaseWithRealCoordinator creates a use case with a real coordinator
func createUseCaseWithRealCoordinator(
	mediaRepo *mocks.MediaRepository,
	movieRepo *mocks.MovieRepository,
	tvRepo *mocks.TVRepository,
	musicRepo *mocks.MusicRepository,
	scanStateRepo *mocks.ScanStateRepository,
) *ScanLibraryUseCase {
	coordConfig := filesystem.DefaultCoordinatorConfig()
	coordConfig.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	coord := filesystem.NewCoordinator(coordConfig)

	return &ScanLibraryUseCase{
		mediaRepos: &MediaRepositories{
			Media: mediaRepo,
			Movie: movieRepo,
			TV:    tvRepo,
			Music: musicRepo,
		},
		scanRepos: &ScanRepositories{
			ScanState: scanStateRepo,
		},
		coordinator: coord,
		config:      DefaultScanConfig(),
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
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

// TestProcessFileWithCheckpoint_Integration_FileStatFailure tests file stat errors
func TestProcessFileWithCheckpoint_Integration_FileStatFailure(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)
	tvRepo := mocks.NewTVRepository(t)
	musicRepo := mocks.NewMusicRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	uc := createUseCaseWithRealCoordinator(mediaRepo, movieRepo, tvRepo, musicRepo, scanStateRepo)

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

	_, err := uc.processFileWithCheckpoint(ctx, lib, checkpoint, existingMediaCache)

	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to stat file") {
		t.Errorf("Expected 'failed to stat file' error, got: %v", err)
	}

	if len(scanStateRepo.GetStates()) != 0 {
		t.Errorf("Expected 0 scan states (file stat error), got %d", len(scanStateRepo.GetStates()))
	}
}

// TestProcessFileWithCheckpoint_Integration_MovieSuccess tests successful movie processing
func TestProcessFileWithCheckpoint_Integration_MovieSuccess(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)
	tvRepo := mocks.NewTVRepository(t)
	musicRepo := mocks.NewMusicRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	uc := createUseCaseWithRealCoordinator(mediaRepo, movieRepo, tvRepo, musicRepo, scanStateRepo)

	filePath := createTestFile(t, "The Matrix (1999).mp4")

	lib := &library.Library{
		ID:   1,
		Name: "Movies",
		Path: filepath.Dir(filePath),
		Type: library.LibraryTypeMovies,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  filePath,
		FileHash:  "hash123",
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	hasWarning, err := uc.processFileWithCheckpoint(ctx, lib, checkpoint, existingMediaCache)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Real coordinator will likely warn because it's not a real video file
	if !hasWarning {
		t.Log("Note: No warning from coordinator for dummy file (expected warning)")
	}

	// Verify scan state was created
	states := scanStateRepo.GetStates()
	if len(states) != 1 {
		t.Fatalf("Expected 1 scan state, got %d", len(states))
	}

	state := states[0]
	if state.LibraryID != 1 {
		t.Errorf("LibraryID = %d, want 1", state.LibraryID)
	}
	if state.FilePath != filePath {
		t.Errorf("FilePath = %q, want %q", state.FilePath, filePath)
	}
	if state.FileHash != "hash123" {
		t.Errorf("FileHash = %q, want hash123", state.FileHash)
	}
	if state.MediaID == nil {
		t.Error("Expected MediaID to be set")
	}
}

// TestProcessFileWithCheckpoint_Integration_TVSuccess tests successful TV episode processing
func TestProcessFileWithCheckpoint_Integration_TVSuccess(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)
	tvRepo := mocks.NewTVRepository(t)
	musicRepo := mocks.NewMusicRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	uc := createUseCaseWithRealCoordinator(mediaRepo, movieRepo, tvRepo, musicRepo, scanStateRepo)

	filePath := createTestFile(t, "Breaking Bad - S01E01 - Pilot.mp4")

	lib := &library.Library{
		ID:   1,
		Name: "TV Shows",
		Path: filepath.Dir(filePath),
		Type: library.LibraryTypeTV,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  filePath,
		FileHash:  "hash456",
		FileSize:  2048,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	hasWarning, err := uc.processFileWithCheckpoint(ctx, lib, checkpoint, existingMediaCache)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Will have warning from real coordinator
	_ = hasWarning

	// Verify scan state
	if len(scanStateRepo.GetStates()) != 1 {
		t.Errorf("Expected 1 scan state")
	}
}

// TestProcessFileWithCheckpoint_Integration_MusicSuccess tests successful music processing
func TestProcessFileWithCheckpoint_Integration_MusicSuccess(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)
	tvRepo := mocks.NewTVRepository(t)
	musicRepo := mocks.NewMusicRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	uc := createUseCaseWithRealCoordinator(mediaRepo, movieRepo, tvRepo, musicRepo, scanStateRepo)

	filePath := createTestFile(t, "01 - Song Title.mp3")

	lib := &library.Library{
		ID:   1,
		Name: "Music",
		Path: filepath.Dir(filePath),
		Type: library.LibraryTypeMusic,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  filePath,
		FileHash:  "hash789",
		FileSize:  5120,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	hasWarning, err := uc.processFileWithCheckpoint(ctx, lib, checkpoint, existingMediaCache)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	_ = hasWarning

	if len(scanStateRepo.GetStates()) != 1 {
		t.Error("Expected 1 scan state")
	}
}

// TestProcessFileWithCheckpoint_Integration_UnknownLibraryType tests unknown library type handling
func TestProcessFileWithCheckpoint_Integration_UnknownLibraryType(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)
	tvRepo := mocks.NewTVRepository(t)
	musicRepo := mocks.NewMusicRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	uc := createUseCaseWithRealCoordinator(mediaRepo, movieRepo, tvRepo, musicRepo, scanStateRepo)

	filePath := createTestFile(t, "test.mp4")

	lib := &library.Library{
		ID:   1,
		Name: "Test",
		Path: filepath.Dir(filePath),
		Type: library.LibraryType("invalid"),
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  filePath,
		FileHash:  "hash999",
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	_, err := uc.processFileWithCheckpoint(ctx, lib, checkpoint, existingMediaCache)

	if err == nil {
		t.Error("Expected error for unknown library type")
	}
	if !strings.Contains(err.Error(), "unknown library type") {
		t.Errorf("Expected 'unknown library type' error, got: %v", err)
	}

	if len(scanStateRepo.GetStates()) != 0 {
		t.Error("Expected 0 scan states for unknown library type")
	}
}

// TestProcessFileWithCheckpoint_Integration_DatabaseError tests database error handling
func TestProcessFileWithCheckpoint_Integration_DatabaseError(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)
	tvRepo := mocks.NewTVRepository(t)
	musicRepo := mocks.NewMusicRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	// Inject database error
	movieRepo.WithCreateError(errors.New("database is locked"))

	uc := createUseCaseWithRealCoordinator(mediaRepo, movieRepo, tvRepo, musicRepo, scanStateRepo)

	filePath := createTestFile(t, "Test Movie (2020).mp4")

	lib := &library.Library{
		ID:   1,
		Name: "Movies",
		Path: filepath.Dir(filePath),
		Type: library.LibraryTypeMovies,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  filePath,
		FileHash:  "hash111",
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	_, err := uc.processFileWithCheckpoint(ctx, lib, checkpoint, existingMediaCache)

	if err == nil {
		t.Error("Expected database error")
	}
	if !strings.Contains(err.Error(), "database is locked") {
		t.Errorf("Expected 'database is locked' error, got: %v", err)
	}

	if len(scanStateRepo.GetStates()) != 0 {
		t.Error("Expected 0 scan states after database error")
	}
}

// TestProcessFileWithCheckpoint_Integration_ScanStateUpsertFailure tests non-fatal scan state errors
func TestProcessFileWithCheckpoint_Integration_ScanStateUpsertFailure(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)
	tvRepo := mocks.NewTVRepository(t)
	musicRepo := mocks.NewMusicRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	// Inject scan state upsert error
	scanStateRepo.UpsertErr = errors.New("scan state database error")

	uc := createUseCaseWithRealCoordinator(mediaRepo, movieRepo, tvRepo, musicRepo, scanStateRepo)

	filePath := createTestFile(t, "Another Movie (2021).mp4")

	lib := &library.Library{
		ID:   1,
		Name: "Movies",
		Path: filepath.Dir(filePath),
		Type: library.LibraryTypeMovies,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  filePath,
		FileHash:  "hash222",
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	_, err := uc.processFileWithCheckpoint(ctx, lib, checkpoint, existingMediaCache)

	// Should NOT fail despite scan state error
	if err != nil {
		t.Errorf("Should not fail when scan state upsert fails, got: %v", err)
	}
}

// TestProcessFileWithCheckpoint_Integration_MetadataFields tests that all metadata fields are set correctly
func TestProcessFileWithCheckpoint_Integration_MetadataFields(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)
	tvRepo := mocks.NewTVRepository(t)
	musicRepo := mocks.NewMusicRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	uc := createUseCaseWithRealCoordinator(mediaRepo, movieRepo, tvRepo, musicRepo, scanStateRepo)

	filePath := createTestFile(t, "Metadata Test (2022).mp4")

	const testLibraryID int64 = 99
	const testScanJobID int64 = 12345
	const testFileHash = "abcdef123456"

	lib := &library.Library{
		ID:   testLibraryID,
		Name: "Movies",
		Path: filepath.Dir(filePath),
		Type: library.LibraryTypeMovies,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: testScanJobID,
		FilePath:  filePath,
		FileHash:  testFileHash,
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	_, err := uc.processFileWithCheckpoint(ctx, lib, checkpoint, existingMediaCache)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	states := scanStateRepo.GetStates()
	if len(states) != 1 {
		t.Fatalf("Expected 1 scan state, got %d", len(states))
	}

	state := states[0]

	// Verify all metadata fields
	if state.LibraryID != testLibraryID {
		t.Errorf("LibraryID = %d, want %d", state.LibraryID, testLibraryID)
	}
	if state.FilePath != filePath {
		t.Errorf("FilePath = %q, want %q", state.FilePath, filePath)
	}
	if state.FileHash != testFileHash {
		t.Errorf("FileHash = %q, want %q", state.FileHash, testFileHash)
	}
	if state.ScanJobID != testScanJobID {
		t.Errorf("ScanJobID = %d, want %d", state.ScanJobID, testScanJobID)
	}
	if state.FileSize <= 0 {
		t.Errorf("FileSize = %d, should be > 0", state.FileSize)
	}
	if state.FileMTime.IsZero() {
		t.Error("FileMTime should be set")
	}
	if state.LastScannedAt.IsZero() {
		t.Error("LastScannedAt should be set")
	}
	if state.MediaID == nil {
		t.Error("MediaID should be set after successful processing")
	}
}

// TestProcessFileWithCheckpoint_Integration_ExtensionNormalization tests extension lowercasing
func TestProcessFileWithCheckpoint_Integration_ExtensionNormalization(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)
	tvRepo := mocks.NewTVRepository(t)
	musicRepo := mocks.NewMusicRepository(t)
	scanStateRepo := mocks.NewScanStateRepository(t)

	uc := createUseCaseWithRealCoordinator(mediaRepo, movieRepo, tvRepo, musicRepo, scanStateRepo)

	// Create file with uppercase extension
	filePath := createTestFile(t, "Uppercase Test (2023).MP4")

	lib := &library.Library{
		ID:   1,
		Name: "Movies",
		Path: filepath.Dir(filePath),
		Type: library.LibraryTypeMovies,
	}

	checkpoint := &scanner.ScanCheckpoint{
		ID:        1,
		ScanJobID: 100,
		FilePath:  filePath,
		FileHash:  "hash333",
		FileSize:  1024,
	}

	existingMediaCache := &sync.Map{}
	ctx := context.Background()

	_, err := uc.processFileWithCheckpoint(ctx, lib, checkpoint, existingMediaCache)

	// Should not error despite uppercase extension
	if err != nil {
		t.Errorf("Should handle uppercase extension, got error: %v", err)
	}

	// Verify file was processed
	if len(scanStateRepo.GetStates()) == 0 {
		t.Error("Expected file to be processed despite uppercase extension")
	}
}
