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
		// Algorithm weights
		Property("similar_weight", sdk.Integer().
			Title("Similar Items Weight").
			Description("Weight for 'similar to' recommendations (0-100)").
			Default(50).
			Min(0).
			Max(100)).
		Property("favorite_weight", sdk.Integer().
			Title("Favorites Weight").
			Description("Weight for favorite-based recommendations (0-100)").
			Default(50).
			Min(0).
			Max(100)).
		// Sections for UI grouping
		Section(sdk.NewSection("general").
			Title("General").
			Properties("enabled", "max_recommendations")).
		Section(sdk.NewSection("algorithm").
			Title("Algorithm Weights").
			Properties("similar_weight", "favorite_weight")).
		// Home screen widgets
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
			// "Your Favorites" - items user has favorited
			{
				ID:              "favorites",
				Type:            sdk.WidgetTypeMediaRow,
				Location:        sdk.LocationHomepageSections,
				ClientTypes:     []string{sdk.ClientTypeAll},
				Priority:        70,
				CacheTTLSeconds: 120, // 2 minutes - changes when user favorites
				Config: map[string]any{
					"endpoint": "/favorites",
					"title":    "Your Favorites",
				},
				SettingsKey: "enabled",
			},
		})
}
