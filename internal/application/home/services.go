package home

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/mantonx/viewra/internal/application/movies"
	"github.com/mantonx/viewra/internal/application/tv"
	"github.com/mantonx/viewra/internal/domain/home"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/ratings"
)

// MediaItemWithTime wraps a typed media item with its creation time for sorting.
type MediaItemWithTime struct {
	Type      string // "movie" or "tv_show"
	Movie     *movies.MovieResponse
	TVShow    *tv.TVShowSummary
	CreatedAt time.Time
	UpdatedAt time.Time

	// Progress contains playback progress (for continue watching).
	Progress *home.MediaProgress

	// EpisodeContext contains episode details for TV shows (for continue watching).
	EpisodeContext *home.EpisodeContext
}

// RecentlyAddedServiceImpl implements RecentlyAddedService.
type RecentlyAddedServiceImpl struct {
	movieRepo media.MovieRepository
	tvRepo    media.TVRepository
}

// NewRecentlyAddedService creates a new recently added service.
func NewRecentlyAddedService(movieRepo media.MovieRepository, tvRepo media.TVRepository) *RecentlyAddedServiceImpl {
	return &RecentlyAddedServiceImpl{
		movieRepo: movieRepo,
		tvRepo:    tvRepo,
	}
}

// GetRecentlyAdded returns recently added media items (movies and TV shows combined).
func (s *RecentlyAddedServiceImpl) GetRecentlyAdded(ctx context.Context, limit int) ([]*home.MediaItem, error) {
	// Fetch from both - errors are non-fatal, we return what we can get
	movieItems, moviesErr := s.GetRecentlyAddedMovies(ctx, limit)
	showItems, showsErr := s.GetRecentlyAddedTVShows(ctx, limit)

	// If both failed, return the first error
	if moviesErr != nil && showsErr != nil {
		return nil, moviesErr
	}

	// Combine and sort by CreatedAt descending
	items := make([]*home.MediaItem, 0, len(movieItems)+len(showItems))
	items = append(items, movieItems...)
	items = append(items, showItems...)

	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	if len(items) > limit {
		items = items[:limit]
	}

	return items, nil
}

// GetRecentlyAddedMovies returns recently added movies only.
func (s *RecentlyAddedServiceImpl) GetRecentlyAddedMovies(ctx context.Context, limit int) ([]*home.MediaItem, error) {
	if s.movieRepo == nil {
		return []*home.MediaItem{}, nil
	}

	movieList, err := s.movieRepo.ListRecentlyAdded(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list recently added movies: %w", err)
	}

	items := make([]*home.MediaItem, 0, len(movieList))
	for _, m := range movieList {
		items = append(items, movieToMediaItem(m))
	}
	return items, nil
}

// GetRecentlyAddedTVShows returns recently added TV shows only.
func (s *RecentlyAddedServiceImpl) GetRecentlyAddedTVShows(ctx context.Context, limit int) ([]*home.MediaItem, error) {
	if s.tvRepo == nil {
		return []*home.MediaItem{}, nil
	}

	shows, err := s.tvRepo.ListRecentlyAddedShows(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list recently added TV shows: %w", err)
	}

	items := make([]*home.MediaItem, 0, len(shows))
	for _, show := range shows {
		items = append(items, tvShowToMediaItem(&show))
	}
	return items, nil
}

// GetRecentlyAddedFull returns recently added media with full typed data for frontend components.
func (s *RecentlyAddedServiceImpl) GetRecentlyAddedFull(ctx context.Context, limit int) ([]MediaItemWithTime, error) {
	items := make([]MediaItemWithTime, 0, limit*2)

	// Fetch movies
	if s.movieRepo != nil {
		movieList, err := s.movieRepo.ListRecentlyAdded(ctx, limit)
		if err == nil {
			for _, m := range movieList {
				resp := movies.ToMovieResponse(m)
				items = append(items, MediaItemWithTime{
					Type:      "movie",
					Movie:     &resp,
					CreatedAt: m.CreatedAt,
					UpdatedAt: m.Media.UpdatedAt,
				})
			}
		}
	}

	// Fetch TV shows
	if s.tvRepo != nil {
		shows, err := s.tvRepo.ListRecentlyAddedShows(ctx, limit)
		if err == nil {
			for _, show := range shows {
				summary := tv.ToTVShowSummary(&show)
				items = append(items, MediaItemWithTime{
					Type:      "tv_show",
					TVShow:    &summary,
					CreatedAt: show.CreatedAt,
					UpdatedAt: show.UpdatedAt,
				})
			}
		}
	}

	// Sort by UpdatedAt descending (most recently updated first)
	// This includes both new additions AND re-scanned/re-enriched items
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	if len(items) > limit {
		items = items[:limit]
	}

	return items, nil
}

// FavoritesServiceImpl implements FavoritesService.
type FavoritesServiceImpl struct {
	movieRepo   media.MovieRepository
	tvRepo      media.TVRepository
	ratingsRepo ratings.Repository
}

