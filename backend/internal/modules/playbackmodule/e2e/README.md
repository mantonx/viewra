### 4. External Plugin Integration Tests (`plugins/`)

**Purpose**: Validate integration with external plugin binaries via GRPC

**Files**:
- `external_plugin_integration_test.go` - External FFmpeg plugin integration via GRPC
- `plugin_discovery_test.go` - Plugin discovery and architecture analysis

**Coverage**:
- ✅ Mock/Simulated environment (100% functional - in-memory testing)
- 🔍 **Critical Issue Found**: External plugin integration failing
- ✅ Plugin discovery mechanisms
- ✅ Architecture comparison (simulated vs external integration)
- ✅ Plugin requirements analysis

**Key Tests**:
```bash
go test -v ./e2e/plugins -run TestE2EExternalPluginIntegration
go test -v ./e2e/plugins -run TestE2EPluginDiscovery
go test -v ./e2e/plugins -run TestE2EArchitectureValidation
```

## 🛠️ Test Architecture

### Mock/Simulated vs External Plugin Integration

Our E2E suite uses a dual approach with clearer terminology:

**Mock/Simulated Environment** (for rapid development):
- Uses `MockPluginManager` and `MockTranscodingService`
- Simulates FFmpeg behavior without actual transcoding
- 100% test success rate
- Fast execution (< 1 second per test)
- Purpose: Test playback module logic and API layer

**External Plugin Integration** (for production validation):
- Uses actual `ExternalPluginManager` with GRPC communication
- Tests real plugin discovery and loading
- Communicates with actual FFmpeg plugin binaries
- Currently failing due to plugin integration issues
- Slower execution (5-30 seconds per test)  
- Purpose: Test full integration chain: PlaybackModule → ExternalPluginManager → GRPC → FFmpeg Binary

### Critical Integration Chain

The external plugin integration tests the complete chain:

```
PlaybackModule 
  ↓
TranscodeManager.DiscoverTranscodingPlugins()
  ↓
ExternalPluginManager.GetRunningPlugins()
  ↓
GRPC Communication
  ↓
FFmpeg Plugin Binary
  ↓
Actual FFmpeg Process
```

## 🚨 Critical Findings

### 🔥 **Priority 1: External Plugin Integration Failure**

**Discovery**: External plugin tests fail while simulated tests pass perfectly

```bash
# Simulated Environment
go test -run TestE2ETranscodingDASH    # ✅ 201 Created
go test -run TestE2ETranscodingHLS     # ✅ 201 Created

# External Plugin Integration  
go test -run TestE2EExternalPluginIntegration  # ❌ 500 Internal Server Error
```

**Root Cause**: "no suitable transcoding plugin found"

**Detailed Analysis**:
The external plugin integration fails because:

1. **Plugin Discovery Issue**: `setupExternalPluginEnvironment` previously used `NewSimplePlaybackModule` (no plugin support)
2. **GRPC Communication**: External plugins need proper GRPC client setup
3. **Plugin Directory**: May not be finding plugins in expected location (`/viewra-data/plugins`)
4. **Build Process**: External plugins may not be built/deployed correctly

**Fixed Approach**:
```go
// BROKEN (old approach):
playbackModule := NewSimplePlaybackModule(logger, db) // No external plugins!

// CORRECT (new approach):
externalPluginManager := pluginmodule.NewExternalPluginManager(db, logger)
err := externalPluginManager.Initialize(ctx, "/viewra-data/plugins", hostServices)
adapter := NewExternalPluginManagerAdapter(externalPluginManager)
playbackModule := NewPlaybackModule(logger, adapter)
``` 