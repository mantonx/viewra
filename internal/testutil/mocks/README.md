# Centralized Mock Repository

This package provides centralized mock implementations for all repository interfaces and dependencies used in ViewRA tests.

## Purpose

**Problem:** Previously, mock repositories were duplicated across test files, leading to:
- 1,500+ lines of duplicate code
- Inconsistent mock implementations
- Missing method implementations causing compilation failures
- Difficult maintenance when interfaces change

**Solution:** This package provides a single source of truth for all mocks with:
- Complete interface implementations
- Consistent API patterns
- Built-in error injection
- Test helpers and verification methods

## Usage Patterns

### Basic Usage

```go
func TestMyUseCase(t *testing.T) {
    // Create a mock repository
    mediaRepo := mocks.NewMediaRepository(t)

    // Use it in your test
    useCase := application.NewMyUseCase(mediaRepo)
    result, err := useCase.Execute(ctx, input)

    // Make assertions
    require.NoError(t, err)
    assert.NotNil(t, result)
}
```

### Pre-populating Data

Use builder methods to set up test data:

```go
func TestListMedia(t *testing.T) {
    // Create test data
    media1 := &media.Media{ID: 1, Path: "/test/video1.mp4"}
    media2 := &media.Media{ID: 2, Path: "/test/video2.mp4"}

    // Set up mock with data
    mediaRepo := mocks.NewMediaRepository(t).
        WithMedia(media1, media2)

    // Execute test
    result, err := useCase.ListMedia(ctx, libraryID)

    // Verify
    assert.Len(t, result, 2)
}
```

### Error Injection

Inject errors to test error handling:

```go
func TestCreateMediaError(t *testing.T) {
    expectedErr := errors.New("database error")

    mediaRepo := mocks.NewMediaRepository(t).
        WithCreateError(expectedErr)

    err := useCase.CreateMedia(ctx, media)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "database error")
}
```

### Verification Helpers

Use assertion methods to verify mock state:

```go
func TestDeleteMedia(t *testing.T) {
    media := &media.Media{ID: 1, Path: "/test/video.mp4"}

    mediaRepo := mocks.NewMediaRepository(t).
        WithMedia(media)

    err := useCase.DeleteMedia(ctx, 1)

    require.NoError(t, err)
    mediaRepo.AssertMediaCount(0) // Verify media was deleted
}
```

## Available Mocks

### Repository Mocks

- **MediaRepository** - Base media repository
  - `NewMediaRepository(t)` - Create mock
  - `WithMedia(...media)` - Pre-populate media
  - `WithCreateError(err)` - Inject create error
  - `AssertMediaCount(n)` - Verify count

- **MovieRepository** - Movie-specific repository
  - `NewMovieRepository(t)` - Create mock
  - `WithMovies(...movies)` - Pre-populate movies
  - `MovieExists(id)` - Check if movie exists

- **TVRepository** - TV show/season/episode repository
  - `NewTVRepository(t)` - Create mock
  - `WithShows(...shows)` - Pre-populate shows
  - `WithSeasons(...seasons)` - Pre-populate seasons
  - `WithEpisodes(...episodes)` - Pre-populate episodes

- **MusicRepository** - Music track/album/artist repository
  - `NewMusicRepository(t)` - Create mock
  - `WithTracks(...tracks)` - Pre-populate tracks

- **LibraryRepository** - Library management repository
  - `NewLibraryRepository(t)` - Create mock
  - `WithLibraries(...libraries)` - Pre-populate libraries

- **ProgressRepository** - Watch progress tracking
  - `NewProgressRepository(t)` - Create mock
  - `WithProgress(...progress)` - Pre-populate progress

- **ScanJobRepository** - Library scan job tracking
  - `NewScanJobRepository(t)` - Create mock

- **ImageRepository** - Media image/artwork repository
  - `NewImageRepository(t)` - Create mock

- **TranscodeRepository** - Transcoding job repository
  - `NewTranscodeRepository(t)` - Create mock

### Service Mocks

- **Scheduler** - Job scheduler
  - `NewScheduler(t)` - Create mock

- **TranscodeQueue** - Transcoding queue
  - `NewTranscodeQueue(t)` - Create mock

- **SessionManager** - Transcoding session manager
  - `NewSessionManager(t)` - Create mock

- **CleanupService** - Cleanup service
  - `NewCleanupService(t)` - Create mock

## Common Patterns

### Testing CRUD Operations

