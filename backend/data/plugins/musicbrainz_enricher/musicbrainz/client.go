package musicbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// BaseURL is the MusicBrainz API base URL
	BaseURL = "https://musicbrainz.org/ws/2"
	
	// CoverArtBaseURL is the Cover Art Archive base URL
	CoverArtBaseURL = "https://coverartarchive.org"
)

// Client handles communication with MusicBrainz and Cover Art Archive APIs
type Client struct {
	httpClient  *http.Client
	userAgent   string
	rateLimiter *RateLimiter
}

// RateLimiter implements rate limiting for API requests
type RateLimiter struct {
	lastRequest time.Time
	interval    time.Duration
}

// NewClient creates a new MusicBrainz API client
func NewClient(userAgent string, rateLimit float64) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: userAgent,
		rateLimiter: &RateLimiter{
			interval: time.Duration(1.0/rateLimit) * time.Second,
		},
	}
}

// Wait implements rate limiting by waiting if necessary
func (r *RateLimiter) Wait() {
	now := time.Now()
	if elapsed := now.Sub(r.lastRequest); elapsed < r.interval {
		time.Sleep(r.interval - elapsed)
	}
	r.lastRequest = time.Now()
}

// SearchRecordings searches for recordings in MusicBrainz
func (c *Client) SearchRecordings(ctx context.Context, title, artist, album string) ([]Recording, error) {
	// Rate limit the request
	c.rateLimiter.Wait()
	
	// Build search query
	query := c.buildSearchQuery(title, artist, album)
	if query == "" {
		return nil, fmt.Errorf("empty search query")
	}
	
	// Encode query and build URL
	encodedQuery := url.QueryEscape(query)
	searchURL := fmt.Sprintf("%s/recording?query=%s&fmt=json&inc=artists+releases+release-groups", 
		BaseURL, encodedQuery)
	
	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	
	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MusicBrainz API error: %d", resp.StatusCode)
	}
	
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	// Parse JSON response
	var searchResponse SearchResponse
	if err := json.Unmarshal(body, &searchResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return searchResponse.Recordings, nil
}

// GetCoverArt retrieves cover art information for a release
func (c *Client) GetCoverArt(ctx context.Context, releaseID string) (*CoverArtResponse, error) {
	// Rate limit the request
	c.rateLimiter.Wait()
	
	// Build URL
	artworkURL := fmt.Sprintf("%s/release/%s", CoverArtBaseURL, releaseID)
	
	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", artworkURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	
	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Check status code
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no cover art found for release %s", releaseID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Cover Art Archive error: %d", resp.StatusCode)
	}
	
	// Parse JSON response
	var coverArt CoverArtResponse
	if err := json.NewDecoder(resp.Body).Decode(&coverArt); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	return &coverArt, nil
}

// DownloadImage downloads an image from the given URL
func (c *Client) DownloadImage(ctx context.Context, imageURL string) ([]byte, error) {
	// Rate limit the request
	c.rateLimiter.Wait()
	
	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("User-Agent", c.userAgent)
	
	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image download error: %d", resp.StatusCode)
	}
	
	// Read image data
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}
	
	return imageData, nil
}

// buildSearchQuery constructs a MusicBrainz search query
func (c *Client) buildSearchQuery(title, artist, album string) string {
	var parts []string
	
	if title != "" {
		parts = append(parts, fmt.Sprintf("recording:\"%s\"", title))
	}
	if artist != "" {
		parts = append(parts, fmt.Sprintf("artist:\"%s\"", artist))
	}
	if album != "" {
		parts = append(parts, fmt.Sprintf("release:\"%s\"", album))
	}
	
	return strings.Join(parts, " AND ")
}

// FindBestArtwork finds the best artwork image based on preferences
func (c *Client) FindBestArtwork(coverArt *CoverArtResponse, quality string) *CoverArtImage {
	if len(coverArt.Images) == 0 {
		return nil
	}
	
	// First, try to find an image matching the preferred quality
	for _, image := range coverArt.Images {
		if quality == "front" && image.Front {
			return &image
		}
		if quality == "back" && image.Back {
			return &image
		}
		// Check if the quality matches any of the image types
		for _, imageType := range image.Types {
			if strings.EqualFold(imageType, quality) {
				return &image
			}
		}
	}
	
	// Fallback: return the first front image
	for _, image := range coverArt.Images {
		if image.Front {
			return &image
		}
	}
	
	// Last resort: return the first image
	return &coverArt.Images[0]
} 