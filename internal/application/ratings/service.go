package ratings

import (
	"context"
	"time"

	"github.com/mantonx/viewra/internal/domain/ratings"
)

// Service provides all ratings-related operations.
type Service struct {
	repo ratings.Repository
}

// NewService creates a new ratings service.
func NewService(repo ratings.Repository) *Service {
	return &Service{repo: repo}
}

// RatingDTO represents a user rating for API responses.
type RatingDTO struct {
	EntityType string    `json:"entity_type"`
	EntityID   int64     `json:"entity_id"`
	Rating     string    `json:"rating"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CreateRatingRequest represents a request to create or update a rating.
type CreateRatingRequest struct {
	UserID     string `json:"-"`
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	Rating     string `json:"rating"`
}

// ListRatingsRequest represents a request to list ratings.
type ListRatingsRequest struct {
	UserID     string `json:"-"`
	EntityType string `json:"entity_type,omitempty"`
	Rating     string `json:"rating,omitempty"`
}

// Get returns a specific rating for a user and entity.
func (s *Service) Get(ctx context.Context, userID, entityType string, entityID int64) (*RatingDTO, error) {
	rating, err := s.repo.Get(ctx, userID, entityType, entityID)
	if err != nil {
		return nil, err
	}
	if rating == nil {
		return nil, nil
	}
	return toDTO(rating), nil
}

// List returns all ratings for a user with optional filters.
func (s *Service) List(ctx context.Context, req *ListRatingsRequest) ([]*RatingDTO, error) {
	userRatings, err := s.repo.List(ctx, req.UserID, req.EntityType, req.Rating)
	if err != nil {
		return nil, err
	}
	return toDTOs(userRatings), nil
}

// CreateOrUpdate creates or updates a rating.
func (s *Service) CreateOrUpdate(ctx context.Context, req *CreateRatingRequest) (*RatingDTO, error) {
	rating := &ratings.UserRating{
		UserID:     req.UserID,
		EntityType: req.EntityType,
		EntityID:   req.EntityID,
		Rating:     req.Rating,
	}

	if err := rating.Validate(); err != nil {
		return nil, err
	}

	if err := s.repo.Upsert(ctx, rating); err != nil {
		return nil, err
	}

	// Fetch the created/updated rating to get timestamps
	created, err := s.repo.Get(ctx, req.UserID, req.EntityType, req.EntityID)
	if err != nil {
		return nil, err
	}
	return toDTO(created), nil
}

// Delete removes a rating.
func (s *Service) Delete(ctx context.Context, userID, entityType string, entityID int64) error {
	return s.repo.Delete(ctx, userID, entityType, entityID)
}

// HasRatings returns true if the user has any ratings.
func (s *Service) HasRatings(ctx context.Context, userID string) (bool, error) {
	return s.repo.HasRatings(ctx, userID)
}

// GetFavoriteIDs returns entity IDs that the user has favorited.
func (s *Service) GetFavoriteIDs(ctx context.Context, userID, entityType string, limit int) ([]int64, error) {
	return s.repo.ListByRating(ctx, userID, entityType, ratings.RatingFavorite, limit)
}

// CountFavorites returns the number of favorites for a user.
func (s *Service) CountFavorites(ctx context.Context, userID string) (int, error) {
	return s.repo.CountByRating(ctx, userID, ratings.RatingFavorite)
}

func toDTO(r *ratings.UserRating) *RatingDTO {
	return &RatingDTO{
		EntityType: r.EntityType,
		EntityID:   r.EntityID,
		Rating:     r.Rating,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

func toDTOs(rs []*ratings.UserRating) []*RatingDTO {
	dtos := make([]*RatingDTO, len(rs))
	for i, r := range rs {
		dtos[i] = toDTO(r)
	}
	return dtos
}
