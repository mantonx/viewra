package media

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/application/library/scan/scanutil"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// =============================================================================
// AtomicDeduplicator Integration Tests
// =============================================================================
// Note: AtomicDeduplicator unit tests are in scan/scanutil/dedup_test.go
// These tests verify deduplication behavior in the context of image extraction.

func TestAtomicDeduplicator_ArtistProcessing(t *testing.T) {
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
			dedup := &scanutil.AtomicDeduplicator{}

			// First call
			result1 := dedup.TryMark(tt.artistName)
			if result1 != tt.expectedFirst {
				t.Errorf("First call: got %v, want %v", result1, tt.expectedFirst)
			}

			// Second call
			if tt.callCount >= 2 {
				result2 := dedup.TryMark(tt.artistName)
				if result2 != tt.expectedSecond {
					t.Errorf("Second call: got %v, want %v", result2, tt.expectedSecond)
				}
			}

			// Different artist test
			if tt.name == "different artists both return true" {
				result3 := dedup.TryMark("DifferentArtist")
				if result3 != tt.expectedSecond {
					t.Errorf("Different artist call: got %v, want %v", result3, tt.expectedSecond)
				}
			}
		})
	}
}

func TestAtomicDeduplicator_Concurrent(t *testing.T) {
	dedup := &scanutil.AtomicDeduplicator{}

	const numGoroutines = 100
	const key = "TestKey"

	var wg sync.WaitGroup
	results := make(chan bool, numGoroutines)

	// Launch concurrent goroutines trying to mark the same key
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- dedup.TryMark(key)
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

// Benchmark for concurrent processing
func BenchmarkAtomicDeduplicator(b *testing.B) {
	dedup := &scanutil.AtomicDeduplicator{}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Use different keys to avoid contention
			key := string(rune('A' + (i % 26)))
			dedup.TryMark(key)
			i++
		}
	})
}

// =============================================================================
// Image Extraction Helper
// =============================================================================

// makeImageDeps creates a Deps instance for image extraction tests.
func makeImageDeps(
	movieExtractor MovieImageExtractor,
	episodeExtractor TVEpisodeImageExtractor,
	showExtractor TVShowImageExtractor,
	seasonExtractor TVSeasonImageExtractor,
	trackExtractor MusicTrackImageExtractor,
	albumExtractor MusicAlbumImageExtractor,
	artistExtractor MusicArtistImageExtractor,
	scanRepos *scan.ScanRepositories,
	mediaRepos *scan.MediaRepositories,
	processedArtists *scanutil.AtomicDeduplicator,
	processedShows *scanutil.AtomicDeduplicator,
) *Deps {
	return &Deps{
		MovieExtractor:   movieExtractor,
		EpisodeExtractor: episodeExtractor,
		ShowExtractor:    showExtractor,
		SeasonExtractor:  seasonExtractor,
		TrackExtractor:   trackExtractor,
		AlbumExtractor:   albumExtractor,
		ArtistExtractor:  artistExtractor,
		ScanRepos:        scanRepos,
		MediaRepos:       mediaRepos,
		ProcessedArtists: processedArtists,
		ProcessedShows:   processedShows,
		Logger:           testLogger(),
	}
}

// =============================================================================
// Nil Extractor Tests
// =============================================================================

