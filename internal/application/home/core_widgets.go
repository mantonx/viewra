package home

import "github.com/mantonx/viewra/pkg/plugin/sdk"

// CorePluginID is the plugin ID used for core (built-in) widgets.
const CorePluginID = "core"

// CoreWidgets returns the built-in widgets provided by the core application.
// These widgets work without any plugins installed.
func CoreWidgets() []sdk.Widget {
	return []sdk.Widget{
		{
			ID:              "continue-watching",
			Type:            sdk.WidgetTypeContinueRow,
			Location:        sdk.LocationHomepageSections,
			ClientTypes:     []string{sdk.ClientTypeAll},
			Priority:        95,
			CacheTTLSeconds: 60,
			Config: map[string]any{
				"title": "Continue Watching",
			},
		},
		{
			ID:              "recently-added",
			Type:            sdk.WidgetTypeMediaRow,
			Location:        sdk.LocationHomepageSections,
			ClientTypes:     []string{sdk.ClientTypeAll},
			Priority:        85,
			CacheTTLSeconds: 300,
			Config: map[string]any{
				"title": "Recently Added",
			},
		},
		{
			ID:              "favorites",
			Type:            sdk.WidgetTypeMediaRow,
			Location:        sdk.LocationHomepageSections,
			ClientTypes:     []string{sdk.ClientTypeAll},
			Priority:        70,
			CacheTTLSeconds: 120,
			Config: map[string]any{
				"title": "Your Favorites",
			},
		},
		{
			ID:                 "trending",
			Type:               sdk.WidgetTypeMediaRow,
			Location:           sdk.LocationHomepageSections,
			ClientTypes:        []string{sdk.ClientTypeAll},
			Priority:           50,
			CacheTTLSeconds:    3600,
			RequiredCapability: "trending",
			Config: map[string]any{
				"title": "Trending",
			},
		},
		{
			ID:              "search-hero-fallback",
			Type:            sdk.WidgetTypeSearchHero,
			Location:        sdk.LocationHomepageTop,
			ClientTypes:     []string{sdk.ClientTypeWeb, sdk.ClientTypeIOS, sdk.ClientTypeAndroid},
			Priority:        50, // Lower than semantic-search's 100
			CacheTTLSeconds: 3600,
			Config: map[string]any{
				"placeholder":      "Search...",
				"show_suggestions": true,
				"suggestions_type": "genres",
			},
		},
	}
}
