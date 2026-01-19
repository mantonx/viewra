package plugins

import "github.com/mantonx/viewra/internal/infrastructure/plugins/manifest"

// Category is an alias for manifest.DisplayCategory.
// Kept for backwards compatibility with existing code.
type Category = manifest.DisplayCategory

// Categories returns the predefined plugin categories from the manifest package.
// This is the single source of truth for display categories.
func Categories() []Category {
	return manifest.DisplayCategories
}

// IsValidCategory returns true if the given ID is a valid display category.
func IsValidCategory(id string) bool {
	return manifest.IsValidDisplayCategory(id)
}

// GetCategoryLabel returns the display label for a category ID.
// Returns "Other" if the ID is not found.
func GetCategoryLabel(id string) string {
	return manifest.GetDisplayCategoryLabel(id)
}

// GetCategoryPriority returns the sort priority for a category ID.
// Returns 99 (Other's priority) if the ID is not found.
func GetCategoryPriority(id string) int {
	return manifest.GetDisplayCategoryPriority(id)
}
