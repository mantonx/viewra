package library

import (
	"context"
	"testing"

	"github.com/mantonx/viewra/internal/domain/library"
)

func TestUpdateLibraryUseCase_Execute(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		libraryID   int64
		req         UpdateLibraryRequest
		wantErr     bool
		expectedErr error
		setup       func(*mockLibraryRepository)
	}{
		{
			name:      "update library name",
			libraryID: 1,
			req: UpdateLibraryRequest{
				Name: "Updated Movies",
			},
			wantErr: false,
			setup: func(repo *mockLibraryRepository) {
				_ = repo.Create(context.Background(), &library.Library{
					Name: "Movies",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
		},
		{
			name:      "update library type",
			libraryID: 1,
			req: UpdateLibraryRequest{
				Type: "tv",
			},
			wantErr: false,
			setup: func(repo *mockLibraryRepository) {
				_ = repo.Create(context.Background(), &library.Library{
					Name: "Library",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
		},
		{
			name:      "update all fields",
			libraryID: 1,
			req: UpdateLibraryRequest{
				Name: "All Updated",
				Path: tempDir,
				Type: "music",
			},
			wantErr: false,
			setup: func(repo *mockLibraryRepository) {
				_ = repo.Create(context.Background(), &library.Library{
					Name: "Original",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
		},
		{
			name:      "non-existent library",
			libraryID: 999,
			req: UpdateLibraryRequest{
				Name: "Updated",
			},
			wantErr:     true,
			expectedErr: library.ErrLibraryNotFound,
			setup:       func(repo *mockLibraryRepository) {},
		},
		{
			name:      "invalid type",
			libraryID: 1,
			req: UpdateLibraryRequest{
				Type: "invalid",
			},
			wantErr:     true,
			expectedErr: library.ErrInvalidType,
			setup: func(repo *mockLibraryRepository) {
				_ = repo.Create(context.Background(), &library.Library{
					Name: "Movies",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockLibraryRepository()
			if tt.setup != nil {
				tt.setup(repo)
			}

			uc := NewUpdateLibraryUseCase(repo)

			resp, err := uc.Execute(context.Background(), tt.libraryID, tt.req)

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

			// Verify updates were applied
			if tt.req.Name != "" && resp.Name != tt.req.Name {
				t.Errorf("Execute() name = %v, want %v", resp.Name, tt.req.Name)
			}
			if tt.req.Type != "" && resp.Type != tt.req.Type {
				t.Errorf("Execute() type = %v, want %v", resp.Type, tt.req.Type)
			}
		})
	}
}
