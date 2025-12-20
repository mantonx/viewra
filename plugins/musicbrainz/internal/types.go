package internal

// MusicBrainz API response types.
// These are pure data structures for JSON unmarshaling.
// API documentation: https://musicbrainz.org/doc/MusicBrainz_API

// Recording represents a single track/recording in MusicBrainz.
type Recording struct {
	ID             string          `json:"id"`
	Title          string          `json:"title"`
	Length         int             `json:"length"` // Duration in milliseconds
	Disambiguation string          `json:"disambiguation,omitempty"`
	ArtistCredit   []ArtistCredit  `json:"artist-credit,omitempty"`
	Releases       []ReleaseStub   `json:"releases,omitempty"`
	ISRCs          []string        `json:"isrcs,omitempty"`
	Tags           []Tag           `json:"tags,omitempty"`
	Score          int             `json:"score,omitempty"` // Search relevance score (0-100)
}

// RecordingSearchResponse is the response from MusicBrainz recording search.
type RecordingSearchResponse struct {
	Created    string      `json:"created"`
	Count      int         `json:"count"`
	Offset     int         `json:"offset"`
	Recordings []Recording `json:"recordings"`
}

// Release represents an album/single/EP release in MusicBrainz.
type Release struct {
	ID                 string           `json:"id"`
	Title              string           `json:"title"`
	Status             string           `json:"status,omitempty"`     // Official, Promotion, Bootleg, Pseudo-Release
	Date               string           `json:"date,omitempty"`       // Release date (YYYY, YYYY-MM, or YYYY-MM-DD)
	Country            string           `json:"country,omitempty"`    // ISO 3166-1 alpha-2 country code
	Barcode            string           `json:"barcode,omitempty"`
	Disambiguation     string           `json:"disambiguation,omitempty"`
	Quality            string           `json:"quality,omitempty"`
	ArtistCredit       []ArtistCredit   `json:"artist-credit,omitempty"`
	ReleaseGroup       *ReleaseGroup    `json:"release-group,omitempty"`
	Media              []Medium         `json:"media,omitempty"`
	LabelInfo          []LabelInfo      `json:"label-info,omitempty"`
	TextRepresentation *TextRepresentation `json:"text-representation,omitempty"`
	Tags               []Tag            `json:"tags,omitempty"`
	Score              int              `json:"score,omitempty"` // Search relevance score (0-100)
}

// ReleaseStub is a minimal release reference used in recording results.
type ReleaseStub struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Status       string        `json:"status,omitempty"`
	Date         string        `json:"date,omitempty"`
	Country      string        `json:"country,omitempty"`
	ReleaseGroup *ReleaseGroup `json:"release-group,omitempty"`
	TrackCount   int           `json:"track-count,omitempty"`
}

// ReleaseSearchResponse is the response from MusicBrainz release search.
type ReleaseSearchResponse struct {
	Created  string    `json:"created"`
	Count    int       `json:"count"`
	Offset   int       `json:"offset"`
	Releases []Release `json:"releases"`
}

// ReleaseGroup represents an album concept (all versions/editions of an album).
type ReleaseGroup struct {
	ID               string         `json:"id"`
	Title            string         `json:"title"`
	PrimaryType      string         `json:"primary-type,omitempty"`      // Album, Single, EP, Broadcast, Other
	SecondaryTypes   []string       `json:"secondary-types,omitempty"`   // Compilation, Soundtrack, Live, Remix, etc.
	FirstReleaseDate string         `json:"first-release-date,omitempty"`
	Disambiguation   string         `json:"disambiguation,omitempty"`
	ArtistCredit     []ArtistCredit `json:"artist-credit,omitempty"`
	Releases         []Release      `json:"releases,omitempty"`
	Tags             []Tag          `json:"tags,omitempty"`
	Score            int            `json:"score,omitempty"` // Search relevance score (0-100)
}

// ReleaseGroupSearchResponse is the response from MusicBrainz release-group search.
type ReleaseGroupSearchResponse struct {
	Created       string         `json:"created"`
	Count         int            `json:"count"`
	Offset        int            `json:"offset"`
	ReleaseGroups []ReleaseGroup `json:"release-groups"`
}

