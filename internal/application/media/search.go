package media

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
)

// SearchResult represents a single search result.
type SearchResult struct {
	ID        int64   `json:"id"`
	MediaType string  `json:"media_type"` // "movie", "tv_show", "tv_episode"
	Title     string  `json:"title"`
	Year      int     `json:"year,omitempty"`
	Plot      string  `json:"plot,omitempty"`
	Score     float32 `json:"score"` // Match relevance (0.0 - 1.0)
}

// SearchResponse contains search results and metadata.
type SearchResponse struct {
	Results  []SearchResult `json:"results"`
	Total    int            `json:"total"`
	Fallback bool           `json:"fallback"` // True if using basic search (no semantic search plugin)
}

// SearchService provides basic text search for media when no search plugin is available.
type SearchService struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewSearchService creates a new SearchService.
func NewSearchService(db *sql.DB, logger *slog.Logger) *SearchService {
	return &SearchService{
		db:     db,
		logger: logger,
	}
}

// Search performs LIKE-based text search across movies, TV shows, and episodes.
// This is used as a fallback when no semantic search plugin is configured.
func (s *SearchService) Search(ctx context.Context, query string, limit int) (*SearchResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return &SearchResponse{Results: []SearchResult{}, Total: 0, Fallback: true}, nil
	}

	// Prepare search pattern for LIKE
	pattern := "%" + escapeLike(query) + "%"

	results := make([]SearchResult, 0)

	// Search movies
	movieResults, err := s.searchMovies(ctx, pattern, limit)
	if err != nil {
		s.logger.Warn("movie search failed", "error", err)
	} else {
		results = append(results, movieResults...)
	}

	// Search TV shows
	tvResults, err := s.searchTVShows(ctx, pattern, limit)
	if err != nil {
		s.logger.Warn("tv show search failed", "error", err)
	} else {
		results = append(results, tvResults...)
	}

	// Search TV episodes
	episodeResults, err := s.searchTVEpisodes(ctx, pattern, limit)
	if err != nil {
		s.logger.Warn("episode search failed", "error", err)
	} else {
		results = append(results, episodeResults...)
	}

	// Sort by score (title matches first, then plot matches)
	// and limit total results
	sortAndLimitResults(&results, limit)

	return &SearchResponse{
		Results:  results,
		Total:    len(results),
		Fallback: true,
	}, nil
}

func (s *SearchService) searchMovies(ctx context.Context, pattern string, limit int) ([]SearchResult, error) {
	// Score: 1.0 for title match, 0.5 for plot/tagline match
	query := `
		SELECT 
			m.media_id,
			med.title,
			COALESCE(m.year, 0),
			COALESCE(m.plot, ''),
			CASE 
				WHEN med.title LIKE ? ESCAPE '\' THEN 1.0
				WHEN COALESCE(m.original_title, '') LIKE ? ESCAPE '\' THEN 0.9
				WHEN COALESCE(m.tagline, '') LIKE ? ESCAPE '\' THEN 0.7
				WHEN COALESCE(m.plot, '') LIKE ? ESCAPE '\' THEN 0.5
				ELSE 0.3
			END as score
		FROM movies m
		JOIN media med ON med.id = m.media_id
		WHERE med.title LIKE ? ESCAPE '\'
		   OR COALESCE(m.original_title, '') LIKE ? ESCAPE '\'
		   OR COALESCE(m.tagline, '') LIKE ? ESCAPE '\'
		   OR COALESCE(m.plot, '') LIKE ? ESCAPE '\'
		ORDER BY score DESC, med.title ASC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query,
		pattern, pattern, pattern, pattern, // for CASE
		pattern, pattern, pattern, pattern, // for WHERE
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query movies: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Title, &r.Year, &r.Plot, &r.Score); err != nil {
			return nil, fmt.Errorf("scan movie: %w", err)
		}
		r.MediaType = "movie"
		results = append(results, r)
	}

	return results, rows.Err()
}

func (s *SearchService) searchTVShows(ctx context.Context, pattern string, limit int) ([]SearchResult, error) {
	query := `
		SELECT 
			id,
			title,
			COALESCE(year, 0),
			COALESCE(plot, ''),
			CASE 
				WHEN title LIKE ? ESCAPE '\' THEN 1.0
				WHEN COALESCE(original_title, '') LIKE ? ESCAPE '\' THEN 0.9
				WHEN COALESCE(plot, '') LIKE ? ESCAPE '\' THEN 0.5
				ELSE 0.3
			END as score
		FROM tv_shows
		WHERE title LIKE ? ESCAPE '\'
		   OR COALESCE(original_title, '') LIKE ? ESCAPE '\'
		   OR COALESCE(plot, '') LIKE ? ESCAPE '\'
		ORDER BY score DESC, title ASC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query,
		pattern, pattern, pattern, // for CASE
		pattern, pattern, pattern, // for WHERE
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query tv shows: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Title, &r.Year, &r.Plot, &r.Score); err != nil {
			return nil, fmt.Errorf("scan tv show: %w", err)
		}
		r.MediaType = "tv_show"
		results = append(results, r)
	}

	return results, rows.Err()
}

func (s *SearchService) searchTVEpisodes(ctx context.Context, pattern string, limit int) ([]SearchResult, error) {
	query := `
		SELECT 
			e.id,
			e.title,
			s.title as show_title,
			e.season_number,
			e.episode_number,
			COALESCE(e.plot, ''),
			CASE 
				WHEN e.title LIKE ? ESCAPE '\' THEN 1.0
				WHEN COALESCE(e.plot, '') LIKE ? ESCAPE '\' THEN 0.5
				ELSE 0.3
			END as score
		FROM tv_episodes e
		JOIN tv_seasons sea ON sea.id = e.season_id
		JOIN tv_shows s ON s.id = sea.show_id
		WHERE e.title LIKE ? ESCAPE '\'
		   OR COALESCE(e.plot, '') LIKE ? ESCAPE '\'
		ORDER BY score DESC, s.title ASC, e.season_number ASC, e.episode_number ASC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query,
		pattern, pattern, // for CASE
		pattern, pattern, // for WHERE
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query episodes: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var showTitle string
		var seasonNum, episodeNum int
		var plot string
		if err := rows.Scan(&r.ID, &r.Title, &showTitle, &seasonNum, &episodeNum, &plot, &r.Score); err != nil {
			return nil, fmt.Errorf("scan episode: %w", err)
		}
		r.MediaType = "tv_episode"
		// Format title as "Show Name - S01E05 - Episode Title"
		r.Title = fmt.Sprintf("%s - S%02dE%02d - %s", showTitle, seasonNum, episodeNum, r.Title)
		r.Plot = plot
		results = append(results, r)
	}

	return results, rows.Err()
}

// escapeLike escapes special characters for LIKE pattern matching.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// sortAndLimitResults sorts results by score descending and limits to n results.
func sortAndLimitResults(results *[]SearchResult, limit int) {
	// Simple bubble sort - fine for small result sets
	r := *results
	for i := 0; i < len(r); i++ {
		for j := i + 1; j < len(r); j++ {
			if r[j].Score > r[i].Score {
				r[i], r[j] = r[j], r[i]
			}
		}
	}

	if len(r) > limit {
		*results = r[:limit]
	}
}
