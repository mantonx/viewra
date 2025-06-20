# Playback Module Refactoring Plan

## Goal
Refactor the playback module to follow the same clean architecture pattern as the scanner module.

## Current State
- Single massive module.go file (855 lines) mixing all concerns
- Module lifecycle, business logic, HTTP handlers, and configuration all in one file
- Multiple duplicate files (routes.go, api_handlers.go) causing confusion
- No clear separation between module interface and implementation

## Target State (Based on Scanner Module)
- Small module.go (~250 lines) handling only module lifecycle
- Separate manager.go for business logic and background services
- Dedicated api_handler.go for HTTP route handlers
- Clear separation of concerns

## Refactoring Steps

### Phase 1: Extract Manager (COMPLETED ✅)
- [x] Create manager.go similar to scanner's
- [x] Move all business logic to manager:
  - [x] Configuration management
  - [x] Session management (activeSessionsMu, activeSessions)
  - [x] Transcoding manager setup
  - [x] Cleanup service management
  - [x] Planner logic
  - [x] Background services
- [x] Implement constructor NewManager() with proper dependencies
- [x] Add Start() and Stop() methods for background services

### Phase 2: Extract HTTP Handlers (COMPLETED ✅)
- [x] Create api_handler.go for all HTTP route handlers
- [x] Move all handler methods to APIHandler struct:
  - [x] HandlePlaybackDecision
  - [x] HandleStartTranscode
  - [x] HandleSeekAhead
  - [x] HandleGetSession
  - [x] HandleStopTranscode
  - [x] HandleListSessions
  - [x] HandleGetStats
  - [x] HandleHealth
  - [x] HandleCleanupRun
  - [x] HandleCleanupStats
  - [x] Streaming handlers (manifest, segments, etc.)
- [x] Fix handler signatures to use standard Gin context

### Phase 3: Refactor module.go (COMPLETED ✅)
- [x] Remove PlaybackModule struct completely
- [x] Keep only module lifecycle functions:
  - [x] init() for registration
  - [x] NewModule() constructor
  - [x] Init() - create manager, run migrations
  - [x] RegisterRoutes() - create APIHandler and register routes
  - [x] Shutdown() - stop manager
  - [x] ID(), Description() - module metadata
- [x] Remove all business logic, handlers, and state management
- [x] Make it similar in size and scope to scanner's module.go

### Phase 4: Fix External Dependencies (COMPLETED ✅)
- [x] Update mediamodule to use new architecture:
  - [x] Changed playback_integration.go to use Manager instead of PlaybackModule
  - [x] Updated module.go to create Module and access Manager
  - [x] Fixed route registration
- [x] Updated server.go:
  - [x] Removed old plugin connection code (now handled internally)
  - [x] Cleaned up unused imports and adapters

### Phase 5: Update E2E Tests (COMPLETED ✅)
- [x] Updated all E2E test files to use new Module architecture:
  - [x] docker/docker_test.go
  - [x] plugins/plugin_discovery_test.go
  - [x] integration/transcoding_integration_test.go
  - [x] external_plugin_integration_test.go
  - [x] error_handling/error_handling_test.go
- [x] Fixed test helpers to use Module.RegisterRoutes()
- [x] Updated benchmark tests

### Phase 6: Cleanup (COMPLETED ✅)
- [x] Delete module.go.backup
- [x] Ensure no duplicate files remain
- [x] Create dedicated routes.go for route registration
- [x] Update documentation

## File Structure After Refactoring

```
playbackmodule/
├── module.go          (278 lines - lifecycle only) ✅
├── manager.go         (327 lines - business logic) ✅
├── api_handler.go     (396 lines - HTTP handlers) ✅
├── routes.go          (NEW - route registration) ✅
├── transcode_manager.go (existing - unchanged)
├── planner.go         (existing - unchanged)
├── types.go           (existing - unchanged)
├── plugin_adapter.go  (existing - unchanged)
├── request_converter.go (existing - unchanged)
└── README.md         (updated documentation) ✅
```

## Benefits
1. **Clear Separation of Concerns**: Each file has a single, well-defined responsibility
2. **Improved Testability**: Business logic isolated in manager, easier to test
3. **Better Maintainability**: Smaller, focused files are easier to understand and modify
4. **Consistent Architecture**: Follows the same pattern as other modules
5. **No Tech Debt**: Clean implementation without compatibility layers

## Migration Notes
For developers updating code that uses the old PlaybackModule:
1. Replace `playbackModule.GetManager()` with direct manager access
2. Use `Module.RegisterRoutes()` instead of old registration methods
3. Access business logic through the Manager interface

## Status: COMPLETED ✅
The refactoring has been successfully completed with all tests passing and no tech debt introduced. 