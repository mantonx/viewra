package progress

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/progress"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestUpdateProgress_CreateNew(t *testing.T) {
	repo := mocks.NewProgressRepository(t)
	req := &UpdateProgressRequest{
		MediaID:         1,
		UserID:          1,
		ProgressSeconds: 45,
		DurationSeconds: 100,
	}

	resp, err := UpdateProgress(context.Background(), repo, req)
	if err != nil {
		t.Fatalf("UpdateProgress() error = %v", err)
	}

	if resp.MediaID != 1 {
		t.Errorf("MediaID = %v, want 1", resp.MediaID)
	}
	if resp.ProgressSeconds != 45 {
		t.Errorf("ProgressSeconds = %v, want 45", resp.ProgressSeconds)
	}
	if resp.ProgressPercentage != 45.0 {
		t.Errorf("ProgressPercentage = %v, want 45.0", resp.ProgressPercentage)
	}
	if resp.IsWatched {
		t.Error("IsWatched = true, want false (45% < 90%)")
	}
}

func TestUpdateProgress_AutoMarkWatched(t *testing.T) {
	repo := mocks.NewProgressRepository(t)
	req := &UpdateProgressRequest{
		MediaID:         1,
		UserID:          1,
		ProgressSeconds: 90,
		DurationSeconds: 100,
	}

	resp, err := UpdateProgress(context.Background(), repo, req)
	if err != nil {
		t.Fatalf("UpdateProgress() error = %v", err)
	}

	if !resp.IsWatched {
		t.Error("IsWatched = false, want true (90% should auto-mark)")
	}
	if resp.ProgressPercentage != 90.0 {
		t.Errorf("ProgressPercentage = %v, want 90.0", resp.ProgressPercentage)
	}
}

func TestUpdateProgress_DoesNotAutoMark(t *testing.T) {
	repo := mocks.NewProgressRepository(t)
	req := &UpdateProgressRequest{
		MediaID:         1,
		UserID:          1,
		ProgressSeconds: 89,
		DurationSeconds: 100,
	}

	resp, err := UpdateProgress(context.Background(), repo, req)
	if err != nil {
		t.Fatalf("UpdateProgress() error = %v", err)
	}

	if resp.IsWatched {
		t.Error("IsWatched = true, want false (89% should NOT auto-mark)")
	}
}

func TestUpdateProgress_UpdateExisting(t *testing.T) {
	existing := &progress.WatchProgress{
		ID:              1,
		MediaID:         1,
		UserID:          1,
		ProgressSeconds: 30,
		DurationSeconds: 100,
		IsWatched:       false,
		CreatedAt:       time.Now().Add(-1 * time.Hour),
		UpdatedAt:       time.Now().Add(-1 * time.Hour),
	}

	repo := mocks.NewProgressRepository(t).WithProgress(existing)

	req := &UpdateProgressRequest{
		MediaID:         1,
		UserID:          1,
		ProgressSeconds: 95,
		DurationSeconds: 100,
	}

	resp, err := UpdateProgress(context.Background(), repo, req)
	if err != nil {
		t.Fatalf("UpdateProgress() error = %v", err)
	}

	if resp.ProgressSeconds != 95 {
		t.Errorf("ProgressSeconds = %v, want 95", resp.ProgressSeconds)
	}
	if !resp.IsWatched {
		t.Error("IsWatched = false, want true (95% should auto-mark)")
	}
}

func TestUpdateProgress_InvalidMediaID(t *testing.T) {
	repo := mocks.NewProgressRepository(t)
	req := &UpdateProgressRequest{
		MediaID:         0,
		UserID:          1,
		ProgressSeconds: 50,
		DurationSeconds: 100,
	}

	_, err := UpdateProgress(context.Background(), repo, req)
	if err != progress.ErrInvalidMediaID {
		t.Errorf("UpdateProgress() error = %v, want %v", err, progress.ErrInvalidMediaID)
	}
}

func TestUpdateProgress_NegativeProgress(t *testing.T) {
	repo := mocks.NewProgressRepository(t)
	req := &UpdateProgressRequest{
		MediaID:         1,
		UserID:          1,
		ProgressSeconds: -10,
		DurationSeconds: 100,
	}

	_, err := UpdateProgress(context.Background(), repo, req)
	if err != progress.ErrInvalidProgress {
		t.Errorf("UpdateProgress() error = %v, want %v", err, progress.ErrInvalidProgress)
	}
}

func TestUpdateProgress_NegativeDuration(t *testing.T) {
	repo := mocks.NewProgressRepository(t)
	req := &UpdateProgressRequest{
		MediaID:         1,
		UserID:          1,
		ProgressSeconds: 50,
		DurationSeconds: -100,
	}

	_, err := UpdateProgress(context.Background(), repo, req)
	if err != progress.ErrInvalidDuration {
		t.Errorf("UpdateProgress() error = %v, want %v", err, progress.ErrInvalidDuration)
	}
}

func TestUpdateProgress_ProgressExceedsDuration(t *testing.T) {
	repo := mocks.NewProgressRepository(t)
	req := &UpdateProgressRequest{
		MediaID:         1,
		UserID:          1,
		ProgressSeconds: 150,
		DurationSeconds: 100,
	}

	_, err := UpdateProgress(context.Background(), repo, req)
	if err != progress.ErrProgressExceedsDuration {
		t.Errorf("UpdateProgress() error = %v, want %v", err, progress.ErrProgressExceedsDuration)
	}
}

func TestUpdateProgress_ZeroDuration(t *testing.T) {
	repo := mocks.NewProgressRepository(t)
	req := &UpdateProgressRequest{
		MediaID:         1,
		UserID:          1,
		ProgressSeconds: 0,
		DurationSeconds: 0,
	}

	resp, err := UpdateProgress(context.Background(), repo, req)
	if err != nil {
		t.Fatalf("UpdateProgress() error = %v", err)
	}

	if resp.ProgressPercentage != 0.0 {
		t.Errorf("ProgressPercentage = %v, want 0.0", resp.ProgressPercentage)
	}
	if resp.IsWatched {
		t.Error("IsWatched = true, want false (0 duration shouldn't auto-mark)")
	}
}

func TestUpdateProgress_RepositoryError(t *testing.T) {
	testErr := errors.New("database error")
	repo := mocks.NewProgressRepository(t).WithCreateError(testErr)

	req := &UpdateProgressRequest{
		MediaID:         1,
		UserID:          1,
		ProgressSeconds: 50,
		DurationSeconds: 100,
	}

	_, err := UpdateProgress(context.Background(), repo, req)
	if err != testErr {
		t.Errorf("UpdateProgress() error = %v, want %v", err, testErr)
	}
}
