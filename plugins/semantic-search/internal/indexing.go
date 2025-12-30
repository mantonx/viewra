package internal

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// IndexingService handles media indexing for semantic search.
type IndexingService struct {
	embeddingService *EmbeddingService
	vector           *sdk.VectorClient
	data             *sdk.DataClient
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
	vector *sdk.VectorClient,
	data *sdk.DataClient,
	config IndexingConfig,
	logger *slog.Logger,
) *IndexingService {
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}
	return &IndexingService{
		embeddingService: embeddingService,
		vector:           vector,
		data:             data,
		batchSize:        batchSize,
		logger:           logger,
	}
}

// IndexSingle indexes a single entity. Called by the enrichment pipeline.
// It fetches full media details from the host to build rich searchable text.
func (s *IndexingService) IndexSingle(ctx context.Context, entityType EntityType, entityID int64, _ *sdk.Media) error {
	// Fetch full media details from the host, passing the media type
	// so the host can query the correct table directly
	details, err := s.data.GetMediaDetails(ctx, entityID, string(entityType))
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

	err = s.vector.Store(ctx, sdk.Embedding{
		EntityType: string(entityType),
		EntityID:   entityID,
		Vector:     embedding,
		Text:       text,
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

		mediaList, err := s.data.ListMediaByLibrary(ctx, libraryID, int(limit), int(offset))
		if err != nil {
			return fmt.Errorf("list media: %w", err)
		}

		// Update total on first batch
		if offset == 0 {
			total = int64(mediaList.Total)
			s.updateProgress(entityType, total, 0, 0, "")
			s.logger.Info("found media items", "total", total)
		}

		for _, media := range mediaList.Items {
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

			if err := s.IndexSingle(ctx, itemEntityType, media.ID, nil); err != nil {
				s.logger.Warn("failed to index media", "id", media.ID, "type", itemEntityType, "error", err)
				atomic.AddInt64(&failed, 1)
			} else {
				atomic.AddInt64(&processed, 1)
			}

			if processed%100 == 0 {
				s.updateProgress(entityType, total, processed, failed, "")
			}
		}

		if !mediaList.HasMore {
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
func (s *IndexingService) buildTextFromDetails(details *sdk.MediaDetails) string {
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
// Also includes language and country of origin for better international content discovery.
func (s *IndexingService) buildMovieText(m *sdk.MediaDetails) string {
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

	// Language and country (important for international content discovery)
	if m.OriginalLanguage != "" {
		langName := getLanguageName(m.OriginalLanguage)
		b.WriteString(fmt.Sprintf("Language: %s\n", langName))
	}
	if m.CountryOfOrigin != "" {
		b.WriteString(fmt.Sprintf("Country: %s\n", m.CountryOfOrigin))
	}

	// Era context (helps with decade-based searches like "80s action movies")
	if m.Year > 0 {
		era := getEraLabel(int32(m.Year))
		if era != "" {
			b.WriteString(fmt.Sprintf("Era: %s\n", era))
		}
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

	// Writers
	if len(m.Writers) > 0 {
		b.WriteString(fmt.Sprintf("Written by: %s\n", strings.Join(m.Writers, ", ")))
	}

	// Producers
	if len(m.Producers) > 0 {
		b.WriteString(fmt.Sprintf("Produced by: %s\n", strings.Join(m.Producers, ", ")))
	}

	// Cast (all cast members, no limit - important for actor-based searches)
	if len(m.Cast) > 0 {
		castNames := make([]string, 0, len(m.Cast))
		for _, c := range m.Cast {
			castNames = append(castNames, c.Name)
		}
		b.WriteString(fmt.Sprintf("Cast: %s\n", strings.Join(castNames, ", ")))
	}

	// Studios
	if len(m.Studios) > 0 {
		b.WriteString(fmt.Sprintf("Studios: %s\n", strings.Join(m.Studios, ", ")))
	}

	// Location/setting keywords (from TMDB)
	if len(m.LocationKeywords) > 0 {
		b.WriteString(fmt.Sprintf("Setting: %s\n", strings.Join(m.LocationKeywords, ", ")))
	}

	// Theme keywords (from TMDB) - themes, plot elements, character types, etc.
	if len(m.ThemeKeywords) > 0 {
		b.WriteString(fmt.Sprintf("Themes: %s\n", strings.Join(m.ThemeKeywords, ", ")))
	}

	return b.String()
}

// buildTVShowText builds text for TV shows.
// Format: Title (Year). Tagline. Plot. Network: X. Genres: X, Y. Mood: X, Y.
// Includes language and country for K-drama, J-drama discovery etc.
func (s *IndexingService) buildTVShowText(m *sdk.MediaDetails) string {
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

	// Language and country (important for K-drama, J-drama, etc.)
	if m.OriginalLanguage != "" {
		langName := getLanguageName(m.OriginalLanguage)
		b.WriteString(fmt.Sprintf("Language: %s\n", langName))
		// Add regional drama type hints
		switch m.OriginalLanguage {
		case "ko":
			b.WriteString("Type: K-drama Korean drama\n")
		case "ja":
			b.WriteString("Type: J-drama Japanese drama\n")
		case "zh":
			b.WriteString("Type: C-drama Chinese drama\n")
		case "th":
			b.WriteString("Type: Thai drama lakorn\n")
		case "tr":
			b.WriteString("Type: Turkish drama dizi\n")
		}
	}
	if m.CountryOfOrigin != "" {
		b.WriteString(fmt.Sprintf("Country: %s\n", m.CountryOfOrigin))
	}

	// Tagline
	if m.Tagline != "" {
		b.WriteString(fmt.Sprintf("Tagline: %s\n", m.Tagline))
	}

	// Plot
	if m.Plot != "" {
		b.WriteString(fmt.Sprintf("Plot: %s\n", m.Plot))
	}

	// Directors (created by)
	if len(m.Directors) > 0 {
		b.WriteString(fmt.Sprintf("Created by: %s\n", strings.Join(m.Directors, ", ")))
	}

	// Writers
	if len(m.Writers) > 0 {
		b.WriteString(fmt.Sprintf("Written by: %s\n", strings.Join(m.Writers, ", ")))
	}

	// Producers
	if len(m.Producers) > 0 {
		b.WriteString(fmt.Sprintf("Produced by: %s\n", strings.Join(m.Producers, ", ")))
	}

	// Cast (all cast members, no limit - important for actor-based searches)
	if len(m.Cast) > 0 {
		castNames := make([]string, 0, len(m.Cast))
		for _, c := range m.Cast {
			castNames = append(castNames, c.Name)
		}
		b.WriteString(fmt.Sprintf("Cast: %s\n", strings.Join(castNames, ", ")))
	}

	// Studios/Networks
	if len(m.Studios) > 0 {
		b.WriteString(fmt.Sprintf("Network: %s\n", strings.Join(m.Studios, ", ")))
	}

	// Location/setting keywords (from TMDB)
	if len(m.LocationKeywords) > 0 {
		b.WriteString(fmt.Sprintf("Setting: %s\n", strings.Join(m.LocationKeywords, ", ")))
	}

	// Theme keywords (from TMDB) - themes, plot elements, character types, etc.
	if len(m.ThemeKeywords) > 0 {
		b.WriteString(fmt.Sprintf("Themes: %s\n", strings.Join(m.ThemeKeywords, ", ")))
	}

	return b.String()
}

// buildTVEpisodeText builds text for TV episodes.
// Format: Show - S01E05: Episode Title. Plot.
func (s *IndexingService) buildTVEpisodeText(m *sdk.MediaDetails) string {
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
func (s *IndexingService) buildMusicArtistText(m *sdk.MediaDetails) string {
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
func (s *IndexingService) buildMusicAlbumText(m *sdk.MediaDetails) string {
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
func (s *IndexingService) buildMusicTrackText(m *sdk.MediaDetails) string {
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
func (s *IndexingService) buildGenericText(m *sdk.MediaDetails) string {
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

	return b.String()
}

// getLanguageName converts an ISO 639-1 language code to a human-readable name.
// This helps with semantic search by including full language names in embeddings.
func getLanguageName(code string) string {
	languageNames := map[string]string{
		"en": "English",
		"es": "Spanish",
		"fr": "French",
		"de": "German",
		"it": "Italian",
		"pt": "Portuguese",
		"ru": "Russian",
		"ja": "Japanese",
		"ko": "Korean",
		"zh": "Chinese",
		"hi": "Hindi",
		"ar": "Arabic",
		"th": "Thai",
		"vi": "Vietnamese",
		"tr": "Turkish",
		"pl": "Polish",
		"nl": "Dutch",
		"sv": "Swedish",
		"da": "Danish",
		"no": "Norwegian",
		"fi": "Finnish",
		"el": "Greek",
		"he": "Hebrew",
		"id": "Indonesian",
		"ms": "Malay",
		"tl": "Filipino",
		"cs": "Czech",
		"hu": "Hungarian",
		"ro": "Romanian",
		"uk": "Ukrainian",
	}

	if name, ok := languageNames[strings.ToLower(code)]; ok {
		return name
	}
	return code // Return the code if no mapping found
}

// getEraLabel returns a decade/era label for better search matching.
// Users often search for "80s movies" or "90s action".
func getEraLabel(year int32) string {
	switch {
	case year >= 2020:
		return "2020s contemporary modern"
	case year >= 2010:
		return "2010s"
	case year >= 2000:
		return "2000s"
	case year >= 1990:
		return "90s nineties"
	case year >= 1980:
		return "80s eighties"
	case year >= 1970:
		return "70s seventies"
	case year >= 1960:
		return "60s sixties"
	case year >= 1950:
		return "50s fifties classic"
	case year >= 1940:
		return "40s forties classic"
	case year >= 1930:
		return "30s thirties classic golden age"
	case year < 1930 && year > 0:
		return "silent era classic vintage"
	default:
		return ""
	}
}
