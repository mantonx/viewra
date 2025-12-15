package library

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/mantonx/viewra/internal/application/common"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/testutil/mocks"
	_ "modernc.org/sqlite"
)

// TestLibraryService_Create_EdgeCases tests edge cases in Create to improve coverage
func TestLibraryService_Create_EdgeCases(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		req         CreateLibraryRequest
		setupMocks  func(*mocks.LibraryRepository)
		expectError bool
		errorMsg    string
	}{
		{
			name: "ExistsWithTx returns error",
			req: CreateLibraryRequest{
				Name: "Test Library",
				Path: tempDir,
				Type: "movies",
			},
			setupMocks: func(repo *mocks.LibraryRepository) {
				repo.ExistsErr = errors.New("database connection failed")
			},
			expectError: true,
			errorMsg:    "failed to check library existence",
		},
		{
			name: "CreateWithTx returns error",
			req: CreateLibraryRequest{
				Name: "Test Library",
				Path: tempDir,
				Type: "movies",
			},
			setupMocks: func(repo *mocks.LibraryRepository) {
				repo.CreateErr = errors.New("database insert failed")
			},
			expectError: true,
			errorMsg:    "failed to create library",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewLibraryRepository(t)
			if tt.setupMocks != nil {
				tt.setupMocks(repo)
			}

			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatalf("Failed to open test database: %v", err)
			}
			defer db.Close()

			txManager := common.NewTxManager(db)
			svc := NewLibraryService(repo, nil, nil, txManager, slog.New(slog.NewTextHandler(io.Discard, nil)))

			_, err = svc.Create(context.Background(), tt.req)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestLibraryService_Update_EdgeCases tests edge cases in Update to improve coverage
func TestLibraryService_Update_EdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	nonExistentPath := filepath.Join(tempDir, "does-not-exist")

	tests := []struct {
		name        string
		id          int64
		req         UpdateLibraryRequest
		setupMocks  func(*mocks.LibraryRepository)
		expectError bool
		errorMsg    string
	}{
		{
			name: "update with invalid type causes validation error",
			id:   1,
			req: UpdateLibraryRequest{
				Type: "invalid", // Invalid type causes validation error
			},
			setupMocks: func(repo *mocks.LibraryRepository) {
				repo.WithLibraries(&library.Library{
					ID:   1,
					Name: "Original",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
			expectError: true,
			errorMsg:    "validation failed",
		},
		{
			name: "update path to non-existent directory",
			id:   2,
			req: UpdateLibraryRequest{
				Path: nonExistentPath,
			},
			setupMocks: func(repo *mocks.LibraryRepository) {
				repo.WithLibraries(&library.Library{
					ID:   2,
					Name: "Test",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
			expectError: true,
			errorMsg:    "does not exist",
		},
		{
			name: "update path - validation passes for invalid type",
			id:   3,
			req: UpdateLibraryRequest{
				Type: "invalid_type",
			},
			setupMocks: func(repo *mocks.LibraryRepository) {
				repo.WithLibraries(&library.Library{
					ID:   3,
					Name: "Test",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
			expectError: true,
			errorMsg:    "validation failed",
		},
		{
			name: "update path to same path - should succeed",
			id:   4,
			req: UpdateLibraryRequest{
				Path: tempDir,
			},
			setupMocks: func(repo *mocks.LibraryRepository) {
				repo.WithLibraries(&library.Library{
					ID:   4,
					Name: "Library 4",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
			expectError: false,
		},
		{
			name: "update name only - no path check",
			id:   6,
			req: UpdateLibraryRequest{
				Name: "Updated Name",
			},
			setupMocks: func(repo *mocks.LibraryRepository) {
				repo.WithLibraries(&library.Library{
					ID:   6,
					Name: "Original Name",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
			expectError: false,
		},
		{
			name: "update type only - no path check",
			id:   7,
			req: UpdateLibraryRequest{
				Type: string(library.LibraryTypeTV),
			},
			setupMocks: func(repo *mocks.LibraryRepository) {
				repo.WithLibraries(&library.Library{
					ID:   7,
					Name: "Test",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewLibraryRepository(t)
			if tt.setupMocks != nil {
				tt.setupMocks(repo)
			}

			svc := NewLibraryService(repo, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
			_, err := svc.Update(context.Background(), tt.id, tt.req)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
