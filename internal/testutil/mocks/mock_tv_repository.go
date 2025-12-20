package mocks

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
)

// TVRepository is a mock implementation of media.TVRepository for testing.
type TVRepository struct {
	t             testing.TB
	mu            sync.RWMutex
	episodes      map[int64]*media.TVEpisode
	shows         map[int64]media.TVShow   // ShowID -> Show
	seasons       map[int64]media.TVSeason // SeasonID -> Season
	nextEpisodeID int64
	nextShowID    int64
	nextSeasonID  int64

	// Error injection
	CreateErr error
	GetErr    error
	ListErr   error
	UpdateErr error
	SearchErr error
	CountErr  error
}

// NewTVRepository creates a new mock TV repository.
func NewTVRepository(t testing.TB) *TVRepository {
	return &TVRepository{
		t:             t,
		episodes:      make(map[int64]*media.TVEpisode),
		shows:         make(map[int64]media.TVShow),
		seasons:       make(map[int64]media.TVSeason),
		nextEpisodeID: 1,
		nextShowID:    1,
		nextSeasonID:  1,
	}
}

// WithEpisodes pre-populates the mock with TV episodes.
func (r *TVRepository) WithEpisodes(items ...*media.TVEpisode) *TVRepository {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, item := range items {
		r.episodes[item.ID] = item
		if item.ID >= r.nextEpisodeID {
			r.nextEpisodeID = item.ID + 1
		}
	}
	return r
}

// WithShows pre-populates the mock with TV shows.
func (r *TVRepository) WithShows(shows ...media.TVShow) *TVRepository {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, show := range shows {
		r.shows[show.ID] = show
		if show.ID >= r.nextShowID {
			r.nextShowID = show.ID + 1
		}
	}
	return r
}

// WithSeasons pre-populates the mock with TV seasons.
func (r *TVRepository) WithSeasons(seasons ...media.TVSeason) *TVRepository {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, season := range seasons {
		r.seasons[season.ID] = season
		if season.ID >= r.nextSeasonID {
			r.nextSeasonID = season.ID + 1
		}
	}
	return r
}

// WithCreateError injects an error for Create operations.
func (r *TVRepository) WithCreateError(err error) *TVRepository {
	r.CreateErr = err
	return r
}

// Implementation of media.TVRepository interface

func (r *TVRepository) CreateTVEpisode(ctx context.Context, episode *media.TVEpisode) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if episode.ID == 0 {
		episode.ID = r.nextEpisodeID
		r.nextEpisodeID++
	}

	r.episodes[episode.ID] = episode
	return nil
}

func (r *TVRepository) GetTVEpisodeByID(ctx context.Context, id int64) (*media.TVEpisode, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	episode, exists := r.episodes[id]
	if !exists {
		return nil, sql.ErrNoRows
	}

	return episode, nil
}

func (r *TVRepository) ListTVEpisodesByLibrary(ctx context.Context, libraryID int64) ([]*media.TVEpisode, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*media.TVEpisode
	for _, episode := range r.episodes {
		if episode.LibraryID == libraryID {
			result = append(result, episode)
		}
	}

	return result, nil
}

func (r *TVRepository) ListTVEpisodesByShow(ctx context.Context, libraryID int64, showTitle string) ([]*media.TVEpisode, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*media.TVEpisode
	for _, episode := range r.episodes {
		if episode.LibraryID == libraryID && episode.ShowTitle == showTitle {
			result = append(result, episode)
		}
	}

	return result, nil
}

func (r *TVRepository) ListTVEpisodesByShowID(ctx context.Context, showID int64) ([]*media.TVEpisode, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*media.TVEpisode
	for _, episode := range r.episodes {
		// Find episodes that match this show ID through shows map
		for _, show := range r.shows {
			if show.ID == showID && episode.ShowTitle == show.Title {
				result = append(result, episode)
				break
			}
		}
	}

	return result, nil
}

func (r *TVRepository) UpdateTVEpisode(ctx context.Context, episode *media.TVEpisode) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.episodes[episode.ID]; !exists {
		return sql.ErrNoRows
	}

	r.episodes[episode.ID] = episode
	return nil
}

