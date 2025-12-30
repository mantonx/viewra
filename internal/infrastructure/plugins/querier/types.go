package querier

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

// MediaDetailsInfo contains full metadata for plugin indexing.
type MediaDetailsInfo struct {
	ID          int64
	MediaType   string
	Title       string
	Year        int
	LibraryID   int64
	ExternalIDs map[string]string

	// Rich metadata
	Plot             string
	Tagline          string
	Genres           []string
	Directors        []string
	Writers          []string
	Producers        []string
	Cast             []CastMemberInfo
	Studios          []string
	ContentRating    string
	RuntimeMinutes   int
	OriginalLanguage string
	CountryOfOrigin  string

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

	// Keywords for search (from TMDB)
	LocationKeywords []string // Location-related keywords (cities, countries, etc.)
	ThemeKeywords    []string // Non-location keywords (themes, moods, plot elements)
}

// CastMemberInfo represents a cast member.
type CastMemberInfo struct {
	Name      string
	Character string
	Order     int
}
