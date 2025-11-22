package progress

import (
	"context"
	"testing"

	"github.com/viewra/viewra/internal/domain/progress"
)

func TestMarkWatched(t *testing.T) {
	tests := []struct {
		name           string
		req            *MarkWatchedRequest
		existingProg   *progress.WatchProgress
		wantErr        bool
		wantIsWatched  bool
	}{
		{
			name: "mark new media as watched",
			req: &MarkWatchedRequest{
				MediaID: 100,
				UserID:  1,
			},
			existingProg:  nil,
			wantErr:       false,
			wantIsWatched: true,
		},
		{
			name: "mark existing progress as watched",
			req: &MarkWatchedRequest{
				MediaID: 100,
				UserID:  1,
			},
			existingProg: &progress.WatchProgress{
				ID:              1,
				MediaID:         100,
				UserID:          1,
				ProgressSeconds: 500,
				DurationSeconds: 1000,
				IsWatched:       false,
			},
			wantErr:       false,
			wantIsWatched: true,
		},
		{
			name: "invalid media ID",
			req: &MarkWatchedRequest{
				MediaID: 0,
				UserID:  1,
			},
			existingProg: nil,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			ctx := context.Background()

			if tt.existingProg != nil {
				repo.progresses[tt.existingProg.ID] = tt.existingProg
			}

			resp, err := MarkWatched(ctx, repo, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("MarkWatched() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if resp.IsWatched != tt.wantIsWatched {
					t.Errorf("MarkWatched() IsWatched = %v, want %v", resp.IsWatched, tt.wantIsWatched)
				}
				if resp.MediaID != tt.req.MediaID {
					t.Errorf("MarkWatched() MediaID = %v, want %v", resp.MediaID, tt.req.MediaID)
				}
			}
		})
	}
}

func TestMarkUnwatched(t *testing.T) {
	tests := []struct {
		name          string
		req           *MarkWatchedRequest
		existingProg  *progress.WatchProgress
		wantErr       bool
		wantIsWatched bool
	}{
		{
			name: "mark existing as unwatched",
			req: &MarkWatchedRequest{
				MediaID: 100,
				UserID:  1,
			},
			existingProg: &progress.WatchProgress{
				ID:              1,
				MediaID:         100,
				UserID:          1,
				ProgressSeconds: 1000,
				DurationSeconds: 1000,
				IsWatched:       true,
			},
			wantErr:       false,
			wantIsWatched: false,
		},
		{
			name: "progress not found",
			req: &MarkWatchedRequest{
				MediaID: 999,
				UserID:  1,
			},
			existingProg: nil,
			wantErr:      true,
		},
		{
			name: "invalid media ID",
			req: &MarkWatchedRequest{
				MediaID: 0,
				UserID:  1,
			},
			existingProg: nil,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			ctx := context.Background()

			if tt.existingProg != nil {
				repo.progresses[tt.existingProg.ID] = tt.existingProg
			}

			resp, err := MarkUnwatched(ctx, repo, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("MarkUnwatched() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if resp.IsWatched != tt.wantIsWatched {
					t.Errorf("MarkUnwatched() IsWatched = %v, want %v", resp.IsWatched, tt.wantIsWatched)
				}
				if resp.ProgressSeconds != 0 {
					t.Errorf("MarkUnwatched() ProgressSeconds should be reset to 0, got %.2f", resp.ProgressSeconds)
				}
			}
		})
	}
}

func TestDeleteProgress(t *testing.T) {
	tests := []struct {
		name         string
		mediaID      int64
		userID       int64
		existingProg *progress.WatchProgress
		wantErr      bool
	}{
		{
			name:    "delete existing progress",
			mediaID: 100,
			userID:  1,
			existingProg: &progress.WatchProgress{
				ID:      1,
				MediaID: 100,
				UserID:  1,
			},
			wantErr: false,
		},
		{
			name:         "progress not found",
			mediaID:      999,
			userID:       1,
			existingProg: nil,
			wantErr:      true,
		},
		{
			name:         "invalid media ID",
			mediaID:      0,
			userID:       1,
			existingProg: nil,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			ctx := context.Background()

			if tt.existingProg != nil {
				repo.progresses[tt.existingProg.ID] = tt.existingProg
			}

			err := DeleteProgress(ctx, repo, tt.mediaID, tt.userID)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteProgress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify progress was deleted
				if _, exists := repo.progresses[tt.existingProg.ID]; exists {
					t.Error("DeleteProgress() did not delete the progress")
				}
			}
		})
	}
}
