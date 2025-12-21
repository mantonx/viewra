package internal

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// IndexingService handles media indexing for semantic search.
type IndexingService struct {
	embeddingService *EmbeddingService
	embeddingsClient pluginv1.HostEmbeddingsClient
	dataClient       pluginv1.HostDataClient
	batchSize        int
	logger           *slog.Logger

	// Indexing state
	mu         sync.RWMutex
	isIndexing bool
	cancelFunc context.CancelFunc
	progress   IndexingProgress
}

// NewIndexingService creates a new indexing service.
func NewIndexingService(
	embeddingService *EmbeddingService,
	embeddingsClient pluginv1.HostEmbeddingsClient,
	dataClient pluginv1.HostDataClient,
	config IndexingConfig,
	logger *slog.Logger,
) *IndexingService {
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}
	return &IndexingService{
		embeddingService: embeddingService,
		embeddingsClient: embeddingsClient,
		dataClient:       dataClient,
		batchSize:        batchSize,
		logger:           logger,
	}
}

// IndexSingle indexes a single entity. Called by the enrichment pipeline.
// It fetches full media details from the host to build rich searchable text.
func (s *IndexingService) IndexSingle(ctx context.Context, entityType EntityType, entityID int64, _ *pluginv1.Media) error {
	// Fetch full media details from the host, passing the media type
	// so the host can query the correct table directly
	details, err := s.dataClient.GetMediaDetails(ctx, &pluginv1.MediaQuery{
		MediaId:   entityID,
		MediaType: string(entityType),
	})
	if err != nil {
		return fmt.Errorf("get media details for %s:%d: %w", entityType, entityID, err)
	}

	text := s.buildTextFromDetails(details)
	if text == "" {
		s.logger.Debug("skipping entity with empty text", "type", entityType, "id", entityID)
		return nil
	}

	embedding, err := s.embeddingService.EmbedSingle(ctx, text)
	if err != nil {
		return fmt.Errorf("generate embedding for %s:%d: %w", entityType, entityID, err)
	}

	_, err = s.embeddingsClient.Store(ctx, &pluginv1.StoreEmbeddingRequest{
		EntityType: string(entityType),
		EntityId:   entityID,
		Embedding:  embedding,
		Text:       truncateText(text, 500),
	})
	if err != nil {
		return fmt.Errorf("store embedding for %s:%d: %w", entityType, entityID, err)
	}

	s.logger.Debug("indexed entity", "type", entityType, "id", entityID, "text_len", len(text))
	return nil
}

// IndexLibrary indexes all media in a library.
func (s *IndexingService) IndexLibrary(ctx context.Context, libraryID int64, libraryType string) error {
	s.mu.Lock()
	if s.isIndexing {
		s.mu.Unlock()
		return fmt.Errorf("indexing already in progress")
	}
	s.isIndexing = true
	ctx, s.cancelFunc = context.WithCancel(ctx)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isIndexing = false
		s.cancelFunc = nil
		s.mu.Unlock()
	}()

	s.logger.Info("starting library indexing", "library_id", libraryID, "type", libraryType)

	entityType := s.getEntityType(libraryType)
	var processed, failed, total int64
	offset := int32(0)
	limit := int32(100)

	// Paginate through all media in the library
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := s.dataClient.ListMediaByLibrary(ctx, &pluginv1.ListMediaRequest{
			LibraryId: libraryID,
			Limit:     limit,
			Offset:    offset,
		})
		if err != nil {
			return fmt.Errorf("list media: %w", err)
		}

		// Update total on first batch
		if offset == 0 {
			total = int64(resp.Total)
			s.updateProgress(entityType, total, 0, 0, "")
			s.logger.Info("found media items", "total", total)
		}

		for _, media := range resp.Items {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Use media type from the item - more reliable than library type
			itemEntityType := EntityType(media.MediaType)
			if itemEntityType == "" {
				itemEntityType = entityType // fallback to library-derived type
			}

			if err := s.IndexSingle(ctx, itemEntityType, media.Id, nil); err != nil {
				s.logger.Warn("failed to index media", "id", media.Id, "type", itemEntityType, "error", err)
				atomic.AddInt64(&failed, 1)
			} else {
				atomic.AddInt64(&processed, 1)
			}

			if processed%100 == 0 {
				s.updateProgress(entityType, total, processed, failed, "")
			}
		}

		if !resp.HasMore {
			break
		}
		offset += limit
	}

	s.updateProgress(entityType, total, processed, failed, "")
	s.logger.Info("library indexing complete", "total", total, "processed", processed, "failed", failed)
	return nil
}

// Cancel cancels any running indexing operation.
func (s *IndexingService) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
}

// GetProgress returns the current indexing progress.
func (s *IndexingService) GetProgress() *IndexingProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.isIndexing {
		return nil
	}
	progress := s.progress
	return &progress
}

