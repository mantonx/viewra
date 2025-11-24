package nfo

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
)

// TVShowNFO represents a Kodi/Plex-compatible tvshow.nfo file structure
type TVShowNFO struct {
	XMLName xml.Name `xml:"tvshow"`

	// Basic information
	Title         string `xml:"title"`
	OriginalTitle string `xml:"originaltitle"`
	SortTitle     string `xml:"sorttitle"`
	Year          int    `xml:"year"`
	Premiered     string `xml:"premiered"`
	Plot          string `xml:"plot"`
	Outline       string `xml:"outline"`
	Tagline       string `xml:"tagline"`

	// Status
	Status string `xml:"status"` // Continuing, Ended, etc.

	// Ratings
	Rating     float32 `xml:"rating"`     // Legacy single rating
	UserRating float32 `xml:"userrating"` // User's rating

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
	ID     string `xml:"id"` // Legacy ID field
	IMDb   string `xml:"imdb"`
	TVDbID string `xml:"tvdbid"`
	TMDbID string `xml:"tmdbid"`

	// UniqueID (new format)
	UniqueIDs []UniqueID `xml:"uniqueid"`

	// People
	Actors []Actor `xml:"actor"`

	// Production
	Studio           string `xml:"studio"`
	Network          string `xml:"network"` // TV network
	Country          string `xml:"country"`
	OriginalLanguage string `xml:"originallanguage"`

	// Categories
	Genres []string `xml:"genre"`
	Tags   []string `xml:"tag"`

	// Air information
	AirDay  string `xml:"airday"`
	AirTime string `xml:"airtime"`

	// Metadata
	EpisodeGuide string `xml:"episodeguide"`
	Season       int    `xml:"season"` // For season-specific NFOs
}

// EpisodeNFO represents a Kodi/Plex-compatible episode.nfo file structure
type EpisodeNFO struct {
	XMLName xml.Name `xml:"episodedetails"`

	// Basic information
	Title         string `xml:"title"`
	OriginalTitle string `xml:"originaltitle"`
	ShowTitle     string `xml:"showtitle"`
	Season        int    `xml:"season"`
	Episode       int    `xml:"episode"`
	DisplaySeason int    `xml:"displayseason"` // For specials
	DisplayEpisode int   `xml:"displayepisode"` // For specials
	Aired         string `xml:"aired"`
	Plot          string `xml:"plot"`
	Outline       string `xml:"outline"`

	// Ratings
	Rating     float32 `xml:"rating"`
	UserRating float32 `xml:"userrating"`

	// Ratings (new format)
	Ratings struct {
		Rating []Rating `xml:"rating"`
	} `xml:"ratings"`

	// IDs
	ID     string `xml:"id"`
	IMDb   string `xml:"imdb"`
	TVDbID string `xml:"tvdbid"`
	TMDbID string `xml:"tmdbid"`

	// UniqueID (new format)
	UniqueIDs []UniqueID `xml:"uniqueid"`

	// People
	Director string  `xml:"director"`
	Credits  []string `xml:"credits"` // Writers
	Actors   []Actor  `xml:"actor"`

	// Runtime
	Runtime string `xml:"runtime"` // In minutes

	// Metadata
	Watched   bool `xml:"watched"`
	PlayCount int  `xml:"playcount"`
}

// TVShowMetadata represents the extracted show-level metadata
type TVShowMetadata struct {
	Title            string
	OriginalTitle    string
	SortTitle        string
	Year             int
	PremiereDate     time.Time
	Plot             string
	Tagline          string
	Status           string
	Cast             []string
	Genre            []string
	ContentRating    string
	MaturityRating   int
	IMDbID           string
	TVDbID           int
	TMDbID           int
	Network          string
	OriginalLanguage string
	CountryOfOrigin  string
	AirDay           string
	AirTime          string
}

