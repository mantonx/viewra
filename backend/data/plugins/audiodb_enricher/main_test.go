package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mantonx/viewra/plugins/audiodb_enricher/audiodb"
	"github.com/mantonx/viewra/plugins/audiodb_enricher/config"
	"github.com/mantonx/viewra/plugins/audiodb_enricher/internal"
	"github.com/mantonx/viewra/plugins/audiodb_enricher/tagreader"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// createTestLogger creates a test logger instance
func createTestLogger() hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Name:  "test-audiodb-enricher",
		Level: hclog.Off, // Suppress logs during testing
	})
}

func TestAudioDBConfig(t *testing.T) {
	// Test default configuration
	cfg := config.DefaultConfig()
	if !cfg.Enabled {
		t.Error("Default config should be enabled")
	}
	if cfg.MatchThreshold != 0.75 {
		t.Errorf("Expected match threshold 0.75, got %f", cfg.MatchThreshold)
	}
	if cfg.CacheDurationHours != 168 {
		t.Errorf("Expected cache duration 168 hours, got %d", cfg.CacheDurationHours)
	}

	// Test configuration validation
	if err := cfg.Validate(); err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}

	// Test invalid configuration
	cfg.MatchThreshold = 1.5 // Invalid threshold
	if err := cfg.Validate(); err == nil {
		t.Error("Config with invalid match threshold should fail validation")
	}

	// Test configuration overrides
	cfg = config.DefaultConfig()
	overrides := map[string]string{
		"enabled":        "false",
		"match_threshold": "0.8",
		"auto_enrich":    "false",
	}
	
	if err := cfg.ApplyOverrides(overrides); err != nil {
		t.Errorf("Failed to apply overrides: %v", err)
	}
	
	if cfg.Enabled {
		t.Error("Config should be disabled after override")
	}
	if cfg.MatchThreshold != 0.8 {
		t.Errorf("Expected match threshold 0.8, got %f", cfg.MatchThreshold)
	}
	if cfg.AutoEnrich {
		t.Error("AutoEnrich should be disabled after override")
	}
}

func TestAudioDBModels(t *testing.T) {
	// Test track model
	track := audiodb.Track{
		IDTrack:   "123",
		StrTrack:  "Test Song",
		StrArtist: "Test Artist",
		StrAlbum:  "Test Album",
		StrGenre:  "Rock",
	}

	if track.StrTrack != "Test Song" {
		t.Errorf("Expected track title 'Test Song', got '%s'", track.StrTrack)
	}

	// Test artist model
	artist := audiodb.Artist{
		IDArtist:  "456",
		StrArtist: "Test Artist",
		StrGenre:  "Rock",
		StrCountry: "USA",
	}

	if artist.StrArtist != "Test Artist" {
		t.Errorf("Expected artist name 'Test Artist', got '%s'", artist.StrArtist)
	}

	// Test album model
	album := audiodb.Album{
		IDAlbum:         "789",
		StrAlbum:        "Test Album",
		StrArtist:       "Test Artist",
		IntYearReleased: "2023",
	}

	if album.StrAlbum != "Test Album" {
		t.Errorf("Expected album name 'Test Album', got '%s'", album.StrAlbum)
	}
}