// IsIndexing returns whether indexing is in progress.
func (s *IndexingService) IsIndexing() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isIndexing
}

func (s *IndexingService) updateProgress(entityType EntityType, total, processed, failed int64, lastError string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	if s.progress.StartedAt == 0 {
		s.progress.StartedAt = now
	}
	s.progress = IndexingProgress{
		EntityType:  entityType,
		Total:       total,
		Processed:   processed,
		Failed:      failed,
		LastError:   lastError,
		StartedAt:   s.progress.StartedAt,
		LastUpdated: now,
	}
}

func (s *IndexingService) getEntityType(mediaType string) EntityType {
	switch mediaType {
	case "movie", "movies":
		return EntityMovie
	case "tv", "tvshow", "tv_show":
		return EntityTVShow
	case "music":
		return EntityMusicArtist
	default:
		return EntityType(mediaType)
	}
}

// buildTextFromDetails builds rich searchable text from full media details.
// Text format follows the AI Assistant spec for optimal semantic search.
func (s *IndexingService) buildTextFromDetails(details *pluginv1.MediaDetails) string {
	if details == nil {
		return ""
	}

	switch details.MediaType {
	case "movie":
		return s.buildMovieText(details)
	case "tv_show":
		return s.buildTVShowText(details)
	case "tv_episode":
		return s.buildTVEpisodeText(details)
	case "music_artist":
		return s.buildMusicArtistText(details)
	case "music_album":
		return s.buildMusicAlbumText(details)
	case "music_track":
		return s.buildMusicTrackText(details)
	default:
		// Fallback for unknown types
		return s.buildGenericText(details)
	}
}

// buildMovieText builds text for movies.
// Format: Title (Year). Tagline. Plot. Directed by X. Starring A, B, C. Genres: X, Y. Mood: X, Y.
func (s *IndexingService) buildMovieText(m *pluginv1.MediaDetails) string {
	var b strings.Builder

	// Title and year
	b.WriteString(fmt.Sprintf("Title: %s", m.Title))
	if m.Year > 0 {
		b.WriteString(fmt.Sprintf(" (%d)", m.Year))
	}
	b.WriteString("\n")

	// Genres
	if len(m.Genres) > 0 {
		b.WriteString(fmt.Sprintf("Genre: %s\n", strings.Join(m.Genres, ", ")))
	}

	// Tagline
	if m.Tagline != "" {
		b.WriteString(fmt.Sprintf("Tagline: %s\n", m.Tagline))
	}

	// Plot
	if m.Plot != "" {
		b.WriteString(fmt.Sprintf("Plot: %s\n", m.Plot))
	}

	// Directors
	if len(m.Directors) > 0 {
		b.WriteString(fmt.Sprintf("Directed by: %s\n", strings.Join(m.Directors, ", ")))
	}

	// Cast (top 5)
	if len(m.Cast) > 0 {
		castNames := make([]string, 0, min(5, len(m.Cast)))
		for i, c := range m.Cast {
			if i >= 5 {
				break
			}
			castNames = append(castNames, c.Name)
		}
		b.WriteString(fmt.Sprintf("Cast: %s\n", strings.Join(castNames, ", ")))
	}

	// Studios
	if len(m.Studios) > 0 {
		b.WriteString(fmt.Sprintf("Studios: %s\n", strings.Join(m.Studios, ", ")))
	}

	// Mood tags (AI-generated)
	if len(m.MoodTags) > 0 {
		b.WriteString(fmt.Sprintf("Mood: %s\n", strings.Join(m.MoodTags, ", ")))
	}

	return b.String()
}

// buildTVShowText builds text for TV shows.
// Format: Title (Year). Tagline. Plot. Network: X. Genres: X, Y. Mood: X, Y.
func (s *IndexingService) buildTVShowText(m *pluginv1.MediaDetails) string {
	var b strings.Builder

	// Title and year
	b.WriteString(fmt.Sprintf("Title: %s", m.Title))
	if m.Year > 0 {
		b.WriteString(fmt.Sprintf(" (%d)", m.Year))
	}
	b.WriteString("\n")

	// Genres
	if len(m.Genres) > 0 {
		b.WriteString(fmt.Sprintf("Genre: %s\n", strings.Join(m.Genres, ", ")))
	}

	// Tagline
	if m.Tagline != "" {
		b.WriteString(fmt.Sprintf("Tagline: %s\n", m.Tagline))
	}

	// Plot
	if m.Plot != "" {
		b.WriteString(fmt.Sprintf("Plot: %s\n", m.Plot))
	}

	// Cast (top 5)
	if len(m.Cast) > 0 {
		castNames := make([]string, 0, min(5, len(m.Cast)))
		for i, c := range m.Cast {
			if i >= 5 {
				break
			}
			castNames = append(castNames, c.Name)
		}
		b.WriteString(fmt.Sprintf("Cast: %s\n", strings.Join(castNames, ", ")))
	}

	// Studios/Networks
	if len(m.Studios) > 0 {
		b.WriteString(fmt.Sprintf("Network: %s\n", strings.Join(m.Studios, ", ")))
	}

	// Mood tags (AI-generated)
	if len(m.MoodTags) > 0 {
		b.WriteString(fmt.Sprintf("Mood: %s\n", strings.Join(m.MoodTags, ", ")))
	}

	return b.String()
}

