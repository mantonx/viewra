package audiodb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
)

// Client handles communication with The AudioDB API
type Client struct {
	logger     hclog.Logger
	httpClient *http.Client
	baseURL    string
	apiKey     string
	userAgent  string
}

// NewClient creates a new AudioDB API client
func NewClient(logger hclog.Logger, apiKey, userAgent string) *Client {
	baseURL := "https://www.theaudiodb.com/api/v1/json"
	if apiKey != "" {
		baseURL = fmt.Sprintf("%s/%s", baseURL, apiKey)
	} else {
		baseURL = fmt.Sprintf("%s/1", baseURL)
	}

	return &Client{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:   baseURL,
		apiKey:    apiKey,
		userAgent: userAgent,
	}
}

// SearchArtist searches for an artist by name
func (c *Client) SearchArtist(artistName string) (*ArtistResponse, error) {
	searchURL := fmt.Sprintf("%s/search.php?s=%s", c.baseURL, url.QueryEscape(artistName))
	
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("User-Agent", c.userAgent)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var artistResponse ArtistResponse
	if err := json.NewDecoder(resp.Body).Decode(&artistResponse); err != nil {
		return nil, fmt.Errorf("failed to decode artist response: %w", err)
	}
	
	return &artistResponse, nil
}

// GetTracksByArtist retrieves all tracks for a given artist ID
func (c *Client) GetTracksByArtist(artistID string) (*TrackResponse, error) {
	tracksURL := fmt.Sprintf("%s/track.php?m=%s", c.baseURL, artistID)
	
	req, err := http.NewRequest("GET", tracksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create tracks request: %w", err)
	}
	
	req.Header.Set("User-Agent", c.userAgent)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tracks API request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("tracks API returned status %d", resp.StatusCode)
	}
	
	var trackResponse TrackResponse
	if err := json.NewDecoder(resp.Body).Decode(&trackResponse); err != nil {
		return nil, fmt.Errorf("failed to decode tracks response: %w", err)
	}
	
	return &trackResponse, nil
}

// GetAlbumInfo retrieves album information by album ID
func (c *Client) GetAlbumInfo(albumID string) (*AlbumResponse, error) {
	albumURL := fmt.Sprintf("%s/album.php?m=%s", c.baseURL, albumID)
	
	req, err := http.NewRequest("GET", albumURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create album request: %w", err)
	}
	
	req.Header.Set("User-Agent", c.userAgent)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("album API request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("album API returned status %d", resp.StatusCode)
	}
	
	var albumResponse AlbumResponse
	if err := json.NewDecoder(resp.Body).Decode(&albumResponse); err != nil {
		return nil, fmt.Errorf("failed to decode album response: %w", err)
	}
	
	return &albumResponse, nil
}

// HealthCheck performs a basic health check against the AudioDB API
func (c *Client) HealthCheck() error {
	resp, err := c.httpClient.Get("https://www.theaudiodb.com/api/v1/json/1/search.php?s=Queen")
	if err != nil {
		return fmt.Errorf("AudioDB API not reachable: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("AudioDB API returned status %d", resp.StatusCode)
	}
	
	return nil
}

// DownloadImage downloads an image from the given URL and returns the raw data
func (c *Client) DownloadImage(ctx context.Context, imageURL string) ([]byte, string, error) {
	if imageURL == "" {
		return nil, "", fmt.Errorf("image URL is empty")
	}

	// Create request with timeout context
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", c.userAgent)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("image download failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("image download error: HTTP %d", resp.StatusCode)
	}

	// Read image data
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}

	// Get content type from headers
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		// Try to detect from URL extension
		contentType = detectContentTypeFromURL(imageURL)
	}

	return imageData, contentType, nil
}

// detectContentTypeFromURL tries to detect content type from URL extension
func detectContentTypeFromURL(url string) string {
	url = strings.ToLower(url)
	if strings.Contains(url, ".jpg") || strings.Contains(url, ".jpeg") {
		return "image/jpeg"
	}
	if strings.Contains(url, ".png") {
		return "image/png"
	}
	if strings.Contains(url, ".gif") {
		return "image/gif"
	}
	if strings.Contains(url, ".webp") {
		return "image/webp"
	}
	// Default to JPEG
	return "image/jpeg"
} 