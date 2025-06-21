package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mantonx/viewra/plugins/tmdb_enricher_v2/internal/config"
	"github.com/mantonx/viewra/plugins/tmdb_enricher_v2/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
)

// APIClient handles all TMDb API interactions
type APIClient struct {
	config      *config.Config
	logger      plugins.Logger
	httpClient  *http.Client
	lastAPICall *time.Time
}

// NewAPIClient creates a new TMDb API client
func NewAPIClient(cfg *config.Config, logger plugins.Logger) *APIClient {
	return &APIClient{
		config: cfg,
		logger: logger,
		httpClient: &http.Client{
			Timeout: cfg.API.GetRequestTimeout(),
		},
	}
}

// MakeRequest makes an HTTP request to the TMDb API with rate limiting
func (c *APIClient) MakeRequest(url string, result interface{}) error {
	// Ensure rate limiting
	if c.lastAPICall != nil {
		elapsed := time.Since(*c.lastAPICall)
		if elapsed < c.config.API.GetRequestDelay() {
			time.Sleep(c.config.API.GetRequestDelay() - elapsed)
		}
	}
	now := time.Now()
	c.lastAPICall = &now

	if c.config.Debug.LogAPIRequests {
		c.logger.Debug("making TMDb API request", "url", url)
	}

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication header for JWT tokens or add API key to query params for legacy keys
	if c.isJWTToken(c.config.API.Key) {
		req.Header.Set("Authorization", "Bearer "+c.config.API.Key)
	}
	// For legacy API keys, they should already be in the URL as query parameters

	// Set user agent
	req.Header.Set("User-Agent", c.config.API.UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("TMDb API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	return nil
}

// isJWTToken checks if the API key is a JWT token
func (c *APIClient) isJWTToken(apiKey string) bool {
	return len(apiKey) > 50 && apiKey[:3] == "eyJ" && len(apiKey) > 100
}

// GetMovieImages fetches images for a movie
func (c *APIClient) GetMovieImages(tmdbID int) (*types.ImagesResponse, error) {
	var url string
	if c.isJWTToken(c.config.API.Key) {
		url = fmt.Sprintf("https://api.themoviedb.org/3/movie/%d/images", tmdbID)
	} else {
		url = fmt.Sprintf("https://api.themoviedb.org/3/movie/%d/images?api_key=%s", tmdbID, c.config.API.Key)
	}

	var response types.ImagesResponse
	if err := c.MakeRequest(url, &response); err != nil {
		return nil, fmt.Errorf("failed to fetch movie images for ID %d: %w", tmdbID, err)
	}

	return &response, nil
}

// GetTVImages fetches images for a TV show
func (c *APIClient) GetTVImages(tmdbID int) (*types.ImagesResponse, error) {
	var url string
	if c.isJWTToken(c.config.API.Key) {
		url = fmt.Sprintf("https://api.themoviedb.org/3/tv/%d/images", tmdbID)
	} else {
		url = fmt.Sprintf("https://api.themoviedb.org/3/tv/%d/images?api_key=%s", tmdbID, c.config.API.Key)
	}

	var response types.ImagesResponse
	if err := c.MakeRequest(url, &response); err != nil {
		return nil, fmt.Errorf("failed to fetch TV images for ID %d: %w", tmdbID, err)
	}

	return &response, nil
}

// GetSeasonDetails fetches details for a TV season including images
func (c *APIClient) GetSeasonDetails(tmdbID, seasonNumber int) (*types.TVSeasonDetails, error) {
	var url string
	if c.isJWTToken(c.config.API.Key) {
		url = fmt.Sprintf("https://api.themoviedb.org/3/tv/%d/season/%d?append_to_response=images", tmdbID, seasonNumber)
	} else {
		url = fmt.Sprintf("https://api.themoviedb.org/3/tv/%d/season/%d?api_key=%s&append_to_response=images",
			tmdbID, seasonNumber, c.config.API.Key)
	}

	var response types.TVSeasonDetails
	if err := c.MakeRequest(url, &response); err != nil {
		return nil, fmt.Errorf("failed to fetch season %d details for TV ID %d: %w", seasonNumber, tmdbID, err)
	}

	return &response, nil
}

// GetEpisodeDetails fetches details for a specific episode
func (c *APIClient) GetEpisodeDetails(tmdbID, seasonNumber, episodeNumber int) (*types.TVEpisodeDetails, error) {
	var url string
	if c.isJWTToken(c.config.API.Key) {
		url = fmt.Sprintf("https://api.themoviedb.org/3/tv/%d/season/%d/episode/%d", tmdbID, seasonNumber, episodeNumber)
	} else {
		url = fmt.Sprintf("https://api.themoviedb.org/3/tv/%d/season/%d/episode/%d?api_key=%s",
			tmdbID, seasonNumber, episodeNumber, c.config.API.Key)
	}

	var response types.TVEpisodeDetails
	if err := c.MakeRequest(url, &response); err != nil {
		return nil, fmt.Errorf("failed to fetch episode %d of season %d for TV ID %d: %w",
			episodeNumber, seasonNumber, tmdbID, err)
	}

	return &response, nil
}