func TestExtractImagesForMovie_nilExtractor(t *testing.T) {
	deps := makeImageDeps(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// Should not panic when extractor is nil
	ExtractImagesForMovie(context.Background(), deps, nil, "/test/movie.mp4")
}

func TestExtractImagesForEpisode_nilExtractor(t *testing.T) {
	deps := makeImageDeps(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// Should not panic when extractor is nil
	ExtractImagesForEpisode(context.Background(), deps, nil, "/test/episode.mp4", 1)
}

func TestExtractImagesForTrack_nilExtractors(t *testing.T) {
	processedArtists := &scanutil.AtomicDeduplicator{}
	deps := makeImageDeps(nil, nil, nil, nil, nil, nil, nil, nil, nil, processedArtists, nil)

	// Should not panic when extractors are nil (track must be valid)
	track := &media.MusicTrack{
		Media: media.Media{
			ID:        1,
			LibraryID: 1,
		},
	}
	ExtractImagesForTrack(context.Background(), deps, track, "/test/track.mp3")
}

// =============================================================================
// Movie Image Extraction Tests
// =============================================================================

func TestExtractImagesForMovie_success(t *testing.T) {
	mockExtractor := mocks.NewMovieImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)

	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	deps := makeImageDeps(mockExtractor, nil, nil, nil, nil, nil, nil, scanRepos, nil, nil, nil)

	movie := &media.Movie{
		Media: media.Media{
			ID:        123,
			LibraryID: 1,
		},
	}

	ExtractImagesForMovie(context.Background(), deps, movie, "/path/to/movie.mkv")

	if mockExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 Execute call, got %d", mockExtractor.ExecuteCalls)
	}
}

func TestExtractImagesForMovie_error(t *testing.T) {
	mockExtractor := mocks.NewMovieImageExtractor(t)
	mockExtractor.ExecuteErr = errors.New("extraction failed")

	mockScanState := mocks.NewScanStateRepository(t)
	mockScanState.WithStates(&scanner.ScanState{
		LibraryID: 1,
		FilePath:  "/path/to/movie.mkv",
	})

	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	deps := makeImageDeps(mockExtractor, nil, nil, nil, nil, nil, nil, scanRepos, nil, nil, nil)

	movie := &media.Movie{
		Media: media.Media{
			ID:        123,
			LibraryID: 1,
		},
	}

	// Should not panic on error, should record warning
	ExtractImagesForMovie(context.Background(), deps, movie, "/path/to/movie.mkv")

	if mockExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 Execute call, got %d", mockExtractor.ExecuteCalls)
	}
}

// =============================================================================
// Episode Image Extraction Tests
// =============================================================================

func TestExtractImagesForEpisode_success(t *testing.T) {
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

	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	mediaRepos := &scan.MediaRepositories{TV: mockTVRepo}
	processedShows := &scanutil.AtomicDeduplicator{}
	deps := makeImageDeps(nil, mockExtractor, mockShowExtractor, mockSeasonExtractor, nil, nil, nil, scanRepos, mediaRepos, nil, processedShows)

	episode := &media.TVEpisode{
		Media: media.Media{
			ID:        456,
			LibraryID: 1,
		},
		ShowTitle: "Test Show",
		Season:    1,
		Episode:   1,
	}

	ExtractImagesForEpisode(context.Background(), deps, episode, "/path/to/episode.mkv", 1)

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

func TestExtractImagesForEpisode_error(t *testing.T) {
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

	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	mediaRepos := &scan.MediaRepositories{TV: mockTVRepo}
	processedShows := &scanutil.AtomicDeduplicator{}
	deps := makeImageDeps(nil, mockExtractor, mockShowExtractor, mockSeasonExtractor, nil, nil, nil, scanRepos, mediaRepos, nil, processedShows)

	episode := &media.TVEpisode{
		Media:     media.Media{ID: 456, LibraryID: 1},
		ShowTitle: "Test Show",
		Season:    1,
		Episode:   1,
	}

	// Should not panic on error, should record warning
	ExtractImagesForEpisode(context.Background(), deps, episode, "/path/to/episode.mkv", 1)

	if mockExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 episode Execute call, got %d", mockExtractor.ExecuteCalls)
	}
}

