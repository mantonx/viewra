package sdk

import (
	"context"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"google.golang.org/grpc"
)

// ProgressClient wraps the HostProgress service for reading user watch history.
// Plugins use this to access watch progress for collaborative filtering
// and personalized recommendations.
type ProgressClient struct {
	client pluginv1.HostProgressClient
}

// NewProgressClient creates a new progress client.
func NewProgressClient(conn *grpc.ClientConn) *ProgressClient {
	return &ProgressClient{client: pluginv1.NewHostProgressClient(conn)}
}

// WatchProgress represents a user's watch progress for a media item.
type WatchProgress struct {
	MediaID         int64
	MediaType       string  // "movie", "tv_episode"
	ProgressSeconds float64 // Current playback position
	DurationSeconds float64 // Total duration
	ProgressPercent float64 // Progress as percentage (0-100)
	IsWatched       bool    // True if marked as watched
	LastWatchedAt   time.Time
}

// ListWatchedItems returns all watched items for a user.
// Optional filter: mediaType (empty for all types).
func (c *ProgressClient) ListWatchedItems(ctx context.Context, userID, mediaType string, limit, offset int) ([]*WatchProgress, error) {
	if limit <= 0 {
		limit = 100
	}
	resp, err := c.client.ListWatchedItems(ctx, &pluginv1.ListWatchedItemsRequest{
		UserId:    userID,
		MediaType: mediaType,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		return nil, err
	}

	items := make([]*WatchProgress, len(resp.Items))
	for i, item := range resp.Items {
		items[i] = protoToWatchProgress(item)
	}
	return items, nil
}

// ListInProgressItems returns items the user is currently watching (partial progress).
// Optional filter: mediaType (empty for all types).
func (c *ProgressClient) ListInProgressItems(ctx context.Context, userID, mediaType string, limit, offset int) ([]*WatchProgress, error) {
	if limit <= 0 {
		limit = 100
	}
	resp, err := c.client.ListInProgressItems(ctx, &pluginv1.ListInProgressItemsRequest{
		UserId:    userID,
		MediaType: mediaType,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		return nil, err
	}

	items := make([]*WatchProgress, len(resp.Items))
	for i, item := range resp.Items {
		items[i] = protoToWatchProgress(item)
	}
	return items, nil
}

// GetWatchedEntityIDs returns just the entity IDs of watched items.
// More efficient than ListWatchedItems when only IDs are needed.
// Optional filter: mediaType (empty for all types).
func (c *ProgressClient) GetWatchedEntityIDs(ctx context.Context, userID, mediaType string, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 1000
	}
	resp, err := c.client.GetWatchedEntityIDs(ctx, &pluginv1.GetWatchedEntityIDsRequest{
		UserId:    userID,
		MediaType: mediaType,
		Limit:     int32(limit),
	})
	if err != nil {
		return nil, err
	}
	return resp.EntityIds, nil
}

// HasWatchHistory returns whether a user has any watch history.
func (c *ProgressClient) HasWatchHistory(ctx context.Context, userID string) (bool, error) {
	resp, err := c.client.HasWatchHistory(ctx, &pluginv1.HasWatchHistoryRequest{
		UserId: userID,
	})
	if err != nil {
		return false, err
	}
	return resp.HasWatchHistory, nil
}

// protoToWatchProgress converts a protobuf WatchProgressItem to SDK WatchProgress.
func protoToWatchProgress(item *pluginv1.WatchProgressItem) *WatchProgress {
	return &WatchProgress{
		MediaID:         item.MediaId,
		MediaType:       item.MediaType,
		ProgressSeconds: item.ProgressSeconds,
		DurationSeconds: item.DurationSeconds,
		ProgressPercent: item.ProgressPercent,
		IsWatched:       item.IsWatched,
		LastWatchedAt:   time.Unix(item.LastWatchedAt, 0),
	}
}
