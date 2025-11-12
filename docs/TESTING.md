# Testing Strategy

## Overview

This document outlines ViewRA's testing strategy, current coverage, and improvement roadmap.

## Current Test Coverage: 37.8%

### Coverage by Layer

#### Domain Layer (Business Logic) - 88.9% average ✅
- **library**: 89.0% - Excellent
- **media**: 91.9% - Excellent ⭐ (improved from 54.1%)
- **scanner**: 85.7% - Excellent

#### Application Layer (Use Cases) - 62.5% average ⚠️
- **common**: 67.9% - Good
- **library**: 32.0% - Needs significant improvement
- **media**: 87.5% - Excellent

#### Infrastructure Layer - 72.5% average ✅
- **persistence/common**: 100.0% - Perfect ⭐ (improved from 47.1%)
- **streaming**: 93.8% - Excellent
- **ffmpeg**: 89.0% - Excellent
- **filesystem**: 76.7% - Good
- **persistence/media**: 64.9% - Good (improved from 52.3%)
- **persistence/library**: 63.5% - Good
- **persistence/adapters**: 59.1% - Fair
- **persistence/scanjob**: 0% - New package, no tests
- **database (sqlc)**: 0% - Generated code, typically not tested

#### API Layer - 13.2% ⚠️
- **api/handlers**: 13.2% - Tests for error handling and helper functions
- **api**: 0% - No tests
- **api/routes**: 0% - No tests

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
Follow Go best practices:
```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name    string
        input   Type
        want    Type
        wantErr bool
    }{
        // Test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Arrange, Act, Assert
        })
    }
}
```

## Priority Improvements

### Phase 1: Critical Gaps (Target: 50% overall)
1. **Application/Library Use Cases** (32% → 70%)
   - CreateLibraryUseCase tests
   - UpdateLibraryUseCase tests
   - DeleteLibraryUseCase tests
   - ScanLibraryUseCase tests

2. **Domain/Media** (54% → 80%)
   - Complete media entity tests
   - Media validation tests
   - Media error cases

3. **Persistence/Common** (47% → 70%)
   - Database helper tests
   - Transaction tests
   - Error mapping tests

### Phase 2: API Layer (Target: 60% overall)
4. **API Handlers** (0% → 70%)
   - LibraryHandler integration tests
   - MediaHandler integration tests
   - StreamHandler integration tests
   - Error response tests

5. **API Routes** (0% → 100%)
   - Route registration tests
   - Middleware tests (future)

### Phase 3: Infrastructure Completeness (Target: 70% overall)
6. **Persistence/ScanJob** (0% → 80%)
   - Repository tests
   - Query tests

7. **Remaining Persistence** (50-65% → 80%)
   - Improve media persistence tests
   - Improve adapter tests

## Test Examples

### Excellent Test Coverage Example: Streaming

The `internal/infrastructure/streaming` package demonstrates our testing standards:

**Coverage**: 93.8%
**Test Files**: 3 (content_type_test.go, range_test.go, service_test.go)
**Test Lines**: 559 lines
**Features Tested**:
- All 22 video/audio MIME types
- All HTTP range formats (normal, suffix, open-ended)
- File operations with real I/O
- Error conditions (11 different error cases)
- Edge cases (single byte, beyond file size, etc.)

### Pattern to Follow

```go
// Example from streaming tests
func TestService_PrepareStream(t *testing.T) {
    // Setup: Create test fixtures
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.mp4")
    testContent := make([]byte, 1000)
    os.WriteFile(testFile, testContent, 0644)

    service := NewService()

    t.Run("Full file without range", func(t *testing.T) {
        // Arrange
        req := StreamRequest{FilePath: testFile}

        // Act
        resp, err := service.PrepareStream(req)

        // Assert
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        defer resp.Close()

        if resp.FileSize != 1000 {
            t.Errorf("FileSize = %d, want 1000", resp.FileSize)
        }
    })

    // More test cases...
}
```

## Running Tests

### All Tests
```bash
go test ./...
```

### With Coverage
```bash
go test ./... -cover
```

### Coverage Report
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Specific Package
```bash
go test ./internal/domain/library/... -v
```

### Short Tests (Skip Integration)
```bash
go test ./... -short
```

## Test Organization

### File Naming
- Test files: `*_test.go`
- Same package: `package mypackage`
- Black-box testing: `package mypackage_test`

### Test Naming
- Functions: `TestFunctionName`
- Methods: `TestTypeName_MethodName`
- Subtests: Descriptive names in natural language

### Test Fixtures
- Use `t.TempDir()` for temporary directories
- Clean up with `defer` statements
- Create reusable test helpers in `testdata/` or `testing.go` files

## Coverage Goals

- **Critical Paths**: 90%+ (domain, core use cases)
- **Infrastructure**: 70%+ (persistence, adapters)
- **API Layer**: 70%+ (handlers, routes)
- **Overall Project**: 70%+

## Continuous Improvement

1. **PR Requirements**:
   - New code must have tests
   - Coverage should not decrease
   - Tests must pass in CI

2. **Monthly Review**:
   - Check coverage trends
   - Identify gaps
   - Prioritize improvements

3. **Quality Metrics**:
   - Test execution time
   - Flaky test rate
   - Coverage by layer

## Resources

- [Go Testing Best Practices](https://go.dev/doc/effective_go#testing)
- [Table-Driven Tests](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [Test Fixtures](https://dave.cheney.net/2016/05/10/test-fixtures-in-go)
