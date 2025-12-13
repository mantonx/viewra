package library

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/application/library/scan/scanutil"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/system"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// =============================================================================
// Primitive Pointer Helpers
// =============================================================================

// intPtr creates a pointer to an int value.
func intPtr(i int) *int {
	return &i
}

// =============================================================================
// Logger Helper
// =============================================================================

// testLogger returns a logger that discards all output (for tests).
// Use this instead of creating slog.New(slog.NewTextHandler(io.Discard, nil)) inline.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// =============================================================================
// Test Fixture Builders
// =============================================================================

// newTestLibrary creates a library with sensible defaults for testing.
// Use functional options to customize specific fields.
func newTestLibrary(opts ...func(*library.Library)) *library.Library {
	lib := &library.Library{
		ID:        1,
		Name:      "Test Library",
		Path:      "/test/library",
		Type:      library.LibraryTypeMovies,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	for _, opt := range opts {
		opt(lib)
	}
	return lib
}

// Library option functions

func withLibraryID(id int64) func(*library.Library) {
	return func(l *library.Library) { l.ID = id }
}

func withLibraryPath(path string) func(*library.Library) {
	return func(l *library.Library) { l.Path = path }
}

func withLibraryType(t library.LibraryType) func(*library.Library) {
	return func(l *library.Library) { l.Type = t }
}

func withLibraryName(name string) func(*library.Library) {
	return func(l *library.Library) { l.Name = name }
}

// newTestScanJob creates a scan job with sensible defaults for testing.
func newTestScanJob(libraryID int64, opts ...func(*scanner.ScanJob)) *scanner.ScanJob {
	job := &scanner.ScanJob{
		ID:             1,
		LibraryID:      libraryID,
		Status:         scanner.ScanStatusRunning,
		Phase:          scanner.ScanPhaseProcessing,
		FilesFound:     0,
		FilesProcessed: 0,
		ErrorCount:     0,
		WarningCount:   0,
		DiscoveryDone:  false,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	for _, opt := range opts {
		opt(job)
	}
	return job
}

// ScanJob option functions

func withJobID(id int64) func(*scanner.ScanJob) {
	return func(j *scanner.ScanJob) { j.ID = id }
}

func withJobStatus(status scanner.ScanStatus) func(*scanner.ScanJob) {
	return func(j *scanner.ScanJob) { j.Status = status }
}

func withJobPhase(phase scanner.ScanPhase) func(*scanner.ScanJob) {
	return func(j *scanner.ScanJob) { j.Phase = phase }
}

func withFilesFound(count int64) func(*scanner.ScanJob) {
	return func(j *scanner.ScanJob) { j.FilesFound = count }
}

func withFilesProcessed(count int64) func(*scanner.ScanJob) {
	return func(j *scanner.ScanJob) { j.FilesProcessed = count }
}

func withDiscoveryDone() func(*scanner.ScanJob) {
	return func(j *scanner.ScanJob) { j.DiscoveryDone = true }
}

func withEstimatedTotal(count int64) func(*scanner.ScanJob) {
	return func(j *scanner.ScanJob) { j.EstimatedTotal = count }
}

// newTestCheckpoint creates a checkpoint with sensible defaults for testing.
func newTestCheckpoint(jobID int64, filePath string, opts ...func(*scanner.ScanCheckpoint)) *scanner.ScanCheckpoint {
	cp := &scanner.ScanCheckpoint{
		ID:        0, // Will be assigned by repository
		ScanJobID: jobID,
		FilePath:  filePath,
		Status:    scanner.CheckpointPending,
		CreatedAt: time.Now(),
	}
	for _, opt := range opts {
		opt(cp)
	}
	return cp
}

// Checkpoint option functions

func withCheckpointID(id int64) func(*scanner.ScanCheckpoint) {
	return func(cp *scanner.ScanCheckpoint) { cp.ID = id }
}

func withCheckpointStatus(status scanner.CheckpointStatus) func(*scanner.ScanCheckpoint) {
	return func(cp *scanner.ScanCheckpoint) { cp.Status = status }
}

func withCheckpointHash(hash string) func(*scanner.ScanCheckpoint) {
	return func(cp *scanner.ScanCheckpoint) { cp.FileHash = hash }
}

func withCheckpointError(msg string, category scanner.ErrorCategory) func(*scanner.ScanCheckpoint) {
	return func(cp *scanner.ScanCheckpoint) {
		cp.ErrorMessage = msg
		cp.ErrorCategory = category
	}
}

// newTestMedia creates a media entity with sensible defaults for testing.
func newTestMedia(libraryID int64, filePath string, opts ...func(*media.Media)) *media.Media {
	m := &media.Media{
		ID:        0, // Will be assigned by repository
		LibraryID: libraryID,
		FilePath:  filePath,
		Title:     "Test Media",
		Duration:  3600, // 1 hour in seconds
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Media option functions

func withMediaID(id int64) func(*media.Media) {
	return func(m *media.Media) { m.ID = id }
}

func withMediaTitle(title string) func(*media.Media) {
	return func(m *media.Media) { m.Title = title }
}

func withMediaDuration(duration int) func(*media.Media) {
	return func(m *media.Media) { m.Duration = duration }
}

// =============================================================================
// Mock Repository Collection
// =============================================================================

// testRepos holds all mock repositories used in tests.
// Access individual repos to configure expectations or verify calls.
type testRepos struct {
	Library    *mocks.LibraryRepository
	Media      *mocks.MediaRepository
	Movie      *mocks.MovieRepository
	TV         *mocks.TVRepository    // Episodes, Shows, Seasons
	Music      *mocks.MusicRepository // Tracks, Albums, Artists
	ScanJob    *mocks.ScanJobRepository
	Checkpoint *mocks.CheckpointRepository
	ScanState  *mocks.ScanStateRepository
}

// newTestRepos creates a fresh set of mock repositories for a test.
func newTestRepos(t *testing.T) *testRepos {
	return &testRepos{
		Library:    mocks.NewLibraryRepository(t),
		Media:      mocks.NewMediaRepository(t),
		Movie:      mocks.NewMovieRepository(t),
		TV:         mocks.NewTVRepository(t),
		Music:      mocks.NewMusicRepository(t),
		ScanJob:    mocks.NewScanJobRepository(t),
		Checkpoint: mocks.NewCheckpointRepository(t),
		ScanState:  mocks.NewScanStateRepository(t),
	}
}

// toMediaRepositories converts testRepos to MediaRepositories for use case construction.
func (r *testRepos) toMediaRepositories() *scan.MediaRepositories {
	return &scan.MediaRepositories{
		Library: r.Library,
		Media:   r.Media,
		Movie:   r.Movie,
		TV:      r.TV,
		Music:   r.Music,
	}
}

// toScanRepositories converts testRepos to ScanRepositories for use case construction.
func (r *testRepos) toScanRepositories() *scan.ScanRepositories {
	return &scan.ScanRepositories{
		ScanJob:    r.ScanJob,
		Checkpoint: r.Checkpoint,
		ScanState:  r.ScanState,
	}
}

// =============================================================================
// ScanLibraryUseCase Test Builder
// =============================================================================

// testUseCaseBuilder is a fluent builder for creating ScanLibraryUseCase instances in tests.
type testUseCaseBuilder struct {
	t             *testing.T
	repos         *testRepos
	config        scan.Config
	systemProfile *system.Profile
	libraries     []*library.Library
	scanJobs      []*scanner.ScanJob
	checkpoints   []*scanner.ScanCheckpoint
	mediaItems    []*media.Media
	movies        []*media.Movie
	scanStates    []*scanner.ScanState
}

// newTestUseCaseBuilder creates a new builder for ScanLibraryUseCase.
func newTestUseCaseBuilder(t *testing.T) *testUseCaseBuilder {
	return &testUseCaseBuilder{
		t:      t,
		repos:  newTestRepos(t),
		config: scan.DefaultConfig(),
	}
}

// WithConfig sets a custom scan configuration.
func (b *testUseCaseBuilder) WithConfig(config scan.Config) *testUseCaseBuilder {
	b.config = config
	return b
}

// WithSystemProfile sets the system profile for worker count calculation.
func (b *testUseCaseBuilder) WithSystemProfile(profile *system.Profile) *testUseCaseBuilder {
	b.systemProfile = profile
	return b
}

// WithLibrary adds a library to the mock repository.
func (b *testUseCaseBuilder) WithLibrary(libs ...*library.Library) *testUseCaseBuilder {
	b.libraries = append(b.libraries, libs...)
	return b
}

// WithScanJob adds scan jobs to the mock repository.
func (b *testUseCaseBuilder) WithScanJob(jobs ...*scanner.ScanJob) *testUseCaseBuilder {
	b.scanJobs = append(b.scanJobs, jobs...)
	return b
}

// WithCheckpoints adds checkpoints to the mock repository.
func (b *testUseCaseBuilder) WithCheckpoints(cps ...*scanner.ScanCheckpoint) *testUseCaseBuilder {
	b.checkpoints = append(b.checkpoints, cps...)
	return b
}

// WithMedia adds media entities to the mock repository.
func (b *testUseCaseBuilder) WithMedia(items ...*media.Media) *testUseCaseBuilder {
	b.mediaItems = append(b.mediaItems, items...)
	return b
}

// WithMovies adds movies to the mock repository.
func (b *testUseCaseBuilder) WithMovies(movies ...*media.Movie) *testUseCaseBuilder {
	b.movies = append(b.movies, movies...)
	return b
}

// WithScanStates adds scan states to the mock repository.
func (b *testUseCaseBuilder) WithScanStates(states ...*scanner.ScanState) *testUseCaseBuilder {
	b.scanStates = append(b.scanStates, states...)
	return b
}

// Build constructs the ScanLibraryUseCase with all configured mocks.
// Returns both the use case and the repos for additional assertions.
func (b *testUseCaseBuilder) Build() (*ScanLibraryUseCase, *testRepos) {
	// Configure repositories with provided data
	if len(b.libraries) > 0 {
		b.repos.Library.WithLibraries(b.libraries...)
	}
	if len(b.scanJobs) > 0 {
		b.repos.ScanJob.WithJobs(b.scanJobs...)
	}
	if len(b.checkpoints) > 0 {
		b.repos.Checkpoint.WithCheckpoints(b.checkpoints...)
	}
	if len(b.mediaItems) > 0 {
		b.repos.Media.WithMedia(b.mediaItems...)
	}
	if len(b.movies) > 0 {
		b.repos.Movie.WithMovies(b.movies...)
	}
	if len(b.scanStates) > 0 {
		b.repos.ScanState.WithStates(b.scanStates...)
	}

	// Create the use case
	uc := &ScanLibraryUseCase{
		mediaRepos:    b.repos.toMediaRepositories(),
		scanRepos:     b.repos.toScanRepositories(),
		config:        b.config,
		systemProfile: b.systemProfile,
		logger:        testLogger(),
		// Initialize sync.Map fields
		processedArtists: scanutil.AtomicDeduplicator{},
		processedShows:   scanutil.AtomicDeduplicator{},
	}

	return uc, b.repos
}

// =============================================================================
// Convenience Functions
// =============================================================================

// newTestScanUseCase is a convenience function for simple test setups.
// For more complex setups, use newTestUseCaseBuilder() directly.
func newTestScanUseCase(t *testing.T) (*ScanLibraryUseCase, *testRepos) {
	return newTestUseCaseBuilder(t).Build()
}

// newTestScanUseCaseWithLibrary creates a use case with a single library pre-configured.
func newTestScanUseCaseWithLibrary(t *testing.T, lib *library.Library) (*ScanLibraryUseCase, *testRepos) {
	return newTestUseCaseBuilder(t).
		WithLibrary(lib).
		Build()
}

// newTestScanUseCaseWithJob creates a use case with a library and scan job pre-configured.
func newTestScanUseCaseWithJob(t *testing.T, lib *library.Library, job *scanner.ScanJob) (*ScanLibraryUseCase, *testRepos) {
	return newTestUseCaseBuilder(t).
		WithLibrary(lib).
		WithScanJob(job).
		Build()
}

// =============================================================================
// Batch Checkpoint Helpers
// =============================================================================

// newTestCheckpointBatch creates multiple checkpoints with incrementing IDs.
// Useful for testing batch processing logic.
func newTestCheckpointBatch(jobID int64, count int, status scanner.CheckpointStatus) []*scanner.ScanCheckpoint {
	cps := make([]*scanner.ScanCheckpoint, count)
	for i := 0; i < count; i++ {
		cps[i] = newTestCheckpoint(jobID, "/test/file"+string(rune('A'+i))+".mp4",
			withCheckpointID(int64(i+1)),
			withCheckpointStatus(status),
		)
	}
	return cps
}

// newTestCheckpointMixed creates a mixed batch of checkpoints with various statuses.
// Returns checkpoints in order: completed, failed, warning, pending.
func newTestCheckpointMixed(jobID int64, completed, failed, warning, pending int) []*scanner.ScanCheckpoint {
	var cps []*scanner.ScanCheckpoint
	id := int64(1)

	for i := 0; i < completed; i++ {
		cps = append(cps, newTestCheckpoint(jobID, "/test/completed"+string(rune('A'+i))+".mp4",
			withCheckpointID(id),
			withCheckpointStatus(scanner.CheckpointCompleted),
		))
		id++
	}
	for i := 0; i < failed; i++ {
		cps = append(cps, newTestCheckpoint(jobID, "/test/failed"+string(rune('A'+i))+".mp4",
			withCheckpointID(id),
			withCheckpointStatus(scanner.CheckpointFailed),
			withCheckpointError("test error", scanner.ErrorCategoryMetadata),
		))
		id++
	}
	for i := 0; i < warning; i++ {
		cps = append(cps, newTestCheckpoint(jobID, "/test/warning"+string(rune('A'+i))+".mp4",
			withCheckpointID(id),
			withCheckpointStatus(scanner.CheckpointWarning),
		))
		id++
	}
	for i := 0; i < pending; i++ {
		cps = append(cps, newTestCheckpoint(jobID, "/test/pending"+string(rune('A'+i))+".mp4",
			withCheckpointID(id),
			withCheckpointStatus(scanner.CheckpointPending),
		))
		id++
	}

	return cps
}
