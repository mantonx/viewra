package plugins

// ComputeFilterTabs returns the available filter tabs with counts based on the plugin list.
// Uses the predefined Categories for ordering and labels.
func ComputeFilterTabs(plugins []PluginSummary) []FilterTab {
	counts := map[string]int{
		"all":      len(plugins),
		"disabled": 0,
	}

	// Count plugins by their display category ID
	for _, p := range plugins {
		if !p.Enabled {
			counts["disabled"]++
		}
		// DisplayCategory now stores the category ID (e.g., "search", "providers")
		if p.DisplayCategory != "" && p.DisplayCategory != "local" && p.DisplayCategory != "other" {
			counts[p.DisplayCategory]++
		}
	}

	// Build tabs using the predefined category order
	tabs := []FilterTab{
		{ID: "all", Label: "All", Count: counts["all"]},
	}

	for _, cat := range Categories() {
		// Skip "local" and "other" as filter tabs (they're grouping categories, not filters)
		if cat.ID == "local" || cat.ID == "other" {
			continue
		}
		if counts[cat.ID] > 0 {
			tabs = append(tabs, FilterTab{ID: cat.ID, Label: cat.Label, Count: counts[cat.ID]})
		}
	}

	tabs = append(tabs, FilterTab{ID: "disabled", Label: "Disabled", Count: counts["disabled"]})

	return tabs
}

// computeDisplayCategory determines the UI category for a plugin.
// Plugins should declare display_category in their manifest; builtins default to "local".
func computeDisplayCategory(manifestCategory string, isBuiltin bool) string {
	if manifestCategory != "" && IsValidCategory(manifestCategory) {
		return manifestCategory
	}
	if isBuiltin {
		return "local"
	}
	return "other"
}
