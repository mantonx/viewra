# ADR 019: Watch Progress Tracking Reliability

**Status**: Accepted
**Date**: 2025-11-21
**Deciders**: Development Team
**Context**: Phase 5.7 - Video Player Enhancement & Polish

## Context and Problem Statement

Viewra needs reliable watch progress tracking that:
1. **Accurately tracks playback position** for both direct streams and transcoded videos
2. **Supports confident resume** - users can pause and resume exactly where they left off
3. **Works across stream types** - direct play (MP4/MKV) and progressive HLS transcoding
4. **Handles edge cases** - seeks, browser closes, network interruptions
5. **Performs efficiently** - minimal API calls, batch queries where possible

This ADR documents the **current implementation state**, **identifies critical issues**, and **proposes solutions** to achieve 95%+ reliability for watch progress tracking.

## Decision Drivers

- **User Experience**: Seamless resume experience
- **Data Accuracy**: Sub-second precision for smooth resume
- **Performance**: Minimal server load, batched queries
- **Reliability**: Handle edge cases (seeks, crashes, network issues)
- **Maintainability**: Clean, testable code without closure bugs
- **Multi-user Ready**: Support future multi-user features

## Current Implementation Analysis

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                         Frontend                             │
├─────────────────────────────────────────────────────────────┤
│  VideoPlayer.tsx                                             │
│  ├─ Fetches initial position on playback start              │
│  ├─ useProgressUpdater hook (10s interval updates)          │
│  ├─ Throttled timeupdate (1/sec to reduce renders)          │
│  └─ Handles streamOffsetRef for transcoded seeks            │
├─────────────────────────────────────────────────────────────┤
│  useMediaPlayback.ts                                         │
│  └─ Fetches progress before starting playback               │
├─────────────────────────────────────────────────────────────┤
│  useProgress.ts (React Query hooks)                         │
│  ├─ useMediaProgress(mediaId) - single fetch                │
│  ├─ useBatchProgress(mediaIds) - batch fetch                │
│  ├─ useUpdateProgress() - mutation                          │
│  └─ useProgressUpdater() - periodic updates                 │
└─────────────────────────────────────────────────────────────┘
                              ↓ HTTP API
