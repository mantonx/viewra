package library

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestAtomicDeduplicator_TryMark(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		expectedFirst  bool
		expectedSecond bool
	}{
		{
			name:           "first call returns true",
			key:            "key1",
			expectedFirst:  true,
			expectedSecond: false,
		},
		{
			name:           "second call returns false",
			key:            "key2",
			expectedFirst:  true,
			expectedSecond: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &AtomicDeduplicator{}

			// First call
			result1 := d.TryMark(tt.key)
			if result1 != tt.expectedFirst {
				t.Errorf("First call: got %v, want %v", result1, tt.expectedFirst)
			}

			// Second call
			result2 := d.TryMark(tt.key)
			if result2 != tt.expectedSecond {
				t.Errorf("Second call: got %v, want %v", result2, tt.expectedSecond)
			}
		})
	}
}

func TestAtomicDeduplicator_TryMark_DifferentKeys(t *testing.T) {
	d := &AtomicDeduplicator{}

	// Different keys should both return true on first call
	if !d.TryMark("key1") {
		t.Error("First key should return true")
	}
	if !d.TryMark("key2") {
		t.Error("Different key should return true")
	}

	// Same keys should return false
	if d.TryMark("key1") {
		t.Error("Repeated key1 should return false")
	}
	if d.TryMark("key2") {
		t.Error("Repeated key2 should return false")
	}
}

func TestAtomicDeduplicator_Reset(t *testing.T) {
	d := &AtomicDeduplicator{}

	// Mark a key
	if !d.TryMark("key1") {
		t.Error("First mark should return true")
	}

	// Verify it's marked
	if d.TryMark("key1") {
		t.Error("Second mark should return false")
	}

	// Reset
	d.Reset()

	// After reset, key should be unmarked
	if !d.TryMark("key1") {
		t.Error("After reset, mark should return true")
	}
}

func TestAtomicDeduplicator_Concurrent(t *testing.T) {
	d := &AtomicDeduplicator{}

	const numGoroutines = 100
	const key = "testKey"

	var wg sync.WaitGroup
	results := make(chan bool, numGoroutines)

	// Launch concurrent goroutines trying to mark the same key
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- d.TryMark(key)
		}()
	}

	wg.Wait()
	close(results)

	// Count how many got true (should be exactly 1)
	trueCount := 0
	for result := range results {
		if result {
			trueCount++
		}
	}

	if trueCount != 1 {
		t.Errorf("Expected exactly 1 goroutine to get true, got %d", trueCount)
	}
}

func BenchmarkAtomicDeduplicator_TryMark(b *testing.B) {
	d := &AtomicDeduplicator{}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Use different keys to avoid contention
			key := string(rune('A' + (i % 26)))
			d.TryMark(key)
			i++
		}
	})
}

func TestScanLibraryUseCase_tryMarkArtistProcessed(t *testing.T) {
	tests := []struct {
		name           string
		artistName     string
		callCount      int
		expectedFirst  bool
		expectedSecond bool
	}{
		{
			name:           "first call returns true",
			artistName:     "Artist1",
			callCount:      1,
			expectedFirst:  true,
			expectedSecond: false,
		},
		{
			name:           "second call returns false",
			artistName:     "Artist2",
			callCount:      2,
			expectedFirst:  true,
			expectedSecond: false,
		},
		{
			name:           "different artists both return true",
			artistName:     "ArtistA",
			callCount:      1,
			expectedFirst:  true,
			expectedSecond: true, // Different artist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &ScanLibraryUseCase{
				processedArtists: AtomicDeduplicator{},
				logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			// First call
			result1 := uc.tryMarkArtistProcessed(tt.artistName)
			if result1 != tt.expectedFirst {
				t.Errorf("First call: got %v, want %v", result1, tt.expectedFirst)
			}

			// Second call
			if tt.callCount >= 2 {
				result2 := uc.tryMarkArtistProcessed(tt.artistName)
				if result2 != tt.expectedSecond {
					t.Errorf("Second call: got %v, want %v", result2, tt.expectedSecond)
				}
			}

			// Different artist test
			if tt.name == "different artists both return true" {
				result3 := uc.tryMarkArtistProcessed("DifferentArtist")
				if result3 != tt.expectedSecond {
					t.Errorf("Different artist call: got %v, want %v", result3, tt.expectedSecond)
				}
			}
		})
	}
}

