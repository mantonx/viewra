# ViewRA Technical Debt Remediation Plan

Generated: 2025-12-17

## Overview

This plan addresses technical debt identified across four categories:
1. Test compilation failures (blocking)
2. TODOs and incomplete implementations
3. Error handling inconsistencies
4. Documentation gaps

---

## Phase 1: Unblock CI (Priority: CRITICAL)

**Goal**: Fix test compilation so `go test ./...` passes

### Tasks

#### 1.1 Fix media_test.go constructor calls
- **File**: `internal/api/handlers/media_test.go`
- **Lines**: 151, 256
- **Issue**: `NewMediaHandler` expects 4 params, tests pass 3
- **Fix**: Add `nil` as 4th argument for `getTracks`
- **Effort**: 5 minutes

#### 1.2 Fix transcode_test.go constructor call
- **File**: `internal/api/handlers/transcode_test.go`
- **Lines**: 181-194
- **Issue**: `NewTranscodeHandler` expects 11 params, test passes 12
- **Fix**: Remove extra 12th argument (`nil, // TrackRepo`)
- **Effort**: 5 minutes

#### 1.3 Verify library_test.go mock completeness
- **File**: `internal/api/handlers/library_test.go`
- **Issue**: Mock may not implement all `ScanLibraryExecutor` methods
- **Fix**: Compare mock methods against interface, add any missing
- **Effort**: 15 minutes

**Phase 1 Total**: ~30 minutes

---

## Phase 2: Error Handling Standardization (Priority: HIGH)

**Goal**: Consistent error handling across all layers

### Tasks

#### 2.1 Replace direct `sql.ErrNoRows` comparisons
- **Files**:
  - `internal/application/plugins/service.go` (8 occurrences)
  - `internal/infrastructure/persistence/enrichment/queue_repository.go` (2)
  - `internal/infrastructure/persistence/enrichment/pipeline_repository.go` (2)
- **Fix**: Replace `if err == sql.ErrNoRows` with `if errors.Is(err, sql.ErrNoRows)`
- **Effort**: 30 minutes

#### 2.2 Standardize infrastructure → domain error mapping
- **Current state**: Some repos return `nil, nil`, others return domain errors
- **Target state**: All repos convert `sql.ErrNoRows` to domain-specific errors
- **Files to update**:
  - `enrichment/pipeline_repository.go:171` - return domain error instead of `nil, nil`
  - `enrichment/queue_repository.go:172,375` - return domain error or empty struct
- **Effort**: 1 hour

#### 2.3 Deprecate ErrorResponse, migrate to APIError
- **File**: `internal/api/handlers/errors.go`
- **Current**: Both `ErrorResponse` and `APIError` defined, both in use
- **Target**: Remove `handleError()`, use only `ErrorMapper.MapError()`
- **Files to update**: All handlers calling `handleError()`
- **Effort**: 2 hours

#### 2.4 Create domain errors for plugins package
- **File**: `internal/domain/plugins/errors.go` (new)
- **Current**: `ErrPluginNotFound` defined in application layer
- **Target**: Domain errors in domain layer, application layer wraps
- **Effort**: 1 hour

**Phase 2 Total**: ~4-5 hours

---

## Phase 3: Complete Blocking Features (Priority: HIGH)

**Goal**: Resolve TODOs that block functionality

### Tasks

#### 3.1 Implement plugin library querier
- **File**: `internal/infrastructure/plugins/host_data.go:128`
- **Current**: Returns "not implemented" error
- **Target**: Query library metadata and return to plugins
- **Effort**: 2-3 hours

#### 3.2 Implement music artist pagination
- **File**: `internal/infrastructure/persistence/music/repository.go:264`
- **Current**: Returns empty list
- **Target**: Proper paginated query once scanner populates artists
- **Dependency**: Scanner must create artist entities first
- **Effort**: 2 hours (after scanner update)

#### 3.3 Add plugin configuration support
- **Files**: `pkg/plugin/sdk/enricher.go:174,179`
- **Current**: `GetSettingsSchema()` and `Configure()` return empty/success
- **Target**: Implement schema definition and configuration storage
- **Effort**: 3-4 hours

**Phase 3 Total**: ~7-9 hours

---

## Phase 4: Domain Model Completion (Priority: MEDIUM)

**Goal**: Complete TV episode metadata model

### Tasks

#### 4.1 Add missing TV episode fields to domain
- **File**: `internal/domain/tvshow/episode.go`
- **Fields to add**:
  - `AbsoluteNumber int`
  - `DvdSeason int`
  - `DvdEpisode int`
  - `OriginalTitle string`
  - `ContentRating string`
  - `MaturityRating int`
  - `TMDbID int64`
- **Effort**: 30 minutes

#### 4.2 Update persistence mappers
- **File**: `internal/infrastructure/persistence/tvshow/types.go:148-158`
- **Update**: Map new fields from DB row to domain entity
- **Effort**: 30 minutes

#### 4.3 Update API response DTOs
- **Files**: API handlers and OpenAPI spec
- **Update**: Include new fields in TV episode responses
- **Effort**: 1 hour

**Phase 4 Total**: ~2 hours

---