func TestAudioDBMapper(t *testing.T) {
	mapper := audiodb.NewMapper()

	// Test match score calculation
	score := mapper.CalculateMatchScore("Hello", "Artist", "Album", "Hello", "Artist", "Album")
	if score != 1.0 {
		t.Errorf("Expected perfect match score 1.0, got %f", score)
	}

	score = mapper.CalculateMatchScore("Hello", "Artist", "", "World", "Artist", "")
	if score >= 1.0 {
		t.Errorf("Expected imperfect match score < 1.0, got %f", score)
	}

	// Test track to search result conversion
	track := audiodb.Track{
		IDTrack:   "123",
		StrTrack:  "Test Song",
		StrArtist: "Test Artist",
		StrAlbum:  "Test Album",
		StrGenre:  "Rock",
	}

	result := mapper.TrackToSearchResult(track, 0.85)
	if result.Id != "123" {
		t.Errorf("Expected result ID '123', got '%s'", result.Id)
	}
	if result.Title != "Test Song" {
		t.Errorf("Expected result title 'Test Song', got '%s'", result.Title)
	}
	if result.Score != 0.85 {
		t.Errorf("Expected result score 0.85, got %f", result.Score)
	}

	// Test metadata extraction
	metadata := mapper.ExtractMetadataFromTrack(track)
	if metadata["title"] != "Test Song" {
		t.Errorf("Expected metadata title 'Test Song', got '%v'", metadata["title"])
	}
	if metadata["audiodb_track_id"] != "123" {
		t.Errorf("Expected AudioDB track ID '123', got '%v'", metadata["audiodb_track_id"])
	}
}

func TestInternalUtils(t *testing.T) {
	// Test file utilities
	fileUtils := internal.NewFileUtils()
	
	if !fileUtils.IsSupportedAudioFile("/path/to/song.mp3") {
		t.Error("MP3 file should be supported")
	}
	if fileUtils.IsSupportedAudioFile("/path/to/document.txt") {
		t.Error("Text file should not be supported")
	}

	ext := fileUtils.GetFileExtension("/path/to/song.mp3")
	if ext != "mp3" {
		t.Errorf("Expected extension 'mp3', got '%s'", ext)
	}

	// Test string utilities
	stringUtils := internal.NewStringUtils()
	
	normalized := stringUtils.NormalizeString("  Hello World  ")
	if normalized != "hello world" {
		t.Errorf("Expected normalized string 'hello world', got '%s'", normalized)
	}

	if !stringUtils.IsEmpty("   ") {
		t.Error("Whitespace-only string should be considered empty")
	}
	if stringUtils.IsEmpty("hello") {
		t.Error("Non-empty string should not be considered empty")
	}

	truncated := stringUtils.TruncateString("This is a very long string", 10)
	if len(truncated) > 10 {
		t.Errorf("Truncated string should be max 10 chars, got %d", len(truncated))
	}

	// Test cache key builder
	cacheBuilder := internal.NewCacheKeyBuilder()
	
	key := cacheBuilder.BuildTrackSearchKey("Song", "Artist", "Album")
	if key == "" {
		t.Error("Cache key should not be empty")
	}

	artistKey := cacheBuilder.BuildArtistSearchKey("Artist Name")
	if artistKey == "" {
		t.Error("Artist cache key should not be empty")
	}
}

func TestTagReader(t *testing.T) {
	reader := tagreader.NewReader()

	// Test supported formats
	if !reader.IsSupported("/path/to/song.mp3") {
		t.Error("MP3 should be supported")
	}
	if reader.IsSupported("/path/to/document.txt") {
		t.Error("Text file should not be supported")
	}

	formats := reader.GetSupportedFormats()
	if len(formats) == 0 {
		t.Error("Should have supported formats")
	}

	// Test basic info extraction
	metadata := reader.ExtractBasicInfo("/music/Artist/Album/01 - Song Title.mp3")
	if metadata.Artist == "" {
		t.Error("Should extract artist from path")
	}
	if metadata.Album == "" {
		t.Error("Should extract album from path")
	}
	if metadata.Title == "" {
		t.Error("Should extract title from filename")
	}

	// Test metadata map conversion
	metadata.Title = "Test Song"
	metadata.Artist = "Test Artist"
	metadata.Year = 2023
	
	metadataMap := metadata.ExtractMetadataMap()
	if metadataMap["title"] != "Test Song" {
		t.Errorf("Expected title 'Test Song', got '%s'", metadataMap["title"])
	}
	if metadataMap["year"] != "2023" {
		t.Errorf("Expected year '2023', got '%s'", metadataMap["year"])
	}

	// Test metadata validation
	if !metadata.HasBasicMetadata() {
		t.Error("Metadata should have basic info (title and artist)")
	}

	emptyMetadata := &tagreader.Metadata{}
	if !emptyMetadata.IsEmpty() {
		t.Error("Empty metadata should be detected as empty")
	}
}

