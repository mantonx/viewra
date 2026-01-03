package sdk

import (
	"context"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"google.golang.org/grpc"
)

// RatingsClient wraps the HostRatings service for reading user ratings.
// Plugins use this to access user preferences (favorites, likes, dislikes)
// for generating personalized recommendations.
type RatingsClient struct {
	client pluginv1.HostRatingsClient
}

// NewRatingsClient creates a new ratings client.
func NewRatingsClient(conn *grpc.ClientConn) *RatingsClient {
	return &RatingsClient{client: pluginv1.NewHostRatingsClient(conn)}
}

// Rating represents a user's rating for a media item.
type Rating struct {
	ID         int64
	UserID     string
	EntityType string // "movie", "tv_show", "tv_episode"
	EntityID   int64
	Rating     string // "favorite", "up", "down"
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Rating type constants.
const (
	RatingFavorite = "favorite"
	RatingUp       = "up"
	RatingDown     = "down"
)

// ListRatings returns all ratings for a user.
// Optional filters: entityType (empty for all), ratingType (empty for all).
func (c *RatingsClient) ListRatings(ctx context.Context, userID, entityType, ratingType string) ([]*Rating, error) {
	resp, err := c.client.ListRatings(ctx, &pluginv1.ListRatingsRequest{
		UserId:     userID,
		EntityType: entityType,
		RatingType: ratingType,
	})
	if err != nil {
		return nil, err
	}

	ratings := make([]*Rating, len(resp.Ratings))
	for i, r := range resp.Ratings {
		ratings[i] = protoToRating(r)
	}
	return ratings, nil
}

// GetFavoriteIDs returns entity IDs that the user has marked as favorite.
func (c *RatingsClient) GetFavoriteIDs(ctx context.Context, userID, entityType string, limit int) ([]int64, error) {
	return c.getRatedEntityIDs(ctx, userID, entityType, RatingFavorite, limit)
}

// GetUpvotedIDs returns entity IDs that the user has upvoted (liked).
func (c *RatingsClient) GetUpvotedIDs(ctx context.Context, userID, entityType string, limit int) ([]int64, error) {
	return c.getRatedEntityIDs(ctx, userID, entityType, RatingUp, limit)
}

// GetDownvotedIDs returns entity IDs that the user has downvoted (disliked).
func (c *RatingsClient) GetDownvotedIDs(ctx context.Context, userID, entityType string, limit int) ([]int64, error) {
	return c.getRatedEntityIDs(ctx, userID, entityType, RatingDown, limit)
}

// GetPositivelyRatedIDs returns entity IDs that the user has rated positively (favorite or up).
// This is useful for recommendations that treat both ratings as positive signals.
func (c *RatingsClient) GetPositivelyRatedIDs(ctx context.Context, userID, entityType string, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 100
	}
	resp, err := c.client.GetPositivelyRatedIDs(ctx, &pluginv1.GetPositivelyRatedIDsRequest{
		UserId:     userID,
		EntityType: entityType,
		Limit:      int32(limit),
	})
	if err != nil {
		return nil, err
	}
	return resp.EntityIds, nil
}

// getRatedEntityIDs is a helper to fetch entity IDs with a specific rating.
func (c *RatingsClient) getRatedEntityIDs(ctx context.Context, userID, entityType, rating string, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 100
	}
	resp, err := c.client.GetRatedEntityIDs(ctx, &pluginv1.GetRatedEntityIDsRequest{
		UserId:     userID,
		EntityType: entityType,
		Rating:     rating,
		Limit:      int32(limit),
	})
	if err != nil {
		return nil, err
	}
	return resp.EntityIds, nil
}

// HasRatings returns whether a user has any ratings.
func (c *RatingsClient) HasRatings(ctx context.Context, userID string) (bool, error) {
	resp, err := c.client.HasRatings(ctx, &pluginv1.HasRatingsRequest{
		UserId: userID,
	})
	if err != nil {
		return false, err
	}
	return resp.HasRatings, nil
}

// protoToRating converts a protobuf UserRating to SDK Rating.
func protoToRating(r *pluginv1.UserRating) *Rating {
	return &Rating{
		ID:         r.Id,
		UserID:     r.UserId,
		EntityType: r.EntityType,
		EntityID:   r.EntityId,
		Rating:     r.Rating,
		CreatedAt:  time.Unix(r.CreatedAt, 0),
		UpdatedAt:  time.Unix(r.UpdatedAt, 0),
	}
}
