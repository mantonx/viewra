package internal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

const (
	// MusicBrainz API base URL
	mbBaseURL = "https://musicbrainz.org/ws/2"

	// Cover Art Archive base URL
	caaBaseURL = "https://coverartarchive.org"

	// Request timeouts
	requestTimeout = 30 * time.Second

	// Rate limiting: MusicBrainz requires ~1 request per second
	// We use 1100ms to be safe and respectful
	minRequestInterval = 1100 * time.Millisecond

	// Retry settings
	maxRetries        = 3
	initialRetryDelay = 2 * time.Second
	maxRetryDelay     = 30 * time.Second
	backoffMultiplier = 2.0

	// Cache key prefix
	cachePrefix = "mb:"
)

// Client handles MusicBrainz and Cover Art Archive API requests.
type Client struct {
	httpClient  *http.Client
	userAgent   string
	logger      *slog.Logger

	// Rate limiting
	lastRequest time.Time
	rateMu      sync.Mutex

	// Caching via host storage
	storage      pluginv1.HostStorageClient
	cacheTTLSecs int64

	// Cover art settings
	fetchCoverArt bool
	coverArtSize  string
}

// ClientConfig holds configuration for the MusicBrainz client.
type ClientConfig struct {
	UserAgent     string
	CacheTTLHours int
	Storage       pluginv1.HostStorageClient
	Logger        *slog.Logger
	FetchCoverArt bool
	CoverArtSize  string // "small", "large", or "original"
}

// NewClient creates a new MusicBrainz API client with caching and rate limiting.
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.UserAgent == "" {
		return nil, errors.New("user agent is required for MusicBrainz API")
	}

	ttlHours := cfg.CacheTTLHours
	if ttlHours == 0 {
		ttlHours = 168 // 1 week default
	}

	coverArtSize := cfg.CoverArtSize
	if coverArtSize == "" {
		coverArtSize = "large"
	}

	c := &Client{
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
		userAgent:     cfg.UserAgent,
		logger:        cfg.Logger,
		storage:       cfg.Storage,
		cacheTTLSecs:  int64(ttlHours) * 3600,
		fetchCoverArt: cfg.FetchCoverArt,
		coverArtSize:  coverArtSize,
	}

	return c, nil
}

// Close closes the client and its resources.
func (c *Client) Close() error {
	return nil
}

