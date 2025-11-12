# ADR 004: Scan Library Testing Strategy

## Status
Proposed

## Context
The `ScanLibraryUseCase` in `internal/application/library/scan_library.go` is a critical piece of functionality with 0% test coverage. This 323-line file handles:
- Initiating library scans with concurrent job management
- Background goroutine-based scanning with the filesystem coordinator
- Real-time progress tracking and database updates
- Processing scan results and creating/updating media entries (movies, TV, music)
- Scan job lifecycle management (running, completed, failed states)

Current coverage: **0%** (0/323 lines)
Target coverage: **70%+**

## Decision

We will implement a comprehensive, multi-layered testing strategy for the scan functionality:

### Phase 1: Unit Tests for Individual Functions (Target: 40% coverage)

#### 1.1 Test DTO Conversion Functions
**File**: `scan_dto.go` (currently 0% coverage)
- `ToStartScanResponse()` - Convert ScanJob to StartScanResponse
- `ToScanProgressResponse()` - Convert ScanJob to ScanProgressResponse
- `ToScanHistoryResponse()` - Convert []ScanJob to ScanHistoryResponse

**Test cases**:
- Valid scan job with all fields populated
- Scan job with nil CompletedAt (ongoing scan)
- Empty job list for history
- Multiple jobs with different statuses

**Complexity**: Low
**Estimated effort**: 1-2 hours
**Value**: High (simple, high-confidence tests)

#### 1.2 Test Query Methods (GetProgress, GetLatestScan, GetScanHistory)
**Lines**: 294-322

**Test cases**:
- GetProgress: Valid job ID, non-existent job ID
- GetLatestScan: Library with scans, library without scans
- GetScanHistory: Various limits (0, 1, 100), empty history

**Mocks needed**: `ScanJobRepository`

**Complexity**: Low
**Estimated effort**: 2-3 hours

#### 1.3 Test Media Processing Functions
**Lines**: 185-292
- `processMovie()` - Create/update movie entries
- `processTVEpisode()` - Create/update TV episode entries
- `processMusicTrack()` - Create/update music track entries

**Test cases per function**:
- Create new media entry (first scan)
- Update existing media entry (rescan)
- Handle repository errors gracefully
- Nil/optional fields (Year, Season, Episode, TrackNumber)

**Mocks needed**: `MediaRepository`, `MovieRepository`, `TVRepository`, `MusicRepository`

**Complexity**: Medium
**Estimated effort**: 4-6 hours
**Value**: High (core business logic)

### Phase 2: Integration Tests for StartScan (Target: 60% coverage)

#### 2.1 Test StartScan Job Creation
**Lines**: 44-84

**Test cases**:
- Successfully create scan job for valid library
- Reject scan when library doesn't exist
- Prevent duplicate scans (already running check)
- Verify job record is created with correct initial state

**Mocks needed**: `LibraryRepository`, `ScanJobRepository`

**Complexity**: Medium
**Estimated effort**: 3-4 hours
**Challenge**: Background goroutine - need to wait/sync for testing

**Approach**:
```go
// Option 1: Add injectable goroutine launcher for testing
type goroutineLauncher func(func())

// Option 2: Test synchronously by exposing runScan as testable
func TestStartScan_CreatesJob(t *testing.T) {
    // Mock repositories
    // Call StartScan
    // Verify job created with Running status
    // Verify goroutine launched (can't easily test)
}
```

### Phase 3: Complex Integration Tests (Target: 70% coverage)

#### 3.1 Test Result Processing Flow
**Lines**: 137-182 (`processResults()`)

**Test cases**:
- Process empty result channel (no files found)
- Process successful results for each media type
- Handle errors in result channel
- Verify progress updates at intervals (ticker-based)
- Verify final progress update

**Mocks needed**: All media repositories, `ScanJobRepository`

**Complexity**: High
**Estimated effort**: 6-8 hours
**Challenge**:
- Channel-based concurrency
- Time-based ticker logic
- Goroutine synchronization

**Approach**:
```go
// Make processResults testable by:
// 1. Accept ticker as parameter (inject controllable ticker in tests)
// 2. Make it synchronous (return when done processing)

func TestProcessResults_Movies(t *testing.T) {
    resultChan := make(chan scanner.ScanResult, 10)

    // Send test results
    resultChan <- scanner.ScanResult{
        FilePath: "test.mp4",
        Title: "Test Movie",
        Duration: 7200,
    }
    close(resultChan)

    // Call processResults synchronously
    // Verify media created via mock
    // Verify progress updated
}
```

#### 3.2 Test Full Scan Flow (End-to-End)
**Lines**: 87-134 (`runScan()`)

**Test cases**:
- Complete successful scan with mixed results
- Scan with filesystem coordinator errors
- Verify job completes with correct final status
- Verify progress reflects coordinator stats

**Mocks needed**: All repositories + filesystem coordinator mock

**Complexity**: Very High
**Estimated effort**: 10-12 hours
**Challenge**:
- Requires mocking `filesystem.Coordinator`
- Complex goroutine orchestration
- Multiple async channels

