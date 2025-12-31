// Search provider plugin support for ViewRA.
//
// This file provides the SearchProvider interface for building plugins
// that provide search functionality. Multiple search providers can be
// registered, with the highest priority enabled one being the default.
//
// # Quick Start
//
// Implement the SearchProvider interface in your plugin:
//
//	type MySearchPlugin struct {
//	    sdk.Base
//	    // ...
//	}
//
//	func (p *MySearchPlugin) Search(ctx context.Context, req *sdk.SearchProviderRequest) (*sdk.SearchProviderResponse, error) {
//	    // Perform search
//	    return &sdk.SearchProviderResponse{
//	        Results: results,
//	        Total:   len(results),
//	    }, nil
//	}
//
//	func (p *MySearchPlugin) GetSuggestions(ctx context.Context, req *sdk.SuggestionRequest) (*sdk.SuggestionResponse, error) {
//	    // Return contextual suggestions
//	    return &sdk.SuggestionResponse{
//	        Suggestions: suggestions,
//	    }, nil
//	}
//
//	func (p *MySearchPlugin) GetSearchProviderInfo(ctx context.Context) (*sdk.SearchProviderInfo, error) {
//	    return &sdk.SearchProviderInfo{
//	        ID:           "my-search",
//	        Name:         "My Search",
//	        Description:  "AI-powered semantic search",
//	        Priority:     100,
//	        Capabilities: []string{"natural_language", "suggestions"},
//	    }, nil
//	}
//
// Then declare the capability in plugin.yml:
//
//	provides:
//	  - search_provider
package sdk

import (
	"context"
)

// SearchProvider is the interface that search provider plugins must implement.
// Plugins providing this capability offer search functionality to the home screen
// and other parts of the application.
type SearchProvider interface {
	mustEmbedBase()

	// Search performs a search with the given request.
	// This is called when users search via the search hero or global search.
	Search(ctx context.Context, req *SearchProviderRequest) (*SearchProviderResponse, error)

	// GetSuggestions returns contextual suggestions for the search hero.
	// Suggestions can be personalized based on user, time, weather, etc.
	GetSuggestions(ctx context.Context, req *SuggestionRequest) (*SuggestionResponse, error)

	// GetSearchProviderInfo returns metadata about this search provider.
	// Used for provider selection UI and priority ordering.
	GetSearchProviderInfo(ctx context.Context) (*SearchProviderInfo, error)
}

// SearchProviderRequest contains parameters for a search request.
type SearchProviderRequest struct {
	// Query is the search query text.
	Query string

	// EntityTypes filters results by media types.
	// Empty means all types. Example: ["movie", "tv_show"]
	EntityTypes []string

	// Limit is the maximum number of results to return.
	Limit int

	// UserID is the requesting user (for context enrichment).
	UserID string
}

// SearchProviderResponse contains search results.
type SearchProviderResponse struct {
	// Results is the list of search results.
	Results []SearchResult

	// Total is the total number of results (may be more than len(Results)).
	Total int

	// Provider is the ID of the provider that served this result.
	Provider string

	// Fallback indicates if this is a fallback result (basic text search).
	Fallback bool
}

// SearchResult represents a single search result.
type SearchResult struct {
	// EntityType is the media type: "movie", "tv_show", "tv_episode", etc.
	EntityType string

	// EntityID is the database ID of the media item.
	EntityID int64

	// Title is the display title.
	Title string

	// Year is the release/air year.
	Year int

	// Poster is the URL to the poster image.
	Poster string

	// Score is the relevance score (0.0-1.0).
	Score float32

	// Reason is an optional explanation for why this result matched.
	// Example: "Matches your search for 'sci-fi adventure'"
	Reason string
}

// SuggestionRequest contains parameters for getting search suggestions.
type SuggestionRequest struct {
	// UserID is the requesting user (for personalization).
	UserID string

	// Context describes where suggestions will be shown.
	// Values: "homepage" (search hero), "search_focus" (focused search input)
	Context string

	// Limit is the maximum number of suggestions to return.
	Limit int
}

// SuggestionResponse contains search suggestions.
type SuggestionResponse struct {
	// Suggestions is the list of suggestions to display.
	Suggestions []Suggestion
}

// SearchProviderInfo contains metadata about a search provider.
type SearchProviderInfo struct {
	// ID is a unique identifier for this provider.
	// Example: "semantic-search", "basic-search"
	ID string

	// Name is the human-readable name.
	// Example: "AI-Powered Search", "Basic Search"
	Name string

	// Description describes what this provider does.
	// Example: "Semantic search using AI embeddings"
	Description string

	// Icon is the icon name to display.
	// Common icons: "sparkles", "search", "brain"
	Icon string

	// Priority determines which provider is used by default.
	// Higher values = higher priority. Suggested ranges:
	//   100+ = AI-powered providers (semantic search)
	//   50   = Enhanced providers (fuzzy matching)
	//   10   = Basic providers (text search)
	Priority int

	// Capabilities lists what this provider supports.
	// Values:
	//   "natural_language" - Understands natural language queries
	//   "suggestions"      - Provides contextual suggestions
	//   "context_aware"    - Uses user context (time, weather, etc.)
	//   "fuzzy_matching"   - Handles typos and variations
	Capabilities []string
}
