package media

import (
	"context"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestGetMediaUseCase_Execute(t *testing.T) {
	tests := []struct {
		name        string
		mediaID     int64
		setup       func(*mocks.MediaRepository)
		wantErr     bool
		expectedErr error
	}{
		{
			name:    "existing media",
			mediaID: 1,
			setup: func(repo *mocks.MediaRepository) {
				repo.WithMedia(&media.Media{
					ID:        1,
					LibraryID: 1,
					Title:     "Test Movie",
					FilePath:  "movies/test.mp4",
					FileSize:  1000000,
					Duration:  7200,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				})
			},
			wantErr: false,
		},
		{
			name:        "non-existent media",
			mediaID:     999,
			setup:       func(repo *mocks.MediaRepository) {},
			wantErr:     true,
			expectedErr: media.ErrMediaNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMediaRepository(t)
			if tt.setup != nil {
				tt.setup(repo)
			}

			uc := NewGetMediaUseCase(repo)

			resp, err := uc.Execute(context.Background(), tt.mediaID)

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

			if resp.ID != tt.mediaID {
				t.Errorf("Execute() ID = %v, want %v", resp.ID, tt.mediaID)
			}
		})
	}
}