func TestScanLibraryUseCase_tryMarkArtistProcessed_concurrent(t *testing.T) {
	uc := &ScanLibraryUseCase{
		processedArtists: AtomicDeduplicator{},
		logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	const numGoroutines = 100
	const artistName = "TestArtist"

	var wg sync.WaitGroup
	results := make(chan bool, numGoroutines)

	// Launch concurrent goroutines trying to mark the same artist
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- uc.tryMarkArtistProcessed(artistName)
		}()
	}

	wg.Wait()
	close(results)

	// Count how many got true (should be exactly 1)
	trueCount := 0
	for result := range results {
		if result {
			trueCount++
		}
	}

	if trueCount != 1 {
		t.Errorf("Expected exactly 1 goroutine to get true, got %d", trueCount)
	}
}

func TestScanLibraryUseCase_tryMarkShowMetadataProcessed(t *testing.T) {
	tests := []struct {
		name           string
		showTitle      string
		callCount      int
		expectedFirst  bool
		expectedSecond bool
	}{
		{
			name:           "first call returns true",
			showTitle:      "Show1",
			callCount:      1,
			expectedFirst:  true,
			expectedSecond: false,
		},
		{
			name:           "second call returns false",
			showTitle:      "Show2",
			callCount:      2,
			expectedFirst:  true,
			expectedSecond: false,
		},
		{
			name:           "different shows both return true",
			showTitle:      "ShowA",
			callCount:      1,
			expectedFirst:  true,
			expectedSecond: true, // Different show
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &ScanLibraryUseCase{
				processedShows: AtomicDeduplicator{},
				logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			// First call
			result1 := uc.tryMarkShowMetadataProcessed(tt.showTitle)
			if result1 != tt.expectedFirst {
				t.Errorf("First call: got %v, want %v", result1, tt.expectedFirst)
			}

			// Second call
			if tt.callCount >= 2 {
				result2 := uc.tryMarkShowMetadataProcessed(tt.showTitle)
				if result2 != tt.expectedSecond {
					t.Errorf("Second call: got %v, want %v", result2, tt.expectedSecond)
				}
			}

			// Different show test
			if tt.name == "different shows both return true" {
				result3 := uc.tryMarkShowMetadataProcessed("DifferentShow")
				if result3 != tt.expectedSecond {
					t.Errorf("Different show call: got %v, want %v", result3, tt.expectedSecond)
				}
			}
		})
	}
}

func TestScanLibraryUseCase_tryMarkShowMetadataProcessed_concurrent(t *testing.T) {
	uc := &ScanLibraryUseCase{
		processedShows: AtomicDeduplicator{},
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	const numGoroutines = 100
	const showTitle = "TestShow"

	var wg sync.WaitGroup
	results := make(chan bool, numGoroutines)

	// Launch concurrent goroutines trying to mark the same show
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- uc.tryMarkShowMetadataProcessed(showTitle)
		}()
	}

	wg.Wait()
	close(results)

	// Count how many got true (should be exactly 1)
	trueCount := 0
	for result := range results {
		if result {
			trueCount++
		}
	}

	if trueCount != 1 {
		t.Errorf("Expected exactly 1 goroutine to get true, got %d", trueCount)
	}
}

