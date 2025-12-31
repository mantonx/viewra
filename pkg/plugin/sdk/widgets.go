// Widget definitions for ViewRA home screen plugin integration.
//
// Plugins can register widgets to appear on the home screen via their
// settings schema. Widgets are rendered by clients (web, iOS, Roku, etc.)
// using the data provided by the plugin.
//
// # Quick Start
//
// In your plugin's schema.go, add widgets:
//
//	func SettingsSchema() *sdk.Schema {
//	    return sdk.NewSchema("My Plugin Settings").
//	        Meta(sdk.PluginMeta{...}).
//	        Widgets([]sdk.Widget{
//	            {
//	                ID:              "my-recommendations",
//	                Type:            sdk.WidgetTypeMediaRow,
//	                Location:        sdk.LocationHomepageSections,
//	                ClientTypes:     []string{sdk.ClientTypeAll},
//	                Priority:        80,
//	                CacheTTLSeconds: 600,
//	                Config: map[string]any{
//	                    "endpoint": "/recommendations",
//	                    "title":    "Recommended for You",
//	                },
//	                SettingsKey: "enabled",
//	            },
//	        })
//	}
//
// Then implement the HTTP endpoint to return widget data:
//
//	func (p *Plugin) HandleHTTP(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
//	    if req.Path == "/recommendations" {
//	        items := p.getRecommendations(ctx, req.UserID)
//	        return jsonResponse(200, map[string]any{
//	            "title": "Recommended for You",
//	            "items": items,
//	        })
//	    }
//	    // ...
//	}
package sdk

// Widget locations define where a widget appears on the home screen.
const (
	// LocationHomepageTop is the top section of the homepage (search hero, featured content).
	LocationHomepageTop = "homepage-top"

	// LocationHomepageSections is the main content area with horizontal rows.
	LocationHomepageSections = "homepage-sections"
)

// Widget types define how a widget is rendered.
const (
	// WidgetTypeSearchHero is a search input with AI-generated suggestion chips.
	// Best for: semantic-search plugin on web/iOS/Android.
	// Data shape: {placeholder, suggestions[], search_url}
	WidgetTypeSearchHero = "search-hero"

	// WidgetTypeFeaturedRow is a horizontal row of featured items.
	// Best for: constrained clients (Roku) as an alternative to search-hero.
	// Data shape: {title, items[]}
	WidgetTypeFeaturedRow = "featured-row"

	// WidgetTypeContinueRow is a continue watching row with progress bars.
	// Best for: showing in-progress content.
	// Data shape: {title, items[] with progress}
	WidgetTypeContinueRow = "continue-row"

	// WidgetTypeMediaRow is a generic horizontal media row.
	// Best for: recommendations, similar items, trending.
	// Data shape: {title, subtitle?, items[], see_all_url?}
	WidgetTypeMediaRow = "media-row"
)

// Client types identify which clients should render a widget.
const (
	// ClientTypeAll means all clients should render this widget.
	ClientTypeAll = "all"

	// ClientTypeWeb is the web browser client.
	ClientTypeWeb = "web"

	// ClientTypeIOS is the iOS/tvOS client.
	ClientTypeIOS = "ios"

	// ClientTypeAndroid is the Android/Android TV client.
	ClientTypeAndroid = "android"

	// ClientTypeRoku is the Roku channel.
	ClientTypeRoku = "roku"

	// ClientTypeFireTV is the Amazon Fire TV client.
	ClientTypeFireTV = "firetv"

	// ClientTypeSmartTV is LG WebOS, Samsung Tizen, etc.
	ClientTypeSmartTV = "smarttv"
)

// Widget defines a UI section that a plugin provides for the home screen.
// Widgets are registered via the settings schema and rendered by clients.
type Widget struct {
	// ID is a unique identifier for this widget within the plugin.
	// Example: "rec-for-you", "search-hero"
	ID string `json:"id"`

	// Type determines how the widget is rendered.
	// Use WidgetTypeSearchHero, WidgetTypeMediaRow, etc.
	Type string `json:"type"`

	// Location specifies where on the home screen this widget appears.
	// Use LocationHomepageTop or LocationHomepageSections.
	Location string `json:"location"`

	// ClientTypes specifies which clients should render this widget.
	// Use ["all"] for all clients, or specific types like ["web", "ios"].
	// Clients filter widgets by their type and only render matching ones.
	ClientTypes []string `json:"client_types"`

	// Priority determines order within a location (higher = rendered first).
	// Use 100 for critical widgets, 50 for normal, 0 for fallback.
	Priority int `json:"priority"`

	// CacheTTLSeconds is how long clients should cache this widget's data.
	// Use 60 for frequently changing data, 3600 for stable data.
	CacheTTLSeconds int `json:"cache_ttl_seconds,omitempty"`

	// Config contains widget-specific configuration passed to the client.
	// Common fields:
	//   - endpoint: HTTP endpoint to fetch widget data (e.g., "/recommendations")
	//   - title: Default title for the widget
	//   - show_suggestions: For search-hero, whether to show suggestion chips
	Config map[string]any `json:"config,omitempty"`

	// RequiredCapability makes this widget only appear when a capability is available.
	// Example: "search_provider" - only show if semantic search is enabled.
	RequiredCapability string `json:"required_capability,omitempty"`

	// SettingsKey references a boolean setting that controls widget visibility.
	// When the setting is false, the widget is hidden.
	// Example: "enabled", "show_recommendations"
	SettingsKey string `json:"settings_key,omitempty"`
}

