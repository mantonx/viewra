package home

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/home"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// TestRecentlyAddedServiceImpl tests
func TestNewRecentlyAddedService(t *testing.T) {
	movieRepo := mocks.NewMovieRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	s := NewRecentlyAddedService(movieRepo, tvRepo)

	if s == nil {
		t.Fatal("NewRecentlyAddedService returned nil")
	}
}

func TestRecentlyAddedServiceGetRecentlyAdded(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	movieRepo := mocks.NewMovieRepository(t)
	movieRepo.WithMovies(
		&media.Movie{
			Media: media.Media{ID: 1, Title: "Movie 1", CreatedAt: now},
		},
		&media.Movie{
			Media: media.Media{ID: 2, Title: "Movie 2", CreatedAt: yesterday},
		},
	)

	tvRepo := mocks.NewTVRepository(t)
	tvRepo.WithShows(
		media.TVShow{ID: 10, Title: "Show 1", CreatedAt: now.Add(-time.Hour)},
	)

	s := NewRecentlyAddedService(movieRepo, tvRepo)
	items, err := s.GetRecentlyAdded(context.Background(), 10)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
	// Should be sorted by CreatedAt descending
	if items[0].EntityID != 1 {
		t.Errorf("first item should be movie 1 (most recent), got ID %d", items[0].EntityID)
	}
}

func TestRecentlyAddedServiceGetRecentlyAdded_BothErrors(t *testing.T) {
	movieRepo := mocks.NewMovieRepository(t)
	movieRepo.ListRecentlyAddedErr = errors.New("movie error")

	tvRepo := mocks.NewTVRepository(t)
	tvRepo.ListRecentlyAddedShowsErr = errors.New("tv error")

	s := NewRecentlyAddedService(movieRepo, tvRepo)
	_, err := s.GetRecentlyAdded(context.Background(), 10)

	if err == nil {
		t.Error("expected error when both repos fail")
	}
}

func TestRecentlyAddedServiceGetRecentlyAdded_PartialError(t *testing.T) {
	now := time.Now()
	movieRepo := mocks.NewMovieRepository(t)
	movieRepo.ListRecentlyAddedErr = errors.New("movie error")

	tvRepo := mocks.NewTVRepository(t)
	tvRepo.WithShows(media.TVShow{ID: 10, Title: "Show 1", CreatedAt: now})

	s := NewRecentlyAddedService(movieRepo, tvRepo)
	items, err := s.GetRecentlyAdded(context.Background(), 10)

	// Should succeed with partial data
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item (from TV), got %d", len(items))
	}
}

func TestRecentlyAddedServiceGetRecentlyAddedMovies_NilRepo(t *testing.T) {
	s := &RecentlyAddedServiceImpl{movieRepo: nil}
	items, err := s.GetRecentlyAddedMovies(context.Background(), 10)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty slice with nil repo, got %d items", len(items))
	}
}

func TestRecentlyAddedServiceGetRecentlyAddedTVShows_NilRepo(t *testing.T) {
	s := &RecentlyAddedServiceImpl{tvRepo: nil}
	items, err := s.GetRecentlyAddedTVShows(context.Background(), 10)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty slice with nil repo, got %d items", len(items))
	}
}

func TestRecentlyAddedServiceGetRecentlyAddedFull(t *testing.T) {
	now := time.Now()
	movieRepo := mocks.NewMovieRepository(t)
	movieRepo.WithMovies(
		&media.Movie{
			Media: media.Media{ID: 1, Title: "Movie 1", CreatedAt: now, UpdatedAt: now},
			Year:  2024,
		},
	)

	tvRepo := mocks.NewTVRepository(t)
	tvRepo.WithShows(
		media.TVShow{ID: 10, Title: "Show 1", Year: 2023, CreatedAt: now, UpdatedAt: now.Add(-time.Hour)},
	)

	s := NewRecentlyAddedService(movieRepo, tvRepo)
	items, err := s.GetRecentlyAddedFull(context.Background(), 10)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
	// Should be sorted by UpdatedAt descending
	if items[0].Type != "movie" {
		t.Errorf("first item should be movie (most recently updated), got %s", items[0].Type)
	}
}

// TestGenresServiceImpl tests
func TestNewGenresService(t *testing.T) {
	movieRepo := mocks.NewMovieRepository(t)
	s := NewGenresService(movieRepo)

	if s == nil {
		t.Fatal("NewGenresService returned nil")
	}
}

