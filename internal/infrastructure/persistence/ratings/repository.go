package ratings

import (
	"context"
	"database/sql"
	"time"

	"github.com/mantonx/viewra/internal/domain/ratings"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
)

// Repository implements ratings.Repository using the unified querier.
type Repository struct {
	db      *sql.DB
	querier *unified.Querier
}

// NewRepository creates a new ratings repository.
func NewRepository(db *sql.DB, driver string) *Repository {
	return &Repository{
		db:      db,
		querier: unified.NewQuerier(db, driver),
	}
}

// Get returns a specific rating for a user and entity.
func (r *Repository) Get(ctx context.Context, userID, entityType string, entityID int64) (*ratings.UserRating, error) {
	row, err := r.querier.GetUserRating(ctx, unified.GetUserRatingParams{
		UserID:     userID,
		EntityType: entityType,
		EntityID:   entityID,
	})
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toRating(&row), nil
}

// List returns all ratings for a user with optional filters.
func (r *Repository) List(ctx context.Context, userID, entityType, ratingType string) ([]*ratings.UserRating, error) {
	var rows []unified.UserRating
	var err error

	switch {
	case entityType != "" && ratingType != "":
		rows, err = r.querier.ListUserRatingsByTypeAndRating(ctx, unified.ListUserRatingsByTypeAndRatingParams{
			UserID:     userID,
			EntityType: entityType,
			Rating:     ratingType,
		})
	case entityType != "":
		rows, err = r.querier.ListUserRatingsByType(ctx, unified.ListUserRatingsByTypeParams{
			UserID:     userID,
			EntityType: entityType,
		})
	case ratingType != "":
		rows, err = r.querier.ListUserRatingsByRating(ctx, unified.ListUserRatingsByRatingParams{
			UserID: userID,
			Rating: ratingType,
		})
	default:
		rows, err = r.querier.ListUserRatings(ctx, userID)
	}

	if err != nil {
		return nil, err
	}
	return toRatings(rows), nil
}

// ListByRating returns entity IDs for a user with a specific rating.
func (r *Repository) ListByRating(ctx context.Context, userID, entityType, rating string, limit int) ([]int64, error) {
	if entityType != "" {
		return r.querier.ListEntityIDsByTypeAndRating(ctx, unified.ListEntityIDsByTypeAndRatingParams{
			UserID:     userID,
			EntityType: entityType,
			Rating:     rating,
			Limit:      int64(limit),
		})
	}
	return r.querier.ListEntityIDsByRating(ctx, unified.ListEntityIDsByRatingParams{
		UserID: userID,
		Rating: rating,
		Limit:  int64(limit),
	})
}

// ListByPositiveRating returns entity IDs for a user with positive ratings (favorite or up).
func (r *Repository) ListByPositiveRating(ctx context.Context, userID, entityType string, limit int) ([]int64, error) {
	if entityType != "" {
		return r.querier.ListEntityIDsByTypeAndPositiveRating(ctx, unified.ListEntityIDsByTypeAndPositiveRatingParams{
			UserID:     userID,
			EntityType: entityType,
			Limit:      int64(limit),
		})
	}
	return r.querier.ListEntityIDsByPositiveRating(ctx, unified.ListEntityIDsByPositiveRatingParams{
		UserID: userID,
		Limit:  int64(limit),
	})
}

// Upsert creates or updates a rating.
func (r *Repository) Upsert(ctx context.Context, rating *ratings.UserRating) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.querier.UpsertUserRating(ctx, unified.UpsertUserRatingParams{
		UserID:     rating.UserID,
		EntityType: rating.EntityType,
		EntityID:   rating.EntityID,
		Rating:     rating.Rating,
		CreatedAt:  sql.NullString{String: now, Valid: true},
		UpdatedAt:  sql.NullString{String: now, Valid: true},
	})
	return err
}

// Delete removes a rating.
func (r *Repository) Delete(ctx context.Context, userID, entityType string, entityID int64) error {
	return r.querier.DeleteUserRating(ctx, unified.DeleteUserRatingParams{
		UserID:     userID,
		EntityType: entityType,
		EntityID:   entityID,
	})
}

// DeleteAllForUser removes all ratings for a user.
func (r *Repository) DeleteAllForUser(ctx context.Context, userID string) error {
	return r.querier.DeleteAllUserRatings(ctx, userID)
}

// HasRatings returns true if the user has any ratings.
func (r *Repository) HasRatings(ctx context.Context, userID string) (bool, error) {
	result, err := r.querier.HasUserRatings(ctx, userID)
	if err != nil {
		return false, err
	}
	return result != 0, nil
}

// CountByRating returns the count of ratings of a specific type for a user.
func (r *Repository) CountByRating(ctx context.Context, userID, rating string) (int, error) {
	count, err := r.querier.CountUserRatingsByRating(ctx, unified.CountUserRatingsByRatingParams{
		UserID: userID,
		Rating: rating,
	})
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// toRating converts a SQLC row to a domain entity.
func toRating(row *unified.UserRating) *ratings.UserRating {
	r := &ratings.UserRating{
		ID:         row.ID,
		UserID:     row.UserID,
		EntityType: row.EntityType,
		EntityID:   row.EntityID,
		Rating:     row.Rating,
	}
	if row.CreatedAt.Valid {
		r.CreatedAt, _ = time.Parse(time.RFC3339, row.CreatedAt.String)
	}
	if row.UpdatedAt.Valid {
		r.UpdatedAt, _ = time.Parse(time.RFC3339, row.UpdatedAt.String)
	}
	return r
}

// toRatings converts multiple SQLC rows to domain entities.
func toRatings(rows []unified.UserRating) []*ratings.UserRating {
	result := make([]*ratings.UserRating, len(rows))
	for i := range rows {
		result[i] = toRating(&rows[i])
	}
	return result
}
