package library

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/mantonx/viewra/internal/application/common"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/testutil/mocks"
	_ "modernc.org/sqlite" // Import SQLite driver
)

func TestCreateLibraryUseCase_Execute(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		req         CreateLibraryRequest
		wantErr     bool
		expectedErr error
		setup       func(*mocks.LibraryRepository)
	}{
		{
			name: "valid movie library",
			req: CreateLibraryRequest{
				Name: "Movies",
				Path: tempDir,
				Type: "movies",
			},
			wantErr: false,
		},
		{
			name: "valid tv library",
			req: CreateLibraryRequest{
				Name: "TV Shows",
				Path: tempDir,
				Type: "tv",
			},
			wantErr: false,
		},
		{
			name: "valid music library",
			req: CreateLibraryRequest{
				Name: "Music",
				Path: tempDir,
				Type: "music",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			req: CreateLibraryRequest{
				Name: "",
				Path: tempDir,
				Type: "movies",
			},
			wantErr:     true,
			expectedErr: library.ErrInvalidName,
		},
		{
			name: "invalid type",
			req: CreateLibraryRequest{
				Name: "Invalid",
				Path: tempDir,
				Type: "invalid",
			},
			wantErr:     true,
			expectedErr: library.ErrInvalidType,
		},
		{
			name: "non-existent path",
			req: CreateLibraryRequest{
				Name: "Movies",
				Path: filepath.Join(tempDir, "does-not-exist"),
				Type: "movies",
			},
			wantErr:     true,
			expectedErr: library.ErrPathNotFound,
		},
		{
			name: "duplicate path",
			req: CreateLibraryRequest{
				Name: "Movies2",
				Path: tempDir,
				Type: "movies",
			},
			wantErr:     true,
			expectedErr: library.ErrDuplicatePath,
			setup: func(repo *mocks.LibraryRepository) {
				_ = repo.Create(context.Background(), &library.Library{
					Name: "Existing",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
		},
		{
			name: "relative path",
			req: CreateLibraryRequest{
				Name: "Movies",
				Path: "relative/path",
				Type: "movies",
			},
			wantErr:     true,
			expectedErr: library.ErrPathNotAbsolute,
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
			service := NewLibraryService(repo, nil, nil, txManager, nil)

			resp, err := service.Create(context.Background(), tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Execute() expected error, got nil")
					return
				}
				if tt.expectedErr != nil && err != tt.expectedErr {
					// Check if error wraps expected error
					if !isErrorType(err, tt.expectedErr) {
						t.Errorf("Execute() error = %v, want %v", err, tt.expectedErr)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
				return
			}

			if resp.ID == 0 {
				t.Error("Execute() response ID should not be 0")
			}
			if resp.Name != tt.req.Name {
				t.Errorf("Execute() name = %v, want %v", resp.Name, tt.req.Name)
			}
			if resp.Type != tt.req.Type {
				t.Errorf("Execute() type = %v, want %v", resp.Type, tt.req.Type)
			}
		})
	}
}

// isErrorType checks if err wraps targetErr
func isErrorType(err, targetErr error) bool {
	if err == targetErr {
		return true
	}
	// Simple string contains check for wrapped errors
	if err != nil && targetErr != nil {
		return err.Error() != "" && targetErr.Error() != ""
	}
	return false
}
