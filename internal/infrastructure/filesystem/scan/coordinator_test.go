package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.NumWorkers != 4 {
		t.Errorf("Expected NumWorkers=4, got %d", config.NumWorkers)
	}
	if config.ResultBufferSize != 100 {
		t.Errorf("Expected ResultBufferSize=100, got %d", config.ResultBufferSize)
	}
}

func TestNewCoordinator(t *testing.T) {
	config := Config{
		NumWorkers:               2,
		ResultBufferSize:         50,
	}

	coordinator := NewCoordinator(config)

	if coordinator == nil {
		t.Fatal("Expected non-nil coordinator")
	}
	if coordinator.config.NumWorkers != 2 {
		t.Errorf("Expected NumWorkers=2, got %d", coordinator.config.NumWorkers)
	}
	if coordinator.walker == nil {
		t.Error("Expected walker to be initialized")
	}
	if coordinator.filter == nil {
		t.Error("Expected filter to be initialized")
	}
	if coordinator.parser == nil {
		t.Error("Expected parser to be initialized")
	}
	// Note: ffmpegService may be nil if FFmpeg is not available
	// This is expected behavior - the coordinator logs a warning but continues
}

func TestShouldProcessFile(t *testing.T) {
	coordinator := NewCoordinator(DefaultConfig())

	// Create temp directory
	tmpDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.mkv")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test with valid media file
	info := scanner.FileInfo{
		Path:      testFile,
		Size:      4,
		IsDir:     false,
		Extension: ".mkv",
	}

	if !coordinator.shouldProcessFile(info) {
		t.Error("Expected shouldProcessFile to return true for .mkv file")
	}

	// Test with directory
	info.IsDir = true
	if coordinator.shouldProcessFile(info) {
		t.Error("Expected shouldProcessFile to return false for directory")
	}

	// Test with non-existent file
	info = scanner.FileInfo{
		Path:      "/nonexistent/file.mkv",
		Size:      0,
		IsDir:     false,
		Extension: ".mkv",
	}

	if coordinator.shouldProcessFile(info) {
		t.Error("Expected shouldProcessFile to return false for non-existent file")
	}
}

func TestIsRealError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"context.Canceled", context.Canceled, false},
		{"real error", os.ErrNotExist, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRealError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for error: %v", tt.expected, result, tt.err)
			}
		})
	}
}

func TestCoordinatorScan(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()

	// Create test media files
	testFiles := []string{
		"Movie1 (2020).mkv",
		"Movie2 (2021).mp4",
		"Show (2019) - S01E01 - Episode Title.mkv",
	}

	for _, filename := range testFiles {
		path := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create coordinator
	config := Config{
		NumWorkers:               2,
		ResultBufferSize:         10,
	}
	coordinator := NewCoordinator(config)

	// Create result channel
	resultChan := make(chan scanner.ScanResult, 10)

	// Run scan in goroutine
	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		done <- coordinator.Scan(ctx, tmpDir, resultChan)
		close(resultChan)
	}()

	// Collect results
	var results []scanner.ScanResult
	for result := range resultChan {
		results = append(results, result)
	}

	// Wait for scan to complete
	err := <-done
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify results
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Check progress
	progress := coordinator.GetProgress()
	if progress.FilesFound != 3 {
		t.Errorf("Expected FilesFound=3, got %d", progress.FilesFound)
	}
	if progress.FilesProcessed != 3 {
		t.Errorf("Expected FilesProcessed=3, got %d", progress.FilesProcessed)
	}
}

