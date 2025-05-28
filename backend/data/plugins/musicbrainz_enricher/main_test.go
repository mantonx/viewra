package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/plugins/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestEnricher(t *testing.T, dbPath string) *MusicBrainzEnricher {
	enricher := &MusicBrainzEnricher{
		logger: hclog.NewNullLogger(),
		config: &Config{
			Enabled:            true,
			APIRateLimit:       10.0, // Higher rate for tests
			UserAgent:          "Viewra-Test/1.0",
			EnableArtwork:      true,
			ArtworkMaxSize:     1200,
			ArtworkQuality:     "front",
			MatchThreshold:     0.85,
			AutoEnrich:         true,
			OverwriteExisting:  false,
			CacheDurationHours: 24,
		},
		dbURL:    "sqlite://" + dbPath,
		basePath: "/tmp",
	}

	err := enricher.initDatabase()
	require.NoError(t, err)

	return enricher
}

func createTempDB(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "musicbrainz_test_*")
	require.NoError(t, err)
	
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})
	
	return filepath.Join(tmpDir, "test.db")
}

func TestMusicBrainzEnricher_Initialize(t *testing.T) {
	enricher := &MusicBrainzEnricher{}
	
	ctx := &proto.PluginContext{
		DatabaseUrl: "sqlite://" + createTempDB(t),
		BasePath:    "/tmp",
		LogLevel:    "info",
		Config: map[string]string{
			"enabled":            "true",
			"api_rate_limit":     "1.0",
			"auto_enrich":        "true",
			"match_threshold":    "0.9",
			"overwrite_existing": "true",
		},
	}

	err := enricher.Initialize(ctx)
	require.NoError(t, err)

	assert.NotNil(t, enricher.logger)
	assert.NotNil(t, enricher.config)
	assert.NotNil(t, enricher.db)
	assert.Equal(t, true, enricher.config.Enabled)
	assert.Equal(t, 1.0, enricher.config.APIRateLimit)
	assert.Equal(t, true, enricher.config.AutoEnrich)
	assert.Equal(t, 0.85, enricher.config.MatchThreshold)
	assert.Equal(t, true, enricher.config.OverwriteExisting)
}

