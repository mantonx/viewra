package library

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	appImages "github.com/mantonx/viewra/internal/application/images"
	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// Mock image cleanup executor for scan cleanup tests
type mockImageCleanupExecutor struct {
	cleanCacheForHashesFunc  func(ctx context.Context, hashes []string) error
	cleanCacheForHashesCalls [][]string // Track calls
}

func (m *mockImageCleanupExecutor) CleanOrphanedImages(ctx context.Context) (*appImages.CleanupStats, error) {
	return nil, nil
}

func (m *mockImageCleanupExecutor) CleanCacheForHashes(ctx context.Context, hashes []string) error {
	m.cleanCacheForHashesCalls = append(m.cleanCacheForHashesCalls, hashes)
	if m.cleanCacheForHashesFunc != nil {
		return m.cleanCacheForHashesFunc(ctx, hashes)
	}
	return nil
}

func TestScanLibraryUseCase_cleanupStaleMedia(t *testing.T) {
	tests := []struct {
		name         string
		libraryID    int64
		foundFiles   map[string]bool
		setupRepos   func(*mocks.MediaRepository, *mocks.ImageRepository)
		setupCleanup func(*mockImageCleanupExecutor)
		checkRepos   func(*testing.T, *mocks.MediaRepository, *mocks.ImageRepository)
		checkCleanup func(*testing.T, *mockImageCleanupExecutor)
	}{
		{
			name:      "no stale media - all files found",
			libraryID: 1,
			foundFiles: map[string]bool{
				"/movies/movie1.mp4": true,
				"/movies/movie2.mp4": true,
				"/movies/movie3.mp4": true,
			},
			setupRepos: func(mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				mediaRepo.WithMedia(
					&media.Media{ID: 1, LibraryID: 1, FilePath: "/movies/movie1.mp4"},
					&media.Media{ID: 2, LibraryID: 1, FilePath: "/movies/movie2.mp4"},
					&media.Media{ID: 3, LibraryID: 1, FilePath: "/movies/movie3.mp4"},
				)
			},
			setupCleanup: func(cleanup *mockImageCleanupExecutor) {},
			checkRepos: func(t *testing.T, mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				allMedia, _ := mediaRepo.ListByLibrary(context.Background(), 1)
				if len(allMedia) != 3 {
					t.Errorf("Expected 3 media items, got %d", len(allMedia))
				}
			},
			checkCleanup: func(t *testing.T, cleanup *mockImageCleanupExecutor) {
				if len(cleanup.cleanCacheForHashesCalls) > 0 {
					t.Error("CleanCacheForHashes should not be called when no files are stale")
				}
			},
		},
		{
			name:      "some stale media - under 10% threshold",
			libraryID: 1,
			foundFiles: map[string]bool{
				"/movies/movie1.mp4": true,
				"/movies/movie2.mp4": true,
				"/movies/movie3.mp4": true,
				"/movies/movie4.mp4": true,
				"/movies/movie5.mp4": true,
				"/movies/movie6.mp4": true,
				"/movies/movie7.mp4": true,
				"/movies/movie8.mp4": true,
				"/movies/movie9.mp4": true,
				// movie10.mp4 is missing (stale)
			},
			setupRepos: func(mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				mediaRepo.WithMedia(
					&media.Media{ID: 1, LibraryID: 1, FilePath: "/movies/movie1.mp4"},
					&media.Media{ID: 2, LibraryID: 1, FilePath: "/movies/movie2.mp4"},
					&media.Media{ID: 3, LibraryID: 1, FilePath: "/movies/movie3.mp4"},
					&media.Media{ID: 4, LibraryID: 1, FilePath: "/movies/movie4.mp4"},
					&media.Media{ID: 5, LibraryID: 1, FilePath: "/movies/movie5.mp4"},
					&media.Media{ID: 6, LibraryID: 1, FilePath: "/movies/movie6.mp4"},
					&media.Media{ID: 7, LibraryID: 1, FilePath: "/movies/movie7.mp4"},
					&media.Media{ID: 8, LibraryID: 1, FilePath: "/movies/movie8.mp4"},
					&media.Media{ID: 9, LibraryID: 1, FilePath: "/movies/movie9.mp4"},
					&media.Media{ID: 10, LibraryID: 1, FilePath: "/movies/movie10.mp4"}, // Stale
				)

				// Add images for the stale media
				hash1 := "hash1"
				hash2 := "hash2"
				imageRepo.WithImages(
					&images.Image{ID: 1, MediaID: intPtr(10), FileHash: &hash1},
					&images.Image{ID: 2, MediaID: intPtr(10), FileHash: &hash2},
				)
			},
			setupCleanup: func(cleanup *mockImageCleanupExecutor) {},
			checkRepos: func(t *testing.T, mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				allMedia, _ := mediaRepo.ListByLibrary(context.Background(), 1)
				// Should delete 1 stale media (10% threshold)
				if len(allMedia) != 9 {
					t.Errorf("Expected 9 media items after cleanup, got %d", len(allMedia))
				}

				// Verify the stale media was deleted
				_, err := mediaRepo.GetByID(context.Background(), 10)
				if err == nil {
					t.Error("Expected stale media ID 10 to be deleted")
				}
			},
			checkCleanup: func(t *testing.T, cleanup *mockImageCleanupExecutor) {
				if len(cleanup.cleanCacheForHashesCalls) != 1 {
					t.Errorf("Expected 1 call to CleanCacheForHashes, got %d", len(cleanup.cleanCacheForHashesCalls))
					return
				}

				// Check that the correct hashes were passed
				hashes := cleanup.cleanCacheForHashesCalls[0]
				if len(hashes) != 2 {
					t.Errorf("Expected 2 hashes, got %d", len(hashes))
				}
			},
		},
		{
			name:      "too many stale files - refuse to cleanup (>10%)",
			libraryID: 1,
			foundFiles: map[string]bool{
				"/movies/movie1.mp4": true,
				"/movies/movie2.mp4": true,
				// 3-10 are missing (80% stale)
			},
			setupRepos: func(mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				mediaRepo.WithMedia(
					&media.Media{ID: 1, LibraryID: 1, FilePath: "/movies/movie1.mp4"},
					&media.Media{ID: 2, LibraryID: 1, FilePath: "/movies/movie2.mp4"},
					&media.Media{ID: 3, LibraryID: 1, FilePath: "/movies/movie3.mp4"},
					&media.Media{ID: 4, LibraryID: 1, FilePath: "/movies/movie4.mp4"},
					&media.Media{ID: 5, LibraryID: 1, FilePath: "/movies/movie5.mp4"},
					&media.Media{ID: 6, LibraryID: 1, FilePath: "/movies/movie6.mp4"},
					&media.Media{ID: 7, LibraryID: 1, FilePath: "/movies/movie7.mp4"},
					&media.Media{ID: 8, LibraryID: 1, FilePath: "/movies/movie8.mp4"},
					&media.Media{ID: 9, LibraryID: 1, FilePath: "/movies/movie9.mp4"},
					&media.Media{ID: 10, LibraryID: 1, FilePath: "/movies/movie10.mp4"},
				)
			},
			setupCleanup: func(cleanup *mockImageCleanupExecutor) {},
			checkRepos: func(t *testing.T, mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				allMedia, _ := mediaRepo.ListByLibrary(context.Background(), 1)
				// Should NOT delete any media (safety threshold)
				if len(allMedia) != 10 {
					t.Errorf("Expected 10 media items (no cleanup), got %d", len(allMedia))
				}
			},
			checkCleanup: func(t *testing.T, cleanup *mockImageCleanupExecutor) {
				if len(cleanup.cleanCacheForHashesCalls) > 0 {
					t.Error("CleanCacheForHashes should not be called when >10% is stale")
				}
			},
		},
		{
			name:      "exactly 10% stale - should cleanup (boundary test)",
			libraryID: 1,
			foundFiles: map[string]bool{
				"/movies/movie1.mp4": true,
				"/movies/movie2.mp4": true,
				"/movies/movie3.mp4": true,
				"/movies/movie4.mp4": true,
				"/movies/movie5.mp4": true,
				"/movies/movie6.mp4": true,
				"/movies/movie7.mp4": true,
				"/movies/movie8.mp4": true,
				"/movies/movie9.mp4": true,
				// movie10.mp4 is missing (exactly 10%)
			},
			setupRepos: func(mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				mediaRepo.WithMedia(
					&media.Media{ID: 1, LibraryID: 1, FilePath: "/movies/movie1.mp4"},
					&media.Media{ID: 2, LibraryID: 1, FilePath: "/movies/movie2.mp4"},
					&media.Media{ID: 3, LibraryID: 1, FilePath: "/movies/movie3.mp4"},
					&media.Media{ID: 4, LibraryID: 1, FilePath: "/movies/movie4.mp4"},
					&media.Media{ID: 5, LibraryID: 1, FilePath: "/movies/movie5.mp4"},
					&media.Media{ID: 6, LibraryID: 1, FilePath: "/movies/movie6.mp4"},
					&media.Media{ID: 7, LibraryID: 1, FilePath: "/movies/movie7.mp4"},
					&media.Media{ID: 8, LibraryID: 1, FilePath: "/movies/movie8.mp4"},
					&media.Media{ID: 9, LibraryID: 1, FilePath: "/movies/movie9.mp4"},
					&media.Media{ID: 10, LibraryID: 1, FilePath: "/movies/movie10.mp4"},
				)
				// Add an image for the stale media
				hash := "stale_image_hash"
				imageRepo.WithImages(
					&images.Image{ID: 1, MediaID: intPtr(10), FileHash: &hash},
				)
			},
			setupCleanup: func(cleanup *mockImageCleanupExecutor) {},
			checkRepos: func(t *testing.T, mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				allMedia, _ := mediaRepo.ListByLibrary(context.Background(), 1)
				// Should delete exactly 10% (1 file) because 10.0 is not > 10.0
				if len(allMedia) != 9 {
					t.Errorf("Expected 9 media items after cleanup, got %d", len(allMedia))
				}
			},
			checkCleanup: func(t *testing.T, cleanup *mockImageCleanupExecutor) {
				if len(cleanup.cleanCacheForHashesCalls) != 1 {
					t.Errorf("Expected 1 call to CleanCacheForHashes, got %d", len(cleanup.cleanCacheForHashesCalls))
				}
			},
		},
		{
			name:       "empty library",
			libraryID:  1,
			foundFiles: map[string]bool{},
			setupRepos: func(mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				// No media items
			},
			setupCleanup: func(cleanup *mockImageCleanupExecutor) {},
			checkRepos: func(t *testing.T, mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				allMedia, _ := mediaRepo.ListByLibrary(context.Background(), 1)
				if len(allMedia) != 0 {
					t.Errorf("Expected 0 media items, got %d", len(allMedia))
				}
			},
			checkCleanup: func(t *testing.T, cleanup *mockImageCleanupExecutor) {
				if len(cleanup.cleanCacheForHashesCalls) > 0 {
					t.Error("CleanCacheForHashes should not be called for empty library")
				}
			},
		},
		{
			name:      "error listing media - no cleanup",
			libraryID: 1,
			foundFiles: map[string]bool{
				"/movies/movie1.mp4": true,
			},
			setupRepos: func(mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				mediaRepo.ListErr = errors.New("database error")
			},
			setupCleanup: func(cleanup *mockImageCleanupExecutor) {},
			checkRepos: func(t *testing.T, mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				// No verification needed - error prevents cleanup
			},
			checkCleanup: func(t *testing.T, cleanup *mockImageCleanupExecutor) {
				if len(cleanup.cleanCacheForHashesCalls) > 0 {
					t.Error("CleanCacheForHashes should not be called on error")
				}
			},
		},
		{
			name:      "image cleanup error - still deletes media",
			libraryID: 1,
			foundFiles: map[string]bool{
				"/movies/movie1.mp4":  true,
				"/movies/movie2.mp4":  true,
				"/movies/movie3.mp4":  true,
				"/movies/movie4.mp4":  true,
				"/movies/movie5.mp4":  true,
				"/movies/movie6.mp4":  true,
				"/movies/movie7.mp4":  true,
				"/movies/movie8.mp4":  true,
				"/movies/movie9.mp4":  true,
				"/movies/movie10.mp4": true,
				// movie11.mp4 is missing (< 10% stale)
			},
			setupRepos: func(mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				mediaRepo.WithMedia(
					&media.Media{ID: 1, LibraryID: 1, FilePath: "/movies/movie1.mp4"},
					&media.Media{ID: 2, LibraryID: 1, FilePath: "/movies/movie2.mp4"},
					&media.Media{ID: 3, LibraryID: 1, FilePath: "/movies/movie3.mp4"},
					&media.Media{ID: 4, LibraryID: 1, FilePath: "/movies/movie4.mp4"},
					&media.Media{ID: 5, LibraryID: 1, FilePath: "/movies/movie5.mp4"},
					&media.Media{ID: 6, LibraryID: 1, FilePath: "/movies/movie6.mp4"},
					&media.Media{ID: 7, LibraryID: 1, FilePath: "/movies/movie7.mp4"},
					&media.Media{ID: 8, LibraryID: 1, FilePath: "/movies/movie8.mp4"},
					&media.Media{ID: 9, LibraryID: 1, FilePath: "/movies/movie9.mp4"},
					&media.Media{ID: 10, LibraryID: 1, FilePath: "/movies/movie10.mp4"},
					&media.Media{ID: 11, LibraryID: 1, FilePath: "/movies/movie11.mp4"},
				)
				hash := "hash1"
				imageRepo.WithImages(
					&images.Image{ID: 1, MediaID: intPtr(11), FileHash: &hash},
				)
			},
			setupCleanup: func(cleanup *mockImageCleanupExecutor) {
				cleanup.cleanCacheForHashesFunc = func(ctx context.Context, hashes []string) error {
					return errors.New("cleanup error")
				}
			},
			checkRepos: func(t *testing.T, mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				allMedia, _ := mediaRepo.ListByLibrary(context.Background(), 1)
				// Media should still be deleted even if image cleanup fails
				if len(allMedia) != 10 {
					t.Errorf("Expected 10 media items after cleanup, got %d", len(allMedia))
				}
			},
			checkCleanup: func(t *testing.T, cleanup *mockImageCleanupExecutor) {
				if len(cleanup.cleanCacheForHashesCalls) != 1 {
					t.Errorf("Expected 1 call to CleanCacheForHashes, got %d", len(cleanup.cleanCacheForHashesCalls))
				}
			},
		},
		{
			name:      "nil image cleanup - still works",
			libraryID: 1,
			foundFiles: map[string]bool{
				"/movies/movie1.mp4":  true,
				"/movies/movie2.mp4":  true,
				"/movies/movie3.mp4":  true,
				"/movies/movie4.mp4":  true,
				"/movies/movie5.mp4":  true,
				"/movies/movie6.mp4":  true,
				"/movies/movie7.mp4":  true,
				"/movies/movie8.mp4":  true,
				"/movies/movie9.mp4":  true,
				"/movies/movie10.mp4": true,
				// movie11.mp4 is missing
			},
			setupRepos: func(mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				mediaRepo.WithMedia(
					&media.Media{ID: 1, LibraryID: 1, FilePath: "/movies/movie1.mp4"},
					&media.Media{ID: 2, LibraryID: 1, FilePath: "/movies/movie2.mp4"},
					&media.Media{ID: 3, LibraryID: 1, FilePath: "/movies/movie3.mp4"},
					&media.Media{ID: 4, LibraryID: 1, FilePath: "/movies/movie4.mp4"},
					&media.Media{ID: 5, LibraryID: 1, FilePath: "/movies/movie5.mp4"},
					&media.Media{ID: 6, LibraryID: 1, FilePath: "/movies/movie6.mp4"},
					&media.Media{ID: 7, LibraryID: 1, FilePath: "/movies/movie7.mp4"},
					&media.Media{ID: 8, LibraryID: 1, FilePath: "/movies/movie8.mp4"},
					&media.Media{ID: 9, LibraryID: 1, FilePath: "/movies/movie9.mp4"},
					&media.Media{ID: 10, LibraryID: 1, FilePath: "/movies/movie10.mp4"},
					&media.Media{ID: 11, LibraryID: 1, FilePath: "/movies/movie11.mp4"},
				)
			},
			setupCleanup: func(cleanup *mockImageCleanupExecutor) {
				// Will test with nil cleanup
			},
			checkRepos: func(t *testing.T, mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				allMedia, _ := mediaRepo.ListByLibrary(context.Background(), 1)
				if len(allMedia) != 10 {
					t.Errorf("Expected 10 media items after cleanup, got %d", len(allMedia))
				}
			},
			checkCleanup: func(t *testing.T, cleanup *mockImageCleanupExecutor) {
				// Not checked - cleanup will be nil
			},
		},
		{
			name:      "multiple stale with duplicate image hashes",
			libraryID: 1,
			foundFiles: map[string]bool{
				"/movies/movie1.mp4":  true,
				"/movies/movie2.mp4":  true,
				"/movies/movie3.mp4":  true,
				"/movies/movie4.mp4":  true,
				"/movies/movie5.mp4":  true,
				"/movies/movie6.mp4":  true,
				"/movies/movie7.mp4":  true,
				"/movies/movie8.mp4":  true,
				"/movies/movie9.mp4":  true,
				"/movies/movie10.mp4": true,
				"/movies/movie11.mp4": true,
				"/movies/movie12.mp4": true,
				"/movies/movie13.mp4": true,
				"/movies/movie14.mp4": true,
				"/movies/movie15.mp4": true,
				"/movies/movie16.mp4": true,
				"/movies/movie17.mp4": true,
				"/movies/movie18.mp4": true,
				"/movies/movie19.mp4": true,
				"/movies/movie20.mp4": true,
				// movie21.mp4 and movie22.mp4 are missing (< 10% stale: 2/22 = 9%)
			},
			setupRepos: func(mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				mediaRepo.WithMedia(
					&media.Media{ID: 1, LibraryID: 1, FilePath: "/movies/movie1.mp4"},
					&media.Media{ID: 2, LibraryID: 1, FilePath: "/movies/movie2.mp4"},
					&media.Media{ID: 3, LibraryID: 1, FilePath: "/movies/movie3.mp4"},
					&media.Media{ID: 4, LibraryID: 1, FilePath: "/movies/movie4.mp4"},
					&media.Media{ID: 5, LibraryID: 1, FilePath: "/movies/movie5.mp4"},
					&media.Media{ID: 6, LibraryID: 1, FilePath: "/movies/movie6.mp4"},
					&media.Media{ID: 7, LibraryID: 1, FilePath: "/movies/movie7.mp4"},
					&media.Media{ID: 8, LibraryID: 1, FilePath: "/movies/movie8.mp4"},
					&media.Media{ID: 9, LibraryID: 1, FilePath: "/movies/movie9.mp4"},
					&media.Media{ID: 10, LibraryID: 1, FilePath: "/movies/movie10.mp4"},
					&media.Media{ID: 11, LibraryID: 1, FilePath: "/movies/movie11.mp4"},
					&media.Media{ID: 12, LibraryID: 1, FilePath: "/movies/movie12.mp4"},
					&media.Media{ID: 13, LibraryID: 1, FilePath: "/movies/movie13.mp4"},
					&media.Media{ID: 14, LibraryID: 1, FilePath: "/movies/movie14.mp4"},
					&media.Media{ID: 15, LibraryID: 1, FilePath: "/movies/movie15.mp4"},
					&media.Media{ID: 16, LibraryID: 1, FilePath: "/movies/movie16.mp4"},
					&media.Media{ID: 17, LibraryID: 1, FilePath: "/movies/movie17.mp4"},
					&media.Media{ID: 18, LibraryID: 1, FilePath: "/movies/movie18.mp4"},
					&media.Media{ID: 19, LibraryID: 1, FilePath: "/movies/movie19.mp4"},
					&media.Media{ID: 20, LibraryID: 1, FilePath: "/movies/movie20.mp4"},
					&media.Media{ID: 21, LibraryID: 1, FilePath: "/movies/movie21.mp4"},
					&media.Media{ID: 22, LibraryID: 1, FilePath: "/movies/movie22.mp4"},
				)
				// Same hash used by multiple images
				hash1 := "duplicate_hash"
				hash2 := "unique_hash"
				imageRepo.WithImages(
					&images.Image{ID: 1, MediaID: intPtr(21), FileHash: &hash1},
					&images.Image{ID: 2, MediaID: intPtr(21), FileHash: &hash1}, // Duplicate
					&images.Image{ID: 3, MediaID: intPtr(22), FileHash: &hash1}, // Duplicate
					&images.Image{ID: 4, MediaID: intPtr(22), FileHash: &hash2},
				)
			},
			setupCleanup: func(cleanup *mockImageCleanupExecutor) {},
			checkRepos: func(t *testing.T, mediaRepo *mocks.MediaRepository, imageRepo *mocks.ImageRepository) {
				allMedia, _ := mediaRepo.ListByLibrary(context.Background(), 1)
				if len(allMedia) != 20 {
					t.Errorf("Expected 20 media items after cleanup, got %d", len(allMedia))
				}
			},
			checkCleanup: func(t *testing.T, cleanup *mockImageCleanupExecutor) {
				if len(cleanup.cleanCacheForHashesCalls) != 1 {
					t.Errorf("Expected 1 call to CleanCacheForHashes, got %d", len(cleanup.cleanCacheForHashesCalls))
					return
				}

				// Should have unique hashes only
				hashes := cleanup.cleanCacheForHashesCalls[0]
				uniqueHashes := make(map[string]bool)
				for _, h := range hashes {
					uniqueHashes[h] = true
				}
				if len(uniqueHashes) != 2 {
					t.Errorf("Expected 2 unique hashes, got %d", len(uniqueHashes))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mediaRepo := mocks.NewMediaRepository(t)
			imageRepo := mocks.NewImageRepository(t)
			cleanup := &mockImageCleanupExecutor{}

			// Setup
			if tt.setupRepos != nil {
				tt.setupRepos(mediaRepo, imageRepo)
			}
			if tt.setupCleanup != nil {
				tt.setupCleanup(cleanup)
			}

			// Create use case
			var imageCleanup ImageCleanupExecutor = cleanup
			// Test nil cleanup for one test case
			if tt.name == "nil image cleanup - still works" {
				imageCleanup = nil
			}

			uc := &ScanLibraryUseCase{
				mediaRepos: &MediaRepositories{
					Media: mediaRepo,
				},
				imageRepo:    imageRepo,
				imageCleanup: imageCleanup,
				logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			// Execute
			uc.cleanupStaleMedia(context.Background(), tt.libraryID, tt.foundFiles)

			// Check results
			if tt.checkRepos != nil {
				tt.checkRepos(t, mediaRepo, imageRepo)
			}
			if tt.checkCleanup != nil && imageCleanup != nil {
				tt.checkCleanup(t, cleanup)
			}
		})
	}
}

// Helper function for creating int pointers
func intPtr(i int) *int {
	return &i
}