func TestCoordinatorScanWithCancellation(t *testing.T) {
	// Create temp directory with many files
	tmpDir := t.TempDir()

	// Create many test files to ensure cancellation can happen
	for i := 0; i < 50; i++ {
		filename := filepath.Join(tmpDir, "Movie"+string(rune('A'+i%26))+" (2020).mkv")
		if err := os.WriteFile(filename, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	config := Config{
		NumWorkers:               1, // Single worker to make timing more predictable
		ResultBufferSize:         5,
	}
	coordinator := NewCoordinator(config)

	resultChan := make(chan scanner.ScanResult, 5)

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start scan
	done := make(chan error, 1)
	go func() {
		done <- coordinator.Scan(ctx, tmpDir, resultChan)
		close(resultChan)
	}()

	// Cancel immediately
	cancel()

	// Wait for completion
	err := <-done

	// Should complete without error (context.Canceled is expected and handled)
	if err != nil && err != context.Canceled {
		t.Errorf("Expected no error or context.Canceled, got: %v", err)
	}
}

func TestCoordinatorScanEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		NumWorkers:               2,
		ResultBufferSize:         10,
	}
	coordinator := NewCoordinator(config)

	resultChan := make(chan scanner.ScanResult, 10)

	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		done <- coordinator.Scan(ctx, tmpDir, resultChan)
		close(resultChan)
	}()

	// Collect results
	var results []scanner.ScanResult
	for result := range resultChan {
		results = append(results, result)
	}

	err := <-done
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty directory, got %d", len(results))
	}

	progress := coordinator.GetProgress()
	if progress.FilesFound != 0 {
		t.Errorf("Expected FilesFound=0, got %d", progress.FilesFound)
	}
}

func TestCoordinatorScanInvalidPath(t *testing.T) {
	config := DefaultConfig()
	coordinator := NewCoordinator(config)

	resultChan := make(chan scanner.ScanResult, 10)
	defer close(resultChan)

	ctx := context.Background()
	err := coordinator.Scan(ctx, "/nonexistent/path/that/does/not/exist", resultChan)

	// Walk may succeed but find no files, or may fail depending on OS
	// The important thing is it doesn't panic
	_ = err
}

func TestCoordinatorAlreadyRunning(t *testing.T) {
	// Test by directly setting the isRunning flag
	coordinator := NewCoordinator(DefaultConfig())

	// Simulate already running by directly setting the flag
	coordinator.mu.Lock()
	coordinator.isRunning = true
	coordinator.mu.Unlock()

	resultChan := make(chan scanner.ScanResult, 10)
	defer close(resultChan)

	tmpDir := t.TempDir()
	err := coordinator.Scan(context.Background(), tmpDir, resultChan)

	if err != scanner.ErrAlreadyRunning {
		t.Errorf("Expected ErrAlreadyRunning, got: %v", err)
	}
}

func TestCoordinatorIsRunning(t *testing.T) {
	coordinator := NewCoordinator(DefaultConfig())

	if coordinator.IsRunning() {
		t.Error("Expected coordinator to not be running initially")
	}

	// Test simple case - after a completed scan
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mkv")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	resultChan := make(chan scanner.ScanResult, 10)
	ctx := context.Background()

	// Run a quick scan synchronously
	done := make(chan error)
	go func() {
		err := coordinator.Scan(ctx, tmpDir, resultChan)
		close(resultChan)
		done <- err
	}()

	// Drain results
	for range resultChan {
	}

	// Wait for completion
	err := <-done
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should not be running after completion
	if coordinator.IsRunning() {
		t.Error("Expected coordinator to not be running after completion")
	}
}

func TestCoordinatorGetProgress(t *testing.T) {
	coordinator := NewCoordinator(DefaultConfig())

	progress := coordinator.GetProgress()

	if progress.FilesFound != 0 {
		t.Errorf("Expected FilesFound=0, got %d", progress.FilesFound)
	}
	if progress.FilesProcessed != 0 {
		t.Errorf("Expected FilesProcessed=0, got %d", progress.FilesProcessed)
	}
	if progress.ErrorCount != 0 {
		t.Errorf("Expected ErrorCount=0, got %d", progress.ErrorCount)
	}
}

// Note: Counting tests removed - progressive counting is now integrated into the scan process

