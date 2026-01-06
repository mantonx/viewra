package host

import (
	"context"
	"log/slog"
	"strconv"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/progress"
)

// ProgressServer implements the HostProgress gRPC service.
// It provides read-only access to user watch progress for plugins.
type ProgressServer struct {
	pluginv1.UnimplementedHostProgressServer
	progressRepo progress.Repository
	mediaRepo    media.Repository
	logger       *slog.Logger
}

// NewProgressServer creates a new ProgressServer.
func NewProgressServer(progressRepo progress.Repository, mediaRepo media.Repository, logger *slog.Logger) *ProgressServer {
	return &ProgressServer{
		progressRepo: progressRepo,
		mediaRepo:    mediaRepo,
		logger:       logger,
	}
}

// ListWatchedItems returns all watched items for a user.
func (s *ProgressServer) ListWatchedItems(ctx context.Context, req *pluginv1.ListWatchedItemsRequest) (*pluginv1.ListWatchedItemsResponse, error) {
	userID := parseUserID(req.UserId)
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}
	offset := int(req.Offset)

	progressItems, err := s.progressRepo.ListWatchedByUserID(ctx, userID, limit, offset)
	if err != nil {
		s.logger.Error("failed to list watched items", "user_id", req.UserId, "error", err)
		return nil, err
	}

	items := make([]*pluginv1.WatchProgressItem, 0, len(progressItems))
	for _, p := range progressItems {
		item, err := s.toWatchProgressItem(ctx, p, req.MediaType)
		if err != nil {
			// Skip items where media can't be found or doesn't match filter
			continue
		}
		if item != nil {
			items = append(items, item)
		}
	}

	return &pluginv1.ListWatchedItemsResponse{
		Items: items,
		Total: int32(len(items)),
	}, nil
}

// ListInProgressItems returns items the user is currently watching.
func (s *ProgressServer) ListInProgressItems(ctx context.Context, req *pluginv1.ListInProgressItemsRequest) (*pluginv1.ListInProgressItemsResponse, error) {
	userID := parseUserID(req.UserId)
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}
	offset := int(req.Offset)

	progressItems, err := s.progressRepo.ListInProgressByUserID(ctx, userID, limit, offset)
	if err != nil {
		s.logger.Error("failed to list in-progress items", "user_id", req.UserId, "error", err)
		return nil, err
	}

	items := make([]*pluginv1.WatchProgressItem, 0, len(progressItems))
	for _, p := range progressItems {
		item, err := s.toWatchProgressItem(ctx, p, req.MediaType)
		if err != nil {
			// Skip items where media can't be found or doesn't match filter
			continue
		}
		if item != nil {
			items = append(items, item)
		}
	}

	return &pluginv1.ListInProgressItemsResponse{
		Items: items,
		Total: int32(len(items)),
	}, nil
}

// GetWatchedEntityIDs returns entity IDs of watched items.
func (s *ProgressServer) GetWatchedEntityIDs(ctx context.Context, req *pluginv1.GetWatchedEntityIDsRequest) (*pluginv1.EntityIDsResponse, error) {
	userID := parseUserID(req.UserId)
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 1000
	}

	progressItems, err := s.progressRepo.ListWatchedByUserID(ctx, userID, limit, 0)
	if err != nil {
		s.logger.Error("failed to get watched entity IDs", "user_id", req.UserId, "error", err)
		return nil, err
	}

	ids := make([]int64, 0, len(progressItems))
	for _, p := range progressItems {
		// If media type filter is specified, check it
		if req.MediaType != "" {
			mediaInfo, err := s.mediaRepo.GetByID(ctx, p.MediaID)
			if err != nil {
				continue
			}
			if !matchesMediaType(mediaInfo.Type, req.MediaType) {
				continue
			}
		}
		ids = append(ids, p.MediaID)
	}

	return &pluginv1.EntityIDsResponse{EntityIds: ids}, nil
}

// HasWatchHistory returns whether a user has any watch history.
func (s *ProgressServer) HasWatchHistory(ctx context.Context, req *pluginv1.HasWatchHistoryRequest) (*pluginv1.HasWatchHistoryResponse, error) {
	userID := parseUserID(req.UserId)

	// Check for any watched items
	items, err := s.progressRepo.ListWatchedByUserID(ctx, userID, 1, 0)
	if err != nil {
		s.logger.Error("failed to check watch history", "user_id", req.UserId, "error", err)
		return nil, err
	}

	hasHistory := len(items) > 0
	if !hasHistory {
		// Also check in-progress items
		inProgress, err := s.progressRepo.ListInProgressByUserID(ctx, userID, 1, 0)
		if err == nil {
			hasHistory = len(inProgress) > 0
		}
	}

	return &pluginv1.HasWatchHistoryResponse{HasWatchHistory: hasHistory}, nil
}

// toWatchProgressItem converts a domain progress record to protobuf.
func (s *ProgressServer) toWatchProgressItem(ctx context.Context, p *progress.WatchProgress, mediaTypeFilter string) (*pluginv1.WatchProgressItem, error) {
	mediaInfo, err := s.mediaRepo.GetByID(ctx, p.MediaID)
	if err != nil {
		return nil, err
	}

	// Check media type filter
	if mediaTypeFilter != "" && !matchesMediaType(mediaInfo.Type, mediaTypeFilter) {
		return nil, nil // Skip this item (doesn't match filter)
	}

	return &pluginv1.WatchProgressItem{
		MediaId:         p.MediaID,
		MediaType:       normalizeMediaType(mediaInfo.Type),
		ProgressSeconds: p.ProgressSeconds,
		DurationSeconds: p.DurationSeconds,
		ProgressPercent: p.GetProgressPercentage(),
		IsWatched:       p.IsWatched,
		LastWatchedAt:   p.LastWatchedAt.Unix(),
	}, nil
}

// parseUserID converts a string user ID to int64.
// For now, single-user mode uses "1".
func parseUserID(userID string) int64 {
	if userID == "" {
		return 1 // Default user
	}
	id, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return 1 // Default user on parse error
	}
	return id
}

// matchesMediaType checks if a stored media type matches the filter.
// Handles variations like "tv_episode" matching filter "tv_show".
func matchesMediaType(storedType, filter string) bool {
	if filter == "" {
		return true
	}

	// Exact match
	if storedType == filter {
		return true
	}

	// Map "tv_episode" to match "tv_show" filter (episodes are part of shows)
	if filter == "tv_show" && storedType == "tv_episode" {
		return true
	}

	return false
}

// normalizeMediaType converts internal media types to proto-friendly values.
func normalizeMediaType(mediaType string) string {
	switch mediaType {
	case "tv_episode":
		return "tv_episode"
	case "movie":
		return "movie"
	default:
		return mediaType
	}
}

// Verify interface implementation
var _ pluginv1.HostProgressServer = (*ProgressServer)(nil)
