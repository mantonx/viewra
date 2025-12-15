package media

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// mockEnrichmentEnqueuer is a mock for testing enqueueForEnrichment
type mockEnrichmentEnqueuer struct {
	enqueueCalls []struct {
		mediaID   int64
		mediaType enrichment.MediaType
	}
	enqueueErr error
}

func (m *mockEnrichmentEnqueuer) EnqueueFirstStage(ctx context.Context, mediaID int64, mediaType enrichment.MediaType) error {
	m.enqueueCalls = append(m.enqueueCalls, struct {
		mediaID   int64
		mediaType enrichment.MediaType
	}{mediaID, mediaType})
	return m.enqueueErr
}

func TestEnqueueForEnrichment_NilEnqueuer(t *testing.T) {
	deps := &Deps{
		EnrichmentEnqueuer: nil,
		Logger:             testLogger(),
	}

	// Should not panic with nil enqueuer
	enqueueForEnrichment(context.Background(), deps, 1, enrichment.MediaTypeMovie)
}

func TestEnqueueForEnrichment_Success(t *testing.T) {
	enqueuer := &mockEnrichmentEnqueuer{}
	deps := &Deps{
		EnrichmentEnqueuer: enqueuer,
		Logger:             testLogger(),
	}

	enqueueForEnrichment(context.Background(), deps, 123, enrichment.MediaTypeMovie)

	// Wait briefly for goroutine to execute
	time.Sleep(50 * time.Millisecond)

	if len(enqueuer.enqueueCalls) != 1 {
		t.Errorf("Expected 1 enqueue call, got %d", len(enqueuer.enqueueCalls))
	}
	if len(enqueuer.enqueueCalls) > 0 {
		if enqueuer.enqueueCalls[0].mediaID != 123 {
			t.Errorf("Expected mediaID 123, got %d", enqueuer.enqueueCalls[0].mediaID)
		}
		if enqueuer.enqueueCalls[0].mediaType != enrichment.MediaTypeMovie {
			t.Errorf("Expected MediaTypeMovie, got %v", enqueuer.enqueueCalls[0].mediaType)
		}
	}
}

func TestEnqueueForEnrichment_Error(t *testing.T) {
	enqueuer := &mockEnrichmentEnqueuer{
		enqueueErr: errors.New("enrichment failed"),
	}
	deps := &Deps{
		EnrichmentEnqueuer: enqueuer,
		Logger:             testLogger(),
	}

	// Should not panic on error (just logs warning)
	enqueueForEnrichment(context.Background(), deps, 456, enrichment.MediaTypeTV)

	// Wait briefly for goroutine to execute
	time.Sleep(50 * time.Millisecond)

	// Verify call was attempted
	if len(enqueuer.enqueueCalls) != 1 {
		t.Errorf("Expected 1 enqueue call attempt, got %d", len(enqueuer.enqueueCalls))
	}
}

func TestEnqueueForEnrichment_AllMediaTypes(t *testing.T) {
	tests := []struct {
		name      string
		mediaType enrichment.MediaType
	}{
		{"movie", enrichment.MediaTypeMovie},
		{"tv", enrichment.MediaTypeTV},
		{"music", enrichment.MediaTypeMusic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enqueuer := &mockEnrichmentEnqueuer{}
			deps := &Deps{
				EnrichmentEnqueuer: enqueuer,
				Logger:             testLogger(),
			}

			enqueueForEnrichment(context.Background(), deps, 1, tt.mediaType)

			// Wait briefly for goroutine
			time.Sleep(50 * time.Millisecond)

			if len(enqueuer.enqueueCalls) != 1 {
				t.Errorf("Expected 1 enqueue call, got %d", len(enqueuer.enqueueCalls))
				return
			}
			if enqueuer.enqueueCalls[0].mediaType != tt.mediaType {
				t.Errorf("Expected %v, got %v", tt.mediaType, enqueuer.enqueueCalls[0].mediaType)
			}
		})
	}
}

