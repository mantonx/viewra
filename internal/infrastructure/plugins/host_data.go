package plugins

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// MediaQuerier is the interface for querying media data.
// This abstracts the database layer so plugins don't need direct DB access.
type MediaQuerier interface {
	// GetMediaByID returns a media item by its database ID.
	GetMediaByID(ctx context.Context, id int64) (*MediaInfo, error)

	// GetMediaByExternalID returns a media item by an external ID.
	GetMediaByExternalID(ctx context.Context, provider, externalID string) (*MediaInfo, error)

	// SearchMedia searches for media by title and optional year.
	SearchMedia(ctx context.Context, title string, year int, mediaType string, limit int) ([]*MediaInfo, error)

	// GetFilePath returns the file path for a media item.
	GetFilePath(ctx context.Context, mediaID int64) (string, error)

	// GetExternalIDs returns all external IDs for a media item.
	GetExternalIDs(ctx context.Context, mediaID int64) (map[string]string, error)
}

// MediaInfo represents basic media information exposed to plugins.
type MediaInfo struct {
	ID          int64
	MediaType   string
	Title       string
	Year        int
	FilePath    string
	LibraryID   int64
	ExternalIDs map[string]string
}

// HostDataServer implements the HostData gRPC service.
// This runs in the host process and provides read-only access to media data.
type HostDataServer struct {
	pluginv1.UnimplementedHostDataServer
	querier MediaQuerier
	logger  *slog.Logger
}

// NewHostDataServer creates a new HostDataServer.
func NewHostDataServer(querier MediaQuerier, logger *slog.Logger) *HostDataServer {
	return &HostDataServer{
		querier: querier,
		logger:  logger,
	}
}

// GetMedia retrieves a single media item by ID.
func (s *HostDataServer) GetMedia(ctx context.Context, req *pluginv1.MediaQuery) (*pluginv1.Media, error) {
	if req.MediaId == 0 {
		return nil, errors.New("media_id is required")
	}

	media, err := s.querier.GetMediaByID(ctx, req.MediaId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("media not found")
		}
		s.logger.Error("failed to get media", "media_id", req.MediaId, "error", err)
		return nil, err
	}

	return mediaInfoToProto(media), nil
}

// GetMediaByExternalId looks up media by external ID.
func (s *HostDataServer) GetMediaByExternalId(ctx context.Context, req *pluginv1.ExternalIdQuery) (*pluginv1.Media, error) {
	if req.Provider == "" || req.ExternalId == "" {
		return nil, errors.New("provider and external_id are required")
	}

	media, err := s.querier.GetMediaByExternalID(ctx, req.Provider, req.ExternalId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("media not found")
		}
		s.logger.Error("failed to get media by external ID",
			"provider", req.Provider,
			"external_id", req.ExternalId,
			"error", err)
		return nil, err
	}

	return mediaInfoToProto(media), nil
}

// SearchMedia searches for media by title/year.
func (s *HostDataServer) SearchMedia(ctx context.Context, req *pluginv1.SearchQuery) (*pluginv1.MediaList, error) {
	if req.Title == "" {
		return nil, errors.New("title is required")
	}

	limit := int(req.Limit)
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	results, err := s.querier.SearchMedia(ctx, req.Title, int(req.Year), req.MediaType, limit)
	if err != nil {
		s.logger.Error("failed to search media",
			"title", req.Title,
			"year", req.Year,
			"media_type", req.MediaType,
			"error", err)
		return nil, err
	}

	items := make([]*pluginv1.Media, 0, len(results))
	for _, m := range results {
		items = append(items, mediaInfoToProto(m))
	}

	return &pluginv1.MediaList{Items: items}, nil
}

// GetLibrary retrieves library information.
func (s *HostDataServer) GetLibrary(ctx context.Context, req *pluginv1.LibraryId) (*pluginv1.Library, error) {
	// TODO: Implement when library querier is available
	return nil, errors.New("not implemented")
}

// GetFilePath returns the full file path for a media item.
func (s *HostDataServer) GetFilePath(ctx context.Context, req *pluginv1.MediaId) (*pluginv1.FilePath, error) {
	if req.Id == 0 {
		return nil, errors.New("id is required")
	}

	path, err := s.querier.GetFilePath(ctx, req.Id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("media not found")
		}
		s.logger.Error("failed to get file path", "media_id", req.Id, "error", err)
		return nil, err
	}

	return &pluginv1.FilePath{Path: path}, nil
}

func mediaInfoToProto(m *MediaInfo) *pluginv1.Media {
	if m == nil {
		return nil
	}

	return &pluginv1.Media{
		Id:          m.ID,
		MediaType:   m.MediaType,
		Title:       m.Title,
		Year:        int32(m.Year),
		FilePath:    m.FilePath,
		LibraryId:   m.LibraryID,
		ExternalIds: m.ExternalIDs,
	}
}
