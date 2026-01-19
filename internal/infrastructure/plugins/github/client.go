// Package github provides a client for fetching plugin releases from GitHub.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// GitHubRepo is the official ViewRA repository for plugins.
	// Third-party repos are not supported for security reasons.
	GitHubRepo = "mantonx/viewra"

	// API endpoints
	apiBaseURL    = "https://api.github.com"
	releasesPath  = "/repos/%s/releases"
	userAgent     = "ViewRA-Plugin-Marketplace/1.0"
	requestTimeout = 30 * time.Second

	// Rate limiting: GitHub allows 60 requests/hour unauthenticated
	requestsPerSecond = 0.5 // Conservative: 30 requests/minute max
	burstSize         = 5
)

// pluginTagPattern matches plugin release tags like "plugin-tmdb-v1.2.3"
var pluginTagPattern = regexp.MustCompile(`^plugin-([a-z0-9-]+)-v(\d+\.\d+\.\d+)$`)

// checksumPattern extracts SHA256 checksum from release body
var checksumPattern = regexp.MustCompile(`([a-f0-9]{64})\s+\S+\.tar\.gz`)

// Client fetches plugin releases from GitHub.
type Client struct {
	httpClient  *http.Client
	rateLimiter *rate.Limiter
	logger      *slog.Logger
}

// Release represents a plugin release from GitHub.
type Release struct {
	TagName     string    `json:"tag_name"`
	PluginID    string    `json:"-"` // Extracted from tag
	Version     string    `json:"-"` // Extracted from tag
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Checksum    string    `json:"-"` // Extracted from body
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

// Asset represents a downloadable file in a release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	DownloadCount      int64  `json:"download_count"`
}

// PluginInfo aggregates release info for a plugin (latest version).
type PluginInfo struct {
	ID            string
	LatestVersion string
	LatestRelease *Release
	AllVersions   []string
}

// NewClient creates a new GitHub client.
func NewClient(logger *slog.Logger) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
		rateLimiter: rate.NewLimiter(rate.Limit(requestsPerSecond), burstSize),
		logger:      logger,
	}
}

// ListPluginReleases fetches all plugin releases from the official repo.
// Returns a map of pluginID -> PluginInfo with the latest version info.
func (c *Client) ListPluginReleases(ctx context.Context) (map[string]*PluginInfo, error) {
	releases, err := c.fetchAllReleases(ctx)
	if err != nil {
		return nil, err
	}

	// Group releases by plugin ID
	plugins := make(map[string]*PluginInfo)
	for i := range releases {
		release := &releases[i]
		if !c.parsePluginTag(release) {
			continue // Not a plugin release
		}

		// Extract checksum from body
		release.Checksum = c.extractChecksum(release.Body)

		info, exists := plugins[release.PluginID]
		if !exists {
			info = &PluginInfo{
				ID:          release.PluginID,
				AllVersions: make([]string, 0),
			}
			plugins[release.PluginID] = info
		}

		info.AllVersions = append(info.AllVersions, release.Version)

		// Track latest by published date
		if info.LatestRelease == nil || release.PublishedAt.After(info.LatestRelease.PublishedAt) {
			info.LatestRelease = release
			info.LatestVersion = release.Version
		}
	}

	c.logger.Debug("fetched plugin releases", "count", len(plugins))
	return plugins, nil
}

// GetRelease fetches a specific plugin release by ID and version.
func (c *Client) GetRelease(ctx context.Context, pluginID, version string) (*Release, error) {
	tag := fmt.Sprintf("plugin-%s-v%s", pluginID, version)
	url := fmt.Sprintf("%s/repos/%s/releases/tags/%s", apiBaseURL, GitHubRepo, tag)

	release, err := c.fetchRelease(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release %s: %w", tag, err)
	}

	if !c.parsePluginTag(release) {
		return nil, fmt.Errorf("invalid plugin tag: %s", tag)
	}

	release.Checksum = c.extractChecksum(release.Body)
	return release, nil
}

// GetLatestRelease fetches the latest release for a plugin.
func (c *Client) GetLatestRelease(ctx context.Context, pluginID string) (*Release, error) {
	plugins, err := c.ListPluginReleases(ctx)
	if err != nil {
		return nil, err
	}

	info, exists := plugins[pluginID]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}

	return info.LatestRelease, nil
}

// GetCompatibleAsset returns the download URL for the current platform.
func (c *Client) GetCompatibleAsset(release *Release) (*Asset, error) {
	// Currently only linux-x64 is supported
	expectedName := fmt.Sprintf("plugin-%s-linux-x64.tar.gz", release.PluginID)

	for i := range release.Assets {
		if release.Assets[i].Name == expectedName {
			return &release.Assets[i], nil
		}
	}

	return nil, fmt.Errorf("no compatible asset found for %s (expected %s)", release.PluginID, expectedName)
}

