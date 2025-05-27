package scanner

import (
	"os"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestFileCountingNoDuplicates(t *testing.T) {
	// Create temporary database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Auto-migrate tables
	err = db.AutoMigrate(&database.MediaFile{}, &database.MusicMetadata{}, &database.MediaLibrary{}, &database.ScanJob{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create test library
	library := &database.MediaLibrary{
		Path: "/test/music",
		Type: "music",
	}
	if err := db.Create(library).Error; err != nil {
		t.Fatalf("Failed to create test library: %v", err)
	}

	// Create test scan job
	scanJob := &database.ScanJob{
		LibraryID:   library.ID,
		Status:      "running",
		FilesFound:  2,
		Progress:    0,
	}
	if err := db.Create(scanJob).Error; err != nil {
		t.Fatalf("Failed to create test scan job: %v", err)
	}

	// Create test scanner (pass nil for eventBus since we're just testing file counting)
	scanner := NewParallelFileScanner(db, scanJob.ID, nil)

	// Create test files in cache (simulating existing files)
	testFile1 := &database.MediaFile{
		Path:      "/test/music/song1.mp3",
		Size:      1000,
		Hash:      "hash1",
		LibraryID: library.ID,
		LastSeen:  time.Now(),
	}
	testFile2 := &database.MediaFile{
		Path:      "/test/music/song2.mp3",
		Size:      2000,
		Hash:      "hash2",
		LibraryID: library.ID,
		LastSeen:  time.Now(),
	}

	// Add files to cache
	scanner.fileCache.cache[testFile1.Path] = testFile1
	scanner.fileCache.cache[testFile2.Path] = testFile2

	// Create mock file info
	mockFileInfo1 := &mockFileInfo{name: "song1.mp3", size: 1000}
	mockFileInfo2 := &mockFileInfo{name: "song2.mp3", size: 2000}

	// Process files (both should be found in cache)
	work1 := scanWork{
		path:      testFile1.Path,
		info:      mockFileInfo1,
		libraryID: library.ID,
	}
	work2 := scanWork{
		path:      testFile2.Path,
		info:      mockFileInfo2,
		libraryID: library.ID,
	}

	// Process both files
	result1 := scanner.processFile(work1)
	result2 := scanner.processFile(work2)

	// Verify no errors
	if result1.error != nil {
		t.Errorf("Unexpected error processing file 1: %v", result1.error)
	}
	if result2.error != nil {
		t.Errorf("Unexpected error processing file 2: %v", result2.error)
	}

	// Verify file counts - should be exactly 2, not 4
	filesProcessed := scanner.filesProcessed.Load()
	if filesProcessed != 2 {
		t.Errorf("Expected 2 files processed, got %d", filesProcessed)
	}

	// Verify byte counts - should be exactly 3000, not 6000
	bytesProcessed := scanner.bytesProcessed.Load()
	expectedBytes := int64(3000)
	if bytesProcessed != expectedBytes {
		t.Errorf("Expected %d bytes processed, got %d", expectedBytes, bytesProcessed)
	}
}

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	name string
	size int64
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return time.Now() }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() interface{}   { return nil } 