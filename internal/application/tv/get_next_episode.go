package tv

import (
	"context"
	"fmt"
	"sort"

	"github.com/mantonx/viewra/internal/domain/media"
	domainprogress "github.com/mantonx/viewra/internal/domain/progress"
)

// GetNextEpisodeUseCase handles the business logic for getting the next episode to watch for a show
type GetNextEpisodeUseCase struct {
	tvRepo       media.TVRepository
	progressRepo domainprogress.Repository
}

// NewGetNextEpisodeUseCase creates a new instance of GetNextEpisodeUseCase
func NewGetNextEpisodeUseCase(tvRepo media.TVRepository, progressRepo domainprogress.Repository) *GetNextEpisodeUseCase {
	return &GetNextEpisodeUseCase{
		tvRepo:       tvRepo,
		progressRepo: progressRepo,
	}
}

// Execute retrieves the next episode to watch for a show
// Logic:
// 1. If there's an episode in progress (not watched), return it
// 2. Otherwise, find the first unwatched episode in chronological order (season, then episode number)
// 3. If all episodes are watched, return the first episode (S01E01 or earliest)
func (uc *GetNextEpisodeUseCase) Execute(ctx context.Context, showID int64) (*TVEpisodeResponse, error) {
	// Get all episodes for the show
	episodes, err := uc.tvRepo.ListTVEpisodesByShowID(ctx, showID)
	if err != nil {
		return nil, fmt.Errorf("failed to list episodes for show: %w", err)
	}

	if len(episodes) == 0 {
		return nil, fmt.Errorf("no episodes found for show ID %d", showID)
	}

	// Sort episodes by season and episode number
	sort.Slice(episodes, func(i, j int) bool {
		if episodes[i].Season != episodes[j].Season {
			// Handle specials (season 0) - they should come last
			if episodes[i].Season == 0 {
				return false
			}
			if episodes[j].Season == 0 {
				return true
			}
			return episodes[i].Season < episodes[j].Season
		}
		return episodes[i].Episode < episodes[j].Episode
	})

	// Get progress for all episodes
	var inProgressEpisode *media.TVEpisode
	var firstUnwatchedEpisode *media.TVEpisode

	for _, ep := range episodes {
		prog, err := uc.progressRepo.GetByMediaID(ctx, ep.ID)
		if err != nil || prog == nil {
			// No progress - this is unwatched
			if firstUnwatchedEpisode == nil {
				firstUnwatchedEpisode = ep
			}
			continue
		}

		// Check if this episode is in progress (has progress but not marked watched)
		if !prog.IsWatched && prog.ProgressSeconds > 0 {
			// Return the first in-progress episode we find
			if inProgressEpisode == nil {
				inProgressEpisode = ep
			}
		} else if !prog.IsWatched {
			// Episode exists in progress DB but has no progress - treat as unwatched
			if firstUnwatchedEpisode == nil {
				firstUnwatchedEpisode = ep
			}
		}
	}

	// Priority 1: Return in-progress episode
	if inProgressEpisode != nil {
		response := ToTVEpisodeResponse(inProgressEpisode)
		return &response, nil
	}

	// Priority 2: Return first unwatched episode
	if firstUnwatchedEpisode != nil {
		response := ToTVEpisodeResponse(firstUnwatchedEpisode)
		return &response, nil
	}

	// Priority 3: All watched - return first episode
	response := ToTVEpisodeResponse(episodes[0])
	return &response, nil
}