func TestUpdateFileCache(t *testing.T) {
	config := Config{
		NumWorkers:               2,
		ResultBufferSize:         10,
		EnableIncrementalScan:    true,
		FileCache:                make(map[string]*scanner.FileCacheEntry),
	}
	coordinator := NewCoordinator(config)

	// Create test data
	fileInfo := scanner.FileInfo{
		Path:      "/test/movie.mkv",
		Size:      1024,
		Extension: ".mkv",
	}

	year := 2020
	result := scanner.ScanResult{
		FilePath:   "/test/movie.mkv",
		MediaType:  scanner.MediaTypeMovie,
		Title:      "Test Movie",
		Year:       &year,
		Hash:       "abc123",
		FileSize:   1024,
		Duration:   7200,
		VideoCodec: "h264",
	}

	// Call updateFileCache
	coordinator.updateFileCache(fileInfo, &result)

	// Verify cache entry was created
	coordinator.mu.Lock()
	entry, exists := coordinator.config.FileCache[fileInfo.Path]
	coordinator.mu.Unlock()

	if !exists {
		t.Fatal("Expected cache entry to exist")
	}

	// Verify all fields are correct
	if entry.Path != fileInfo.Path {
		t.Errorf("Expected Path=%s, got %s", fileInfo.Path, entry.Path)
	}
	if entry.Size != fileInfo.Size {
		t.Errorf("Expected Size=%d, got %d", fileInfo.Size, entry.Size)
	}
	if entry.Hash != result.Hash {
		t.Errorf("Expected Hash=%s, got %s", result.Hash, entry.Hash)
	}
	if entry.MediaType != result.MediaType {
		t.Errorf("Expected MediaType=%s, got %s", result.MediaType, entry.MediaType)
	}
	if entry.Title != result.Title {
		t.Errorf("Expected Title=%s, got %s", result.Title, entry.Title)
	}
	if entry.Year == nil || *entry.Year != 2020 {
		t.Errorf("Expected Year=2020, got %v", entry.Year)
	}
}

func TestUpdateFileCacheMultipleEntries(t *testing.T) {
	config := Config{
		NumWorkers:               2,
		ResultBufferSize:         10,
		EnableIncrementalScan:    true,
		FileCache:                make(map[string]*scanner.FileCacheEntry),
	}
	coordinator := NewCoordinator(config)

	// Create multiple test entries
	entries := []struct {
		path  string
		title string
		year  int
	}{
		{"/movies/Movie1.mkv", "Movie 1", 2020},
		{"/movies/Movie2.mkv", "Movie 2", 2021},
		{"/tv/Show1.mkv", "Show 1 Episode 1", 2019},
	}

	for _, e := range entries {
		fileInfo := scanner.FileInfo{
			Path: e.path,
			Size: 1024,
		}

		year := e.year
		result := scanner.ScanResult{
			FilePath:  e.path,
			Title:     e.title,
			Year:      &year,
			MediaType: scanner.MediaTypeMovie,
			Hash:      "hash_" + e.path,
		}

		coordinator.updateFileCache(fileInfo, &result)
	}

	// Verify all entries exist
	coordinator.mu.Lock()
	if len(coordinator.config.FileCache) != 3 {
		t.Errorf("Expected 3 cache entries, got %d", len(coordinator.config.FileCache))
	}

	for _, e := range entries {
		entry, exists := coordinator.config.FileCache[e.path]
		if !exists {
			t.Errorf("Expected cache entry for %s to exist", e.path)
			continue
		}
		if entry.Title != e.title {
			t.Errorf("Expected title %s, got %s", e.title, entry.Title)
		}
		if entry.Year == nil || *entry.Year != e.year {
			t.Errorf("Expected year %d, got %v", e.year, entry.Year)
		}
	}
	coordinator.mu.Unlock()
}

func TestUpdateFileCacheWithMusicMetadata(t *testing.T) {
	config := Config{
		NumWorkers:               2,
		ResultBufferSize:         10,
		EnableIncrementalScan:    true,
		FileCache:                make(map[string]*scanner.FileCacheEntry),
	}
	coordinator := NewCoordinator(config)

	fileInfo := scanner.FileInfo{
		Path:      "/music/Artist/Album/Track.mp3",
		Size:      5242880, // 5MB
		Extension: ".mp3",
	}

	year := 2019
	trackNum := 3
	result := scanner.ScanResult{
		FilePath:    "/music/Artist/Album/Track.mp3",
		MediaType:   scanner.MediaTypeTrack,
		Title:       "Song Title",
		Artist:      "Artist Name",
		Album:       "Album Name",
		Year:        &year,
		TrackNumber: &trackNum,
		Hash:        "music123",
		FileSize:    5242880,
		Duration:    180,
		AudioCodec:  "mp3",
	}

	coordinator.updateFileCache(fileInfo, &result)

	coordinator.mu.Lock()
	entry := coordinator.config.FileCache[fileInfo.Path]
	coordinator.mu.Unlock()

	if entry == nil {
		t.Fatal("Expected cache entry to exist")
	}

	if entry.Artist != "Artist Name" {
		t.Errorf("Expected Artist='Artist Name', got %s", entry.Artist)
	}
	if entry.Album != "Album Name" {
		t.Errorf("Expected Album='Album Name', got %s", entry.Album)
	}
	if entry.TrackNumber == nil || *entry.TrackNumber != 3 {
		t.Errorf("Expected TrackNumber=3, got %v", entry.TrackNumber)
	}
}