func TestExtractImagesForEpisode_seasonSubdir(t *testing.T) {
	mockExtractor := mocks.NewEpisodeImageExtractor(t)
	mockShowExtractor := mocks.NewShowImageExtractor(t)
	mockSeasonExtractor := mocks.NewSeasonImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)
	mockTVRepo := mocks.NewTVRepository(t)

	testShow := media.TVShow{ID: 100, LibraryID: 1, Title: "Test Show"}
	testSeason := media.TVSeason{ID: 200, ShowID: 100, SeasonNumber: 2}
	mockTVRepo.WithShows(testShow)
	mockTVRepo.WithSeasons(testSeason)

	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	mediaRepos := &scan.MediaRepositories{TV: mockTVRepo}
	processedShows := &scanutil.AtomicDeduplicator{}
	deps := makeImageDeps(nil, mockExtractor, mockShowExtractor, mockSeasonExtractor, nil, nil, nil, scanRepos, mediaRepos, nil, processedShows)

	episode := &media.TVEpisode{
		Media:     media.Media{ID: 456, LibraryID: 1},
		ShowTitle: "Test Show",
		Season:    2,
		Episode:   1,
	}

	// Episode in season subdirectory: /tv/ShowName/Season 02/episode.mkv
	ExtractImagesForEpisode(context.Background(), deps, episode, "/tv/ShowName/Season 02/episode.mkv", 1)

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

// =============================================================================
// Track Image Extraction Tests
// =============================================================================

func TestExtractImagesForTrack_success(t *testing.T) {
	mockTrackExtractor := mocks.NewTrackImageExtractor(t)
	mockAlbumExtractor := mocks.NewAlbumImageExtractor(t)
	mockArtistExtractor := mocks.NewArtistImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)

	processedArtists := &scanutil.AtomicDeduplicator{}
	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	deps := makeImageDeps(nil, nil, nil, nil, mockTrackExtractor, mockAlbumExtractor, mockArtistExtractor, scanRepos, nil, processedArtists, nil)

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

	ExtractImagesForTrack(context.Background(), deps, track, "/music/artist/album/track.flac")

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

func TestExtractImagesForTrack_artistDedup(t *testing.T) {
	mockTrackExtractor := mocks.NewTrackImageExtractor(t)
	mockArtistExtractor := mocks.NewArtistImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)

	processedArtists := &scanutil.AtomicDeduplicator{}
	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	deps := makeImageDeps(nil, nil, nil, nil, mockTrackExtractor, nil, mockArtistExtractor, scanRepos, nil, processedArtists, nil)

	track := &media.MusicTrack{
		Media: media.Media{
			ID:        789,
			LibraryID: 1,
		},
		Artist:   "Same Artist",
		ArtistID: 10,
	}

	// Extract images for same artist twice
	ExtractImagesForTrack(context.Background(), deps, track, "/music/artist/album1/track1.flac")
	ExtractImagesForTrack(context.Background(), deps, track, "/music/artist/album1/track2.flac")

	// Track extractor should be called twice
	if mockTrackExtractor.ExecuteCalls != 2 {
		t.Errorf("Expected 2 track Execute calls, got %d", mockTrackExtractor.ExecuteCalls)
	}

	// Artist extractor should only be called once (deduplication)
	if mockArtistExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 artist Execute call (deduplicated), got %d", mockArtistExtractor.ExecuteCalls)
	}
}

func TestExtractImagesForTrack_noAlbumOrArtist(t *testing.T) {
	mockTrackExtractor := mocks.NewTrackImageExtractor(t)
	mockAlbumExtractor := mocks.NewAlbumImageExtractor(t)
	mockArtistExtractor := mocks.NewArtistImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)

	processedArtists := &scanutil.AtomicDeduplicator{}
	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	deps := makeImageDeps(nil, nil, nil, nil, mockTrackExtractor, mockAlbumExtractor, mockArtistExtractor, scanRepos, nil, processedArtists, nil)

	// Track with no album or artist info
	track := &media.MusicTrack{
		Media: media.Media{
			ID:        789,
			LibraryID: 1,
		},
		// No Artist, Album, ArtistID, or AlbumID
	}

	ExtractImagesForTrack(context.Background(), deps, track, "/music/unknown/track.flac")

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

