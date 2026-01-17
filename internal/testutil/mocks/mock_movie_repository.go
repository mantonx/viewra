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

// MovieRepository is a mock implementation of media.MovieRepository for testing.
type MovieRepository struct {
	t      testing.TB
	mu     sync.RWMutex
	movies map[int64]*media.Movie
	nextID int64

	// Error injection
	CreateErr             error
	GetErr                error
	ListErr               error
	UpdateErr             error
	SearchErr             error
	CountErr              error
	ListRecentlyAddedErr  error
	ListDistinctGenresErr error
}

// NewMovieRepository creates a new mock movie repository.
func NewMovieRepository(t testing.TB) *MovieRepository {
	return &MovieRepository{
		t:      t,
		movies: make(map[int64]*media.Movie),
		nextID: 1,
	}
}

// WithMovies pre-populates the mock with movie items.
func (m *MovieRepository) WithMovies(items ...*media.Movie) *MovieRepository {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, item := range items {
		m.movies[item.ID] = item
		if item.ID >= m.nextID {
			m.nextID = item.ID + 1
		}
	}
	return m
}

// WithCreateError injects an error for CreateMovie operations.
func (m *MovieRepository) WithCreateError(err error) *MovieRepository {
	m.CreateErr = err
	return m
}

// WithGetError injects an error for GetMovieByID operations.
func (m *MovieRepository) WithGetError(err error) *MovieRepository {
	m.GetErr = err
	return m
}

// WithUpdateError injects an error for UpdateMovie operations.
func (m *MovieRepository) WithUpdateError(err error) *MovieRepository {
	m.UpdateErr = err
	return m
}

// AssertMovieCount verifies the number of movies in the repository.
func (m *MovieRepository) AssertMovieCount(expected int) {
	m.t.Helper()
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.movies) != expected {
		m.t.Errorf("expected %d movies, got %d", expected, len(m.movies))
	}
}

// MovieExists checks if a movie exists by ID.
func (m *MovieRepository) MovieExists(id int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.movies[id]
	return exists
}

// Implementation of media.MovieRepository interface

func (m *MovieRepository) CreateMovie(ctx context.Context, movie *media.Movie) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if movie.ID == 0 {
		movie.ID = m.nextID
		m.nextID++
	}

	m.movies[movie.ID] = movie
	return nil
}

func (m *MovieRepository) GetMovieByID(ctx context.Context, id int64) (*media.Movie, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	movie, exists := m.movies[id]
	if !exists {
		return nil, sql.ErrNoRows
	}

	return movie, nil
}

func (m *MovieRepository) ListMoviesByLibrary(ctx context.Context, libraryID int64) ([]*media.Movie, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*media.Movie
	for _, movie := range m.movies {
		if movie.LibraryID == libraryID {
			result = append(result, movie)
		}
	}

	return result, nil
}

func (m *MovieRepository) UpdateMovie(ctx context.Context, movie *media.Movie) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.movies[movie.ID]; !exists {
		return sql.ErrNoRows
	}

	m.movies[movie.ID] = movie
	return nil
}

func (m *MovieRepository) SearchMovies(ctx context.Context, libraryID int64, query string) ([]*media.Movie, error) {
	if m.SearchErr != nil {
		return nil, m.SearchErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*media.Movie
	queryLower := strings.ToLower(query)

	for _, movie := range m.movies {
		if movie.LibraryID == libraryID {
			if strings.Contains(strings.ToLower(movie.Title), queryLower) {
				result = append(result, movie)
			}
		}
	}

	return result, nil
}

func (m *MovieRepository) CountMoviesByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	if m.CountErr != nil {
		return 0, m.CountErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var count int64
	for _, movie := range m.movies {
		if movie.LibraryID == libraryID {
			count++
		}
	}

	return count, nil
}

func (m *MovieRepository) ListMoviesByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]*media.Movie, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect all movies for this library
	var allMovies []*media.Movie
	for _, movie := range m.movies {
		if movie.LibraryID == libraryID {
			allMovies = append(allMovies, movie)
		}
	}

	// Apply pagination
	start := int(pagination.Offset)
	end := start + int(pagination.Limit)

	if start >= len(allMovies) {
		return []*media.Movie{}, nil
	}

	if end > len(allMovies) {
		end = len(allMovies)
	}

	return allMovies[start:end], nil
}

func (m *MovieRepository) CountSearchMoviesByTitle(ctx context.Context, libraryID int64, query string) (int64, error) {
	if m.CountErr != nil {
		return 0, m.CountErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var count int64
	queryLower := strings.ToLower(query)

	for _, movie := range m.movies {
		if movie.LibraryID == libraryID {
			if strings.Contains(strings.ToLower(movie.Title), queryLower) {
				count++
			}
		}
	}

	return count, nil
}

func (m *MovieRepository) SearchMoviesByTitlePaginated(ctx context.Context, libraryID int64, query string, pagination *common.PaginationParams) ([]*media.Movie, error) {
	if m.SearchErr != nil {
		return nil, m.SearchErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect all matching movies
	var matchingMovies []*media.Movie
	queryLower := strings.ToLower(query)

	for _, movie := range m.movies {
		if movie.LibraryID == libraryID {
			if strings.Contains(strings.ToLower(movie.Title), queryLower) {
				matchingMovies = append(matchingMovies, movie)
			}
		}
	}

	// Apply pagination
	start := int(pagination.Offset)
	end := start + int(pagination.Limit)

	if start >= len(matchingMovies) {
		return []*media.Movie{}, nil
	}

	if end > len(matchingMovies) {
		end = len(matchingMovies)
	}

	return matchingMovies[start:end], nil
}

func (m *MovieRepository) ListMovieIDsByLibraryPaginated(ctx context.Context, libraryID int64, pagination *common.PaginationParams) ([]int64, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect all movie IDs for this library
	var allIDs []int64
	for _, movie := range m.movies {
		if movie.LibraryID == libraryID {
			allIDs = append(allIDs, movie.ID)
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

func (m *MovieRepository) ListRecentlyAdded(ctx context.Context, limit int) ([]*media.Movie, error) {
	if m.ListRecentlyAddedErr != nil {
		return nil, m.ListRecentlyAddedErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect all movies and return up to limit
	var result []*media.Movie
	for _, movie := range m.movies {
		result = append(result, movie)
		if len(result) >= limit {
			break
		}
	}

	return result, nil
}

func (m *MovieRepository) ListDistinctGenres(ctx context.Context, limit int) ([]string, error) {
	if m.ListDistinctGenresErr != nil {
		return nil, m.ListDistinctGenresErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Collect distinct genres from all movies
	genreSet := make(map[string]struct{})
	for _, movie := range m.movies {
		for _, genre := range movie.Genre {
			genreSet[genre] = struct{}{}
		}
	}

	var result []string
	for genre := range genreSet {
		result = append(result, genre)
		if len(result) >= limit {
			break
		}
	}

	return result, nil
}
