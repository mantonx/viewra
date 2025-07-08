# Plugin System Simplification Plan

## Current State Analysis

### Plugin Types Currently in Use
1. **Transcoding Providers** (FFmpeg variants)
   - `ffmpeg_software` - Software encoding
   - `ffmpeg_nvidia` - NVIDIA hardware acceleration
   - `ffmpeg_qsv` - Intel Quick Sync
   - `ffmpeg_vaapi` - VA-API acceleration

2. **Enrichment Services**
   - `tmdb_enricher_v2` - Movie/TV metadata
   - `musicbrainz_enricher` - Music metadata (empty)
   - `audiodb_enricher` - Audio metadata (empty)

3. **Built-in Providers**
   - File pipeline provider (internal to transcoding module)

### Unused Plugin Interfaces
The SDK currently defines many unused interfaces:
- MetadataScraperService
- ScannerHookService  
- AdminPageService
- DatabaseService
- APIRegistrationService
- SearchService
- HealthMonitorService
- ConfigurationService
- PerformanceMonitorService
- EnhancedAdminPageService
- DashboardSectionProvider
- And many dashboard-related interfaces

## Simplification Strategy

### Phase 1: Clean Up SDK Interfaces
1. Create new simplified `interfaces.go` with only:
   - `Implementation` interface (core plugin methods)
   - `TranscodingProvider` interface
   - `EnrichmentService` interface
   - Basic types (PluginContext, PluginInfo, etc.)

2. Remove all dashboard/UI related interfaces
3. Remove unused service interfaces
4. Keep asset management for enrichment plugins

### Phase 2: Simplify Plugin Discovery
1. Current system uses complex gRPC communication
2. Simplify to:
   - Direct binary execution for transcoding plugins
   - Simple JSON-RPC or REST for enrichment plugins
   - Remove unnecessary abstraction layers

### Phase 3: Consolidate Plugin Management
1. Current state:
   - Plugin module manages external plugins
   - Transcoding module has its own provider registry
   - Complex service injection between modules

2. Target state:
   - Single plugin registry in plugin module
   - Plugin module exposes transcoding providers to transcoding module
   - Direct provider access without multiple adapters

### Phase 4: Update Existing Plugins
1. Update FFmpeg plugins to use simplified interface
2. Update TMDB enricher to use simplified interface
3. Remove empty enricher plugins or implement them

## Implementation Steps

### Step 1: Create Simplified Interfaces
```go
// sdk/interfaces_v2.go
package plugins

type Implementation interface {
    Initialize(ctx *PluginContext) error
    Start() error
    Stop() error
    Info() (*PluginInfo, error)
    Health() error
    
    // Only the services we use
    TranscodingProvider() TranscodingProvider
    EnrichmentService() EnrichmentService
}
```

### Step 2: Update Plugin Module
- Remove complex gRPC client/server code
- Implement direct plugin loading
- Expose clean API to other modules

### Step 3: Update Transcoding Module
- Remove provider registry duplication
- Get providers directly from plugin module
- Simplify initialization

### Step 4: Test and Migrate
- Test with existing FFmpeg plugins
- Ensure TMDB enricher still works
- Update documentation

## Benefits of Simplification

1. **Reduced Complexity**
   - Remove ~80% of unused code
   - Fewer abstraction layers
   - Easier to understand and maintain

2. **Better Performance**
   - Less overhead from gRPC communication
   - Direct function calls where possible
   - Reduced memory usage

3. **Easier Plugin Development**
   - Simpler interface to implement
   - Clear examples with FFmpeg and TMDB
   - Less boilerplate code

## Risks and Mitigation

1. **Breaking Changes**
   - Keep old interfaces temporarily
   - Provide migration guide
   - Update plugins incrementally

2. **Future Extensibility**
   - Design interfaces with extension in mind
   - Use versioning for interface changes
   - Keep plugin context flexible

## Next Steps

1. Review and approve this plan
2. Create new simplified interfaces
3. Update plugin module to use simplified system
4. Migrate existing plugins
5. Remove old code
6. Update documentation

## Timeline Estimate

- Phase 1: 2-3 hours (interface cleanup)
- Phase 2: 3-4 hours (plugin discovery)
- Phase 3: 4-5 hours (consolidation)
- Phase 4: 2-3 hours (plugin updates)

Total: ~12-15 hours of focused work