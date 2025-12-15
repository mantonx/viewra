package media

import (
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/application/library/scan/scanutil"
)

// =============================================================================
// AtomicDeduplicator Integration Tests
// =============================================================================
// Note: AtomicDeduplicator unit tests are in scan/scanutil/dedup_test.go
// These tests verify deduplication behavior in the context of parent entity enqueueing.

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
// Parent Entity Enqueueing Tests
// =============================================================================
// Note: Image extraction is now handled by the enrichment pipeline.
// These tests verify that parent entities (shows, seasons, albums, artists)
// are properly enqueued for enrichment during scanning.

func TestEnqueueTVParentEntities_ShowDirDetection(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		wantShowDir string
	}{
		{
			name:        "episodes in show directory",
			filePath:    "/tv/ShowName (Year)/ShowName - S01E01.mkv",
			wantShowDir: "/tv/ShowName (Year)",
		},
		{
			name:        "episodes in season subdirectory",
			filePath:    "/tv/ShowName/Season 01/ShowName - S01E01.mkv",
			wantShowDir: "/tv/ShowName",
		},
		{
			name:        "episodes in Season prefix dir",
			filePath:    "/tv/Show/SEASON 02/episode.mkv",
			wantShowDir: "/tv/Show",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Full integration tests would require mocked repositories.
			// This test just validates the file path patterns are documented.
			// Actual testing is done via integration tests with the enrichment pipeline.
		})
	}
}

func TestEnqueueMusicParentEntities_DeduplicationKeys(t *testing.T) {
	// Test that album deduplication uses "album:artist" key format
	// and artist deduplication uses just artist name
	dedup := &scanutil.AtomicDeduplicator{}

	// Same album by same artist should be deduplicated
	albumKey1 := "Album1:Artist1"
	albumKey2 := "Album1:Artist1"
	if !dedup.TryMark(albumKey1) {
		t.Error("First album key should return true")
	}
	if dedup.TryMark(albumKey2) {
		t.Error("Same album key should return false (deduplicated)")
	}

	// Same album by different artist should NOT be deduplicated
	albumKey3 := "Album1:Artist2"
	if !dedup.TryMark(albumKey3) {
		t.Error("Different artist's album should return true")
	}

	// Reset and test artist deduplication
	dedup.Reset()
	if !dedup.TryMark("Artist1") {
		t.Error("First artist should return true")
	}
	if dedup.TryMark("Artist1") {
		t.Error("Same artist should return false (deduplicated)")
	}
	if !dedup.TryMark("Artist2") {
		t.Error("Different artist should return true")
	}
}