// WidgetDataRequest contains parameters for fetching widget data.
// This is passed to plugins when the home service requests widget data.
type WidgetDataRequest struct {
	// WidgetID is the widget being requested.
	WidgetID string

	// UserID is the requesting user (for personalization).
	UserID string

	// ClientType is the type of client making the request.
	ClientType string

	// ImageSize is the preferred image size ("small", "medium", "large").
	ImageSize string
}

// WidgetDataResponse is the response from a plugin for widget data.
type WidgetDataResponse struct {
	// Data is the widget-specific data to render.
	// Shape depends on widget type (see WidgetType* constants).
	Data map[string]any `json:"data"`

	// CacheTTLSeconds overrides the widget's default cache TTL.
	CacheTTLSeconds int `json:"cache_ttl_seconds,omitempty"`
}

// MediaItem represents a media item in widget data.
// Used in WidgetTypeMediaRow, WidgetTypeFeaturedRow, etc.
type MediaItem struct {
	// EntityType is the media type: "movie", "tv_show", "tv_episode", etc.
	EntityType string `json:"entity_type"`

	// EntityID is the database ID of the media item.
	EntityID int64 `json:"entity_id"`

	// Title is the display title.
	Title string `json:"title"`

	// Year is the release/air year.
	Year int `json:"year,omitempty"`

	// Poster is the URL to the poster image.
	Poster string `json:"poster,omitempty"`

	// Backdrop is the URL to the backdrop image.
	Backdrop string `json:"backdrop,omitempty"`

	// Reason is an optional recommendation reason.
	// Example: "Similar to Breaking Bad"
	Reason string `json:"reason,omitempty"`

	// Progress is optional playback progress (for continue watching).
	Progress *MediaProgress `json:"progress,omitempty"`

	// Rating is the user's rating for this item (if any).
	// Values: "up", "down", "favorite", or nil if not rated.
	Rating *string `json:"rating,omitempty"`
}

// MediaProgress represents playback progress for a media item.
type MediaProgress struct {
	// Percent is the progress percentage (0-100).
	Percent int `json:"percent"`

	// PositionSeconds is the playback position in seconds.
	PositionSeconds int `json:"position_seconds"`

	// DurationSeconds is the total duration in seconds.
	DurationSeconds int `json:"duration_seconds"`
}

// Suggestion represents a search suggestion in the search hero widget.
type Suggestion struct {
	// ID is a unique identifier for this suggestion.
	ID string `json:"id"`

	// Label is the display text.
	Label string `json:"label"`

	// Icon is the icon name to display.
	// Common icons: "fire", "cloud-rain", "sun", "moon", "snowflake", "heart"
	Icon string `json:"icon,omitempty"`

	// Description is an optional subtitle.
	Description string `json:"description,omitempty"`

	// Style affects rendering: "primary", "secondary", "accent".
	Style string `json:"style,omitempty"`

	// Action defines what happens when the suggestion is clicked.
	Action SuggestionAction `json:"action"`
}

// SuggestionAction defines the action for a search suggestion.
type SuggestionAction struct {
	// Type is the action type: "search", "filter", "navigate".
	Type string `json:"type"`

	// Query is the search query for Type="search".
	Query string `json:"query,omitempty"`

	// Filter is filter parameters for Type="filter".
	Filter map[string]string `json:"filter,omitempty"`

	// URL is the navigation URL for Type="navigate".
	URL string `json:"url,omitempty"`
}

// WidgetError represents an error from widget data fetching.
// Errors are logged but widgets with errors are silently omitted from the response.
type WidgetError struct {
	// WidgetID is the widget that failed.
	WidgetID string

	// PluginID is the plugin that provides the widget.
	PluginID string

	// Error is the error message.
	Error string

	// Timestamp is when the error occurred.
	Timestamp int64
}
