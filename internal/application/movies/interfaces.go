package movies

import "context"

// ListMoviesExecutor defines the interface for listing movies
type ListMoviesExecutor interface {
	Execute(ctx context.Context, libraryID int64) (ListMoviesResponse, error)
}

// GetMovieExecutor defines the interface for getting a single movie
type GetMovieExecutor interface {
	Execute(ctx context.Context, id int64) (*MovieResponse, error)
}

// SearchMoviesExecutor defines the interface for searching movies
type SearchMoviesExecutor interface {
	Execute(ctx context.Context, libraryID int64, query string) (ListMoviesResponse, error)
}
