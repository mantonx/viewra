# Incomplete Implementations Audit

**Date**: 2025-11-12
**Status**: CRITICAL - Multiple incomplete implementations blocking functionality

## Executive Summary

This document tracks all incomplete implementations in the codebase that need to be resolved for proper functionality. Each item includes the location, impact, and priority.

---

## Critical Issues (Blocking Core Functionality)

### 1. Some FFmpeg Metadata Fields Not Yet Extracted
**Location**: `internal/infrastructure/persistence/media/repository.go:46-68`

**Status**: PARTIAL - Many fields now implemented, some remain as TODOs

**Implemented Fields** ✅:
- `AudioCodec` - Extracted and stored (migration 000004)
- `AspectRatio` - Calculated from Width/Height
- `ResolutionLabel` - Calculated (1080p/4K/etc)
- `SourceType` - Detected from filename (BluRay/WEB-DL/etc)
- Basic video metadata (codec, bitrate, framerate, dimensions)

**Still TODO** ⚠️:
```go
FileHash:          sql.NullString{},              // TODO: Add Hash field to domain.Media
CodecProfile:      sql.NullString{},              // TODO: Extract from FFmpeg if available
ScanType:          sql.NullString{},              // TODO: Extract from FFmpeg
HdrFormat:         sql.NullString{},              // TODO: Extract from FFmpeg
ColorSpace:        sql.NullString{},              // TODO: Extract from FFmpeg
ColorPrimaries:    sql.NullString{},              // TODO: Extract from FFmpeg
ThumbnailPath:     sql.NullString{},              // TODO: Generate during scan
QualityScore:      sql.NullInt32{}/Int64{},       // TODO: Calculate heuristic
Is3d:              sql.NullBool{},                // TODO: Detect from filename
StereoMode:        sql.NullString{},              // TODO: Detect if 3D
```

**Impact**: MEDIUM - Core metadata working, advanced fields (HDR, color space, 3D) missing
**Priority**: P2 - Nice-to-have features
**Affects**: Movies, TV Episodes (video files)

---

### 2. ~~Missing AudioCodec Column~~ ✅ FIXED
**Status**: COMPLETED - Migration 000004 added audio_codec column

**Resolution**:
- Migration `migrations/postgres/000004_add_audio_codec.up.sql` created
- Column added to both PostgreSQL and SQLite schemas
- Repository now maps AudioCodec field correctly (repository.go:52, 86)
- FFmpeg extraction working properly

**Closed**: 2025-11-12

---

### 3. No-Op Type-Specific Repositories
**Location**: `internal/app/noop/repositories.go`

**Problem**:
- MovieRepository, TVRepository, MusicRepository are all no-ops
- Comment says "Phase 3 - will be real later"
- scan_library.go calls them but they do nothing (lines 264, 321, 380)

**Current Behavior**:
```go
func (r *NoOpMovieRepository) CreateMovie(ctx context.Context, movie *media.Movie) error {
    return nil // Does nothing!
}
```

**Impact**: MEDIUM - Type-specific metadata (year, season, episode, etc.) not saved
**Priority**: P1 - Fix after FFmpeg metadata
**Affects**: Movie years, TV season/episode numbers, music track numbers

---

## Medium Priority Issues

### 4. Hash Field Not Populated
**Location**: `internal/infrastructure/persistence/media/repository.go:46`

**Problem**:
- Scanner computes file hashes (coordinator.go:339-346)
- ScanResult contains hash
- Repository ignores it: `FileHash: sql.NullString{}`

**Impact**: MEDIUM - Duplicate detection doesn't work
**Priority**: P1

---

### 5. Update Method Also Incomplete
**Location**: `internal/infrastructure/persistence/media/repository.go:471-556`

**Problem**:
- Same issues as Create method
- Update hard-codes empty values
- Re-scanning won't update metadata

**Impact**: MEDIUM - Re-scans don't fix incomplete data
**Priority**: P1

---

### 6. No Integration Tests for Scan Flow
**Location**: Missing

**Problem**:
- No end-to-end test from file scan → database
- Unit tests pass but integration fails silently
- Would have caught repository mapping issues

**Impact**: MEDIUM - Can't verify full pipeline works
**Priority**: P2

---

## Low Priority Issues

### 7. ~~Calculated Fields~~ ✅ PARTIALLY FIXED
**Fields**: aspect_ratio, resolution_label, quality_score