func TestAudioDBClient(t *testing.T) {
	// Skip if no network access or in CI environment
	if testing.Short() {
		t.Skip("Skipping network test in short mode")
	}

	logger := hclog.NewNullLogger()
	client := audiodb.NewClient(logger, "", "Test/1.0")

	// Test health check (this makes a real API call)
	err := client.HealthCheck()
	if err != nil {
		t.Logf("Health check failed (expected in testing): %v", err)
		// Don't fail the test as this might be expected in CI
	}
}

func TestAudioDBEnricher(t *testing.T) {
	logger := hclog.NewNullLogger()
	enricher := &AudioDBEnricher{
		logger: logger,
		config: &AudioDBConfig{
			Enabled:              true,
			UserAgent:            "Test/1.0",
			MatchThreshold:       0.75,
			AutoEnrich:           true,
			OverwriteExisting:    false,
			CacheDurationHours:   168,
			RequestDelay:         1000,
		},
	}

	// Test plugin info
	info, err := enricher.Info()
	if err != nil {
		t.Errorf("Failed to get plugin info: %v", err)
	}
	if info.Id != "audiodb_enricher" {
		t.Errorf("Expected plugin ID 'audiodb_enricher', got '%s'", info.Id)
	}
	if info.Name != "AudioDB Metadata Enricher" {
		t.Errorf("Expected plugin name 'AudioDB Metadata Enricher', got '%s'", info.Name)
	}

	// Test file support
	if !enricher.CanHandle("/path/to/song.mp3", "audio/mpeg") {
		t.Error("Should handle MP3 files")
	}
	if enricher.CanHandle("/path/to/document.txt", "text/plain") {
		t.Error("Should not handle text files")
	}

	// Test supported types
	types := enricher.GetSupportedTypes()
	if len(types) == 0 {
		t.Error("Should have supported types")
	}

	foundMP3 := false
	for _, mimeType := range types {
		if mimeType == "audio/mpeg" {
			foundMP3 = true
			break
		}
	}
	if !foundMP3 {
		t.Error("Should support audio/mpeg")
	}
}

func TestInternalModels(t *testing.T) {
	// Test AudioDBCache model
	cache := internal.AudioDBCache{
		SearchQuery: "test_query",
		APIResponse: `{"test": "data"}`,
		CachedAt:    time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	if cache.SearchQuery != "test_query" {
		t.Errorf("Expected search query 'test_query', got '%s'", cache.SearchQuery)
	}

	// Test AudioDBEnrichment model
	enrichment := internal.AudioDBEnrichment{
		MediaFileID:     123,
		AudioDBTrackID:  "456",
		EnrichedTitle:   "Test Song",
		EnrichedArtist:  "Test Artist",
		MatchScore:      0.85,
		EnrichedAt:      time.Now(),
	}

	if enrichment.MediaFileID != 123 {
		t.Errorf("Expected media file ID 123, got %d", enrichment.MediaFileID)
	}
	if enrichment.MatchScore != 0.85 {
		t.Errorf("Expected match score 0.85, got %f", enrichment.MatchScore)
	}
}

// Benchmark tests
func BenchmarkStringNormalization(b *testing.B) {
	stringUtils := internal.NewStringUtils()
	testString := "  Hello World With Special Characters!@#$%  "
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stringUtils.NormalizeString(testString)
	}
}

func BenchmarkMatchScoreCalculation(b *testing.B) {
	mapper := audiodb.NewMapper()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mapper.CalculateMatchScore("Hello World", "Test Artist", "Test Album", "Hello World", "Test Artist", "Test Album")
	}
}