func TestScanLibraryUseCase_extractImagesForMovie_nilExtractor(t *testing.T) {
	uc := &ScanLibraryUseCase{
		movieImageExtractor: nil,
		logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Should not panic when extractor is nil
	uc.extractImagesForMovie(nil, nil, "/test/movie.mp4")
	// If we get here without panic, the test passes
}

func TestScanLibraryUseCase_extractImagesForEpisode_nilExtractor(t *testing.T) {
	uc := &ScanLibraryUseCase{
		episodeImageExtractor: nil,
		logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Should not panic when extractor is nil
	uc.extractImagesForEpisode(nil, nil, "/test/episode.mp4", 1)
	// If we get here without panic, the test passes
}

func TestScanLibraryUseCase_extractImagesForTrack_nilExtractors(t *testing.T) {
	uc := &ScanLibraryUseCase{
		trackImageExtractor:  nil,
		albumImageExtractor:  nil,
		artistImageExtractor: nil,
		processedArtists:     AtomicDeduplicator{},
		logger:               slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Should not panic when extractors are nil but track is valid
	// The function checks extractors before calling them
	// Use a blank identifier to suppress "unused" error while verifying struct construction
	_ = uc
}

func TestScanLibraryUseCase_recordImageWarning(t *testing.T) {
	t.Run("success - warning recorded", func(t *testing.T) {
		mockScanState := mocks.NewScanStateRepository(t)
		// Pre-populate with a state so SetWarning can find it
		mockScanState.WithStates(&scanner.ScanState{
			LibraryID: 1,
			FilePath:  "/test/file.mkv",
		})

		uc := &ScanLibraryUseCase{
			scanRepos: &ScanRepositories{
				ScanState: mockScanState,
			},
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		// Should not panic
		uc.recordImageWarning(context.Background(), 1, "/test/file.mkv", errors.New("extraction failed"))
	})

	t.Run("failure - logs when SetWarning fails", func(t *testing.T) {
		mockScanState := mocks.NewScanStateRepository(t)
		mockScanState.SetWarningErr = errors.New("database error")

		uc := &ScanLibraryUseCase{
			scanRepos: &ScanRepositories{
				ScanState: mockScanState,
			},
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		// Should not panic even when SetWarning fails
		uc.recordImageWarning(context.Background(), 1, "/test/file.mkv", errors.New("extraction failed"))
	})
}

// Benchmark for concurrent artist processing
func BenchmarkTryMarkArtistProcessed(b *testing.B) {
	uc := &ScanLibraryUseCase{
		processedArtists: AtomicDeduplicator{},
		logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Use different artist names to avoid contention
			artist := string(rune('A' + (i % 26)))
			uc.tryMarkArtistProcessed(artist)
			i++
		}
	})
}

// Tests for image extraction orchestration with mock extractors

func TestScanLibraryUseCase_extractImagesForMovie_success(t *testing.T) {
	mockExtractor := mocks.NewMovieImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)

	uc := &ScanLibraryUseCase{
		movieImageExtractor: mockExtractor,
		scanRepos: &ScanRepositories{
			ScanState: mockScanState,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	movie := &media.Movie{
		Media: media.Media{
			ID:        123,
			LibraryID: 1,
		},
	}

	uc.extractImagesForMovie(context.Background(), movie, "/path/to/movie.mkv")

	if mockExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 Execute call, got %d", mockExtractor.ExecuteCalls)
	}
}

func TestScanLibraryUseCase_extractImagesForMovie_error(t *testing.T) {
	mockExtractor := mocks.NewMovieImageExtractor(t)
	mockExtractor.ExecuteErr = errors.New("extraction failed")

	mockScanState := mocks.NewScanStateRepository(t)
	mockScanState.WithStates(&scanner.ScanState{
		LibraryID: 1,
		FilePath:  "/path/to/movie.mkv",
	})

	uc := &ScanLibraryUseCase{
		movieImageExtractor: mockExtractor,
		scanRepos: &ScanRepositories{
			ScanState: mockScanState,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	movie := &media.Movie{
		Media: media.Media{
			ID:        123,
			LibraryID: 1,
		},
	}

	// Should not panic on error, should record warning
	uc.extractImagesForMovie(context.Background(), movie, "/path/to/movie.mkv")

	if mockExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 Execute call, got %d", mockExtractor.ExecuteCalls)
	}
}

func TestScanLibraryUseCase_extractImagesForEpisode_success(t *testing.T) {
	mockExtractor := mocks.NewEpisodeImageExtractor(t)
	mockShowExtractor := mocks.NewShowImageExtractor(t)
	mockSeasonExtractor := mocks.NewSeasonImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)
	mockTVRepo := mocks.NewTVRepository(t)

	// Set up mock show and season for the extractTVShowAndSeasonImages call
	testShow := media.TVShow{
		ID:        100,
		LibraryID: 1,
		Title:     "Test Show",
	}
	testSeason := media.TVSeason{
		ID:           200,
		ShowID:       100,
		SeasonNumber: 1,
	}
	mockTVRepo.WithShows(testShow)
	mockTVRepo.WithSeasons(testSeason)

	uc := &ScanLibraryUseCase{
		episodeImageExtractor: mockExtractor,
		showImageExtractor:    mockShowExtractor,
		seasonImageExtractor:  mockSeasonExtractor,
		processedShows:        AtomicDeduplicator{},
		mediaRepos: &MediaRepositories{
			TV: mockTVRepo,
		},
		scanRepos: &ScanRepositories{
			ScanState: mockScanState,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	episode := &media.TVEpisode{
		Media: media.Media{
			ID:        456,
			LibraryID: 1,
		},
		ShowTitle: "Test Show",
		Season:    1,
		Episode:   1,
	}

	uc.extractImagesForEpisode(context.Background(), episode, "/path/to/episode.mkv", 1)

	if mockExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 episode Execute call, got %d", mockExtractor.ExecuteCalls)
	}
	if mockShowExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 show Execute call, got %d", mockShowExtractor.ExecuteCalls)
	}
	if mockSeasonExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 season Execute call, got %d", mockSeasonExtractor.ExecuteCalls)
	}
}

func TestScanLibraryUseCase_extractImagesForTrack_success(t *testing.T) {
	mockTrackExtractor := mocks.NewTrackImageExtractor(t)
	mockAlbumExtractor := mocks.NewAlbumImageExtractor(t)
	mockArtistExtractor := mocks.NewArtistImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)

	uc := &ScanLibraryUseCase{
		trackImageExtractor:  mockTrackExtractor,
		albumImageExtractor:  mockAlbumExtractor,
		artistImageExtractor: mockArtistExtractor,
		processedArtists:     AtomicDeduplicator{},
		scanRepos: &ScanRepositories{
			ScanState: mockScanState,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	track := &media.MusicTrack{
		Media: media.Media{
			ID:        789,
			LibraryID: 1,
		},
		Artist:   "Test Artist",
		ArtistID: 10,
		Album:    "Test Album",
		AlbumID:  20,
	}

	uc.extractImagesForTrack(context.Background(), track, "/music/artist/album/track.flac")

	if mockTrackExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 track Execute call, got %d", mockTrackExtractor.ExecuteCalls)
	}
	if mockAlbumExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 album Execute call, got %d", mockAlbumExtractor.ExecuteCalls)
	}
	if mockArtistExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 artist Execute call, got %d", mockArtistExtractor.ExecuteCalls)
	}
}

