package library

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestLibraryService_List(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(*mocks.LibraryRepository)
		expectError bool
		expectCount int
	}{
		{
			name: "successfully lists libraries",
			setupMocks: func(repo *mocks.LibraryRepository) {
				repo.WithLibraries(
					&library.Library{ID: 1, Name: "Movies", Path: "/movies"},
					&library.Library{ID: 2, Name: "TV Shows", Path: "/tv"},
				)
			},
			expectError: false,
			expectCount: 2,
		},
		{
			name: "returns empty list when no libraries",
			setupMocks: func(repo *mocks.LibraryRepository) {
				// No libraries added
			},
			expectError: false,
			expectCount: 0,
		},
		{
			name: "returns error when repository fails",
			setupMocks: func(repo *mocks.LibraryRepository) {
				repo.ListErr = errors.New("database error")
			},
			expectError: true,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewLibraryRepository(t)
			if tt.setupMocks != nil {
				tt.setupMocks(repo)
			}

			svc := NewLibraryService(repo, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
			result, err := svc.List(context.Background())

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if len(result.Libraries) != tt.expectCount {
				t.Errorf("expected %d libraries, got %d", tt.expectCount, len(result.Libraries))
			}
		})
	}
}

func TestLibraryService_Update(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		id          int64
		req         UpdateLibraryRequest
		setupMocks  func(*mocks.LibraryRepository)
		expectError bool
		expectName  string
	}{
		{
			name: "successfully updates library name",
			id:   1,
			req: UpdateLibraryRequest{
				Name: "Updated Name",
			},
			setupMocks: func(repo *mocks.LibraryRepository) {
				repo.WithLibraries(&library.Library{
					ID:   1,
					Name: "Original Name",
					Path: "/movies",
					Type: library.LibraryTypeMovies,
				})
			},
			expectError: false,
			expectName:  "Updated Name",
		},
		{
			name: "error when updating to non-existent path",
			id:   2,
			req: UpdateLibraryRequest{
				Path: "/nonexistent/path",
			},
			setupMocks: func(repo *mocks.LibraryRepository) {
				repo.WithLibraries(&library.Library{
					ID:   2,
					Name: "Test",
					Path: "/old/path",
					Type: library.LibraryTypeMovies,
				})
			},
			expectError: true,
		},
		{
			name: "successfully updates library type",
			id:   3,
			req: UpdateLibraryRequest{
				Type: string(library.LibraryTypeTV),
			},
			setupMocks: func(repo *mocks.LibraryRepository) {
				repo.WithLibraries(&library.Library{
					ID:   3,
					Name: "Test",
					Path: "/media",
					Type: library.LibraryTypeMovies,
				})
			},
			expectError: false,
			expectName:  "Test",
		},
		{
			name: "error when library not found",
			id:   999,
			req: UpdateLibraryRequest{
				Name: "New Name",
			},
			setupMocks: func(repo *mocks.LibraryRepository) {
				// No library with ID 999
			},
			expectError: true,
		},
		{
			name: "error when update fails",
			id:   4,
			req: UpdateLibraryRequest{
				Name: "New Name",
			},
			setupMocks: func(repo *mocks.LibraryRepository) {
				repo.WithLibraries(&library.Library{
					ID:   4,
					Name: "Test",
					Path: "/media",
					Type: library.LibraryTypeMovies,
				})
				repo.UpdateErr = errors.New("database error")
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewLibraryRepository(t)
			if tt.setupMocks != nil {
				tt.setupMocks(repo)
			}

			svc := NewLibraryService(repo, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
			result, err := svc.Update(ctx, tt.id, tt.req)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result.Name != tt.expectName {
				t.Errorf("expected name %q, got %q", tt.expectName, result.Name)
			}
		})
	}
}

func TestLibraryService_NewLibraryService(t *testing.T) {
	repo := mocks.NewLibraryRepository(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewLibraryService(repo, nil, nil, nil, logger)

	if svc == nil {
		t.Error("expected non-nil service")
	}
	if svc.repo != repo {
		t.Error("repo not set correctly")
	}
	if svc.logger != logger {
		t.Error("logger not set correctly")
	}
}
