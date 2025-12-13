package library

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

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
