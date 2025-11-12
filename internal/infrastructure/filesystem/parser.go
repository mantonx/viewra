package filesystem

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
	"github.com/viewra/viewra/internal/domain/scanner"
)

// Parser implements scanner.FilenameParser
type Parser struct {
	// Compiled regex patterns for performance
	moviePattern   *regexp.Regexp
	tvPattern      *regexp.Regexp
	imdbPattern    *regexp.Regexp
	resPattern     *regexp.Regexp
	qualityPattern *regexp.Regexp
}

// NewParser creates a new filename parser with compiled regex patterns
func NewParser() *Parser {
	return &Parser{
		// Movie pattern: Title (Year) [metadata] - [quality].ext
		// Examples:
		// - Inception (2010) [imdbid-tt1375666] - [Remux-2160p][DTS-HD MA 5.1][HEVC]-4K4U.mkv
		// - 12 Angry Men (1957) [imdbid-tt0050083] - [Remux-2160p][DV HDR10][DTS-HD MA 2.0][HEVC]-HDH.mkv
		moviePattern: regexp.MustCompile(`^(.+?)\s*\((\d{4})\)`),

		// TV pattern: ShowName (Year) - SxxEyy - Episode Title [metadata].ext
		// Examples:
		// - Breaking Bad (2008) - S01E01 - Pilot [Bluray-1080p][EAC3 5.1][x265]-iVy.mkv
		// - 1883 (2021) - S01E02 - Behind Us A Cliff [Bluray-1080p][AAC 5.1][x265]-Vyndros.mkv
		tvPattern: regexp.MustCompile(
			`^(.+?)\s*\((\d{4})\)\s*-\s*[Ss](\d{2})[Ee](\d{2})(?:[Ee](\d{2}))?` +
				`\s*-\s*([^[\]]+?)(?:\s*\[|$)`),

		// IMDB ID pattern: [imdbid-ttXXXXXXX]
		imdbPattern: regexp.MustCompile(`\[imdbid-(tt\d+)\]`),

		// Resolution pattern: 1080p, 2160p, 4K, etc.
		resPattern: regexp.MustCompile(`\b(480|720|1080|2160|4K|8K)p?\b`),

		// Quality pattern: BluRay, Remux, WEB-DL, etc.
		qualityPattern: regexp.MustCompile(`\b(Remux|BluRay|Bluray|WEB-?DL|WEBDL|WEBRip|HDTV|DVDRip|BDRip)\b`),
	}
}

// stripExtensionAndPath removes the file extension and path from a filename
func stripExtensionAndPath(filename string) string {
	base := filepath.Base(filename)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// isValidYear checks if a year is within the valid range (1900-2099)
func isValidYear(year int) bool {
	return year >= 1900 && year <= 2099
}

// isValidTrackNumber checks if a track number is within the valid range (1-999)
func isValidTrackNumber(trackNum int) bool {
	return trackNum > 0 && trackNum <= 999
}

// ParseMovie extracts movie metadata from a filename
func (p *Parser) ParseMovie(filename string) (*scanner.MovieInfo, error) {
	// Remove file extension and path
	nameNoExt := stripExtensionAndPath(filename)

	// Try to match the standard pattern: Title (Year)
	matches := p.moviePattern.FindStringSubmatch(nameNoExt)
	if len(matches) < 3 {
		// Not a valid movie filename
		return nil, nil
	}

	title := strings.TrimSpace(matches[1])
	yearStr := matches[2]

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return nil, nil
	}

	// Validate year range
	if !isValidYear(year) {
		return nil, nil
	}

	info := &scanner.MovieInfo{
		Title: title,
		Year:  year,
	}

	// Extract optional metadata from brackets and tags
	p.extractMovieMetadata(nameNoExt, info)

	return info, nil
}

// extractMovieMetadata extracts resolution, quality, and other metadata
func (p *Parser) extractMovieMetadata(filename string, info *scanner.MovieInfo) {
	// Extract resolution
	if matches := p.resPattern.FindStringSubmatch(filename); matches != nil {
		res := matches[1]
		// Normalize 4K/8K to pixel height
		switch res {
		case "4K":
			info.Resolution = "2160p"
		case "8K":
			info.Resolution = "4320p"
		default:
			info.Resolution = res + "p"
		}
	}

	// Extract quality
	if matches := p.qualityPattern.FindStringSubmatch(filename); matches != nil {
		info.Quality = matches[1]
		// Normalize variations
		if strings.EqualFold(info.Quality, "Bluray") {
			info.Quality = "BluRay"
		} else if strings.Contains(strings.ToLower(info.Quality), "webdl") {
			info.Quality = "WEB-DL"
		}
	}
}

