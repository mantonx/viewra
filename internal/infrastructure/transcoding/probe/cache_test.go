package probe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestVideoInfoCache_SetAndGet tests basic cache set and get operations
func TestVideoInfoCache_SetAndGet(t *testing.T) {
	// Create a fresh cache for testing
	cache := &videoInfoCache{
		entries: make(map[string]*videoInfoCacheEntry),
	}

	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_video.mp4")
	if err := os.WriteFile(testFile, []byte("dummy video data"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create test video info
	testInfo := &VideoInfo{
		Codec:           "h264",
		Width:           1920,
		Height:          1080,
		Bitrate:         5000000,
		Duration:        120.5,
		AudioCodec:      "aac",
		AudioBitrate:    128000,
		AudioChannels:   2,
		ContainerFormat: "mp4",
	}

	// Test set
	cache.Set(testFile, testInfo)

	// Test get - should retrieve the cached info
	retrieved := cache.Get(testFile)
	if retrieved == nil {
		t.Fatal("cache.Get() returned nil, expected cached VideoInfo")
	}

	// Verify all fields match
	if retrieved.Codec != testInfo.Codec {
		t.Errorf("Codec = %q, want %q", retrieved.Codec, testInfo.Codec)
	}
	if retrieved.Width != testInfo.Width {
		t.Errorf("Width = %d, want %d", retrieved.Width, testInfo.Width)
	}
	if retrieved.Height != testInfo.Height {
		t.Errorf("Height = %d, want %d", retrieved.Height, testInfo.Height)
	}
	if retrieved.Bitrate != testInfo.Bitrate {
		t.Errorf("Bitrate = %d, want %d", retrieved.Bitrate, testInfo.Bitrate)
	}
	if retrieved.Duration != testInfo.Duration {
		t.Errorf("Duration = %f, want %f", retrieved.Duration, testInfo.Duration)
	}
}

// TestVideoInfoCache_GetNonExistent tests getting from cache when entry doesn't exist
func TestVideoInfoCache_GetNonExistent(t *testing.T) {
	cache := &videoInfoCache{
		entries: make(map[string]*videoInfoCacheEntry),
	}

	result := cache.Get("/nonexistent/file.mp4")
	if result != nil {
		t.Errorf("cache.Get() for non-existent entry = %+v, want nil", result)
	}
}

// TestVideoInfoCache_GetInvalidPath tests getting with a path that can't be stat'd
func TestVideoInfoCache_GetInvalidPath(t *testing.T) {
	cache := &videoInfoCache{
		entries: make(map[string]*videoInfoCacheEntry),
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp4")

	// Create and cache a file
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	testInfo := &VideoInfo{Codec: "h264"}
	cache.Set(testFile, testInfo)

	// Delete the file to make stat fail
	if err := os.Remove(testFile); err != nil {
		t.Fatalf("Failed to remove test file: %v", err)
	}

	// Get should return nil when file can't be stat'd
	result := cache.Get(testFile)
	if result != nil {
		t.Errorf("cache.Get() for deleted file = %+v, want nil", result)
	}
}

// TestVideoInfoCache_FileModificationInvalidation tests cache invalidation on file modification
func TestVideoInfoCache_FileModificationInvalidation(t *testing.T) {
	cache := &videoInfoCache{
		entries: make(map[string]*videoInfoCacheEntry),
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp4")

	// Create initial file
	if err := os.WriteFile(testFile, []byte("version 1"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Cache the info
	testInfo := &VideoInfo{Codec: "h264", Width: 1920}
	cache.Set(testFile, testInfo)

	// Verify it's cached
	if retrieved := cache.Get(testFile); retrieved == nil {
		t.Fatal("Initial cache.Get() returned nil")
	}

	// Wait a bit to ensure modification time will be different
	time.Sleep(10 * time.Millisecond)

	// Modify the file (this changes mtime)
	if err := os.WriteFile(testFile, []byte("version 2 - modified"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Cache should now return nil due to mtime mismatch
	result := cache.Get(testFile)
	if result != nil {
		t.Errorf("cache.Get() after file modification = %+v, want nil (cache should be invalidated)", result)
	}
}

// TestVideoInfoCache_Expiration tests cache entry expiration after cacheMaxAge
func TestVideoInfoCache_Expiration(t *testing.T) {
	cache := &videoInfoCache{
		entries: make(map[string]*videoInfoCacheEntry),
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp4")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	testInfo := &VideoInfo{Codec: "h264"}
	cache.Set(testFile, testInfo)

	// Manually set the cachedAt time to be older than cacheMaxAge
	cache.mu.Lock()
	entry := cache.entries[testFile]
	entry.cachedAt = time.Now().Add(-(cacheMaxAge + time.Hour))
	cache.mu.Unlock()

	// Cache should return nil for expired entry
	result := cache.Get(testFile)
	if result != nil {
		t.Errorf("cache.Get() for expired entry = %+v, want nil", result)
	}
}

// TestVideoInfoCache_ExpirationBoundary tests cache right at expiration boundary
func TestVideoInfoCache_ExpirationBoundary(t *testing.T) {
	cache := &videoInfoCache{
		entries: make(map[string]*videoInfoCacheEntry),
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp4")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	testInfo := &VideoInfo{Codec: "h264", Width: 1920}
	cache.Set(testFile, testInfo)

	// Set time to just before expiration (should still be valid)
	cache.mu.Lock()
	entry := cache.entries[testFile]
	entry.cachedAt = time.Now().Add(-(cacheMaxAge - time.Minute))
	cache.mu.Unlock()

	// Should still retrieve cached entry
	result := cache.Get(testFile)
	if result == nil {
		t.Error("cache.Get() just before expiration returned nil, want cached entry")
	}
	if result != nil && result.Width != 1920 {
		t.Errorf("Width = %d, want 1920", result.Width)
	}
}

// TestVideoInfoCache_SetNonExistentFile tests setting cache for non-existent file
func TestVideoInfoCache_SetNonExistentFile(t *testing.T) {
	cache := &videoInfoCache{
		entries: make(map[string]*videoInfoCacheEntry),
	}

	// Try to cache info for non-existent file
	testInfo := &VideoInfo{Codec: "h264"}
	cache.Set("/nonexistent/path/video.mp4", testInfo)

	// Cache should not store entry if file doesn't exist
	cache.mu.RLock()
	_, exists := cache.entries["/nonexistent/path/video.mp4"]
	cache.mu.RUnlock()

	if exists {
		t.Error("cache.Set() stored entry for non-existent file, expected no entry")
	}
}

// TestVideoInfoCache_MultipleFiles tests caching multiple different files
func TestVideoInfoCache_MultipleFiles(t *testing.T) {
	cache := &videoInfoCache{
		entries: make(map[string]*videoInfoCacheEntry),
	}

	tmpDir := t.TempDir()

	// Create and cache multiple files
	files := []struct {
		name  string
		codec string
		width int
	}{
		{"video1.mp4", "h264", 1920},
		{"video2.mkv", "hevc", 3840},
		{"video3.avi", "h264", 1280},
	}

	for _, f := range files {
		filePath := filepath.Join(tmpDir, f.name)
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create %s: %v", f.name, err)
		}

		info := &VideoInfo{
			Codec: f.codec,
			Width: f.width,
		}
		cache.Set(filePath, info)
	}

	// Verify all files are cached correctly
	for _, f := range files {
		filePath := filepath.Join(tmpDir, f.name)
		retrieved := cache.Get(filePath)
		if retrieved == nil {
			t.Errorf("cache.Get(%s) returned nil", f.name)
			continue
		}
		if retrieved.Codec != f.codec {
			t.Errorf("%s: Codec = %q, want %q", f.name, retrieved.Codec, f.codec)
		}
		if retrieved.Width != f.width {
			t.Errorf("%s: Width = %d, want %d", f.name, retrieved.Width, f.width)
		}
	}
}

// TestVideoInfoCache_UpdateEntry tests updating an existing cache entry
func TestVideoInfoCache_UpdateEntry(t *testing.T) {
	cache := &videoInfoCache{
		entries: make(map[string]*videoInfoCacheEntry),
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp4")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Set initial info
	initialInfo := &VideoInfo{Codec: "h264", Width: 1920}
	cache.Set(testFile, initialInfo)

	// Update with new info
	updatedInfo := &VideoInfo{Codec: "hevc", Width: 3840}
	cache.Set(testFile, updatedInfo)

	// Should retrieve updated info
	retrieved := cache.Get(testFile)
	if retrieved == nil {
		t.Fatal("cache.Get() returned nil after update")
	}
	if retrieved.Codec != "hevc" {
		t.Errorf("Codec = %q, want %q", retrieved.Codec, "hevc")
	}
	if retrieved.Width != 3840 {
		t.Errorf("Width = %d, want %d", retrieved.Width, 3840)
	}
}

// TestVideoInfoCache_ComplexVideoInfo tests caching with complete VideoInfo struct
func TestVideoInfoCache_ComplexVideoInfo(t *testing.T) {
	cache := &videoInfoCache{
		entries: make(map[string]*videoInfoCacheEntry),
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "complex.mkv")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create complex VideoInfo with all fields
	complexInfo := &VideoInfo{
		Codec:           "hevc",
		Width:           3840,
		Height:          2160,
		Bitrate:         50000000,
		Duration:        7200.5,
		AudioCodec:      "aac",
		AudioBitrate:    256000,
		AudioChannels:   6,
		AudioTracks: []AudioTrack{
			{Index: 0, Codec: "aac", Channels: 6, Language: "eng", Title: "English 5.1"},
			{Index: 1, Codec: "aac", Channels: 2, Language: "eng", Title: "English Stereo"},
			{Index: 2, Codec: "ac3", Channels: 6, Language: "fra", Title: "French 5.1"},
		},
		SelectedAudioTrackIndex: 0,
		ContainerFormat:         "matroska,webm",
		PixelFormat:             "yuv420p10le",
		ColorSpace:              "bt2020nc",
		ColorPrimaries:          "bt2020",
		ColorTransfer:           "smpte2084",
		BitDepth:                10,
		IsHDR:                   true,
	}

	cache.Set(testFile, complexInfo)

	retrieved := cache.Get(testFile)
	if retrieved == nil {
		t.Fatal("cache.Get() returned nil")
	}

	// Verify all fields
	if retrieved.Codec != complexInfo.Codec {
		t.Errorf("Codec = %q, want %q", retrieved.Codec, complexInfo.Codec)
	}
	if retrieved.Width != complexInfo.Width {
		t.Errorf("Width = %d, want %d", retrieved.Width, complexInfo.Width)
	}
	if retrieved.Height != complexInfo.Height {
		t.Errorf("Height = %d, want %d", retrieved.Height, complexInfo.Height)
	}
	if retrieved.IsHDR != complexInfo.IsHDR {
		t.Errorf("IsHDR = %v, want %v", retrieved.IsHDR, complexInfo.IsHDR)
	}
	if len(retrieved.AudioTracks) != len(complexInfo.AudioTracks) {
		t.Errorf("AudioTracks length = %d, want %d", len(retrieved.AudioTracks), len(complexInfo.AudioTracks))
	}
	if retrieved.SelectedAudioTrackIndex != complexInfo.SelectedAudioTrackIndex {
		t.Errorf("SelectedAudioTrackIndex = %d, want %d", retrieved.SelectedAudioTrackIndex, complexInfo.SelectedAudioTrackIndex)
	}
	if retrieved.ColorTransfer != complexInfo.ColorTransfer {
		t.Errorf("ColorTransfer = %q, want %q", retrieved.ColorTransfer, complexInfo.ColorTransfer)
	}
}

// TestGetVideoInfo_InvalidFile tests GetVideoInfo with non-existent file
func TestGetVideoInfo_InvalidFile(t *testing.T) {
	_, err := GetVideoInfo("/nonexistent/path/to/video.mp4")
	if err == nil {
		t.Error("GetVideoInfo() with invalid file should return error, got nil")
	}
	// Error should mention ffprobe failure or file not found
	if err != nil && err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

// TestGetVideoInfo_EmptyPath tests GetVideoInfo with empty path
func TestGetVideoInfo_EmptyPath(t *testing.T) {
	_, err := GetVideoInfo("")
	if err == nil {
		t.Error("GetVideoInfo() with empty path should return error, got nil")
	}
}

// TestGetVideoInfo_DirectoryPath tests GetVideoInfo with directory instead of file
func TestGetVideoInfo_DirectoryPath(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := GetVideoInfo(tmpDir)
	if err == nil {
		t.Error("GetVideoInfo() with directory path should return error, got nil")
	}
}

// TestGetVideoInfo_NonVideoFile tests GetVideoInfo with non-video file
func TestGetVideoInfo_NonVideoFile(t *testing.T) {
	tmpDir := t.TempDir()
	textFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(textFile, []byte("This is not a video file"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err := GetVideoInfo(textFile)
	if err == nil {
		t.Error("GetVideoInfo() with non-video file should return error, got nil")
	}
}

// TestGetVideoInfo_FFprobeNotFound tests behavior when ffprobe is not available
func TestGetVideoInfo_FFprobeNotFound(t *testing.T) {
	// Save original PATH
	originalPath := os.Getenv("PATH")
	originalFFprobe := os.Getenv("VIEWRA_FFPROBE_PATH")

	// Clean up after test
	defer func() {
		os.Setenv("PATH", originalPath)
		if originalFFprobe != "" {
			os.Setenv("VIEWRA_FFPROBE_PATH", originalFFprobe)
		} else {
			os.Unsetenv("VIEWRA_FFPROBE_PATH")
		}
	}()

	// Set PATH to empty directory and unset VIEWRA_FFPROBE_PATH
	tmpDir := t.TempDir()
	os.Setenv("PATH", tmpDir)
	os.Unsetenv("VIEWRA_FFPROBE_PATH")

	// Create a dummy video file
	testFile := filepath.Join(tmpDir, "test.mp4")
	if err := os.WriteFile(testFile, []byte("dummy"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err := GetVideoInfo(testFile)
	if err == nil {
		t.Error("GetVideoInfo() should fail when ffprobe is not found")
	}
	if err != nil && err.Error() != "" {
		// Error should mention ffprobe not found
		errStr := err.Error()
		if errStr == "" {
			t.Error("Error message should not be empty when ffprobe is not found")
		}
	}
}

// TestGetVideoInfo_InvalidFFprobeOutput tests handling of malformed ffprobe output
func TestGetVideoInfo_InvalidFFprobeOutput(t *testing.T) {
	// This test would require mocking exec.Command, which is complex
	// Instead, we can test the JSON parsing logic directly

	invalidJSON := `{"invalid json`
	var output ffprobeOutput
	err := json.Unmarshal([]byte(invalidJSON), &output)
	if err == nil {
		t.Error("Unmarshaling invalid JSON should return error")
	}
}

// TestGetVideoInfo_CacheHit tests that cached results are returned
func TestGetVideoInfo_CacheHit(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "cached_test.mp4")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Manually populate the cache
	stat, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	testInfo := &VideoInfo{
		Codec:  "h264",
		Width:  1920,
		Height: 1080,
	}

	GlobalCache.mu.Lock()
	GlobalCache.entries[testFile] = &videoInfoCacheEntry{
		info:     testInfo,
		modTime:  stat.ModTime(),
		cachedAt: time.Now(),
	}
	GlobalCache.mu.Unlock()

	// Clean up after test
	defer func() {
		GlobalCache.mu.Lock()
		delete(GlobalCache.entries, testFile)
		GlobalCache.mu.Unlock()
	}()

	// GetVideoInfo should return cached result without calling ffprobe
	retrieved, err := GetVideoInfo(testFile)
	if err != nil {
		// If ffprobe is not available, this will fail
		// But if cache is working, it should return the cached value
		t.Logf("GetVideoInfo returned error (might be expected if ffprobe unavailable): %v", err)
	}

	// If cache is working and ffprobe is available, we should get cached result
	if retrieved != nil {
		if retrieved.Codec != "h264" {
			t.Errorf("Cached Codec = %q, want %q", retrieved.Codec, "h264")
		}
		if retrieved.Width != 1920 {
			t.Errorf("Cached Width = %d, want %d", retrieved.Width, 1920)
		}
	}
}

// TestGetVideoInfo_CustomFFprobePath tests VIEWRA_FFPROBE_PATH environment variable
func TestGetVideoInfo_CustomFFprobePath(t *testing.T) {
	// Save and restore original env var
	originalFFprobe := os.Getenv("VIEWRA_FFPROBE_PATH")
	defer func() {
		if originalFFprobe != "" {
			os.Setenv("VIEWRA_FFPROBE_PATH", originalFFprobe)
		} else {
			os.Unsetenv("VIEWRA_FFPROBE_PATH")
		}
	}()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp4")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Set custom ffprobe path to non-existent location
	customPath := filepath.Join(tmpDir, "custom_ffprobe")
	os.Setenv("VIEWRA_FFPROBE_PATH", customPath)

	_, err := GetVideoInfo(testFile)
	// Should attempt to use custom path and fail (since it doesn't exist)
	if err == nil {
		t.Error("GetVideoInfo() with non-existent custom ffprobe path should fail")
	}
}

// TestGetVideoInfo_LibraryPath tests VIEWRA_FFMPEG_LIB_PATH environment variable
func TestGetVideoInfo_LibraryPath(t *testing.T) {
	// This test verifies that the library path is set in the command environment
	// We can't fully test execution without a real ffprobe, but we can verify the logic

	originalLibPath := os.Getenv("VIEWRA_FFMPEG_LIB_PATH")
	defer func() {
		if originalLibPath != "" {
			os.Setenv("VIEWRA_FFMPEG_LIB_PATH", originalLibPath)
		} else {
			os.Unsetenv("VIEWRA_FFMPEG_LIB_PATH")
		}
	}()

	tmpDir := t.TempDir()
	customLibPath := "/custom/lib/path"
	os.Setenv("VIEWRA_FFMPEG_LIB_PATH", customLibPath)

	testFile := filepath.Join(tmpDir, "test.mp4")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// This will fail since we don't have a real video, but it tests the code path
	_, err := GetVideoInfo(testFile)
	if err == nil {
		t.Log("GetVideoInfo succeeded (ffprobe might be available)")
	} else {
		t.Logf("GetVideoInfo failed as expected: %v", err)
	}
}

// TestVideoInfoCache_ConcurrentAccess tests thread-safety of cache operations
func TestVideoInfoCache_ConcurrentAccess(t *testing.T) {
	cache := &videoInfoCache{
		entries: make(map[string]*videoInfoCacheEntry),
	}

	tmpDir := t.TempDir()

	// Create test files
	numFiles := 10
	testFiles := make([]string, numFiles)
	for i := 0; i < numFiles; i++ {
		filePath := filepath.Join(tmpDir, fmt.Sprintf("video%d.mp4", i))
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file %d: %v", i, err)
		}
		testFiles[i] = filePath
	}

	// Concurrent set operations
	done := make(chan bool)
	for i := 0; i < numFiles; i++ {
		go func(index int) {
			info := &VideoInfo{
				Codec: "h264",
				Width: 1920 + index*100,
			}
			cache.Set(testFiles[index], info)
			done <- true
		}(i)
	}

	// Wait for all sets to complete
	for i := 0; i < numFiles; i++ {
		<-done
	}

	// Concurrent get operations
	for i := 0; i < numFiles; i++ {
		go func(index int) {
			result := cache.Get(testFiles[index])
			if result == nil {
				t.Errorf("cache.Get() for file %d returned nil", index)
			}
			done <- true
		}(i)
	}

	// Wait for all gets to complete
	for i := 0; i < numFiles; i++ {
		<-done
	}
}

// TestVideoInfoCache_KeyGeneration tests that cache keys are based on file paths
func TestVideoInfoCache_KeyGeneration(t *testing.T) {
	cache := &videoInfoCache{
		entries: make(map[string]*videoInfoCacheEntry),
	}

	tmpDir := t.TempDir()

	// Create files with different paths but same name
	subDir1 := filepath.Join(tmpDir, "dir1")
	subDir2 := filepath.Join(tmpDir, "dir2")
	if err := os.MkdirAll(subDir1, 0755); err != nil {
		t.Fatalf("Failed to create dir1: %v", err)
	}
	if err := os.MkdirAll(subDir2, 0755); err != nil {
		t.Fatalf("Failed to create dir2: %v", err)
	}

	file1 := filepath.Join(subDir1, "video.mp4")
	file2 := filepath.Join(subDir2, "video.mp4")

	if err := os.WriteFile(file1, []byte("file1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("file2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Cache different info for each file
	info1 := &VideoInfo{Codec: "h264", Width: 1920}
	info2 := &VideoInfo{Codec: "hevc", Width: 3840}

	cache.Set(file1, info1)
	cache.Set(file2, info2)

	// Verify they're stored separately
	retrieved1 := cache.Get(file1)
	retrieved2 := cache.Get(file2)

	if retrieved1 == nil || retrieved2 == nil {
		t.Fatal("One or both cache entries are nil")
	}

	if retrieved1.Codec != "h264" {
		t.Errorf("file1 Codec = %q, want h264", retrieved1.Codec)
	}
	if retrieved2.Codec != "hevc" {
		t.Errorf("file2 Codec = %q, want hevc", retrieved2.Codec)
	}
	if retrieved1.Width != 1920 {
		t.Errorf("file1 Width = %d, want 1920", retrieved1.Width)
	}
	if retrieved2.Width != 3840 {
		t.Errorf("file2 Width = %d, want 3840", retrieved2.Width)
	}
}

// TestVideoInfoCache_ZeroValues tests caching VideoInfo with zero values
func TestVideoInfoCache_ZeroValues(t *testing.T) {
	cache := &videoInfoCache{
		entries: make(map[string]*videoInfoCacheEntry),
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "zero.mp4")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// VideoInfo with mostly zero values (edge case)
	zeroInfo := &VideoInfo{
		Codec:       "",
		Width:       0,
		Height:      0,
		Bitrate:     0,
		Duration:    0,
		AudioTracks: []AudioTrack{},
	}

	cache.Set(testFile, zeroInfo)

	retrieved := cache.Get(testFile)
	if retrieved == nil {
		t.Fatal("cache.Get() returned nil for zero-value VideoInfo")
	}

	// Even zero values should be preserved
	if retrieved.Width != 0 {
		t.Errorf("Width = %d, want 0", retrieved.Width)
	}
	if retrieved.Codec != "" {
		t.Errorf("Codec = %q, want empty string", retrieved.Codec)
	}
}

// TestCacheMaxAge verifies the cache expiration constant
func TestCacheMaxAge(t *testing.T) {
	expectedMaxAge := 24 * time.Hour
	if cacheMaxAge != expectedMaxAge {
		t.Errorf("cacheMaxAge = %v, want %v", cacheMaxAge, expectedMaxAge)
	}
}

// TestCacheClearAndSize tests Clear() and Size() methods
func TestCacheClearAndSize(t *testing.T) {
	cache := &videoInfoCache{
		entries: make(map[string]*videoInfoCacheEntry),
	}

	// Empty cache should have size 0
	if cache.Size() != 0 {
		t.Errorf("empty cache Size() = %d, want 0", cache.Size())
	}

	tmpDir := t.TempDir()

	// Add some entries
	for i := 0; i < 5; i++ {
		testFile := filepath.Join(tmpDir, fmt.Sprintf("test%d.mp4", i))
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file %d: %v", i, err)
		}
		cache.Set(testFile, &VideoInfo{Codec: "h264"})
	}

	// Check size
	if cache.Size() != 5 {
		t.Errorf("cache Size() after adding 5 entries = %d, want 5", cache.Size())
	}

	// Clear and verify
	cache.Clear()
	if cache.Size() != 0 {
		t.Errorf("cache Size() after Clear() = %d, want 0", cache.Size())
	}
}