func TestExtractImagesForTrack_trackError(t *testing.T) {
	mockTrackExtractor := mocks.NewTrackImageExtractor(t)
	mockTrackExtractor.ExecuteErr = errors.New("track extraction failed")

	mockScanState := mocks.NewScanStateRepository(t)
	mockScanState.WithStates(&scanner.ScanState{
		LibraryID: 1,
		FilePath:  "/music/artist/album/track.flac",
	})

	processedArtists := &scanutil.AtomicDeduplicator{}
	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	deps := makeImageDeps(nil, nil, nil, nil, mockTrackExtractor, nil, nil, scanRepos, nil, processedArtists, nil)

	track := &media.MusicTrack{
		Media: media.Media{ID: 789, LibraryID: 1},
	}

	// Should not panic on error
	ExtractImagesForTrack(context.Background(), deps, track, "/music/artist/album/track.flac")

	if mockTrackExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 track Execute call, got %d", mockTrackExtractor.ExecuteCalls)
	}
}

func TestExtractImagesForTrack_albumError(t *testing.T) {
	mockTrackExtractor := mocks.NewTrackImageExtractor(t)
	mockAlbumExtractor := mocks.NewAlbumImageExtractor(t)
	mockAlbumExtractor.ExecuteErr = errors.New("album extraction failed")

	mockScanState := mocks.NewScanStateRepository(t)
	mockScanState.WithStates(&scanner.ScanState{
		LibraryID: 1,
		FilePath:  "/music/artist/album/track.flac",
	})

	processedArtists := &scanutil.AtomicDeduplicator{}
	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	deps := makeImageDeps(nil, nil, nil, nil, mockTrackExtractor, mockAlbumExtractor, nil, scanRepos, nil, processedArtists, nil)

	track := &media.MusicTrack{
		Media:   media.Media{ID: 789, LibraryID: 1},
		Album:   "Test Album",
		AlbumID: 20,
	}

	// Should not panic on error
	ExtractImagesForTrack(context.Background(), deps, track, "/music/artist/album/track.flac")

	if mockAlbumExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 album Execute call, got %d", mockAlbumExtractor.ExecuteCalls)
	}
}

func TestExtractImagesForTrack_artistError(t *testing.T) {
	mockTrackExtractor := mocks.NewTrackImageExtractor(t)
	mockArtistExtractor := mocks.NewArtistImageExtractor(t)
	mockArtistExtractor.ExecuteErr = errors.New("artist extraction failed")

	mockScanState := mocks.NewScanStateRepository(t)

	processedArtists := &scanutil.AtomicDeduplicator{}
	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	deps := makeImageDeps(nil, nil, nil, nil, mockTrackExtractor, nil, mockArtistExtractor, scanRepos, nil, processedArtists, nil)

	track := &media.MusicTrack{
		Media:    media.Media{ID: 789, LibraryID: 1},
		Artist:   "Test Artist",
		ArtistID: 10,
	}

	// Should not panic on error
	ExtractImagesForTrack(context.Background(), deps, track, "/music/artist/album/track.flac")

	if mockArtistExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 artist Execute call, got %d", mockArtistExtractor.ExecuteCalls)
	}
}

// =============================================================================
// extractTVShowAndSeasonImages Tests
// =============================================================================

func TestExtractTVShowAndSeasonImages_showNotFound(t *testing.T) {
	mockShowExtractor := mocks.NewShowImageExtractor(t)
	mockSeasonExtractor := mocks.NewSeasonImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)
	mockTVRepo := mocks.NewTVRepository(t)
	// No shows added - should log warning and return early

	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	mediaRepos := &scan.MediaRepositories{TV: mockTVRepo}
	deps := makeImageDeps(nil, nil, mockShowExtractor, mockSeasonExtractor, nil, nil, nil, scanRepos, mediaRepos, nil, nil)

	// Should not panic when show not found
	extractTVShowAndSeasonImages(context.Background(), deps, "NonExistent Show", 1, "/path/to/show", 1)

	// No extractors should be called
	if mockShowExtractor.ExecuteCalls != 0 {
		t.Errorf("Expected 0 show Execute calls, got %d", mockShowExtractor.ExecuteCalls)
	}
	if mockSeasonExtractor.ExecuteCalls != 0 {
		t.Errorf("Expected 0 season Execute calls, got %d", mockSeasonExtractor.ExecuteCalls)
	}
}

