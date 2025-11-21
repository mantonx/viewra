package progress

import (
	"context"

	"github.com/viewra/viewra/internal/domain/progress"
)

// GetBatchProgressByMediaIDs retrieves watch progress for multiple media items.
// Returns a map of media_id -> progress. Missing media IDs won't be in the map.
func GetBatchProgressByMediaIDs(ctx context.Context, repo progress.Repository, mediaIDs []int64, userID int64) (*BatchProgressResponse, error) {
	if len(mediaIDs) == 0 {
		return &BatchProgressResponse{Progress: make(map[int64]*WatchProgressResponse)}, nil
	}

	// Default to user_id = 1 for single-user mode
	if userID == 0 {
		userID = 1
	}

	progressMap, err := repo.GetBatchByMediaIDs(ctx, mediaIDs, userID)
	if err != nil {
		return nil, err
	}

	// Convert domain entities to DTOs
	responseMap := make(map[int64]*WatchProgressResponse, len(progressMap))
	for mediaID, prog := range progressMap {
		responseMap[mediaID] = toResponse(prog)
	}

	return &BatchProgressResponse{Progress: responseMap}, nil
}