func BenchmarkCacheKeyBuilding(b *testing.B) {
	cacheBuilder := internal.NewCacheKeyBuilder()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cacheBuilder.BuildTrackSearchKey("Song Title", "Artist Name", "Album Name")
	}
}

// MockMediaAssetService implements MediaAssetService for testing
type MockMediaAssetService struct {
	savedAssets []AssetRequest
	shouldFail  bool
}

func (m *MockMediaAssetService) SaveAsset(ctx context.Context, request *AssetRequest) (*AssetResponse, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("mock save asset failure")
	}
	
	m.savedAssets = append(m.savedAssets, *request)
	
	return &AssetResponse{
		ID:          uint(len(m.savedAssets)),
		MediaFileID: request.MediaFileID,
		Type:        request.Type,
		Category:    request.Category,
		Subtype:     request.Subtype,
		Path:        fmt.Sprintf("/assets/%d", len(m.savedAssets)),
		MimeType:    request.MimeType,
		Size:        int64(len(request.Data)),
		Hash:        "mock-hash",
		CreatedAt:   time.Now().Unix(),
	}, nil
}

func (m *MockMediaAssetService) AssetExists(ctx context.Context, mediaFileID uint, assetType, category string) (bool, error) {
	if m.shouldFail {
		return false, fmt.Errorf("mock asset exists failure")
	}
	
	// Check if we already have an asset for this file/type/category
	for _, asset := range m.savedAssets {
		if asset.MediaFileID == mediaFileID && asset.Type == assetType && asset.Category == category {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockMediaAssetService) GetAsset(ctx context.Context, assetID uint) (*AssetResponse, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("mock get asset failure")
	}
	
	if assetID <= 0 || int(assetID) > len(m.savedAssets) {
		return nil, fmt.Errorf("asset not found")
	}
	
	asset := m.savedAssets[assetID-1]
	return &AssetResponse{
		ID:          assetID,
		MediaFileID: asset.MediaFileID,
		Type:        asset.Type,
		Category:    asset.Category,
		Subtype:     asset.Subtype,
		Path:        fmt.Sprintf("/assets/%d", assetID),
		MimeType:    asset.MimeType,
		Size:        int64(len(asset.Data)),
		Hash:        "mock-hash",
		CreatedAt:   time.Now().Unix(),
	}, nil
}

func (m *MockMediaAssetService) DeleteAsset(ctx context.Context, assetID uint) error {
	if m.shouldFail {
		return fmt.Errorf("mock delete asset failure")
	}
	return nil
}

// TestMediaAssetServiceIntegration tests the plugin's use of MediaAssetService
func TestMediaAssetServiceIntegration(t *testing.T) {
	enricher := &AudioDBEnricher{
		logger: createTestLogger(),
		config: &AudioDBConfig{
			Enabled:              true,
			EnableArtwork:        true,
			DownloadAlbumArt:     true,
			SkipExistingAssets:   true,
			MaxAssetSize:         1024 * 1024, // 1MB
			AssetTimeout:         30,
			RetryFailedDownloads: true,
			MaxRetries:           3,
		},
	}
	
	// Test that MediaAssetService is initially nil
	assert.Nil(t, enricher.MediaAssetService())
	
	// Test setting MediaAssetService
	mockService := &MockMediaAssetService{}
	enricher.SetMediaAssetService(mockService)
	
	// Verify service is set
	assert.Equal(t, mockService, enricher.MediaAssetService())
	
	// Test asset existence check (when skip_existing_assets is true)
	ctx := context.Background()
	exists, err := enricher.assetService.AssetExists(ctx, 123, "music", "album")
	assert.NoError(t, err)
	assert.False(t, exists)
	
	// Test SaveAsset through the service
	request := &AssetRequest{
		MediaFileID: 123,
		Type:        "music",
		Category:    "album",
		Subtype:     "artwork",
		Data:        []byte("fake image data"),
		MimeType:    "image/jpeg",
		SourceURL:   "http://example.com/image.jpg",
	}
	
	response, err := mockService.SaveAsset(ctx, request)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, uint(123), response.MediaFileID)
	assert.Equal(t, "music", response.Type)
	assert.Equal(t, "album", response.Category)
	
	// Verify the asset was saved in our mock
	assert.Len(t, mockService.savedAssets, 1)
	assert.Equal(t, "music", mockService.savedAssets[0].Type)
	
	// Test that asset now exists
	exists, err = enricher.assetService.AssetExists(ctx, 123, "music", "album")
	assert.NoError(t, err)
	assert.True(t, exists) // Should now exist after we saved it
}