// EpisodeMetadata represents the extracted episode-level metadata
type EpisodeMetadata struct {
	Title          string
	ShowTitle      string
	Season         int
	Episode        int
	DisplaySeason  int
	DisplayEpisode int
	AirDate        time.Time
	Plot           string
	RuntimeMinutes int
	Director       string
	Writers        []string
	GuestStars     []string
	Rating         float32
	IMDbID         string
	TVDbID         int
	TMDbID         int
}

// ParseTVShowNFO parses a tvshow.nfo file and returns extracted metadata
func ParseTVShowNFO(nfoPath string) (*TVShowMetadata, error) {
	// Read the NFO file
	data, err := os.ReadFile(nfoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read NFO file: %w", err)
	}

	// Parse XML
	var nfo TVShowNFO
	if err := xml.Unmarshal(data, &nfo); err != nil {
		return nil, fmt.Errorf("failed to parse NFO XML: %w", err)
	}

	// Convert to our metadata format
	// Use provided sort title if available, otherwise normalize the title
	sortTitle := nfo.SortTitle
	if sortTitle == "" {
		sortTitle = domainCommon.NormalizeSortTitle(nfo.Title)
	} else {
		// Even if sort title is provided, normalize it for consistent sorting
		sortTitle = domainCommon.NormalizeSortTitle(sortTitle)
	}

	metadata := &TVShowMetadata{
		Title:            nfo.Title,
		OriginalTitle:    nfo.OriginalTitle,
		SortTitle:        sortTitle,
		Year:             nfo.Year,
		Plot:             nfo.Plot,
		Tagline:          nfo.Tagline,
		Status:           nfo.Status,
		Network:          nfo.Network,
		OriginalLanguage: nfo.OriginalLanguage,
		CountryOfOrigin:  nfo.Country,
		AirDay:           nfo.AirDay,
		AirTime:          nfo.AirTime,
	}

	// Parse premiere date
	if nfo.Premiered != "" {
		if t, err := parseDate(nfo.Premiered); err == nil {
			metadata.PremiereDate = t
		}
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

	// Parse maturity rating
	if nfo.Rating > 0 {
		metadata.MaturityRating = int(nfo.Rating)
	} else if len(nfo.Ratings.Rating) > 0 {
		metadata.MaturityRating = int(getBestRating(nfo.Ratings.Rating))
	}

	// Extract IDs - try multiple fields
	if nfo.IMDb != "" {
		metadata.IMDbID = cleanIMDbID(nfo.IMDb)
	}
	metadata.TVDbID = parseIntSafe(nfo.TVDbID)
	metadata.TMDbID = parseIntSafe(nfo.TMDbID)

	// Check new UniqueID format using common helper
	if metadata.IMDbID == "" {
		if imdbID := parseIDFromUniqueIDs(nfo.UniqueIDs, "imdb"); imdbID != "" {
			metadata.IMDbID = cleanIMDbID(imdbID)
		}
	}
	if metadata.TVDbID == 0 {
		if tvdbID := parseIDFromUniqueIDs(nfo.UniqueIDs, "tvdb"); tvdbID != "" {
			metadata.TVDbID = parseIntSafe(tvdbID)
		}
	}
	if metadata.TMDbID == 0 {
		if tmdbID := parseIDFromUniqueIDs(nfo.UniqueIDs, "tmdb"); tmdbID != "" {
			metadata.TMDbID = parseIntSafe(tmdbID)
		}
	}

	return metadata, nil
}

// ParseEpisodeNFO parses an episode.nfo file and returns extracted metadata
func ParseEpisodeNFO(nfoPath string) (*EpisodeMetadata, error) {
	// Read the NFO file
	data, err := os.ReadFile(nfoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read NFO file: %w", err)
	}

	// Parse XML
	var nfo EpisodeNFO
	if err := xml.Unmarshal(data, &nfo); err != nil {
		return nil, fmt.Errorf("failed to parse NFO XML: %w", err)
	}

	// Convert to our metadata format
	metadata := &EpisodeMetadata{
		Title:          nfo.Title,
		ShowTitle:      nfo.ShowTitle,
		Season:         nfo.Season,
		Episode:        nfo.Episode,
		DisplaySeason:  nfo.DisplaySeason,
		DisplayEpisode: nfo.DisplayEpisode,
		Plot:           nfo.Plot,
		Director:       nfo.Director,
		Writers:        nfo.Credits,
	}

	// Parse air date
	if nfo.Aired != "" {
		if t, err := parseDate(nfo.Aired); err == nil {
			metadata.AirDate = t
		}
	}

	// Parse runtime
	if nfo.Runtime != "" {
		metadata.RuntimeMinutes = parseRuntime(nfo.Runtime)
	}

	// Extract guest stars (actors in episode NFO are typically guest stars)
	metadata.GuestStars = extractActorNames(nfo.Actors)

	// Parse rating
	if nfo.Rating > 0 {
		metadata.Rating = nfo.Rating
	} else if len(nfo.Ratings.Rating) > 0 {
		metadata.Rating = getBestRating(nfo.Ratings.Rating)
	}

	// Extract IDs
	if nfo.IMDb != "" {
		metadata.IMDbID = cleanIMDbID(nfo.IMDb)
	}
	metadata.TVDbID = parseIntSafe(nfo.TVDbID)
	metadata.TMDbID = parseIntSafe(nfo.TMDbID)

	// Check new UniqueID format
	if metadata.IMDbID == "" {
		if imdbID := parseIDFromUniqueIDs(nfo.UniqueIDs, "imdb"); imdbID != "" {
			metadata.IMDbID = cleanIMDbID(imdbID)
		}
	}
	if metadata.TVDbID == 0 {
		if tvdbID := parseIDFromUniqueIDs(nfo.UniqueIDs, "tvdb"); tvdbID != "" {
			metadata.TVDbID = parseIntSafe(tvdbID)
		}
	}
	if metadata.TMDbID == 0 {
		if tmdbID := parseIDFromUniqueIDs(nfo.UniqueIDs, "tmdb"); tmdbID != "" {
			metadata.TMDbID = parseIntSafe(tmdbID)
		}
	}

	return metadata, nil
}

// FindTVShowNFO searches for a tvshow.nfo file in a show directory
func FindTVShowNFO(showDir string) (string, error) {
	// Check for tvshow.nfo directly
	nfoPath := filepath.Join(showDir, "tvshow.nfo")
	if fileExists(nfoPath) {
		return nfoPath, nil
	}

	// Look for any .nfo file in the directory
	entries, err := os.ReadDir(showDir)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".nfo") {
			// Ignore episode NFOs
			if !strings.Contains(strings.ToLower(entry.Name()), "episode") {
				return filepath.Join(showDir, entry.Name()), nil
			}
		}
	}

	return "", fmt.Errorf("no tvshow NFO file found in: %s", showDir)
}

// FindEpisodeNFO searches for an episode.nfo file associated with an episode file
func FindEpisodeNFO(episodePath string) (string, error) {
	return FindNFOFile(episodePath, "episode.nfo")
}

// DetermineShowDirectory extracts the TV show directory from an episode file path
// Handles two common structures:
// 1. With season subdirs: /path/to/show/Season 01/episode.mkv -> /path/to/show
// 2. Without season subdirs: /path/to/show/episode.mkv -> /path/to/show
func DetermineShowDirectory(episodeFilePath string) string {
	// Get the directory containing the episode file
	episodeDir := filepath.Dir(episodeFilePath)
	dirName := filepath.Base(episodeDir)

	// Check if episode is in a season subdirectory (e.g., "Season 01", "Season 1", "S01")
	lowerDirName := strings.ToLower(dirName)
	if strings.HasPrefix(lowerDirName, "season") || strings.HasPrefix(lowerDirName, "s") && len(dirName) <= 4 {
		// Episode is in a season subdir, so parent dir is the show dir
		return filepath.Dir(episodeDir)
	}

	// Episode is directly in the show directory
	return episodeDir
}
