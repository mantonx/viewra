package nfo

import (
	"encoding/xml"
	"fmt"
	"os"
	"time"
)

// MovieNFO represents a Kodi/Plex-compatible movie .nfo file structure
type MovieNFO struct {
	XMLName xml.Name `xml:"movie"`

	// Basic information
	Title         string `xml:"title"`
	OriginalTitle string `xml:"originaltitle"`
	SortTitle     string `xml:"sorttitle"`
	Year          int    `xml:"year"`
	ReleaseDate   string `xml:"releasedate"`
	Plot          string `xml:"plot"`
	Outline       string `xml:"outline"` // Short plot summary
	Tagline       string `xml:"tagline"`

	// Runtime
	Runtime string `xml:"runtime"` // In minutes, sometimes with "min" suffix

	// Ratings
	Rating       float32 `xml:"rating"`       // Legacy single rating
	UserRating   float32 `xml:"userrating"`   // User's rating
	CriticRating float32 `xml:"criticrating"` // Critic rating

	// Ratings (new format)
	Ratings struct {
		Rating []Rating `xml:"rating"`
	} `xml:"ratings"`

	// Content ratings
	MPAARating     string `xml:"mpaa"`
	Certification  string `xml:"certification"`
	ContentRating  string `xml:"contentrating"`
	MaturityRating string `xml:"maturityrating"`

	// IDs
	ID     string `xml:"id"` // Legacy ID field (often IMDb)
	IMDb   string `xml:"imdb"` // IMDb ID
	TMDbID string `xml:"tmdbid"`

	// UniqueID (new format)
	UniqueIDs []UniqueID `xml:"uniqueid"`

	// People
	Director string   `xml:"director"`
	Credits  []string `xml:"credits"` // Writers
	Actors   []Actor  `xml:"actor"`

	// Production
	Studio           string `xml:"studio"`
	Country          string `xml:"country"`
	OriginalLanguage string `xml:"originallanguage"`

	// Financial
	Budget  string `xml:"budget"`  // Sometimes includes currency
	Revenue string `xml:"revenue"` // Sometimes includes currency

	// Categories
	Genres []string `xml:"genre"`
	Tags   []string `xml:"tag"`
	Sets   []string `xml:"set"` // Collections

	// Awards
	Awards string `xml:"awards"`

	// Additional metadata
	Premiered string `xml:"premiered"`
	Status    string `xml:"status"` // Released, etc.
	Watched   bool   `xml:"watched"`
	PlayCount int    `xml:"playcount"`

	// Art (simple format - we don't need complex art metadata for now)
	Poster string `xml:"poster"`
}

// MovieMetadata represents the extracted metadata we care about
type MovieMetadata struct {
	Title             string
	OriginalTitle     string
	SortTitle         string
	Year              int
	ReleaseDate       time.Time
	Plot              string
	Tagline           string
	RuntimeMinutes    int
	Director          string
	Cast              []string
	Genre             []string
	ContentRating     string
	MaturityRating    int
	ContentAdvisories []string
	IMDbID            string
	TMDbID            int
	Budget            int64
	Revenue           int64
	OriginalLanguage  string
	CountryOfOrigin   string
	AwardsSummary     string
}

// ParseMovieNFO parses a movie .nfo file and returns extracted metadata
func ParseMovieNFO(nfoPath string) (*MovieMetadata, error) {
	// Read the NFO file
	data, err := os.ReadFile(nfoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read NFO file: %w", err)
	}

	// Parse XML
	var nfo MovieNFO
	if err := xml.Unmarshal(data, &nfo); err != nil {
		return nil, fmt.Errorf("failed to parse NFO XML: %w", err)
	}

	// Convert to our metadata format
	metadata := &MovieMetadata{
		Title:            nfo.Title,
		OriginalTitle:    nfo.OriginalTitle,
		SortTitle:        nfo.SortTitle,
		Year:             nfo.Year,
		Plot:             nfo.Plot,
		Tagline:          nfo.Tagline,
		Director:         nfo.Director,
		OriginalLanguage: nfo.OriginalLanguage,
		CountryOfOrigin:  nfo.Country,
		AwardsSummary:    nfo.Awards,
	}

	// Parse release date
	if nfo.ReleaseDate != "" {
		if t, err := parseDate(nfo.ReleaseDate); err == nil {
			metadata.ReleaseDate = t
		}
	} else if nfo.Premiered != "" {
		if t, err := parseDate(nfo.Premiered); err == nil {
			metadata.ReleaseDate = t
		}
	}

	// Parse runtime (can be "90", "90 min", etc.)
	if nfo.Runtime != "" {
		metadata.RuntimeMinutes = parseRuntime(nfo.Runtime)
	}

	// Extract cast names using common helper
	metadata.Cast = extractActorNames(nfo.Actors)

	// Extract genres
	metadata.Genre = nfo.Genres

	// Extract content rating
	if nfo.MPAARating != "" {
		metadata.ContentRating = nfo.MPAARating
	} else if nfo.Certification != "" {
		metadata.ContentRating = nfo.Certification
	} else if nfo.ContentRating != "" {
		metadata.ContentRating = nfo.ContentRating
	}

	// Parse maturity rating (convert rating to 0-10 scale)
	if nfo.Rating > 0 {
		metadata.MaturityRating = int(nfo.Rating)
	}

	// Extract IDs - try multiple fields
	if nfo.IMDb != "" {
		metadata.IMDbID = cleanIMDbID(nfo.IMDb)
	} else if nfo.ID != "" {
		metadata.IMDbID = cleanIMDbID(nfo.ID)
	}
	metadata.TMDbID = parseIntSafe(nfo.TMDbID)

	// Check new UniqueID format using common helper
	if metadata.IMDbID == "" {
		if imdbID := parseIDFromUniqueIDs(nfo.UniqueIDs, "imdb"); imdbID != "" {
			metadata.IMDbID = cleanIMDbID(imdbID)
		}
	}
	if metadata.TMDbID == 0 {
		if tmdbID := parseIDFromUniqueIDs(nfo.UniqueIDs, "tmdb"); tmdbID != "" {
			metadata.TMDbID = parseIntSafe(tmdbID)
		}
	}

	// Parse budget and revenue (remove currency symbols)
	metadata.Budget = parseMoney(nfo.Budget)
	metadata.Revenue = parseMoney(nfo.Revenue)

	// Content advisories from tags
	metadata.ContentAdvisories = nfo.Tags

	return metadata, nil
}

// FindMovieNFO searches for a .nfo file associated with a movie file
// Uses the common FindNFOFile helper with movie-specific filenames
func FindMovieNFO(moviePath string) (string, error) {
	return FindNFOFile(moviePath, "movie.nfo")
}
