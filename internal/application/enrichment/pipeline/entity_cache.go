package pipeline

import (
	"container/list"
	"sync"
)

// EntityCache provides LRU caching for parent entity data during enrichment.
// This eliminates redundant database lookups when processing multiple children
// of the same parent (e.g., 500 episodes of the same TV show).
//
// The cache stores lightweight entity info needed for building EnrichRequests:
// - TV Shows: title, year, directory, external IDs
// - TV Seasons: season number, show ID
// - Music Albums: title, artist, directory, external IDs
// - Music Artists: name, external IDs
type EntityCache struct {
	maxSize int

	mu       sync.RWMutex
	shows    *lruCache[int64, *CachedTVShow]
	seasons  *lruCache[int64, *CachedTVSeason]
	albums   *lruCache[int64, *CachedAlbum]
	artists  *lruCache[int64, *CachedArtist]
}

// CachedTVShow contains cached TV show data for episode enrichment.
type CachedTVShow struct {
	ID          int64
	Title       string
	Year        int
	Directory   string
	ExternalIDs map[string]string // provider -> externalID
}

// CachedTVSeason contains cached TV season data.
type CachedTVSeason struct {
	ID           int64
	ShowID       int64
	SeasonNumber int
}

// CachedAlbum contains cached album data for track enrichment.
type CachedAlbum struct {
	ID          int64
	Title       string
	AlbumArtist string
	Directory   string
	ExternalIDs map[string]string
}

// CachedArtist contains cached artist data.
type CachedArtist struct {
	ID          int64
	Name        string
	ExternalIDs map[string]string
}

// NewEntityCache creates a new entity cache with the specified max size per entity type.
// Recommended: 10,000 entries per type for large libraries.
func NewEntityCache(maxSize int) *EntityCache {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &EntityCache{
		maxSize: maxSize,
		shows:   newLRUCache[int64, *CachedTVShow](maxSize),
		seasons: newLRUCache[int64, *CachedTVSeason](maxSize),
		albums:  newLRUCache[int64, *CachedAlbum](maxSize),
		artists: newLRUCache[int64, *CachedArtist](maxSize),
	}
}

// GetShow returns a cached TV show, or nil if not cached.
func (c *EntityCache) GetShow(id int64) *CachedTVShow {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.shows.Get(id)
}

// PutShow caches a TV show.
func (c *EntityCache) PutShow(show *CachedTVShow) {
	if show == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shows.Put(show.ID, show)
}

// GetSeason returns a cached TV season, or nil if not cached.
func (c *EntityCache) GetSeason(id int64) *CachedTVSeason {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.seasons.Get(id)
}

// PutSeason caches a TV season.
func (c *EntityCache) PutSeason(season *CachedTVSeason) {
	if season == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seasons.Put(season.ID, season)
}

// GetAlbum returns a cached album, or nil if not cached.
func (c *EntityCache) GetAlbum(id int64) *CachedAlbum {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.albums.Get(id)
}

// PutAlbum caches an album.
func (c *EntityCache) PutAlbum(album *CachedAlbum) {
	if album == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.albums.Put(album.ID, album)
}

// GetArtist returns a cached artist, or nil if not cached.
func (c *EntityCache) GetArtist(id int64) *CachedArtist {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.artists.Get(id)
}

// PutArtist caches an artist.
func (c *EntityCache) PutArtist(artist *CachedArtist) {
	if artist == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.artists.Put(artist.ID, artist)
}

// InvalidateShow removes a TV show from the cache.
// Call this when show data is updated.
func (c *EntityCache) InvalidateShow(id int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shows.Remove(id)
}

// InvalidateAlbum removes an album from the cache.
func (c *EntityCache) InvalidateAlbum(id int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.albums.Remove(id)
}

// InvalidateArtist removes an artist from the cache.
func (c *EntityCache) InvalidateArtist(id int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.artists.Remove(id)
}

// Stats returns cache statistics for monitoring.
func (c *EntityCache) Stats() EntityCacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return EntityCacheStats{
		Shows:   c.shows.Len(),
		Seasons: c.seasons.Len(),
		Albums:  c.albums.Len(),
		Artists: c.artists.Len(),
		MaxSize: c.maxSize,
	}
}

// EntityCacheStats contains cache statistics.
type EntityCacheStats struct {
	Shows   int
	Seasons int
	Albums  int
	Artists int
	MaxSize int
}

// lruCache is a simple LRU cache implementation.
type lruCache[K comparable, V any] struct {
	maxSize int
	items   map[K]*list.Element
	order   *list.List // front = most recently used
}

type lruEntry[K comparable, V any] struct {
	key   K
	value V
}

func newLRUCache[K comparable, V any](maxSize int) *lruCache[K, V] {
	return &lruCache[K, V]{
		maxSize: maxSize,
		items:   make(map[K]*list.Element),
		order:   list.New(),
	}
}

func (c *lruCache[K, V]) Get(key K) V {
	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		return elem.Value.(*lruEntry[K, V]).value
	}
	var zero V
	return zero
}

func (c *lruCache[K, V]) Put(key K, value V) {
	if elem, ok := c.items[key]; ok {
		// Update existing entry
		c.order.MoveToFront(elem)
		elem.Value.(*lruEntry[K, V]).value = value
		return
	}

	// Add new entry
	entry := &lruEntry[K, V]{key: key, value: value}
	elem := c.order.PushFront(entry)
	c.items[key] = elem

	// Evict oldest if over capacity
	for c.order.Len() > c.maxSize {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.items, oldest.Value.(*lruEntry[K, V]).key)
		}
	}
}

func (c *lruCache[K, V]) Remove(key K) {
	if elem, ok := c.items[key]; ok {
		c.order.Remove(elem)
		delete(c.items, key)
	}
}

func (c *lruCache[K, V]) Len() int {
	return c.order.Len()
}
