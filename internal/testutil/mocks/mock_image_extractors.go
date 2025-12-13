package mocks

import (
	"context"
	"testing"

	"github.com/mantonx/viewra/internal/domain/images"
)

// MovieImageExtractor is a mock implementation for testing
type MovieImageExtractor struct {
	t            testing.TB
	ExecuteErr   error
	ExecuteCalls int
}

// NewMovieImageExtractor creates a new mock movie image extractor
func NewMovieImageExtractor(t testing.TB) *MovieImageExtractor {
	return &MovieImageExtractor{t: t}
}

// Execute implements the movie image extraction interface
func (m *MovieImageExtractor) Execute(ctx context.Context, movieFilePath string, mediaType images.MediaType, entityID int, mediaID *int) error {
	m.ExecuteCalls++
	return m.ExecuteErr
}

// EpisodeImageExtractor is a mock implementation for testing
type EpisodeImageExtractor struct {
	t            testing.TB
	ExecuteErr   error
	ExecuteCalls int
}

// NewEpisodeImageExtractor creates a new mock episode image extractor
func NewEpisodeImageExtractor(t testing.TB) *EpisodeImageExtractor {
	return &EpisodeImageExtractor{t: t}
}

// Execute implements the episode image extraction interface
func (e *EpisodeImageExtractor) Execute(ctx context.Context, episodeFilePath string, mediaType images.MediaType, entityID int, mediaID *int) error {
	e.ExecuteCalls++
	return e.ExecuteErr
}

// ShowImageExtractor is a mock implementation for testing
type ShowImageExtractor struct {
	t            testing.TB
	ExecuteErr   error
	ExecuteCalls int
}

// NewShowImageExtractor creates a new mock show image extractor
func NewShowImageExtractor(t testing.TB) *ShowImageExtractor {
	return &ShowImageExtractor{t: t}
}

// Execute implements the show image extraction interface
func (s *ShowImageExtractor) Execute(ctx context.Context, showDir string, mediaType images.MediaType, entityID int) error {
	s.ExecuteCalls++
	return s.ExecuteErr
}

// SeasonImageExtractor is a mock implementation for testing
type SeasonImageExtractor struct {
	t            testing.TB
	ExecuteErr   error
	ExecuteCalls int
}

// NewSeasonImageExtractor creates a new mock season image extractor
func NewSeasonImageExtractor(t testing.TB) *SeasonImageExtractor {
	return &SeasonImageExtractor{t: t}
}

// Execute implements the season image extraction interface
func (s *SeasonImageExtractor) Execute(ctx context.Context, showDir string, seasonNumber int, mediaType images.MediaType, entityID int) error {
	s.ExecuteCalls++
	return s.ExecuteErr
}

// TrackImageExtractor is a mock implementation for testing
type TrackImageExtractor struct {
	t            testing.TB
	ExecuteErr   error
	ExecuteCalls int
}

// NewTrackImageExtractor creates a new mock track image extractor
func NewTrackImageExtractor(t testing.TB) *TrackImageExtractor {
	return &TrackImageExtractor{t: t}
}

// Execute implements the track image extraction interface
func (t *TrackImageExtractor) Execute(ctx context.Context, trackFilePath string, mediaType images.MediaType, entityID int, mediaID *int) error {
	t.ExecuteCalls++
	return t.ExecuteErr
}

// AlbumImageExtractor is a mock implementation for testing
type AlbumImageExtractor struct {
	t            testing.TB
	ExecuteErr   error
	ExecuteCalls int
}

// NewAlbumImageExtractor creates a new mock album image extractor
func NewAlbumImageExtractor(t testing.TB) *AlbumImageExtractor {
	return &AlbumImageExtractor{t: t}
}

// Execute implements the album image extraction interface
func (a *AlbumImageExtractor) Execute(ctx context.Context, albumDir string, mediaType images.MediaType, entityID int) error {
	a.ExecuteCalls++
	return a.ExecuteErr
}

// ArtistImageExtractor is a mock implementation for testing
type ArtistImageExtractor struct {
	t            testing.TB
	ExecuteErr   error
	ExecuteCalls int
}

// NewArtistImageExtractor creates a new mock artist image extractor
func NewArtistImageExtractor(t testing.TB) *ArtistImageExtractor {
	return &ArtistImageExtractor{t: t}
}

// Execute implements the artist image extraction interface
func (a *ArtistImageExtractor) Execute(ctx context.Context, artistDir string, mediaType images.MediaType, entityID int) error {
	a.ExecuteCalls++
	return a.ExecuteErr
}