// buildTVEpisodeText builds text for TV episodes.
// Format: Show - S01E05: Episode Title. Plot.
func (s *IndexingService) buildTVEpisodeText(m *pluginv1.MediaDetails) string {
	var b strings.Builder

	// Show title and episode info
	if m.ShowTitle != "" {
		b.WriteString(fmt.Sprintf("Show: %s\n", m.ShowTitle))
	}
	b.WriteString(fmt.Sprintf("Episode: S%02dE%02d - %s\n", m.SeasonNumber, m.EpisodeNumber, m.Title))

	// Plot
	if m.Plot != "" {
		b.WriteString(fmt.Sprintf("Plot: %s\n", m.Plot))
	}

	// Directors (for episodes)
	if len(m.Directors) > 0 {
		b.WriteString(fmt.Sprintf("Directed by: %s\n", strings.Join(m.Directors, ", ")))
	}

	// Writers
	if len(m.Writers) > 0 {
		b.WriteString(fmt.Sprintf("Written by: %s\n", strings.Join(m.Writers, ", ")))
	}

	return b.String()
}

// buildMusicArtistText builds text for music artists.
// Format: Name. Bio. Genre: X. From Country.
func (s *IndexingService) buildMusicArtistText(m *pluginv1.MediaDetails) string {
	var b strings.Builder

	// Name
	b.WriteString(fmt.Sprintf("Artist: %s\n", m.Title))

	// Genres
	if len(m.Genres) > 0 {
		b.WriteString(fmt.Sprintf("Genre: %s\n", strings.Join(m.Genres, ", ")))
	}

	// Biography
	if m.Biography != "" {
		b.WriteString(fmt.Sprintf("Biography: %s\n", m.Biography))
	}

	// Country
	if m.Country != "" {
		b.WriteString(fmt.Sprintf("From: %s\n", m.Country))
	}

	return b.String()
}

// buildMusicAlbumText builds text for music albums.
// Format: Title by Artist (Year). Genre: X. Type: album/single/ep.
func (s *IndexingService) buildMusicAlbumText(m *pluginv1.MediaDetails) string {
	var b strings.Builder

	// Title and artist
	b.WriteString(fmt.Sprintf("Album: %s", m.Title))
	if m.ArtistName != "" {
		b.WriteString(fmt.Sprintf(" by %s", m.ArtistName))
	}
	if m.Year > 0 {
		b.WriteString(fmt.Sprintf(" (%d)", m.Year))
	}
	b.WriteString("\n")

	// Genres
	if len(m.Genres) > 0 {
		b.WriteString(fmt.Sprintf("Genre: %s\n", strings.Join(m.Genres, ", ")))
	}

	// Release type
	if m.ReleaseType != "" {
		b.WriteString(fmt.Sprintf("Type: %s\n", m.ReleaseType))
	}

	return b.String()
}

// buildMusicTrackText builds text for music tracks.
// Format: Title by Artist. Album: X. Genre: X.
func (s *IndexingService) buildMusicTrackText(m *pluginv1.MediaDetails) string {
	var b strings.Builder

	// Title and artist
	b.WriteString(fmt.Sprintf("Track: %s", m.Title))
	if m.ArtistName != "" {
		b.WriteString(fmt.Sprintf(" by %s", m.ArtistName))
	}
	b.WriteString("\n")

	// Album
	if m.AlbumTitle != "" {
		b.WriteString(fmt.Sprintf("Album: %s\n", m.AlbumTitle))
	}

	// Genres
	if len(m.Genres) > 0 {
		b.WriteString(fmt.Sprintf("Genre: %s\n", strings.Join(m.Genres, ", ")))
	}

	return b.String()
}

// buildGenericText builds text for unknown media types.
func (s *IndexingService) buildGenericText(m *pluginv1.MediaDetails) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Title: %s", m.Title))
	if m.Year > 0 {
		b.WriteString(fmt.Sprintf(" (%d)", m.Year))
	}
	b.WriteString("\n")

	if len(m.Genres) > 0 {
		b.WriteString(fmt.Sprintf("Genre: %s\n", strings.Join(m.Genres, ", ")))
	}

	if m.Plot != "" {
		b.WriteString(fmt.Sprintf("Description: %s\n", m.Plot))
	}

	// Mood tags (AI-generated)
	if len(m.MoodTags) > 0 {
		b.WriteString(fmt.Sprintf("Mood: %s\n", strings.Join(m.MoodTags, ", ")))
	}

	return b.String()
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
