package internal

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// SettingsSchema returns the JSON Schema for Recommendations settings.
func SettingsSchema() *sdk.Schema {
	return sdk.NewSchema("Recommendations Settings").
		Meta(sdk.PluginMeta{
			DisplayName: "Recommendations",
			Description: "Personalized recommendations based on your ratings and watch history",
			Tip:         "Rate movies and shows to get better recommendations.",
			Icon:        "sparkles",
		}).
		// General settings
		Property("enabled", sdk.Boolean().
			Title("Enable Recommendations").
			Description("Show personalized recommendation rows on the home screen").
			Default(true)).
		Property("max_recommendations", sdk.Integer().
			Title("Max Items Per Row").
			Description("Maximum number of items to show in each recommendation row").
			Default(20).
			Min(5).
			Max(50)).
		// Hybrid scoring
		Property("use_hybrid_scoring", sdk.Boolean().
			Title("Use Hybrid Scoring").
			Description("Enable advanced hybrid recommendation engine combining multiple strategies").
			Default(true)).
		Property("collaborative_weight", sdk.Integer().
			Title("Collaborative Filtering Weight").
			Description("Weight for 'users who watched X also watched Y' patterns (0-100)").
			Default(50).
			Min(0).
			Max(100)).
		Property("semantic_weight", sdk.Integer().
			Title("Content Similarity Weight").
			Description("Weight for content-based similarity (plot, cast, genres) (0-100)").
			Default(30).
			Min(0).
			Max(100)).
		Property("exploration_weight", sdk.Integer().
			Title("Exploration Weight").
			Description("Weight for discovery of new content outside your usual preferences (0-100)").
			Default(10).
			Min(0).
			Max(100)).
		// Legacy algorithm weights (deprecated but kept for backwards compatibility)
		Property("similar_weight", sdk.Integer().
			Title("Similar Items Weight (Legacy)").
			Description("Deprecated: Use Content Similarity Weight instead").
			Default(50).
			Min(0).
			Max(100)).
		Property("favorite_weight", sdk.Integer().
			Title("Favorites Weight (Legacy)").
			Description("Deprecated: Use Collaborative Filtering Weight instead").
			Default(50).
			Min(0).
			Max(100)).
		// Sections for UI grouping
		Section(sdk.NewSection("general").
			Title("General").
			Properties("enabled", "max_recommendations")).
		Section(sdk.NewSection("hybrid").
			Title("Hybrid Scoring").
			Properties("use_hybrid_scoring", "collaborative_weight", "semantic_weight", "exploration_weight")).
		// Home screen widgets
		// Note: "favorites" widget is now provided by core, not this plugin
		// These widgets don't require specific capabilities because the plugin
		// handles fallback internally (vector_search -> genre-based if unavailable)
		Widgets([]sdk.Widget{
			// "For You" - personalized recommendations based on ratings
			{
				ID:              "rec-for-you",
				Type:            sdk.WidgetTypeMediaRow,
				Location:        sdk.LocationHomepageSections,
				ClientTypes:     []string{sdk.ClientTypeAll},
				Priority:        90,  // High priority - personalized content
				CacheTTLSeconds: 300, // 5 minutes
				Config: map[string]any{
					"endpoint": "/recommendations/for-you",
					"title":    "Recommended For You",
				},
				SettingsKey: "enabled",
			},
			// "Because You Liked" - similar to favorited items
			{
				ID:              "rec-because-you-liked",
				Type:            sdk.WidgetTypeMediaRow,
				Location:        sdk.LocationHomepageSections,
				ClientTypes:     []string{sdk.ClientTypeAll},
				Priority:        80,
				CacheTTLSeconds: 600, // 10 minutes
				Config: map[string]any{
					"endpoint": "/recommendations/because-you-liked",
					"title":    "Because You Liked...",
				},
				SettingsKey: "enabled",
			},
			// Hidden Gems - curated collection of overlooked films
			{
				ID:              "rec-hidden-gems",
				Type:            sdk.WidgetTypeMediaRow,
				Location:        sdk.LocationHomepageSections,
				ClientTypes:     []string{sdk.ClientTypeAll},
				Priority:        70,
				CacheTTLSeconds: 3600, // 1 hour - curated content changes less often
				Config: map[string]any{
					"endpoint":        "/themed/row",
					"endpoint_params": map[string]string{"id": "hidden-gems"},
					"title":           "Hidden Gems",
				},
				SettingsKey: "enabled",
			},
			// Time-of-day mood row (late night, morning, evening)
			{
				ID:              "rec-mood",
				Type:            sdk.WidgetTypeMediaRow,
				Location:        sdk.LocationHomepageSections,
				ClientTypes:     []string{sdk.ClientTypeAll},
				Priority:        65,
				CacheTTLSeconds: 1800, // 30 minutes - changes with time of day
				Config: map[string]any{
					"endpoint":        "/themed/row",
					"endpoint_params": map[string]string{"id": "mood"},
					"title":           "Perfect For Right Now",
					"dynamic":         true, // Indicates this widget's content varies by context
				},
				SettingsKey: "enabled",
			},
		})
}
