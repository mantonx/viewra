// Package manifest provides parsing and validation for plugin.yml manifest files.
package manifest

import "strings"

// DisplayCategory represents a plugin category for UI grouping.
type DisplayCategory struct {
	ID       string // Identifier used in manifests and filtering (e.g., "search")
	Label    string // Display label (e.g., "Search")
	Priority int    // Sort order (lower = first)
}

// DisplayCategories defines the allowed display categories for plugins.
// These determine how plugins are grouped in the UI.
// Plugins specify one of these IDs in their manifest's display_category field.
var DisplayCategories = []DisplayCategory{
	{ID: "search", Label: "Search", Priority: 1},
	{ID: "recommendations", Label: "Recommendations", Priority: 2},
	{ID: "enrichers", Label: "Enrichers", Priority: 3},
	{ID: "providers", Label: "AI Providers", Priority: 4},
	{ID: "media_management", Label: "Media Management", Priority: 5}, // *arr apps, Jellyseerr, Overseerr, Ombi
	{ID: "auth", Label: "Authentication", Priority: 6},               // Auth providers (Authelia, Authentik, LDAP, OIDC)
	{ID: "notifications", Label: "Notifications", Priority: 7},       // Notification sinks (Discord, email, webhooks)
	{ID: "analytics", Label: "Analytics", Priority: 8},               // Watch history, statistics, insights (Trakt, etc.)
	{ID: "monitoring", Label: "Monitoring", Priority: 9},             // Observability (Prometheus, Grafana, health checks)
	{ID: "local", Label: "Local", Priority: 10},                      // Built-in plugins
	{ID: "other", Label: "Other", Priority: 99},
}

// ValidCapabilities defines the allowed capability strings.
// Capabilities declare what a plugin provides to the system and other plugins.
var ValidCapabilities = []string{
	// Search capabilities
	"search",          // General search functionality
	"search_provider", // Can be used as a search backend
	"vector_search",   // Vector/semantic search support

	// AI provider capabilities
	"provider",  // Base AI provider capability (use with provider:* for specific providers)
	"embedding", // Can generate embeddings
	"chat",      // Can handle chat/completion requests

	// Enricher capabilities
	"metadata",     // Can enrich with metadata
	"artwork",      // Can provide artwork/images
	"external_ids", // Can provide external IDs (IMDB, TMDB, etc.)
	"trending",     // Can provide trending content
	"subtitles",    // Can provide or fetch subtitles
	"lyrics",       // Can provide song lyrics

	// Notification capabilities
	"notification_sink", // Can receive and deliver notifications
	"webhook_sender",    // Can send webhooks to external services
	"webhook_receiver",  // Can receive incoming webhooks (e.g., from GitHub, Sonarr, etc.)

	// Recommendation capabilities
	"recommendations", // Can provide content recommendations

	// Sync/integration capabilities
	"watch_history",  // Can sync watch history (Trakt, Simkl, etc.)
	"scrobble",       // Can scrobble plays to external services
	"list_sync",      // Can sync playlists/collections with external services
	"calendar",       // Can provide release calendar data

	// Analytics capabilities
	"statistics", // Can provide viewing statistics and insights
	"reports",    // Can generate usage reports

	// Playback capabilities
	"playback_reporting", // Reports playback events to external services
	"skip_intro",         // Can detect and skip intros
	"skip_credits",       // Can detect and skip credits

	// Storage/backup capabilities
	"backup",  // Can backup/restore data
	"storage", // Provides external storage integration

	// Authentication capabilities
	"auth_provider", // Can authenticate users (Authelia, Authentik, OIDC, LDAP)
	"user_sync",     // Can sync users/groups from external directory

	// Monitoring/observability capabilities
	"metrics",      // Exposes metrics (Prometheus, StatsD, etc.)
	"tracing",      // Distributed tracing (OpenTelemetry, Jaeger)
	"health_check", // External health check integration

	// Transcoding capabilities
	"transcode", // Can provide transcoding profiles or hardware acceleration

	// Download/acquisition capabilities
	"download_client", // Can send to download clients (qBittorrent, SABnzbd, etc.)

	// Media management capabilities (*arr ecosystem)
	"media_requests",  // Can handle media requests (Jellyseerr, Overseerr, Ombi)
	"library_sync",    // Can sync library with external managers (Sonarr, Radarr, Lidarr)
	"collection_sync", // Can sync collections/playlists with *arr apps
}

