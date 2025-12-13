package library

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/mantonx/viewra/internal/application/common"
	appImages "github.com/mantonx/viewra/internal/application/images"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/testutil/mocks"
	_ "modernc.org/sqlite" // Import SQLite driver
)

// mockImageCleanup is a mock implementation of ImageCleanupExecutor for testing
type mockImageCleanup struct {
	cleanOrphanedImagesCalled bool
	cleanOrphanedImagesFunc   func(ctx context.Context) (*appImages.CleanupStats, error)
	cleanCacheForHashesCalls  [][]string
}

func (m *mockImageCleanup) CleanOrphanedImages(ctx context.Context) (*appImages.CleanupStats, error) {
	m.cleanOrphanedImagesCalled = true
	if m.cleanOrphanedImagesFunc != nil {
		return m.cleanOrphanedImagesFunc(ctx)
	}
	return &appImages.CleanupStats{
		OrphanedFiles: 5,
		DeletedFiles:  5,
		BytesFreed:    1024000,
	}, nil
}

func (m *mockImageCleanup) CleanCacheForHashes(ctx context.Context, hashes []string) error {
	m.cleanCacheForHashesCalls = append(m.cleanCacheForHashesCalls, hashes)
	return nil
}

func TestDeleteLibraryUseCase_Execute(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		libraryID   int64
		wantErr     bool
		expectedErr error
		setup       func(*mocks.LibraryRepository)
	}{
		{
			name:      "delete existing library",
			libraryID: 1,
			wantErr:   false,
			setup: func(repo *mocks.LibraryRepository) {
				_ = repo.Create(context.Background(), &library.Library{
					Name: "Movies",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
		},
		{
			name:        "delete non-existent library",
			libraryID:   999,
			wantErr:     true,
			expectedErr: library.ErrLibraryNotFound,
			setup:       func(repo *mocks.LibraryRepository) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewLibraryRepository(t)
			if tt.setup != nil {
				tt.setup(repo)
			}

			// Create an in-memory SQLite database for testing transactions
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatalf("Failed to open test database: %v", err)
			}
			defer db.Close()

			txManager := common.NewTxManager(db)
			// Create mock image repo and cleanup - can be nil for these tests
			service := NewLibraryService(repo, nil, nil, txManager, nil)

			err = service.Delete(context.Background(), tt.libraryID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Execute() expected error, got nil")
					return
				}
				return
			}

			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
				return
			}

			// Verify library was deleted
			_, err = repo.GetByID(context.Background(), tt.libraryID)
			if err != library.ErrLibraryNotFound {
				t.Error("Execute() library should be deleted but still exists")
			}
		})
	}
}

