package home

import "time"

// Section represents a single section in the home screen.
type Section struct {
	// ID is a unique identifier for this section.
	ID string `json:"id"`

	// Type determines how the section is rendered.
	// Values: "search-hero", "featured-row", "continue-row", "media-row"
	Type string `json:"type"`

	// Location specifies where the section appears.
	// Values: "homepage-top", "homepage-sections"
	Location string `json:"location"`

	// ClientTypes specifies which clients should render this section.
	// Values: "all", "web", "ios", "android", "roku", "firetv", "smarttv"
	ClientTypes []string `json:"client_types"`

	// Priority is the widget's base priority (higher = earlier).
	Priority int `json:"priority"`

	// Position is the order in which sections appear (after user prefs applied).
	Position int `json:"position"`

	// Hidden indicates if the user has hidden this section.
	Hidden bool `json:"hidden"`

	// CacheTTLSeconds is how long clients should cache this data.
	CacheTTLSeconds int `json:"cache_ttl_seconds,omitempty"`

	// Data contains the section-specific data.
	// Shape depends on Type.
	Data map[string]any `json:"data,omitempty"`

	// DataURL is the endpoint to fetch section data (for deferred loading).
	DataURL string `json:"data_url,omitempty"`

	// PluginID is the plugin that provides this section.
	PluginID string `json:"plugin_id,omitempty"`
}

// HomeRequest contains parameters for getting the home screen.
type HomeRequest struct {
	// UserID is the requesting user (for personalization).
	UserID string

	// ClientType is the type of client making the request.
	// Values: "web", "ios", "android", "roku", "firetv", "smarttv"
	ClientType string

	// Inline indicates whether to include data inline or return URLs only.
	Inline bool

	// Sections is an optional list of section IDs to include.
	// Empty means all sections.
	Sections []string

	// ImageSize is the preferred image size.
	// Values: "small", "medium", "large"
	ImageSize string
}

// HomeResponse is the response from the home service.
type HomeResponse struct {
	// Sections is the list of home screen sections.
	Sections []*Section `json:"sections"`

	// Preferences contains user preference settings.
	Preferences *PreferencesInfo `json:"preferences"`

	// Meta contains contextual metadata.
	Meta *HomeMeta `json:"meta"`
}

// PreferencesInfo describes what customization options are available.
type PreferencesInfo struct {
	// CanReorder indicates if the user can reorder sections.
	CanReorder bool `json:"can_reorder"`

	// CanHide indicates if the user can hide sections.
	CanHide bool `json:"can_hide"`

	// UpdateURL is the endpoint to update preferences.
	UpdateURL string `json:"update_url"`
}

// HomeMeta contains contextual metadata about the home response.
type HomeMeta struct {
	// GeneratedAt is when the response was generated.
	GeneratedAt time.Time `json:"generated_at"`

	// UserContext contains information about the user's context.
	UserContext *UserContext `json:"user_context,omitempty"`

	// Hero contains data for the hero backdrop section.
	Hero *HeroData `json:"hero,omitempty"`
}

// HeroData contains information for the hero backdrop section.
type HeroData struct {
	// BackdropMediaID is the media ID to fetch backdrop from.
	BackdropMediaID int64 `json:"backdrop_media_id,omitempty"`

	// BackdropMediaType is the type of media (movie or tv_show).
	BackdropMediaType string `json:"backdrop_media_type,omitempty"`

	// Greeting is a time-based greeting (Good morning, Good afternoon, etc.).
	Greeting string `json:"greeting,omitempty"`

	// DateText is the formatted current date (Saturday, January 4).
	DateText string `json:"date_text,omitempty"`
}

// UserContext describes the user's current context for personalization.
type UserContext struct {
	// HasWatchHistory indicates if the user has watch history.
	HasWatchHistory bool `json:"has_watch_history"`

	// HasRatings indicates if the user has rated any content.
	HasRatings bool `json:"has_ratings"`

	// TimeOfDay is the current time of day.
	// Values: "morning", "afternoon", "evening", "night"
	TimeOfDay string `json:"time_of_day,omitempty"`

	// Season is the current season.
	// Values: "spring", "summer", "fall", "winter"
	Season string `json:"season,omitempty"`
}