```go
func TestCRUD(t *testing.T) {
    repo := mocks.NewMediaRepository(t)

    // Create
    media := &media.Media{ID: 1, Path: "/test.mp4"}
    err := repo.Create(ctx, media)
    require.NoError(t, err)

    // Read
    found, err := repo.GetByID(ctx, 1)
    require.NoError(t, err)
    assert.Equal(t, media.Path, found.Path)

    // Update
    media.Path = "/updated.mp4"
    err = repo.Update(ctx, media)
    require.NoError(t, err)

    // Delete
    err = repo.Delete(ctx, 1)
    require.NoError(t, err)

    repo.AssertMediaCount(0)
}
```

### Testing Pagination

```go
func TestPagination(t *testing.T) {
    // Create 10 items
    var items []*media.Media
    for i := 1; i <= 10; i++ {
        items = append(items, &media.Media{
            ID:   int64(i),
            Path: fmt.Sprintf("/test%d.mp4", i),
        })
    }

    repo := mocks.NewMediaRepository(t).
        WithMedia(items...)

    // Page 1
    page1, err := repo.ListByLibrary(ctx, libraryID, 5, 0)
    require.NoError(t, err)
    assert.Len(t, page1, 5)

    // Page 2
    page2, err := repo.ListByLibrary(ctx, libraryID, 5, 5)
    require.NoError(t, err)
    assert.Len(t, page2, 5)
}
```

### Testing Error Scenarios

```go
func TestErrorHandling(t *testing.T) {
    tests := []struct {
        name        string
        setupMock   func() *mocks.MediaRepository
        expectedErr string
    }{
        {
            name: "database connection error",
            setupMock: func() *mocks.MediaRepository {
                return mocks.NewMediaRepository(t).
                    WithGetError(errors.New("connection failed"))
            },
            expectedErr: "connection failed",
        },
        {
            name: "not found error",
            setupMock: func() *mocks.MediaRepository {
                return mocks.NewMediaRepository(t).
                    WithGetError(sql.ErrNoRows)
            },
            expectedErr: "no rows",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := tt.setupMock()
            _, err := repo.GetByID(ctx, 1)
            assert.Error(t, err)
            assert.Contains(t, err.Error(), tt.expectedErr)
        })
    }
}
```

## Adding Error Injection

All mocks support error injection for testing error paths. Methods are named after the repository method:

```go
type MediaRepository struct {
    // Error injection fields
    CreateErr      error
    GetErr         error
    UpdateErr      error
    DeleteErr      error
    ListErr        error
    ExistsByPathErr error
}

// Builder methods
func (m *MediaRepository) WithCreateError(err error) *MediaRepository
func (m *MediaRepository) WithGetError(err error) *MediaRepository
// ... etc
```

## Best Practices

1. **Use builder pattern** - Chain methods for readable test setup
2. **Inject errors explicitly** - Don't rely on default behavior
3. **Use verification helpers** - Call `Assert*` methods to verify state
4. **Keep mocks simple** - Don't add business logic to mocks
5. **Test one thing** - Each test should verify a single behavior

## Migration Guide

### Migrating from Local Mocks

**Before:**
```go
type mockMediaRepository struct {
    media map[int64]*media.Media
}

func newMockMediaRepository() *mockMediaRepository {
    return &mockMediaRepository{media: make(map[int64]*media.Media)}
}
```

**After:**
```go
import "github.com/mantonx/viewra/internal/testutil/mocks"

func TestSomething(t *testing.T) {
    repo := mocks.NewMediaRepository(t)
    // Use repo in test
}
```

### Updating Imports

Remove local mock definitions and update imports:

```go
import (
    "testing"
    "github.com/mantonx/viewra/internal/testutil/mocks"
    // Remove any local mock struct definitions
)
```

## Troubleshooting

### Mock doesn't implement interface

**Problem:** `cannot use repo (type *mocks.MediaRepository) as type media.Repository`

**Solution:** Ensure the mock implements all methods of the interface. Check that the interface hasn't added new methods.

### Test data not found

**Problem:** Mock returns empty results even after `WithMedia()`

**Solution:** Verify you're using the same mock instance. Don't create multiple instances:

```go
// WRONG
repo := mocks.NewMediaRepository(t)
repo.WithMedia(media)
useCase := NewUseCase(mocks.NewMediaRepository(t)) // New instance!

// CORRECT
repo := mocks.NewMediaRepository(t).WithMedia(media)
useCase := NewUseCase(repo)
```

### Error not being returned

**Problem:** Expected error not returned in test

**Solution:** Ensure error injection is set before calling the method:

```go
repo := mocks.NewMediaRepository(t).
    WithGetError(expectedErr) // Set error first

result, err := repo.GetByID(ctx, 1) // Then call method
```

## Contributing

When adding new mocks:

1. Implement ALL interface methods
2. Add builder methods for error injection
3. Add helper methods for common operations
4. Follow existing naming conventions
5. Update this README with usage examples
