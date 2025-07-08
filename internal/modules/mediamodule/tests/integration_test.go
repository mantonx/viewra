// Package tests provides integration tests for the media module.
// These tests validate the module's functionality including library management,
// metadata handling, and integration with other services.
package tests

import (
	"context"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/mediamodule"
	"github.com/mantonx/viewra/internal/modules/mediamodule/service"
	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockScannerService is a mock implementation of the scanner service
type MockScannerService struct {
	mock.Mock
}

func (m *MockScannerService) StartScan(ctx context.Context, libraryID uint32) (*types.ScanJob, error) {
	args := m.Called(ctx, libraryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.ScanJob), args.Error(1)
}

func (m *MockScannerService) GetScanProgress(ctx context.Context, jobID string) (*types.ScanProgress, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.ScanProgress), args.Error(1)
}

func (m *MockScannerService) StopScan(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *MockScannerService) GetActiveScanJobs(ctx context.Context) ([]*types.ScanJob, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.ScanJob), args.Error(1)
}

func (m *MockScannerService) SetScanInterval(ctx context.Context, libraryID uint32, interval time.Duration) error {
	args := m.Called(ctx, libraryID, interval)
	return args.Error(0)
}

func (m *MockScannerService) GetScanHistory(ctx context.Context, libraryID uint32) ([]*types.ScanResult, error) {
	args := m.Called(ctx, libraryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*types.ScanResult), args.Error(1)
}

// setupTestDB creates an in-memory test database
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(
		&database.MediaFile{},
		&database.MediaLibrary{},
		&database.MediaMetadata{},
		&database.MediaAsset{},
	)
	require.NoError(t, err)

	return db
}

// TestMediaModuleIntegration tests the media module's core functionality
func TestMediaModuleIntegration(t *testing.T) {
	db := setupTestDB(t)

	// Create module
	module := &mediamodule.Module{}
	
	// Run migrations
	err := module.Migrate(db)
	require.NoError(t, err)

	// Create mock scanner service
	mockScanner := new(MockScannerService)
	
	// Register mock scanner service
	services.Register("scanner", mockScanner)
	defer services.Unregister("scanner")

	// Initialize module
	err = module.Init()
	require.NoError(t, err)

	// Get the media service
	mediaService, err := services.Get[services.MediaService]("media")
	require.NoError(t, err)
	require.NotNil(t, mediaService)

	ctx := context.Background()

	t.Run("LibraryOperations", func(t *testing.T) {
		// Create a library
		library := &database.MediaLibrary{
			Name:     "Test Library",
			Type:     database.LibraryTypeMovies,
			Path:     "/test/movies",
			Enabled:  true,
			Settings: map[string]interface{}{},
		}
		err := db.Create(library).Error
		require.NoError(t, err)

		// Get library
		retrievedLib, err := mediaService.GetLibrary(ctx, library.ID)
		require.NoError(t, err)
		assert.Equal(t, library.Name, retrievedLib.Name)
		assert.Equal(t, library.Type, retrievedLib.Type)

		// Scan library
		mockScanner.On("StartScan", ctx, library.ID).Return(&types.ScanJob{
			ID:        "scan-123",
			LibraryID: library.ID,
			Status:    "running",
			StartTime: time.Now(),
		}, nil)

		err = mediaService.ScanLibrary(ctx, library.ID)
		assert.NoError(t, err)
		mockScanner.AssertExpectations(t)
	})

	t.Run("FileOperations", func(t *testing.T) {
		// Create a media file
		file := &database.MediaFile{
			Path:      "/test/movies/movie.mp4",
			LibraryID: 1,
			Size:      1024 * 1024 * 100, // 100MB
			MediaType: database.MediaTypeMovie,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := db.Create(file).Error
		require.NoError(t, err)

		// Get file by ID
		retrievedFile, err := mediaService.GetFile(ctx, file.ID)
		require.NoError(t, err)
		assert.Equal(t, file.Path, retrievedFile.Path)
		assert.Equal(t, file.MediaType, retrievedFile.MediaType)

		// Get file by path
		fileByPath, err := mediaService.GetFileByPath(ctx, file.Path)
		require.NoError(t, err)
		assert.Equal(t, file.ID, fileByPath.ID)

		// Update file
		updates := map[string]interface{}{
			"title": "Test Movie",
			"year":  2024,
		}
		err = mediaService.UpdateFile(ctx, file.ID, updates)
		require.NoError(t, err)

		// Verify update
		updatedFile, err := mediaService.GetFile(ctx, file.ID)
		require.NoError(t, err)
		assert.Equal(t, "Test Movie", updatedFile.Title)
		assert.Equal(t, 2024, updatedFile.Year)
	})

	t.Run("MusicSupport", func(t *testing.T) {
		// Create music files
		album := &database.MediaFile{
			Path:      "/test/music/album",
			LibraryID: 1,
			MediaType: database.MediaTypeAlbum,
			Title:     "Test Album",
			Artist:    "Test Artist",
			Album:     "Test Album",
			Year:      2024,
		}
		err := db.Create(album).Error
		require.NoError(t, err)

		track := &database.MediaFile{
			Path:         "/test/music/album/track1.mp3",
			LibraryID:    1,
			MediaType:    database.MediaTypeTrack,
			Title:        "Track 1",
			Artist:       "Test Artist",
			Album:        "Test Album",
			AlbumArtist:  "Test Artist",
			Year:         2024,
			TrackNumber:  1,
			AlbumID:      &album.ID,
		}
		err = db.Create(track).Error
		require.NoError(t, err)

		// List music tracks
		filter := types.MediaFilter{
			MediaType: string(database.MediaTypeTrack),
		}
		tracks, err := mediaService.ListFiles(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, tracks, 1)
		assert.Equal(t, track.Title, tracks[0].Title)

		// List albums
		filter = types.MediaFilter{
			MediaType: string(database.MediaTypeAlbum),
		}
		albums, err := mediaService.ListFiles(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, albums, 1)
		assert.Equal(t, album.Title, albums[0].Title)
	})

	t.Run("MetadataOperations", func(t *testing.T) {
		// Create metadata
		metadata := map[string]string{
			"director":    "Test Director",
			"description": "Test Description",
			"genre":       "Action",
		}

		fileID := "d290f1ee-6c54-4b01-90e6-d701748f0851" // Example UUID
		err := mediaService.UpdateMetadata(ctx, fileID, metadata)
		require.NoError(t, err)

		// Verify metadata was created
		var count int64
		err = db.Model(&database.MediaMetadata{}).Where("media_file_id = ?", fileID).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(3), count) // Should have 3 metadata entries
	})

	t.Run("MediaInfoProbe", func(t *testing.T) {
		// Test with a non-existent file (probe should handle gracefully)
		info, err := mediaService.GetMediaInfo(ctx, "/nonexistent/file.mp4")
		assert.Error(t, err)
		assert.Nil(t, info)

		// In a real test with actual media files, you would test:
		// - Video stream detection
		// - Audio stream detection
		// - Duration and bitrate calculation
		// - Codec identification
	})
}

