package media

import (
	"io"
	"log/slog"
	"testing"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/application/library/scan/scanutil"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// testLogger returns a logger that discards all output (for tests).
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testDeps creates a Deps instance with the provided repositories and mock extractors.
// This is the standard way to set up dependencies for media processing tests.
func testDeps(t *testing.T, mediaRepos *scan.MediaRepositories, scanRepos *scan.ScanRepositories) *Deps {
	return &Deps{
		MediaRepos:       mediaRepos,
		ScanRepos:        scanRepos,
		ImageRepo:        mocks.NewImageRepository(t),
		MovieExtractor:   mocks.NewMovieImageExtractor(t),
		EpisodeExtractor: mocks.NewEpisodeImageExtractor(t),
		ShowExtractor:    mocks.NewShowImageExtractor(t),
		SeasonExtractor:  mocks.NewSeasonImageExtractor(t),
		AlbumExtractor:   mocks.NewAlbumImageExtractor(t),
		ArtistExtractor:  mocks.NewArtistImageExtractor(t),
		TrackExtractor:   mocks.NewTrackImageExtractor(t),
		ProcessedArtists: &scanutil.AtomicDeduplicator{},
		ProcessedShows:   &scanutil.AtomicDeduplicator{},
		Logger:           testLogger(),
	}
}

// testMediaRepos creates MediaRepositories with the provided mocks.
func testMediaRepos(
	t *testing.T,
	mediaRepo *mocks.MediaRepository,
	movieRepo *mocks.MovieRepository,
	tvRepo *mocks.TVRepository,
	musicRepo *mocks.MusicRepository,
) *scan.MediaRepositories {
	if mediaRepo == nil {
		mediaRepo = mocks.NewMediaRepository(t)
	}
	if movieRepo == nil {
		movieRepo = mocks.NewMovieRepository(t)
	}
	if tvRepo == nil {
		tvRepo = mocks.NewTVRepository(t)
	}
	if musicRepo == nil {
		musicRepo = mocks.NewMusicRepository(t)
	}
	return &scan.MediaRepositories{
		Library: mocks.NewLibraryRepository(t),
		Media:   mediaRepo,
		Movie:   movieRepo,
		TV:      tvRepo,
		Music:   musicRepo,
	}
}

// testScanRepos creates ScanRepositories with basic mocks.
func testScanRepos(t *testing.T) *scan.ScanRepositories {
	return &scan.ScanRepositories{
		ScanJob:    mocks.NewScanJobRepository(t),
		Checkpoint: mocks.NewCheckpointRepository(t),
		ScanState:  mocks.NewScanStateRepository(t),
	}
}
