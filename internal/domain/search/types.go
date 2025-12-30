package search

// Result represents a single search result.
type Result struct {
	ID        int64   // Entity ID (media_id for movies, show_id for TV shows)
	MediaType string  // "movie", "tv_show", "tv_episode", "music_artist", "music_album", "music_track"
	Title     string  // Display title
	Year      int     // Release/air year
	LibraryID int64   // Library this item belongs to
	Score     float32 // Relevance score (0.0 - 1.0), higher is better
}

// Request contains search parameters.
type Request struct {
	Query      string   // Search query text
	MediaTypes []string // Filter by media types (empty = all)
	Limit      int      // Max results to return
}

// Response contains search results and metadata.
type Response struct {
	Results  []Result `json:"results"`
	Total    int      `json:"total"`
	Fallback bool     `json:"fallback"` // True if using fallback search (no semantic search plugin)
}
