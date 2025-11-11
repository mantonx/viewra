package media

import (
	"time"

	"github.com/viewra/viewra/internal/domain/media"
)

// MediaResponse represents the base media response for all media types
type MediaResponse struct {
	ID        int64     `json:"id"`
	LibraryID int64     `json:"library_id"`
	Title     string    `json:"title"`
	FilePath  string    `json:"file_path"`
	FileSize  int64     `json:"file_size"`
	Duration  int       `json:"duration"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MovieResponse represents a movie media item with all metadata
type MovieResponse struct {
	MediaResponse
	Year        int      `json:"year,omitempty"`
	IMDbID      string   `json:"imdb_id,omitempty"`
	TMDbID      int      `json:"tmdb_id,omitempty"`
	Director    string   `json:"director,omitempty"`
	Rating      float32  `json:"rating,omitempty"`
	Genres      []string `json:"genres,omitempty"`
	Description string   `json:"description,omitempty"`
}

// TVEpisodeResponse represents a TV episode media item with all metadata
type TVEpisodeResponse struct {
	MediaResponse
	ShowTitle    string `json:"show_title,omitempty"`
	Season       int    `json:"season,omitempty"`
	Episode      int    `json:"episode,omitempty"`
	EpisodeTitle string `json:"episode_title,omitempty"`
	TVDbID       int    `json:"tvdb_id,omitempty"`
	IMDbID       string `json:"imdb_id,omitempty"`
	AirDate      string `json:"air_date,omitempty"`
	Description  string `json:"description,omitempty"`
}

// MusicTrackResponse represents a music track media item with all metadata
type MusicTrackResponse struct {
	MediaResponse
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	TrackNumber int    `json:"track_number,omitempty"`
	Year        int    `json:"year,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Composer    string `json:"composer,omitempty"`
	Publisher   string `json:"publisher,omitempty"`
	Bitrate     int    `json:"bitrate,omitempty"`
}

// ListMediaResponse represents a paginated list of media items
type ListMediaResponse struct {
	Media []MediaResponse `json:"media"`
	Total int             `json:"total"`
}

// ListMoviesResponse represents a paginated list of movies
type ListMoviesResponse struct {
	Movies []MovieResponse `json:"movies"`
	Total  int             `json:"total"`
}

// ListTVEpisodesResponse represents a paginated list of TV episodes
type ListTVEpisodesResponse struct {
	Episodes []TVEpisodeResponse `json:"episodes"`
	Total    int                 `json:"total"`
}

// ListMusicTracksResponse represents a paginated list of music tracks
type ListMusicTracksResponse struct {
	Tracks []MusicTrackResponse `json:"tracks"`
	Total  int                  `json:"total"`
}

// SearchMediaRequest represents search parameters
type SearchMediaRequest struct {
	Query     string `json:"query" validate:"required,min=1"`
	LibraryID int64  `json:"library_id"`
}

// ToMediaResponse converts a domain media entity to a DTO
func ToMediaResponse(m *media.Media) MediaResponse {
	return MediaResponse{
		ID:        m.ID,
		LibraryID: m.LibraryID,
		Title:     m.Title,
		FilePath:  m.FilePath,
		FileSize:  m.FileSize,
		Duration:  m.Duration,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

// ToMovieResponse converts a domain movie entity to a DTO
func ToMovieResponse(m *media.Movie) MovieResponse {
	return MovieResponse{
		MediaResponse: ToMediaResponse(&m.Media),
		Year:          m.Year,
		IMDbID:        m.IMDbID,
		TMDbID:        m.TMDbID,
		Director:      m.Director,
		Rating:        m.Rating,
		Genres:        m.Genres,
		Description:   m.Description,
	}
}

// ToTVEpisodeResponse converts a domain TV episode entity to a DTO
func ToTVEpisodeResponse(tv *media.TVEpisode) TVEpisodeResponse {
	return TVEpisodeResponse{
		MediaResponse: ToMediaResponse(&tv.Media),
		ShowTitle:     tv.ShowTitle,
		Season:        tv.Season,
		Episode:       tv.Episode,
		EpisodeTitle:  tv.EpisodeTitle,
		TVDbID:        tv.TVDbID,
		IMDbID:        tv.IMDbID,
		AirDate:       tv.AirDate,
		Description:   tv.Description,
	}
}

// ToMusicTrackResponse converts a domain music track entity to a DTO
func ToMusicTrackResponse(m *media.MusicTrack) MusicTrackResponse {
	return MusicTrackResponse{
		MediaResponse: ToMediaResponse(&m.Media),
		Artist:        m.Artist,
		Album:         m.Album,
		TrackNumber:   m.TrackNumber,
		Year:          m.Year,
		Genre:         m.Genre,
		Composer:      m.Composer,
		Publisher:     m.Publisher,
		Bitrate:       m.Bitrate,
	}
}

// ToListMediaResponse converts a list of domain media to list response DTO
func ToListMediaResponse(mediaList []*media.Media) ListMediaResponse {
	responses := make([]MediaResponse, len(mediaList))
	for i, m := range mediaList {
		responses[i] = ToMediaResponse(m)
	}

	return ListMediaResponse{
		Media: responses,
		Total: len(responses),
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

// ToListMusicTracksResponse converts a list of domain music tracks to list response DTO
func ToListMusicTracksResponse(tracks []*media.MusicTrack) ListMusicTracksResponse {
	responses := make([]MusicTrackResponse, len(tracks))
	for i, t := range tracks {
		responses[i] = ToMusicTrackResponse(t)
	}

	return ListMusicTracksResponse{
		Tracks: responses,
		Total:  len(responses),
	}
}