// ParseTVEpisode extracts TV show metadata from a filename
func (p *Parser) ParseTVEpisode(filename string) (*scanner.TVEpisodeInfo, error) {
	// Remove file extension and path
	nameNoExt := stripExtensionAndPath(filename)

	// Try to match the standard pattern: ShowName (Year) - SxxEyy - Episode Title
	matches := p.tvPattern.FindStringSubmatch(nameNoExt)
	if len(matches) < 7 {
		// Not a valid TV episode filename
		return nil, nil
	}

	showName := strings.TrimSpace(matches[1])
	yearStr := matches[2]
	seasonStr := matches[3]
	episodeStr := matches[4]
	episodeEndStr := matches[5] // Optional second episode (for multi-episode files)
	episodeTitle := strings.TrimSpace(matches[6])

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return nil, nil
	}

	season, err := strconv.Atoi(seasonStr)
	if err != nil {
		return nil, nil
	}

	episode, err := strconv.Atoi(episodeStr)
	if err != nil {
		return nil, nil
	}

	// Validate ranges
	if !isValidYear(year) {
		return nil, nil
	}
	if season < 0 || season > 99 {
		return nil, nil
	}
	if episode < 1 || episode > 999 {
		return nil, nil
	}

	info := &scanner.TVEpisodeInfo{
		ShowName:     showName,
		Year:         year,
		Season:       season,
		Episode:      episode,
		EpisodeTitle: episodeTitle,
	}

	// Handle multi-episode files (e.g., S01E01E02)
	if episodeEndStr != "" {
		if episodeEnd, err := strconv.Atoi(episodeEndStr); err == nil {
			if episodeEnd > episode && episodeEnd <= 999 {
				info.EpisodeEnd = episodeEnd
			}
		}
	}

	return info, nil
}

// ParseMusic extracts music metadata from a file
// Priority: ID3 tags > filename parsing
func (p *Parser) ParseMusic(path string) (*scanner.MusicInfo, error) {
	// Try ID3 tags first (preferred method)
	if info, err := p.parseID3Tags(path); err == nil && info != nil {
		return info, nil
	}

	// Fallback to filename parsing
	return p.parseMusicFilename(path), nil
}

// parseID3Tags reads metadata from ID3 tags
func (p *Parser) parseID3Tags(path string) (*scanner.MusicInfo, error) {
	file, cleanup, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	metadata, err := tag.ReadFrom(file)
	if err != nil {
		return nil, err
	}

	info := &scanner.MusicInfo{
		Title:       metadata.Title(),
		Artist:      metadata.Artist(),
		Album:       metadata.Album(),
		AlbumArtist: metadata.AlbumArtist(),
		Genre:       metadata.Genre(),
	}

	// Extract track number
	track, _ := metadata.Track()
	info.TrackNumber = track

	// Extract year
	info.Year = metadata.Year()

	// Only return if we got meaningful data
	if info.Title != "" || info.Artist != "" {
		return info, nil
	}

	return nil, nil
}

// parseMusicFilename extracts metadata from filename as fallback
// Pattern: Artist - Album - TrackNum - Title.ext
// Example: Arcade Fire - Funeral - 01 - Neighborhood #1 (Tunnels).flac
func (p *Parser) parseMusicFilename(path string) *scanner.MusicInfo {
	nameNoExt := stripExtensionAndPath(path)

	// Try pattern: Artist - Album - TrackNum - Title
	parts := strings.Split(nameNoExt, " - ")
	if len(parts) >= 4 {
		trackStr := strings.TrimSpace(parts[2])
		trackNum, err := strconv.Atoi(trackStr)
		if err == nil && isValidTrackNumber(trackNum) {
			return &scanner.MusicInfo{
				Artist:      strings.TrimSpace(parts[0]),
				Album:       strings.TrimSpace(parts[1]),
				TrackNumber: trackNum,
				Title:       strings.TrimSpace(strings.Join(parts[3:], " - ")),
			}
		}
	}

	// Try simpler pattern: TrackNum - Title
	if len(parts) >= 2 {
		trackStr := strings.TrimSpace(parts[0])
		trackNum, err := strconv.Atoi(trackStr)
		if err == nil && isValidTrackNumber(trackNum) {
			return &scanner.MusicInfo{
				TrackNumber: trackNum,
				Title:       strings.TrimSpace(strings.Join(parts[1:], " - ")),
			}
		}
	}

	// Fallback: use filename as title
	return &scanner.MusicInfo{
		Title: nameNoExt,
	}
}