**Approach**:
```go
// Option 1: Mock filesystem.Coordinator (requires interface)
type ScannerCoordinator interface {
    Scan(ctx, path, resultChan) error
    GetProgress() Progress
}

// Option 2: Integration test with real temp directory
func TestRunScan_Integration(t *testing.T) {
    // Create temp directory with test files
    // Use real coordinator
    // Mock only database repositories
    // Verify end state
}
```

## Testing Infrastructure Needed

### 1. Mock Repositories
Create comprehensive mocks for:
- `library.Repository` ✓ (already exists in tests)
- `media.Repository` (need to create)
- `media.MovieRepository` (need to create)
- `media.TVRepository` (need to create)
- `media.MusicRepository` (need to create)
- `scanner.ScanJobRepository` (need to create)

### 2. Test Fixtures
Create reusable test data:
- Sample `ScanResult` objects for each media type
- Sample `ScanJob` objects in various states
- Sample media files in temp directories

### 3. Helper Functions
```go
// Wait for goroutine with timeout
func waitForJobComplete(t *testing.T, repo, jobID, timeout) *ScanJob

// Create test scan results
func createMovieResult(filePath, title string) ScanResult
func createTVResult(filePath, title string, season, episode int) ScanResult
func createMusicResult(filePath, title, artist, album string) ScanResult

// Verify media created
func assertMovieCreated(t *testing.T, repo MovieRepository, expected *Movie)
```

## Implementation Roadmap

### Week 1: Foundation (Target: 40% coverage)
- [ ] Day 1-2: Create all mock repositories
- [ ] Day 3: Test DTO conversion functions (scan_dto.go)
- [ ] Day 4-5: Test query methods (GetProgress, GetLatestScan, GetScanHistory)

### Week 2: Core Logic (Target: 60% coverage)
- [ ] Day 1-3: Test media processing functions (processMovie, processTV, processMusic)
- [ ] Day 4-5: Test StartScan job creation and validation

### Week 3: Integration (Target: 70% coverage)
- [ ] Day 1-3: Test processResults with channels and concurrency
- [ ] Day 4-5: Partial runScan tests or integration tests with real coordinator

## Constraints & Trade-offs

### What We CAN Test Easily
✅ DTO conversions
✅ Query methods
✅ Media processing logic
✅ StartScan validation and job creation
✅ Progress calculation

### What's HARD to Test
⚠️ Background goroutines (need careful synchronization)
⚠️ Real-time progress updates (ticker-based)
⚠️ Filesystem coordinator integration (requires interface/mock)
⚠️ Channel-based communication between goroutines

### What We SHOULD NOT Test
❌ Filesystem scanning itself (tested in filesystem package)
❌ Database operations (tested in persistence layer)
❌ Concurrent safety of atomic operations (trust stdlib)

## Success Criteria

1. **Coverage**: 70%+ of scan_library.go lines covered
2. **Reliability**: All tests pass consistently (no flaky tests)
3. **Maintainability**: Tests are readable and well-documented
4. **Performance**: Test suite completes in < 10 seconds
5. **Confidence**: Critical paths (media processing, job lifecycle) fully tested

## Alternatives Considered

### Alternative 1: Skip Testing Background Goroutines
**Pros**: Simpler, faster to implement
**Cons**: Leaves critical async logic untested
**Decision**: Rejected - too risky for critical functionality

### Alternative 2: Refactor for Testability First
**Pros**: Makes testing easier, improves code quality
**Cons**: Requires significant code changes, delays feature work
**Decision**: Partial adoption - add interfaces where needed

### Alternative 3: Integration Tests Only
**Pros**: Tests real behavior end-to-end
**Cons**: Slower, harder to isolate failures, requires filesystem setup
**Decision**: Use sparingly for full flow validation only

## Implementation Notes

### Making Code More Testable

#### Current Issues:
1. `runScan()` launches goroutine - can't easily wait for completion in tests
2. `filesystem.Coordinator` is concrete type - hard to mock
3. Hard-coded `fmt.Printf` for errors - can't verify error handling

#### Suggested Improvements:
```go
// 1. Add interface for coordinator
type FilesystemScanner interface {
    Scan(ctx, path, resultChan) error
    GetProgress() *Progress
}

// 2. Inject coordinator in constructor
type ScanLibraryUseCase struct {
    // ... existing fields
    scanner FilesystemScanner // injectable for testing
}

// 3. Make runScan return error channel for testing
func (uc *ScanLibraryUseCase) runScan(...) <-chan error {
    errChan := make(chan error, 1)
    go func() {
        // ... existing logic
        errChan <- scanErr
    }()
    return errChan
}
```

## References

- [Testing Concurrent Go Code](https://golang.org/doc/effective_go#concurrency)
- [Table-Driven Tests in Go](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- Existing test patterns in `create_library_test.go`
- Streaming package tests (93.8% coverage) as reference

## Timeline

- **Phase 1 Complete**: 2 weeks (40% coverage)
- **Phase 2 Complete**: 4 weeks (60% coverage)
- **Phase 3 Complete**: 6 weeks (70% coverage)

## Next Steps

1. Review and approve this testing strategy
2. Create GitHub issues for each testing phase
3. Start with Phase 1 (DTO and query tests) - lowest risk, highest value
4. Consider refactoring suggestions for improved testability
5. Set up CI to enforce coverage doesn't decrease
