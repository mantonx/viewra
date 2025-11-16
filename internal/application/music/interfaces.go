package music

import "context"

// ListArtistsExecutor defines the interface for listing music artists
type ListArtistsExecutor interface {
	Execute(ctx context.Context, libraryID int64) (ListArtistsResponse, error)
}

// ListAlbumsExecutor defines the interface for listing music albums
type ListAlbumsExecutor interface {
	ExecuteByArtist(ctx context.Context, libraryID int64, artist string) (ListAlbumsResponse, error)
}

// ListTracksExecutor defines the interface for listing music tracks
type ListTracksExecutor interface {
	ExecuteByAlbum(ctx context.Context, libraryID int64, album string) (ListTracksResponse, error)
	ExecuteByLibrary(ctx context.Context, libraryID int64) (ListTracksResponse, error)
}

// GetTrackExecutor defines the interface for getting a single music track
type GetTrackExecutor interface {
	Execute(ctx context.Context, id int64) (*MusicTrackResponse, error)
}

// SearchTracksExecutor defines the interface for searching music tracks
type SearchTracksExecutor interface {
	Execute(ctx context.Context, libraryID int64, query string) (ListTracksResponse, error)
}