func TestProcessMediaWithCache_CacheHit(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)

	// Setup existing media
	mediaRepo.WithMedia(&media.Media{
		ID:        100,
		LibraryID: 1,
		FilePath:  "/movies/test.mp4",
	})

	mediaRepos := &scan.MediaRepositories{
		Library: mocks.NewLibraryRepository(t),
		Media:   mediaRepo,
		Movie:   movieRepo,
		TV:      mocks.NewTVRepository(t),
		Music:   mocks.NewMusicRepository(t),
	}

	deps := &Deps{
		MediaRepos: mediaRepos,
		Logger:     testLogger(),
	}

	// Pre-populate cache
	cache := &sync.Map{}
	cache.Store("/movies/test.mp4", int64(100))

	var mediaID int64 = 0
	updateCalled := false
	postSaveCalled := false

	callbacks := UpsertCallbacks{
		GetMediaID: func() int64 { return mediaID },
		SetMediaID: func(id int64) { mediaID = id },
		Update: func(ctx context.Context) error {
			updateCalled = true
			return nil
		},
		Create: func(ctx context.Context) error {
			t.Error("Create should not be called on cache hit")
			return nil
		},
		PostSave: func(ctx context.Context) {
			postSaveCalled = true
		},
	}

	result, err := ProcessMediaWithCache(context.Background(), deps, 1, "/movies/test.mp4", cache, callbacks)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
	if *result != 100 {
		t.Errorf("Expected mediaID 100, got %d", *result)
	}
	if !updateCalled {
		t.Error("Expected Update to be called")
	}
	if !postSaveCalled {
		t.Error("Expected PostSave to be called")
	}
}

func TestProcessMediaWithCache_CacheMiss_CreateSuccess(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)

	mediaRepos := &scan.MediaRepositories{
		Library: mocks.NewLibraryRepository(t),
		Media:   mediaRepo,
		Movie:   movieRepo,
		TV:      mocks.NewTVRepository(t),
		Music:   mocks.NewMusicRepository(t),
	}

	deps := &Deps{
		MediaRepos: mediaRepos,
		Logger:     testLogger(),
	}

	cache := &sync.Map{}

	var mediaID int64 = 200
	createCalled := false
	postSaveCalled := false

	callbacks := UpsertCallbacks{
		GetMediaID: func() int64 { return mediaID },
		SetMediaID: func(id int64) { mediaID = id },
		Update: func(ctx context.Context) error {
			t.Error("Update should not be called on new create")
			return nil
		},
		Create: func(ctx context.Context) error {
			createCalled = true
			return nil
		},
		PostSave: func(ctx context.Context) {
			postSaveCalled = true
		},
	}

	result, err := ProcessMediaWithCache(context.Background(), deps, 1, "/movies/new.mp4", cache, callbacks)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
	if *result != 200 {
		t.Errorf("Expected mediaID 200, got %d", *result)
	}
	if !createCalled {
		t.Error("Expected Create to be called")
	}
	if !postSaveCalled {
		t.Error("Expected PostSave to be called")
	}

	// Verify cache was updated
	if cached, found := cache.Load("/movies/new.mp4"); !found || cached.(int64) != 200 {
		t.Error("Expected cache to be updated with new mediaID")
	}
}