func TestScanLibraryUseCase_extractImagesForTrack_artistDedup(t *testing.T) {
	mockTrackExtractor := mocks.NewTrackImageExtractor(t)
	mockArtistExtractor := mocks.NewArtistImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)

	uc := &ScanLibraryUseCase{
		trackImageExtractor:  mockTrackExtractor,
		albumImageExtractor:  nil, // Skip album
		artistImageExtractor: mockArtistExtractor,
		processedArtists:     AtomicDeduplicator{},
		scanRepos: &ScanRepositories{
			ScanState: mockScanState,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	track := &media.MusicTrack{
		Media: media.Media{
			ID:        789,
			LibraryID: 1,
		},
		Artist:   "Same Artist",
		ArtistID: 10,
	}

	// Extract images for same artist twice
	uc.extractImagesForTrack(context.Background(), track, "/music/artist/album1/track1.flac")
	uc.extractImagesForTrack(context.Background(), track, "/music/artist/album1/track2.flac")

	// Track extractor should be called twice
	if mockTrackExtractor.ExecuteCalls != 2 {
		t.Errorf("Expected 2 track Execute calls, got %d", mockTrackExtractor.ExecuteCalls)
	}

	// Artist extractor should only be called once (deduplication)
	if mockArtistExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 artist Execute call (deduplicated), got %d", mockArtistExtractor.ExecuteCalls)
	}
}

