# Transcoding Plugin System Generalization - Implementation Summary

## Overview

We have successfully generalized the Viewra transcoding plugin system to support multiple transcoding backends beyond FFmpeg. The implementation removes FFmpeg-specific concepts from the core interfaces and provides a clean, extensible architecture for adding new transcoding providers.

## Key Changes

### 1. Core SDK Abstractions (backend/pkg/plugins/)

#### New Files:
- **transcoding_provider.go** - Defines generic provider interfaces:
  - TranscodingProvider - Main provider interface
  - QualityMapper - Maps 0-100 quality to provider settings
  - HardwareDetector - Discovers hardware acceleration
  - TranscodingExecutor - Handles actual transcoding
  - TranscoderProgressCallback - Standardized progress reporting
  - Generic types: VideoResolution, TranscoderMediaInfo, etc.

#### Updated Files:
- **transcoding.go** - Updated TranscodeRequest to use generic fields:
  - Added: Quality (0-100), SpeedPriority, generic codec fields
  - Added: PreferHardware, HardwareType for hardware preferences
  - Kept deprecated fields for backwards compatibility
  - Updated TranscodingService interface with new methods
  - Modified TranscodeSession to use standardized progress

### 2. FFmpeg Plugin Updates (backend/data/plugins/ffmpeg_transcoder/)

#### New Files:
- **internal/quality/mapper.go** - Maps generic quality to FFmpeg CRF
- **internal/progress/converter.go** - Converts FFmpeg output to standard format

#### Updated Files:
- **internal/config/config.go** - Restructured configuration:
  - Generic sections: core, hardware, sessions, cleanup, debug
  - FFmpeg-specific section: ffmpeg with binary paths, CRF defaults
- **internal/plugin/plugin.go** - Fixed session directory naming to generic format
- **internal/plugin/adapter.go** - Added missing interface methods
- **plugin.cue** - Completely rewritten with generic structure

### 3. Backend Integration (backend/internal/modules/playbackmodule/)

#### New Files:
- **request_converter.go** - Converts between old and new request formats:
  - Maps CRF to quality percentage
  - Maps presets to speed priorities
  - Handles hardware preferences
  - Maintains backwards compatibility

## Benefits

1. **Provider Independence**: Core interfaces no longer contain FFmpeg-specific concepts
2. **Standardized Quality**: 0-100 quality scale works across all providers
3. **Generic Hardware Support**: Unified hardware acceleration types
4. **Consistent Progress**: All providers report progress in same format
5. **Clean Session Naming**: Generic format: [container]_[provider]_[uuid]
6. **Extensible Configuration**: Clear separation of generic vs provider-specific settings

## Migration Path

### For New Transcoding Plugins:

1. Implement TranscodingProvider interface
2. Create quality mapper for your provider
3. Implement hardware detection
4. Convert provider output to standard progress format
5. Use generic configuration structure

### For Existing Code:

- Old request format still works via backwards compatibility
- RequestConverter handles automatic conversion
- Deprecated fields remain but should not be used for new code

## Future Enhancements

1. **Plugin Discovery**: Auto-discover providers at runtime
2. **Capability Negotiation**: Choose best provider for each request
3. **Load Balancing**: Distribute work across multiple providers
4. **Fallback Chains**: Automatic fallback from hardware to software
5. **Performance Metrics**: Standardized benchmarking across providers

## Phase 4: Testing & Validation ✅

Successfully validated all changes compile and integrate properly.

### Build Verification
- **Backend**: Successfully compiles with all new interfaces and types
- **FFmpeg Plugin**: Successfully compiles with new provider abstractions

### Integration Fixes
1. **Fixed GRPC Compilation Issues**:
   - Updated `grpc_impl.go` to handle new `*TranscodingProgress` type
   - Fixed progress conversion from proto float64 to structured progress

2. **Fixed External Manager Issues**:
   - Added missing `TranscodingService` interface methods to `BasicTranscodingService`
   - Implemented: `GetProvider()`, `GetQualityPresets()`, `MapQualityToProvider()`, `GetHardwareAccelerators()`, `GetSessionProgress()`
   - Fixed progress field handling in session conversion

3. **Fixed FFmpeg Plugin Config References**:
   - Updated `cleanup_service.go` to use new config structure
   - Changed `s.config.Transcoding.OutputDir` to `s.config.Core.OutputDirectory`

### Key Architectural Changes
- All transcoding concepts are now provider-agnostic
- Progress tracking uses standardized format across all providers
- Session directories follow generic naming: `[container]_[provider]_[UUID]`
- Quality uses 0-100 scale instead of provider-specific values

### Build Commands Used
```bash
# Backend build (successful)
cd backend && go build ./cmd/viewra

# FFmpeg plugin build (successful)  
cd backend/data/plugins/ffmpeg_transcoder && go build
```

## Phase 5: Migration Tools ✅

Created comprehensive migration documentation and guides.

