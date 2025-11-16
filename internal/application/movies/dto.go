package movies

import (
	"time"

	"github.com/viewra/viewra/internal/domain/media"
)

// MovieResponse represents a movie with all its metadata
type MovieResponse struct {
	// Base Media fields
	ID        int64     `json:"id"`
	LibraryID int64     `json:"library_id"`
	Title     string    `json:"title"`
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

	// Movie-specific fields
	Year             int       `json:"year,omitempty"`
	ReleaseDate      time.Time `json:"release_date,omitempty"`
	OriginalTitle    string    `json:"original_title,omitempty"`
	SortTitle        string    `json:"sort_title,omitempty"`
	RuntimeMinutes   int       `json:"runtime_minutes,omitempty"`
	IMDbID           string    `json:"imdb_id,omitempty"`
	TMDbID           int       `json:"tmdb_id,omitempty"`
	Director         string    `json:"director,omitempty"`
	Cast             []string  `json:"cast,omitempty"`
	Genre            []string  `json:"genre,omitempty"`
	Plot             string    `json:"plot,omitempty"`
	Tagline          string    `json:"tagline,omitempty"`
	ContentRating    string    `json:"content_rating,omitempty"`
	MaturityRating   int       `json:"maturity_rating,omitempty"`
	ContentAdvisories []string `json:"content_advisories,omitempty"`
	Budget           int64     `json:"budget,omitempty"`
	Revenue          int64     `json:"revenue,omitempty"`
	OriginalLanguage string    `json:"original_language,omitempty"`
	CountryOfOrigin  string    `json:"country_of_origin,omitempty"`
	AwardsSummary    string    `json:"awards_summary,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListMoviesResponse represents a list of movies
type ListMoviesResponse struct {
	Movies []MovieResponse `json:"movies"`
	Total  int             `json:"total"`
}

// ToMovieResponse converts a domain movie entity to a DTO
func ToMovieResponse(m *media.Movie) MovieResponse {
	return MovieResponse{
		ID:              m.ID,
		LibraryID:       m.LibraryID,
		Title:           m.Title,
		FilePath:        m.FilePath,
		FileSize:        m.FileSize,
		Duration:        m.Duration,
		IsExtra:         m.IsExtra,
		Width:           m.Width,
		Height:          m.Height,
		VideoCodec:      m.VideoCodec,
		AudioCodec:      m.AudioCodec,
		Bitrate:         m.Bitrate,
		FrameRate:       m.FrameRate,
		ContainerFormat: m.ContainerFormat,
		Year:            m.Year,
		ReleaseDate:     m.ReleaseDate,
		OriginalTitle:   m.OriginalTitle,
		SortTitle:       m.SortTitle,
		RuntimeMinutes:  m.RuntimeMinutes,
		IMDbID:          m.IMDbID,
		TMDbID:          m.TMDbID,
		Director:        m.Director,
		Cast:            m.Cast,
		Genre:           m.Genre,
		Plot:            m.Plot,
		Tagline:         m.Tagline,
		ContentRating:   m.ContentRating,
		MaturityRating:  m.MaturityRating,
		ContentAdvisories: m.ContentAdvisories,
		Budget:          m.Budget,
		Revenue:         m.Revenue,
		OriginalLanguage: m.OriginalLanguage,
		CountryOfOrigin: m.CountryOfOrigin,
		AwardsSummary:   m.AwardsSummary,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

// ToListMoviesResponse converts a list of domain movies to list response DTO
func ToListMoviesResponse(movies []*media.Movie) ListMoviesResponse {
	responses := make([]MovieResponse, len(movies))
	for i, m := range movies {
		responses[i] = ToMovieResponse(m)
	}

	return ListMoviesResponse{
		Movies: responses,
		Total:  len(responses),
	}
}
