# E2E Testing Guide

> **Comprehensive End-to-End Testing Framework for PlaybackModule**

This directory contains the complete E2E testing suite for the PlaybackModule, organized by test category and designed to validate real-world usage scenarios.

## 📁 Test Organization

```
e2e/
├── integration/        # Core transcoding workflows & system integration
├── error_handling/     # Error scenarios, edge cases & validation  
├── docker/            # Container integration & volume mounting
├── plugins/           # Plugin discovery, mock vs real comparison
└── performance/       # Performance testing & benchmarking (planned)
```

## 🎯 Test Categories

### 1. Integration Tests (`integration/`)

**Purpose**: Validate core transcoding workflows end-to-end

**Files**:
- `transcoding_integration_test.go` - Original comprehensive integration tests

**Coverage**:
- ✅ DASH transcoding sessions
- ✅ HLS transcoding sessions  
- ✅ Session lifecycle management
- ✅ File generation and serving
- ✅ System integration features

**Key Tests**:
```bash
go test -v ./e2e/integration -run TestE2ETranscodingDASH
go test -v ./e2e/integration -run TestE2ETranscodingHLS
go test -v ./e2e/integration -run TestE2ESystemIntegration
```

### 2. Error Handling Tests (`error_handling/`)

**Purpose**: Validate system behavior under error conditions

**Files**:
- `error_handling_test.go` - Comprehensive error scenario testing

**Coverage**:
- ✅ Invalid input file handling
- 🔍 **Issues Found**: Request validation too permissive
- ✅ Session not found scenarios
- 🔍 **Issues Found**: Content-Type validation missing
- ✅ Network resilience (client disconnects)
- ✅ Multiple concurrent clients
- 🔍 **Issues Found**: HTTP method handling (404 vs 405)

**Key Tests**:
```bash
go test -v ./e2e/error_handling -run TestE2EErrorHandling
go test -v ./e2e/error_handling -run TestE2ENetworkResilienceScenarios
go test -v ./e2e/error_handling -run TestE2EProtocolErrors
```

### 3. Docker Integration Tests (`docker/`)

**Purpose**: Validate containerized deployment and volume mounting

**Files**:
- `docker_integration_test.go` - Docker-specific integration testing

**Coverage**:
- ✅ Docker volume accessibility
- ✅ DASH manifest serving via API
- ✅ DASH segment serving via API  
- ✅ HLS playlist serving via API
- ✅ Docker volume stress testing (5 concurrent sessions)
- ✅ Resource cleanup

**Key Tests**:
```bash
go test -v ./e2e/docker -run TestE2EDockerTranscodingIntegration
go test -v ./e2e/docker -run TestE2EDockerVolumeStress
```

### 4. Plugin Tests (`plugins/`)

**Purpose**: Validate plugin architecture and real plugin integration

**Files**:
- `real_ffmpeg_test.go` - Real FFmpeg plugin integration
- `plugin_discovery_test.go` - Plugin discovery and architecture analysis

**Coverage**:
- ✅ Mock plugin environment (100% functional)
- 🔍 **Critical Issue Found**: Real plugin integration failing
- ✅ Plugin discovery mechanisms
- ✅ Architecture comparison (mock vs real)
- ✅ Plugin requirements analysis

**Key Tests**:
```bash
go test -v ./e2e/plugins -run TestE2ERealFFmpegIntegration
go test -v ./e2e/plugins -run TestE2EPluginDiscovery
go test -v ./e2e/plugins -run TestE2EArchitectureValidation
```

## 🚀 Running E2E Tests

### Quick Start

```bash
# Run all E2E tests
go test -v ./internal/modules/playbackmodule/e2e/...

# Run specific categories
go test -v ./internal/modules/playbackmodule/e2e/docker
go test -v ./internal/modules/playbackmodule/e2e/error_handling
go test -v ./internal/modules/playbackmodule/e2e/integration
go test -v ./internal/modules/playbackmodule/e2e/plugins
```

### Selective Testing

```bash
# Test Docker integration only
go test -v ./e2e/docker -run TestE2EDockerTranscodingIntegration

# Test error handling only  
go test -v ./e2e/error_handling -run TestE2EErrorHandling

# Test real plugin integration
go test -v ./e2e/plugins -run TestE2ERealFFmpegIntegration

# Skip long-running tests
go test -short -v ./e2e/...
```

### Test Environment Setup

The E2E tests automatically:
- ✅ Create temporary directories and test videos
- ✅ Set up in-memory test databases
- ✅ Configure Docker-style directory mounting
- ✅ Create mock plugin environments
- ✅ Clean up all resources after tests

## 📊 Test Results Summary

Based on comprehensive E2E testing, here's our current status:

### ✅ **Fully Working Areas**

| Component | Status | Details |
|-----------|--------|---------|
| **Docker Integration** | ✅ Complete | Volume mounting, directory config working |
| **DASH Streaming** | ✅ Complete | Manifest generation (1014 bytes) and serving |
| **HLS Streaming** | ✅ Complete | Playlist generation (147 bytes) and serving |
| **Session Management** | ✅ Complete | Creation, status, cleanup all working |
| **Network Resilience** | ✅ Complete | Client disconnects, concurrency handled |
| **Mock Environment** | ✅ Complete | 100% functional for development |

### 🔍 **Issues Discovered**

| Issue | Priority | Impact | Status |
|-------|----------|--------|--------|
| **Request Validation** | 🔥 Critical | Security vulnerability | Accepts invalid requests with 201 |
| **Real Plugin Integration** | 🔥 Critical | Core functionality | Returns 500 instead of working |
| **HTTP Method Handling** | ⚠️ Medium | Standards compliance | Returns 404 instead of 405 |
| **Content-Type Validation** | ⚠️ Medium | API robustness | Missing validation |

