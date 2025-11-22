package progress

import (
	"context"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/progress"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestGetProgressByMediaID(t *testing.T) {
	ctx := context.Background()

	// Create test progress
	testProgress := &progress.WatchProgress{
		ID:              1,
		MediaID:         100,
		UserID:          1,
		ProgressSeconds: 300,
		DurationSeconds: 1000,
		IsWatched:       false,
		LastWatchedAt:   time.Now(),
	}

	repo := mocks.NewProgressRepository(t).
		WithProgress(testProgress)

	// Test successful retrieval
	resp, err := GetProgressByMediaID(ctx, repo, 100)
	if err != nil {
		t.Fatalf("GetProgressByMediaID() failed: %v", err)
	}
	if resp.MediaID != 100 {
		t.Errorf("Expected MediaID 100, got %d", resp.MediaID)
	}
	if resp.ProgressSeconds != 300 {
		t.Errorf("Expected ProgressSeconds 300, got %.2f", resp.ProgressSeconds)
	}

	// Test not found
	emptyRepo := mocks.NewProgressRepository(t)
	_, err = GetProgressByMediaID(ctx, emptyRepo, 999)
	if err == nil {
		t.Error("Expected error for non-existent progress, got nil")
	}
}

func TestGetProgressByMediaIDAndUserID(t *testing.T) {
	ctx := context.Background()

	// Create test progress
	testProgress := &progress.WatchProgress{
		ID:              1,
		MediaID:         100,
		UserID:          5,
		ProgressSeconds: 500,
		DurationSeconds: 2000,
		IsWatched:       false,
		LastWatchedAt:   time.Now(),
	}

	repo := mocks.NewProgressRepository(t).
		WithProgress(testProgress)

	// Test successful retrieval
	resp, err := GetProgressByMediaIDAndUserID(ctx, repo, 100, 5)
	if err != nil {
		t.Fatalf("GetProgressByMediaIDAndUserID() failed: %v", err)
	}
	if resp.MediaID != 100 {
		t.Errorf("Expected MediaID 100, got %d", resp.MediaID)
	}
	if resp.UserID != 5 {
		t.Errorf("Expected UserID 5, got %d", resp.UserID)
	}

	// Test wrong user
	_, err = GetProgressByMediaIDAndUserID(ctx, repo, 100, 999)
	if err == nil {
		t.Error("Expected error for wrong user, got nil")
	}
}

func TestListProgressByUserID(t *testing.T) {
	ctx := context.Background()

	// Create test progress records
	repo := mocks.NewProgressRepository(t).
		WithProgress(
			&progress.WatchProgress{
				ID:              1,
				MediaID:         100,
				UserID:          1,
				ProgressSeconds: 300,
				DurationSeconds: 1000,
			},
			&progress.WatchProgress{
				ID:              2,
				MediaID:         200,
				UserID:          1,
				ProgressSeconds: 500,
				DurationSeconds: 2000,
			},
			&progress.WatchProgress{
				ID:              3,
				MediaID:         300,
				UserID:          2,
				ProgressSeconds: 100,
				DurationSeconds: 1000,
			},
		)

	// Test listing for user 1
	resp, err := ListProgressByUserID(ctx, repo, 1, 10, 0)
	if err != nil {
		t.Fatalf("ListProgressByUserID() failed: %v", err)
	}
	if len(resp.Progress) != 2 {
		t.Errorf("Expected 2 progress records, got %d", len(resp.Progress))
	}

	// Test listing for user 2
	resp, err = ListProgressByUserID(ctx, repo, 2, 10, 0)
	if err != nil {
		t.Fatalf("ListProgressByUserID() failed: %v", err)
	}
	if len(resp.Progress) != 1 {
		t.Errorf("Expected 1 progress record, got %d", len(resp.Progress))
	}
}

func TestListWatchedByUserID(t *testing.T) {
	ctx := context.Background()

	// Create test progress records
	repo := mocks.NewProgressRepository(t).
		WithProgress(
			&progress.WatchProgress{
				ID:        1,
				MediaID:   100,
				UserID:    1,
				IsWatched: true,
			},
			&progress.WatchProgress{
				ID:        2,
				MediaID:   200,
				UserID:    1,
				IsWatched: false,
			},
			&progress.WatchProgress{
				ID:        3,
				MediaID:   300,
				UserID:    1,
				IsWatched: true,
			},
		)

	// Test listing watched items
	resp, err := ListWatchedByUserID(ctx, repo, 1, 10, 0)
	if err != nil {
		t.Fatalf("ListWatchedByUserID() failed: %v", err)
	}
	if len(resp.Progress) != 2 {
		t.Errorf("Expected 2 watched items, got %d", len(resp.Progress))
	}
}

func TestListInProgressByUserID(t *testing.T) {
	ctx := context.Background()

	// Create test progress records
	repo := mocks.NewProgressRepository(t).
		WithProgress(
			&progress.WatchProgress{
				ID:              1,
				MediaID:         100,
				UserID:          1,
				ProgressSeconds: 500,
				DurationSeconds: 1000,
				IsWatched:       false,
			},
			&progress.WatchProgress{
				ID:              2,
				MediaID:         200,
				UserID:          1,
				ProgressSeconds: 0,
				DurationSeconds: 1000,
				IsWatched:       false,
			},
			&progress.WatchProgress{
				ID:              3,
				MediaID:         300,
				UserID:          1,
				ProgressSeconds: 900,
				DurationSeconds: 1000,
				IsWatched:       false,
			},
		)

	// Test listing in-progress items
	resp, err := ListInProgressByUserID(ctx, repo, 1, 10, 0)
	if err != nil {
		t.Fatalf("ListInProgressByUserID() failed: %v", err)
	}
	// Should return items with progress > 0 and not watched
	if len(resp.Progress) != 2 {
		t.Errorf("Expected 2 in-progress items, got %d", len(resp.Progress))
	}
}
