package internal

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// SettingsSchema returns the JSON Schema for TMDb plugin settings.
func SettingsSchema() *sdk.Schema {
	return sdk.NewSchema("TMDb Settings").
		Meta(sdk.PluginMeta{
			DisplayName: "TMDb",
			Description: "Fetch movie and TV metadata from The Movie Database",
			Tip:         "Uses your API key to fetch metadata, artwork, and credits from TMDb.",
			Icon:        "film",
		}).
		// Language setting - most common languages for media
		Property("language", sdk.String().
			Title("Language").
			Description("Preferred language for metadata (e.g., en-US, de-DE, ja-JP)").
			Default("en-US").
			EnumStrings(
				"en-US", "en-GB", "de-DE", "es-ES", "es-MX",
				"fr-FR", "it-IT", "ja-JP", "ko-KR", "pt-BR",
				"pt-PT", "ru-RU", "zh-CN", "zh-TW", "nl-NL",
				"pl-PL", "sv-SE", "tr-TR", "ar-SA", "hi-IN",
			)).
		// Include adult content
		Property("include_adult", sdk.Boolean().
			Title("Include Adult Content").
			Description("Include adult content in search results").
			Default(false)).
		// Cache TTL
		Property("cache_ttl_hours", sdk.Integer().
			Title("Cache Duration (hours)").
			Description("How long to cache API responses. Reduces API usage.").
			Default(24).
			Min(1).
			Max(720)). // Max 30 days
		// Image fetching settings
		Property("fetch_images", sdk.Boolean().
			Title("Fetch Images").
			Description("Download poster, backdrop, and logo images from TMDb").
			Default(true)).
		Property("image_types", sdk.Array().
			Title("Image Types").
			Description("Which types of images to download").
			ItemsEnum("poster", "backdrop", "logo").
			Default([]string{"poster", "backdrop"}).
			DependsOn("fetch_images", true)).
		Property("image_size", sdk.String().
			Title("Image Size").
			Description("Size of downloaded images. Larger sizes use more storage.").
			Default("w780").
			EnumStrings("w342", "w500", "w780", "original").
			DependsOn("fetch_images", true)).
		// Actor/cast photos
		Property("fetch_actor_photos", sdk.Boolean().
			Title("Fetch Actor Photos").
			Description("Download profile photos for cast members").
			Default(false)).
		Property("max_actor_photos", sdk.Integer().
			Title("Max Actor Photos").
			Description("Maximum number of actor photos to download per title").
			Default(10).
			Min(0).
			Max(50).
			DependsOn("fetch_actor_photos", true)).
		// Sections for UI organization
		Section(sdk.NewSection("general").
			Title("General").
			Properties("language", "include_adult", "cache_ttl_hours")).
		Section(sdk.NewSection("images").
			Title("Images").
			Properties("fetch_images", "image_types", "image_size")).
		Section(sdk.NewSection("cast").
			Title("Cast").
			Properties("fetch_actor_photos", "max_actor_photos"))
}
