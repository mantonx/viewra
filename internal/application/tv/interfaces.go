package tv

import (
	"context"

	"github.com/viewra/viewra/internal/domain/common"
)

// ListTVShowsExecutor defines the interface for listing TV shows
type ListTVShowsExecutor interface {
	Execute(ctx context.Context, libraryID int64) (ListTVShowsResponse, error)
	ExecuteWithPagination(ctx context.Context, libraryID int64, pagination *common.PaginationParams) (ListTVShowsResponse, error)
}

// GetTVShowExecutor defines the interface for getting a single TV show
type GetTVShowExecutor interface {
	Execute(ctx context.Context, showID int64) (*TVShowDetailResponse, error)
}

// ListTVEpisodesExecutor defines the interface for listing TV episodes
type ListTVEpisodesExecutor interface {
	ExecuteByShow(ctx context.Context, libraryID int64, showTitle string) (ListTVEpisodesResponse, error)
	ExecuteByShowID(ctx context.Context, showID int64) (ListTVEpisodesResponse, error)
	ExecuteByLibrary(ctx context.Context, libraryID int64) (ListTVEpisodesResponse, error)
}

// GetTVEpisodeExecutor defines the interface for getting a single TV episode
type GetTVEpisodeExecutor interface {
	Execute(ctx context.Context, id int64) (*TVEpisodeResponse, error)
}

// SearchTVEpisodesExecutor defines the interface for searching TV episodes
type SearchTVEpisodesExecutor interface {
	Execute(ctx context.Context, libraryID int64, query string) (ListTVEpisodesResponse, error)
}
