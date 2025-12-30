package internal

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// SettingsSchema returns the JSON Schema for MusicBrainz plugin settings.
func SettingsSchema() *sdk.Schema {
	return sdk.NewSchema("MusicBrainz Settings").
		Meta(sdk.PluginMeta{
			DisplayName: "MusicBrainz",
			Description: "Fetch music metadata from MusicBrainz and cover art from Cover Art Archive",
			Tip:         "MusicBrainz requires a User-Agent for API access. Contact email helps them reach you if issues arise.",
			Icon:        "music",
		}).
		// Contact email (required by MusicBrainz policy)
		Property("contact_email", sdk.String().
			Title("Contact Email").
			Description("Contact email for MusicBrainz API policy compliance. Not shared publicly.").
			Required()).
		// Match confidence threshold
		Property("min_confidence", sdk.Number().
			Title("Minimum Match Confidence").
			Description("Minimum confidence score (0-1) for accepting a match. Higher values reduce false positives.").
			Default(0.7).
			Min(0.0).
			Max(1.0)).
		// Cache TTL
		Property("cache_ttl_hours", sdk.Integer().
			Title("Cache Duration (hours)").
			Description("How long to cache API responses. MusicBrainz data changes rarely.").
			Default(168). // 1 week
			Min(24).
			Max(720)). // Max 30 days
		// Cover art settings
		Property("fetch_cover_art", sdk.Boolean().
			Title("Fetch Cover Art").
			Description("Download album cover art from Cover Art Archive").
			Default(true)).
		Property("cover_art_size", sdk.String().
			Title("Cover Art Size").
			Description("Size of downloaded cover art images").
			Default("large").
			EnumStrings("small", "large", "original").
			DependsOn("fetch_cover_art", true)).
		// Artist images (from fanart.tv integration in the future)
		Property("fetch_artist_images", sdk.Boolean().
			Title("Fetch Artist Images").
			Description("Download artist photos (requires additional setup)").
			Default(false)).
		// Sections for UI organization
		Section(sdk.NewSection("general").
			Title("General").
			Properties("contact_email", "min_confidence", "cache_ttl_hours")).
		Section(sdk.NewSection("images").
			Title("Images").
			Properties("fetch_cover_art", "cover_art_size", "fetch_artist_images"))
}
