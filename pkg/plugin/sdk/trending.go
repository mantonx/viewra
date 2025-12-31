// Trending provider plugin support for ViewRA.
//
// This file provides the TrendingProvider interface for building plugins
// that provide trending/popular content data. Trending data is matched
// against the user's library to show relevant trending items.
//
// # Quick Start
//
// Implement the TrendingProvider interface in your enricher plugin:
//
//	type TMDbPlugin struct {
//	    sdk.Base
//	    // ...
//	}
//
//	func (p *TMDbPlugin) GetTrending(ctx context.Context, req *sdk.TrendingRequest) (*sdk.TrendingResponse, error) {
//	    // Fetch trending from TMDb API
//	    return &sdk.TrendingResponse{
//	        Items:  items,
//	        Window: req.Window,
//	        Source: "tmdb",
//	    }, nil
//	}
//
//	func (p *TMDbPlugin) GetTrendingProviderInfo(ctx context.Context) (*sdk.TrendingProviderInfo, error) {
//	    return &sdk.TrendingProviderInfo{
//	        ID:          "tmdb",
//	        Name:        "TMDb Trending",
//	        Description: "Trending movies and TV shows from The Movie Database",
//	        Windows:     []string{"day", "week"},
//	        MediaTypes:  []string{"movie", "tv", "all"},
//	        UpdateFreq:  "daily",
//	    }, nil
//	}
//
// Then declare the capability in plugin.yml:
//
//	provides:
//	  - enricher
//	  - trending
package sdk

import (
	"context"
)

// TrendingProvider is the interface that trending provider plugins must implement.
// Plugins providing this capability offer trending/popular content data
// from external sources (TMDb, Trakt, etc.).
type TrendingProvider interface {
	mustEmbedBase()

	// GetTrending returns currently trending items from this provider.
	// The response contains external IDs that the core service matches
	// against the user's local library.
	GetTrending(ctx context.Context, req *TrendingRequest) (*TrendingResponse, error)

	// GetTrendingProviderInfo returns metadata about this trending source.
	// Used for provider selection and capability discovery.
	GetTrendingProviderInfo(ctx context.Context) (*TrendingProviderInfo, error)
}

// TrendingRequest contains parameters for a trending request.
type TrendingRequest struct {
	// MediaType filters by media type.
	// Values: "movie", "tv", "all" (default: "all")
	MediaType string

	// Window is the time window for trending.
	// Values: "day", "week" (default: "week")
	Window string

	// Limit is the maximum number of items to return.
	// Default: 20
	Limit int

	// Region is an optional ISO 3166-1 country code for regional trending.
	// Example: "US", "GB", "DE"
	Region string
}

// TrendingResponse contains trending items from a provider.
type TrendingResponse struct {
	// Items is the list of trending items.
	Items []TrendingItem

	// Window is the time window used.
	Window string

	// Source is the provider that served this data.
	// Example: "tmdb", "trakt"
	Source string

	// CachedAt is the Unix timestamp when this data was fetched.
	// Used for cache management.
	CachedAt int64
}

// TrendingItem represents a single trending item from an external source.
type TrendingItem struct {
	// ExternalID is the external identifier in format "source:id".
	// Example: "tmdb:12345", "imdb:tt1234567"
	ExternalID string

	// MediaType is "movie" or "tv".
	MediaType string

	// Title is the display title.
	Title string

	// Year is the release/air year.
	Year int

	// Popularity is the provider-specific popularity score.
	// Higher is more popular. Scale varies by provider.
	Popularity float32

	// PosterPath is the external URL to the poster image.
	// Example: "https://image.tmdb.org/t/p/w500/abc123.jpg"
	PosterPath string

	// Overview is a brief description/plot summary.
	Overview string

	// LocalID is the matched local library ID (filled in by the core service).
	// Nil if not matched to local library.
	LocalID *int64

	// LocalMatched indicates whether this item was matched to the local library.
	LocalMatched bool
}

// TrendingProviderInfo contains metadata about a trending provider.
type TrendingProviderInfo struct {
	// ID is a unique identifier for this provider.
	// Example: "tmdb", "trakt"
	ID string

	// Name is the human-readable name.
	// Example: "TMDb Trending", "Trakt Popular"
	Name string

	// Description describes what this provider offers.
	// Example: "Trending movies and TV shows from The Movie Database"
	Description string

	// Windows lists supported time windows.
	// Example: ["day", "week"]
	Windows []string

	// MediaTypes lists supported media types.
	// Example: ["movie", "tv", "all"]
	MediaTypes []string

	// UpdateFreq describes how often data is updated.
	// Values: "hourly", "daily", "weekly"
	UpdateFreq string
}

// TrendingResult is the processed trending data with local library matches.
// Returned by the core TrendingService after matching.
type TrendingResult struct {
	// Items contains trending items matched to the local library.
	Items []TrendingItem

	// Source is the provider that served the original data.
	Source string

	// Window is the time window used.
	Window string

	// TotalMatched is the number of items matched to local library.
	TotalMatched int

	// TotalTrending is the total trending items from the provider.
	TotalTrending int
}