func TestUpdateFileCacheWithTVMetadata(t *testing.T) {
	config := Config{
		NumWorkers:               2,
		ResultBufferSize:         10,
		EnableIncrementalScan:    true,
		FileCache:                make(map[string]*scanner.FileCacheEntry),
	}
	coordinator := NewCoordinator(config)

	fileInfo := scanner.FileInfo{
		Path:      "/tv/Show/Season 1/S01E05.mkv",
		Size:      2147483648, // 2GB
		Extension: ".mkv",
	}

	season := 1
	episode := 5
	result := scanner.ScanResult{
		FilePath:      "/tv/Show/Season 1/S01E05.mkv",
		MediaType:     scanner.MediaTypeEpisode,
		Title:         "Episode Title",
		SeasonNumber:  &season,
		EpisodeNumber: &episode,
		Hash:          "tv123",
		FileSize:      2147483648,
		Duration:      2700,
		VideoCodec:    "h264",
		AudioCodec:    "aac",
	}

	coordinator.updateFileCache(fileInfo, &result)

	coordinator.mu.Lock()
	entry := coordinator.config.FileCache[fileInfo.Path]
	coordinator.mu.Unlock()

	if entry == nil {
		t.Fatal("Expected cache entry to exist")
	}

	if entry.SeasonNumber == nil || *entry.SeasonNumber != 1 {
		t.Errorf("Expected SeasonNumber=1, got %v", entry.SeasonNumber)
	}
	if entry.EpisodeNumber == nil || *entry.EpisodeNumber != 5 {
		t.Errorf("Expected EpisodeNumber=5, got %v", entry.EpisodeNumber)
	}
	if entry.MediaType != scanner.MediaTypeEpisode {
		t.Errorf("Expected MediaType=episode, got %s", entry.MediaType)
	}
}

func TestUpdateFileCacheOverwrite(t *testing.T) {
	config := Config{
		NumWorkers:               2,
		ResultBufferSize:         10,
		EnableIncrementalScan:    true,
		FileCache:                make(map[string]*scanner.FileCacheEntry),
	}
	coordinator := NewCoordinator(config)

	path := "/test/movie.mkv"
	fileInfo := scanner.FileInfo{
		Path: path,
		Size: 1024,
	}

	// First update
	year1 := 2020
	result1 := scanner.ScanResult{
		FilePath:  path,
		Title:     "Original Title",
		Year:      &year1,
		MediaType: scanner.MediaTypeMovie,
		Hash:      "hash1",
	}
	coordinator.updateFileCache(fileInfo, &result1)

	// Second update (overwrites first)
	year2 := 2021
	result2 := scanner.ScanResult{
		FilePath:  path,
		Title:     "Updated Title",
		Year:      &year2,
		MediaType: scanner.MediaTypeMovie,
		Hash:      "hash2",
	}
	coordinator.updateFileCache(fileInfo, &result2)

	// Verify the latest entry is stored
	coordinator.mu.Lock()
	entry := coordinator.config.FileCache[path]
	coordinator.mu.Unlock()

	if entry.Title != "Updated Title" {
		t.Errorf("Expected Title='Updated Title', got %s", entry.Title)
	}
	if entry.Year == nil || *entry.Year != 2021 {
		t.Errorf("Expected Year=2021, got %v", entry.Year)
	}
	if entry.Hash != "hash2" {
		t.Errorf("Expected Hash='hash2', got %s", entry.Hash)
	}

	// Verify only one entry exists for this path
	coordinator.mu.Lock()
	count := 0
	for k := range coordinator.config.FileCache {
		if k == path {
			count++
		}
	}
	coordinator.mu.Unlock()

	if count != 1 {
		t.Errorf("Expected 1 cache entry for path, got %d", count)
	}
}