func TestProcessMediaWithCache_UpdateError(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)

	mediaRepos := &scan.MediaRepositories{
		Library: mocks.NewLibraryRepository(t),
		Media:   mediaRepo,
		Movie:   movieRepo,
		TV:      mocks.NewTVRepository(t),
		Music:   mocks.NewMusicRepository(t),
	}

	deps := &Deps{
		MediaRepos: mediaRepos,
		Logger:     testLogger(),
	}

	cache := &sync.Map{}
	cache.Store("/movies/test.mp4", int64(100))

	var mediaID int64 = 100

	callbacks := UpsertCallbacks{
		GetMediaID: func() int64 { return mediaID },
		SetMediaID: func(id int64) { mediaID = id },
		Update: func(ctx context.Context) error {
			return errors.New("update failed")
		},
		Create: func(ctx context.Context) error { return nil },
		PostSave: func(ctx context.Context) {
			t.Error("PostSave should not be called on update error")
		},
	}

	result, err := ProcessMediaWithCache(context.Background(), deps, 1, "/movies/test.mp4", cache, callbacks)

	if err == nil {
		t.Error("Expected error on update failure")
	}
	if result != nil {
		t.Error("Expected nil result on error")
	}
}

func TestProcessMediaWithCache_CreateNonConstraintError(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)

	mediaRepos := &scan.MediaRepositories{
		Library: mocks.NewLibraryRepository(t),
		Media:   mediaRepo,
		Movie:   movieRepo,
		TV:      mocks.NewTVRepository(t),
		Music:   mocks.NewMusicRepository(t),
	}

	deps := &Deps{
		MediaRepos: mediaRepos,
		Logger:     testLogger(),
	}

	cache := &sync.Map{}

	callbacks := UpsertCallbacks{
		GetMediaID: func() int64 { return 0 },
		SetMediaID: func(id int64) {},
		Update:     func(ctx context.Context) error { return nil },
		Create: func(ctx context.Context) error {
			return errors.New("generic database error")
		},
		PostSave: func(ctx context.Context) {
			t.Error("PostSave should not be called on create error")
		},
	}

	result, err := ProcessMediaWithCache(context.Background(), deps, 1, "/movies/new.mp4", cache, callbacks)

	if err == nil {
		t.Error("Expected error on non-constraint create failure")
	}
	if result != nil {
		t.Error("Expected nil result on error")
	}
}

func TestProcessMediaWithCache_ConstraintError_CacheHitOnRetry(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)

	mediaRepos := &scan.MediaRepositories{
		Library: mocks.NewLibraryRepository(t),
		Media:   mediaRepo,
		Movie:   movieRepo,
		TV:      mocks.NewTVRepository(t),
		Music:   mocks.NewMusicRepository(t),
	}

	deps := &Deps{
		MediaRepos: mediaRepos,
		Logger:     testLogger(),
	}

	cache := &sync.Map{}

	var mediaID int64 = 0
	createCallCount := 0
	updateCalled := false

	callbacks := UpsertCallbacks{
		GetMediaID: func() int64 { return mediaID },
		SetMediaID: func(id int64) { mediaID = id },
		Update: func(ctx context.Context) error {
			updateCalled = true
			return nil
		},
		Create: func(ctx context.Context) error {
			createCallCount++
			// Simulate another worker adding to cache during race
			cache.Store("/movies/race.mp4", int64(300))
			return errors.New("UNIQUE constraint failed")
		},
		PostSave: func(ctx context.Context) {},
	}

	result, err := ProcessMediaWithCache(context.Background(), deps, 1, "/movies/race.mp4", cache, callbacks)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
	if *result != 300 {
		t.Errorf("Expected mediaID 300 from cache retry, got %d", *result)
	}
	if createCallCount != 1 {
		t.Errorf("Expected Create to be called once, got %d", createCallCount)
	}
	if !updateCalled {
		t.Error("Expected Update to be called after cache hit on retry")
	}
}

