# Testing Strategy

## Overview

This document outlines ViewRA's testing strategy, current coverage, and improvement roadmap.

## Current Test Coverage: 44.1%

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
| internal/infrastructure/persistence/progress | 0.0% | High | Needs Tests |
| internal/api/handlers/progress | 0.0% | High | Needs Tests |
| internal/api/routes | 0.0% | Low | Not Critical |
| internal/api | 0.0% | Low | Not Critical |

## Testing Principles

### 1. Test Pyramid
- **Unit Tests** (70%): Fast, isolated tests for business logic
- **Integration Tests** (20%): Test component interactions
- **E2E Tests** (10%): Full system tests (future)

### 2. What to Test
- ✅ Domain logic and business rules
- ✅ Use case orchestration
- ✅ Data transformations
- ✅ Error handling and edge cases
- ✅ Infrastructure adapters
- ⚠️ HTTP handlers (integration tests)
- ❌ Generated code (sqlc)
- ❌ Simple getters/setters

### 3. Test Structure

Follow Go best practices with table-driven tests:

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name    string
        input   Type
        want    Type
        wantErr bool
    }{
        {
            name: "happy path",
            input: validInput,
            want: expectedOutput,
            wantErr: false,
        },
        {
            name: "error case",
            input: invalidInput,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Arrange
            // Act
            got, err := FunctionName(tt.input)

            // Assert
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            require.Equal(t, tt.want, got)
        })
    }
}
```

## Priority Improvements

### Phase 1: Critical Infrastructure (Target: +15% overall)

#### 1.1 Scan Job Repository (0% → 80%+)
**Package**: `internal/infrastructure/persistence/scanjob`
**Impact**: Very High - Critical for scan functionality
**Estimated Effort**: 2-3 days
**Target**: 80%+ coverage

**Test Strategy**:
- Integration tests with both SQLite and PostgreSQL
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

#### 1.2 Progress Feature Repository & Handlers (0% → 80%+)
**Packages**:
- `internal/infrastructure/persistence/progress`
- `internal/api/handlers/progress`

**Impact**: High - User-facing watch progress tracking
**Estimated Effort**: 2-3 days
**Target**: 80%+ coverage

**What Exists**:
- ✅ Domain tests: entity_test.go (comprehensive)
- ✅ Application tests: update_progress_test.go, get_progress_test.go, mark_watched_test.go
- ❌ Repository tests: Missing
- ❌ API handler tests: Missing

**Test Cases Needed**:
1. Repository integration tests with dual DB support
2. HTTP handler tests for all 8 endpoints
3. Request validation tests
4. Error response tests
5. Pagination tests

#### 1.3 API Handlers (13.2% → 70%+)
**Package**: `internal/api/handlers`
**Impact**: High - User-facing functionality
**Estimated Effort**: 3-4 days
**Target**: 70%+ coverage

**Files to Test**:
- `library.go` - Library CRUD and scan endpoints (0% coverage)
- `media.go` - Media list and get endpoints (0% coverage)
- `stream.go` - Streaming endpoint (0% coverage)
- `errors.go` - Already at 100% ✅
- `helpers.go` - Already at 100% ✅

**Test Strategy**:
- Use httptest for HTTP handler testing
- Mock all use cases
- Test request parsing and validation
- Test response serialization
- Test error handling and status codes

### Phase 2: Medium Priority (Target: +10% overall)

#### 2.1 Persistence Adapters (59.1% → 80%+)
**Package**: `internal/infrastructure/persistence/adapters`
**Estimated Effort**: 1-2 days

#### 2.2 Application/Library (62.1% → 80%+)
**Package**: `internal/application/library`
**Estimated Effort**: 1-2 days
**Note**: Recently improved from 32% to 62.1%

### Phase 3: Nice-to-Have (Future)
- API routes (currently 0% but low priority - simple registration)
- Additional edge cases for well-covered packages
- Performance benchmarks
- E2E integration tests

## Running Tests

### Quick Commands

```bash
# Run all tests
make test

# Run tests with coverage
go test -v -coverprofile=coverage.out ./...

# View coverage report
go tool cover -html=coverage.out

# Coverage summary
go tool cover -func=coverage.out | grep total

# Run specific package tests
go test -v ./internal/domain/media/...

# Run tests in short mode (skip integration tests)
go test -short ./...
```

### Coverage Targets

- **Domain Layer**: Maintain >85% (currently 88.9%)
- **Application Layer**: Improve to >70% (currently 62.5%)
- **Infrastructure Layer**: Improve to >75% (currently 72.5%)
- **API Layer**: Improve to >60% (currently 13.2%)

**Overall Target**: 60% coverage within 2-3 weeks

## Test Conventions

### File Naming
- Test files: `*_test.go` in same package
- Integration tests: Use build tag `// +build integration` if needed
- Mock files: `mock_*.go` (if using mockgen)

### Test Data
- Use test fixtures in `testdata/` directories
- Keep test data minimal and focused
- Use table-driven tests for multiple scenarios

### Assertions
- Use `testify/require` for assertions that should stop test execution
- Use `testify/assert` for non-critical assertions
- Always include meaningful error messages

### Database Tests
- Use in-memory SQLite for fast tests
- Use test containers for PostgreSQL integration tests
- Clean up database state after each test
- Use transactions for test isolation when possible

## Continuous Improvement

### Weekly Goals
- Add 2-3% coverage per week
- Focus on one package at a time
- Review and update this document monthly

### Automated Checks
- CI runs all tests on every PR
- Coverage report generated and tracked
- Fail PR if coverage decreases by >2%

### Documentation
- Update test counts in this document after major additions
- Document complex test scenarios
- Link to ADRs for testing decisions

**Last Updated**: 2025-11-12
