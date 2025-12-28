package pipeline

import (
	"testing"
)

func TestNewEntityCache(t *testing.T) {
	cache := NewEntityCache(100)
	if cache == nil {
		t.Fatal("NewEntityCache returned nil")
	}

	stats := cache.Stats()
	if stats.MaxSize != 100 {
		t.Errorf("expected maxSize 100, got %d", stats.MaxSize)
	}
	if stats.Shows != 0 || stats.Seasons != 0 || stats.Albums != 0 || stats.Artists != 0 {
		t.Error("expected empty cache")
	}
}

func TestNewEntityCache_DefaultSize(t *testing.T) {
	cache := NewEntityCache(0)
	if cache == nil {
		t.Fatal("NewEntityCache returned nil")
	}

	stats := cache.Stats()
	if stats.MaxSize != 10000 {
		t.Errorf("expected default maxSize 10000, got %d", stats.MaxSize)
	}
}

func TestEntityCache_Shows(t *testing.T) {
	cache := NewEntityCache(100)

	// Test Get on empty cache
	if got := cache.GetShow(1); got != nil {
		t.Error("expected nil for missing show")
	}

	// Test Put and Get
	show := &CachedTVShow{
		ID:          1,
		Title:       "Test Show",
		Year:        2020,
		Directory:   "/media/tv/test",
		ExternalIDs: map[string]string{"tmdb": "12345"},
	}
	cache.PutShow(show)

	got := cache.GetShow(1)
	if got == nil {
		t.Fatal("expected cached show")
	}
	if got.Title != "Test Show" {
		t.Errorf("expected title 'Test Show', got '%s'", got.Title)
	}
	if got.Year != 2020 {
		t.Errorf("expected year 2020, got %d", got.Year)
	}
	if got.ExternalIDs["tmdb"] != "12345" {
		t.Errorf("expected tmdb ID '12345', got '%s'", got.ExternalIDs["tmdb"])
	}

	// Test Invalidate
	cache.InvalidateShow(1)
	if got := cache.GetShow(1); got != nil {
		t.Error("expected nil after invalidation")
	}

	// Test nil Put (should not panic)
	cache.PutShow(nil)
}

func TestEntityCache_Seasons(t *testing.T) {
	cache := NewEntityCache(100)

	// Test Get on empty cache
	if got := cache.GetSeason(1); got != nil {
		t.Error("expected nil for missing season")
	}

	// Test Put and Get
	season := &CachedTVSeason{
		ID:           1,
		ShowID:       10,
		SeasonNumber: 1,
	}
	cache.PutSeason(season)

	got := cache.GetSeason(1)
	if got == nil {
		t.Fatal("expected cached season")
	}
	if got.ShowID != 10 {
		t.Errorf("expected showID 10, got %d", got.ShowID)
	}
	if got.SeasonNumber != 1 {
		t.Errorf("expected season number 1, got %d", got.SeasonNumber)
	}

	// Test nil Put (should not panic)
	cache.PutSeason(nil)
}

func TestEntityCache_Albums(t *testing.T) {
	cache := NewEntityCache(100)

	// Test Get on empty cache
	if got := cache.GetAlbum(1); got != nil {
		t.Error("expected nil for missing album")
	}

	// Test Put and Get
	album := &CachedAlbum{
		ID:          1,
		Title:       "Test Album",
		AlbumArtist: "Test Artist",
		Directory:   "/media/music/artist/album",
		ExternalIDs: map[string]string{"musicbrainz": "abc-123"},
	}
	cache.PutAlbum(album)

	got := cache.GetAlbum(1)
	if got == nil {
		t.Fatal("expected cached album")
	}
	if got.Title != "Test Album" {
		t.Errorf("expected title 'Test Album', got '%s'", got.Title)
	}
	if got.AlbumArtist != "Test Artist" {
		t.Errorf("expected artist 'Test Artist', got '%s'", got.AlbumArtist)
	}

	// Test Invalidate
	cache.InvalidateAlbum(1)
	if got := cache.GetAlbum(1); got != nil {
		t.Error("expected nil after invalidation")
	}

	// Test nil Put (should not panic)
	cache.PutAlbum(nil)
}

