package music

import (
	"context"

	"github.com/viewra/viewra/internal/domain/common"
)

// ListArtistsExecutor defines the interface for listing music artists
type ListArtistsExecutor interface {
	Execute(ctx context.Context, libraryID int64) (ListArtistsResponse, error)
	ExecuteWithPagination(ctx context.Context, libraryID int64, pagination *common.PaginationParams) (ListArtistsResponse, error)
}

// ListAlbumsByArtistIDExecutor defines the interface for listing albums by artist ID
type ListAlbumsByArtistIDExecutor interface {
	Execute(ctx context.Context, artistTrackID int64) (ListAlbumsResponse, error)
}

// ListTracksByAlbumIDExecutor defines the interface for listing tracks by album ID
type ListTracksByAlbumIDExecutor interface {
	Execute(ctx context.Context, albumTrackID int64) (ListTracksResponse, error)
}

// GetTrackExecutor defines the interface for getting a single music track
type GetTrackExecutor interface {
	Execute(ctx context.Context, id int64) (*MusicTrackResponse, error)
}

// SearchTracksExecutor defines the interface for searching music tracks
type SearchTracksExecutor interface {
	Execute(ctx context.Context, libraryID int64, query string) (ListTracksResponse, error)
}
