package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// GetTrendingProviderInfo returns metadata about the TMDb trending provider.
func (p *TMDbPlugin) GetTrendingProviderInfo(ctx context.Context) (*sdk.TrendingProviderInfo, error) {
	return &sdk.TrendingProviderInfo{
		ID:          "tmdb",
		Name:        "TMDb Trending",
		Description: "Trending movies and TV shows from The Movie Database",
		Windows:     []string{"day", "week"},
		MediaTypes:  []string{"movie", "tv", "all"},
		UpdateFreq:  "daily",
	}, nil
}

// GetTrending returns currently trending items from TMDb.
func (p *TMDbPlugin) GetTrending(ctx context.Context, req *sdk.TrendingRequest) (*sdk.TrendingResponse, error) {
	p.mu.RLock()
	client := p.client
	p.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("TMDb client not initialized")
	}

	// Validate and default parameters
	mediaType := req.MediaType
	if mediaType == "" {
		mediaType = "all"
	}

	window := req.Window
	if window == "" {
		window = "week"
	}

	// Fetch from TMDb
	resp, err := client.GetTrending(ctx, mediaType, window)
	if err != nil {
		p.recordError()
		return nil, fmt.Errorf("fetch trending: %w", err)
	}

	// Convert to SDK format
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > len(resp.Results) {
		limit = len(resp.Results)
	}

	items := make([]sdk.TrendingItem, 0, limit)
	for i, r := range resp.Results {
		if i >= limit {
			break
		}

		item := convertTrendingResult(r)
		items = append(items, item)
	}

	return &sdk.TrendingResponse{
		Items:    items,
		Window:   window,
		Source:   "tmdb",
		CachedAt: time.Now().Unix(),
	}, nil
}

// convertTrendingResult converts a TMDb trending result to SDK format.
func convertTrendingResult(r TrendingResult) sdk.TrendingItem {
	// Determine title and year based on media type
	var title string
	var year int

	mediaType := r.MediaType
	if mediaType == "" {
		// If not specified in response (single type request), infer from fields
		if r.Title != "" {
			mediaType = "movie"
		} else if r.Name != "" {
			mediaType = "tv"
		}
	}

	switch mediaType {
	case "movie":
		title = r.Title
		if title == "" {
			title = r.OriginalTitle
		}
		year = extractYear(r.ReleaseDate)
	case "tv":
		title = r.Name
		if title == "" {
			title = r.OriginalName
		}
		year = extractYear(r.FirstAirDate)
	}

	return sdk.TrendingItem{
		ExternalID:   fmt.Sprintf("tmdb:%d", r.ID),
		MediaType:    mediaType,
		Title:        title,
		Year:         year,
		Popularity:   float32(r.Popularity),
		PosterPath:   ImageURL(r.PosterPath, "w500"),
		Overview:     r.Overview,
		LocalID:      nil,
		LocalMatched: false,
	}
}

// extractYear extracts year from a date string (YYYY-MM-DD format).
func extractYear(dateStr string) int {
	if len(dateStr) < 4 {
		return 0
	}
	year, _ := strconv.Atoi(dateStr[:4])
	return year
}

// handleTrending handles HTTP requests for trending data.
func (p *TMDbPlugin) handleTrending(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "GET" {
		return sdk.JSONError(http.StatusMethodNotAllowed, "method not allowed")
	}

	// Parse query parameters
	mediaType := req.Query["type"]
	if mediaType == "" {
		mediaType = "all"
	}

	window := req.Query["window"]
	if window == "" {
		window = "week"
	}

	limit := 20
	if limitStr := req.Query["limit"]; limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Get trending data
	resp, err := p.GetTrending(ctx, &sdk.TrendingRequest{
		MediaType: mediaType,
		Window:    window,
		Limit:     limit,
	})
	if err != nil {
		return sdk.JSONError(http.StatusInternalServerError, err.Error())
	}

	return sdk.JSONResponse(http.StatusOK, resp)
}

// handleTrendingInfo returns trending provider metadata.
func (p *TMDbPlugin) handleTrendingInfo(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "GET" {
		return sdk.JSONError(http.StatusMethodNotAllowed, "method not allowed")
	}

	info, err := p.GetTrendingProviderInfo(ctx)
	if err != nil {
		return sdk.JSONError(http.StatusInternalServerError, err.Error())
	}

	return sdk.JSONResponse(http.StatusOK, info)
}
