package audiodb

import (
	"strconv"
	"strings"

	"github.com/mantonx/viewra/internal/plugins/proto"
)

// Mapper handles conversion between AudioDB responses and plugin formats
type Mapper struct{}

// NewMapper creates a new mapper instance
func NewMapper() *Mapper {
	return &Mapper{}
}

// TrackToSearchResult converts an AudioDB track to a SearchResult
func (m *Mapper) TrackToSearchResult(track Track, score float64) *proto.SearchResult {
	return &proto.SearchResult{
		Id:     track.IDTrack,
		Title:  track.StrTrack,
		Artist: track.StrArtist,
		Album:  track.StrAlbum,
		Score:  score,
		Metadata: map[string]string{
			"audiodb_track_id":  track.IDTrack,
			"audiodb_artist_id": track.IDArtist,
			"audiodb_album_id":  track.IDAlbum,
			"genre":             track.StrGenre,
			"mood":              track.StrMood,
			"style":             track.StrStyle,
			"theme":             track.StrTheme,
			"duration":          track.IntDuration,
			"track_number":      track.IntTrackNumber,
			"description":       track.StrDescriptionEN,
			"musicbrainz_id":    track.StrMusicBrainzID,
		},
	}
}

// ExtractMetadataFromTrack creates a metadata map from an AudioDB track
func (m *Mapper) ExtractMetadataFromTrack(track Track) map[string]interface{} {
	metadata := make(map[string]interface{})
	
	// Basic track information
	if track.StrTrack != "" {
		metadata["title"] = track.StrTrack
	}
	if track.StrArtist != "" {
		metadata["artist"] = track.StrArtist
	}
	if track.StrAlbum != "" {
		metadata["album"] = track.StrAlbum
	}
	
	// Genre and style information
	if track.StrGenre != "" {
		metadata["genre"] = track.StrGenre
	}
	if track.StrStyle != "" {
		metadata["style"] = track.StrStyle
	}
	if track.StrMood != "" {
		metadata["mood"] = track.StrMood
	}
	if track.StrTheme != "" {
		metadata["theme"] = track.StrTheme
	}
	
	// Technical information
	if track.IntDuration != "" {
		if duration, err := strconv.Atoi(track.IntDuration); err == nil {
			metadata["duration_seconds"] = duration
		}
	}
	if track.IntTrackNumber != "" {
		if trackNum, err := strconv.Atoi(track.IntTrackNumber); err == nil {
			metadata["track_number"] = trackNum
		}
	}
	if track.IntCD != "" {
		if cd, err := strconv.Atoi(track.IntCD); err == nil {
			metadata["disc_number"] = cd
		}
	}
	
	// Description
	if track.StrDescriptionEN != "" {
		metadata["description"] = track.StrDescriptionEN
	}
	
	// External IDs
	if track.StrMusicBrainzID != "" {
		metadata["musicbrainz_track_id"] = track.StrMusicBrainzID
	}
	if track.StrMusicBrainzAlbumID != "" {
		metadata["musicbrainz_album_id"] = track.StrMusicBrainzAlbumID
	}
	if track.StrMusicBrainzArtistID != "" {
		metadata["musicbrainz_artist_id"] = track.StrMusicBrainzArtistID
	}
	
	// AudioDB specific IDs
	metadata["audiodb_track_id"] = track.IDTrack
	metadata["audiodb_artist_id"] = track.IDArtist
	metadata["audiodb_album_id"] = track.IDAlbum
	
	return metadata
}

// ExtractMetadataFromAlbum creates a metadata map from an AudioDB album
func (m *Mapper) ExtractMetadataFromAlbum(album Album) map[string]interface{} {
	metadata := make(map[string]interface{})
	
	// Basic album information
	if album.StrAlbum != "" {
		metadata["album"] = album.StrAlbum
	}
	if album.StrArtist != "" {
		metadata["album_artist"] = album.StrArtist
	}
	
	// Release information
	if album.IntYearReleased != "" {
		if year, err := strconv.Atoi(album.IntYearReleased); err == nil {
			metadata["year"] = year
			metadata["release_year"] = year
		}
	}
	if album.StrLabel != "" {
		metadata["label"] = album.StrLabel
	}
	if album.StrReleaseFormat != "" {
		metadata["release_format"] = album.StrReleaseFormat
	}
	
	// Genre and style
	if album.StrGenre != "" {
		metadata["album_genre"] = album.StrGenre
	}
	if album.StrStyle != "" {
		metadata["album_style"] = album.StrStyle
	}
	if album.StrMood != "" {
		metadata["album_mood"] = album.StrMood
	}
	
	// Artwork URLs
	if album.StrAlbumThumb != "" {
		metadata["artwork_thumb"] = album.StrAlbumThumb
	}
	if album.StrAlbumThumbHQ != "" {
		metadata["artwork_hq"] = album.StrAlbumThumbHQ
	}
	if album.StrAlbumThumbBack != "" {
		metadata["artwork_back"] = album.StrAlbumThumbBack
	}
	
	// Description
	if album.StrDescriptionEN != "" {
		metadata["album_description"] = album.StrDescriptionEN
	}
	
	// External IDs
	if album.StrMusicBrainzID != "" {
		metadata["musicbrainz_album_id"] = album.StrMusicBrainzID
	}
	if album.StrMusicBrainzArtistID != "" {
		metadata["musicbrainz_artist_id"] = album.StrMusicBrainzArtistID
	}
	
	// AudioDB IDs
	metadata["audiodb_album_id"] = album.IDAlbum
	metadata["audiodb_artist_id"] = album.IDArtist
	
	return metadata
}

