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

	// GetMediaDetails returns full metadata for a media item (for AI indexing).
	GetMediaDetails(ctx context.Context, id int64) (*MediaDetailsInfo, error)

	// GetMediaByExternalID returns a media item by an external ID.
	GetMediaByExternalID(ctx context.Context, provider, externalID string) (*MediaInfo, error)

	// SearchMedia searches for media by title and optional year.
	SearchMedia(ctx context.Context, title string, year int, mediaType string, limit int) ([]*MediaInfo, error)

	// ListMediaByLibrary lists all media in a library with pagination.
	ListMediaByLibrary(ctx context.Context, libraryID int64, limit, offset int) ([]*MediaDetailsInfo, int, error)

	// GetFilePath returns the file path for a media item.
	GetFilePath(ctx context.Context, mediaID int64) (string, error)

	// GetExternalIDs returns all external IDs for a media item.
	GetExternalIDs(ctx context.Context, mediaID int64) (map[string]string, error)

	// GetLibrary returns library information by ID.
	GetLibrary(ctx context.Context, id int64) (*LibraryInfo, error)

	// GetMoodTags returns mood tags for a media item.
	GetMoodTags(ctx context.Context, mediaID int64) ([]*MoodTagInfo, error)

	// SetMoodTags stores mood tags for a media item (replaces existing).
	SetMoodTags(ctx context.Context, mediaID int64, tags []*MoodTagInfo) error

	// DeleteMoodTags removes all mood tags for a media item.
	DeleteMoodTags(ctx context.Context, mediaID int64) error
}

// MoodTagInfo represents a mood tag for a media item.
type MoodTagInfo struct {
	Tag        string
	Confidence float32
}

// LibraryInfo represents library information exposed to plugins.
type LibraryInfo struct {
	ID        int64
	Name      string
	Path      string
	MediaType string // "movies", "tv", or "music"
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

// MediaDetailsInfo contains full metadata for AI indexing.
type MediaDetailsInfo struct {
	ID          int64
	MediaType   string
	Title       string
	Year        int
	LibraryID   int64
	ExternalIDs map[string]string

	// Rich metadata
	Plot           string
	Tagline        string
	Genres         []string
	Directors      []string
	Writers        []string
	Cast           []CastMemberInfo
	Studios        []string
	ContentRating  string
	RuntimeMinutes int

	// TV-specific
	ShowTitle     string
	SeasonNumber  int
	EpisodeNumber int

	// Music-specific
	ArtistName  string
	AlbumTitle  string
	Biography   string
	Country     string
	ReleaseType string

	// AI-generated
	MoodTags []string
}

// CastMemberInfo represents a cast member.
type CastMemberInfo struct {
	Name      string
	Character string
	Order     int
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
	if req.Id == 0 {
		return nil, errors.New("id is required")
	}

	lib, err := s.querier.GetLibrary(ctx, req.Id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("library not found")
		}
		s.logger.Error("failed to get library", "library_id", req.Id, "error", err)
		return nil, err
	}

	return &pluginv1.Library{
		Id:        lib.ID,
		Name:      lib.Name,
		Path:      lib.Path,
		MediaType: lib.MediaType,
	}, nil
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

// GetMediaDetails retrieves full metadata for a media item.
func (s *HostDataServer) GetMediaDetails(ctx context.Context, req *pluginv1.MediaQuery) (*pluginv1.MediaDetails, error) {
	if req.MediaId == 0 {
		return nil, errors.New("media_id is required")
	}

	details, err := s.querier.GetMediaDetails(ctx, req.MediaId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("media not found")
		}
		s.logger.Error("failed to get media details", "media_id", req.MediaId, "error", err)
		return nil, err
	}

	return mediaDetailsToProto(details), nil
}

// ListMediaByLibrary lists all media in a library with pagination.
func (s *HostDataServer) ListMediaByLibrary(ctx context.Context, req *pluginv1.ListMediaRequest) (*pluginv1.MediaDetailsList, error) {
	if req.LibraryId == 0 {
		return nil, errors.New("library_id is required")
	}

	limit := int(req.Limit)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := int(req.Offset)
	if offset < 0 {
		offset = 0
	}

	items, total, err := s.querier.ListMediaByLibrary(ctx, req.LibraryId, limit, offset)
	if err != nil {
		s.logger.Error("failed to list media by library", "library_id", req.LibraryId, "error", err)
		return nil, err
	}

	protoItems := make([]*pluginv1.MediaDetails, len(items))
	for i, item := range items {
		protoItems[i] = mediaDetailsToProto(item)
	}

	return &pluginv1.MediaDetailsList{
		Items:   protoItems,
		Total:   int32(total),
		HasMore: offset+len(items) < total,
	}, nil
}

