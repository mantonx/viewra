package media

import (
	"context"
	"testing"
	"time"

	"github.com/viewra/viewra/internal/domain/media"
)

func TestListMediaUseCase_Execute(t *testing.T) {
	tests := []struct {
		name          string
		libraryID     int64
		expectedCount int
		wantErr       bool
		setup         func(*mockMediaRepository)
	}{
		{
			name:          "empty library",
			libraryID:     1,
			expectedCount: 0,
			wantErr:       false,
			setup:         func(repo *mockMediaRepository) {},
		},
		{
			name:          "single media item",
			libraryID:     1,
			expectedCount: 1,
			wantErr:       false,
			setup: func(repo *mockMediaRepository) {
				_ = repo.Create(context.Background(), &media.Media{
					LibraryID: 1,
					Title:     "Movie 1",
					FilePath:  "movies/movie1.mp4",
					FileSize:  1000000,
					Duration:  7200,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				})
			},
		},
		{
			name:          "multiple media items",
			libraryID:     1,
			expectedCount: 3,
			wantErr:       false,
			setup: func(repo *mockMediaRepository) {
				for i := 1; i <= 3; i++ {
					_ = repo.Create(context.Background(), &media.Media{
						LibraryID: 1,
						Title:     "Movie",
						FilePath:  "movies/movie.mp4",
						FileSize:  1000000,
						Duration:  7200,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					})
				}
			},
		},
		{
			name:          "filter by library",
			libraryID:     1,
			expectedCount: 2,
			wantErr:       false,
			setup: func(repo *mockMediaRepository) {
				// Add items for library 1
				for i := 1; i <= 2; i++ {
					_ = repo.Create(context.Background(), &media.Media{
						LibraryID: 1,
						Title:     "Movie",
						FilePath:  "movies/movie.mp4",
						FileSize:  1000000,
						Duration:  7200,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					})
				}
				// Add items for library 2 (should be filtered out)
				_ = repo.Create(context.Background(), &media.Media{
					LibraryID: 2,
					Title:     "Other Movie",
					FilePath:  "movies/other.mp4",
					FileSize:  1000000,
					Duration:  7200,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockMediaRepository()
			if tt.setup != nil {
				tt.setup(repo)
			}

			uc := NewListMediaUseCase(repo)

			resp, err := uc.Execute(context.Background(), tt.libraryID)

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

			if resp.Total != tt.expectedCount {
				t.Errorf("Execute() total = %v, want %v", resp.Total, tt.expectedCount)
			}

			if len(resp.Media) != tt.expectedCount {
				t.Errorf("Execute() media count = %v, want %v", len(resp.Media), tt.expectedCount)
			}
		})
	}
}

func TestListMediaUseCase_ExecuteByType(t *testing.T) {
	repo := newMockMediaRepository()

	// Add some test data
	_ = repo.Create(context.Background(), &media.Media{
		LibraryID: 1,
		Title:     "Test Movie",
		FilePath:  "movies/test.mp4",
		FileSize:  1000000,
		Duration:  7200,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	uc := NewListMediaUseCase(repo)

	resp, err := uc.ExecuteByType(context.Background(), 1, media.MediaTypeMovie)

	if err != nil {
		t.Errorf("ExecuteByType() unexpected error = %v", err)
		return
	}

	if resp.Total != 1 {
		t.Errorf("ExecuteByType() total = %v, want 1", resp.Total)
	}
}