## 🛠️ Test Architecture

### Mock vs Real Testing

Our E2E suite uses a dual approach:

**Mock Environment** (for rapid development):
- Uses `MockPluginManager` and `MockTranscodingService`
- Simulates FFmpeg behavior without actual transcoding
- 100% test success rate
- Fast execution (< 1 second per test)

**Real Environment** (for production validation):
- Attempts to use actual FFmpeg plugin binaries
- Tests real plugin discovery and loading
- Currently failing due to missing plugin setup
- Slower execution (5-30 seconds per test)

### Test Data Management

```go
// Each test gets isolated environment
type TestData struct {
    VideoPath        string  // Generated test video
    TempDir          string  // Temporary directory  
    TranscodingDir   string  // Docker-style transcoding directory
    ExpectedDuration int     // Video duration for validation
}
```

### Mock Services

Our comprehensive mock implementation includes:

```go
type MockTranscodingService struct {
    sessions map[string]*plugins.TranscodeSession
    mu       sync.RWMutex  // Thread-safe access
}
```

**Features**:
- Thread-safe session management
- Realistic DASH/HLS file generation
- Progress simulation
- Error condition simulation
- Resource cleanup

## 🚨 Critical Findings

### 🔥 **Priority 1: Request Validation Gap**

**Discovery**: E2E tests revealed the API accepts invalid requests

```bash
# Test Result (PROBLEMATIC)
curl -X POST /api/playback/start -H "Content-Type: text/plain" -d "{}"
# Returns: 201 Created ❌ (Should return: 400 Bad Request)

curl -X POST /api/playback/start -d '{}' 
# Returns: 201 Created ❌ (Should return: 400 Bad Request)
```

**Impact**: Security vulnerability - system accepts malformed requests

### 🔥 **Priority 2: Plugin Integration Failure**

**Discovery**: Real FFmpeg tests fail while mocks pass perfectly

```bash
# Mock Environment
go test -run TestE2ETranscodingDASH    # ✅ 201 Created
go test -run TestE2ETranscodingHLS     # ✅ 201 Created

# Real Plugin Environment  
go test -run TestE2ERealFFmpegIntegration  # ❌ 500 Internal Server Error
```

**Root Cause**: "no suitable transcoding plugin found"

### ⚠️ **Priority 3: HTTP Standards Compliance**

**Discovery**: Wrong HTTP status codes for unsupported methods

```bash
curl -X PUT /api/playback/start
# Returns: 404 Not Found ❌ (Should return: 405 Method Not Allowed)
```

## 📋 Production Readiness Checklist

Based on E2E test results:

### Ready for Production ✅
- [x] Core transcoding logic
- [x] Session management  
- [x] Docker deployment
- [x] File generation (DASH/HLS)
- [x] API endpoint functionality
- [x] Network resilience

### Requires Fixes Before Production ⚠️
- [ ] **Request validation** (Security critical)
- [ ] **Real plugin integration** (Functionality critical)  
- [ ] **HTTP method handling** (Standards compliance)
- [ ] **Content-Type validation** (API robustness)

### Enhancements for Production Scale 🚀
- [ ] Rate limiting
- [ ] Authentication/authorization
- [ ] Comprehensive monitoring
- [ ] Performance optimization
- [ ] Resource constraints

## 🧪 Test Development Guidelines

### Adding New E2E Tests

1. **Choose the Right Category**:
   - `integration/` - Core workflow tests
   - `error_handling/` - Error scenarios  
   - `docker/` - Container-specific tests
   - `plugins/` - Plugin-related tests

2. **Follow Naming Conventions**:
   ```go
   func TestE2E[Category][Feature](t *testing.T) {
       // Test implementation
   }
   ```

3. **Use Existing Test Helpers**:
   ```go
   testData := setupTestEnvironment(t)
   defer os.RemoveAll(testData.TempDir)
   
   db := setupTestDatabase(t)
   playbackModule := setupPluginEnabledEnvironment(t, db)
   ```

4. **Include Cleanup**:
   ```go
   defer os.RemoveAll(testData.TempDir)
   
   // Clean up sessions
   req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/playback/session/%s", sessionID), nil)
   router.ServeHTTP(w, req)
   ```

### Test Quality Standards

- ✅ **Isolation**: Each test cleans up after itself
- ✅ **Reliability**: Tests pass consistently  
- ✅ **Speed**: Use `-short` flag for quick tests
- ✅ **Coverage**: Test both success and failure paths
- ✅ **Documentation**: Clear test names and comments

## 📈 Future Enhancements

### Planned Test Categories

1. **Performance Tests** (`performance/`)
   - Transcoding speed benchmarks
   - Concurrent session limits
   - Memory usage profiling
   - CPU utilization monitoring

2. **Security Tests** (`security/`)
   - Input validation fuzzing
   - Authentication bypass attempts
   - Authorization boundary testing
   - Rate limiting validation

3. **Integration Tests** (`integration/`)
   - Real media file testing
   - Client compatibility testing
   - Browser-specific validation
   - Network condition simulation

## 🤝 Contributing

When contributing E2E tests:

1. **Run the full suite** before submitting
2. **Add both mock and real environment tests** when applicable
3. **Include error scenarios** alongside happy path tests
4. **Update this documentation** with new test categories
5. **Ensure tests are deterministic** and not flaky

---

**E2E Testing Status**: 🟡 **Production Ready with Critical Fixes Required**

The comprehensive E2E testing framework successfully validates the PlaybackModule's core functionality while identifying critical areas for improvement before production deployment. 