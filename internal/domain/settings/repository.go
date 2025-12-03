package settings

import "context"

// SystemRepository defines the interface for system settings persistence.
type SystemRepository interface {
	// Get retrieves a system setting by key.
	// Returns ErrSettingNotFound if the setting doesn't exist.
	Get(ctx context.Context, key string) (*SystemSetting, error)

	// GetByCategory retrieves all system settings in a category.
	GetByCategory(ctx context.Context, category Category) ([]*SystemSetting, error)

	// GetAll retrieves all system settings.
	GetAll(ctx context.Context) ([]*SystemSetting, error)

	// Set creates or updates a system setting.
	Set(ctx context.Context, setting *SystemSetting) error

	// Delete removes a system setting.
	Delete(ctx context.Context, key string) error
}

// UserRepository defines the interface for user settings persistence.
type UserRepository interface {
	// Get retrieves a user setting by user ID and key.
	// Returns ErrSettingNotFound if the setting doesn't exist.
	Get(ctx context.Context, userID, key string) (*UserSetting, error)

	// GetAll retrieves all settings for a user.
	GetAll(ctx context.Context, userID string) ([]*UserSetting, error)

	// Set creates or updates a user setting.
	Set(ctx context.Context, setting *UserSetting) error

	// Delete removes a user setting.
	Delete(ctx context.Context, userID, key string) error

	// DeleteAll removes all settings for a user.
	DeleteAll(ctx context.Context, userID string) error
}