func TestExtractTVShowAndSeasonImages_seasonNotFound(t *testing.T) {
	mockShowExtractor := mocks.NewShowImageExtractor(t)
	mockSeasonExtractor := mocks.NewSeasonImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)
	mockTVRepo := mocks.NewTVRepository(t)

	// Add show but no season
	testShow := media.TVShow{ID: 100, LibraryID: 1, Title: "Test Show"}
	mockTVRepo.WithShows(testShow)
	// No seasons - should log warning after show extraction

	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	mediaRepos := &scan.MediaRepositories{TV: mockTVRepo}
	deps := makeImageDeps(nil, nil, mockShowExtractor, mockSeasonExtractor, nil, nil, nil, scanRepos, mediaRepos, nil, nil)

	// Should continue with show extraction but skip season
	extractTVShowAndSeasonImages(context.Background(), deps, "Test Show", 1, "/path/to/show", 1)

	// Show extractor should be called
	if mockShowExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 show Execute call, got %d", mockShowExtractor.ExecuteCalls)
	}
	// Season extractor should not be called since season not found
	if mockSeasonExtractor.ExecuteCalls != 0 {
		t.Errorf("Expected 0 season Execute calls (season not found), got %d", mockSeasonExtractor.ExecuteCalls)
	}
}

func TestExtractTVShowAndSeasonImages_showExtractorError(t *testing.T) {
	mockShowExtractor := mocks.NewShowImageExtractor(t)
	mockShowExtractor.ExecuteErr = errors.New("show extraction failed")
	mockSeasonExtractor := mocks.NewSeasonImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)
	mockTVRepo := mocks.NewTVRepository(t)

	testShow := media.TVShow{ID: 100, LibraryID: 1, Title: "Test Show"}
	testSeason := media.TVSeason{ID: 200, ShowID: 100, SeasonNumber: 1}
	mockTVRepo.WithShows(testShow)
	mockTVRepo.WithSeasons(testSeason)

	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	mediaRepos := &scan.MediaRepositories{TV: mockTVRepo}
	deps := makeImageDeps(nil, nil, mockShowExtractor, mockSeasonExtractor, nil, nil, nil, scanRepos, mediaRepos, nil, nil)

	// Should not panic on show extraction error, should continue to season
	extractTVShowAndSeasonImages(context.Background(), deps, "Test Show", 1, "/path/to/show", 1)

	if mockShowExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 show Execute call, got %d", mockShowExtractor.ExecuteCalls)
	}
	if mockSeasonExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 season Execute call (despite show error), got %d", mockSeasonExtractor.ExecuteCalls)
	}
}

func TestExtractTVShowAndSeasonImages_seasonExtractorError(t *testing.T) {
	mockShowExtractor := mocks.NewShowImageExtractor(t)
	mockSeasonExtractor := mocks.NewSeasonImageExtractor(t)
	mockSeasonExtractor.ExecuteErr = errors.New("season extraction failed")
	mockScanState := mocks.NewScanStateRepository(t)
	mockTVRepo := mocks.NewTVRepository(t)

	testShow := media.TVShow{ID: 100, LibraryID: 1, Title: "Test Show"}
	testSeason := media.TVSeason{ID: 200, ShowID: 100, SeasonNumber: 1}
	mockTVRepo.WithShows(testShow)
	mockTVRepo.WithSeasons(testSeason)

	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	mediaRepos := &scan.MediaRepositories{TV: mockTVRepo}
	deps := makeImageDeps(nil, nil, mockShowExtractor, mockSeasonExtractor, nil, nil, nil, scanRepos, mediaRepos, nil, nil)

	// Should not panic on season extraction error
	extractTVShowAndSeasonImages(context.Background(), deps, "Test Show", 1, "/path/to/show", 1)

	if mockSeasonExtractor.ExecuteCalls != 1 {
		t.Errorf("Expected 1 season Execute call, got %d", mockSeasonExtractor.ExecuteCalls)
	}
}

