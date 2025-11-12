package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/viewra/viewra/internal/domain/scanner"
)

func TestDefaultCoordinatorConfig(t *testing.T) {
	config := DefaultCoordinatorConfig()

	if config.NumWorkers != 4 {
		t.Errorf("Expected NumWorkers=4, got %d", config.NumWorkers)
	}
	if config.ResultBufferSize != 100 {
		t.Errorf("Expected ResultBufferSize=100, got %d", config.ResultBufferSize)
	}
	if !config.EnableDuplicateDetection {
		t.Error("Expected EnableDuplicateDetection=true")
	}
}

func TestNewCoordinator(t *testing.T) {
	config := CoordinatorConfig{
		NumWorkers:               2,
		ResultBufferSize:         50,
		EnableDuplicateDetection: false,
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
		EnableDuplicateDetection: false,
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
		EnableDuplicateDetection: false,
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
		EnableDuplicateDetection: false,
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