// NewFavoritesService creates a new favorites service.
func NewFavoritesService(
	movieRepo media.MovieRepository,
	tvRepo media.TVRepository,
	ratingsRepo ratings.Repository,
) *FavoritesServiceImpl {
	return &FavoritesServiceImpl{
		movieRepo:   movieRepo,
		tvRepo:      tvRepo,
		ratingsRepo: ratingsRepo,
	}
}

// HasFavorites returns true if the user has any favorites.
func (s *FavoritesServiceImpl) HasFavorites(ctx context.Context, userID string) bool {
	ids, err := s.ratingsRepo.ListByRating(ctx, userID, "", "favorite", 1)
	return err == nil && len(ids) > 0
}

// GetFavorites returns the user's favorite media items.
func (s *FavoritesServiceImpl) GetFavorites(ctx context.Context, userID string, limit int) ([]*home.MediaItem, error) {
	items := make([]*home.MediaItem, 0, limit)

	// Fetch favorite movies
	if s.movieRepo != nil {
		movieIDs, err := s.ratingsRepo.ListByRating(ctx, userID, "movie", "favorite", limit)
		if err == nil {
			for _, id := range movieIDs {
				movie, err := s.movieRepo.GetMovieByID(ctx, id)
				if err == nil {
					items = append(items, movieToMediaItem(movie))
				}
			}
		}
	}

	// Fetch favorite TV shows
	if s.tvRepo != nil {
		showIDs, err := s.ratingsRepo.ListByRating(ctx, userID, "tv_show", "favorite", limit)
		if err == nil {
			for _, id := range showIDs {
				show, err := s.tvRepo.GetTVShowByID(ctx, id)
				if err == nil {
					items = append(items, tvShowToMediaItem(&show))
				}
			}
		}
	}

	if len(items) > limit {
		items = items[:limit]
	}

	return items, nil
}

// GetFavoritesFull returns favorite media with full typed data.
func (s *FavoritesServiceImpl) GetFavoritesFull(ctx context.Context, userID string, limit int) ([]MediaItemWithTime, error) {
	items := make([]MediaItemWithTime, 0, limit)

	// Fetch favorite movies
	if s.movieRepo != nil {
		movieIDs, err := s.ratingsRepo.ListByRating(ctx, userID, "movie", "favorite", limit)
		if err == nil {
			for _, id := range movieIDs {
				movie, err := s.movieRepo.GetMovieByID(ctx, id)
				if err == nil {
					resp := movies.ToMovieResponse(movie)
					items = append(items, MediaItemWithTime{
						Type:      "movie",
						Movie:     &resp,
						CreatedAt: movie.CreatedAt,
					})
				}
			}
		}
	}

	// Fetch favorite TV shows
	if s.tvRepo != nil {
		showIDs, err := s.ratingsRepo.ListByRating(ctx, userID, "tv_show", "favorite", limit)
		if err == nil {
			for _, id := range showIDs {
				show, err := s.tvRepo.GetTVShowByID(ctx, id)
				if err == nil {
					summary := tv.ToTVShowSummary(&show)
					items = append(items, MediaItemWithTime{
						Type:      "tv_show",
						TVShow:    &summary,
						CreatedAt: show.CreatedAt,
					})
				}
			}
		}
	}

	if len(items) > limit {
		items = items[:limit]
	}

	return items, nil
}

// GenresServiceImpl implements GenresService using the movie repository.
type GenresServiceImpl struct {
	movieRepo media.MovieRepository
}

// NewGenresService creates a new genres service.
func NewGenresService(movieRepo media.MovieRepository) *GenresServiceImpl {
	return &GenresServiceImpl{movieRepo: movieRepo}
}

// GetDistinctGenres returns distinct genres from the library.
func (s *GenresServiceImpl) GetDistinctGenres(ctx context.Context, limit int) ([]string, error) {
	return s.movieRepo.ListDistinctGenres(ctx, limit)
}

// movieToMediaItem converts a domain Movie to a home MediaItem.
func movieToMediaItem(m *media.Movie) *home.MediaItem {
	return &home.MediaItem{
		EntityType: "movie",
		EntityID:   m.Media.ID,
		Title:      m.Media.Title,
		Year:       m.Year,
		Poster:     fmt.Sprintf("/api/images/movies/%d/poster", m.Media.ID),
		CreatedAt:  m.Media.CreatedAt,
	}
}

// tvShowToMediaItem converts a domain TVShow to a home MediaItem.
func tvShowToMediaItem(s *media.TVShow) *home.MediaItem {
	return &home.MediaItem{
		EntityType: "tv_show",
		EntityID:   s.ID,
		Title:      s.Title,
		Year:       s.Year,
		Poster:     fmt.Sprintf("/api/images/tv_shows/%d/poster", s.ID),
		CreatedAt:  s.CreatedAt,
	}
}
