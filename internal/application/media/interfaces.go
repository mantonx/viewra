package media

import "context"

// GetMediaExecutor defines the interface for getting media
type GetMediaExecutor interface {
	Execute(ctx context.Context, id int64) (MediaResponse, error)
}

// ListMediaExecutor defines the interface for listing media
type ListMediaExecutor interface {
	ExecuteAll(ctx context.Context) (ListMediaResponse, error)
	Execute(ctx context.Context, libraryID int64) (ListMediaResponse, error)
}

// StreamInfoExecutor defines the interface for getting detailed stream info
type StreamInfoExecutor interface {
	Execute(ctx context.Context, mediaID int64) (*StreamInfoResponse, error)
	ExecuteWithCapabilities(ctx context.Context, mediaID int64, supportedVideoCodecs []string, supportsHDRDisplay bool, quality string) (*StreamInfoResponse, error)
}

// GetTracksExecutor defines the interface for getting audio and subtitle tracks
type GetTracksExecutor interface {
	Execute(ctx context.Context, mediaID int64) (*GetTracksResponse, error)
}