func TestEntityCache_Artists(t *testing.T) {
	cache := NewEntityCache(100)

	// Test Get on empty cache
	if got := cache.GetArtist(1); got != nil {
		t.Error("expected nil for missing artist")
	}

	// Test Put and Get
	artist := &CachedArtist{
		ID:          1,
		Name:        "Test Artist",
		ExternalIDs: map[string]string{"musicbrainz": "xyz-789"},
	}
	cache.PutArtist(artist)

	got := cache.GetArtist(1)
	if got == nil {
		t.Fatal("expected cached artist")
	}
	if got.Name != "Test Artist" {
		t.Errorf("expected name 'Test Artist', got '%s'", got.Name)
	}

	// Test Invalidate
	cache.InvalidateArtist(1)
	if got := cache.GetArtist(1); got != nil {
		t.Error("expected nil after invalidation")
	}

	// Test nil Put (should not panic)
	cache.PutArtist(nil)
}

func TestEntityCache_LRUEviction(t *testing.T) {
	// Create a small cache to test eviction
	cache := NewEntityCache(3)

	// Add 3 shows
	cache.PutShow(&CachedTVShow{ID: 1, Title: "Show 1"})
	cache.PutShow(&CachedTVShow{ID: 2, Title: "Show 2"})
	cache.PutShow(&CachedTVShow{ID: 3, Title: "Show 3"})

	stats := cache.Stats()
	if stats.Shows != 3 {
		t.Errorf("expected 3 shows, got %d", stats.Shows)
	}

	// Access show 1 to make it recently used
	cache.GetShow(1)

	// Add a 4th show, should evict show 2 (least recently used)
	cache.PutShow(&CachedTVShow{ID: 4, Title: "Show 4"})

	stats = cache.Stats()
	if stats.Shows != 3 {
		t.Errorf("expected 3 shows after eviction, got %d", stats.Shows)
	}

	// Show 1 should still exist (was accessed recently)
	if got := cache.GetShow(1); got == nil {
		t.Error("expected show 1 to exist (recently accessed)")
	}

	// Show 2 should be evicted
	if got := cache.GetShow(2); got != nil {
		t.Error("expected show 2 to be evicted")
	}

	// Show 3 and 4 should exist
	if got := cache.GetShow(3); got == nil {
		t.Error("expected show 3 to exist")
	}
	if got := cache.GetShow(4); got == nil {
		t.Error("expected show 4 to exist")
	}
}

func TestEntityCache_UpdateExisting(t *testing.T) {
	cache := NewEntityCache(100)

	// Add a show
	cache.PutShow(&CachedTVShow{ID: 1, Title: "Original Title"})

	// Update the same show
	cache.PutShow(&CachedTVShow{ID: 1, Title: "Updated Title"})

	got := cache.GetShow(1)
	if got == nil {
		t.Fatal("expected cached show")
	}
	if got.Title != "Updated Title" {
		t.Errorf("expected updated title, got '%s'", got.Title)
	}

	// Should not have duplicate entries
	stats := cache.Stats()
	if stats.Shows != 1 {
		t.Errorf("expected 1 show after update, got %d", stats.Shows)
	}
}

func TestLRUCache_Basic(t *testing.T) {
	cache := newLRUCache[string, int](3)

	// Test empty get
	if got := cache.Get("missing"); got != 0 {
		t.Errorf("expected 0 for missing key, got %d", got)
	}

	// Test put and get
	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)

	if got := cache.Get("a"); got != 1 {
		t.Errorf("expected 1, got %d", got)
	}
	if got := cache.Get("b"); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
	if got := cache.Get("c"); got != 3 {
		t.Errorf("expected 3, got %d", got)
	}

	if cache.Len() != 3 {
		t.Errorf("expected len 3, got %d", cache.Len())
	}
}

func TestLRUCache_Remove(t *testing.T) {
	cache := newLRUCache[string, int](10)

	cache.Put("a", 1)
	cache.Put("b", 2)

	cache.Remove("a")

	if got := cache.Get("a"); got != 0 {
		t.Error("expected 0 after removal")
	}
	if got := cache.Get("b"); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
	if cache.Len() != 1 {
		t.Errorf("expected len 1, got %d", cache.Len())
	}

	// Remove non-existent key (should not panic)
	cache.Remove("missing")
}