// SearchRecordings searches for recordings (tracks) by query.
func (c *Client) SearchRecordings(ctx context.Context, artist, track, album string) (*RecordingSearchResponse, error) {
	query := buildLuceneQuery(map[string]string{
		"artist":       artist,
		"recording":    track,
		"release":      album,
	})

	params := url.Values{}
	params.Set("query", query)
	params.Set("limit", "10")

	var result RecordingSearchResponse
	if err := c.get(ctx, "/recording", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetRecording fetches a recording by its MusicBrainz ID with related data.
func (c *Client) GetRecording(ctx context.Context, mbid string) (*Recording, error) {
	params := url.Values{}
	params.Set("inc", "artist-credits+releases+isrcs+tags")

	var result Recording
	if err := c.get(ctx, "/recording/"+mbid, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SearchReleases searches for releases (albums) by query.
func (c *Client) SearchReleases(ctx context.Context, artist, album string, year int) (*ReleaseSearchResponse, error) {
	queryParts := map[string]string{
		"artist":  artist,
		"release": album,
	}
	if year > 0 {
		queryParts["date"] = fmt.Sprintf("%d", year)
	}
	query := buildLuceneQuery(queryParts)

	params := url.Values{}
	params.Set("query", query)
	params.Set("limit", "10")

	var result ReleaseSearchResponse
	if err := c.get(ctx, "/release", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetRelease fetches a release by its MusicBrainz ID with related data.
func (c *Client) GetRelease(ctx context.Context, mbid string) (*Release, error) {
	params := url.Values{}
	params.Set("inc", "artist-credits+labels+recordings+release-groups+tags")

	var result Release
	if err := c.get(ctx, "/release/"+mbid, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SearchReleaseGroups searches for release groups (album concepts) by query.
func (c *Client) SearchReleaseGroups(ctx context.Context, artist, album string) (*ReleaseGroupSearchResponse, error) {
	query := buildLuceneQuery(map[string]string{
		"artist":       artist,
		"releasegroup": album,
	})

	params := url.Values{}
	params.Set("query", query)
	params.Set("limit", "10")

	var result ReleaseGroupSearchResponse
	if err := c.get(ctx, "/release-group", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetReleaseGroup fetches a release group by its MusicBrainz ID with releases.
func (c *Client) GetReleaseGroup(ctx context.Context, mbid string) (*ReleaseGroup, error) {
	params := url.Values{}
	params.Set("inc", "artist-credits+releases+tags")

	var result ReleaseGroup
	if err := c.get(ctx, "/release-group/"+mbid, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SearchArtists searches for artists by name.
func (c *Client) SearchArtists(ctx context.Context, name string) (*ArtistSearchResponse, error) {
	query := buildLuceneQuery(map[string]string{
		"artist": name,
	})

	params := url.Values{}
	params.Set("query", query)
	params.Set("limit", "10")

	var result ArtistSearchResponse
	if err := c.get(ctx, "/artist", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetArtist fetches an artist by their MusicBrainz ID with related data.
func (c *Client) GetArtist(ctx context.Context, mbid string) (*Artist, error) {
	params := url.Values{}
	params.Set("inc", "tags")

	var result Artist
	if err := c.get(ctx, "/artist/"+mbid, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetCoverArt fetches cover art for a release from the Cover Art Archive.
func (c *Client) GetCoverArt(ctx context.Context, releaseID string) (*CoverArtResponse, error) {
	if !c.fetchCoverArt {
		return nil, nil
	}

	// Cover Art Archive has its own rate limit, but we'll be conservative
	c.waitForRateLimit()

	cacheKey := cachePrefix + "caa:" + releaseID
	if cached, ok := c.getFromCache(ctx, cacheKey); ok {
		var result CoverArtResponse
		if err := json.Unmarshal(cached, &result); err == nil {
			return &result, nil
		}
	}

	reqURL := fmt.Sprintf("%s/release/%s", caaBaseURL, releaseID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cover art request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// No cover art available - cache this as empty
		c.setCache(ctx, cacheKey, []byte("{}"))
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover art API error (status %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	c.setCache(ctx, cacheKey, body)

	var result CoverArtResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetCoverArtURL returns the appropriate cover art URL based on configuration.
func (c *Client) GetCoverArtURL(img *CoverArtImage) string {
	switch c.coverArtSize {
	case "small":
		if img.Thumbnails.Small != "" {
			return img.Thumbnails.Small
		}
		if img.Thumbnails.XL250 != "" {
			return img.Thumbnails.XL250
		}
	case "large":
		if img.Thumbnails.Large != "" {
			return img.Thumbnails.Large
		}
		if img.Thumbnails.XL500 != "" {
			return img.Thumbnails.XL500
		}
	}
	// Fall back to original
	return img.Image
}

// buildLuceneQuery builds a MusicBrainz Lucene query from field-value pairs.
func buildLuceneQuery(parts map[string]string) string {
	var terms []string
	for field, value := range parts {
		if value == "" {
			continue
		}
		// Escape special Lucene characters and quote the value
		escaped := escapeLucene(value)
		terms = append(terms, fmt.Sprintf("%s:(%s)", field, escaped))
	}
	return strings.Join(terms, " AND ")
}

// escapeLucene escapes special characters in Lucene queries.
func escapeLucene(s string) string {
	// Characters that need escaping: + - && || ! ( ) { } [ ] ^ " ~ * ? : \ /
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`+`, `\+`,
		`-`, `\-`,
		`!`, `\!`,
		`(`, `\(`,
		`)`, `\)`,
		`{`, `\{`,
		`}`, `\}`,
		`[`, `\[`,
		`]`, `\]`,
		`^`, `\^`,
		`"`, `\"`,
		`~`, `\~`,
		`*`, `\*`,
		`?`, `\?`,
		`:`, `\:`,
		`/`, `\/`,
	)
	return replacer.Replace(s)
}

// cacheKey generates a cache key for a request.
func cacheKey(path string, params url.Values) string {
	h := sha256.New()
	h.Write([]byte(path))
	h.Write([]byte(params.Encode()))
	return cachePrefix + hex.EncodeToString(h.Sum(nil))
}

// getFromCache retrieves a cached response if available.
func (c *Client) getFromCache(ctx context.Context, key string) ([]byte, bool) {
	if c.storage == nil {
		return nil, false
	}

	resp, err := c.storage.KVGet(ctx, &pluginv1.KVKey{Key: key})
	if err != nil {
		c.logger.Debug("cache get error", "key", key, "error", err)
		return nil, false
	}

	if !resp.Exists {
		return nil, false
	}

	return resp.Value, true
}

// setCache stores a response in the cache with TTL.
func (c *Client) setCache(ctx context.Context, key string, response []byte) {
	if c.storage == nil {
		return
	}

	_, err := c.storage.KVSet(ctx, &pluginv1.KVEntry{
		Key:        key,
		Value:      response,
		TtlSeconds: c.cacheTTLSecs,
	})
	if err != nil {
		c.logger.Warn("failed to cache response", "key", key, "error", err)
	}
}

// waitForRateLimit blocks until we can make another request.
func (c *Client) waitForRateLimit() {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()

	elapsed := time.Since(c.lastRequest)
	if elapsed < minRequestInterval {
		time.Sleep(minRequestInterval - elapsed)
	}
	c.lastRequest = time.Now()
}

// isRetryableError returns true if the status code indicates a retryable error.
func isRetryableError(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout,     // 504
		http.StatusBadGateway:         // 502
		return true
	}
	return false
}

// get performs a GET request to the MusicBrainz API with caching, rate limiting, and retries.
func (c *Client) get(ctx context.Context, path string, params url.Values, result interface{}) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("fmt", "json")

	// Check cache first
	key := cacheKey(path, params)
	if cached, ok := c.getFromCache(ctx, key); ok {
		c.logger.Debug("cache hit", "path", path)
		return json.Unmarshal(cached, result)
	}

	reqURL := fmt.Sprintf("%s%s?%s", mbBaseURL, path, params.Encode())

	var lastErr error
	delay := initialRetryDelay

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			c.logger.Debug("retrying request", "attempt", attempt, "delay", delay, "path", path)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay = time.Duration(float64(delay) * backoffMultiplier)
			if delay > maxRetryDelay {
				delay = maxRetryDelay
			}
		}

		// Rate limit
		c.waitForRateLimit()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		// MusicBrainz requires a proper User-Agent header
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				if attempt < maxRetries {
					lastErr = err
					continue
				}
			}
			return fmt.Errorf("request failed: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			if attempt < maxRetries {
				lastErr = err
				continue
			}
			return fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			if isRetryableError(resp.StatusCode) && attempt < maxRetries {
				lastErr = fmt.Errorf("MusicBrainz API error (status %d): %s", resp.StatusCode, string(body))
				if resp.StatusCode == http.StatusTooManyRequests {
					delay = 5 * time.Second // Back off more on rate limit
				}
				continue
			}
			return fmt.Errorf("MusicBrainz API error (status %d): %s", resp.StatusCode, string(body))
		}

		// Cache the successful response
		c.setCache(ctx, key, body)

		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		return nil
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}