// fetchAllReleases fetches all releases with pagination.
func (c *Client) fetchAllReleases(ctx context.Context) ([]Release, error) {
	var allReleases []Release
	page := 1
	perPage := 100

	for {
		url := fmt.Sprintf("%s"+releasesPath+"?page=%d&per_page=%d", apiBaseURL, GitHubRepo, page, perPage)

		var releases []Release
		if err := c.doRequest(ctx, url, &releases); err != nil {
			return nil, err
		}

		if len(releases) == 0 {
			break
		}

		allReleases = append(allReleases, releases...)

		if len(releases) < perPage {
			break // Last page
		}
		page++
	}

	return allReleases, nil
}

// fetchRelease fetches a single release.
func (c *Client) fetchRelease(ctx context.Context, url string) (*Release, error) {
	var release Release
	if err := c.doRequest(ctx, url, &release); err != nil {
		return nil, err
	}
	return &release, nil
}

// doRequest makes an HTTP request with rate limiting.
func (c *Client) doRequest(ctx context.Context, url string, result interface{}) error {
	// Wait for rate limiter
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found")
	}

	if resp.StatusCode == http.StatusForbidden {
		// Check for rate limit
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return fmt.Errorf("GitHub API rate limit exceeded")
		}
		return fmt.Errorf("forbidden")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

// parsePluginTag extracts plugin ID and version from a release tag.
// Returns false if the tag doesn't match the expected pattern.
func (c *Client) parsePluginTag(release *Release) bool {
	matches := pluginTagPattern.FindStringSubmatch(release.TagName)
	if len(matches) != 3 {
		return false
	}
	release.PluginID = matches[1]
	release.Version = matches[2]
	return true
}

// extractChecksum extracts SHA256 checksum from release body.
func (c *Client) extractChecksum(body string) string {
	matches := checksumPattern.FindStringSubmatch(body)
	if len(matches) >= 2 {
		return strings.ToLower(matches[1])
	}
	return ""
}

// CachedClient wraps Client with caching.
type CachedClient struct {
	client   *Client
	cache    map[string]*PluginInfo
	cacheMu  sync.RWMutex
	cacheAt  time.Time
	cacheTTL time.Duration
}

// NewCachedClient creates a client with caching.
func NewCachedClient(logger *slog.Logger, cacheTTL time.Duration) *CachedClient {
	return &CachedClient{
		client:   NewClient(logger),
		cache:    make(map[string]*PluginInfo),
		cacheTTL: cacheTTL,
	}
}

// ListPluginReleases returns cached results if valid, otherwise fetches fresh.
func (c *CachedClient) ListPluginReleases(ctx context.Context) (map[string]*PluginInfo, error) {
	c.cacheMu.RLock()
	if time.Since(c.cacheAt) < c.cacheTTL && len(c.cache) > 0 {
		result := c.cache
		c.cacheMu.RUnlock()
		return result, nil
	}
	c.cacheMu.RUnlock()

	// Fetch fresh
	plugins, err := c.client.ListPluginReleases(ctx)
	if err != nil {
		return nil, err
	}

	// Update cache
	c.cacheMu.Lock()
	c.cache = plugins
	c.cacheAt = time.Now()
	c.cacheMu.Unlock()

	return plugins, nil
}

// RefreshCache forces a cache refresh.
func (c *CachedClient) RefreshCache(ctx context.Context) error {
	plugins, err := c.client.ListPluginReleases(ctx)
	if err != nil {
		return err
	}

	c.cacheMu.Lock()
	c.cache = plugins
	c.cacheAt = time.Now()
	c.cacheMu.Unlock()

	return nil
}

// GetRelease delegates to the underlying client (not cached).
func (c *CachedClient) GetRelease(ctx context.Context, pluginID, version string) (*Release, error) {
	return c.client.GetRelease(ctx, pluginID, version)
}

// GetLatestRelease uses cache if available.
func (c *CachedClient) GetLatestRelease(ctx context.Context, pluginID string) (*Release, error) {
	plugins, err := c.ListPluginReleases(ctx)
	if err != nil {
		return nil, err
	}

	info, exists := plugins[pluginID]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}

	return info.LatestRelease, nil
}

// GetCompatibleAsset delegates to the underlying client.
func (c *CachedClient) GetCompatibleAsset(release *Release) (*Asset, error) {
	return c.client.GetCompatibleAsset(release)
}

// CacheAge returns how old the cache is.
func (c *CachedClient) CacheAge() time.Duration {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	return time.Since(c.cacheAt)
}
