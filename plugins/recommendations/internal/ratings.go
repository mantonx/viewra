package internal

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// RatingsService handles user ratings storage and retrieval.
type RatingsService struct {
	sql    *sdk.SQLClient
	logger *slog.Logger
}

// NewRatingsService creates a new ratings service.
func NewRatingsService(sql *sdk.SQLClient, logger *slog.Logger) *RatingsService {
	return &RatingsService{
		sql:    sql,
		logger: logger,
	}
}

// GetRatings returns all ratings for a user, optionally filtered by entity type and rating type.
func (s *RatingsService) GetRatings(ctx context.Context, userID, entityType, ratingType string) ([]Rating, error) {
	query := `SELECT entity_type, entity_id, rating, created_at, updated_at 
		FROM user_ratings WHERE user_id = ?`
	args := []any{userID}

	if entityType != "" {
		query += " AND entity_type = ?"
		args = append(args, entityType)
	}
	if ratingType != "" {
		query += " AND rating = ?"
		args = append(args, ratingType)
	}
	query += " ORDER BY updated_at DESC"

	rows, err := s.sql.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query ratings: %w", err)
	}
	defer rows.Close()

	var ratings []Rating
	for rows.Next() {
		var entType, rating, createdAt, updatedAt string
		var entityID int64

		if err := rows.Scan(&entType, &entityID, &rating, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan rating: %w", err)
		}

		r := Rating{
			UserID:     userID,
			EntityType: entType,
			EntityID:   entityID,
			Rating:     rating,
			CreatedAt:  parseTime(createdAt),
			UpdatedAt:  parseTime(updatedAt),
		}
		ratings = append(ratings, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ratings: %w", err)
	}

	return ratings, nil
}

// GetRating returns a specific rating for a user and entity.
func (s *RatingsService) GetRating(ctx context.Context, userID, entityType string, entityID int64) (*Rating, error) {
	row := s.sql.QueryRow(ctx,
		`SELECT rating, created_at, updated_at FROM user_ratings 
		WHERE user_id = ? AND entity_type = ? AND entity_id = ?`,
		userID, entityType, entityID)

	var rating, createdAt, updatedAt string
	if err := row.Scan(&rating, &createdAt, &updatedAt); err != nil {
		if err == sdk.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query rating: %w", err)
	}

	return &Rating{
		UserID:     userID,
		EntityType: entityType,
		EntityID:   entityID,
		Rating:     rating,
		CreatedAt:  parseTime(createdAt),
		UpdatedAt:  parseTime(updatedAt),
	}, nil
}

// SetRating creates or updates a rating.
func (s *RatingsService) SetRating(ctx context.Context, userID, entityType string, entityID int64, rating string) error {
	now := time.Now().Format(time.RFC3339)

	_, _, err := s.sql.Exec(ctx,
		`INSERT INTO user_ratings (user_id, entity_type, entity_id, rating, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, entity_type, entity_id) DO UPDATE SET
			rating = excluded.rating,
			updated_at = excluded.updated_at`,
		userID, entityType, entityID, rating, now, now)
	if err != nil {
		return fmt.Errorf("upsert rating: %w", err)
	}

	s.logger.Debug("rating set", "user_id", userID, "entity_type", entityType, "entity_id", entityID, "rating", rating)
	return nil
}

// DeleteRating removes a rating.
func (s *RatingsService) DeleteRating(ctx context.Context, userID, entityType string, entityID int64) error {
	_, _, err := s.sql.Exec(ctx,
		`DELETE FROM user_ratings WHERE user_id = ? AND entity_type = ? AND entity_id = ?`,
		userID, entityType, entityID)
	if err != nil {
		return fmt.Errorf("delete rating: %w", err)
	}

	s.logger.Debug("rating deleted", "user_id", userID, "entity_type", entityType, "entity_id", entityID)
	return nil
}

// GetFavoriteEntityIDs returns entity IDs that the user has favorited.
func (s *RatingsService) GetFavoriteEntityIDs(ctx context.Context, userID, entityType string, limit int) ([]int64, error) {
	query := `SELECT entity_id FROM user_ratings 
		WHERE user_id = ? AND rating = 'favorite'`
	args := []any{userID}

	if entityType != "" {
		query += " AND entity_type = ?"
		args = append(args, entityType)
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.sql.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query favorite IDs: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan entity_id: %w", err)
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate favorites: %w", err)
	}

	return ids, nil
}

// GetUpvotedEntityIDs returns entity IDs that the user has upvoted.
func (s *RatingsService) GetUpvotedEntityIDs(ctx context.Context, userID, entityType string, limit int) ([]int64, error) {
	query := `SELECT entity_id FROM user_ratings 
		WHERE user_id = ? AND rating IN ('up', 'favorite')`
	args := []any{userID}

	if entityType != "" {
		query += " AND entity_type = ?"
		args = append(args, entityType)
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.sql.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query upvoted IDs: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan entity_id: %w", err)
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate upvoted: %w", err)
	}

	return ids, nil
}

// GetDownvotedEntityIDs returns entity IDs that the user has downvoted.
func (s *RatingsService) GetDownvotedEntityIDs(ctx context.Context, userID string, limit int) ([]int64, error) {
	rows, err := s.sql.Query(ctx,
		`SELECT entity_id FROM user_ratings 
		WHERE user_id = ? AND rating = 'down'
		ORDER BY updated_at DESC LIMIT ?`,
		userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query downvoted IDs: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan entity_id: %w", err)
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate downvoted: %w", err)
	}

	return ids, nil
}

// HasRatings returns true if the user has any ratings.
func (s *RatingsService) HasRatings(ctx context.Context, userID string) bool {
	row := s.sql.QueryRow(ctx,
		`SELECT 1 FROM user_ratings WHERE user_id = ? LIMIT 1`, userID)

	var exists int
	if err := row.Scan(&exists); err != nil {
		return false
	}
	return exists == 1
}

// parseTime parses a time string in RFC3339 format.
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