func TestLibraryService_Delete(t *testing.T) {
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name               string
		libraryID          int64
		setupRepo          func(*mocks.LibraryRepository)
		setupImageCleanup  func(*mockImageCleanup)
		wantErr            bool
		checkErr           func(*testing.T, error)
		checkRepo          func(*testing.T, *mocks.LibraryRepository)
		checkImageCleanup  func(*testing.T, *mockImageCleanup)
	}{
		{
			name:      "successful deletion with image cleanup",
			libraryID: 1,
			setupRepo: func(repo *mocks.LibraryRepository) {
				_ = repo.Create(context.Background(), &library.Library{
					ID:   1,
					Name: "Movies",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
			setupImageCleanup: func(cleanup *mockImageCleanup) {
				cleanup.cleanOrphanedImagesFunc = func(ctx context.Context) (*appImages.CleanupStats, error) {
					return &appImages.CleanupStats{
						OrphanedFiles: 10,
						DeletedFiles:  10,
						BytesFreed:    5000000,
					}, nil
				}
			},
			wantErr: false,
			checkRepo: func(t *testing.T, repo *mocks.LibraryRepository) {
				t.Helper()
				repo.AssertLibraryCount(0)
			},
			checkImageCleanup: func(t *testing.T, cleanup *mockImageCleanup) {
				t.Helper()
				if !cleanup.cleanOrphanedImagesCalled {
					t.Error("Expected CleanOrphanedImages to be called")
				}
			},
		},
		{
			name:      "successful deletion without image cleanup executor (nil)",
			libraryID: 1,
			setupRepo: func(repo *mocks.LibraryRepository) {
				_ = repo.Create(context.Background(), &library.Library{
					ID:   1,
					Name: "Movies",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
			setupImageCleanup: nil, // Will pass nil cleanup executor
			wantErr:           false,
			checkRepo: func(t *testing.T, repo *mocks.LibraryRepository) {
				t.Helper()
				repo.AssertLibraryCount(0)
			},
			checkImageCleanup: nil,
		},
		{
			name:      "library not found - GetByIDWithTx fails",
			libraryID: 999,
			setupRepo: func(repo *mocks.LibraryRepository) {
				// No library created
			},
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				if !errors.Is(err, library.ErrLibraryNotFound) && err.Error() == "" {
					t.Errorf("Expected library not found error, got: %v", err)
				}
			},
			checkRepo: func(t *testing.T, repo *mocks.LibraryRepository) {
				t.Helper()
				repo.AssertLibraryCount(0)
			},
		},
		{
			name:      "GetByIDWithTx error - database error",
			libraryID: 1,
			setupRepo: func(repo *mocks.LibraryRepository) {
				repo.GetByIDWithTxErr = errors.New("database connection error")
			},
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				if err == nil {
					t.Error("Expected error, got nil")
				}
			},
		},
		{
			name:      "DeleteWithTx error - database error",
			libraryID: 1,
			setupRepo: func(repo *mocks.LibraryRepository) {
				_ = repo.Create(context.Background(), &library.Library{
					ID:   1,
					Name: "Movies",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
				repo.DeleteWithTxErr = errors.New("delete constraint violation")
			},
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				if err == nil || err.Error() == "" {
					t.Error("Expected database error, got nil or empty")
				}
			},
			checkRepo: func(t *testing.T, repo *mocks.LibraryRepository) {
				t.Helper()
				// Library should still exist due to rollback
				repo.AssertLibraryCount(1)
			},
		},
		{
			name:      "image cleanup fails - deletion still succeeds (best effort)",
			libraryID: 1,
			setupRepo: func(repo *mocks.LibraryRepository) {
				_ = repo.Create(context.Background(), &library.Library{
					ID:   1,
					Name: "Movies",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
			setupImageCleanup: func(cleanup *mockImageCleanup) {
				cleanup.cleanOrphanedImagesFunc = func(ctx context.Context) (*appImages.CleanupStats, error) {
					return nil, errors.New("cache cleanup failed")
				}
			},
			wantErr: false, // Cleanup failure doesn't fail the deletion
			checkRepo: func(t *testing.T, repo *mocks.LibraryRepository) {
				t.Helper()
				repo.AssertLibraryCount(0) // Library should still be deleted
			},
			checkImageCleanup: func(t *testing.T, cleanup *mockImageCleanup) {
				t.Helper()
				if !cleanup.cleanOrphanedImagesCalled {
					t.Error("Expected CleanOrphanedImages to be called even if it fails")
				}
			},
		},
		{
			name:      "multiple libraries - delete specific one",
			libraryID: 2,
			setupRepo: func(repo *mocks.LibraryRepository) {
				_ = repo.Create(context.Background(), &library.Library{
					ID:   1,
					Name: "Movies",
					Path: tempDir + "/movies",
					Type: library.LibraryTypeMovies,
				})
				_ = repo.Create(context.Background(), &library.Library{
					ID:   2,
					Name: "TV Shows",
					Path: tempDir + "/tv",
					Type: library.LibraryTypeTV,
				})
				_ = repo.Create(context.Background(), &library.Library{
					ID:   3,
					Name: "Music",
					Path: tempDir + "/music",
					Type: library.LibraryTypeMusic,
				})
			},
			wantErr: false,
			checkRepo: func(t *testing.T, repo *mocks.LibraryRepository) {
				t.Helper()
				repo.AssertLibraryCount(2) // Two libraries should remain

				// Verify specific library was deleted
				_, err := repo.GetByID(context.Background(), 2)
				if err != library.ErrLibraryNotFound {
					t.Error("Library 2 should be deleted")
				}

				// Verify others still exist
				_, err = repo.GetByID(context.Background(), 1)
				if err != nil {
					t.Error("Library 1 should still exist")
				}
				_, err = repo.GetByID(context.Background(), 3)
				if err != nil {
					t.Error("Library 3 should still exist")
				}
			},
		},
		{
			name:      "delete library with ID 0 - invalid ID",
			libraryID: 0,
			setupRepo: func(repo *mocks.LibraryRepository) {
				// No library with ID 0 should exist
			},
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				if err == nil {
					t.Error("Expected error for invalid library ID")
				}
			},
		},
		{
			name:      "delete library with negative ID",
			libraryID: -1,
			setupRepo: func(repo *mocks.LibraryRepository) {
				// No library with negative ID should exist
			},
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				if err == nil {
					t.Error("Expected error for negative library ID")
				}
			},
		},
		{
			name:      "image cleanup returns nil stats - should not crash",
			libraryID: 1,
			setupRepo: func(repo *mocks.LibraryRepository) {
				_ = repo.Create(context.Background(), &library.Library{
					ID:   1,
					Name: "Movies",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
			setupImageCleanup: func(cleanup *mockImageCleanup) {
				cleanup.cleanOrphanedImagesFunc = func(ctx context.Context) (*appImages.CleanupStats, error) {
					return nil, nil // Return nil stats (edge case)
				}
			},
			wantErr: false,
			checkRepo: func(t *testing.T, repo *mocks.LibraryRepository) {
				t.Helper()
				repo.AssertLibraryCount(0)
			},
			checkImageCleanup: func(t *testing.T, cleanup *mockImageCleanup) {
				t.Helper()
				if !cleanup.cleanOrphanedImagesCalled {
					t.Error("Expected CleanOrphanedImages to be called")
				}
			},
		},
		{
			name:      "image cleanup returns zero stats",
			libraryID: 1,
			setupRepo: func(repo *mocks.LibraryRepository) {
				_ = repo.Create(context.Background(), &library.Library{
					ID:   1,
					Name: "Movies",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
			setupImageCleanup: func(cleanup *mockImageCleanup) {
				cleanup.cleanOrphanedImagesFunc = func(ctx context.Context) (*appImages.CleanupStats, error) {
					return &appImages.CleanupStats{
						OrphanedFiles: 0,
						DeletedFiles:  0,
						BytesFreed:    0,
					}, nil
				}
			},
			wantErr: false,
			checkRepo: func(t *testing.T, repo *mocks.LibraryRepository) {
				t.Helper()
				repo.AssertLibraryCount(0)
			},
			checkImageCleanup: func(t *testing.T, cleanup *mockImageCleanup) {
				t.Helper()
				if !cleanup.cleanOrphanedImagesCalled {
					t.Error("Expected CleanOrphanedImages to be called")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock repository
			repo := mocks.NewLibraryRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			// Create mock image cleanup
			var imageCleanup *mockImageCleanup
			if tt.setupImageCleanup != nil {
				imageCleanup = &mockImageCleanup{}
				tt.setupImageCleanup(imageCleanup)
			}

			// Create an in-memory SQLite database for testing transactions
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatalf("Failed to open test database: %v", err)
			}
			defer db.Close()

			txManager := common.NewTxManager(db)

			// Create service with or without image cleanup
			var service *LibraryService
			if imageCleanup != nil {
				service = NewLibraryService(repo, nil, imageCleanup, txManager, logger)
			} else {
				service = NewLibraryService(repo, nil, nil, txManager, logger)
			}

			// Execute delete
			err = service.Delete(context.Background(), tt.libraryID)

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			// Check repository state
			if tt.checkRepo != nil {
				tt.checkRepo(t, repo)
			}

			// Check image cleanup state
			if tt.checkImageCleanup != nil && imageCleanup != nil {
				tt.checkImageCleanup(t, imageCleanup)
			}
		})
	}
}

func TestLibraryService_Delete_TransactionRollback(t *testing.T) {
	// Test that transaction rollback works correctly when deletion fails
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("transaction rollback on delete failure", func(t *testing.T) {
		repo := mocks.NewLibraryRepository(t)

		// Create a library
		_ = repo.Create(context.Background(), &library.Library{
			ID:   1,
			Name: "Movies",
			Path: tempDir,
			Type: library.LibraryTypeMovies,
		})

		// Inject error in DeleteWithTx
		repo.DeleteWithTxErr = errors.New("foreign key constraint")

		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("Failed to open test database: %v", err)
		}
		defer db.Close()

		txManager := common.NewTxManager(db)
		service := NewLibraryService(repo, nil, nil, txManager, logger)

		// Attempt to delete - should fail
		err = service.Delete(context.Background(), 1)
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		// Library should still exist (transaction rolled back)
		lib, err := repo.GetByID(context.Background(), 1)
		if err != nil {
			t.Fatalf("Library should still exist after rollback: %v", err)
		}
		if lib.ID != 1 {
			t.Error("Wrong library returned")
		}
	})

	t.Run("transaction rollback on GetByID failure", func(t *testing.T) {
		repo := mocks.NewLibraryRepository(t)

		// Inject error in GetByIDWithTx (library check fails)
		repo.GetByIDWithTxErr = errors.New("connection timeout")

		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("Failed to open test database: %v", err)
		}
		defer db.Close()

		txManager := common.NewTxManager(db)
		service := NewLibraryService(repo, nil, nil, txManager, logger)

		// Attempt to delete - should fail early
		err = service.Delete(context.Background(), 1)
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		// Repository should be unchanged
		repo.AssertLibraryCount(0)
	})
}

func TestLibraryService_Delete_ImageCleanupBehavior(t *testing.T) {
	// Test various image cleanup scenarios
	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("cleanup called after transaction commits", func(t *testing.T) {
		repo := mocks.NewLibraryRepository(t)
		_ = repo.Create(context.Background(), &library.Library{
			ID:   1,
			Name: "Movies",
			Path: tempDir,
			Type: library.LibraryTypeMovies,
		})

		cleanup := &mockImageCleanup{
			cleanOrphanedImagesFunc: func(ctx context.Context) (*appImages.CleanupStats, error) {
				// At this point, the library should already be deleted
				// (cleanup happens after commit)
				_, err := repo.GetByID(ctx, 1)
				if err != library.ErrLibraryNotFound {
					return nil, errors.New("library should be deleted before cleanup runs")
				}
				return &appImages.CleanupStats{OrphanedFiles: 3, DeletedFiles: 3}, nil
			},
		}

		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("Failed to open test database: %v", err)
		}
		defer db.Close()

		txManager := common.NewTxManager(db)
		service := NewLibraryService(repo, nil, cleanup, txManager, logger)

		err = service.Delete(context.Background(), 1)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if !cleanup.cleanOrphanedImagesCalled {
			t.Error("Cleanup should have been called")
		}
	})

	t.Run("cleanup not called on transaction failure", func(t *testing.T) {
		repo := mocks.NewLibraryRepository(t)
		_ = repo.Create(context.Background(), &library.Library{
			ID:   1,
			Name: "Movies",
			Path: tempDir,
			Type: library.LibraryTypeMovies,
		})

		// Inject deletion error
		repo.DeleteWithTxErr = errors.New("constraint violation")

		cleanup := &mockImageCleanup{}

		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("Failed to open test database: %v", err)
		}
		defer db.Close()

		txManager := common.NewTxManager(db)
		service := NewLibraryService(repo, nil, cleanup, txManager, logger)

		err = service.Delete(context.Background(), 1)
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		// Cleanup should NOT be called since transaction failed
		if cleanup.cleanOrphanedImagesCalled {
			t.Error("Cleanup should not be called when transaction fails")
		}

		// Library should still exist
		repo.AssertLibraryCount(1)
	})

	t.Run("cleanup failure doesn't affect deletion success", func(t *testing.T) {
		repo := mocks.NewLibraryRepository(t)
		_ = repo.Create(context.Background(), &library.Library{
			ID:   1,
			Name: "Movies",
			Path: tempDir,
			Type: library.LibraryTypeMovies,
		})

		cleanup := &mockImageCleanup{
			cleanOrphanedImagesFunc: func(ctx context.Context) (*appImages.CleanupStats, error) {
				return nil, errors.New("disk full - cannot delete cache")
			},
		}

		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("Failed to open test database: %v", err)
		}
		defer db.Close()

		txManager := common.NewTxManager(db)
		service := NewLibraryService(repo, nil, cleanup, txManager, logger)

		err = service.Delete(context.Background(), 1)
		if err != nil {
			t.Errorf("Deletion should succeed even if cleanup fails: %v", err)
		}

		// Library should be deleted
		repo.AssertLibraryCount(0)

		// Cleanup was attempted
		if !cleanup.cleanOrphanedImagesCalled {
			t.Error("Cleanup should have been attempted")
		}
	})
}