// ExtractMetadataFromArtist creates a metadata map from an AudioDB artist
func (m *Mapper) ExtractMetadataFromArtist(artist Artist) map[string]interface{} {
	metadata := make(map[string]interface{})
	
	// Basic artist information
	if artist.StrArtist != "" {
		metadata["artist"] = artist.StrArtist
	}
	if artist.StrArtistAlternate != "" {
		metadata["artist_alternate"] = artist.StrArtistAlternate
	}
	
	// Formation/birth information
	if artist.IntFormedYear != "" {
		if year, err := strconv.Atoi(artist.IntFormedYear); err == nil {
			metadata["formed_year"] = year
		}
	}
	if artist.IntBornYear != "" {
		if year, err := strconv.Atoi(artist.IntBornYear); err == nil {
			metadata["born_year"] = year
		}
	}
	if artist.IntDiedYear != "" {
		if year, err := strconv.Atoi(artist.IntDiedYear); err == nil {
			metadata["died_year"] = year
		}
	}
	
	// Genre and style
	if artist.StrGenre != "" {
		metadata["artist_genre"] = artist.StrGenre
	}
	if artist.StrStyle != "" {
		metadata["artist_style"] = artist.StrStyle
	}
	if artist.StrMood != "" {
		metadata["artist_mood"] = artist.StrMood
	}
	
	// Location and label
	if artist.StrCountry != "" {
		metadata["country"] = artist.StrCountry
	}
	if artist.StrLabel != "" {
		metadata["artist_label"] = artist.StrLabel
	}
	
	// Biography
	if artist.StrBiographyEN != "" {
		metadata["biography"] = artist.StrBiographyEN
	}
	
	// Social media and web presence
	if artist.StrWebsite != "" {
		metadata["website"] = artist.StrWebsite
	}
	if artist.StrFacebook != "" {
		metadata["facebook"] = artist.StrFacebook
	}
	if artist.StrTwitter != "" {
		metadata["twitter"] = artist.StrTwitter
	}
	
	// Artwork
	if artist.StrArtistThumb != "" {
		metadata["artist_thumb"] = artist.StrArtistThumb
	}
	if artist.StrArtistLogo != "" {
		metadata["artist_logo"] = artist.StrArtistLogo
	}
	if artist.StrArtistFanart != "" {
		metadata["artist_fanart"] = artist.StrArtistFanart
	}
	
	// External IDs
	if artist.StrMusicBrainzID != "" {
		metadata["musicbrainz_artist_id"] = artist.StrMusicBrainzID
	}
	
	// AudioDB ID
	metadata["audiodb_artist_id"] = artist.IDArtist
	
	return metadata
}

// CalculateMatchScore computes a similarity score between query and result
func (m *Mapper) CalculateMatchScore(queryTitle, queryArtist, queryAlbum, resultTitle, resultArtist, resultAlbum string) float64 {
	// Normalize strings for comparison
	queryTitle = strings.ToLower(strings.TrimSpace(queryTitle))
	queryArtist = strings.ToLower(strings.TrimSpace(queryArtist))
	queryAlbum = strings.ToLower(strings.TrimSpace(queryAlbum))
	resultTitle = strings.ToLower(strings.TrimSpace(resultTitle))
	resultArtist = strings.ToLower(strings.TrimSpace(resultArtist))
	resultAlbum = strings.ToLower(strings.TrimSpace(resultAlbum))
	
	// Calculate individual similarities
	titleScore := m.stringSimilarity(queryTitle, resultTitle)
	artistScore := m.stringSimilarity(queryArtist, resultArtist)
	
	// Weight: title=40%, artist=40%, album=20%
	score := titleScore*0.4 + artistScore*0.4
	
	// Include album score if both album names are provided
	if queryAlbum != "" && resultAlbum != "" {
		albumScore := m.stringSimilarity(queryAlbum, resultAlbum)
		score = titleScore*0.35 + artistScore*0.35 + albumScore*0.3
	}
	
	return score
}

// stringSimilarity calculates Levenshtein-based similarity between two strings
func (m *Mapper) stringSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}
	
	if maxLen == 0 {
		return 1.0
	}
	
	distance := m.levenshteinDistance(s1, s2)
	return 1.0 - float64(distance)/float64(maxLen)
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func (m *Mapper) levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}
	
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}
	
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}
	
	return matrix[len(s1)][len(s2)]
}

// min returns the minimum of three integers
func min(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
} 