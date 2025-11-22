package library

import (
	"context"
	"database/sql"
	"testing"

    _ "modernc.org/sqlite" // Import SQLite driver
	"github.com/mantonx/viewra/internal/application/common"
	"github.com/mantonx/viewra/internal/domain/library"
)

func TestDeleteLibraryUseCase_Execute(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		libraryID   int64
		wantErr     bool
		expectedErr error
		setup       func(*mockLibraryRepository)
	}{
		{
			name:      "delete existing library",
			libraryID: 1,
			wantErr:   false,
			setup: func(repo *mockLibraryRepository) {
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
			setup:       func(repo *mockLibraryRepository) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockLibraryRepository()
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
			uc := NewDeleteLibraryUseCase(repo, nil, nil, txManager)

			err = uc.Execute(context.Background(), tt.libraryID)

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