// displayCategoryByID provides O(1) lookup for display categories.
var displayCategoryByID = func() map[string]DisplayCategory {
	m := make(map[string]DisplayCategory, len(DisplayCategories))
	for _, c := range DisplayCategories {
		m[c.ID] = c
	}
	return m
}()

// validCapabilitySet provides O(1) lookup for capabilities.
var validCapabilitySet = func() map[string]bool {
	m := make(map[string]bool, len(ValidCapabilities))
	for _, c := range ValidCapabilities {
		m[c] = true
	}
	return m
}()

// IsValidDisplayCategory returns true if the given value is a valid display category.
func IsValidDisplayCategory(category string) bool {
	_, ok := displayCategoryByID[category]
	return ok
}

// GetDisplayCategory returns the DisplayCategory for a given ID.
// Returns the "other" category if the ID is not found.
func GetDisplayCategory(id string) DisplayCategory {
	if c, ok := displayCategoryByID[id]; ok {
		return c
	}
	return displayCategoryByID["other"]
}

// GetDisplayCategoryLabel returns the display label for a category ID.
// Returns "Other" if the ID is not found.
func GetDisplayCategoryLabel(id string) string {
	return GetDisplayCategory(id).Label
}

// GetDisplayCategoryPriority returns the sort priority for a category ID.
// Returns 99 (Other's priority) if the ID is not found.
func GetDisplayCategoryPriority(id string) int {
	return GetDisplayCategory(id).Priority
}

// displayCategoryIDs returns all valid display category IDs for error messages.
func displayCategoryIDs() []string {
	ids := make([]string, len(DisplayCategories))
	for i, c := range DisplayCategories {
		ids[i] = c.ID
	}
	return ids
}

// IsValidCapability returns true if the given value is a valid capability.
// Supports dynamic capabilities like "provider:ollama" where the base "provider" is valid.
func IsValidCapability(capability string) bool {
	if validCapabilitySet[capability] {
		return true
	}
	// Check for dynamic provider capabilities (e.g., "provider:ollama", "provider:anthropic")
	if strings.HasPrefix(capability, "provider:") {
		return true
	}
	return false
}

// Manifest represents the plugin.yml metadata file.
// This is read by the host before starting the plugin binary.
type Manifest struct {
	// Identity (required)
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`

	// Authorship
	Author   string `yaml:"author"`
	License  string `yaml:"license"`
	Homepage string `yaml:"homepage"`

	// Compatibility
	MinHostVersion string `yaml:"min_host_version"`

	// DisplayCategory specifies which UI category this plugin belongs to. (required)
	// Valid values: "search", "recommendations", "enrichers", "providers", "local", "other"
	DisplayCategory string `yaml:"display_category"`

	// Capabilities declares what this plugin provides to others. (required)
	// e.g., ["search", "vector_search"] or ["provider", "provider:ollama", "embedding"]
	// Other plugins can depend on these via their 'requires' field.
	Capabilities []string `yaml:"capabilities"`

	// Requires declares what capabilities this plugin needs from other plugins.
	// The host will check that at least one enabled plugin provides each capability.
	// e.g., ["embedding"] for a search plugin that needs an embedding provider
	Requires []string `yaml:"requires,omitempty"`

	// MediaTypes lists the media types this enricher supports.
	MediaTypes []string `yaml:"media_types,omitempty"`

	// Required permissions
	Permissions []string `yaml:"permissions"`
}
