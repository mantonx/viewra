package host

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/querier"
)

// Type aliases for backward compatibility - types are now defined in querier package.
type (
	LibraryInfo      = querier.LibraryInfo
	MediaInfo        = querier.MediaInfo
	MediaDetailsInfo = querier.MediaDetailsInfo
	CastMemberInfo   = querier.CastMemberInfo
)

// MediaQuerier is the interface for querying media data.
// This abstracts the database layer so plugins don't need direct DB access.
type MediaQuerier interface {
	// GetMediaByID returns a media item by its database ID.
	GetMediaByID(ctx context.Context, id int64) (*MediaInfo, error)

	// GetMediaDetails returns full metadata for a media item (for plugin indexing).
	// mediaType is optional - if empty, it will try to determine the type from the media table.
	GetMediaDetails(ctx context.Context, id int64, mediaType string) (*MediaDetailsInfo, error)

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

	// ListMediaByGenre returns media items matching a genre pattern.
	// mediaType should be "movie" or "tv_show".
	// libraryID=0 means all libraries.
	// excludeIDs are entity IDs to exclude from results.
	ListMediaByGenre(ctx context.Context, mediaType, genre string, libraryID int64, excludeIDs []int64, limit int) ([]*MediaInfo, error)

	// ListMediaByDirector returns media items directed by a specific person.
	// mediaType should be "movie" or "tv_show".
	// libraryID=0 means all libraries.
	// excludeIDs are entity IDs to exclude from results.
	ListMediaByDirector(ctx context.Context, mediaType, directorName string, libraryID int64, excludeIDs []int64, limit int) ([]*MediaInfo, error)
}

// DataServer implements the HostData gRPC service.
// This runs in the host process and provides read-only access to media data.
type DataServer struct {
	pluginv1.UnimplementedHostDataServer
	querier MediaQuerier
	logger  *slog.Logger
}

// NewDataServer creates a new DataServer.
func NewDataServer(querier MediaQuerier, logger *slog.Logger) *DataServer {
	return &DataServer{
		querier: querier,
		logger:  logger,
	}
}

