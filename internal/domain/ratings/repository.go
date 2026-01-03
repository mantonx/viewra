package ratings

import "context"

// Repository defines the interface for persisting and retrieving user ratings.
type Repository interface {
	// Get returns a specific rating for a user and entity.
	// Returns nil, nil if not found.
	Get(ctx context.Context, userID, entityType string, entityID int64) (*UserRating, error)

	// List returns all ratings for a user.
	// Optional filters: entityType (empty for all), ratingType (empty for all).
	List(ctx context.Context, userID, entityType, ratingType string) ([]*UserRating, error)

	// ListByRating returns entity IDs for a user with a specific rating.
	ListByRating(ctx context.Context, userID, entityType, rating string, limit int) ([]int64, error)

	// ListByPositiveRating returns entity IDs for a user with positive ratings (favorite or up).
	ListByPositiveRating(ctx context.Context, userID, entityType string, limit int) ([]int64, error)

	// Upsert creates or updates a rating.
	Upsert(ctx context.Context, rating *UserRating) error

	// Delete removes a rating.
	Delete(ctx context.Context, userID, entityType string, entityID int64) error

	// DeleteAllForUser removes all ratings for a user.
	DeleteAllForUser(ctx context.Context, userID string) error

	// HasRatings returns true if the user has any ratings.
	HasRatings(ctx context.Context, userID string) (bool, error)

	// CountByRating returns the count of ratings of a specific type for a user.
	CountByRating(ctx context.Context, userID, rating string) (int, error)
}