func TestExtractTVShowAndSeasonImages_showDeduplication(t *testing.T) {
	mockShowExtractor := mocks.NewShowImageExtractor(t)
	mockSeasonExtractor := mocks.NewSeasonImageExtractor(t)
	mockScanState := mocks.NewScanStateRepository(t)
	mockTVRepo := mocks.NewTVRepository(t)

	testShow := media.TVShow{ID: 100, LibraryID: 1, Title: "Test Show"}
	testSeason1 := media.TVSeason{ID: 200, ShowID: 100, SeasonNumber: 1}
	testSeason2 := media.TVSeason{ID: 201, ShowID: 100, SeasonNumber: 2}
	mockTVRepo.WithShows(testShow)
	mockTVRepo.WithSeasons(testSeason1, testSeason2)

	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	mediaRepos := &scan.MediaRepositories{TV: mockTVRepo}
	deps := makeImageDeps(nil, nil, mockShowExtractor, mockSeasonExtractor, nil, nil, nil, scanRepos, mediaRepos, nil, nil)

	// Extract for same show twice, different seasons
	extractTVShowAndSeasonImages(context.Background(), deps, "Test Show", 1, "/path/to/show", 1)
	extractTVShowAndSeasonImages(context.Background(), deps, "Test Show", 1, "/path/to/show", 2)

	// Show extractor called for each execution (processedShows only controls enrichment enqueueing, not show image extraction)
	if mockShowExtractor.ExecuteCalls != 2 {
		t.Errorf("Expected 2 show Execute calls, got %d", mockShowExtractor.ExecuteCalls)
	}
	// Season extractor should be called twice (different seasons)
	if mockSeasonExtractor.ExecuteCalls != 2 {
		t.Errorf("Expected 2 season Execute calls, got %d", mockSeasonExtractor.ExecuteCalls)
	}
}

func TestExtractTVShowAndSeasonImages_nilExtractors(t *testing.T) {
	mockScanState := mocks.NewScanStateRepository(t)
	mockTVRepo := mocks.NewTVRepository(t)

	testShow := media.TVShow{ID: 100, LibraryID: 1, Title: "Test Show"}
	testSeason := media.TVSeason{ID: 200, ShowID: 100, SeasonNumber: 1}
	mockTVRepo.WithShows(testShow)
	mockTVRepo.WithSeasons(testSeason)

	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	mediaRepos := &scan.MediaRepositories{TV: mockTVRepo}
	// nil extractors
	deps := makeImageDeps(nil, nil, nil, nil, nil, nil, nil, scanRepos, mediaRepos, nil, nil)

	// Should not panic with nil extractors
	extractTVShowAndSeasonImages(context.Background(), deps, "Test Show", 1, "/path/to/show", 1)
}

// =============================================================================
// recordImageWarning Tests
// =============================================================================

func TestRecordImageWarning_success(t *testing.T) {
	mockScanState := mocks.NewScanStateRepository(t)
	mockScanState.WithStates(&scanner.ScanState{
		LibraryID: 1,
		FilePath:  "/path/to/file.mp4",
	})

	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	deps := makeImageDeps(nil, nil, nil, nil, nil, nil, nil, scanRepos, nil, nil, nil)

	// Should not panic
	recordImageWarning(context.Background(), deps, 1, "/path/to/file.mp4", errors.New("test error"))
}

func TestRecordImageWarning_setWarningError(t *testing.T) {
	mockScanState := mocks.NewScanStateRepository(t)
	mockScanState.SetWarningErr = errors.New("failed to set warning")

	scanRepos := &scan.ScanRepositories{ScanState: mockScanState}
	deps := makeImageDeps(nil, nil, nil, nil, nil, nil, nil, scanRepos, nil, nil, nil)

	// Should not panic even when SetWarning fails
	recordImageWarning(context.Background(), deps, 1, "/path/to/file.mp4", errors.New("original error"))
}
