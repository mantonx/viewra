package media

import (
	"context"
	"fmt"
	"strings"

	"github.com/mantonx/viewra/internal/domain/media"
)

// SearchMediaUseCase handles the business logic for searching media
type SearchMediaUseCase struct {
	repo      media.Repository
	movieRepo media.MovieRepository
	tvRepo    media.TVRepository
	musicRepo media.MusicRepository
}

// NewSearchMediaUseCase creates a new instance of SearchMediaUseCase
func NewSearchMediaUseCase(
	repo media.Repository,
	movieRepo media.MovieRepository,
	tvRepo media.TVRepository,
	musicRepo media.MusicRepository,
) *SearchMediaUseCase {
	return &SearchMediaUseCase{
		repo:      repo,
		movieRepo: movieRepo,
		tvRepo:    tvRepo,
		musicRepo: musicRepo,
	}
}

// SearchMovies searches for movies by title
func (uc *SearchMediaUseCase) SearchMovies(ctx context.Context, libraryID int64, query string) (ListMoviesResponse, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return ListMoviesResponse{}, fmt.Errorf("search query cannot be empty")
	}

	movies, err := uc.movieRepo.SearchMovies(ctx, libraryID, query)
	if err != nil {
		return ListMoviesResponse{}, fmt.Errorf("failed to search movies: %w", err)
	}

	return ToListMoviesResponse(movies), nil
}

// SearchTVEpisodes searches for TV episodes by show title or episode title
func (uc *SearchMediaUseCase) SearchTVEpisodes(ctx context.Context, libraryID int64, query string) (ListTVEpisodesResponse, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return ListTVEpisodesResponse{}, fmt.Errorf("search query cannot be empty")
	}

	episodes, err := uc.tvRepo.SearchTVEpisodes(ctx, libraryID, query)
	if err != nil {
		return ListTVEpisodesResponse{}, fmt.Errorf("failed to search TV episodes: %w", err)
	}

	return ToListTVEpisodesResponse(episodes), nil
}

// SearchMusicTracks searches for music tracks by title, artist, or album
func (uc *SearchMediaUseCase) SearchMusicTracks(ctx context.Context, libraryID int64, query string) (ListMusicTracksResponse, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return ListMusicTracksResponse{}, fmt.Errorf("search query cannot be empty")
	}

	tracks, err := uc.musicRepo.SearchMusicTracks(ctx, libraryID, query)
	if err != nil {
		return ListMusicTracksResponse{}, fmt.Errorf("failed to search music tracks: %w", err)
	}

	return ToListMusicTracksResponse(tracks), nil
}