// Artist represents a musician, band, or other credited entity.
type Artist struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	SortName       string   `json:"sort-name,omitempty"`
	Type           string   `json:"type,omitempty"`      // Person, Group, Orchestra, Choir, Character, Other
	Country        string   `json:"country,omitempty"`   // ISO 3166-1 alpha-2 country code
	Disambiguation string   `json:"disambiguation,omitempty"`
	LifeSpan       *LifeSpan `json:"life-span,omitempty"`
	Area           *Area    `json:"area,omitempty"`
	BeginArea      *Area    `json:"begin-area,omitempty"`
	Gender         string   `json:"gender,omitempty"`    // Male, Female, Other, Not applicable
	Tags           []Tag    `json:"tags,omitempty"`
	Score          int      `json:"score,omitempty"` // Search relevance score (0-100)
}

// ArtistSearchResponse is the response from MusicBrainz artist search.
type ArtistSearchResponse struct {
	Created string   `json:"created"`
	Count   int      `json:"count"`
	Offset  int      `json:"offset"`
	Artists []Artist `json:"artists"`
}

// ArtistCredit represents credited artists on a release or recording.
type ArtistCredit struct {
	Name       string  `json:"name,omitempty"`    // As credited (may differ from artist name)
	JoinPhrase string  `json:"joinphrase,omitempty"` // " & ", " feat. ", etc.
	Artist     *Artist `json:"artist,omitempty"`
}

// Medium represents a physical or digital medium (CD, vinyl, digital media).
type Medium struct {
	Position    int     `json:"position"`
	Format      string  `json:"format,omitempty"` // CD, Vinyl, Digital Media, etc.
	TrackCount  int     `json:"track-count"`
	Tracks      []Track `json:"tracks,omitempty"`
	Title       string  `json:"title,omitempty"` // For named discs
}

// Track represents a track on a medium.
type Track struct {
	ID        string     `json:"id"`
	Number    string     `json:"number"` // Track number (can be "A1", "1", etc.)
	Title     string     `json:"title"`
	Length    int        `json:"length,omitempty"` // Duration in milliseconds
	Position  int        `json:"position"`
	Recording *Recording `json:"recording,omitempty"`
}

// LabelInfo represents label and catalog number information.
type LabelInfo struct {
	CatalogNumber string `json:"catalog-number,omitempty"`
	Label         *Label `json:"label,omitempty"`
}

// Label represents a record label.
type Label struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SortName       string `json:"sort-name,omitempty"`
	LabelCode      int    `json:"label-code,omitempty"`
	Type           string `json:"type,omitempty"` // Distributor, Holding, Production, etc.
	Country        string `json:"country,omitempty"`
	Disambiguation string `json:"disambiguation,omitempty"`
}

// Area represents a geographic area.
type Area struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	SortName       string   `json:"sort-name,omitempty"`
	ISO3166_1Codes []string `json:"iso-3166-1-codes,omitempty"`
	Type           string   `json:"type,omitempty"`
}

// LifeSpan represents birth/death dates for a person or begin/end dates for a group.
type LifeSpan struct {
	Begin string `json:"begin,omitempty"`
	End   string `json:"end,omitempty"`
	Ended bool   `json:"ended,omitempty"`
}

// TextRepresentation represents the language and script of a release.
type TextRepresentation struct {
	Language string `json:"language,omitempty"` // ISO 639-3 language code
	Script   string `json:"script,omitempty"`   // ISO 15924 script code
}

// Tag represents a folksonomy tag with vote count.
type Tag struct {
	Name  string `json:"name"`
	Count int    `json:"count"` // Number of votes for this tag
}

// CoverArtArchive response types

// CoverArtResponse is the response from the Cover Art Archive API.
type CoverArtResponse struct {
	Images  []CoverArtImage `json:"images"`
	Release string          `json:"release"` // MusicBrainz release ID
}

// CoverArtImage represents a single image from Cover Art Archive.
type CoverArtImage struct {
	ID         int64            `json:"id"`
	Types      []string         `json:"types"` // Front, Back, Medium, Booklet, etc.
	Front      bool             `json:"front"`
	Back       bool             `json:"back"`
	Comment    string           `json:"comment,omitempty"`
	Image      string           `json:"image"`      // Full size image URL
	Thumbnails CoverArtThumbnails `json:"thumbnails"`
	Approved   bool             `json:"approved"`
	Edit       int              `json:"edit,omitempty"`
}

// CoverArtThumbnails contains thumbnail URLs for an image.
type CoverArtThumbnails struct {
	Small string `json:"small,omitempty"` // 250px
	Large string `json:"large,omitempty"` // 500px
	XL250 string `json:"250,omitempty"`   // Alternative 250px
	XL500 string `json:"500,omitempty"`   // Alternative 500px
	XL1200 string `json:"1200,omitempty"` // 1200px
}