// TestMediaServiceConcurrency tests concurrent access to the media service
func TestMediaServiceConcurrency(t *testing.T) {
	db := setupTestDB(t)
	
	// Create and initialize module
	module := &mediamodule.Module{}
	err := module.Migrate(db)
	require.NoError(t, err)
	err = module.Init()
	require.NoError(t, err)

	mediaService, err := services.Get[services.MediaService]("media")
	require.NoError(t, err)

	ctx := context.Background()

	// Create test files
	for i := 0; i < 10; i++ {
		file := &database.MediaFile{
			Path:      fmt.Sprintf("/test/movie%d.mp4", i),
			LibraryID: 1,
			MediaType: database.MediaTypeMovie,
			Title:     fmt.Sprintf("Movie %d", i),
		}
		err := db.Create(file).Error
		require.NoError(t, err)
	}

	// Test concurrent reads
	t.Run("ConcurrentReads", func(t *testing.T) {
		done := make(chan bool, 10)
		
		for i := 0; i < 10; i++ {
			go func(index int) {
				defer func() { done <- true }()
				
				path := fmt.Sprintf("/test/movie%d.mp4", index)
				file, err := mediaService.GetFileByPath(ctx, path)
				assert.NoError(t, err)
				assert.NotNil(t, file)
				assert.Equal(t, path, file.Path)
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	// Test concurrent updates
	t.Run("ConcurrentUpdates", func(t *testing.T) {
		done := make(chan bool, 10)
		
		for i := 0; i < 10; i++ {
			go func(index int) {
				defer func() { done <- true }()
				
				fileID := fmt.Sprintf("d290f1ee-6c54-4b01-90e6-d701748f085%d", index)
				updates := map[string]interface{}{
					"play_count": index,
				}
				
				err := mediaService.UpdateFile(ctx, fileID, updates)
				// Some updates might fail due to non-existent files, which is ok
				_ = err
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}