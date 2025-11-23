package library

import (
	"context"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestListLibrariesUseCase_Execute(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(*mocks.LibraryRepository)
		expectedCount int
		wantErr       bool
	}{
		{
			name: "empty list",
			setup: func(repo *mocks.LibraryRepository) {
				// No libraries
			},
			expectedCount: 0,
			wantErr:       false,
		},
		{
			name: "single library",
			setup: func(repo *mocks.LibraryRepository) {
				_ = repo.Create(context.Background(), &library.Library{
					Name:      "Movies",
					Path:      "/media/movies",
					Type:      library.LibraryTypeMovies,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				})
			},
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name: "multiple libraries",
			setup: func(repo *mocks.LibraryRepository) {
				libraries := []library.Library{
					{
						Name:      "Movies",
						Path:      "/media/movies",
						Type:      library.LibraryTypeMovies,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					{
						Name:      "TV Shows",
						Path:      "/media/tv",
						Type:      library.LibraryTypeTV,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					{
						Name:      "Music",
						Path:      "/media/music",
						Type:      library.LibraryTypeMusic,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				}
				for _, lib := range libraries {
					libCopy := lib
					_ = repo.Create(context.Background(), &libCopy)
				}
			},
			expectedCount: 3,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewLibraryRepository(t)
			if tt.setup != nil {
				tt.setup(repo)
			}

			service := NewLibraryService(repo, nil, nil, nil, nil)

			resp, err := service.List(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Errorf("Execute() expected error, got nil")
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

			if len(resp.Libraries) != tt.expectedCount {
				t.Errorf("Execute() libraries count = %v, want %v", len(resp.Libraries), tt.expectedCount)
			}
		})
	}
}
