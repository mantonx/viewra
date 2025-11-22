package noop

import (
	"context"
	"testing"

	"github.com/mantonx/viewra/internal/domain/media"
)

// TestNoOpRepositoriesDoNotPanic verifies that no-op repositories can be safely called
// without causing nil pointer panics, even when Phase 3 features aren't implemented yet.
func TestNoOpRepositoriesDoNotPanic(t *testing.T) {
	ctx := context.Background()

	t.Run("MovieRepository operations don't panic", func(t *testing.T) {
		repo := NewMovieRepository()

		// Create should succeed silently
		err := repo.CreateMovie(ctx, &media.Movie{})
		if err != nil {
			t.Errorf("CreateMovie returned error: %v", err)
		}

		// List operations should return empty slices
		movies, err := repo.ListMoviesByLibrary(ctx, 1)
		if err != nil {
			t.Errorf("ListMoviesByLibrary returned error: %v", err)
		}
		if movies == nil {
			t.Error("ListMoviesByLibrary returned nil, expected empty slice")
		}

		// Search should return empty results
		results, err := repo.SearchMovies(ctx, 1, "test")
		if err != nil {
			t.Errorf("SearchMovies returned error: %v", err)
		}
		if results == nil {
			t.Error("SearchMovies returned nil, expected empty slice")
		}
	})

	t.Run("TVRepository operations don't panic", func(t *testing.T) {
		repo := NewTVRepository()

		// Create should succeed silently
		err := repo.CreateTVEpisode(ctx, &media.TVEpisode{})
		if err != nil {
			t.Errorf("CreateTVEpisode returned error: %v", err)
		}

		// List operations should return empty slices
		episodes, err := repo.ListTVEpisodesByLibrary(ctx, 1)
		if err != nil {
			t.Errorf("ListTVEpisodesByLibrary returned error: %v", err)
		}
		if episodes == nil {
			t.Error("ListTVEpisodesByLibrary returned nil, expected empty slice")
		}
	})

	t.Run("MusicRepository operations don't panic", func(t *testing.T) {
		repo := NewMusicRepository()

		// Create should succeed silently
		err := repo.CreateMusicTrack(ctx, &media.MusicTrack{})
		if err != nil {
			t.Errorf("CreateMusicTrack returned error: %v", err)
		}

		// List operations should return empty slices
		tracks, err := repo.ListMusicTracksByLibrary(ctx, 1)
		if err != nil {
			t.Errorf("ListMusicTracksByLibrary returned error: %v", err)
		}
		if tracks == nil {
			t.Error("ListMusicTracksByLibrary returned nil, expected empty slice")
		}
	})
}

// TestNoOpRepositoriesReturnEmptyResults verifies that query operations
// return empty slices rather than nil to prevent nil pointer issues downstream.
func TestNoOpRepositoriesReturnEmptyResults(t *testing.T) {
	ctx := context.Background()

	t.Run("Movie queries return empty slices", func(t *testing.T) {
		repo := NewMovieRepository()

		movies, _ := repo.ListMoviesByLibrary(ctx, 1)
		if len(movies) != 0 {
			t.Errorf("Expected 0 movies, got %d", len(movies))
		}

		results, _ := repo.SearchMovies(ctx, 1, "test")
		if len(results) != 0 {
			t.Errorf("Expected 0 search results, got %d", len(results))
		}
	})

	t.Run("TV queries return empty slices", func(t *testing.T) {
		repo := NewTVRepository()

		episodes, _ := repo.ListTVEpisodesByLibrary(ctx, 1)
		if len(episodes) != 0 {
			t.Errorf("Expected 0 episodes, got %d", len(episodes))
		}

		byShow, _ := repo.ListTVEpisodesByShow(ctx, 1, "Test Show")
		if len(byShow) != 0 {
			t.Errorf("Expected 0 episodes by show, got %d", len(byShow))
		}
	})

	t.Run("Music queries return empty slices", func(t *testing.T) {
		repo := NewMusicRepository()

		tracks, _ := repo.ListMusicTracksByLibrary(ctx, 1)
		if len(tracks) != 0 {
			t.Errorf("Expected 0 tracks, got %d", len(tracks))
		}

		byAlbum, _ := repo.ListMusicTracksByAlbum(ctx, 1, "Test Album")
		if len(byAlbum) != 0 {
			t.Errorf("Expected 0 tracks by album, got %d", len(byAlbum))
		}

		byArtist, _ := repo.ListMusicTracksByArtist(ctx, 1, "Test Artist")
		if len(byArtist) != 0 {
			t.Errorf("Expected 0 tracks by artist, got %d", len(byArtist))
		}
	})
}