┌─────────────────────────────────────────────────────────────┐
│                          Backend (Go)                        │
├─────────────────────────────────────────────────────────────┤
│  API Layer (handlers/progress.go)                           │
│  ├─ PUT /api/progress           - Update progress           │
│  ├─ GET /api/progress/{id}      - Get single                │
│  ├─ GET /api/progress/batch     - Batch query (1,2,3)       │
│  ├─ POST /api/progress/mark-watched                         │
│  └─ POST /api/progress/mark-unwatched                       │
├─────────────────────────────────────────────────────────────┤
│  Application Layer (application/progress/*.go)              │
│  ├─ UpdateProgress() - validates, auto-marks 90%            │
│  ├─ GetProgressByMediaIDAndUserID()                         │
│  └─ GetBatchProgressByMediaIDs() - efficient batch          │
├─────────────────────────────────────────────────────────────┤
│  Domain Layer (domain/progress/entity.go)                   │
│  ├─ WatchProgress entity (ProgressSeconds: int)             │
│  ├─ Validation: prevents invalid states                     │
│  ├─ Auto-watched: >=90% completion                          │
│  └─ UpdateProgress(): auto-marks/unmarks watched            │
├─────────────────────────────────────────────────────────────┤
│  Repository Layer (infrastructure/persistence/progress/)    │
│  ├─ SQLite + PostgreSQL support                             │
│  ├─ Batch queries via GetBatchByMediaIDs()                  │
│  └─ Type conversions: int ↔ float64                         │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│  Database (SQLite)                                           │
│  watch_progress table:                                       │
│  ├─ position REAL (float, seconds)                          │
│  ├─ duration REAL (float, seconds)                          │
│  ├─ watched BOOLEAN                                          │
│  ├─ selected_quality TEXT (nullable)     ← NEW              │
│  ├─ selected_audio_track INTEGER (nullable)  ← NEW          │
│  ├─ selected_subtitle_track INTEGER (nullable)  ← NEW       │
│  ├─ UNIQUE(media_id, user_id)                               │
│  └─ Indexes on: media_id, user_id, watched, last_watched    │
└─────────────────────────────────────────────────────────────┘
```

### Playback Preferences Persistence (Implemented)

As of migration `000052_add_playback_preferences`, the `watch_progress` table stores user playback preferences per video:

| Column | Type | Description |
|--------|------|-------------|
| `selected_quality` | TEXT | Quality ID (e.g., "1080p-10m", "original") |
| `selected_audio_track` | INTEGER | FFmpeg stream index of audio track |
| `selected_subtitle_track` | INTEGER | Subtitle track ID (-1 for off) |

**How It Works:**

1. **Saving**: Frontend includes preferences in progress update requests
2. **Restoring**: On playback start, saved preferences are applied automatically
3. **Fallback**: If saved quality/track no longer exists, uses defaults

**API Request Example:**

```json
{
  "media_id": 123,
  "user_id": 1,
  "progress_seconds": 45.3,
  "duration_seconds": 125.7,
  "selected_quality": "1080p-10m",
  "selected_audio_track": 1,
  "selected_subtitle_track": 2
}
```

**Benefits:**

- Users resume with their preferred quality (not auto-recommended)
- Audio language preference persists (e.g., Japanese audio for anime)
- Subtitle preference persists (e.g., English subs on foreign films)

### What Works Well ✅

1. **Backend Architecture**
   - Clean domain-driven design with proper layering
   - Auto-watched threshold (90%) prevents manual marking
   - Batch API (`/api/progress/batch`) eliminates N+1 queries
   - Unique constraint prevents duplicate progress records
   - Proper indexes for query performance

2. **Frontend Progress Updates**
   - Fixed closure bug (using refs instead of captured variables)
   - Throttled timeupdate events (1/sec) reduces re-renders by 90%
   - Periodic updates (10s interval) balance accuracy vs server load
   - React Query integration with cache invalidation

3. **Stream Type Support**
   - **Direct streams**: Simple `video.currentTime` tracking
   - **HLS transcoded**: `streamOffsetRef` handles FFmpeg restarts on seeks

4. **Database Schema**
   - `UNIQUE(media_id, user_id)` prevents duplicates
   - Proper foreign key with `ON DELETE CASCADE`
   - Indexes on common query patterns

### Critical Issues Identified 🔴

#### Issue #1: Type Mismatch - Float vs Int (HIGH SEVERITY)

**Location**: Domain entity vs Database schema
**Impact**: Precision loss causes inaccurate resume positions

**Problem:**
```go
// Domain entity uses integers
type WatchProgress struct {
    ProgressSeconds int  // ❌ Integer - loses fractional seconds
    DurationSeconds int  // ❌ Integer - loses fractional seconds
}

// Database stores floats
CREATE TABLE watch_progress (
    position REAL NOT NULL,  -- ✅ Float - precise
    duration REAL            -- ✅ Float - precise
)

// Repository conversion causes precision loss
func sqliteRowToProgress(row sqlc_sqlite.WatchProgress) *progress.WatchProgress {
    return &progress.WatchProgress{
        ProgressSeconds: int(row.Position),         // ❌ 125.7 → 125
        DurationSeconds: int(common.ParseNullFloat64(row.Duration)),
    }
}
```

**Example Impact:**
- Video duration: **125.7 seconds**
- User pauses at: **45.3 seconds**
- Database stores: `45.3` (correct)
- Domain reads: `45` (lost 0.3 seconds)
- API returns: `45` (frontend sees wrong value)
- User resumes: **45 seconds** instead of **45.3 seconds**

**Why This Matters:**
- 0.3 seconds might seem small, but compounds over time
- User experience: "It feels slightly off when I resume"
- For precise scenes (dialogue cuts, action sequences), accuracy matters
- Inconsistent with industry standards (float timestamps are common)

#### Issue #2: No Immediate Seek Progress Save (MEDIUM SEVERITY)

**Location**: Frontend VideoPlayer seek handling
**Impact**: Progress loss if user closes video after seeking

**Problem:**
```typescript
// Current behavior:
const handleSeek = (time: number) => {
  video.currentTime = time  // ✅ Seeks correctly
  // ❌ No immediate progress save - waits for 10s interval
}

// useProgressUpdater updates every 10 seconds
intervalId = setInterval(() => {
  updateProgress.mutate({ ... })
}, 10000)  // 10 second delay
```

**Example Impact:**
- User watches from **00:05:00**
- User seeks to **00:15:00**
- User closes video after **3 seconds**
- Progress saved: **00:05:00** (last interval update)
- Next resume: Starts at **00:05:00** instead of **00:15:00**

**Frequency**: Common - users often seek then immediately close

#### Issue #3: useProgressUpdater Uses Mutable Closures (LOW SEVERITY)

**Location**: [web/src/lib/hooks/useProgress.ts:171-227](web/src/lib/hooks/useProgress.ts#L171-L227)
**Impact**: Hard to maintain, fragile lifecycle management

**Problem:**
```typescript
export function useProgressUpdater(...) {
  let intervalId: ReturnType<typeof setInterval> | null = null  // ❌ Mutable
  let lastProgressSeconds = 0  // ❌ Outside React lifecycle

  const startTracking = (currentTimeSeconds: number) => {
    lastProgressSeconds = Math.floor(currentTimeSeconds)
    // ❌ Closure captures these variables
    intervalId = setInterval(() => { ... }, updateIntervalMs)
  }
}
```

**Why This Is Problematic:**
- Variables outside React's lifecycle management
- No automatic cleanup on unmount (relies on manual cleanup)
- Hard to test (relies on closure behavior)
- Not following React best practices (should use `useRef` + `useCallback`)

**Current Status**: Works correctly in VideoPlayer because it uses refs properly, but the hook itself is fragile.

#### Issue #4: No Browser Close Progress Save (LOW SEVERITY)

**Location**: Missing `beforeunload` handler
**Impact**: Progress loss if user closes browser/tab

**Problem:**
- User watches to **00:45:00**
- Last interval update: **00:42:00** (3 seconds ago)
- User closes browser tab
- Progress saved: **00:42:00**
- Resume position: **3 seconds behind**

**Frequency**: Moderate - users often close tabs/windows

### Performance Analysis

**Current Load (per user watching):**
- Progress updates: 1 request every 10 seconds = **6 req/min**
- During 2-hour movie: **720 requests total**
- Multiple users: Scales linearly (no caching possible for writes)

**Batch Loading (library browsing):**
- ✅ Good: Uses `GET /api/progress/batch?media_ids=1,2,3`
- ✅ Reduces N+1 queries from 50 requests → 1 request
- ✅ Efficient for displaying progress bars on cards

## Decision

### Solution 1: Fix Type Mismatch - Use Float64 Throughout

**Change Domain Entity to Float64:**

```go
// internal/domain/progress/entity.go
type WatchProgress struct {
    ID              int64
    UserID          int64
    MediaID         int64
    ProgressSeconds float64  // ✅ Changed from int
    DurationSeconds float64  // ✅ Changed from int
    IsWatched       bool
    LastWatchedAt   time.Time
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// Update validation to handle floats
func (wp *WatchProgress) IsValid() error {
    if wp.MediaID <= 0 {
        return ErrInvalidMediaID
    }
    if wp.ProgressSeconds < 0 {
        return ErrInvalidProgress
    }
    if wp.DurationSeconds < 0 {
        return ErrInvalidDuration
    }
    if wp.ProgressSeconds > wp.DurationSeconds {
        return ErrProgressExceedsDuration
    }
    return nil
}
```

**Update Repository to Remove Conversion:**

```go
// internal/infrastructure/persistence/progress/repository.go
func sqliteRowToProgress(row sqlc_sqlite.WatchProgress) *progress.WatchProgress {
    return &progress.WatchProgress{
        ID:              row.ID,
        UserID:          common.ParseNullInt64(row.UserID),
        MediaID:         row.MediaID,
        ProgressSeconds: row.Position,  // ✅ Direct float64 assignment
        DurationSeconds: common.ParseNullFloat64(row.Duration),  // ✅ Float64
        IsWatched:       common.ParseNullBool(row.Watched),
        LastWatchedAt:   common.ParseNullTime(row.LastWatched),
        CreatedAt:       common.ParseNullTime(row.CreatedAt),
        UpdatedAt:       common.ParseNullTime(row.UpdatedAt),
    }
}

func (r *Repository) Create(ctx context.Context, prog *progress.WatchProgress) error {
    // ... validation ...
    if r.dbType == "sqlite" {
        result, err := r.sqliteQuerier.CreateWatchProgress(ctx, sqlc_sqlite.CreateWatchProgressParams{
            MediaID:     prog.MediaID,
            UserID:      common.NullInt64(prog.UserID),
            Position:    prog.ProgressSeconds,  // ✅ Direct float64 assignment
            Duration:    common.NullFloat64(prog.DurationSeconds),  // ✅ Float64
            Watched:     common.NullBool(prog.IsWatched),
            LastWatched: common.NullTime(prog.LastWatchedAt),
            CreatedAt:   common.NullTime(prog.CreatedAt),
            UpdatedAt:   common.NullTime(prog.UpdatedAt),
        })
        // ...
    }
}
```

**Update DTOs:**

```go
// internal/application/progress/dto.go
type UpdateProgressRequest struct {
    MediaID         int64   `json:"media_id"`
    UserID          int64   `json:"user_id"`
    ProgressSeconds float64 `json:"progress_seconds"`  // ✅ Changed from int
    DurationSeconds float64 `json:"duration_seconds"`  // ✅ Changed from int
}

type WatchProgressResponse struct {
    ID                 int64     `json:"id"`
    MediaID            int64     `json:"media_id"`
    UserID             int64     `json:"user_id"`
    ProgressSeconds    float64   `json:"progress_seconds"`  // ✅ Changed from int
    DurationSeconds    float64   `json:"duration_seconds"`  // ✅ Changed from int
    ProgressPercentage float64   `json:"progress_percentage"`
    IsWatched          bool      `json:"is_watched"`
    LastWatchedAt      time.Time `json:"last_watched_at"`
    CreatedAt          time.Time `json:"created_at"`
    UpdatedAt          time.Time `json:"updated_at"`
}
```

**Frontend Already Handles Floats:**

TypeScript/JavaScript natively uses `number` type which is float64, so no frontend changes needed.

**Benefits:**
- ✅ Eliminates precision loss (0.7 seconds → 700ms matters)
- ✅ Matches database schema (no conversions)
- ✅ Industry standard for video timestamps
- ✅ Smooth resume experience
- ✅ Frontend already compatible

**Risks:**
- ⚠️ Requires backend changes across 3 layers
- ⚠️ Need to update all existing tests
- ⚠️ API contract change (minor version bump needed)

### Solution 2: Add Immediate Progress Save on Seek

**Add to VideoPlayer seek handler:**

```typescript
// web/src/components/media/VideoPlayer/VideoPlayer.tsx
const handleSeek = (time: number) => {
  const video = videoRef.current
  const hls = hlsRef.current
  if (!video) return

  // ✅ ADD: Immediate progress save before seek
  if (progressUpdaterRef.current && videoDuration > 0) {
    progressUpdaterRef.current.updateCurrentTime(time)
    // Send immediate update (don't wait for 10s interval)
    progressUpdaterRef.current.immediateUpdate()
  }

  // ... existing seek logic (large seek detection, manifest reload, etc.) ...
}
```

**Add immediateUpdate method to useProgressUpdater:**

```typescript
// web/src/lib/hooks/useProgress.ts
export function useProgressUpdater(...) {
  // ... existing code ...

  const immediateUpdate = useCallback(() => {
    if (currentTimeRef.current > 0) {
      updateProgress.mutate({
        media_id: mediaId,
        user_id: 1,
        progress_seconds: currentTimeRef.current,
        duration_seconds: Math.floor(durationSeconds),
      })
    }
  }, [mediaId, durationSeconds, updateProgress])

  return {
    startTracking,
    updateCurrentTime,
    stopTracking,
    immediateUpdate,  // ✅ New method
  }
}
```

**Benefits:**
- ✅ No progress loss on seek + immediate close
- ✅ Minimal code change (one method, one call site)
- ✅ User-initiated actions get immediate save
- ✅ Matches user expectations

**Risks:**
- ⚠️ Increases API calls (1 extra call per seek)
- ⚠️ Acceptable tradeoff for data reliability

### Solution 3: Refactor useProgressUpdater with Proper Hooks

**Replace closure-based implementation with React hooks:**

```typescript
// web/src/lib/hooks/useProgress.ts
export function useProgressUpdater(
  mediaId: number,
  durationSeconds: number,
  updateIntervalMs = 10000
) {
  const updateProgress = useUpdateProgress()
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const currentTimeRef = useRef<number>(0)

  // ✅ Use useCallback for stable function references
  const startTracking = useCallback((currentTimeSeconds: number) => {
    currentTimeRef.current = Math.floor(currentTimeSeconds)

    // Clear existing interval
    if (intervalRef.current) {
      clearInterval(intervalRef.current)
    }

    // Set up periodic updates
    intervalRef.current = setInterval(() => {
      if (currentTimeRef.current > 0) {
        updateProgress.mutate({
          media_id: mediaId,
          user_id: 1,
          progress_seconds: currentTimeRef.current,
          duration_seconds: Math.floor(durationSeconds),
        })
      }
    }, updateIntervalMs)
  }, [mediaId, durationSeconds, updateIntervalMs, updateProgress])

  const updateCurrentTime = useCallback((currentTimeSeconds: number) => {
    currentTimeRef.current = Math.floor(currentTimeSeconds)
  }, [])

  const immediateUpdate = useCallback(() => {
    if (currentTimeRef.current > 0) {
      updateProgress.mutate({
        media_id: mediaId,
        user_id: 1,
        progress_seconds: currentTimeRef.current,
        duration_seconds: Math.floor(durationSeconds),
      })
    }
  }, [mediaId, durationSeconds, updateProgress])

  const stopTracking = useCallback(() => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current)
      intervalRef.current = null
    }

    // Send final update
    if (currentTimeRef.current > 0) {
      updateProgress.mutate({
        media_id: mediaId,
        user_id: 1,
        progress_seconds: currentTimeRef.current,
        duration_seconds: Math.floor(durationSeconds),
      })
    }
  }, [mediaId, durationSeconds, updateProgress])

  // ✅ Automatic cleanup on unmount
  useEffect(() => {
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
      }
    }
  }, [])

  return {
    startTracking,
    updateCurrentTime,
    immediateUpdate,
    stopTracking,
  }
}
```

**Benefits:**
- ✅ Proper React lifecycle management
- ✅ Automatic cleanup on unmount
- ✅ Stable function references (prevents re-renders)
- ✅ Easier to test and maintain
- ✅ Follows React best practices

**Risks:**
- ⚠️ None - this is the correct pattern

### Solution 4: Add Browser Close Progress Save (Optional)

**Add beforeunload handler to VideoPlayer:**

```typescript
// web/src/components/media/VideoPlayer/VideoPlayer.tsx
useEffect(() => {
  const handleBeforeUnload = () => {
    if (currentTime > 0 && videoDuration > 0) {
      // Use sendBeacon for guaranteed delivery during page unload
      const data = JSON.stringify({
        media_id: mediaId,
        user_id: 1,
        progress_seconds: Math.floor(currentTime),
        duration_seconds: Math.floor(videoDuration),
      })

      // sendBeacon is specifically designed for this use case
      navigator.sendBeacon(`${API_BASE_URL}/api/progress`, data)
    }
  }

  window.addEventListener('beforeunload', handleBeforeUnload)
  return () => window.removeEventListener('beforeunload', handleBeforeUnload)
}, [mediaId, currentTime, videoDuration])
```

**Backend needs to accept sendBeacon requests:**

```go
// internal/api/handlers/progress.go
func (h *ProgressHandler) UpdateProgress(c *gin.Context) {
    var req progress.UpdateProgressRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{
            Error: "Invalid request body",
        })
        return
    }

    // Default to user_id = 1 for now (single-user mode)
    if req.UserID == 0 {
        req.UserID = 1
    }

    response, err := progress.UpdateProgress(c.Request.Context(), h.repo, &req)
    if err != nil {
        // ... error handling ...
    }

    c.JSON(http.StatusOK, response)
}
```

**Benefits:**
- ✅ Captures progress on browser close
- ✅ No data loss on unexpected tab close
- ✅ `sendBeacon` is designed for this (queued even if page unloads)

**Risks:**
- ⚠️ `sendBeacon` has limited browser support (but good coverage)
- ⚠️ POST might need CORS adjustment
- ⚠️ Low priority - periodic updates already handle most cases

## Implementation Plan

### Phase 1: Critical Fixes (Week 1)

**Priority**: HIGH
**Estimated Effort**: 4-6 hours

1. **Fix Type Mismatch** (Solution 1)
   - [ ] Update domain entity (`ProgressSeconds`, `DurationSeconds` → `float64`)
   - [ ] Update application DTOs (requests/responses)
   - [ ] Remove float→int conversions in repository
   - [ ] Update all unit tests
   - [ ] Frontend: No changes needed (already uses `number`)
   - [ ] Regenerate OpenAPI client types (`make swagger-gen`)

2. **Add Immediate Seek Save** (Solution 2)
   - [ ] Add `immediateUpdate()` to `useProgressUpdater`
   - [ ] Call `immediateUpdate()` in `handleSeek()`
   - [ ] Test: Seek → close → verify progress saved

### Phase 2: Refactoring (Week 2)

**Priority**: MEDIUM
**Estimated Effort**: 2-3 hours

3. **Refactor useProgressUpdater** (Solution 3)
   - [ ] Replace closures with `useRef` + `useCallback`
   - [ ] Add `useEffect` cleanup
   - [ ] Update tests
   - [ ] Verify no regressions in VideoPlayer

### Phase 3: Enhancement (Week 3 - Optional)

**Priority**: LOW
**Estimated Effort**: 2-3 hours

4. **Add Browser Close Save** (Solution 4)
   - [ ] Add `beforeunload` event listener
   - [ ] Use `navigator.sendBeacon()`
   - [ ] Test cross-browser compatibility
   - [ ] Verify CORS allows sendBeacon

## Testing Strategy

### Unit Tests

```go
// Backend: Test float64 precision
func TestWatchProgress_FloatPrecision(t *testing.T) {
    wp := &progress.WatchProgress{
        MediaID:         1,
        UserID:          1,
        ProgressSeconds: 45.3,  // Float with decimal
        DurationSeconds: 125.7,
    }

    // Save to database
    err := repo.Create(ctx, wp)
    require.NoError(t, err)

    // Retrieve from database
    retrieved, err := repo.GetByMediaIDAndUserID(ctx, 1, 1)
    require.NoError(t, err)

    // Verify precision preserved
    assert.Equal(t, 45.3, retrieved.ProgressSeconds)
    assert.Equal(t, 125.7, retrieved.DurationSeconds)
}
```

### Integration Tests

```typescript
// Frontend: Test seek + immediate save
describe('VideoPlayer seek behavior', () => {
  it('saves progress immediately after seek', async () => {
    const { videoPlayer, mockProgressApi } = renderVideoPlayer()

    // Seek to 15:00
    await videoPlayer.seek(900) // 15 minutes = 900 seconds

    // Verify immediate API call (don't wait for interval)
    expect(mockProgressApi.updateProgress).toHaveBeenCalledWith({
      media_id: 1,
      user_id: 1,
      progress_seconds: 900,
      duration_seconds: 7200,
    })
    expect(mockProgressApi.updateProgress).toHaveBeenCalledTimes(1)
  })
})
```

### Manual Testing Scenarios

1. **Resume Accuracy**
   - Pause at 1:23.7 (83.7 seconds)
   - Close player
   - Reopen player
   - **Expected**: Resume at 1:23 or 1:24 (within 1 second)
   - **With float fix**: Resume at exactly 1:23.7

2. **Seek + Close**
   - Watch from 5:00
   - Seek to 15:00
   - Close immediately (within 1 second)
   - Reopen player
   - **Expected**: Resume at ~15:00 (not 5:00)

3. **Auto-Watched Threshold**
   - Watch to 91% completion (e.g., 2:44 of 3:00 video)
   - Close player
   - Check database: `watched = TRUE`
   - **Expected**: Marked as watched automatically

4. **Browser Close**
   - Watch to 45:00
   - Wait 3 seconds after last interval update
   - Close browser tab (Ctrl+W)
   - Reopen and check progress
   - **Expected**: Progress within 3 seconds of close time

5. **Transcoded Stream Large Seek**
   - Start HLS transcoded video
   - Seek forward 5 minutes (triggers FFmpeg restart)
   - Verify `streamOffsetRef` adjusted correctly
   - Close and reopen
   - **Expected**: Resume at seek position (not offset position)

## Consequences

### Positive

- ✅ **Accurate Resume**: Float64 eliminates precision loss
- ✅ **No Data Loss**: Immediate seek save prevents progress loss
- ✅ **Maintainable**: Hook refactor follows React best practices
- ✅ **User Trust**: Reliable progress tracking builds confidence
- ✅ **Future-Proof**: Clean architecture for multi-user support
- ✅ **Performance**: Batch queries already efficient

### Negative

- ⚠️ **API Change**: Float64 change requires API version bump
- ⚠️ **Migration**: Need to update all tests
- ⚠️ **Increased Requests**: Seek save adds ~1-5 requests per session
- ⚠️ **Complexity**: `streamOffsetRef` logic remains complex (but necessary)

### Neutral

- ℹ️ Database schema unchanged (already stores floats)
- ℹ️ Frontend unchanged (already uses `number`)
- ℹ️ Existing progress data compatible (no migration needed)

## Related Decisions

- **ADR 005**: On-demand Transcoding Strategy (affects progress tracking for HLS)
- **ADR 016**: Seek Position Transcoding (FFmpeg restart handling)
- **ADR 015**: Player Enhancement Strategy (progress bar UI)
- **Phase 5.7**: Video Player Enhancement & Polish

## References

- [internal/domain/progress/entity.go](internal/domain/progress/entity.go) - Domain entity
- [internal/infrastructure/persistence/progress/repository.go](internal/infrastructure/persistence/progress/repository.go) - Repository with type conversions
- [web/src/lib/hooks/useProgress.ts](web/src/lib/hooks/useProgress.ts) - Frontend hooks
- [web/src/components/media/VideoPlayer/VideoPlayer.tsx](web/src/components/media/VideoPlayer/VideoPlayer.tsx) - Player implementation
- [internal/infrastructure/database/queries/sqlite/watch_progress.sql](internal/infrastructure/database/queries/sqlite/watch_progress.sql) - SQL queries

## Appendix: Current Reliability Assessment

| Aspect | Current | After Fixes | Notes |
|--------|---------|-------------|-------|
| **Direct Stream Tracking** | 9/10 | 9.5/10 | Float fix improves precision |
| **HLS Transcoded Tracking** | 8/10 | 9/10 | streamOffsetRef working well |
| **Seek Reliability** | 6/10 | 9/10 | Immediate save prevents loss |
| **Resume Accuracy** | 7/10 | 9.5/10 | Float precision critical |
| **Auto-Watched (90%)** | 9/10 | 9/10 | Already solid |
| **Browser Close** | 7/10 | 8/10 | Periodic updates + optional beacon |
| **Batch Query Performance** | 9/10 | 9/10 | Already optimized |
| **Code Maintainability** | 7/10 | 9/10 | Hook refactor improves |
| **Overall Reliability** | **7.5/10** | **9/10** | Ready for production |

**Confidence Level**: HIGH - Clear path from 75% → 90% reliability
