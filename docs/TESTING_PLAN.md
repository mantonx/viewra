# Testing Plan - ViewRA v2

## Current Status (as of 2025-11-11)

### Overall Coverage: 44.1%

### Coverage by Package

| Package | Coverage | Priority | Status |
|---------|----------|----------|--------|
| **High Coverage (>80%)** | | | |
| internal/infrastructure/persistence/common | 100.0% | - | ✅ Complete |
| internal/infrastructure/streaming | 93.8% | Medium | Good |
| internal/domain/media | 91.9% | - | ✅ Complete |
| internal/domain/library | 89.0% | - | ✅ Complete |
| internal/infrastructure/ffmpeg | 89.0% | Medium | Good |
| internal/application/media | 87.5% | Medium | Good |
| internal/domain/scanner | 85.7% | Medium | Good |
| **Medium Coverage (60-80%)** | | | |
| internal/infrastructure/filesystem | 76.7% | Medium | Good |
| internal/application/common | 67.9% | Medium | Good |
| internal/infrastructure/persistence/media | 64.9% | Medium | Good |
| internal/infrastructure/persistence/library | 63.5% | Medium | Good |
| internal/application/library | 62.1% | High | Recently Improved |
| **Low Coverage (<60%)** | | | |
| internal/infrastructure/persistence/adapters | 59.1% | High | Needs Work |
| internal/api/handlers | 13.2% | **Critical** | Urgent |
| **No Coverage (0%)** | | | |
| internal/infrastructure/persistence/scanjob | 0.0% | **Critical** | Urgent |
| internal/api/routes | 0.0% | Low | Not Critical |
| internal/api | 0.0% | Low | Not Critical |

## Recent Achievements

### Scan Library Testing (Phases 1-3)
- **Phase 1**: DTO conversion functions - 100% coverage
- **Phase 2**: Query methods - 100% coverage
- **Phase 3**: Media processing functions - 100% coverage
- **Result**: Application/library package improved from 32% to 62.1%

## Priority Areas for Improvement

### Priority 1: Critical Infrastructure (Target: +15% overall)

#### 1.1 Scan Job Repository (0% → 80%+)
**Package**: `internal/infrastructure/persistence/scanjob`
**Impact**: Very High - Critical for scan functionality
**Estimated Effort**: 2-3 days
**Target**: 80%+ coverage

**Files to Test**:
- `repository.go` (343 lines) - All CRUD operations

**Test Strategy**:
- Integration tests with both SQLite and PostgreSQL
- Use sqlmock or testcontainers for database testing
- Test all CRUD operations
- Test error handling (sql.ErrNoRows, connection errors)
- Test data conversion between domain and persistence layers

**Test Cases Needed**:
1. Create scan job (SQLite and Postgres)
2. GetByID - existing and non-existent
3. GetLatestByLibrary - with and without jobs
4. ListByLibrary - with limits, empty results
5. ListRunning - filter by status
6. UpdateProgress - valid and invalid IDs
7. UpdateStatus - valid and invalid IDs
8. Complete - success and error paths
9. Delete - valid and invalid IDs
10. DeleteOld - various retention periods
11. convertToScanJob - both database types

**Complexity**: Medium-High
- Requires database setup
- Dual database support adds complexity
- Type conversions need careful testing

#### 1.2 API Handlers (13.2% → 70%+)
**Package**: `internal/api/handlers`
**Impact**: High - User-facing functionality
**Estimated Effort**: 3-4 days
**Target**: 70%+ coverage

**Files to Test**:
- `library.go` - Library CRUD and scan endpoints (0% coverage)
- `media.go` - Media list and get endpoints (0% coverage)
- `stream.go` - Streaming endpoint (0% coverage)
- `errors.go` - Already at 100%
- `helpers.go` - Already at 100%

**Test Strategy**:
- Use httptest for HTTP handler testing
- Mock all use cases
- Test request parsing and validation
- Test response serialization
- Test error handling

