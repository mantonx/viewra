package ratings

import (
	"errors"
	"time"
)

// Rating types
const (
	RatingUp       = "up"
	RatingDown     = "down"
	RatingFavorite = "favorite"
)

// Errors
var (
	ErrInvalidRating     = errors.New("invalid rating type")
	ErrRatingNotFound    = errors.New("rating not found")
	ErrInvalidEntityType = errors.New("invalid entity type")
)

// ValidEntityTypes defines allowed entity types for ratings.
var ValidEntityTypes = map[string]bool{
	"movie":      true,
	"tv_show":    true,
	"tv_episode": true,
}

// UserRating represents a user's rating for a media item.
type UserRating struct {
	ID         int64
	UserID     string
	EntityType string
	EntityID   int64
	Rating     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Validate validates the rating entity.
func (r *UserRating) Validate() error {
	if r.UserID == "" {
		return errors.New("user_id is required")
	}
	if r.EntityType == "" {
		return ErrInvalidEntityType
	}
	if !ValidEntityTypes[r.EntityType] {
		return ErrInvalidEntityType
	}
	if r.EntityID <= 0 {
		return errors.New("entity_id must be positive")
	}
	if !IsValidRating(r.Rating) {
		return ErrInvalidRating
	}
	return nil
}

// IsValidRating checks if the rating value is valid.
func IsValidRating(rating string) bool {
	return rating == RatingUp || rating == RatingDown || rating == RatingFavorite
}