// GetMedia retrieves a single media item by ID.
func (s *DataServer) GetMedia(ctx context.Context, req *pluginv1.MediaQuery) (*pluginv1.Media, error) {
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
func (s *DataServer) GetMediaByExternalId(ctx context.Context, req *pluginv1.ExternalIdQuery) (*pluginv1.Media, error) {
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
func (s *DataServer) SearchMedia(ctx context.Context, req *pluginv1.SearchQuery) (*pluginv1.MediaList, error) {
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
func (s *DataServer) GetLibrary(ctx context.Context, req *pluginv1.LibraryId) (*pluginv1.Library, error) {
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
func (s *DataServer) GetFilePath(ctx context.Context, req *pluginv1.MediaId) (*pluginv1.FilePath, error) {
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
func (s *DataServer) GetMediaDetails(ctx context.Context, req *pluginv1.MediaQuery) (*pluginv1.MediaDetails, error) {
	if req.MediaId == 0 {
		return nil, errors.New("media_id is required")
	}

	details, err := s.querier.GetMediaDetails(ctx, req.MediaId, req.MediaType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("media not found")
		}
		s.logger.Error("failed to get media details", "media_id", req.MediaId, "media_type", req.MediaType, "error", err)
		return nil, err
	}

	return mediaDetailsToProto(details), nil
}

// ListMediaByLibrary lists all media in a library with pagination.
func (s *DataServer) ListMediaByLibrary(ctx context.Context, req *pluginv1.ListMediaRequest) (*pluginv1.MediaDetailsList, error) {
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

	s.logger.Debug("ListMediaByLibrary called",
		"library_id", req.LibraryId,
		"limit", limit,
		"offset", offset,
	)

	items, total, err := s.querier.ListMediaByLibrary(ctx, req.LibraryId, limit, offset)
	if err != nil {
		s.logger.Error("failed to list media by library", "library_id", req.LibraryId, "error", err)
		return nil, err
	}

	s.logger.Debug("ListMediaByLibrary result",
		"library_id", req.LibraryId,
		"items_count", len(items),
		"total", total,
	)

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
		Id:               d.ID,
		MediaType:        d.MediaType,
		Title:            d.Title,
		Year:             int32(d.Year),
		LibraryId:        d.LibraryID,
		ExternalIds:      d.ExternalIDs,
		Plot:             d.Plot,
		Tagline:          d.Tagline,
		Genres:           d.Genres,
		Directors:        d.Directors,
		Writers:          d.Writers,
		Producers:        d.Producers,
		Cast:             cast,
		Studios:          d.Studios,
		ContentRating:    d.ContentRating,
		RuntimeMinutes:   int32(d.RuntimeMinutes),
		OriginalLanguage: d.OriginalLanguage,
		CountryOfOrigin:  d.CountryOfOrigin,
		ShowTitle:        d.ShowTitle,
		SeasonNumber:     int32(d.SeasonNumber),
		EpisodeNumber:    int32(d.EpisodeNumber),
		ArtistName:       d.ArtistName,
		AlbumTitle:       d.AlbumTitle,
		Biography:        d.Biography,
		Country:          d.Country,
		ReleaseType:      d.ReleaseType,
		LocationKeywords: d.LocationKeywords,
		ThemeKeywords:    d.ThemeKeywords,
		Composers:        d.Composers,
		Cinematographers: d.Cinematographers,
		PlaybackInfo:     playbackInfoToProto(d.PlaybackInfo),
	}
}

// playbackInfoToProto converts PlaybackInfoData to proto PlaybackInfo.
func playbackInfoToProto(p *querier.PlaybackInfoData) *pluginv1.PlaybackInfo {
	if p == nil {
		return nil
	}

	audioTracks := make([]*pluginv1.AudioTrack, len(p.AudioTracks))
	for i, t := range p.AudioTracks {
		audioTracks[i] = &pluginv1.AudioTrack{
			Codec:         t.Codec,
			Channels:      int32(t.Channels),
			ChannelLayout: t.ChannelLayout,
			Language:      t.Language,
			IsDefault:     t.IsDefault,
			IsCommentary:  t.IsCommentary,
		}
	}

	subtitleTracks := make([]*pluginv1.SubtitleTrack, len(p.SubtitleTracks))
	for i, t := range p.SubtitleTracks {
		subtitleTracks[i] = &pluginv1.SubtitleTrack{
			Language:   t.Language,
			Codec:      t.Codec,
			IsForced:   t.IsForced,
			IsSdh:      t.IsSDH,
			IsExternal: t.IsExternal,
		}
	}

	return &pluginv1.PlaybackInfo{
		Width:           int32(p.Width),
		Height:          int32(p.Height),
		ResolutionLabel: p.ResolutionLabel,
		HdrFormat:       p.HDRFormat,
		VideoCodec:      p.VideoCodec,
		Bitrate:         p.Bitrate,
		AudioTracks:     audioTracks,
		SubtitleTracks:  subtitleTracks,
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

// ListMediaByGenre lists media items matching a genre pattern.
// Used for genre-based recommendations when semantic search is unavailable.
func (s *DataServer) ListMediaByGenre(ctx context.Context, req *pluginv1.ListMediaByGenreRequest) (*pluginv1.MediaList, error) {
	if req.MediaType == "" {
		return nil, errors.New("media_type is required")
	}
	if req.Genre == "" {
		return nil, errors.New("genre is required")
	}

	// Validate media type
	if req.MediaType != "movie" && req.MediaType != "tv_show" {
		return nil, errors.New("media_type must be 'movie' or 'tv_show'")
	}

	limit := int(req.Limit)
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	s.logger.Debug("ListMediaByGenre called",
		"media_type", req.MediaType,
		"genre", req.Genre,
		"library_id", req.LibraryId,
		"exclude_ids_count", len(req.ExcludeIds),
		"limit", limit,
	)

	items, err := s.querier.ListMediaByGenre(ctx, req.MediaType, req.Genre, req.LibraryId, req.ExcludeIds, limit)
	if err != nil {
		s.logger.Error("failed to list media by genre",
			"media_type", req.MediaType,
			"genre", req.Genre,
			"error", err)
		return nil, err
	}

	protoItems := make([]*pluginv1.Media, len(items))
	for i, item := range items {
		protoItems[i] = mediaInfoToProto(item)
	}

	return &pluginv1.MediaList{Items: protoItems}, nil
}

// ListMediaByDirector lists media items directed by a specific person.
// Used for director-based recommendations and themed collections.
func (s *DataServer) ListMediaByDirector(ctx context.Context, req *pluginv1.ListMediaByDirectorRequest) (*pluginv1.MediaList, error) {
	if req.MediaType == "" {
		return nil, errors.New("media_type is required")
	}
	if req.DirectorName == "" {
		return nil, errors.New("director_name is required")
	}

	// Validate media type
	if req.MediaType != "movie" && req.MediaType != "tv_show" {
		return nil, errors.New("media_type must be 'movie' or 'tv_show'")
	}

	limit := int(req.Limit)
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	s.logger.Debug("ListMediaByDirector called",
		"media_type", req.MediaType,
		"director_name", req.DirectorName,
		"library_id", req.LibraryId,
		"exclude_ids_count", len(req.ExcludeIds),
		"limit", limit,
	)

	items, err := s.querier.ListMediaByDirector(ctx, req.MediaType, req.DirectorName, req.LibraryId, req.ExcludeIds, limit)
	if err != nil {
		s.logger.Error("failed to list media by director",
			"media_type", req.MediaType,
			"director_name", req.DirectorName,
			"error", err)
		return nil, err
	}

	protoItems := make([]*pluginv1.Media, len(items))
	for i, item := range items {
		protoItems[i] = mediaInfoToProto(item)
	}

	return &pluginv1.MediaList{Items: protoItems}, nil
}