// TestMediaAssetServiceFailure tests error handling with MediaAssetService
func TestMediaAssetServiceFailure(t *testing.T) {
	enricher := &AudioDBEnricher{
		logger: createTestLogger(),
		config: &AudioDBConfig{
			Enabled:            true,
			EnableArtwork:      true,
			SkipExistingAssets: true,
		},
	}
	
	// Test with failing service
	mockService := &MockMediaAssetService{shouldFail: true}
	enricher.SetMediaAssetService(mockService)
	
	ctx := context.Background()
	
	// Test that asset existence check handles failures gracefully
	exists, err := enricher.assetService.AssetExists(ctx, 123, "music", "album")
	assert.Error(t, err)
	assert.False(t, exists)
}

func TestAudioDBFullWorkflowIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full workflow integration test in short mode")
	}

	// Create test enricher with database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	assert.NoError(t, err)
	
	// Migrate tables
	err = db.AutoMigrate(&AudioDBCache{}, &AudioDBEnrichment{})
	assert.NoError(t, err)
	
	// Create mock asset service
	mockAssetService := &MockMediaAssetService{
		savedAssets: make([]AssetRequest, 0),
		shouldFail:  false,
	}
	
	// Create enricher with working configuration
	enricher := &AudioDBEnricher{
		logger: hclog.New(&hclog.LoggerOptions{
			Name:  "test-audiodb-full-workflow",
			Level: hclog.Info,
		}),
		db: db,
		config: &AudioDBConfig{
			Enabled:              true,
			APIKey:               "2", // Working test key for Coldplay
			UserAgent:            "Viewra AudioDB Enricher Test/1.0.0",
			MatchThreshold:       0.6,
			CacheDurationHours:   1,
			RequestDelay:         1000, // 1 second delay
			EnableArtwork:        true,
			DownloadAlbumArt:     true,
			DownloadArtistImages: false, // Keep false to limit API calls
			MaxAssetSize:         10 * 1024 * 1024, // 10MB
			AssetTimeout:         30,
			SkipExistingAssets:   false, // Don't skip for testing
			RetryFailedDownloads: true,
			MaxRetries:           2,
		},
		assetService: mockAssetService,
	}

	t.Run("Complete Enrichment with Database Persistence", func(t *testing.T) {
		mediaFileID := uint(54321)
		
		// Perform enrichment
		err := enricher.enrichTrack(mediaFileID, "Yellow", "Coldplay", "Parachutes")
		
		// Allow time for async operations
		time.Sleep(4 * time.Second)
		
		if err != nil {
			t.Logf("Enrichment completed with note: %v", err)
		}

		// ✅ Verify database enrichment record was created
		var enrichment AudioDBEnrichment
		err = db.Where("media_file_id = ?", mediaFileID).First(&enrichment).Error
		assert.NoError(t, err, "Enrichment record should be saved to database")
		
		if err == nil {
			t.Logf("✅ DATABASE RECORD SUCCESSFULLY CREATED:")
			t.Logf("   - Media File ID: %d", enrichment.MediaFileID)
			t.Logf("   - AudioDB Track ID: %s", enrichment.AudioDBTrackID)
			t.Logf("   - AudioDB Artist ID: %s", enrichment.AudioDBArtistID)
			t.Logf("   - AudioDB Album ID: %s", enrichment.AudioDBAlbumID)
			t.Logf("   - Enriched Title: %s", enrichment.EnrichedTitle)
			t.Logf("   - Enriched Artist: %s", enrichment.EnrichedArtist)
			t.Logf("   - Enriched Album: %s", enrichment.EnrichedAlbum)
			t.Logf("   - Enriched Year: %d", enrichment.EnrichedYear)
			t.Logf("   - Enriched Genre: %s", enrichment.EnrichedGenre)
			t.Logf("   - Match Score: %.2f", enrichment.MatchScore)
			t.Logf("   - Artwork URL: %s", enrichment.ArtworkURL)
			t.Logf("   - Enriched At: %s", enrichment.EnrichedAt.Format(time.RFC3339))
			
			// Core assertions
			assert.NotEmpty(t, enrichment.AudioDBTrackID)
			assert.NotEmpty(t, enrichment.AudioDBArtistID)
			assert.NotEmpty(t, enrichment.AudioDBAlbumID)
			assert.Equal(t, "Coldplay", enrichment.EnrichedArtist)
			assert.Greater(t, enrichment.MatchScore, 0.6)
		}

		// ✅ Verify cache entry was created
		var cacheCount int64
		db.Model(&AudioDBCache{}).Count(&cacheCount)
		assert.Greater(t, cacheCount, int64(0), "Cache entries should be created")
		
		if cacheCount > 0 {
			var cache AudioDBCache
			db.First(&cache)
			t.Logf("✅ CACHE ENTRY SUCCESSFULLY CREATED:")
			t.Logf("   - Search Query: %s", cache.SearchQuery)
			t.Logf("   - Response Size: %d bytes", len(cache.APIResponse))
			t.Logf("   - Cached At: %s", cache.CachedAt.Format(time.RFC3339))
			t.Logf("   - Expires At: %s", cache.ExpiresAt.Format(time.RFC3339))
		}

		// ✅ Verify asset downloads were attempted
		assetSaveCount := len(mockAssetService.savedAssets)
		if assetSaveCount > 0 {
			t.Logf("✅ ASSET DOWNLOADS SUCCESSFULLY PROCESSED:")
			for i, asset := range mockAssetService.savedAssets {
				t.Logf("   Asset %d:", i+1)
				t.Logf("     - Media File ID: %d", asset.MediaFileID)
				t.Logf("     - Type: %s", asset.Type)
				t.Logf("     - Category: %s", asset.Category)
				t.Logf("     - Subtype: %s", asset.Subtype)
				t.Logf("     - MIME Type: %s", asset.MimeType)
				t.Logf("     - Data Size: %d bytes", len(asset.Data))
				t.Logf("     - Source URL: %s", asset.SourceURL)
				
				// Verify asset data integrity
				assert.Equal(t, mediaFileID, asset.MediaFileID)
				assert.Equal(t, "music", asset.Type)
				assert.Greater(t, len(asset.Data), 0, "Asset data should not be empty")
				assert.NotEmpty(t, asset.SourceURL, "Source URL should be provided")
				assert.Contains(t, asset.MimeType, "image/", "Should be an image MIME type")
			}
		} else {
			t.Log("ℹ️  No assets were downloaded (might be due to API limitations or async timing)")
		}

		// Database summary
		var totalEnrichments, totalCaches int64
		db.Model(&AudioDBEnrichment{}).Count(&totalEnrichments)
		db.Model(&AudioDBCache{}).Count(&totalCaches)
		
		t.Logf("📊 FINAL DATABASE SUMMARY:")
		t.Logf("   - Total Enrichment Records: %d", totalEnrichments)
		t.Logf("   - Total Cache Records: %d", totalCaches)
		t.Logf("   - Total Asset Downloads: %d", assetSaveCount)
	})

	t.Run("Verify Database Query Performance", func(t *testing.T) {
		// Test that we can efficiently query enrichments
		var enrichments []AudioDBEnrichment
		start := time.Now()
		err := db.Find(&enrichments).Error
		queryTime := time.Since(start)
		
		assert.NoError(t, err)
		t.Logf("✅ Database query completed in %v", queryTime)
		t.Logf("   - Found %d enrichment records", len(enrichments))
		
		if len(enrichments) > 0 {
			// Test specific field queries
			var countByColdplay int64
			db.Model(&AudioDBEnrichment{}).Where("enriched_artist = ?", "Coldplay").Count(&countByColdplay)
			t.Logf("   - Coldplay enrichments: %d", countByColdplay)
			
			assert.Greater(t, countByColdplay, int64(0), "Should have Coldplay enrichments")
		}
	})
}