**Status**:
- ✅ `aspect_ratio` - IMPLEMENTED via `media.CalculateAspectRatio()` (repository.go:50, 84)
- ✅ `resolution_label` - IMPLEMENTED via `media.CalculateResolutionLabel()` (repository.go:62, 96)
- ⚠️ `quality_score` - Still TODO (repository.go:63, 97)

**Remaining Work**: Quality score heuristic calculation

**Impact**: LOW - Nice-to-have metadata
**Priority**: P3

---

### 8. Source Type Detection Not Implemented
**Field**: source_type

**Status**: ⚠️ TODO - Function is a stub returning empty string

**Location**: `internal/domain/media/metadata.go:60-63`

**Problem**:
- `DetectSourceType()` function exists but only contains a TODO comment
- Always returns `""` for all files
- Repository calls it (lines 61, 95, 499, 534) but database gets NULL values
- No pattern detection implemented

**Missing Detection Patterns**:
- BluRay, WEB-DL, HDTV, DVDRip, BDRip, etc.

**Impact**: MEDIUM - Source quality not tracked, affects duplicate detection
**Priority**: P2 - Nice-to-have for quality assessment

---

### 9. 3D Detection Not Implemented
**Fields**: is_3d, stereo_mode

**Problem**: Should detect from filename patterns:
- 3D, Half-SBS, Half-OU, Full-SBS, etc.

**Impact**: LOW - Only affects 3D content
**Priority**: P3

---

## Prevention Strategy

### Immediate Actions

1. **Add Integration Tests** - Test full scan-to-database flow
2. **Code Review Checklist** - Verify all fields mapped in repository layer
3. **Audit Script** - Automated check for sql.Null{} empty values

### Process Improvements

1. **Definition of Done**:
   - All domain fields mapped to database
   - Integration test proves end-to-end flow
   - No TODO/FIXME in production code paths
   - Documentation updated

2. **Vertical Slice Development**:
   - Complete one feature fully (domain → repo → API → UI)
   - Don't leave layers partially implemented

3. **Test-First for Integration**:
   - Write integration test before implementation
   - Fails until all layers properly connected

4. **Regular Audits**:
   - Weekly grep for TODO/FIXME/no-op
   - Track in this document

---

## Summary by Priority

**P0 (Critical - Blocking):** 0 items ✅
All critical items resolved (AudioCodec, AspectRatio, ResolutionLabel)

**P1 (High Priority):** 3 items
- No-op type-specific repositories (blocks type-specific metadata)
- Hash field not populated (blocks duplicate detection)
- Update method incomplete (blocks re-scan metadata refresh)

**P2 (Medium Priority):** 3 items
- Source type detection stub (nice-to-have quality tracking)
- Missing integration tests (limits confidence in scan pipeline)
- Infrastructure/API test coverage for progress feature

**P3 (Low Priority):** 5 items
- Advanced metadata fields (CodecProfile, ScanType, HDR, Color)
- Thumbnail generation
- Quality score calculation
- 3D detection

**Total Estimated Effort:** ~2-3 days for P1, ~1 week for all remaining

---

**Last Updated:** 2025-11-12
**Status:** Active development - See individual issue sections above for details

---

## ~~Testing Coverage Issues~~ ✅ FIXED

### 10. ~~Watch Progress Feature - Zero Test Coverage~~ ✅ TESTS EXIST
**Location**: `internal/domain/progress/`, `internal/application/progress/`

**Status**: CLAIM WAS FALSE - Comprehensive tests exist

**Test Files Found** ✅:
1. ✅ `internal/domain/progress/entity_test.go` - Domain entity tests
2. ✅ `internal/application/progress/update_progress_test.go` - Update use case tests
3. ✅ `internal/application/progress/get_progress_test.go` - Query use case tests
4. ✅ `internal/application/progress/mark_watched_test.go` - Watched/unwatched tests

**Still Missing** ⚠️:
- `internal/infrastructure/persistence/progress/repository_test.go` - Repository integration tests
- `internal/api/handlers/progress_test.go` - API handler tests

**Correction**: The original claim of "0.0% coverage" was incorrect. Domain and application layers have comprehensive test coverage. Only infrastructure and API layers need additional tests.

**Updated Priority**: P2 - Add integration and API tests when time permits
**Closed**: 2025-11-12 (claim corrected)