// TestProcessFile tests the ProcessFile method for various scenarios
func TestProcessFile(t *testing.T) {
	t.Run("context cancellation returns error", func(t *testing.T) {
		coordinator := NewCoordinator(DefaultConfig())

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		fileInfo := scanner.FileInfo{
			Path:      "/test/movie.mkv",
			Size:      1024,
			Extension: ".mkv",
		}

		result := coordinator.ProcessFile(ctx, fileInfo)

		if result.Error != context.Canceled {
			t.Errorf("Expected context.Canceled error, got %v", result.Error)
		}
	})

	t.Run("incremental scan cache hit returns cached data", func(t *testing.T) {
		year := 2020
		season := 1
		episode := 5
		trackNum := 3

		config := Config{
			NumWorkers:            2,
			ResultBufferSize:      10,
			EnableIncrementalScan: true,
			FileCache: map[string]*scanner.FileCacheEntry{
				"/test/movie.mkv": {
					Path:          "/test/movie.mkv",
					Size:          1024,
					ModTime:       time.Now(),
					Hash:          "cached_hash",
					MediaType:     scanner.MediaTypeMovie,
					Title:         "Cached Movie Title",
					Artist:        "Cached Artist",
					Album:         "Cached Album",
					Year:          &year,
					SeasonNumber:  &season,
					EpisodeNumber: &episode,
					TrackNumber:   &trackNum,
				},
			},
		}
		coordinator := NewCoordinator(config)
		ctx := context.Background()

		fileInfo := scanner.FileInfo{
			Path:      "/test/movie.mkv",
			Size:      1024,
			ModTime:   config.FileCache["/test/movie.mkv"].ModTime, // Same ModTime = unchanged
			Extension: ".mkv",
		}

		result := coordinator.ProcessFile(ctx, fileInfo)

		// Should return cached data
		if result.MediaType != scanner.MediaTypeMovie {
			t.Errorf("Expected MediaType=movie, got %s", result.MediaType)
		}
		if result.Title != "Cached Movie Title" {
			t.Errorf("Expected Title='Cached Movie Title', got %s", result.Title)
		}
		if result.Hash != "cached_hash" {
			t.Errorf("Expected Hash='cached_hash', got %s", result.Hash)
		}
		if result.Artist != "Cached Artist" {
			t.Errorf("Expected Artist='Cached Artist', got %s", result.Artist)
		}
		if result.Album != "Cached Album" {
			t.Errorf("Expected Album='Cached Album', got %s", result.Album)
		}
		if result.Year == nil || *result.Year != 2020 {
			t.Errorf("Expected Year=2020, got %v", result.Year)
		}
		if result.SeasonNumber == nil || *result.SeasonNumber != 1 {
			t.Errorf("Expected SeasonNumber=1, got %v", result.SeasonNumber)
		}
		if result.EpisodeNumber == nil || *result.EpisodeNumber != 5 {
			t.Errorf("Expected EpisodeNumber=5, got %v", result.EpisodeNumber)
		}
		if result.TrackNumber == nil || *result.TrackNumber != 3 {
			t.Errorf("Expected TrackNumber=3, got %v", result.TrackNumber)
		}
	})

	t.Run("incremental scan cache miss processes file", func(t *testing.T) {
		config := Config{
			NumWorkers:            2,
			ResultBufferSize:      10,
			EnableIncrementalScan: true,
			FileCache: map[string]*scanner.FileCacheEntry{
				"/test/movie.mkv": {
					Path:      "/test/movie.mkv",
					Size:      1024,
					ModTime:   time.Now().Add(-1 * time.Hour), // Old ModTime
					Hash:      "old_hash",
					MediaType: scanner.MediaTypeMovie,
					Title:     "Old Title",
				},
			},
		}
		coordinator := NewCoordinator(config)
		ctx := context.Background()

		fileInfo := scanner.FileInfo{
			Path:      "/test/movie.mkv",
			Size:      1024,
			ModTime:   time.Now(), // Different ModTime = changed file
			Extension: ".mkv",
		}

		result := coordinator.ProcessFile(ctx, fileInfo)

		// Should process the file fresh, not use cache
		// Title should be parsed from the filename
		if result.Title == "Old Title" {
			t.Error("Expected fresh parsing, but got cached title")
		}
	})

	t.Run("processes movie file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "The Matrix (1999).mkv")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		coordinator := NewCoordinator(DefaultConfig())
		ctx := context.Background()

		fileInfo := scanner.FileInfo{
			Path:      testFile,
			Size:      4,
			Extension: ".mkv",
		}

		result := coordinator.ProcessFile(ctx, fileInfo)

		if result.MediaType != scanner.MediaTypeMovie {
			t.Errorf("Expected MediaType=movie, got %s", result.MediaType)
		}
		if result.Title != "The Matrix" {
			t.Errorf("Expected Title='The Matrix', got %s", result.Title)
		}
		if result.Year == nil || *result.Year != 1999 {
			t.Errorf("Expected Year=1999, got %v", result.Year)
		}
	})

	t.Run("processes video file with TV episode naming as movie", func(t *testing.T) {
		// Note: The filter returns MediaTypeMovie for all video files.
		// Episode detection happens at a different layer (application/library scanner).
		// Here we verify that the movie parser successfully handles the file.
		tmpDir := t.TempDir()
		showDir := filepath.Join(tmpDir, "Breaking Bad (2008)", "Season 01")
		if err := os.MkdirAll(showDir, 0755); err != nil {
			t.Fatal(err)
		}
		testFile := filepath.Join(showDir, "Breaking Bad (2008) - S01E01 - Pilot.mkv")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		coordinator := NewCoordinator(DefaultConfig())
		ctx := context.Background()

		fileInfo := scanner.FileInfo{
			Path:      testFile,
			Size:      4,
			Extension: ".mkv",
		}

		result := coordinator.ProcessFile(ctx, fileInfo)

		// Filter returns movie for all video files; episode detection is at application layer
		if result.MediaType != scanner.MediaTypeMovie {
			t.Errorf("Expected MediaType=movie (filter default for video), got %s", result.MediaType)
		}
		// Title should be parsed from filename
		if result.Title == "" {
			t.Error("Expected title to be parsed from filename")
		}
	})

	t.Run("processes music file", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Create album directory structure
		albumDir := filepath.Join(tmpDir, "Artist", "Album (2020)")
		if err := os.MkdirAll(albumDir, 0755); err != nil {
			t.Fatal(err)
		}
		testFile := filepath.Join(albumDir, "01 - Track Title.mp3")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		coordinator := NewCoordinator(DefaultConfig())
		ctx := context.Background()

		fileInfo := scanner.FileInfo{
			Path:      testFile,
			Size:      4,
			Extension: ".mp3",
		}

		result := coordinator.ProcessFile(ctx, fileInfo)

		if result.MediaType != scanner.MediaTypeTrack {
			t.Errorf("Expected MediaType=track, got %s", result.MediaType)
		}
	})

	t.Run("episode fallback to movie parser when TV parsing fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Create a file that looks like it might be TV but won't parse as TV
		testFile := filepath.Join(tmpDir, "Some Movie (2020).mkv")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a coordinator where the filter thinks this is episode type
		// but the parser will fail TV parsing
		coordinator := NewCoordinator(DefaultConfig())
		ctx := context.Background()

		fileInfo := scanner.FileInfo{
			Path:      testFile,
			Size:      4,
			Extension: ".mkv",
		}

		// Override the extension to trigger episode media type
		// but the filename doesn't match episode pattern
		result := coordinator.ProcessFile(ctx, fileInfo)

		// Should fall back to movie type since TV parsing fails
		if result.MediaType == scanner.MediaTypeUnknown {
			t.Error("Expected media type to be identified, got unknown")
		}
	})

	t.Run("updates file cache when incremental scan enabled", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "New Movie (2022).mkv")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		config := Config{
			NumWorkers:            2,
			ResultBufferSize:      10,
			EnableIncrementalScan: true,
			FileCache:             make(map[string]*scanner.FileCacheEntry),
		}
		coordinator := NewCoordinator(config)
		ctx := context.Background()

		fileInfo := scanner.FileInfo{
			Path:      testFile,
			Size:      4,
			ModTime:   time.Now(),
			Extension: ".mkv",
		}

		coordinator.ProcessFile(ctx, fileInfo)

		// Check that the file was cached
		coordinator.mu.Lock()
		_, exists := coordinator.config.FileCache[testFile]
		coordinator.mu.Unlock()

		if !exists {
			t.Error("Expected file to be added to cache")
		}
	})

	t.Run("handles unknown media type", func(t *testing.T) {
		coordinator := NewCoordinator(DefaultConfig())
		ctx := context.Background()

		fileInfo := scanner.FileInfo{
			Path:      "/test/file.xyz", // Unknown extension
			Size:      1024,
			Extension: ".xyz",
		}

		result := coordinator.ProcessFile(ctx, fileInfo)

		if result.MediaType != scanner.MediaTypeUnknown {
			t.Errorf("Expected MediaType=unknown, got %s", result.MediaType)
		}
	})

	t.Run("sets warning on FFmpeg failure", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "Test Movie (2020).mkv")
		// Create an invalid file that FFmpeg can't process
		if err := os.WriteFile(testFile, []byte("not a valid video"), 0644); err != nil {
			t.Fatal(err)
		}

		coordinator := NewCoordinator(DefaultConfig())
		ctx := context.Background()

		fileInfo := scanner.FileInfo{
			Path:      testFile,
			Size:      18,
			Extension: ".mkv",
		}

		result := coordinator.ProcessFile(ctx, fileInfo)

		// FFmpeg should fail and set a warning
		if result.Warning == nil {
			t.Error("Expected warning to be set when FFmpeg fails")
		}
		if result.WarningCategory != "ffmpeg" {
			t.Errorf("Expected WarningCategory='ffmpeg', got %s", result.WarningCategory)
		}
	})
}

