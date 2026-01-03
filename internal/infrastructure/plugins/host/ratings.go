package host

import (
	"context"
	"log/slog"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/domain/ratings"
)

// RatingsServer implements the HostRatings gRPC service.
// It provides read-only access to user ratings for plugins.
type RatingsServer struct {
	pluginv1.UnimplementedHostRatingsServer
	repo   ratings.Repository
	logger *slog.Logger
}

// NewRatingsServer creates a new RatingsServer.
func NewRatingsServer(repo ratings.Repository, logger *slog.Logger) *RatingsServer {
	return &RatingsServer{
		repo:   repo,
		logger: logger,
	}
}

// ListRatings returns all ratings for a user, optionally filtered.
func (s *RatingsServer) ListRatings(ctx context.Context, req *pluginv1.ListRatingsRequest) (*pluginv1.ListRatingsResponse, error) {
	userRatings, err := s.repo.List(ctx, req.UserId, req.EntityType, req.RatingType)
	if err != nil {
		s.logger.Error("failed to list ratings", "user_id", req.UserId, "error", err)
		return nil, err
	}

	protoRatings := make([]*pluginv1.UserRating, len(userRatings))
	for i, r := range userRatings {
		protoRatings[i] = domainToProto(r)
	}

	return &pluginv1.ListRatingsResponse{Ratings: protoRatings}, nil
}

// GetRatedEntityIDs returns entity IDs with a specific rating type.
func (s *RatingsServer) GetRatedEntityIDs(ctx context.Context, req *pluginv1.GetRatedEntityIDsRequest) (*pluginv1.EntityIDsResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	ids, err := s.repo.ListByRating(ctx, req.UserId, req.EntityType, req.Rating, limit)
	if err != nil {
		s.logger.Error("failed to get rated entity IDs",
			"user_id", req.UserId,
			"entity_type", req.EntityType,
			"rating", req.Rating,
			"error", err)
		return nil, err
	}

	return &pluginv1.EntityIDsResponse{EntityIds: ids}, nil
}

// GetPositivelyRatedIDs returns entity IDs with positive ratings (favorite or up).
func (s *RatingsServer) GetPositivelyRatedIDs(ctx context.Context, req *pluginv1.GetPositivelyRatedIDsRequest) (*pluginv1.EntityIDsResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}

	ids, err := s.repo.ListByPositiveRating(ctx, req.UserId, req.EntityType, limit)
	if err != nil {
		s.logger.Error("failed to get positively rated entity IDs",
			"user_id", req.UserId,
			"entity_type", req.EntityType,
			"error", err)
		return nil, err
	}

	return &pluginv1.EntityIDsResponse{EntityIds: ids}, nil
}

// HasRatings returns whether a user has any ratings.
func (s *RatingsServer) HasRatings(ctx context.Context, req *pluginv1.HasRatingsRequest) (*pluginv1.HasRatingsResponse, error) {
	hasRatings, err := s.repo.HasRatings(ctx, req.UserId)
	if err != nil {
		s.logger.Error("failed to check if user has ratings", "user_id", req.UserId, "error", err)
		return nil, err
	}

	return &pluginv1.HasRatingsResponse{HasRatings: hasRatings}, nil
}

// domainToProto converts a domain UserRating to protobuf.
func domainToProto(r *ratings.UserRating) *pluginv1.UserRating {
	return &pluginv1.UserRating{
		Id:         r.ID,
		UserId:     r.UserID,
		EntityType: r.EntityType,
		EntityId:   r.EntityID,
		Rating:     r.Rating,
		CreatedAt:  r.CreatedAt.Unix(),
		UpdatedAt:  r.UpdatedAt.Unix(),
	}
}

// Verify interface implementation
var _ pluginv1.HostRatingsServer = (*RatingsServer)(nil)
