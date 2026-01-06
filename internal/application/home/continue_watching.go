package home

import (
	"context"
	"fmt"
	"sort"

	"github.com/mantonx/viewra/internal/application/movies"
	"github.com/mantonx/viewra/internal/application/tv"
	"github.com/mantonx/viewra/internal/domain/home"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/progress"
)

// ContinueWatchingServiceImpl implements ContinueWatchingService.
type ContinueWatchingServiceImpl struct {
	progressRepo progress.Repository
	mediaRepo    media.Repository
	movieRepo    media.MovieRepository
	tvRepo       media.TVRepository
}

// NewContinueWatchingService creates a new continue watching service.
func NewContinueWatchingService(
	progressRepo progress.Repository,
	mediaRepo media.Repository,
	movieRepo media.MovieRepository,
	tvRepo media.TVRepository,
) *ContinueWatchingServiceImpl {
	return &ContinueWatchingServiceImpl{
		progressRepo: progressRepo,
		mediaRepo:    mediaRepo,
		movieRepo:    movieRepo,
		tvRepo:       tvRepo,
	}
}

// HasHistory returns true if the user has watch history.
func (s *ContinueWatchingServiceImpl) HasHistory(ctx context.Context, userID string) bool {
	// Convert string userID to int64 (for now, single-user mode uses "1")
	uid := parseUserID(userID)
	items, err := s.progressRepo.ListInProgressByUserID(ctx, uid, 1, 0)
	return err == nil && len(items) > 0
}

// GetContinueWatchingFull returns items the user is currently watching with full typed data.
func (s *ContinueWatchingServiceImpl) GetContinueWatchingFull(ctx context.Context, userID string, limit int) ([]MediaItemWithTime, error) {
	uid := parseUserID(userID)

	progressItems, err := s.progressRepo.ListInProgressByUserID(ctx, uid, limit*2, 0) // Fetch extra in case some fail
	if err != nil {
		return nil, fmt.Errorf("list in-progress: %w", err)
	}

	items := make([]MediaItemWithTime, 0, len(progressItems))
	seenShows := make(map[int64]bool) // Track shows we've already added to avoid duplicates

	for _, p := range progressItems {
		mediaInfo, err := s.mediaRepo.GetByID(ctx, p.MediaID)
		if err != nil {
			// Skip items where media can't be found
			continue
		}

		// Build progress info
		positionSecs := int(p.ProgressSeconds)
		durationSecs := int(p.DurationSeconds)
		progressInfo := &home.MediaProgress{
			Percent:         CalculateProgressPercent(positionSecs, durationSecs),
			PositionSeconds: positionSecs,
			DurationSeconds: durationSecs,
			RemainingText:   FormatRemainingTime(positionSecs, durationSecs),
		}

		switch mediaInfo.Type {
		case "movie":
			if s.movieRepo != nil {
				movie, err := s.movieRepo.GetMovieByID(ctx, p.MediaID)
				if err == nil {
					resp := movies.ToMovieResponse(movie)
					items = append(items, MediaItemWithTime{
						Type:      "movie",
						Movie:     &resp,
						CreatedAt: p.UpdatedAt,
						Progress:  progressInfo,
					})
				}
			}
		case "tv_episode":
			// For TV episodes, we need to get the show and episode details
			if s.tvRepo != nil {
				episode, err := s.tvRepo.GetTVEpisodeByID(ctx, p.MediaID)
				if err == nil && episode != nil {
					// Skip if we've already added this show
					if seenShows[episode.ShowID] {
						continue
					}
					seenShows[episode.ShowID] = true

					show, err := s.tvRepo.GetTVShowByID(ctx, episode.ShowID)
					if err == nil {
						summary := tv.ToTVShowSummary(&show)

						// Build episode context
						episodeContext := &home.EpisodeContext{
							Season:         episode.Season,
							Episode:        episode.Episode,
							EpisodeTitle:   episode.EpisodeTitle,
							ShowTitle:      show.Title,
							EpisodeMediaID: p.MediaID,
						}

						items = append(items, MediaItemWithTime{
							Type:           "tv_show",
							TVShow:         &summary,
							CreatedAt:      p.UpdatedAt,
							Progress:       progressInfo,
							EpisodeContext: episodeContext,
						})
					}
				}
			}
		}

		if len(items) >= limit {
			break
		}
	}

	// Sort by last watched time (most recent first)
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	if len(items) > limit {
		items = items[:limit]
	}

	return items, nil
}

// parseUserID converts a string user ID to int64.
// Returns 1 as default for single-user mode.
func parseUserID(userID string) int64 {
	if userID == "" || userID == "1" || userID == "anonymous" {
		return 1
	}
	// For future multi-user support, parse the actual ID
	var id int64
	fmt.Sscanf(userID, "%d", &id)
	if id == 0 {
		return 1
	}
	return id
}
