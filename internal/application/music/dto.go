package music

import (
	"time"

	appcommon "github.com/mantonx/viewra/internal/application/common"
	"github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
)

// ArtistSummary represents an artist with aggregated metadata (for list view)
type ArtistSummary struct {
	ID         int64           `json:"id" binding:"required"` // Representative media_id (first track)
	Name       string          `json:"name" binding:"required"`
	AlbumCount int             `json:"album_count"`
	TrackCount int             `json:"track_count"`
	CreatedAt  time.Time       `json:"created_at"`
	Albums     map[string]bool `json:"-"` // Internal use only for aggregation
}

// AlbumSummary represents an album with aggregated metadata (for list view)
type AlbumSummary struct {
	ID         int64     `json:"id"`        // Album entity ID
	ArtistID   int64     `json:"artist_id"` // Artist entity ID for navigation
	Album      string    `json:"album"`
	Artist     string    `json:"artist"`
	Year       int       `json:"year,omitempty"`
	TrackCount int       `json:"track_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// MusicTrackResponse represents a music track with all its metadata
type MusicTrackResponse struct {
	// Base Media fields
	ID        int64  `json:"id"`
	LibraryID int64  `json:"library_id"`
	Title     string `json:"title"`
	FilePath  string `json:"file_path"`
	FileSize  int64  `json:"file_size"`
	Duration  int    `json:"duration"` // in seconds
	IsExtra   bool   `json:"is_extra"`

	// Technical metadata
	Width           int     `json:"width,omitempty"`
	Height          int     `json:"height,omitempty"`
	VideoCodec      string  `json:"video_codec,omitempty"`
	AudioCodec      string  `json:"audio_codec,omitempty"`
	MediaBitrate    int64   `json:"media_bitrate,omitempty"` // From base media
	FrameRate       float64 `json:"frame_rate,omitempty"`
	ContainerFormat string  `json:"container_format,omitempty"`

	// Music Track-specific fields
	AlbumID     int64  `json:"album_id,omitempty"`  // Album entity ID for navigation
	ArtistID    int64  `json:"artist_id,omitempty"` // Artist entity ID for navigation
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	AlbumArtist string `json:"album_artist,omitempty"`
	TrackNumber int    `json:"track_number,omitempty"`
	DiscNumber  int    `json:"disc_number,omitempty"`
	Year        int    `json:"year,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Composer    string `json:"composer,omitempty"`
	Publisher   string `json:"publisher,omitempty"`
	Bitrate     int    `json:"bitrate,omitempty"` // Music-specific bitrate in kbps

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListArtistsResponse represents a list of artists with summary data
type ListArtistsResponse struct {
	Artists    []ArtistSummary            `json:"artists"`
	Total      int                        `json:"total"`
	Pagination *common.PaginationMetadata `json:"pagination,omitempty"`
}

// ListAlbumsResponse represents a list of albums with summary data
type ListAlbumsResponse struct {
	Albums     []AlbumSummary             `json:"albums"`
	Total      int                        `json:"total"`
	Pagination *common.PaginationMetadata `json:"pagination,omitempty"`
}

// ListTracksResponse represents a list of music tracks
type ListTracksResponse struct {
	Tracks     []MusicTrackResponse       `json:"tracks"`
	Total      int                        `json:"total"`
	Pagination *common.PaginationMetadata `json:"pagination,omitempty"`
}

// ToMusicTrackResponse converts a domain music track entity to a DTO
func ToMusicTrackResponse(t *media.MusicTrack) MusicTrackResponse {
	return MusicTrackResponse{
		ID:              t.ID,
		LibraryID:       t.LibraryID,
		Title:           t.Title,
		FilePath:        t.FilePath,
		FileSize:        t.FileSize,
		Duration:        t.Duration,
		IsExtra:         t.IsExtra,
		Width:           t.Width,
		Height:          t.Height,
		VideoCodec:      t.VideoCodec,
		AudioCodec:      t.AudioCodec,
		MediaBitrate:    t.Media.Bitrate, // Base media bitrate
		FrameRate:       t.FrameRate,
		ContainerFormat: t.ContainerFormat,
		AlbumID:         t.AlbumID,
		ArtistID:        t.ArtistID,
		Artist:          t.Artist,
		Album:           t.Album,
		AlbumArtist:     t.AlbumArtist,
		TrackNumber:     t.TrackNumber,
		DiscNumber:      t.DiscNumber,
		Year:            t.Year,
		Genre:           t.Genre,
		Composer:        t.Composer,
		Publisher:       t.Publisher,
		Bitrate:         t.Bitrate, // Music-specific bitrate
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

// ToListTracksResponse converts a list of domain music tracks to list response DTO
func ToListTracksResponse(tracks []*media.MusicTrack) ListTracksResponse {
	responses := make([]MusicTrackResponse, len(tracks))
	for i, t := range tracks {
		responses[i] = ToMusicTrackResponse(t)
	}

	return ListTracksResponse{
		Tracks: responses,
		Total:  len(responses),
	}
}

// ListIDsResponse is now defined in application/common package to eliminate duplication
// Kept as type alias for backwards compatibility
type ListIDsResponse = appcommon.ListIDsResponse
