package media

// MusicTrack represents a music track media item with music-specific metadata
type MusicTrack struct {
	Media // Embedded base media fields

	// Entity relationships
	AlbumID  int64 // Foreign key to music_albums table (0 if not linked)
	ArtistID int64 // Foreign key to music_artists table (0 if not linked)

	// Basic music metadata
	Artist      string
	Album       string
	AlbumArtist string
	TrackNumber int
	DiscNumber  int
	Year        int
	Genre       string
	Composer    string
	Publisher   string // Record label
	Bitrate     int    // In kbps

	// Extended metadata
	TotalTracks   int    // Total tracks on disc/album
	TotalDiscs    int    // Total discs in album
	ReleaseDate   string // ISO 8601 date string (YYYY-MM-DD)
	Lyricist      string // Lyric writer
	ISRC          string // International Standard Recording Code
	ReleaseType   string // album, single, ep, compilation, live, remix, soundtrack
	Compilation   bool   // Compilation album flag
	OriginalTitle string // Title in original language
	SortTitle     string // Title for sorting

	// MusicBrainz IDs (reserved for future plugin use)
	MusicBrainzTrackID  string
	MusicBrainzAlbumID  string
	MusicBrainzArtistID string
}

// IsValid validates the music track entity including base media validation
func (m *MusicTrack) IsValid() error {
	// Validate base media fields first
	if err := m.Media.IsValid(); err != nil {
		return err
	}

	// Music-specific validation
	if m.TrackNumber < 0 {
		return ErrInvalidTrackNumber
	}

	if m.DiscNumber < 0 {
		return ErrInvalidTrackNumber // Reuse for disc number
	}

	if m.TotalTracks < 0 {
		return ErrInvalidTrackNumber // Reuse for total tracks
	}

	if m.TotalDiscs < 0 {
		return ErrInvalidTrackNumber // Reuse for total discs
	}

	if m.Year < 0 || m.Year > 2100 {
		if m.Year != 0 { // 0 is acceptable for unknown year
			return ErrInvalidYear
		}
	}

	if m.Bitrate < 0 {
		if m.Bitrate != 0 { // 0 is acceptable for unknown bitrate
			return ErrInvalidRating // Reuse rating error for negative values
		}
	}

	// Validate ReleaseType if specified
	if m.ReleaseType != "" {
		validTypes := map[string]bool{
			"album": true, "single": true, "ep": true, "compilation": true,
			"live": true, "remix": true, "soundtrack": true,
		}
		if !validTypes[m.ReleaseType] {
			return ErrInvalidRating // Reuse for invalid release type
		}
	}

	return nil
}

// Album represents a music album entity
type Album struct {
	ID         int64
	LibraryID  int64
	ArtistID   int64  // Foreign key to music_artists table (0 if not linked)
	Title      string
	AlbumArtist string // Primary album artist
	Artist     string // Fallback artist if album_artist not set
	Year       int
	ReleaseDate string // ISO 8601 date string (YYYY-MM-DD)
	Genre      string
	TotalTracks int
	TotalDiscs int
	RecordLabel string
	ReleaseType string // album, single, ep, compilation, live, remix, soundtrack
	Compilation bool
	MusicBrainzAlbumID string
	CoverArtPath string
	SortTitle   string
	CreatedAt   string
	UpdatedAt   string
}

// IsValid validates the album entity
func (a *Album) IsValid() error {
	if a.LibraryID <= 0 {
		return ErrInvalidLibraryID
	}

	if a.Title == "" {
		return ErrEmptyTitle
	}

	if a.Year < 0 || a.Year > 2100 {
		if a.Year != 0 { // 0 is acceptable for unknown year
			return ErrInvalidYear
		}
	}

	if a.TotalTracks < 0 {
		return ErrInvalidTrackNumber
	}

	if a.TotalDiscs < 0 {
		return ErrInvalidTrackNumber
	}

	// Validate ReleaseType if specified
	if a.ReleaseType != "" {
		validTypes := map[string]bool{
			"album": true, "single": true, "ep": true, "compilation": true,
			"live": true, "remix": true, "soundtrack": true,
		}
		if !validTypes[a.ReleaseType] {
			return ErrInvalidRating // Reuse for invalid release type
		}
	}

	return nil
}

// Artist represents a music artist entity
type Artist struct {
	ID                   int64
	LibraryID            int64
	Name                 string
	SortName             string // For sorting (e.g., "Beatles, The")
	MusicBrainzArtistID  string
	Bio                  string // Artist biography
	Country              string
	FormedYear           int
	Genre                string // Primary genre
	ImagePath            string // Artist photo/image
	CreatedAt            string
	UpdatedAt            string
}

// IsValid validates the artist entity
func (a *Artist) IsValid() error {
	if a.LibraryID <= 0 {
		return ErrInvalidLibraryID
	}

	if a.Name == "" {
		return ErrEmptyTitle // Reuse title error for name
	}

	if a.FormedYear < 0 || a.FormedYear > 2100 {
		if a.FormedYear != 0 { // 0 is acceptable for unknown year
			return ErrInvalidYear
		}
	}

	return nil
}