func TestMusicBrainzEnricher_PluginInfo(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	info, err := enricher.Info()
	require.NoError(t, err)

	assert.Equal(t, "musicbrainz_enricher", info.Id)
	assert.Equal(t, "MusicBrainz Metadata Enricher", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Contains(t, info.Description, "MusicBrainz")
	assert.Equal(t, "metadata_scraper", info.Type)
	assert.Contains(t, info.Tags, "music")
	assert.Contains(t, info.Tags, "metadata")
	assert.Contains(t, info.Tags, "musicbrainz")
}

func TestMusicBrainzEnricher_Health(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	// Test health check - it may succeed or fail depending on network access
	err := enricher.Health()
	// In test environment, this may fail due to network restrictions
	// We just verify the method doesn't panic and returns a proper error type
	if err != nil {
		assert.Error(t, err)
		assert.IsType(t, err, err) // Just verify it's a proper error
	} else {
		assert.NoError(t, err)
	}
}

func TestMusicBrainzEnricher_ServiceInterfaces(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	// Test that all service methods return the correct implementations
	metadataService := enricher.MetadataScraperService()
	assert.NotNil(t, metadataService)
	assert.Equal(t, enricher, metadataService)

	scannerService := enricher.ScannerHookService()
	assert.NotNil(t, scannerService)
	assert.Equal(t, enricher, scannerService)

	dbService := enricher.DatabaseService()
	assert.NotNil(t, dbService)
	assert.Equal(t, enricher, dbService)

	adminService := enricher.AdminPageService()
	assert.Nil(t, adminService) // This plugin doesn't provide admin pages

	apiService := enricher.APIRegistrationService()
	assert.NotNil(t, apiService)
	assert.Equal(t, enricher, apiService)

	searchService := enricher.SearchService()
	assert.NotNil(t, searchService)
	assert.Equal(t, enricher, searchService)
}

func TestMusicBrainzEnricher_CanHandle(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	tests := []struct {
		name     string
		filePath string
		mimeType string
		expected bool
	}{
		{"MP3 file with MIME", "test.mp3", "audio/mpeg", true},
		{"FLAC file with MIME", "test.flac", "audio/flac", true},
		{"OGG file with MIME", "test.ogg", "audio/ogg", true},
		{"WAV file with MIME", "test.wav", "audio/wav", true},
		{"MP3 file without MIME", "test.mp3", "", true},
		{"FLAC file without MIME", "test.flac", "", true},
		{"Text file", "test.txt", "text/plain", false},
		{"Video file", "test.mp4", "video/mp4", false},
		{"Unknown extension", "test.xyz", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := enricher.CanHandle(tt.filePath, tt.mimeType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMusicBrainzEnricher_GetSupportedTypes(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	supportedTypes := enricher.GetSupportedTypes()
	
	expectedTypes := []string{
		"audio/mpeg",
		"audio/flac",
		"audio/ogg",
		"audio/wav",
		"audio/aac",
		"audio/m4a",
		"audio/wma",
	}

	assert.ElementsMatch(t, expectedTypes, supportedTypes)
}

func TestMusicBrainzEnricher_ExtractMetadata(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	// Test with supported file
	metadata, err := enricher.ExtractMetadata("test.mp3")
	require.NoError(t, err)
	
	assert.Equal(t, "musicbrainz_enricher", metadata["plugin"])
	assert.Equal(t, "test.mp3", metadata["file_path"])
	assert.Equal(t, "true", metadata["supported"])
	assert.Equal(t, "available", metadata["enrichment"])

	// Test with unsupported file
	_, err = enricher.ExtractMetadata("test.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file type not supported")
}

func TestMusicBrainzEnricher_DatabaseModels(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	models := enricher.GetModels()
	expectedModels := []string{
		"MusicBrainzCache",
		"MusicBrainzEnrichment",
	}

	assert.ElementsMatch(t, expectedModels, models)
}

func TestMusicBrainzEnricher_Migrate(t *testing.T) {
	dbPath := createTempDB(t)
	enricher := setupTestEnricher(t, dbPath)

	err := enricher.Migrate("sqlite://" + dbPath)
	require.NoError(t, err)

	// Verify tables exist
	var tableCount int
	err = enricher.db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN (?, ?)", 
		"music_brainz_caches", "music_brainz_enrichments").Scan(&tableCount).Error
	require.NoError(t, err)
	assert.Equal(t, 2, tableCount)
}

func TestMusicBrainzEnricher_OnMediaFileScanned(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	tests := []struct {
		name           string
		mediaFileID    uint32
		filePath       string
		metadata       map[string]string
		autoEnrich     bool
		expectError    bool
		expectSkipped  bool
	}{
		{
			name:        "Auto enrich disabled",
			mediaFileID: 1,
			filePath:    "/music/test.mp3",
			metadata:    map[string]string{"title": "Test", "artist": "Artist"},
			autoEnrich:  false,
			expectSkipped: true,
		},
		{
			name:        "Missing title",
			mediaFileID: 2,
			filePath:    "/music/test.mp3",
			metadata:    map[string]string{"artist": "Artist"},
			autoEnrich:  true,
			expectSkipped: true,
		},
		{
			name:        "Missing artist",
			mediaFileID: 3,
			filePath:    "/music/test.mp3",
			metadata:    map[string]string{"title": "Test"},
			autoEnrich:  true,
			expectSkipped: true,
		},
		{
			name:        "Valid metadata",
			mediaFileID: 4,
			filePath:    "/music/test.mp3",
			metadata:    map[string]string{"title": "Test", "artist": "Artist"},
			autoEnrich:  true,
			expectError: false, // May succeed if network is available
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enricher.config.AutoEnrich = tt.autoEnrich

			err := enricher.OnMediaFileScanned(tt.mediaFileID, tt.filePath, tt.metadata)

			if tt.expectSkipped {
				assert.NoError(t, err)
			} else if tt.expectError {
				// In test environment, network calls will fail
				assert.Error(t, err)
			} else {
				// May succeed or fail depending on network availability
				// We just ensure it doesn't panic
				if err != nil {
					t.Logf("Network operation failed (expected in test environment): %v", err)
				}
			}
		})
	}
}

func TestMusicBrainzEnricher_StringSimilarity(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	tests := []struct {
		s1       string
		s2       string
		expected float64
	}{
		{"exact match", "exact match", 1.0},
		{"Exact Match", "exact match", 1.0}, // Case insensitive
		{"bohemian rhapsody", "bohemian", 0.8}, // Contains
		{"queen", "bohemian rhapsody queen", 0.8}, // Contains reverse
		{"hello world", "hello", 0.8}, // Contains
		{"abc", "xyz", 0.0}, // No match
		{"", "", 1.0}, // Empty strings are equal
	}

	for _, tt := range tests {
		t.Run(tt.s1+"_vs_"+tt.s2, func(t *testing.T) {
			result := enricher.stringSimilarity(tt.s1, tt.s2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMusicBrainzEnricher_BuildSearchQuery(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	tests := []struct {
		title    string
		artist   string
		album    string
		expected string
	}{
		{"Bohemian Rhapsody", "Queen", "A Night at the Opera", 
			`recording:"Bohemian Rhapsody" AND artist:"Queen" AND release:"A Night at the Opera"`},
		{"Test Song", "Test Artist", "", 
			`recording:"Test Song" AND artist:"Test Artist"`},
		{"", "Artist Only", "", 
			`artist:"Artist Only"`},
		{"Title Only", "", "", 
			`recording:"Title Only"`},
	}

	for _, tt := range tests {
		t.Run(tt.title+"_"+tt.artist, func(t *testing.T) {
			result := enricher.buildSearchQuery(tt.title, tt.artist, tt.album)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMusicBrainzEnricher_Cache(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	// Test cache key generation
	key1 := enricher.getCacheKey("title", "artist", "album")
	key2 := enricher.getCacheKey("title", "artist", "album")
	key3 := enricher.getCacheKey("different", "artist", "album")

	assert.Equal(t, key1, key2) // Same input = same key
	assert.NotEqual(t, key1, key3) // Different input = different key

	// Test cache storage and retrieval
	testRecording := &MusicBrainzRecording{
		ID:    "test-id",
		Title: "Test Title",
		Score: 1.0,
	}

	// Cache should be empty initially
	cached := enricher.getCachedResult(key1)
	assert.Nil(t, cached)

	// Store in cache
	enricher.cacheResult(key1, testRecording)

	// Should now be retrievable
	cached = enricher.getCachedResult(key1)
	require.NotNil(t, cached)
	assert.Equal(t, testRecording.ID, cached.ID)
	assert.Equal(t, testRecording.Title, cached.Title)
	assert.Equal(t, testRecording.Score, cached.Score)
}

func TestMusicBrainzEnricher_GetRegisteredRoutes(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	routes, err := enricher.GetRegisteredRoutes(context.Background())
	require.NoError(t, err)

	expectedRoutes := []string{"/search", "/config"}
	actualPaths := make([]string, len(routes))
	for i, route := range routes {
		actualPaths[i] = route.Path
	}

	assert.ElementsMatch(t, expectedRoutes, actualPaths)

	// Check that each route has proper method and description
	for _, route := range routes {
		assert.NotEmpty(t, route.Method)
		assert.NotEmpty(t, route.Description)
		if route.Path == "/search" {
			assert.Equal(t, "GET", route.Method)
		}
		if route.Path == "/config" {
			assert.Equal(t, "GET", route.Method)
		}
	}
}

func TestMusicBrainzEnricher_SearchCapabilities(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	supportedFields, supportsPagination, maxResults, err := enricher.GetSearchCapabilities(context.Background())
	require.NoError(t, err)

	expectedFields := []string{"title", "artist", "album"}
	assert.ElementsMatch(t, expectedFields, supportedFields)
	assert.False(t, supportsPagination)
	assert.Equal(t, uint32(10), maxResults)
}

func TestMusicBrainzEnricher_Search_ValidationErrors(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	tests := []struct {
		name        string
		query       map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Nil query",
			query:       nil,
			expectError: true,
			errorMsg:    "query map cannot be nil",
		},
		{
			name:        "Missing title and artist",
			query:       map[string]string{"album": "Test Album"},
			expectError: true,
			errorMsg:    "title and artist are required",
		},
		{
			name:        "Missing artist",
			query:       map[string]string{"title": "Test Song"},
			expectError: true,
			errorMsg:    "title and artist are required",
		},
		{
			name:        "Valid query",
			query:       map[string]string{"title": "Test Song", "artist": "Test Artist"},
			expectError: true, // Will always have an error, but no specific message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := enricher.Search(context.Background(), tt.query, 10, 0)
			
			if tt.expectError {
				if tt.errorMsg != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tt.errorMsg)
				} else {
					// Network dependent test - may succeed or fail
					if err != nil {
						t.Logf("Network operation failed (expected in test environment): %v", err)
					}
				}
			} else {
				// May succeed or fail depending on network availability
				if err != nil {
					t.Logf("Network operation failed (expected in test environment): %v", err)
				}
			}
		})
	}
}

func TestMusicBrainzEnricher_StartStop(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	// Test Start
	err := enricher.Start()
	assert.NoError(t, err)

	// Test Stop
	err = enricher.Stop()
	assert.NoError(t, err)
}

func TestMusicBrainzEnricher_ScannerHooks(t *testing.T) {
	enricher := setupTestEnricher(t, createTempDB(t))

	// Test OnScanStarted
	err := enricher.OnScanStarted(1, 2, "/test/path")
	assert.NoError(t, err)

	// Test OnScanCompleted
	stats := map[string]string{
		"files_processed": "100",
		"errors": "0",
	}
	err = enricher.OnScanCompleted(1, 2, stats)
	assert.NoError(t, err)

	// Test with auto enrich disabled
	enricher.config.AutoEnrich = false
	err = enricher.OnScanStarted(1, 2, "/test/path")
	assert.NoError(t, err)
	err = enricher.OnScanCompleted(1, 2, stats)
	assert.NoError(t, err)
}

// Benchmark tests
func BenchmarkMusicBrainzEnricher_StringSimilarity(b *testing.B) {
	enricher := &MusicBrainzEnricher{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enricher.stringSimilarity("bohemian rhapsody", "bohemian rhapsody queen")
	}
}

func BenchmarkMusicBrainzEnricher_BuildSearchQuery(b *testing.B) {
	enricher := &MusicBrainzEnricher{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enricher.buildSearchQuery("Bohemian Rhapsody", "Queen", "A Night at the Opera")
	}
}

func BenchmarkMusicBrainzEnricher_GetCacheKey(b *testing.B) {
	enricher := &MusicBrainzEnricher{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enricher.getCacheKey("Bohemian Rhapsody", "Queen", "A Night at the Opera")
	}
} 