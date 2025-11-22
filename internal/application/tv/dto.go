package tv

import (
	"time"

	appcommon "github.com/mantonx/viewra/internal/application/common"
	"github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
)

// TVShowSummary represents a TV show with aggregated metadata (for list view)
type TVShowSummary struct {
	ID           int64  `json:"id" binding:"required"`
	LibraryID    int64  `json:"library_id" binding:"required"`
	Title        string `json:"title" binding:"required"`
	SeasonCount  int    `json:"season_count"`
	EpisodeCount int    `json:"episode_count"`
}

// TVShowDetailResponse represents detailed information about a single TV show
type TVShowDetailResponse struct {
	ID           int64  `json:"id"`
	LibraryID    int64  `json:"library_id"`
	Title        string `json:"title"`
	SeasonCount  int    `json:"season_count"`
	EpisodeCount int    `json:"episode_count"`
}

// TVEpisodeResponse represents a TV episode with all its metadata
type TVEpisodeResponse struct {
	// Base Media fields
	ID        int64     `json:"id"`
	LibraryID int64     `json:"library_id"`
	Title     string    `json:"title"` // filename
	FilePath  string    `json:"file_path"`
	FileSize  int64     `json:"file_size"`
	Duration  int       `json:"duration"` // in seconds
	IsExtra   bool      `json:"is_extra"`

	// Technical metadata
	Width           int     `json:"width,omitempty"`
	Height          int     `json:"height,omitempty"`
	VideoCodec      string  `json:"video_codec,omitempty"`
	AudioCodec      string  `json:"audio_codec,omitempty"`
	Bitrate         int64   `json:"bitrate,omitempty"`
	FrameRate       float64 `json:"frame_rate,omitempty"`
	ContainerFormat string  `json:"container_format,omitempty"`

	// TV Episode-specific fields
	ShowTitle    string `json:"show_title"`
	SeasonID     int64  `json:"season_id"`
	Season       int    `json:"season"`
	Episode      int    `json:"episode"`
	EpisodeTitle string `json:"episode_title,omitempty"` // actual episode name
	TVDbID       int    `json:"tvdb_id,omitempty"`
	IMDbID       string `json:"imdb_id,omitempty"`
	AirDate      string `json:"air_date,omitempty"`
	Description  string `json:"description,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListTVShowsResponse represents a list of TV shows with summary data
type ListTVShowsResponse struct {
	Shows      []TVShowSummary            `json:"shows"`
	Total      int                        `json:"total"`
	Pagination *common.PaginationMetadata `json:"pagination,omitempty"`
}

// ListTVEpisodesResponse represents a list of TV episodes
type ListTVEpisodesResponse struct {
	Episodes   []TVEpisodeResponse        `json:"episodes"`
	Total      int                        `json:"total"`
	Pagination *common.PaginationMetadata `json:"pagination,omitempty"`
}

// ToTVEpisodeResponse converts a domain TV episode entity to a DTO
func ToTVEpisodeResponse(ep *media.TVEpisode) TVEpisodeResponse {
	return TVEpisodeResponse{
		ID:              ep.ID,
		LibraryID:       ep.LibraryID,
		Title:           ep.Title,
		FilePath:        ep.FilePath,
		FileSize:        ep.FileSize,
		Duration:        ep.Duration,
		IsExtra:         ep.IsExtra,
		Width:           ep.Width,
		Height:          ep.Height,
		VideoCodec:      ep.VideoCodec,
		AudioCodec:      ep.AudioCodec,
		Bitrate:         ep.Bitrate,
		FrameRate:       ep.FrameRate,
		ContainerFormat: ep.ContainerFormat,
		ShowTitle:       ep.ShowTitle,
		SeasonID:        ep.SeasonID,
		Season:          ep.Season,
		Episode:         ep.Episode,
		EpisodeTitle:    ep.EpisodeTitle,
		TVDbID:          ep.TVDbID,
		IMDbID:          ep.IMDbID,
		AirDate:         ep.AirDate,
		Description:     ep.Description,
		CreatedAt:       ep.CreatedAt,
		UpdatedAt:       ep.UpdatedAt,
	}
}

// ToListTVEpisodesResponse converts a list of domain TV episodes to list response DTO
func ToListTVEpisodesResponse(episodes []*media.TVEpisode) ListTVEpisodesResponse {
	responses := make([]TVEpisodeResponse, len(episodes))
	for i, ep := range episodes {
		responses[i] = ToTVEpisodeResponse(ep)
	}

	return ListTVEpisodesResponse{
		Episodes: responses,
		Total:    len(responses),
	}
}

// ListIDsResponse is now defined in application/common package to eliminate duplication
// Kept as type alias for backwards compatibility
type ListIDsResponse = appcommon.ListIDsResponse
