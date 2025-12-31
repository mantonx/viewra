package home

import "context"

// PreferencesRepository handles storage of user widget preferences.
type PreferencesRepository interface {
	// Get returns all preferences for a user.
	Get(ctx context.Context, userID string) ([]*WidgetPreference, error)

	// GetForWidget returns a specific widget preference.
	GetForWidget(ctx context.Context, userID, widgetID string) (*WidgetPreference, error)

	// Save creates or updates a widget preference.
	Save(ctx context.Context, pref *WidgetPreference) error

	// SaveAll saves multiple preferences in a transaction.
	SaveAll(ctx context.Context, prefs []*WidgetPreference) error

	// Delete removes a specific preference.
	Delete(ctx context.Context, userID, widgetID string) error

	// DeleteAll removes all preferences for a user (reset to defaults).
	DeleteAll(ctx context.Context, userID string) error
}
