package noop

import (
	"context"
	"errors"

	"github.com/viewra/viewra/internal/domain/media"
)

// ErrNotImplemented is returned when Phase 3 features are called
var ErrNotImplemented = errors.New("feature not yet implemented (Phase 3)")

// NoOpMovieRepository is a no-op implementation of MovieRepository for Phase 3.
// Write operations (Create/Update) succeed silently to allow scanning to continue.
// Read operations return empty results or ErrNotImplemented.
//
// Design Decision: Silent success on writes allows the scanner to continue working
// with base media records. Type-specific metadata will be stored once Phase 3 is
// implemented. Error messages are logged by the scanner's fmt.Printf calls.
type NoOpMovieRepository struct{}

// NewMovieRepository creates a new no-op movie repository
func NewMovieRepository() *NoOpMovieRepository {
	return &NoOpMovieRepository{}
}

// CreateMovie silently succeeds as base media record is created by mediaRepo
func (r *NoOpMovieRepository) CreateMovie(_ context.Context, _ *media.Movie) error {
	// Phase 3: Silently succeed - base media record is still created by mediaRepo
	return nil
}

// GetMovieByID returns ErrNotImplemented for Phase 3
func (r *NoOpMovieRepository) GetMovieByID(_ context.Context, _ int64) (*media.Movie, error) {
	return nil, ErrNotImplemented
}

// ListMoviesByLibrary returns empty slice for Phase 3
func (r *NoOpMovieRepository) ListMoviesByLibrary(_ context.Context, _ int64) ([]*media.Movie, error) {
	return []*media.Movie{}, nil
}

// UpdateMovie silently succeeds for Phase 3
func (r *NoOpMovieRepository) UpdateMovie(_ context.Context, _ *media.Movie) error {
	return nil
}

// SearchMovies returns empty slice for Phase 3
func (r *NoOpMovieRepository) SearchMovies(_ context.Context, _ int64, _ string) ([]*media.Movie, error) {
	return []*media.Movie{}, nil
}

// NoOpTVRepository is a no-op implementation of TVRepository for Phase 3.
// See NoOpMovieRepository for design rationale.
type NoOpTVRepository struct{}

// NewTVRepository creates a new no-op TV repository
func NewTVRepository() *NoOpTVRepository {
	return &NoOpTVRepository{}
}

// CreateTVEpisode silently succeeds as base media record is created by mediaRepo
func (r *NoOpTVRepository) CreateTVEpisode(_ context.Context, _ *media.TVEpisode) error {
	// Phase 3: Silently succeed - base media record is still created by mediaRepo
	return nil
}

// GetTVEpisodeByID returns ErrNotImplemented for Phase 3
func (r *NoOpTVRepository) GetTVEpisodeByID(_ context.Context, _ int64) (*media.TVEpisode, error) {
	return nil, ErrNotImplemented
}

// ListTVEpisodesByLibrary returns empty slice for Phase 3
func (r *NoOpTVRepository) ListTVEpisodesByLibrary(_ context.Context, _ int64) ([]*media.TVEpisode, error) {
	return []*media.TVEpisode{}, nil
}

// ListTVEpisodesByShow returns empty slice for Phase 3
func (r *NoOpTVRepository) ListTVEpisodesByShow(_ context.Context, _ int64, _ string) ([]*media.TVEpisode, error) {
	return []*media.TVEpisode{}, nil
}

// UpdateTVEpisode silently succeeds for Phase 3
func (r *NoOpTVRepository) UpdateTVEpisode(_ context.Context, _ *media.TVEpisode) error {
	return nil
}

// SearchTVEpisodes returns empty slice for Phase 3
func (r *NoOpTVRepository) SearchTVEpisodes(_ context.Context, _ int64, _ string) ([]*media.TVEpisode, error) {
	return []*media.TVEpisode{}, nil
}

// NoOpMusicRepository is a no-op implementation of MusicRepository for Phase 3.
// See NoOpMovieRepository for design rationale.
type NoOpMusicRepository struct{}

// NewMusicRepository creates a new no-op music repository
func NewMusicRepository() *NoOpMusicRepository {
	return &NoOpMusicRepository{}
}

// CreateMusicTrack silently succeeds as base media record is created by mediaRepo
func (r *NoOpMusicRepository) CreateMusicTrack(_ context.Context, _ *media.MusicTrack) error {
	// Phase 3: Silently succeed - base media record is still created by mediaRepo
	return nil
}

// GetMusicTrackByID returns ErrNotImplemented for Phase 3
func (r *NoOpMusicRepository) GetMusicTrackByID(_ context.Context, _ int64) (*media.MusicTrack, error) {
	return nil, ErrNotImplemented
}

// ListMusicTracksByLibrary returns empty slice for Phase 3
func (r *NoOpMusicRepository) ListMusicTracksByLibrary(_ context.Context, _ int64) ([]*media.MusicTrack, error) {
	return []*media.MusicTrack{}, nil
}

// ListMusicTracksByAlbum returns empty slice for Phase 3
func (r *NoOpMusicRepository) ListMusicTracksByAlbum(
	_ context.Context,
	_ int64,
	_ string,
) ([]*media.MusicTrack, error) {
	return []*media.MusicTrack{}, nil
}

// ListMusicTracksByArtist returns empty slice for Phase 3
func (r *NoOpMusicRepository) ListMusicTracksByArtist(
	_ context.Context,
	_ int64,
	_ string,
) ([]*media.MusicTrack, error) {
	return []*media.MusicTrack{}, nil
}

// UpdateMusicTrack silently succeeds for Phase 3
func (r *NoOpMusicRepository) UpdateMusicTrack(_ context.Context, _ *media.MusicTrack) error {
	return nil
}

// SearchMusicTracks returns empty slice for Phase 3
func (r *NoOpMusicRepository) SearchMusicTracks(_ context.Context, _ int64, _ string) ([]*media.MusicTrack, error) {
	return []*media.MusicTrack{}, nil
}
