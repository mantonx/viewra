package internal

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// SettingsSchema returns the JSON Schema for Semantic Search settings.
func SettingsSchema() *sdk.Schema {
	return sdk.NewSchema("Semantic Search Settings").
		Meta(sdk.PluginMeta{
			DisplayName: "Semantic Search",
			Description: "AI-powered semantic search for your media library",
			Tip:         "Configure indexing behavior and search parameters.",
			Icon:        "search",
		}).
		// Indexing settings
		Property("auto_index", sdk.Boolean().
			Title("Auto-Index New Media").
			Description("Automatically index media during enrichment pipeline").
			Default(true)).
		Property("reindex_on_metadata_change", sdk.Boolean().
			Title("Re-index on Metadata Change").
			Description("Re-index media when its metadata is updated by enrichers").
			Default(true)).
		Property("batch_size", sdk.Integer().
			Title("Batch Size").
			Description("Number of items to process in each indexing batch").
			Default(50).
			Min(10).
			Max(200)).
		// Search settings
		Property("default_limit", sdk.Integer().
			Title("Default Results").
			Description("Default number of search results to return").
			Default(20).
			Min(5).
			Max(100)).
		Property("min_similarity", sdk.Number().
			Title("Minimum Similarity").
			Description("Minimum similarity score for results to be included").
			Default(0.3).
			Min(0.0).
			Max(1.0)).
		// Mood tags
		Property("mood_tags_enabled", sdk.Boolean().
			Title("Enable Mood Tags").
			Description("Generate mood/vibe tags (e.g., 'uplifting', 'dark') for enhanced search").
			Default(true)).
		// Sections for UI grouping
		Section(sdk.NewSection("indexing").
			Title("Indexing").
			Properties("auto_index", "reindex_on_metadata_change", "batch_size")).
		Section(sdk.NewSection("search").
			Title("Search").
			Properties("default_limit", "min_similarity")).
		Section(sdk.NewSection("features").
			Title("Features").
			Properties("mood_tags_enabled"))
}