func TestGenresServiceGetDistinctGenres(t *testing.T) {
	movieRepo := mocks.NewMovieRepository(t)
	movieRepo.WithMovies(
		&media.Movie{Media: media.Media{ID: 1}, Genre: []string{"Action", "Comedy"}},
		&media.Movie{Media: media.Media{ID: 2}, Genre: []string{"Drama", "Horror"}},
	)

	s := NewGenresService(movieRepo)

	genres, err := s.GetDistinctGenres(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(genres) == 0 {
		t.Error("expected some genres")
	}
}

func TestGenresServiceGetDistinctGenres_Error(t *testing.T) {
	movieRepo := mocks.NewMovieRepository(t)
	movieRepo.ListDistinctGenresErr = errors.New("db error")

	s := NewGenresService(movieRepo)

	_, err := s.GetDistinctGenres(context.Background(), 10)
	if err == nil {
		t.Error("expected error")
	}
}

// Test helper functions
func TestMovieToMediaItem(t *testing.T) {
	now := time.Now()
	movie := &media.Movie{
		Media: media.Media{
			ID:        123,
			Title:     "Test Movie",
			CreatedAt: now,
		},
		Year: 2024,
	}

	item := movieToMediaItem(movie)

	if item.EntityType != "movie" {
		t.Errorf("EntityType = %q, want %q", item.EntityType, "movie")
	}
	if item.EntityID != 123 {
		t.Errorf("EntityID = %d, want %d", item.EntityID, 123)
	}
	if item.Title != "Test Movie" {
		t.Errorf("Title = %q, want %q", item.Title, "Test Movie")
	}
	if item.Year != 2024 {
		t.Errorf("Year = %d, want %d", item.Year, 2024)
	}
	if item.Poster != "/api/images/movies/123/poster" {
		t.Errorf("Poster = %q, want %q", item.Poster, "/api/images/movies/123/poster")
	}
}

func TestTVShowToMediaItem(t *testing.T) {
	now := time.Now()
	show := &media.TVShow{
		ID:        456,
		Title:     "Test Show",
		Year:      2023,
		CreatedAt: now,
	}

	item := tvShowToMediaItem(show)

	if item.EntityType != "tv_show" {
		t.Errorf("EntityType = %q, want %q", item.EntityType, "tv_show")
	}
	if item.EntityID != 456 {
		t.Errorf("EntityID = %d, want %d", item.EntityID, 456)
	}
	if item.Title != "Test Show" {
		t.Errorf("Title = %q, want %q", item.Title, "Test Show")
	}
	if item.Year != 2023 {
		t.Errorf("Year = %d, want %d", item.Year, 2023)
	}
	if item.Poster != "/api/images/tv_shows/456/poster" {
		t.Errorf("Poster = %q, want %q", item.Poster, "/api/images/tv_shows/456/poster")
	}
}

// TestFavoritesServiceImpl tests - testing only the methods we can without complex mocking
func TestFavoritesServiceGetFavorites_NilRepos(t *testing.T) {
	// Create a favorites service with nil movie/tv repos
	s := &FavoritesServiceImpl{
		movieRepo:   nil,
		tvRepo:      nil,
		ratingsRepo: nil,
	}

	items, err := s.GetFavorites(context.Background(), "user1", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty with nil repos, got %d", len(items))
	}
}

func TestFavoritesServiceGetFavoritesFull_NilRepos(t *testing.T) {
	s := &FavoritesServiceImpl{
		movieRepo:   nil,
		tvRepo:      nil,
		ratingsRepo: nil,
	}

	items, err := s.GetFavoritesFull(context.Background(), "user1", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty with nil repos, got %d", len(items))
	}
}

// MediaItemWithTime tests
func TestMediaItemWithTimeCreation(t *testing.T) {
	now := time.Now()

	item := MediaItemWithTime{
		Type:      "movie",
		CreatedAt: now,
		UpdatedAt: now,
		Progress: &home.MediaProgress{
			Percent:         50,
			PositionSeconds: 1800,
			DurationSeconds: 3600,
		},
	}

	if item.Type != "movie" {
		t.Errorf("Type = %q, want %q", item.Type, "movie")
	}
	if item.Progress == nil {
		t.Fatal("Progress should not be nil")
	}
	if item.Progress.Percent != 50 {
		t.Errorf("Progress.Percent = %d, want 50", item.Progress.Percent)
	}
}

func TestMediaItemWithTimeWithEpisodeContext(t *testing.T) {
	item := MediaItemWithTime{
		Type: "tv_show",
		EpisodeContext: &home.EpisodeContext{
			Season:       1,
			Episode:      5,
			EpisodeTitle: "The One Where It Happens",
			ShowTitle:    "Test Show",
		},
	}

	if item.EpisodeContext == nil {
		t.Fatal("EpisodeContext should not be nil")
	}
	if item.EpisodeContext.Season != 1 {
		t.Errorf("Season = %d, want 1", item.EpisodeContext.Season)
	}
	if item.EpisodeContext.Episode != 5 {
		t.Errorf("Episode = %d, want 5", item.EpisodeContext.Episode)
	}
}