func TestProcessMediaWithCache_ConstraintError_FetchFromDB(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)

	// Setup media that will be fetched from DB
	mediaRepo.WithMedia(&media.Media{
		ID:        400,
		LibraryID: 1,
		FilePath:  "/movies/collision.mp4",
	})

	mediaRepos := &scan.MediaRepositories{
		Library: mocks.NewLibraryRepository(t),
		Media:   mediaRepo,
		Movie:   movieRepo,
		TV:      mocks.NewTVRepository(t),
		Music:   mocks.NewMusicRepository(t),
	}

	deps := &Deps{
		MediaRepos: mediaRepos,
		Logger:     testLogger(),
	}

	cache := &sync.Map{}

	var mediaID int64 = 0
	updateCalled := false

	callbacks := UpsertCallbacks{
		GetMediaID: func() int64 { return mediaID },
		SetMediaID: func(id int64) { mediaID = id },
		Update: func(ctx context.Context) error {
			updateCalled = true
			return nil
		},
		Create: func(ctx context.Context) error {
			return errors.New("duplicate key value violates unique constraint")
		},
		PostSave: func(ctx context.Context) {},
	}

	result, err := ProcessMediaWithCache(context.Background(), deps, 1, "/movies/collision.mp4", cache, callbacks)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
	if *result != 400 {
		t.Errorf("Expected mediaID 400 from DB fetch, got %d", *result)
	}
	if !updateCalled {
		t.Error("Expected Update to be called after DB fetch")
	}

	// Verify cache was updated
	if cached, found := cache.Load("/movies/collision.mp4"); !found || cached.(int64) != 400 {
		t.Error("Expected cache to be updated with fetched mediaID")
	}
}

func TestProcessMediaWithCache_ConstraintError_FetchError(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)

	// Setup fetch error
	mediaRepo.GetByFilePathErr = errors.New("database fetch failed")

	mediaRepos := &scan.MediaRepositories{
		Library: mocks.NewLibraryRepository(t),
		Media:   mediaRepo,
		Movie:   movieRepo,
		TV:      mocks.NewTVRepository(t),
		Music:   mocks.NewMusicRepository(t),
	}

	deps := &Deps{
		MediaRepos: mediaRepos,
		Logger:     testLogger(),
	}

	cache := &sync.Map{}

	callbacks := UpsertCallbacks{
		GetMediaID: func() int64 { return 0 },
		SetMediaID: func(id int64) {},
		Update:     func(ctx context.Context) error { return nil },
		Create: func(ctx context.Context) error {
			return errors.New("UNIQUE constraint failed")
		},
		PostSave: func(ctx context.Context) {},
	}

	result, err := ProcessMediaWithCache(context.Background(), deps, 1, "/movies/fetch-fail.mp4", cache, callbacks)

	if err == nil {
		t.Error("Expected error on fetch failure after constraint error")
	}
	if result != nil {
		t.Error("Expected nil result on error")
	}
}

func TestProcessMediaWithCache_ConstraintError_UpdateAfterFetchError(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	movieRepo := mocks.NewMovieRepository(t)

	// Setup media for fetch
	mediaRepo.WithMedia(&media.Media{
		ID:        500,
		LibraryID: 1,
		FilePath:  "/movies/update-fail.mp4",
	})

	mediaRepos := &scan.MediaRepositories{
		Library: mocks.NewLibraryRepository(t),
		Media:   mediaRepo,
		Movie:   movieRepo,
		TV:      mocks.NewTVRepository(t),
		Music:   mocks.NewMusicRepository(t),
	}

	deps := &Deps{
		MediaRepos: mediaRepos,
		Logger:     testLogger(),
	}

	cache := &sync.Map{}

	var mediaID int64 = 0

	callbacks := UpsertCallbacks{
		GetMediaID: func() int64 { return mediaID },
		SetMediaID: func(id int64) { mediaID = id },
		Update: func(ctx context.Context) error {
			return errors.New("update after fetch failed")
		},
		Create: func(ctx context.Context) error {
			return errors.New("UNIQUE constraint failed")
		},
		PostSave: func(ctx context.Context) {},
	}

	result, err := ProcessMediaWithCache(context.Background(), deps, 1, "/movies/update-fail.mp4", cache, callbacks)

	if err == nil {
		t.Error("Expected error on update failure after fetch")
	}
	if result != nil {
		t.Error("Expected nil result on error")
	}
}