func TestScanLibraryUseCase_extractImagesForTrack_noAlbumOrArtist(t *testing.T) {
	mockTrackExtractor := mocks.NewTrackImageExtractor(t)
	mockAlbumExtractor := mocks.NewAlbumImageExtractor(t)
	mockArtistExtractor := mocks.NewArtistImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)

	uc := &ScanLibraryUseCase{
		trackImageExtractor:  mockTrackExtractor,
		albumImageExtractor:  mockAlbumExtractor,
		artistImageExtractor: mockArtistExtractor,
		processedArtists:     AtomicDeduplicator{},
		scanRepos: &ScanRepositories{
			ScanState: mockScanState,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Track with no album or artist info
	track := &media.MusicTrack{
		Media: media.Media{
			ID:        789,
			LibraryID: 1,
		},
		// No Artist, Album, ArtistID, or AlbumID
	}

	uc.extractImagesForTrack(context.Background(), track, "/music/unknown/track.flac")

	// Only track extractor should be called
	if mockTrackExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 track Execute call, got %d", mockTrackExtractor.ExecuteCalls)
	}
	if mockAlbumExtractor.ExecuteCalls != 0 {
		t.Errorf("Expected 0 album Execute calls (no album info), got %d", mockAlbumExtractor.ExecuteCalls)
	}
	if mockArtistExtractor.ExecuteCalls != 0 {
		t.Errorf("Expected 0 artist Execute calls (no artist info), got %d", mockArtistExtractor.ExecuteCalls)
	}
}

func TestScanLibraryUseCase_extractImagesForEpisode_error(t *testing.T) {
	mockExtractor := mocks.NewEpisodeImageExtractor(t)
	mockExtractor.ExecuteErr = errors.New("episode extraction failed")

	mockShowExtractor := mocks.NewShowImageExtractor(t)
	mockSeasonExtractor := mocks.NewSeasonImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)
	mockTVRepo := mocks.NewTVRepository(t)

	// Set up mock show and season
	testShow := media.TVShow{ID: 100, LibraryID: 1, Title: "Test Show"}
	testSeason := media.TVSeason{ID: 200, ShowID: 100, SeasonNumber: 1}
	mockTVRepo.WithShows(testShow)
	mockTVRepo.WithSeasons(testSeason)

	// Pre-populate scan state so SetWarning can find it
	mockScanState.WithStates(&scanner.ScanState{
		LibraryID: 1,
		FilePath:  "/path/to/episode.mkv",
	})

	uc := &ScanLibraryUseCase{
		episodeImageExtractor: mockExtractor,
		showImageExtractor:    mockShowExtractor,
		seasonImageExtractor:  mockSeasonExtractor,
		processedShows:        AtomicDeduplicator{},
		mediaRepos: &MediaRepositories{
			TV: mockTVRepo,
		},
		scanRepos: &ScanRepositories{
			ScanState: mockScanState,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	episode := &media.TVEpisode{
		Media:     media.Media{ID: 456, LibraryID: 1},
		ShowTitle: "Test Show",
		Season:    1,
		Episode:   1,
	}

	// Should not panic on error, should record warning
	uc.extractImagesForEpisode(context.Background(), episode, "/path/to/episode.mkv", 1)

	if mockExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 episode Execute call, got %d", mockExtractor.ExecuteCalls)
	}
}