func (r *TVRepository) SearchTVEpisodes(ctx context.Context, libraryID int64, query string) ([]*media.TVEpisode, error) {
	if r.SearchErr != nil {
		return nil, r.SearchErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*media.TVEpisode
	queryLower := strings.ToLower(query)

	for _, episode := range r.episodes {
		if episode.LibraryID == libraryID {
			showMatch := strings.Contains(strings.ToLower(episode.ShowTitle), queryLower)
			titleMatch := strings.Contains(strings.ToLower(episode.Title), queryLower)

			if showMatch || titleMatch {
				result = append(result, episode)
			}
		}
	}

	return result, nil
}

func (r *TVRepository) GetTVShowByTitle(ctx context.Context, libraryID int64, title string) (media.TVShow, error) {
	if r.GetErr != nil {
		return media.TVShow{}, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, show := range r.shows {
		if show.LibraryID == libraryID && show.Title == title {
			return show, nil
		}
	}

	return media.TVShow{}, sql.ErrNoRows
}

func (r *TVRepository) GetTVSeasonByShowAndNumber(ctx context.Context, showID int64, seasonNumber int64) (media.TVSeason, error) {
	if r.GetErr != nil {
		return media.TVSeason{}, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, season := range r.seasons {
		if season.ShowID == showID && season.SeasonNumber == seasonNumber {
			return season, nil
		}
	}

	return media.TVSeason{}, sql.ErrNoRows
}

func (r *TVRepository) CountTVShowsByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	if r.CountErr != nil {
		return 0, r.CountErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int64
	for _, show := range r.shows {
		if show.LibraryID == libraryID {
			count++
		}
	}

	return count, nil
}

func (r *TVRepository) ListTVShowsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]media.TVShowWithCounts, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect all shows for this library with counts
	var allShows []media.TVShowWithCounts
	for _, show := range r.shows {
		if show.LibraryID == libraryID {
			// Count seasons and episodes for this show
			var seasonCount, episodeCount int64
			for _, season := range r.seasons {
				if season.ShowID == show.ID {
					seasonCount++
				}
			}
			for _, episode := range r.episodes {
				if episode.ShowTitle == show.Title && episode.LibraryID == libraryID {
					episodeCount++
				}
			}

			allShows = append(allShows, media.TVShowWithCounts{
				TVShow:       show,
				SeasonCount:  seasonCount,
				EpisodeCount: episodeCount,
			})
		}
	}

	// Apply pagination
	start := int(pagination.Offset)
	end := start + int(pagination.Limit)

	if start >= len(allShows) {
		return []media.TVShowWithCounts{}, nil
	}

	if end > len(allShows) {
		end = len(allShows)
	}

	return allShows[start:end], nil
}

func (r *TVRepository) CountSearchTVShowsByTitle(ctx context.Context, libraryID int64, query string) (int64, error) {
	if r.CountErr != nil {
		return 0, r.CountErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int64
	queryLower := strings.ToLower(query)

	for _, show := range r.shows {
		if show.LibraryID == libraryID {
			if strings.Contains(strings.ToLower(show.Title), queryLower) {
				count++
			}
		}
	}

	return count, nil
}

func (r *TVRepository) SearchTVShowsByTitlePaginated(ctx context.Context, libraryID int64, query string, pagination *common.PaginationParams) ([]media.TVShow, error) {
	if r.SearchErr != nil {
		return nil, r.SearchErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect all matching shows
	var matchingShows []media.TVShow
	queryLower := strings.ToLower(query)

	for _, show := range r.shows {
		if show.LibraryID == libraryID {
			if strings.Contains(strings.ToLower(show.Title), queryLower) {
				matchingShows = append(matchingShows, show)
			}
		}
	}

	// Apply pagination
	start := int(pagination.Offset)
	end := start + int(pagination.Limit)

	if start >= len(matchingShows) {
		return []media.TVShow{}, nil
	}

	if end > len(matchingShows) {
		end = len(matchingShows)
	}

	return matchingShows[start:end], nil
}

func (r *TVRepository) ListTVShowIDsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]int64, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect all show IDs for this library
	var allIDs []int64
	for _, show := range r.shows {
		if show.LibraryID == libraryID {
			allIDs = append(allIDs, show.ID)
		}
	}

	// Apply pagination
	start := int(pagination.Offset)
	end := start + int(pagination.Limit)

	if start >= len(allIDs) {
		return []int64{}, nil
	}

	if end > len(allIDs) {
		end = len(allIDs)
	}

	return allIDs[start:end], nil
}

// GetTVShowByID retrieves a TV show by its ID.
func (r *TVRepository) GetTVShowByID(ctx context.Context, id int64) (media.TVShow, error) {
	if r.GetErr != nil {
		return media.TVShow{}, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	show, exists := r.shows[id]
	if !exists {
		return media.TVShow{}, sql.ErrNoRows
	}

	return show, nil
}

// UpdateTVShow updates an existing TV show's metadata.
func (r *TVRepository) UpdateTVShow(ctx context.Context, show media.TVShow) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.shows[show.ID]; !exists {
		return sql.ErrNoRows
	}

	r.shows[show.ID] = show
	return nil
}

// SearchTVShowsWithCountsByTitlePaginated searches for TV shows by title with pagination and includes counts.
func (r *TVRepository) SearchTVShowsWithCountsByTitlePaginated(ctx context.Context, libraryID int64, query string, pagination *common.PaginationParams) ([]media.TVShowWithCounts, error) {
	if r.SearchErr != nil {
		return nil, r.SearchErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect all matching shows with counts
	var matchingShows []media.TVShowWithCounts
	queryLower := strings.ToLower(query)

	for _, show := range r.shows {
		if show.LibraryID == libraryID {
			if strings.Contains(strings.ToLower(show.Title), queryLower) {
				// Count seasons and episodes for this show
				var seasonCount, episodeCount int64
				for _, season := range r.seasons {
					if season.ShowID == show.ID {
						seasonCount++
					}
				}
				for _, episode := range r.episodes {
					if episode.ShowTitle == show.Title && episode.LibraryID == libraryID {
						episodeCount++
					}
				}

				matchingShows = append(matchingShows, media.TVShowWithCounts{
					TVShow:       show,
					SeasonCount:  seasonCount,
					EpisodeCount: episodeCount,
				})
			}
		}
	}

	// Apply pagination
	start := int(pagination.Offset)
	end := start + int(pagination.Limit)

	if start >= len(matchingShows) {
		return []media.TVShowWithCounts{}, nil
	}

	if end > len(matchingShows) {
		end = len(matchingShows)
	}

	return matchingShows[start:end], nil
}

// ListTVSeasonsByShow retrieves all seasons for a specific show.
func (r *TVRepository) ListTVSeasonsByShow(ctx context.Context, showID int64) ([]media.TVSeason, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []media.TVSeason
	for _, season := range r.seasons {
		if season.ShowID == showID {
			result = append(result, season)
		}
	}

	return result, nil
}
