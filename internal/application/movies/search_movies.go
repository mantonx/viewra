package movies

import (
	"context"
	"fmt"
	"sort"

	"github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
)

// SearchMoviesUseCase handles the business logic for searching movies.
// When a SemanticSearchProvider is configured and available, it uses semantic search
// to find movies by meaning/mood (e.g., "heartwarming", "relaxing"). Otherwise, it
// falls back to standard text-based title search.
type SearchMoviesUseCase struct {
	repo             media.MovieRepository
	semanticProvider SemanticSearchProvider
}

// NewSearchMoviesUseCase creates a new instance of SearchMoviesUseCase
func NewSearchMoviesUseCase(repo media.MovieRepository) *SearchMoviesUseCase {
	return &SearchMoviesUseCase{
		repo: repo,
	}
}

// WithSemanticSearch adds semantic search capability to the use case.
// When semantic search is available, it will be used for queries that don't
// match any titles directly (semantic queries like "heartwarming", "relaxing").
func (uc *SearchMoviesUseCase) WithSemanticSearch(provider SemanticSearchProvider) *SearchMoviesUseCase {
	uc.semanticProvider = provider
	return uc
}

// Execute searches for movies by title
func (uc *SearchMoviesUseCase) Execute(ctx context.Context, libraryID int64, query string) (ListMoviesResponse, error) {
	if query == "" {
		return ListMoviesResponse{}, fmt.Errorf("search query cannot be empty")
	}

	// Try semantic search first if available
	if uc.semanticProvider != nil && uc.semanticProvider.IsAvailable() {
		movies, err := uc.executeSemanticSearch(ctx, libraryID, query, nil)
		if err == nil && len(movies) > 0 {
			return ToListMoviesResponse(movies), nil
		}
		// Fall through to text search if semantic search returns no results
	}

	movies, err := uc.repo.SearchMovies(ctx, libraryID, query)
	if err != nil {
		return ListMoviesResponse{}, fmt.Errorf("failed to search movies: %w", err)
	}

	return ToListMoviesResponse(movies), nil
}

// ExecuteWithPagination searches for movies by title with pagination
func (uc *SearchMoviesUseCase) ExecuteWithPagination(ctx context.Context, libraryID int64, query string, pagination *common.PaginationParams) (ListMoviesResponse, error) {
	if query == "" {
		return ListMoviesResponse{}, fmt.Errorf("search query cannot be empty")
	}

	if pagination == nil {
		pagination = common.DefaultPaginationParams()
	}

	// Try semantic search first if available
	if uc.semanticProvider != nil && uc.semanticProvider.IsAvailable() {
		movies, err := uc.executeSemanticSearch(ctx, libraryID, query, pagination)
		if err == nil && len(movies) > 0 {
			// For semantic search results, we have the full result set, so pagination is applied in-memory
			total := int64(len(movies))

			// Apply pagination to the results
			start := pagination.Offset
			end := pagination.Offset + pagination.Limit
			if start > int(total) {
				start = int(total)
			}
			if end > int(total) {
				end = int(total)
			}
			paginatedMovies := movies[start:end]

			return ToListMoviesResponseWithPagination(paginatedMovies, total, pagination), nil
		}
		// Fall through to text search if semantic search returns no results or fails
	}

	// Standard text-based search
	total, err := uc.repo.CountSearchMoviesByTitle(ctx, libraryID, query)
	if err != nil {
		return ListMoviesResponse{}, fmt.Errorf("failed to count search results: %w", err)
	}

	movies, err := uc.repo.SearchMoviesByTitlePaginated(ctx, libraryID, query, pagination)
	if err != nil {
		return ListMoviesResponse{}, fmt.Errorf("failed to search movies: %w", err)
	}

	return ToListMoviesResponseWithPagination(movies, total, pagination), nil
}

// executeSemanticSearch performs semantic search and hydrates results with full movie data.
func (uc *SearchMoviesUseCase) executeSemanticSearch(ctx context.Context, libraryID int64, query string, pagination *common.PaginationParams) ([]*media.Movie, error) {
	// Determine limit for semantic search
	// We request more results than needed to allow for filtering by libraryID
	limit := 100
	if pagination != nil && pagination.Limit > 0 {
		limit = pagination.Offset + pagination.Limit*2 // Get extra to account for filtering
		if limit < 100 {
			limit = 100
		}
	}

	// Perform semantic search
	results, err := uc.semanticProvider.Search(ctx, query, []string{"movie"}, limit)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	// Create a map to preserve similarity order
	orderMap := make(map[int64]int)
	for i, r := range results {
		if r.EntityType == "movie" {
			orderMap[r.EntityID] = i
		}
	}

	// Fetch full movie data for each result
	var movies []*media.Movie
	for _, result := range results {
		if result.EntityType != "movie" {
			continue
		}

		movie, err := uc.repo.GetMovieByID(ctx, result.EntityID)
		if err != nil {
			continue // Skip movies that can't be found (may have been deleted)
		}

		// Filter by library ID (0 means all libraries)
		if libraryID != 0 && movie.Media.LibraryID != libraryID {
			continue
		}

		movies = append(movies, movie)
	}

	// Sort movies by their original semantic similarity order
	sort.Slice(movies, func(i, j int) bool {
		return orderMap[movies[i].Media.ID] < orderMap[movies[j].Media.ID]
	})

	return movies, nil
}
