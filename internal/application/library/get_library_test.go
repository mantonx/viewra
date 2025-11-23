package library

import (
	"context"
	"testing"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestGetLibraryUseCase_Execute(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		libraryID   int64
		wantErr     bool
		expectedErr error
		setup       func(*mocks.LibraryRepository)
	}{
		{
			name:      "get existing library",
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
			name:        "get non-existent library",
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

			service := NewLibraryService(repo, nil, nil, nil, nil)

			resp, err := service.Get(context.Background(), tt.libraryID)

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

			if resp.ID != tt.libraryID {
				t.Errorf("Execute() ID = %v, want %v", resp.ID, tt.libraryID)
			}
		})
	}
}