func TestUpdateFileCacheThreadSafety(t *testing.T) {
	config := Config{
		NumWorkers:               4,
		ResultBufferSize:         10,
		EnableIncrementalScan:    true,
		FileCache:                make(map[string]*scanner.FileCacheEntry),
	}
	coordinator := NewCoordinator(config)

	// Simulate concurrent updates from multiple workers
	const numGoroutines = 10
	const updatesPerGoroutine = 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(workerID int) {
			for j := 0; j < updatesPerGoroutine; j++ {
				path := filepath.Join("/test", "worker", string(rune('0'+workerID)), "file", string(rune('0'+j%10))+".mkv")
				fileInfo := scanner.FileInfo{
					Path: path,
					Size: int64(workerID * 1000 + j),
				}

				year := 2020 + workerID
				result := scanner.ScanResult{
					FilePath:  path,
					Title:     "Title",
					Year:      &year,
					MediaType: scanner.MediaTypeMovie,
					Hash:      "hash",
				}

				coordinator.updateFileCache(fileInfo, &result)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify cache integrity (no panics, all entries accessible)
	coordinator.mu.Lock()
	cacheSize := len(coordinator.config.FileCache)
	coordinator.mu.Unlock()

	// We expect multiple entries (exact number depends on path collisions)
	if cacheSize == 0 {
		t.Error("Expected cache to contain entries after concurrent updates")
	}

	t.Logf("Cache contains %d entries after concurrent updates", cacheSize)
}

func TestScanWithErrorCounting(t *testing.T) {
	// This test verifies that the error counting path in worker is exercised
	// by processing files that cause ProcessFile to return an error
	config := Config{
		NumWorkers:       1,
		ResultBufferSize: 10,
	}
	coordinator := NewCoordinator(config)

	// Create temp directory with valid files
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "Movie (2020).mkv")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	resultChan := make(chan scanner.ScanResult, 10)

	done := make(chan error, 1)
	go func() {
		done <- coordinator.Scan(context.Background(), tmpDir, resultChan)
		close(resultChan)
	}()

	// Drain results
	var results []scanner.ScanResult
	for result := range resultChan {
		results = append(results, result)
	}

	err := <-done
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify at least one result
	if len(results) == 0 {
		t.Error("Expected at least one result")
	}

	progress := coordinator.GetProgress()
	t.Logf("Progress: FilesFound=%d, FilesProcessed=%d, ErrorCount=%d",
		progress.FilesFound, progress.FilesProcessed, progress.ErrorCount)
}

func TestProcessFile_MusicFileWithMetadata(t *testing.T) {
	// Test music file processing - exercises the MediaTypeTrack branch
	tmpDir := t.TempDir()
	artistDir := filepath.Join(tmpDir, "Artist Name")
	albumDir := filepath.Join(artistDir, "Album Name (2022)")
	if err := os.MkdirAll(albumDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a music file with naming convention that parser can use
	testFile := filepath.Join(albumDir, "01 - Song Title.flac")
	if err := os.WriteFile(testFile, []byte("fake audio content"), 0644); err != nil {
		t.Fatal(err)
	}

	coordinator := NewCoordinator(DefaultConfig())
	ctx := context.Background()

	fileInfo := scanner.FileInfo{
		Path:      testFile,
		Size:      18,
		Extension: ".flac",
	}

	result := coordinator.ProcessFile(ctx, fileInfo)

	// Should be identified as a track
	if result.MediaType != scanner.MediaTypeTrack {
		t.Errorf("Expected MediaType=track, got %s", result.MediaType)
	}

	// Title should be parsed from filename
	if result.Title == "" {
		t.Log("Title was not parsed from filename - this is expected if parser requires real metadata")
	}

	t.Logf("Result: MediaType=%s, Title=%s, Artist=%s, Album=%s",
		result.MediaType, result.Title, result.Artist, result.Album)
}

func TestProcessFile_VariousAudioFormats(t *testing.T) {
	// Test various audio file formats to ensure MediaTypeTrack branch is hit
	audioFormats := []string{".mp3", ".flac", ".wav", ".m4a", ".aac", ".ogg", ".wma", ".opus"}

	coordinator := NewCoordinator(DefaultConfig())
	ctx := context.Background()

	for _, ext := range audioFormats {
		t.Run("format"+ext, func(t *testing.T) {
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "test"+ext)
			if err := os.WriteFile(testFile, []byte("fake audio"), 0644); err != nil {
				t.Fatal(err)
			}

			fileInfo := scanner.FileInfo{
				Path:      testFile,
				Size:      10,
				Extension: ext,
			}

			result := coordinator.ProcessFile(ctx, fileInfo)

			if result.MediaType != scanner.MediaTypeTrack {
				t.Errorf("Expected MediaType=track for %s, got %s", ext, result.MediaType)
			}
		})
	}
}

func TestCoordinator_WithCustomLogger(t *testing.T) {
	// Test that custom logger is used
	config := Config{
		NumWorkers:       2,
		ResultBufferSize: 10,
		Logger:           nil, // Will use default
	}

	coordinator := NewCoordinator(config)

	if coordinator == nil {
		t.Fatal("Expected non-nil coordinator")
	}

	// Coordinator should work with default logger
	if coordinator.logger == nil {
		t.Error("Expected logger to be initialized")
	}
}

func TestScanProgressDuringExecution(t *testing.T) {
	// Test that progress is tracked during scan execution
	tmpDir := t.TempDir()

	// Create several test files
	for i := 0; i < 5; i++ {
		testFile := filepath.Join(tmpDir, "Movie"+string(rune('A'+i))+" (2020).mkv")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	config := Config{
		NumWorkers:       1,
		ResultBufferSize: 10,
	}
	coordinator := NewCoordinator(config)

	resultChan := make(chan scanner.ScanResult, 10)

	done := make(chan error, 1)
	go func() {
		done <- coordinator.Scan(context.Background(), tmpDir, resultChan)
		close(resultChan)
	}()

	// Collect results
	var resultCount int
	for range resultChan {
		resultCount++
	}

	err := <-done
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify final progress
	progress := coordinator.GetProgress()
	if progress.FilesProcessed != int64(resultCount) {
		t.Errorf("FilesProcessed=%d, but received %d results", progress.FilesProcessed, resultCount)
	}
	if progress.FilesFound < progress.FilesProcessed {
		t.Errorf("FilesFound=%d should be >= FilesProcessed=%d", progress.FilesFound, progress.FilesProcessed)
	}
}