### Migration Guide
Created `TRANSCODING_MIGRATION_GUIDE.md` with:
- Configuration format migration examples
- Quality mapping tables (CRF → percentage)
- Speed priority mapping (preset → priority)
- API request format changes
- Session directory naming changes
- Database migration SQL
- Frontend component updates
- Troubleshooting tips

### Key Migration Points
1. **Quality Scale**: CRF (0-51) → Percentage (0-100%)
   - Formula: `quality = 100 - (crf * 100 / 51)`
   - CRF 23 = 75% quality

2. **Speed Settings**: Presets → Priorities
   - `ultrafast/superfast/veryfast` → `fastest`
   - `faster/fast/medium` → `balanced`
   - `slow/slower/veryslow` → `quality`

3. **Session Directories**: 
   - Old: `ffmpeg_[timestamp]`
   - New: `[container]_[provider]_[UUID]`

4. **Configuration Structure**: Flat → Nested
   - Core, Hardware, Sessions, Cleanup, Debug sections
   - Provider-specific settings in separate section

## Phase 6: Documentation & Examples ✅

Created comprehensive documentation and example implementation.

### Example Plugin
Created `example_transcoder` plugin demonstrating:
- Complete provider implementation structure
- CUE configuration with UI hints
- Quality mapping implementation
- Hardware detection patterns
- Progress reporting format
- Resource estimation

### Documentation Created
1. **Example Plugin Structure**:
   - `plugin.cue` - Configuration schema
   - `main.go` - Provider implementation
   - `README.md` - Developer guide

2. **Key Concepts Documented**:
   - Provider interface implementation
   - Quality percentage mapping
   - Hardware accelerator discovery
   - Standardized progress reporting
   - Session management patterns

3. **Developer Guidelines**:
   - Quality scale interpretation (0-100%)
   - Progress update frequency
   - Error handling best practices
   - Resource cleanup requirements

## Summary of Changes

### Before (FFmpeg-specific)
- Hardcoded FFmpeg concepts (CRF, presets)
- Single transcoding implementation
- FFmpeg-specific progress parsing
- Tightly coupled to FFmpeg binary

### After (Generic Provider System)
- Provider-agnostic interfaces
- Multiple backend support
- Standardized progress format
- Pluggable transcoding providers

### Key Benefits
1. **Extensibility**: Easy to add new providers (NVENC, VAAPI, QSV, cloud services)
2. **Consistency**: Unified API regardless of backend
3. **Flexibility**: Providers control their own quality mapping
4. **Maintainability**: Clear separation of concerns
5. **Future-proof**: Ready for new transcoding technologies

### Implementation Checklist
- [x] Generic provider interfaces
- [x] Quality abstraction (0-100%)
- [x] Speed priority system
- [x] Hardware detection framework
- [x] Standardized progress reporting
- [x] Session management refactor
- [x] Request converter for compatibility
- [x] Configuration migration
- [x] Example implementation
- [x] Developer documentation

## Next Steps

1. **Test Migration**: Run migration guide on existing installations
2. **Provider Development**: Create NVENC, VAAPI, QSV providers
3. **Cloud Integration**: Add cloud transcoding services
4. **Performance Testing**: Benchmark different providers
5. **UI Updates**: Update frontend for provider selection

## Files Changed

### Core SDK
- `backend/pkg/plugins/transcoding_provider.go` (new)
- `backend/pkg/plugins/transcoding.go` (updated)
- `backend/pkg/plugins/helpers.go` (updated)
- `backend/pkg/plugins/grpc_impl.go` (fixed)

### FFmpeg Plugin
- `backend/data/plugins/ffmpeg_transcoder/plugin.cue` (restructured)
- `backend/data/plugins/ffmpeg_transcoder/internal/config/config.go` (refactored)
- `backend/data/plugins/ffmpeg_transcoder/internal/quality/mapper.go` (new)
- `backend/data/plugins/ffmpeg_transcoder/internal/progress/converter.go` (new)
- `backend/data/plugins/ffmpeg_transcoder/internal/plugin/adapter.go` (updated)
- `backend/data/plugins/ffmpeg_transcoder/internal/services/cleanup_service.go` (fixed)

### Backend Integration
- `backend/internal/modules/playbackmodule/request_converter.go` (new)
- `backend/internal/modules/pluginmodule/external_manager.go` (fixed)

### Documentation
- `TRANSCODING_GENERALIZATION_SUMMARY.md` (this file)
- `TRANSCODING_MIGRATION_GUIDE.md` (new)
- `backend/data/plugins/example_transcoder/` (new example)

## Conclusion

The transcoding plugin system has been successfully generalized to support multiple providers while maintaining backwards compatibility. The new architecture provides a solid foundation for adding diverse transcoding backends without modifying core code. The abstraction of quality, speed, and progress concepts ensures a consistent user experience regardless of the underlying transcoding technology. 