// GetMoodTags retrieves mood tags for a media item.
func (s *HostDataServer) GetMoodTags(ctx context.Context, req *pluginv1.MediaQuery) (*pluginv1.MoodTagList, error) {
	if req.MediaId == 0 {
		return nil, errors.New("media_id is required")
	}

	tags, err := s.querier.GetMoodTags(ctx, req.MediaId)
	if err != nil {
		s.logger.Error("failed to get mood tags", "media_id", req.MediaId, "error", err)
		return nil, err
	}

	protoTags := make([]*pluginv1.MoodTag, len(tags))
	for i, t := range tags {
		protoTags[i] = &pluginv1.MoodTag{
			Tag:        t.Tag,
			Confidence: t.Confidence,
		}
	}

	return &pluginv1.MoodTagList{Tags: protoTags}, nil
}

// SetMoodTags stores mood tags for a media item (replaces existing).
func (s *HostDataServer) SetMoodTags(ctx context.Context, req *pluginv1.SetMoodTagsRequest) (*pluginv1.Empty, error) {
	if req.MediaId == 0 {
		return nil, errors.New("media_id is required")
	}

	tags := make([]*MoodTagInfo, len(req.Tags))
	for i, t := range req.Tags {
		tags[i] = &MoodTagInfo{
			Tag:        t.Tag,
			Confidence: t.Confidence,
		}
	}

	if err := s.querier.SetMoodTags(ctx, req.MediaId, tags); err != nil {
		s.logger.Error("failed to set mood tags", "media_id", req.MediaId, "error", err)
		return nil, err
	}

	s.logger.Debug("set mood tags", "media_id", req.MediaId, "count", len(tags))
	return &pluginv1.Empty{}, nil
}

// DeleteMoodTags removes all mood tags for a media item.
func (s *HostDataServer) DeleteMoodTags(ctx context.Context, req *pluginv1.MediaQuery) (*pluginv1.Empty, error) {
	if req.MediaId == 0 {
		return nil, errors.New("media_id is required")
	}

	if err := s.querier.DeleteMoodTags(ctx, req.MediaId); err != nil {
		s.logger.Error("failed to delete mood tags", "media_id", req.MediaId, "error", err)
		return nil, err
	}

	s.logger.Debug("deleted mood tags", "media_id", req.MediaId)
	return &pluginv1.Empty{}, nil
}

func mediaDetailsToProto(d *MediaDetailsInfo) *pluginv1.MediaDetails {
	if d == nil {
		return nil
	}

	cast := make([]*pluginv1.MediaCastMember, len(d.Cast))
	for i, c := range d.Cast {
		cast[i] = &pluginv1.MediaCastMember{
			Name: c.Name,
			Role: c.Character,
		}
	}

	return &pluginv1.MediaDetails{
		Id:             d.ID,
		MediaType:      d.MediaType,
		Title:          d.Title,
		Year:           int32(d.Year),
		LibraryId:      d.LibraryID,
		ExternalIds:    d.ExternalIDs,
		Plot:           d.Plot,
		Tagline:        d.Tagline,
		Genres:         d.Genres,
		Directors:      d.Directors,
		Writers:        d.Writers,
		Cast:           cast,
		Studios:        d.Studios,
		ContentRating:  d.ContentRating,
		RuntimeMinutes: int32(d.RuntimeMinutes),
		ShowTitle:      d.ShowTitle,
		SeasonNumber:   int32(d.SeasonNumber),
		EpisodeNumber:  int32(d.EpisodeNumber),
		ArtistName:     d.ArtistName,
		AlbumTitle:     d.AlbumTitle,
		Biography:      d.Biography,
		Country:        d.Country,
		ReleaseType:    d.ReleaseType,
		MoodTags:       d.MoodTags,
	}
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