**Test Cases per Handler**:
1. **LibraryHandler**:
   - Create: valid request, validation errors, use case errors
   - List: successful list, empty results
   - Get: existing library, non-existent library
   - Update: valid update, validation errors, not found
   - Delete: successful delete, not found
   - Scan: start scan, library not found

2. **MediaHandler**:
   - List: successful list, with filters, empty results
   - Get: existing media, non-existent media

3. **StreamHandler**:
   - Stream: successful stream, invalid range, file not found

**Complexity**: Medium
- HTTP testing well-established in Go
- Need to mock use cases properly
- JSON marshaling/unmarshaling testing

### Priority 2: Improve Medium Coverage (Target: +5% overall)

#### 2.1 Persistence Adapters (59.1% → 80%+)
**Package**: `internal/infrastructure/persistence/adapters`
**Impact**: Medium - Used by all repositories
**Estimated Effort**: 1 day
**Target**: 80%+ coverage

**Missing Coverage**:
- `setField` function: 36.8% coverage
- Complex type conversions
- Edge cases in struct conversion

**Test Cases Needed**:
1. ConvertStruct - complex nested structures
2. ConvertStruct - with nil pointers
3. ConvertStruct - with embedded structs
4. setField - all data types (int, int32, int64, string, time.Time, etc.)
5. setField - sql.Null* types
6. setField - invalid type conversions
7. ConvertSlice - various slice types
8. ConvertSlice - empty slices

**Complexity**: Low-Medium
- Pure logic testing
- No external dependencies
- Requires understanding of reflection

#### 2.2 Application/Library StartScan (62.1% → 75%+)
**Package**: `internal/application/library`
**Impact**: High - Core scanning functionality
**Estimated Effort**: 3-4 days
**Target**: 75%+ coverage

**Missing Coverage**:
- `StartScan` - 0% coverage (job creation, goroutine launch)
- `runScan` - 0% coverage (main scan loop)
- `processResults` - 0% coverage (channel-based result processing)

**Test Strategy**:
- Mock all repositories and filesystem coordinator
- Use channels for testing async behavior
- Add synchronization mechanisms for goroutine testing
- Consider refactoring for better testability

**Test Cases Needed**:
1. **StartScan**:
   - Library doesn't exist
   - Library already being scanned
   - Successful scan start
   - Job creation error

2. **runScan** (Complex):
   - Successful scan completion
   - Filesystem coordinator errors
   - Context cancellation
   - Progress tracking

3. **processResults** (Complex):
   - Process empty channel
   - Process results for each media type
   - Handle errors in channel
   - Ticker-based progress updates

**Complexity**: Very High
- Goroutine orchestration
- Channel-based concurrency
- Time-based logic (tickers)
- Requires careful test design

**Suggested Refactoring**:
```go
// Make goroutine launcher injectable
type GoroutineLauncher func(func())

// Make coordinator mockable
type ScanCoordinator interface {
    Scan(ctx, path, resultChan) error
    GetProgress() *Progress
}
```

### Priority 3: Low-Hanging Fruit (Target: +2% overall)

#### 3.1 Domain Scanner (85.7% → 95%+)
**Package**: `internal/domain/scanner`
**Impact**: Low - Small gaps remaining
**Estimated Effort**: 0.5 days

#### 3.2 Application Media (87.5% → 95%+)
**Package**: `internal/application/media`
**Impact**: Low - Small gaps remaining
**Estimated Effort**: 0.5 days

#### 3.3 Infrastructure FFmpeg (89.0% → 95%+)
**Package**: `internal/infrastructure/ffmpeg`
**Impact**: Low - Small gaps remaining
**Estimated Effort**: 0.5 days

## Implementation Roadmap

### Week 1: Critical Infrastructure - Part 1
**Goal**: Add scan job repository tests
**Target**: +5% overall coverage (44.1% → 49%)

- [ ] Day 1-2: Set up database testing infrastructure (sqlmock or testcontainers)
- [ ] Day 3-4: Implement scan job repository tests (SQLite)
- [ ] Day 5: Implement scan job repository tests (PostgreSQL)