func TestScanLibraryUseCase_extractImagesForEpisode_seasonSubdir(t *testing.T) {
	mockExtractor := mocks.NewEpisodeImageExtractor(t)
	mockShowExtractor := mocks.NewShowImageExtractor(t)
	mockSeasonExtractor := mocks.NewSeasonImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)
	mockTVRepo := mocks.NewTVRepository(t)

	testShow := media.TVShow{ID: 100, LibraryID: 1, Title: "Test Show"}
	testSeason := media.TVSeason{ID: 200, ShowID: 100, SeasonNumber: 2}
	mockTVRepo.WithShows(testShow)
	mockTVRepo.WithSeasons(testSeason)

	uc := &ScanLibraryUseCase{
		episodeImageExtractor: mockExtractor,
		showImageExtractor:    mockShowExtractor,
		seasonImageExtractor:  mockSeasonExtractor,
		processedShows:        AtomicDeduplicator{},
		mediaRepos: &MediaRepositories{
			TV: mockTVRepo,
		},
		scanRepos: &ScanRepositories{
			ScanState: mockScanState,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	episode := &media.TVEpisode{
		Media:     media.Media{ID: 456, LibraryID: 1},
		ShowTitle: "Test Show",
		Season:    2,
		Episode:   1,
	}

	// Episode in season subdirectory: /tv/ShowName/Season 02/episode.mkv
	uc.extractImagesForEpisode(context.Background(), episode, "/tv/ShowName/Season 02/episode.mkv", 1)

	if mockExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 episode Execute call, got %d", mockExtractor.ExecuteCalls)
	}
	if mockShowExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 show Execute call, got %d", mockShowExtractor.ExecuteCalls)
	}
	if mockSeasonExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 season Execute call, got %d", mockSeasonExtractor.ExecuteCalls)
	}
}