## Phase 5: Media Metadata Enhancements (Priority: MEDIUM)

**Goal**: Generate thumbnails and quality scores during scan

### Tasks

#### 5.1 Implement thumbnail generation
- **File**: `internal/infrastructure/metadata/` (new file)
- **Approach**: Extract frame at 10% duration using FFmpeg
- **Integration**: Call during scanner's metadata extraction phase
- **Storage**: Save to `data/thumbnails/{media_id}.jpg`
- **Effort**: 4-6 hours

#### 5.2 Implement quality score calculation
- **File**: `internal/infrastructure/metadata/quality.go` (new)
- **Heuristic factors**:
  - Resolution (4K=100, 1080p=80, 720p=60, SD=40)
  - Bitrate relative to resolution
  - Codec efficiency (HEVC > H264 > MPEG2)
  - Audio quality (surround > stereo > mono)
- **Effort**: 2-3 hours

#### 5.3 Add created_at to summaries
- **Files**:
  - `internal/infrastructure/persistence/tvshow/repository.go`
  - `internal/infrastructure/persistence/music/repository.go`
- **Update**: Include `created_at` in summary queries
- **Frontend**: Enable "new" badges in cards
- **Effort**: 1-2 hours

**Phase 5 Total**: ~7-11 hours

---

## Phase 6: Plugin System Improvements (Priority: MEDIUM)

**Goal**: Complete plugin search and configuration

### Tasks

#### 6.1 Combine movie + TV search results
- **File**: `internal/infrastructure/plugins/media_querier.go:101`
- **Current**: Only returns movies on generic search
- **Target**: Merge and rank results from both types
- **Effort**: 2-3 hours

**Phase 6 Total**: ~2-3 hours

---

## Phase 7: Documentation (Priority: MEDIUM)

**Goal**: Document complex systems for developers

### Tasks

#### 7.1 Plugin Development Guide
- **File**: `docs/guides/PLUGIN_DEVELOPMENT.md`
- **Contents**:
  - Quick start / minimal plugin
  - Plugin lifecycle (Initialize → Enrich → Shutdown)
  - Capabilities system
  - Host services (HostData, HostStorage)
  - Error handling patterns
  - Configuration format
  - Testing strategies
- **Effort**: 3-4 hours

#### 7.2 Enrichment Pipeline Guide
- **File**: `docs/guides/ENRICHMENT_PIPELINE.md`
- **Contents**:
  - End-to-end flow diagram
  - Queue states and transitions
  - Worker pool architecture
  - Error handling and retries
  - Stage configuration
  - Troubleshooting
- **Effort**: 2-3 hours

#### 7.3 Real-Time Updates Guide
- **File**: `docs/guides/REAL_TIME_UPDATES.md`
- **Contents**:
  - SSE architecture
  - Event types and payloads
  - Frontend integration
  - Connection lifecycle
- **Effort**: 1-2 hours

#### 7.4 HLS Transcoding Guide
- **File**: `docs/guides/HLS_TRANSCODING.md`
- **Contents**:
  - Three-tier strategy
  - On-demand job creation
  - Segment generation
  - Quality profiles
- **Effort**: 2-3 hours

**Phase 7 Total**: ~8-12 hours

---

## Phase 8: Polish (Priority: LOW)

### Tasks

#### 8.1 Inject build version
- **File**: `internal/application/settings/service.go:502`
- **Fix**: Use ldflags to inject version at build time
- **Update**: Makefile build target
- **Effort**: 30 minutes

#### 8.2 Complete transcode manifest test
- **File**: `internal/api/handlers/transcode_test.go:467`
- **Fix**: Add mock for successful manifest serving
- **Effort**: 1-2 hours

**Phase 8 Total**: ~2 hours

---

## Summary

| Phase | Priority | Effort | Description |
|-------|----------|--------|-------------|
| 1 | CRITICAL | 30 min | Fix test compilation |
| 2 | HIGH | 4-5 hrs | Standardize error handling |
| 3 | HIGH | 7-9 hrs | Complete blocking features |
| 4 | MEDIUM | 2 hrs | TV episode domain model |
| 5 | MEDIUM | 7-11 hrs | Media metadata enhancements |
| 6 | MEDIUM | 2-3 hrs | Plugin search improvements |
| 7 | MEDIUM | 8-12 hrs | Documentation |
| 8 | LOW | 2 hrs | Polish items |

**Total Estimated Effort**: 33-45 hours

---

## Recommended Execution Order

1. **Immediate** (before next commit): Phase 1
2. **This sprint**: Phases 2, 3
3. **Next sprint**: Phases 4, 5, 6
4. **Ongoing**: Phase 7 (documentation can be incremental)
5. **When convenient**: Phase 8

---

## Tracking

- [x] Phase 1: Test fixes (completed 2025-12-17)
- [x] Phase 2: Error handling (completed 2025-12-17)
- [x] Phase 3: Blocking features (completed 2025-12-17)
- [ ] Phase 4: Domain model
- [ ] Phase 5: Metadata enhancements
- [ ] Phase 6: Plugin improvements
- [ ] Phase 7: Documentation
- [ ] Phase 8: Polish