### Week 2: Critical Infrastructure - Part 2
**Goal**: Add API handler tests
**Target**: +8% overall coverage (49% → 57%)

- [ ] Day 1-2: Test LibraryHandler (Create, List, Get, Update, Delete)
- [ ] Day 3: Test LibraryHandler (Scan endpoint)
- [ ] Day 4: Test MediaHandler (List, Get)
- [ ] Day 5: Test StreamHandler (Stream)

### Week 3: Medium Coverage Improvements
**Goal**: Improve persistence adapters and application library
**Target**: +4% overall coverage (57% → 61%)

- [ ] Day 1: Complete persistence adapters tests (59.1% → 80%)
- [ ] Day 2-3: Add StartScan tests (mock-based, simpler cases)
- [ ] Day 4-5: Add processResults tests (channel-based)

### Week 4: Polish and Low-Hanging Fruit
**Goal**: Close remaining gaps
**Target**: +3% overall coverage (61% → 64%)

- [ ] Day 1: Domain scanner improvements (85.7% → 95%)
- [ ] Day 2: Application media improvements (87.5% → 95%)
- [ ] Day 3: Infrastructure FFmpeg improvements (89.0% → 95%)
- [ ] Day 4: Address any runScan testing (if feasible)
- [ ] Day 5: Documentation and testing strategy review

## Target Coverage by End of Month

| Package | Current | Target | Gain |
|---------|---------|--------|------|
| internal/infrastructure/persistence/scanjob | 0.0% | 80%+ | +80% |
| internal/api/handlers | 13.2% | 70%+ | +57% |
| internal/infrastructure/persistence/adapters | 59.1% | 80%+ | +21% |
| internal/application/library | 62.1% | 75%+ | +13% |
| internal/domain/scanner | 85.7% | 95%+ | +9% |
| internal/application/media | 87.5% | 95%+ | +8% |
| internal/infrastructure/ffmpeg | 89.0% | 95%+ | +6% |

**Overall Project**: 44.1% → **64%+** (+20%)

## Success Criteria

1. **Coverage**: Achieve 64%+ overall test coverage
2. **Critical Paths**: 80%+ coverage for all critical infrastructure
3. **Reliability**: All tests pass consistently (no flaky tests)
4. **Performance**: Test suite completes in < 30 seconds
5. **Maintainability**: Tests are readable and well-documented
6. **CI Integration**: Coverage reports integrated into CI pipeline

## Testing Best Practices

### 1. Table-Driven Tests
Use the established pattern:
```go
tests := []struct {
    name      string
    input     Type
    setupMock func(*mockRepo)
    want      Type
    wantErr   bool
}{
    // test cases
}
```

### 2. Mock Repositories
- Create interface-compliant mocks
- Use in-memory maps for data storage
- Support error injection for testing error paths

### 3. HTTP Handler Testing
- Use `httptest.NewRecorder()` for response capture
- Use `httptest.NewRequest()` for request creation
- Test both success and error paths
- Verify status codes and response bodies

### 4. Database Testing
- Use sqlmock for unit tests (faster)
- Use testcontainers for integration tests (more realistic)
- Test both SQLite and PostgreSQL paths
- Clean up test data after each test

### 5. Concurrency Testing
- Use sync.WaitGroup for goroutine synchronization
- Use channels with timeouts to prevent hangs
- Make time-based logic injectable (tickers)
- Consider table-driven tests even for concurrent code

## Next Steps

1. ✅ Complete Phases 1-3 of scan library testing
2. 🔄 Create this testing plan document
3. ⏭️ Start Week 1: Scan job repository tests
4. ⏭️ Continue with API handler tests
5. ⏭️ Monitor coverage improvements and adjust plan

## References

- [ADR 004: Scan Library Testing Strategy](decisions/004-scan-library-testing-strategy.md)
- [Testing Guide](https://go.dev/doc/tutorial/add-a-test)
- [Table-Driven Tests](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [httptest Package](https://pkg.go.dev/net/http/httptest)
- [sqlmock Package](https://github.com/DATA-DOG/go-sqlmock)