// WidgetPreference represents a user's preference for a widget.
type WidgetPreference struct {
	// ID is the database ID.
	ID int64

	// UserID is the user who owns this preference.
	UserID string

	// WidgetID is the widget this preference applies to.
	WidgetID string

	// Location is where the widget appears.
	Location string

	// Position is the user-specified order.
	Position int

	// Hidden indicates if the user has hidden this widget.
	Hidden bool

	// CreatedAt is when the preference was created.
	CreatedAt time.Time

	// UpdatedAt is when the preference was last updated.
	UpdatedAt time.Time
}

// PreferencesUpdateRequest is the request to update user preferences.
type PreferencesUpdateRequest struct {
	// Sections contains the updated section preferences.
	Sections []SectionPreference `json:"sections"`
}

// SectionPreference is a single section preference update.
type SectionPreference struct {
	// ID is the section ID.
	ID string `json:"id"`

	// Position is the new position.
	Position int `json:"position"`

	// Hidden indicates if the section should be hidden.
	Hidden bool `json:"hidden"`
}

// MediaItem represents a media item in home screen sections.
type MediaItem struct {
	// EntityType is the media type.
	// Values: "movie", "tv_show", "tv_episode", "album", "track"
	EntityType string `json:"entity_type"`

	// EntityID is the database ID.
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
	Reason string `json:"reason,omitempty"`

	// Progress is optional playback progress.
	Progress *MediaProgress `json:"progress,omitempty"`

	// Rating is the user's rating for this item.
	Rating *string `json:"rating,omitempty"`

	// CreatedAt is when the item was added (for sorting, not serialized).
	CreatedAt time.Time `json:"-"`
}

// MediaProgress represents playback progress for a media item.
type MediaProgress struct {
	// Percent is the progress percentage (0-100).
	Percent int `json:"percent"`

	// PositionSeconds is the playback position in seconds.
	PositionSeconds int `json:"position_seconds"`

	// DurationSeconds is the total duration in seconds.
	DurationSeconds int `json:"duration_seconds"`

	// RemainingText is a human-readable remaining time (e.g., "1h 23m left").
	RemainingText string `json:"remaining_text,omitempty"`
}

// EpisodeContext provides context about the current episode for TV shows.
type EpisodeContext struct {
	// Season is the season number.
	Season int `json:"season"`

	// Episode is the episode number within the season.
	Episode int `json:"episode"`

	// EpisodeTitle is the title of the episode.
	EpisodeTitle string `json:"episode_title,omitempty"`

	// ShowTitle is the title of the TV show.
	ShowTitle string `json:"show_title"`

	// EpisodeMediaID is the media ID of the episode (for direct playback).
	EpisodeMediaID int64 `json:"episode_media_id"`
}

// ContinueWatchingItem represents an item in the continue watching row.
// This provides a richer structure than the generic MediaItem.
type ContinueWatchingItem struct {
	// EntityType is the media type: "movie" or "tv_show".
	EntityType string `json:"entity_type"`

	// EntityID is the database ID (movie ID or show ID).
	EntityID int64 `json:"entity_id"`

	// Title is the display title.
	Title string `json:"title"`

	// Year is the release/air year.
	Year int `json:"year,omitempty"`

	// BackdropURL is the URL to the backdrop/fanart image (16:9).
	BackdropURL string `json:"backdrop_url,omitempty"`

	// Progress contains playback progress information.
	Progress *MediaProgress `json:"progress"`

	// EpisodeContext provides episode details for TV shows.
	// Nil for movies.
	EpisodeContext *EpisodeContext `json:"episode_context,omitempty"`

	// LastWatchedAt is when this item was last watched.
	LastWatchedAt time.Time `json:"last_watched_at"`
}
