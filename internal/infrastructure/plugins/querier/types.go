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

	// Crew credits for specialized searches
	Composers        []string // Music composers
	Cinematographers []string // Directors of Photography

	// Similar/recommended titles (from TMDb recommendations)
	SimilarTitles []string

	// Playback information for filtering by technical specs
	PlaybackInfo *PlaybackInfoData
}

// PlaybackInfoData contains technical playback metadata for filtering.
type PlaybackInfoData struct {
	// Video specs
	Width           int
	Height          int
	ResolutionLabel string
	HDRFormat       string
	VideoCodec      string
	Bitrate         int64

	// Audio and subtitle tracks
	AudioTracks    []AudioTrackInfo
	SubtitleTracks []SubtitleTrackInfo
}

// AudioTrackInfo represents an audio stream in a media file.
type AudioTrackInfo struct {
	Codec         string
	Channels      int
	ChannelLayout string
	Language      string
	IsDefault     bool
	IsCommentary  bool
}

// SubtitleTrackInfo represents a subtitle stream in a media file.
type SubtitleTrackInfo struct {
	Language   string
	Codec      string
	IsForced   bool
	IsSDH      bool
	IsExternal bool
}

// CastMemberInfo represents a cast member.
type CastMemberInfo struct {
	Name      string
	Character string
	Order     int
}