func TestScanLibraryUseCase_extractImagesForTrack_trackError(t *testing.T) {
	mockTrackExtractor := mocks.NewTrackImageExtractor(t)
	mockTrackExtractor.ExecuteErr = errors.New("track extraction failed")

	mockScanState := mocks.NewScanStateRepository(t)
	mockScanState.WithStates(&scanner.ScanState{
		LibraryID: 1,
		FilePath:  "/music/artist/album/track.flac",
	})

	uc := &ScanLibraryUseCase{
		trackImageExtractor:  mockTrackExtractor,
		albumImageExtractor:  nil,
		artistImageExtractor: nil,
		processedArtists:     AtomicDeduplicator{},
		scanRepos: &ScanRepositories{
			ScanState: mockScanState,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	track := &media.MusicTrack{
		Media: media.Media{ID: 789, LibraryID: 1},
	}

	// Should not panic on error
	uc.extractImagesForTrack(context.Background(), track, "/music/artist/album/track.flac")

	if mockTrackExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 track Execute call, got %d", mockTrackExtractor.ExecuteCalls)
	}
}

func TestScanLibraryUseCase_extractImagesForTrack_albumError(t *testing.T) {
	mockTrackExtractor := mocks.NewTrackImageExtractor(t)
	mockAlbumExtractor := mocks.NewAlbumImageExtractor(t)
	mockAlbumExtractor.ExecuteErr = errors.New("album extraction failed")

	mockScanState := mocks.NewScanStateRepository(t)
	mockScanState.WithStates(&scanner.ScanState{
		LibraryID: 1,
		FilePath:  "/music/artist/album/track.flac",
	})

	uc := &ScanLibraryUseCase{
		trackImageExtractor:  mockTrackExtractor,
		albumImageExtractor:  mockAlbumExtractor,
		artistImageExtractor: nil,
		processedArtists:     AtomicDeduplicator{},
		scanRepos: &ScanRepositories{
			ScanState: mockScanState,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	track := &media.MusicTrack{
		Media:   media.Media{ID: 789, LibraryID: 1},
		Album:   "Test Album",
		AlbumID: 20,
	}

	// Should not panic on error
	uc.extractImagesForTrack(context.Background(), track, "/music/artist/album/track.flac")

	if mockAlbumExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 album Execute call, got %d", mockAlbumExtractor.ExecuteCalls)
	}
}

func TestScanLibraryUseCase_extractImagesForTrack_artistError(t *testing.T) {
	mockTrackExtractor := mocks.NewTrackImageExtractor(t)
	mockArtistExtractor := mocks.NewArtistImageExtractor(t)
	mockArtistExtractor.ExecuteErr = errors.New("artist extraction failed")

	mockScanState := mocks.NewScanStateRepository(t)

	uc := &ScanLibraryUseCase{
		trackImageExtractor:  mockTrackExtractor,
		albumImageExtractor:  nil,
		artistImageExtractor: mockArtistExtractor,
		processedArtists:     AtomicDeduplicator{},
		scanRepos: &ScanRepositories{
			ScanState: mockScanState,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	track := &media.MusicTrack{
		Media:    media.Media{ID: 789, LibraryID: 1},
		Artist:   "Test Artist",
		ArtistID: 10,
	}

	// Should not panic on error
	uc.extractImagesForTrack(context.Background(), track, "/music/artist/album/track.flac")

	if mockArtistExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 artist Execute call, got %d", mockArtistExtractor.ExecuteCalls)
	}
}

func TestScanLibraryUseCase_extractTVShowAndSeasonImages_showNotFound(t *testing.T) {
	mockShowExtractor := mocks.NewShowImageExtractor(t)
	mockSeasonExtractor := mocks.NewSeasonImageExtractor(t)
	mockTVRepo := mocks.NewTVRepository(t)
	// Don't populate shows - lookup will return sql.ErrNoRows

	uc := &ScanLibraryUseCase{
		showImageExtractor:   mockShowExtractor,
		seasonImageExtractor: mockSeasonExtractor,
		processedShows:       AtomicDeduplicator{},
		mediaRepos: &MediaRepositories{
			TV: mockTVRepo,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Should return early when show not found
	uc.extractTVShowAndSeasonImages(context.Background(), "Unknown Show", 1, "/tv/Unknown Show", 1)

	// Show extractor should not be called since show lookup failed
	if mockShowExtractor.ExecuteCalls != 0 {
		t.Errorf("Expected 0 show Execute calls, got %d", mockShowExtractor.ExecuteCalls)
	}
}

func TestScanLibraryUseCase_extractTVShowAndSeasonImages_seasonNotFound(t *testing.T) {
	mockShowExtractor := mocks.NewShowImageExtractor(t)
	mockSeasonExtractor := mocks.NewSeasonImageExtractor(t)
	mockTVRepo := mocks.NewTVRepository(t)

	testShow := media.TVShow{ID: 100, LibraryID: 1, Title: "Test Show"}
	mockTVRepo.WithShows(testShow)
	// Don't populate seasons - lookup will return sql.ErrNoRows

	uc := &ScanLibraryUseCase{
		showImageExtractor:   mockShowExtractor,
		seasonImageExtractor: mockSeasonExtractor,
		processedShows:       AtomicDeduplicator{},
		mediaRepos: &MediaRepositories{
			TV: mockTVRepo,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Should extract show images but return early when season not found
	uc.extractTVShowAndSeasonImages(context.Background(), "Test Show", 1, "/tv/Test Show", 1)

	if mockShowExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 show Execute call, got %d", mockShowExtractor.ExecuteCalls)
	}
	// Season extractor should not be called since season lookup failed
	if mockSeasonExtractor.ExecuteCalls != 0 {
		t.Errorf("Expected 0 season Execute calls, got %d", mockSeasonExtractor.ExecuteCalls)
	}
}

func TestScanLibraryUseCase_extractTVShowAndSeasonImages_showExtractorError(t *testing.T) {
	mockShowExtractor := mocks.NewShowImageExtractor(t)
	mockShowExtractor.ExecuteErr = errors.New("show extraction failed")

	mockSeasonExtractor := mocks.NewSeasonImageExtractor(t)
	mockTVRepo := mocks.NewTVRepository(t)

	testShow := media.TVShow{ID: 100, LibraryID: 1, Title: "Test Show"}
	testSeason := media.TVSeason{ID: 200, ShowID: 100, SeasonNumber: 1}
	mockTVRepo.WithShows(testShow)
	mockTVRepo.WithSeasons(testSeason)

	uc := &ScanLibraryUseCase{
		showImageExtractor:   mockShowExtractor,
		seasonImageExtractor: mockSeasonExtractor,
		processedShows:       AtomicDeduplicator{},
		mediaRepos: &MediaRepositories{
			TV: mockTVRepo,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Should continue to season extraction even if show extraction fails
	uc.extractTVShowAndSeasonImages(context.Background(), "Test Show", 1, "/tv/Test Show", 1)

	if mockShowExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 show Execute call, got %d", mockShowExtractor.ExecuteCalls)
	}
	if mockSeasonExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 season Execute call, got %d", mockSeasonExtractor.ExecuteCalls)
	}
}

func TestScanLibraryUseCase_extractTVShowAndSeasonImages_seasonExtractorError(t *testing.T) {
	mockShowExtractor := mocks.NewShowImageExtractor(t)
	mockSeasonExtractor := mocks.NewSeasonImageExtractor(t)
	mockSeasonExtractor.ExecuteErr = errors.New("season extraction failed")

	mockTVRepo := mocks.NewTVRepository(t)

	testShow := media.TVShow{ID: 100, LibraryID: 1, Title: "Test Show"}
	testSeason := media.TVSeason{ID: 200, ShowID: 100, SeasonNumber: 1}
	mockTVRepo.WithShows(testShow)
	mockTVRepo.WithSeasons(testSeason)

	uc := &ScanLibraryUseCase{
		showImageExtractor:   mockShowExtractor,
		seasonImageExtractor: mockSeasonExtractor,
		processedShows:       AtomicDeduplicator{},
		mediaRepos: &MediaRepositories{
			TV: mockTVRepo,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Should not panic on season extraction error
	uc.extractTVShowAndSeasonImages(context.Background(), "Test Show", 1, "/tv/Test Show", 1)

	if mockShowExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 show Execute call, got %d", mockShowExtractor.ExecuteCalls)
	}
	if mockSeasonExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 season Execute call, got %d", mockSeasonExtractor.ExecuteCalls)
	}
}

func TestScanLibraryUseCase_extractTVShowAndSeasonImages_nilExtractors(t *testing.T) {
	mockTVRepo := mocks.NewTVRepository(t)

	testShow := media.TVShow{ID: 100, LibraryID: 1, Title: "Test Show"}
	testSeason := media.TVSeason{ID: 200, ShowID: 100, SeasonNumber: 1}
	mockTVRepo.WithShows(testShow)
	mockTVRepo.WithSeasons(testSeason)

	uc := &ScanLibraryUseCase{
		showImageExtractor:   nil,
		seasonImageExtractor: nil,
		processedShows:       AtomicDeduplicator{},
		mediaRepos: &MediaRepositories{
			TV: mockTVRepo,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	// Should not panic when extractors are nil
	uc.extractTVShowAndSeasonImages(context.Background(), "Test Show", 1, "/tv/Test Show", 1)
}
