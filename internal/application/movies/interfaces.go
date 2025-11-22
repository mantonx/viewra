package movies

import (
	"context"

	"github.com/mantonx/viewra/internal/domain/common"
)

// ListMoviesExecutor defines the interface for listing movies
type ListMoviesExecutor interface {
	Execute(ctx context.Context, libraryID int64) (ListMoviesResponse, error)
	ExecuteWithPagination(ctx context.Context, libraryID int64, pagination *common.PaginationParams) (ListMoviesResponse, error)
}

// GetMovieExecutor defines the interface for getting a single movie
type GetMovieExecutor interface {
	Execute(ctx context.Context, id int64) (*MovieResponse, error)
}

// SearchMoviesExecutor defines the interface for searching movies
type SearchMoviesExecutor interface {
	Execute(ctx context.Context, libraryID int64, query string) (ListMoviesResponse, error)
	ExecuteWithPagination(ctx context.Context, libraryID int64, query string, pagination *common.PaginationParams) (ListMoviesResponse, error)
}

// ListMovieIDsExecutor defines the interface for listing movie IDs only
type ListMovieIDsExecutor interface {
	Execute(ctx context.Context, libraryID int64, pagination *common.PaginationParams) (ListIDsResponse, error)
}