func TestSearchTracksDebugging(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping search tracks debugging test in short mode")
	}

	// Create test enricher with working configuration
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	assert.NoError(t, err)
	db.AutoMigrate(&AudioDBCache{}, &AudioDBEnrichment{})
	
	enricher := &AudioDBEnricher{
		logger: hclog.New(&hclog.LoggerOptions{
			Name:  "test-search-debug",
			Level: hclog.Debug, // Enable debug logging
		}),
		db: db,
		config: &AudioDBConfig{
			Enabled:            true,
			APIKey:             "2", // Working test key for Coldplay
			UserAgent:          "Viewra AudioDB Enricher Test/1.0.0",
			MatchThreshold:     0.6,
			CacheDurationHours: 1,
			RequestDelay:       1000, // 1 second delay
		},
	}

	t.Run("Test searchTracks with Coldplay", func(t *testing.T) {
		t.Log("🔍 Testing searchTracks function directly...")
		
		tracks, err := enricher.searchTracks("Yellow", "Coldplay", "Parachutes")
		
		if err != nil {
			t.Logf("❌ searchTracks failed: %v", err)
			
			// Let's test the individual API endpoints directly
			t.Log("🔧 Testing individual API endpoints:")
			
			// Test 1: Direct artist search
			resp, err := http.Get("https://www.theaudiodb.com/api/v1/json/2/search.php?s=coldplay")
			if err != nil {
				t.Logf("❌ Direct artist search failed: %v", err)
			} else {
				t.Logf("✅ Direct artist search: HTTP %d", resp.StatusCode)
				resp.Body.Close()
			}
			
			// Test 2: Albums for known artist ID
			time.Sleep(1 * time.Second)
			resp, err = http.Get("https://www.theaudiodb.com/api/v1/json/2/album.php?i=111239")
			if err != nil {
				t.Logf("❌ Direct albums search failed: %v", err)
			} else {
				t.Logf("✅ Direct albums search: HTTP %d", resp.StatusCode)
				resp.Body.Close()
			}
			
			return
		}
		
		t.Logf("✅ searchTracks succeeded! Found %d tracks", len(tracks))
		
		for i, track := range tracks {
			t.Logf("   Track %d: %s by %s (Album: %s)", i+1, track.StrTrack, track.StrArtist, track.StrAlbum)
			if i >= 4 { // Limit output
				t.Logf("   ... and %d more tracks", len(tracks)-5)
				break
			}
		}
		
		// Verify we found some Coldplay tracks
		assert.Greater(t, len(tracks), 0, "Should find at least some tracks")
		
		// Look for specific tracks
		foundYellow := false
		foundClocks := false
		for _, track := range tracks {
			if strings.Contains(strings.ToLower(track.StrTrack), "yellow") {
				foundYellow = true
				t.Logf("✅ Found Yellow: %s", track.StrTrack)
			}
			if strings.Contains(strings.ToLower(track.StrTrack), "clocks") {
				foundClocks = true
				t.Logf("✅ Found Clocks: %s", track.StrTrack)
			}
		}
		
		if foundYellow {
			t.Log("✅ Successfully found Yellow track in results")
		}
		if foundClocks {
			t.Log("✅ Successfully found Clocks track in results")
		}
	})
} 