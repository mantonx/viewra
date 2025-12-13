package library

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestCollectImageHashesForMedia(t *testing.T) {
	tests := []struct {
		name       string
		mediaID    int64
		setupRepo  func(*mocks.ImageRepository)
		wantHashes []string
	}{
		{
			name:    "single image with hash",
			mediaID: 1,
			setupRepo: func(repo *mocks.ImageRepository) {
				hash := "abc123"
				repo.WithImages(
					&images.Image{
						ID:       1,
						MediaID:  intPtr(1),
						FileHash: &hash,
					},
				)
			},
			wantHashes: []string{"abc123"},
		},
		{
			name:    "multiple images with different hashes",
			mediaID: 1,
			setupRepo: func(repo *mocks.ImageRepository) {
				hash1 := "abc123"
				hash2 := "def456"
				hash3 := "ghi789"
				repo.WithImages(
					&images.Image{
						ID:       1,
						MediaID:  intPtr(1),
						FileHash: &hash1,
					},
					&images.Image{
						ID:       2,
						MediaID:  intPtr(1),
						FileHash: &hash2,
					},
					&images.Image{
						ID:       3,
						MediaID:  intPtr(1),
						FileHash: &hash3,
					},
				)
			},
			wantHashes: []string{"abc123", "def456", "ghi789"},
		},
		{
			name:    "multiple images with duplicate hashes - deduplication",
			mediaID: 1,
			setupRepo: func(repo *mocks.ImageRepository) {
				hash1 := "abc123"
				hash2 := "def456"
				repo.WithImages(
					&images.Image{
						ID:       1,
						MediaID:  intPtr(1),
						FileHash: &hash1,
					},
					&images.Image{
						ID:       2,
						MediaID:  intPtr(1),
						FileHash: &hash1, // Duplicate
					},
					&images.Image{
						ID:       3,
						MediaID:  intPtr(1),
						FileHash: &hash2,
					},
					&images.Image{
						ID:       4,
						MediaID:  intPtr(1),
						FileHash: &hash1, // Duplicate
					},
				)
			},
			// Should deduplicate to 2 unique hashes
			wantHashes: []string{"abc123", "def456"},
		},
		{
			name:    "images with nil hashes - ignored",
			mediaID: 1,
			setupRepo: func(repo *mocks.ImageRepository) {
				hash1 := "abc123"
				repo.WithImages(
					&images.Image{
						ID:       1,
						MediaID:  intPtr(1),
						FileHash: &hash1,
					},
					&images.Image{
						ID:       2,
						MediaID:  intPtr(1),
						FileHash: nil, // Nil hash
					},
					&images.Image{
						ID:       3,
						MediaID:  intPtr(1),
						FileHash: nil, // Nil hash
					},
				)
			},
			wantHashes: []string{"abc123"},
		},
		{
			name:    "images with empty string hashes - ignored",
			mediaID: 1,
			setupRepo: func(repo *mocks.ImageRepository) {
				hash1 := "abc123"
				emptyHash := ""
				repo.WithImages(
					&images.Image{
						ID:       1,
						MediaID:  intPtr(1),
						FileHash: &hash1,
					},
					&images.Image{
						ID:       2,
						MediaID:  intPtr(1),
						FileHash: &emptyHash, // Empty string
					},
				)
			},
			wantHashes: []string{"abc123"},
		},
		{
			name:    "no images for media",
			mediaID: 1,
			setupRepo: func(repo *mocks.ImageRepository) {
				// No images added
			},
			wantHashes: nil,
		},
		{
			name:    "images for different media ID",
			mediaID: 1,
			setupRepo: func(repo *mocks.ImageRepository) {
				hash := "abc123"
				repo.WithImages(
					&images.Image{
						ID:       1,
						MediaID:  intPtr(2), // Different media ID
						FileHash: &hash,
					},
				)
			},
			wantHashes: nil,
		},
		{
			name:    "error getting images",
			mediaID: 1,
			setupRepo: func(repo *mocks.ImageRepository) {
				repo.GetErr = errors.New("database error")
			},
			wantHashes: nil,
		},
		{
			name:    "nil image repository",
			mediaID: 1,
			setupRepo: func(repo *mocks.ImageRepository) {
				// Will test with nil repo
			},
			wantHashes: nil,
		},
		{
			name:    "all images with nil or empty hashes",
			mediaID: 1,
			setupRepo: func(repo *mocks.ImageRepository) {
				emptyHash := ""
				repo.WithImages(
					&images.Image{
						ID:       1,
						MediaID:  intPtr(1),
						FileHash: nil,
					},
					&images.Image{
						ID:       2,
						MediaID:  intPtr(1),
						FileHash: &emptyHash,
					},
					&images.Image{
						ID:       3,
						MediaID:  intPtr(1),
						FileHash: nil,
					},
				)
			},
			wantHashes: nil,
		},
		{
			name:    "large number of images with mixed hashes",
			mediaID: 1,
			setupRepo: func(repo *mocks.ImageRepository) {
				hash1 := "hash1"
				hash2 := "hash2"
				hash3 := "hash3"
				emptyHash := ""
				repo.WithImages(
					&images.Image{ID: 1, MediaID: intPtr(1), FileHash: &hash1},
					&images.Image{ID: 2, MediaID: intPtr(1), FileHash: &hash2},
					&images.Image{ID: 3, MediaID: intPtr(1), FileHash: &hash1}, // Dup
					&images.Image{ID: 4, MediaID: intPtr(1), FileHash: nil},
					&images.Image{ID: 5, MediaID: intPtr(1), FileHash: &hash3},
					&images.Image{ID: 6, MediaID: intPtr(1), FileHash: &emptyHash},
					&images.Image{ID: 7, MediaID: intPtr(1), FileHash: &hash2}, // Dup
					&images.Image{ID: 8, MediaID: intPtr(1), FileHash: &hash1}, // Dup
				)
			},
			wantHashes: []string{"hash1", "hash2", "hash3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock repository
			var imageRepo *mocks.ImageRepository
			if tt.name != "nil image repository" {
				imageRepo = mocks.NewImageRepository(t)
				if tt.setupRepo != nil {
					tt.setupRepo(imageRepo)
				}
			}

			// Execute
			var result []string
			if imageRepo != nil {
				result = CollectImageHashesForMedia(context.Background(), imageRepo, tt.mediaID)
			} else {
				result = CollectImageHashesForMedia(context.Background(), nil, tt.mediaID)
			}

			// Verify results
			if tt.wantHashes == nil {
				if result != nil && len(result) > 0 {
					t.Errorf("Expected nil or empty result, got %v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("Expected non-nil result, got nil")
				return
			}

			// Convert to maps for easier comparison (order doesn't matter)
			wantMap := make(map[string]bool)
			for _, h := range tt.wantHashes {
				wantMap[h] = true
			}

			gotMap := make(map[string]bool)
			for _, h := range result {
				gotMap[h] = true
			}

			// Check that all wanted hashes are present
			if len(wantMap) != len(gotMap) {
				t.Errorf("Expected %d unique hashes, got %d", len(wantMap), len(gotMap))
			}

			for hash := range wantMap {
				if !gotMap[hash] {
					t.Errorf("Expected hash %q not found in result", hash)
				}
			}

			for hash := range gotMap {
				if !wantMap[hash] {
					t.Errorf("Unexpected hash %q found in result", hash)
				}
			}
		})
	}
}

func TestCollectImageHashesForMedia_Integration(t *testing.T) {
	// Integration test verifying the function works with real repository patterns
	t.Run("realistic scenario - movie with multiple image types", func(t *testing.T) {
		repo := mocks.NewImageRepository(t)

		posterHash := "poster_hash_abc"
		backdropHash := "backdrop_hash_def"
		thumbnailHash := "thumbnail_hash_ghi"

		repo.WithImages(
			&images.Image{
				ID:       1,
				MediaID:  intPtr(100),
				FileHash: &posterHash,
			},
			&images.Image{
				ID:       2,
				MediaID:  intPtr(100),
				FileHash: &backdropHash,
			},
			&images.Image{
				ID:       3,
				MediaID:  intPtr(100),
				FileHash: &thumbnailHash,
			},
		)

		hashes := CollectImageHashesForMedia(context.Background(), repo, 100)

		if len(hashes) != 3 {
			t.Fatalf("Expected 3 hashes, got %d", len(hashes))
		}

		hashMap := make(map[string]bool)
		for _, h := range hashes {
			hashMap[h] = true
		}

		expectedHashes := []string{posterHash, backdropHash, thumbnailHash}
		for _, expected := range expectedHashes {
			if !hashMap[expected] {
				t.Errorf("Expected hash %q not found", expected)
			}
		}
	})

	t.Run("error from GetByMediaID returns nil", func(t *testing.T) {
		repo := mocks.NewImageRepository(t)
		repo.GetErr = sql.ErrConnDone

		hashes := CollectImageHashesForMedia(context.Background(), repo, 100)

		if hashes != nil {
			t.Errorf("Expected nil for error case, got %v", hashes)
		}
	})
}
