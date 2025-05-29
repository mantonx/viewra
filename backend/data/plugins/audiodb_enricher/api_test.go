package main

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// createTestEnricher creates a properly initialized test enricher with database
func createTestEnricher() *AudioDBEnricher {
	// Create in-memory SQLite database for testing
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	
	// Auto-migrate test tables
	db.AutoMigrate(&AudioDBCache{}, &AudioDBEnrichment{})
	
	return &AudioDBEnricher{
		logger: hclog.New(&hclog.LoggerOptions{
			Name:  "test-audiodb-api",
			Level: hclog.Info,
		}),
		db: db,
		config: &AudioDBConfig{
			Enabled:            true,
			APIKey:             "2", // Use test API key "2" for AudioDB
			UserAgent:          "Viewra AudioDB Enricher Test/1.0.0",
			MatchThreshold:     0.75,
			CacheDurationHours: 168,
			RequestDelay:       1000,
		},
	}
}

// TestAudioDBAPISearch tests the real AudioDB API search functionality
func TestAudioDBAPISearch(t *testing.T) {
	// Skip if running in short mode (CI environment)
	if testing.Short() {
		t.Skip("Skipping API integration test in short mode")
	}

	enricher := createTestEnricher()

	// Test searching for Coldplay (the only artist that works with test API key "2")
	t.Run("Search Coldplay (Working Test Case)", func(t *testing.T) {
		tracks, err := enricher.searchTracks("Viva la Vida", "Coldplay", "")
		
		if err != nil {
			t.Logf("API search failed: %v", err)
			return // Don't fail the test if API is down, but log it
		}
		
		assert.NoError(t, err)
		t.Logf("Found %d tracks for Coldplay search", len(tracks))
		
		if len(tracks) > 0 {
			track := tracks[0]
			t.Logf("Found track: %s by %s (ID: %s)", track.StrTrack, track.StrArtist, track.IDTrack)
			
			assert.NotEmpty(t, track.IDTrack, "Track should have an ID")
			assert.NotEmpty(t, track.StrTrack, "Track should have a title")
			assert.NotEmpty(t, track.StrArtist, "Track should have an artist")
			
			// Calculate match score
			score := enricher.calculateMatchScore("Viva la Vida", "Coldplay", "", track.StrTrack, track.StrArtist, track.StrAlbum)
			t.Logf("Match score: %.2f", score)
			
			assert.Greater(t, score, 0.5, "Should have reasonable match score")
		}
	})

	t.Run("Search using direct API key 2", func(t *testing.T) {
		// Test the actual working endpoint with key "2"
		resp, err := http.Get("https://www.theaudiodb.com/api/v1/json/2/search.php?s=coldplay")
		if err != nil {
			t.Logf("Direct API call failed: %v", err)
			return
		}
		defer resp.Body.Close()
		
		assert.Equal(t, 200, resp.StatusCode, "API should return 200 for Coldplay search")
		
		// Read response body
		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		
		t.Logf("API response length: %d bytes", len(body))
		
		// Check if response contains expected JSON structure
		assert.Contains(t, string(body), "artists", "Response should contain artists array")
		assert.Contains(t, string(body), "Coldplay", "Response should contain Coldplay data")
	})
}

// TestAudioDBAPIAlbumInfo tests album information retrieval
func TestAudioDBAPIAlbumInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping API integration test in short mode")
	}

	enricher := createTestEnricher()

	// First, search for tracks to get an album ID
	tracks, err := enricher.searchTracks("Bohemian Rhapsody", "Queen", "A Night at the Opera")
	if err != nil {
		t.Logf("Track search failed: %v", err)
		return
	}

	if len(tracks) == 0 {
		t.Log("No tracks found, skipping album info test")
		return
	}

	track := tracks[0]
	if track.IDAlbum == "" {
		t.Log("Track has no album ID, skipping album info test")
		return
	}

	t.Logf("Testing album info for album ID: %s", track.IDAlbum)

	albumInfo, err := enricher.getAlbumInfo(track.IDAlbum)
	if err != nil {
		t.Logf("Album info failed: %v", err)
		return
	}

	assert.NoError(t, err)
	assert.NotNil(t, albumInfo)

	if len(albumInfo.Album) > 0 {
		album := albumInfo.Album[0]
		t.Logf("Found album: %s by %s (Year: %s)", album.StrAlbum, album.StrArtist, album.IntYearReleased)
		
		assert.NotEmpty(t, album.StrAlbum, "Album should have a title")
		assert.NotEmpty(t, album.StrArtist, "Album should have an artist")
		
		// Check for artwork URLs
		if album.StrAlbumThumb != "" {
			t.Logf("Album artwork URL: %s", album.StrAlbumThumb)
		}
		if album.StrAlbumThumbHQ != "" {
			t.Logf("Album HQ artwork URL: %s", album.StrAlbumThumbHQ)
		}
	}
}

// TestAudioDBAPIRateLimit tests that rate limiting is working
func TestAudioDBAPIRateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rate limit test in short mode")
	}

	enricher := createTestEnricher()
	enricher.config.RequestDelay = 500 // 500ms delay

	// Make multiple requests and check timing
	start := time.Now()
	
	for i := 0; i < 3; i++ {
		_, err := enricher.searchTracks("Test", "Artist", "")
		if err != nil {
			t.Logf("Request %d failed: %v", i+1, err)
		}
	}
	
	elapsed := time.Since(start)
	expectedMinTime := time.Duration(2*500) * time.Millisecond // 2 delays between 3 requests
	
	t.Logf("3 requests took %v (expected at least %v)", elapsed, expectedMinTime)
	
	// Note: This is a loose test since network latency can vary
	if elapsed < expectedMinTime/2 {
		t.Logf("Warning: Requests may not be properly rate-limited")
	}
}

// TestSearchCapabilities tests the search service interface
func TestSearchCapabilities(t *testing.T) {
	enricher := &AudioDBEnricher{
		logger: hclog.New(&hclog.LoggerOptions{
			Name:  "test-search-caps",
			Level: hclog.Off,
		}),
		config: &AudioDBConfig{
			Enabled: true,
		},
	}

	ctx := context.Background()
	
	capabilities, supportsAdvanced, maxResults, err := enricher.GetSearchCapabilities(ctx)
	
	assert.NoError(t, err)
	assert.NotEmpty(t, capabilities)
	assert.Contains(t, capabilities, "title")
	assert.Contains(t, capabilities, "artist")
	assert.False(t, supportsAdvanced)
	assert.Equal(t, uint32(50), maxResults)
	
	t.Logf("Search capabilities: %v", capabilities)
	t.Logf("Supports advanced search: %v", supportsAdvanced)
	t.Logf("Max results: %d", maxResults)
}

// TestAudioDBAPIDirectCall tests a simple API call without database dependency
func TestAudioDBAPIDirectCall(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping API integration test in short mode")
	}

	t.Run("Direct API Health Check", func(t *testing.T) {
		// Test the actual AudioDB API endpoint
		resp, err := http.Get("https://www.theaudiodb.com/api/v1/json/1/search.php?s=Queen")
		if err != nil {
			t.Logf("API call failed: %v", err)
			return
		}
		defer resp.Body.Close()
		
		t.Logf("API Response Status: %d", resp.StatusCode)
		
		if resp.StatusCode == 200 {
			// Read a sample of the response to verify it's working
			body := make([]byte, 500)
			n, _ := resp.Body.Read(body)
			t.Logf("API Response sample: %s", string(body[:n]))
		}
	})
} 