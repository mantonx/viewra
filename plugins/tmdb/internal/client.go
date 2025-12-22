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
	"sync"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

const (
	baseURL        = "https://api.themoviedb.org/3"
	imageBaseURL   = "https://image.tmdb.org/t/p"
	requestTimeout = 30 * time.Second

	// Rate limiting: TMDb allows ~40 requests per 10 seconds
	// We use 250ms between requests (4 req/sec = 40 req/10sec)
	minRequestInterval = 250 * time.Millisecond

	// Retry settings
	maxRetries        = 3
	initialRetryDelay = 1 * time.Second
	maxRetryDelay     = 30 * time.Second
	backoffMultiplier = 2.0

	// Cache key prefix
	cachePrefix = "cache:"
)

// Client handles TMDb API requests with rate limiting, caching, and retries.
type Client struct {
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger

	// Rate limiting
	lastRequest time.Time
	rateMu      sync.Mutex

	// Caching via host storage
	storage      pluginv1.HostStorageClient
	cacheTTLSecs int64
}

// ClientConfig holds configuration for the TMDb client.
type ClientConfig struct {
	APIKey        string
	CacheTTLHours int
	Storage       pluginv1.HostStorageClient
	Logger        *slog.Logger
}

// NewClient creates a new TMDb API client with caching and rate limiting.
func NewClient(cfg ClientConfig) (*Client, error) {
	ttlHours := cfg.CacheTTLHours
	if ttlHours == 0 {
		ttlHours = 24
	}

	c := &Client{
		apiKey: cfg.APIKey,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
		logger:       cfg.Logger,
		storage:      cfg.Storage,
		cacheTTLSecs: int64(ttlHours) * 3600,
	}

	return c, nil
}

// Close closes the client and its resources.
func (c *Client) Close() error {
	// No resources to close - host manages storage
	return nil
}

// SearchMovies searches for movies by title and optional year.
func (c *Client) SearchMovies(ctx context.Context, title string, year int) (*MovieSearchResponse, error) {
	params := url.Values{}
	params.Set("query", title)
	if year > 0 {
		params.Set("year", fmt.Sprintf("%d", year))
	}

	var result MovieSearchResponse
	if err := c.get(ctx, "/search/movie", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetMovieDetails fetches detailed movie information.
func (c *Client) GetMovieDetails(ctx context.Context, tmdbID int) (*MovieDetails, error) {
	params := url.Values{}
	params.Set("append_to_response", "credits,images,keywords")

	var result MovieDetails
	if err := c.get(ctx, fmt.Sprintf("/movie/%d", tmdbID), params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SearchTV searches for TV shows by title and optional year.
func (c *Client) SearchTV(ctx context.Context, title string, year int) (*TVSearchResponse, error) {
	params := url.Values{}
	params.Set("query", title)
	if year > 0 {
		params.Set("first_air_date_year", fmt.Sprintf("%d", year))
	}

	var result TVSearchResponse
	if err := c.get(ctx, "/search/tv", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTVDetails fetches detailed TV show information.
func (c *Client) GetTVDetails(ctx context.Context, tmdbID int) (*TVDetails, error) {
	params := url.Values{}
	params.Set("append_to_response", "credits,images,external_ids,keywords")

	var result TVDetails
	if err := c.get(ctx, fmt.Sprintf("/tv/%d", tmdbID), params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// FindByIMDbID looks up a movie or TV show by IMDb ID.
func (c *Client) FindByIMDbID(ctx context.Context, imdbID string) (*FindByExternalIDResponse, error) {
	params := url.Values{}
	params.Set("external_source", "imdb_id")

	var result FindByExternalIDResponse
	if err := c.get(ctx, fmt.Sprintf("/find/%s", imdbID), params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ImageURL returns the full URL for an image path.
func ImageURL(path string, size string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s%s", imageBaseURL, size, path)
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

// isRetryableError returns true if the error is transient and worth retrying.
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

// isRetryableNetworkError returns true for transient network errors.
func isRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled)
}

// isJWTToken checks if the API key is a JWT bearer token (v4 auth).
func (c *Client) isJWTToken() bool {
	return len(c.apiKey) > 100 && len(c.apiKey) >= 3 && c.apiKey[:3] == "eyJ"
}

// get performs a GET request to the TMDb API with caching, rate limiting, and retries.
func (c *Client) get(ctx context.Context, path string, params url.Values, result interface{}) error {
	if params == nil {
		params = url.Values{}
	}

	// Check cache first
	key := cacheKey(path, params)
	if cached, ok := c.getFromCache(ctx, key); ok {
		c.logger.Debug("cache hit", "path", path)
		return json.Unmarshal(cached, result)
	}

	// For legacy API keys, add to query params
	if !c.isJWTToken() {
		params.Set("api_key", c.apiKey)
	}

	reqURL := fmt.Sprintf("%s%s?%s", baseURL, path, params.Encode())

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

		// For JWT tokens (v4 auth), use Bearer authorization header
		if c.isJWTToken() {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if isRetryableNetworkError(err) && attempt < maxRetries {
				lastErr = err
				continue
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
				lastErr = fmt.Errorf("TMDb API error (status %d): %s", resp.StatusCode, string(body))
				// For rate limiting, respect Retry-After header if present
				if resp.StatusCode == http.StatusTooManyRequests {
					delay = 10 * time.Second // TMDb typically rate limits for ~10 seconds
				}
				continue
			}
			return fmt.Errorf("TMDb API error (status %d): %s", resp.StatusCode, string(body))
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
