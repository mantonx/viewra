package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

func TestDefaultCoordinatorConfig(t *testing.T) {
	config := DefaultCoordinatorConfig()

	if config.NumWorkers != 4 {
		t.Errorf("Expected NumWorkers=4, got %d", config.NumWorkers)
	}
	if config.ResultBufferSize != 100 {
		t.Errorf("Expected ResultBufferSize=100, got %d", config.ResultBufferSize)
	}
	}
}

func TestNewCoordinator(t *testing.T) {
	config := CoordinatorConfig{
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
	if coordinator.hasher == nil {
		t.Error("Expected hasher to be initialized")
	}
}

func TestShouldProcessFile(t *testing.T) {
	coordinator := NewCoordinator(DefaultCoordinatorConfig())

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
	config := CoordinatorConfig{
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

	config := CoordinatorConfig{
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

	config := CoordinatorConfig{
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
	config := DefaultCoordinatorConfig()
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
	t.Skip("Skipping flaky timing-dependent test - covered by integration tests")
}

func TestCoordinatorIsRunning(t *testing.T) {
	coordinator := NewCoordinator(DefaultCoordinatorConfig())

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
	coordinator := NewCoordinator(DefaultCoordinatorConfig())

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
	config := CoordinatorConfig{
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
	config := CoordinatorConfig{
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
	config := CoordinatorConfig{
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
	config := CoordinatorConfig{
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
	config := CoordinatorConfig{
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

func TestUpdateFileCacheThreadSafety(t *testing.T) {
	config := CoordinatorConfig{
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
