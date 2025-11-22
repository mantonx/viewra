package progress

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viewra/viewra/internal/domain/progress"
)

// mockRepository implements progress.Repository for testing
type mockRepository struct {
	progresses                map[int64]*progress.WatchProgress
	nextID                    int64
	createFunc               func(ctx context.Context, prog *progress.WatchProgress) error
	updateFunc               func(ctx context.Context, prog *progress.WatchProgress) error
	getByMediaIDAndUserIDFunc func(ctx context.Context, mediaID, userID int64) (*progress.WatchProgress, error)
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		progresses: make(map[int64]*progress.WatchProgress),
		nextID:     1,
	}
}

func (m *mockRepository) Create(ctx context.Context, prog *progress.WatchProgress) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, prog)
	}
	prog.ID = m.nextID
	m.nextID++
	prog.CreatedAt = time.Now()
	prog.UpdatedAt = time.Now()
	m.progresses[prog.ID] = prog
	return nil
}

func (m *mockRepository) Update(ctx context.Context, prog *progress.WatchProgress) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, prog)
	}
	prog.UpdatedAt = time.Now()
	m.progresses[prog.ID] = prog
	return nil
}

func (m *mockRepository) GetByMediaID(ctx context.Context, mediaID int64) (*progress.WatchProgress, error) {
	for _, p := range m.progresses {
		if p.MediaID == mediaID {
			return p, nil
		}
	}
	return nil, progress.ErrProgressNotFound
}

func (m *mockRepository) GetByMediaIDAndUserID(ctx context.Context, mediaID, userID int64) (*progress.WatchProgress, error) {
	if m.getByMediaIDAndUserIDFunc != nil {
		return m.getByMediaIDAndUserIDFunc(ctx, mediaID, userID)
	}
	for _, p := range m.progresses {
		if p.MediaID == mediaID && p.UserID == userID {
			return p, nil
		}
	}
	return nil, progress.ErrProgressNotFound
}

func (m *mockRepository) GetBatchByMediaIDs(ctx context.Context, mediaIDs []int64, userID int64) (map[int64]*progress.WatchProgress, error) {
	result := make(map[int64]*progress.WatchProgress)
	for _, p := range m.progresses {
		if p.UserID == userID {
			for _, mediaID := range mediaIDs {
				if p.MediaID == mediaID {
					result[mediaID] = p
					break
				}
			}
		}
	}
	return result, nil
}

func (m *mockRepository) ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]*progress.WatchProgress, error) {
	var result []*progress.WatchProgress
	for _, p := range m.progresses {
		if p.UserID == userID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockRepository) ListWatchedByUserID(ctx context.Context, userID int64, limit, offset int) ([]*progress.WatchProgress, error) {
	var result []*progress.WatchProgress
	for _, p := range m.progresses {
		if p.UserID == userID && p.IsWatched {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockRepository) ListInProgressByUserID(ctx context.Context, userID int64, limit, offset int) ([]*progress.WatchProgress, error) {
	var result []*progress.WatchProgress
	for _, p := range m.progresses {
		if p.UserID == userID && !p.IsWatched && p.ProgressSeconds > 0 {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockRepository) Delete(ctx context.Context, id int64) error {
	delete(m.progresses, id)
	return nil
}

func (m *mockRepository) DeleteByMediaID(ctx context.Context, mediaID int64) error {
	for id, p := range m.progresses {
		if p.MediaID == mediaID {
			delete(m.progresses, id)
		}
	}
	return nil
}

func (m *mockRepository) Upsert(ctx context.Context, prog *progress.WatchProgress) error {
	if prog.ID == 0 {
		return m.Create(ctx, prog)
	}
	return m.Update(ctx, prog)
}

func TestUpdateProgress_CreateNew(t *testing.T) {
	repo := newMockRepository()
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
	repo := newMockRepository()
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
	repo := newMockRepository()
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

	repo := newMockRepository()
	repo.getByMediaIDAndUserIDFunc = func(ctx context.Context, mediaID, userID int64) (*progress.WatchProgress, error) {
		return existing, nil
	}

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
	repo := newMockRepository()
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
	repo := newMockRepository()
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
	repo := newMockRepository()
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
	repo := newMockRepository()
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
	repo := newMockRepository()
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
	repo := &mockRepository{
		createFunc: func(ctx context.Context, prog *progress.WatchProgress) error {
			return testErr
		},
	}

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
