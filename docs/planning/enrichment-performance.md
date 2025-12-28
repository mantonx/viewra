# Enrichment Pipeline Performance Optimization Plan

Generated: 2024-12-21  
Last Updated: 2024-12-27

## Table of Contents

1. [Overview](#overview)
2. [Priority System](#priority-system)
3. [Phase 0: Bug Fixes (Prerequisite)](#phase-0-bug-fixes-prerequisite) - **COMPLETED**
4. [Phase 1: Quick Wins](#phase-1-quick-wins-priority-high) - Pipeline cache, rate limiter, worker tuning
5. [Phase 2: Batch Operations](#phase-2-batch-operations-priority-high) - Batch enqueue, external ID prefetch
6. [Phase 3: Entity Deduplication](#phase-3-entity-deduplication-priority-high) - LRU cache for TV/Music
7. [Phase 4: Priority & Resilience](#phase-4-priority--resilience-priority-medium) - Interactive priority, circuit breaker
8. [Phase 5: UX Improvements](#phase-5-ux-improvements-priority-medium) - Unified UI, badges, dialogs
9. [Frontend Changes](#frontend-changes)
10. [Phase Summary](#phase-summary-updated)
11. [Complete File Change Summary](#complete-file-change-summary)

---

## Overview

Optimize enrichment pipeline performance for 100K+ item libraries. Current enrichment time is ~50+ hours, primarily bottlenecked by:

1. **Per-item DB operations:** ~300K individual queries during enrichment
2. **Suboptimal rate limit utilization:** Only using ~50% of allowed TMDB throughput
3. **No deduplication:** TV show metadata fetched once per episode instead of once per show
4. **No priority system:** New releases wait behind entire backlog

**Target:** Reduce enrichment time by 60-70% through batching, caching, and deduplication.

### Expected Results

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| 100K library enrichment time | ~50h | ~15-20h | 60-70% |
| DB ops during scan enqueue | 100K | ~200 | 500x |
| DB ops during enrichment | ~2M | ~400K | 5x |
| TMDB throughput | ~2 req/sec | ~4 req/sec | 2x |
| TV show API calls (500 eps) | 500 | 1 | 500x |
| Album API calls (200 tracks) | 200 | 1 | 200x |

### Total Effort

| Phase | Description | Effort | Status |
|-------|-------------|--------|--------|
| 0 | Sort title bug fix (prerequisite) | ~1h | **COMPLETED** |
| 1 | Pipeline cache, rate limiter, worker tuning | 4-6h | **COMPLETED** |
| 2 | Batch enqueue, external ID prefetch, re-prioritization | 10-14h | **COMPLETED** |
| 3 | Entity cache (TV/Music deduplication) | 8-12h | **COMPLETED** |
| 4 | Priority boost, circuit breaker | 6-8h | Pending |
| 5 | UX improvements (unified UI, badges, dialogs) | 10-14h | Pending |
| **Total Remaining** | | **16-22h** | |

---

## Priority System

### Priority Tiers

| Priority | Meaning |
|----------|---------|
| 1000 | User is viewing this item (interactive) |
| 200 | Released/added today |
| 150 | Released/added this week |
| 100 | Released/added this month |
| 50 | Released/added this year |
| 0 | Older |

### Release Date Source (in order of preference)

1. Actual release/air date (from NFO or TMDB, after enrichment)
2. Parsed year from filename (assume Jan 1 of that year)
3. File creation time (fallback)
4. File modified time (final fallback)

### Implementation

```go
func calculatePriority(releaseDate time.Time) int {
    age := time.Since(releaseDate)
    
    switch {
    case age < 24*time.Hour:       return 200  // Today
    case age < 7*24*time.Hour:     return 150  // This week
    case age < 30*24*time.Hour:    return 100  // This month
    case age < 365*24*time.Hour:   return 50   // This year
    default:                       return 0    // Older
    }
}
```

### Re-prioritization

After NFO or TMDB enrichment discovers actual release date, update priority for remaining stages if the tier changes.

---

## Phase 0: Bug Fixes (Prerequisite) - COMPLETED

> **Status:** ✅ **COMPLETED** (2024-12-27)
>
> All tasks in this phase have been implemented:
> - `metadata_applier.go:97` has empty string check: `*metadata.SortTitle != ""`
> - SQL queries use `COALESCE(NULLIF(m.sort_title, ''), med.title)`
> - `normalize_sort_titles.go` supports both SQLite and PostgreSQL

<details>
<summary>Original Phase 0 Details (click to expand)</summary>

**Goal:** Fix sort title handling that causes incorrect alphabetical ordering in movie and TV show lists.

**Estimated Effort:** 1-2 hours

### Problem

Movies like "(500) Days of Summer" and "3 Idiots" appear in wrong positions in the UI. The root cause is twofold:

1. **metadata_applier.go** overwrites `sort_title` with empty string when enricher returns an empty pointer
2. **SQL queries** use `COALESCE(sort_title, title)` which doesn't handle empty strings (only NULL)

### Tasks

#### 0.1 Fix metadata_applier.go Empty String Check

- **Files:** `internal/application/enrichment/pipeline/metadata_applier.go`
- **Issue:** Lines 97-99 (movies) and 269-271 (TV shows) accept empty `SortTitle` pointers, overwriting valid values with empty strings
- **Current (buggy):**
  ```go
  if metadata.SortTitle != nil {
      movie.SortTitle = *metadata.SortTitle
      updated = true
  }
  ```
- **Fix:** Add empty string check (like albums at line 485):
  ```go
  if metadata.SortTitle != nil && *metadata.SortTitle != "" {
      movie.SortTitle = *metadata.SortTitle
      updated = true
  }
  ```
- **Effort:** 15 minutes

#### 0.2 Fix SQL Queries to Handle Empty Strings

- **Files:**
  - `internal/infrastructure/database/queries/sqlite/movies.sql`
  - `internal/infrastructure/database/queries/sqlite/tv_shows.sql`
  - `internal/infrastructure/database/queries/postgres/movies.sql`
  - `internal/infrastructure/database/queries/postgres/tv_shows.sql`
- **Issue:** `COALESCE(m.sort_title, med.title)` doesn't handle empty string `''`
- **Fix:** Use `COALESCE(NULLIF(m.sort_title, ''), med.title)` or `COALESCE(NULLIF(s.sort_title, ''), s.title)`
- **Effort:** 30 minutes

#### 0.3 Regenerate SQLC

- **Command:** `make sqlc-gen`
- **Effort:** 5 minutes

#### 0.4 Enhance Normalization Script for PostgreSQL

- **File:** `scripts/normalize_sort_titles.go`
- **Issue:** Script only supports SQLite, users on PostgreSQL need manual intervention
- **Fix:** Add PostgreSQL driver support with database type detection or flag
- **Effort:** 20 minutes

#### 0.5 Run Normalization Script

- **Purpose:** Recalculate all `sort_title` values from titles using `NormalizeSortTitle()`
- **Command:** `go run scripts/normalize_sort_titles.go`
- **Effort:** 5 minutes

#### 0.6 Verify Fix

- Query database to confirm movies like "(500) Days of Summer" have sort titles like "500 days of summer"
- Test UI sorting to ensure alphabetical order is correct
- **Effort:** 15 minutes

</details>

---

## Phase 1: Quick Wins (Priority: HIGH) - COMPLETED

> **Status:** ✅ **COMPLETED** (2024-12-27)
>
> All tasks implemented:
> - `pipeline_cache.go` - In-memory cache with 5-minute TTL for pipeline stage lookups
> - Rate limiter burst size matches concurrency for better throughput
> - Default remote stage concurrency increased from 2 to 4 workers
> - Tests updated and new cache tests added

**Goal:** Reduce DB operations and improve rate limit utilization with minimal changes.

**Estimated Effort:** 4-6 hours

### Tasks

#### 1.1 Pipeline Configuration Cache ✅

- **Files:** 
  - `internal/application/enrichment/pipeline/pipeline_cache.go` (new)
  - `internal/application/enrichment/pipeline/manager.go`
  - `internal/application/enrichment/pipeline/job_processor.go`
- **Issue:** Every job completion triggers 2 DB queries (`GetStageByName`, `GetNextStage`)
- **Fix:** In-memory cache with 5-minute TTL, invalidated on config API changes
- **Impact:** Eliminate ~200K queries for 100K items
- **Effort:** 2 hours

#### 1.2 Centralized Stage Rate Limiter ✅

- **Files:**
  - `internal/application/enrichment/pipeline/worker_pool.go`
  - `internal/application/enrichment/pipeline/deps.go`
- **Issue:** Rate limiter is per-worker, not per-stage. Polling overhead wastes capacity.
- **Fix:** Single shared `rate.Limiter` across all workers in a pool with burst = concurrency
- **Impact:** More predictable rate limiting, better throughput
- **Effort:** 1 hour

#### 1.3 Tuned Worker Count ✅

- **Files:**
  - `internal/application/enrichment/pipeline/deps.go`
- **Issue:** 2 workers for TMDB (4 req/sec limit) achieves only ~2 req/sec actual
- **Fix:** Calculate optimal workers: `ceil(rate_limit * avg_request_duration)`
  - TMDB: 4 workers (up from 2)
  - MusicBrainz: 2 workers (unchanged, 1 req/sec limit)
- **Impact:** ~2x throughput for rate-limited stages
- **Effort:** 1 hour

---

## Phase 2: Batch Operations (Priority: HIGH) - COMPLETED

> **Status:** ✅ **COMPLETED** (2024-12-27)
>
> All tasks implemented and wired:
> - `enqueue_buffer.go` - Channel-based async buffer with batch flush (500 items or 2s)
> - `EnqueueBuffer` created in `services.go`, started/stopped in `container.go`
> - `EnqueueBuffer` passed to `ScanLibraryUseCase` (replaces direct Manager usage)
> - `EnqueueBatch` - Repository method for transactional batch inserts
> - `GetByMediaBatch` - Batch external ID prefetch called in `worker_pool.go`
> - `priority.go` - Priority calculation from release dates/file timestamps
> - `UpdatePriorityByMedia` - Called in `response_applier.go` after metadata applied
> - Priority passed through entire scan flow (movie.go, tv.go, music.go, common.go)
> - `EnrichmentEnqueuer` interface updated to accept priority parameter

**Goal:** Reduce DB round-trips by batching related operations.

**Estimated Effort:** 10-14 hours

### Tasks

#### 2.1 Batch Enrichment Enqueue ✅

- **Files:**
  - `internal/application/enrichment/pipeline/enqueue_buffer.go` (new)
  - `internal/application/enrichment/pipeline/manager.go`
  - `internal/application/library/scan/media/common.go`
  - `internal/infrastructure/database/queries/postgres/enrichment_queue.sql`
  - `internal/infrastructure/database/queries/sqlite/enrichment_queue.sql`
  - `internal/infrastructure/persistence/enrichment/queue_repository.go`
- **Issue:** After scan, each media item triggers individual goroutine → INSERT
- **Fix:** Channel-based async writer with batch flush (500 items or 2 seconds)
- **Priority:** Calculated from parsed year or file timestamps at enqueue time
- **Impact:** ~100x fewer DB round-trips during scan enqueue
- **Effort:** 6 hours

#### 2.2 External ID Batch Prefetch ✅

- **Files:**
  - `internal/application/enrichment/pipeline/worker_pool.go`
  - `internal/application/enrichment/pipeline/job_processor.go`
  - `internal/infrastructure/database/queries/postgres/external_ids.sql`
  - `internal/infrastructure/database/queries/sqlite/external_ids.sql`
  - `internal/infrastructure/persistence/enrichment/external_id_repository.go`
- **Issue:** Each job fetches external IDs individually (1 SELECT per job)
- **Fix:** Batch fetch when claiming jobs: `GetByMediaBatch(mediaIDs []int64)`
- **Impact:** ~5x fewer DB queries during enrichment
- **Effort:** 4 hours

#### 2.3 Post-Enrichment Re-prioritization ✅

- **Files:**
  - `internal/application/enrichment/pipeline/job_processor.go`
  - `internal/infrastructure/persistence/enrichment/queue_repository.go`
  - `internal/infrastructure/database/queries/postgres/enrichment_queue.sql`
  - `internal/infrastructure/database/queries/sqlite/enrichment_queue.sql`
- **Issue:** Initial priority is a guess based on filename/file age
- **Fix:** After enrichment discovers actual release date, update priority for remaining stages if tier changes
- **SQL:** `UpdatePriorityByMedia(mediaID, mediaType, newPriority)`
- **Effort:** 2 hours

---

## Phase 3: Entity Deduplication (Priority: HIGH) - COMPLETED

> **Status:** ✅ **COMPLETED** (2024-12-27)
>
> All tasks implemented:
> - `entity_cache.go` - LRU cache with 10,000 entries per entity type (shows, seasons, albums, artists)
> - `EntityCache` integrated into Manager and WorkerPool
> - `RequestBuilder` updated to check cache before DB lookups, populate cache on miss
> - Cache used for TV seasons (parent show lookup) and all parent entity types

**Goal:** Cache parent entity metadata to avoid redundant API calls.

**Estimated Effort:** 8-12 hours

### Tasks

#### 3.1 Entity Cache with LRU Eviction ✅

- **Files:**
  - `internal/application/enrichment/pipeline/entity_cache.go` (new)
  - `internal/application/enrichment/pipeline/request_builder.go`
  - `internal/application/enrichment/pipeline/manager.go`
- **Issue:** 
  - 500 episodes of "Breaking Bad" = 500 TMDB show lookups
  - 200 tracks from same album = 200 MusicBrainz album lookups
- **Fix:** LRU cache for shows, seasons, albums, and artists
  - Max entries: 10,000 per entity type
  - Eviction: LRU (least recently used)
  - Keys: entity ID (showID, albumID, artistID)
  - Values: title, year, directory, external IDs
- **Impact:**
  - 500-episode show: 500 lookups → 1 lookup (500x reduction)
  - 200-track album: 200 lookups → 1 lookup (200x reduction)
- **Effort:** 6 hours

#### 3.2 Request Builder Integration ✅

- **Files:**
  - `internal/application/enrichment/pipeline/request_builder.go`
- **Issue:** Request builder fetches parent entity data for every episode/track
- **Fix:** 
  - Check cache before DB lookup
  - On cache miss, fetch and populate cache
  - Include parent external IDs in child requests (e.g., show's TMDB ID for episodes)
- **Entities to cache:**
  - TV Shows (for episode enrichment)
  - TV Seasons (for episode enrichment)
  - Music Albums (for track enrichment)
  - Music Artists (for track/album enrichment)
- **Effort:** 4 hours

---

## Phase 4: Priority & Resilience (Priority: MEDIUM)

**Goal:** Improve UX for interactive use and handle API outages gracefully.

**Estimated Effort:** 6-8 hours

### Tasks

#### 4.1 Interactive Priority Boost

- **Files:**
  - `internal/api/handlers/enrichment.go`
  - `internal/api/routes/enrichment.go`
  - `internal/infrastructure/database/queries/postgres/enrichment_queue.sql`
  - `internal/infrastructure/database/queries/sqlite/enrichment_queue.sql`
  - `internal/infrastructure/persistence/enrichment/queue_repository.go`
  - `web/src/api/enrichment.ts`
  - `web/src/hooks/useAutoEnrich.ts` (new)
- **Issue:** User views unenriched item, must wait behind 100K queue
- **Fix:**
  - API endpoint: `POST /api/enrichment/prioritize`
  - Sets priority = 1000 (higher than any automatic priority)
  - Frontend: Auto-call on media detail view if not enriched
- **Impact:** Immediate enrichment for items user is viewing
- **Effort:** 3 hours

#### 4.2 Circuit Breaker with Status UI

- **Files:**
  - `internal/application/enrichment/pipeline/circuit_breaker.go` (new)
  - `internal/application/enrichment/pipeline/worker_pool.go`
  - `internal/api/handlers/enrichment.go`
  - `web/src/components/enrichment/StageStatus.tsx` (new)
- **Issue:** If TMDB is down, workers keep retrying all jobs
- **Fix:**
  - Per-stage circuit breaker
  - States: closed → open → half-open → closed
  - Threshold: 10 consecutive failures → open
  - Reset timeout: 5 minutes (exponential: 5m, 10m, 20m, max 1h)
  - Auto-retry with backoff
- **API:** `GET /api/enrichment/stages` returns per-stage health
- **SSE:** Publish `enrichment.circuit_state` events on state change
- **UI:** Show stage status with circuit state and retry countdown
- **Impact:** Better resource usage, user visibility into API outages
- **Effort:** 4 hours

---

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Priority system | 5 tiers (0/50/100/150/200) + interactive (1000) | Simple, covers common cases |
| Release date source | NFO/TMDB → filename year → file created → file modified | Best available data |
| Batch enqueue approach | Channel-based async writer | Backpressure handling, cleaner separation |
| Cache eviction | LRU with 10K entry limit | Memory-efficient, natural access pattern |
| Circuit breaker scope | Per-stage | Different endpoints may have different availability |

---

## Frontend Changes

### Current State

The frontend already has solid infrastructure for enrichment:

| Component | Purpose | Status |
|-----------|---------|--------|
| `EnrichmentIndicator` | Shows enrichment progress per library | Exists |
| `useEnrichmentProgress` | SSE hook for real-time enrichment updates | Exists |
| `usePostApiEnrichmentEnqueue` | Mutation hook for enqueue API | Generated |
| `LibraryCard` | Shows scan + enrichment progress | Exists |

### Phase 4 Frontend: Priority Boost

#### 4.1.1 Create Priority Hook

**File:** `web/src/lib/hooks/useEnrichmentPrioritize.ts`

```typescript
import { useCallback, useState } from 'react'
import { usePostApiEnrichmentEnqueue } from '@/lib/api/generated/enrichment'

interface UseEnrichmentPrioritizeOptions {
  onSuccess?: () => void
  onError?: (error: Error) => void
}

/**
 * Hook to prioritize a media item for immediate enrichment.
 * Automatically triggers when an unenriched item is viewed.
 */
export const useEnrichmentPrioritize = (options: UseEnrichmentPrioritizeOptions = {}) => {
  const [prioritizedIds, setPrioritizedIds] = useState<Set<string>>(new Set())
  const { mutate: enqueue, isPending } = usePostApiEnrichmentEnqueue()

  const prioritize = useCallback(
    (mediaId: number, mediaType: string, libraryId: number) => {
      const key = `${mediaType}:${mediaId}`
      
      // Don't re-prioritize same item
      if (prioritizedIds.has(key)) return
      
      enqueue(
        {
          data: {
            media_id: mediaId,
            media_type: mediaType,
            library_id: libraryId,
            stage: '', // Empty = first stage
            priority: 1000, // Interactive priority
          },
        },
        {
          onSuccess: () => {
            setPrioritizedIds((prev) => new Set(prev).add(key))
            options.onSuccess?.()
          },
          onError: (error) => {
            options.onError?.(error as Error)
          },
        }
      )
    },
    [enqueue, prioritizedIds, options]
  )

  return { prioritize, isPending, prioritizedIds }
}
```

#### 4.1.2 Create Auto-Enrich Hook

**File:** `web/src/lib/hooks/useAutoEnrich.ts`

```typescript
import { useEffect } from 'react'
import { useEnrichmentPrioritize } from './useEnrichmentPrioritize'

interface UseAutoEnrichOptions {
  /** Media ID to auto-enrich */
  mediaId: number
  /** Media type: 'movie', 'tv', 'tv_show', 'music', etc. */
  mediaType: string
  /** Library ID the media belongs to */
  libraryId: number
  /** Whether enrichment is complete (has metadata) */
  isEnriched: boolean
  /** Enable/disable auto-enrichment */
  enabled?: boolean
}

/**
 * Automatically prioritizes a media item for enrichment when viewed.
 * Only triggers if the item is not already enriched.
 *
 * @example
 * ```tsx
 * useAutoEnrich({
 *   mediaId: movie.id,
 *   mediaType: 'movie',
 *   libraryId: movie.library_id,
 *   isEnriched: !!movie.plot, // Has metadata = enriched
 * })
 * ```
 */
export const useAutoEnrich = ({
  mediaId,
  mediaType,
  libraryId,
  isEnriched,
  enabled = true,
}: UseAutoEnrichOptions) => {
  const { prioritize, isPending } = useEnrichmentPrioritize()

  useEffect(() => {
    if (!enabled || isEnriched || mediaId <= 0 || libraryId <= 0) return

    prioritize(mediaId, mediaType, libraryId)
  }, [enabled, isEnriched, mediaId, mediaType, libraryId, prioritize])

  return { isPending }
}
```

#### 4.1.3 Integration Points

**Movies page** (`web/src/routes/_layout/movies.index.tsx`):
```typescript
// When playing a movie
useAutoEnrich({
  mediaId: currentMovie.id,
  mediaType: 'movie',
  libraryId: currentMovie.library_id,
  isEnriched: !!currentMovie.plot,
})
```

**TV episode page** (`web/src/routes/_layout/tv.$showId.season.$seasonNumber.tsx`):
```typescript
useAutoEnrich({
  mediaId: episode.id,
  mediaType: 'tv',
  libraryId: episode.library_id,
  isEnriched: !!episode.plot,
})
```

**Music album page** (`web/src/routes/_layout/music.albums.$albumId.tsx`):
```typescript
useAutoEnrich({
  mediaId: album.id,
  mediaType: 'music_album',
  libraryId: album.library_id,
  isEnriched: !!album.musicbrainz_id,
})
```

### Phase 4 Frontend: Circuit Breaker Status

#### 4.2.1 Stage Status Component

**File:** `web/src/components/enrichment/StageStatus/StageStatus.tsx`

```typescript
import { AlertCircle, CheckCircle, Clock, RefreshCw } from 'lucide-react'
import { useSSE } from '@/lib/hooks'
import { cn } from '@/lib/utils'

interface StageState {
  name: string
  state: 'closed' | 'open' | 'half-open'
  pending: number
  processing: number
  failed: number
  lastError?: string
  retryAt?: string
}

interface StageStatusProps {
  className?: string
}

/**
 * Displays enrichment stage health with circuit breaker status.
 * Shows which stages are operational, paused, or recovering.
 */
export const StageStatus = ({ className }: StageStatusProps) => {
  const { lastEvent } = useSSE<{ stages: StageState[] }>('/api/enrichment/stages/stream', {
    enabled: true,
    eventTypes: ['init', 'update'],
  })

  const stages = lastEvent?.stages ?? []

  if (stages.length === 0) return null

  return (
    <div className={cn('space-y-2', className)}>
      <h4 className="text-sm font-medium text-zinc-700 dark:text-zinc-300">
        Enrichment Stages
      </h4>
      <div className="space-y-1">
        {stages.map((stage) => (
          <StageRow key={stage.name} stage={stage} />
        ))}
      </div>
    </div>
  )
}

const StageRow = ({ stage }: { stage: StageState }) => {
  const getIcon = () => {
    switch (stage.state) {
      case 'closed':
        return <CheckCircle className="h-4 w-4 text-green-500" />
      case 'open':
        return <AlertCircle className="h-4 w-4 text-red-500" />
      case 'half-open':
        return <RefreshCw className="h-4 w-4 text-amber-500 animate-spin" />
    }
  }

  const getStatusText = () => {
    switch (stage.state) {
      case 'closed':
        return 'Operational'
      case 'open':
        return stage.retryAt
          ? `Paused (retry ${formatTimeUntil(stage.retryAt)})`
          : 'Paused'
      case 'half-open':
        return 'Recovering...'
    }
  }

  return (
    <div className="flex items-center justify-between text-sm">
      <div className="flex items-center gap-2">
        {getIcon()}
        <span className="font-medium capitalize">{stage.name.replace('-', ' ')}</span>
      </div>
      <div className="flex items-center gap-3 text-zinc-500 dark:text-zinc-400">
        <span>{getStatusText()}</span>
        {stage.pending > 0 && (
          <span className="text-xs">
            {stage.pending.toLocaleString()} pending
          </span>
        )}
      </div>
    </div>
  )
}

const formatTimeUntil = (isoDate: string): string => {
  const diff = new Date(isoDate).getTime() - Date.now()
  if (diff <= 0) return 'now'
  const minutes = Math.ceil(diff / 60000)
  if (minutes < 60) return `in ${minutes}m`
  return `in ${Math.ceil(minutes / 60)}h`
}
```

#### 4.2.2 Integration Location

The `StageStatus` component should be shown in:

1. **Settings page** - Full view of all stages
2. **Library card tooltip** - Quick glance when hovering enrichment indicator
3. **Admin dashboard** (if exists) - Monitoring view

### Frontend File Summary

| File | Type | Purpose |
|------|------|---------|
| `web/src/lib/hooks/useEnrichmentPrioritize.ts` | New | Hook to boost priority via API |
| `web/src/lib/hooks/useAutoEnrich.ts` | New | Auto-prioritize on media view |
| `web/src/components/enrichment/StageStatus/` | New | Circuit breaker status display |
| `web/src/routes/_layout/movies.index.tsx` | Modified | Add useAutoEnrich |
| `web/src/routes/_layout/tv.$showId.*.tsx` | Modified | Add useAutoEnrich |
| `web/src/routes/_layout/music.*.tsx` | Modified | Add useAutoEnrich |

### API Changes Required

| Endpoint | Method | Purpose | Status |
|----------|--------|---------|--------|
| `/api/enrichment/enqueue` | POST | Enqueue with priority | Exists (needs priority support) |
| `/api/enrichment/prioritize` | POST | Dedicated priority boost | New (simpler API) |
| `/api/enrichment/stages` | GET | Stage status + circuit state | New |
| `/api/enrichment/stages/stream` | GET | SSE for circuit state changes | New |
| `/api/libraries/{id}/enrichment/failures` | GET | List failed enrichment items | New |
| `/api/enrichment/retry` | POST | Retry failed items | New |

---

## Phase 5: UX Improvements (Priority: MEDIUM)

**Goal:** Unified, clean UI for scanning and enrichment with progressive disclosure.

**Estimated Effort:** 10-14 hours

### Design Principles

1. **Unified experience:** Scan and enrichment shown together, not as separate concerns
2. **Progressive disclosure:** Collapsed by default, expand for details
3. **Consistent patterns:** Reuse existing dialog patterns (extend ScanErrorsDialog)
4. **Subtle indicators:** Don't overwhelm users with enrichment status on every card

### 5.1 Library Card Redesign (Collapsible)

**Files:**
- `web/src/components/library/LibraryCard/LibraryCard.tsx`
- `web/src/components/library/LibraryCard/LibraryCardDetails.tsx` (new)
- `web/src/components/library/LibraryCard/LibraryCard.types.ts`

**Collapsed State (default):**
```
┌─────────────────────────────────────────────────────────┐
│ Movies Library                              [Scan] [Del]│
│ /mnt/media/movies                                       │
│ Type: movies • ✓ 1,234 files • Enriching 67%           │
│                                         [▼ Show Details]│
└─────────────────────────────────────────────────────────┘
```

**Expanded State:**
```
┌─────────────────────────────────────────────────────────┐
│ Movies Library                              [Scan] [Del]│
│ /mnt/media/movies                                       │
│ Type: movies • ✓ 1,234 files • Enriching 67%           │
│                                         [▲ Hide Details]│
├─────────────────────────────────────────────────────────┤
│ Scan                                                    │
│ ✓ Completed • 1,234 files • 2 warnings [View Issues]   │
│                                                         │
│ Enrichment                                              │
│ ████████████░░░░░░░░ 67% (827/1,234)                   │
│                                                         │
│ Stages:                                                 │
│   ✓ NFO Files        1,234/1,234                       │
│   ⟳ TMDB               827/1,234  ⚠️                    │
│   ○ Local Images         0/1,234                       │
│                                                         │
│ Currently: The Matrix (1999)                            │
│ 3 failed [View Issues]                                  │
└─────────────────────────────────────────────────────────┘
```

**Implementation:**
```typescript
const [isExpanded, setIsExpanded] = useState(false)

// Collapsed: show summary line
// Expanded: show LibraryCardDetails component
```

**Effort:** 4 hours

### 5.2 Unified Issues Dialog (Tabs)

**Files:**
- `web/src/components/library/LibraryIssuesDialog/LibraryIssuesDialog.tsx` (new)
- `web/src/components/library/LibraryIssuesDialog/ScanErrorsTab.tsx` (extract from ScanErrorsDialog)
- `web/src/components/library/LibraryIssuesDialog/ScanWarningsTab.tsx` (new)
- `web/src/components/library/LibraryIssuesDialog/EnrichmentFailuresTab.tsx` (new)
- `web/src/components/library/ScanErrorsDialog/` (deprecate or re-export)

**Design:**
```
┌─────────────────────────────────────────────────────────┐
│ Library Issues - Movies Library                      [X]│
├─────────────────────────────────────────────────────────┤
│ [Scan Errors (2)] [Scan Warnings (5)] [Enrichment (3)] │
├─────────────────────────────────────────────────────────┤
│                                                         │
│ Enrichment Failures                                     │
│                                                         │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ The Godfather (1972)                        [Retry] │ │
│ │ Stage: TMDB • Error: Rate limit exceeded            │ │
│ └─────────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ Obscure Movie (2023)                        [Retry] │ │
│ │ Stage: TMDB • Error: No match found                 │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│                              [Retry All Failed] [Close] │
└─────────────────────────────────────────────────────────┘
```

**Tab badge counts:** Show count in tab header, highlight if > 0

**Effort:** 4 hours

### 5.3 Enrichment Stage Progress Component

**Files:**
- `web/src/components/library/EnrichmentStages/EnrichmentStages.tsx` (new)
- `web/src/components/library/EnrichmentStages/EnrichmentStages.types.ts`

**Design:**
```typescript
interface EnrichmentStagesProps {
  stages: StageProgress[]
  showCircuitStatus?: boolean  // Show ⚠️ for open circuit
}

// Renders:
// ✓ NFO Files        1,234/1,234
// ⟳ TMDB               827/1,234  ⚠️  ← circuit breaker warning
// ○ Local Images         0/1,234
```

**Stage states:**
- `✓` Completed (all items done)
- `⟳` In progress (spinning icon)
- `○` Pending (not started)
- `⚠️` Circuit open (subtle, next to stage name)

**Effort:** 2 hours

### 5.4 Subtle Enrichment Badge on Media Cards

**Files:**
- `web/src/components/media/MediaBadges/MediaBadges.tsx`
- `web/src/components/media/MediaBadges/MediaBadges.types.ts`
- `web/src/components/media/EnrichmentBadge/EnrichmentBadge.tsx` (new)

**Design:**
```
┌──────────────────┐
│ ⟳              │  ← Top-left corner, small (12-14px)
│                  │     Spinning for pending
│   [Poster]       │     ⚠️ for failed
│                  │     Hidden when enriched
└──────────────────┘
```

**Implementation:**
```typescript
interface EnrichmentBadgeProps {
  status: 'pending' | 'failed' | 'enriched'
  className?: string
}

// Only renders if status !== 'enriched'
// Positioned absolute top-left with slight offset
```

**Effort:** 2 hours

### 5.5 Backend: Enrichment Failures Endpoint

**Files:**
- `internal/api/handlers/enrichment.go`
- `internal/api/routes/enrichment.go`
- `internal/infrastructure/database/queries/*/enrichment_queue.sql`

**Endpoint:** `GET /api/libraries/{id}/enrichment/failures`

**Response:**
```json
{
  "failures": [
    {
      "media_id": 123,
      "media_type": "movie",
      "title": "The Godfather",
      "stage": "tmdb",
      "error_message": "Rate limit exceeded",
      "error_category": "rate_limit",
      "attempts": 3,
      "last_attempt_at": "2024-12-21T10:30:00Z"
    }
  ],
  "total": 3
}
```

**Effort:** 2 hours

---

## Phase Summary (Updated)

| Phase | Description | Effort | Impact | Status |
|-------|-------------|--------|--------|--------|
| 0 | Sort title bug fix | ~1h | Bug fix | ✅ Complete |
| 1 | Pipeline cache, rate limiter, worker tuning | 4-6h | Medium | ✅ Complete |
| 2 | Batch enqueue, external ID prefetch, re-prioritization | 10-14h | High | ✅ Complete |
| 3 | Entity cache (TV/Music deduplication) | 8-12h | High | ✅ Complete |
| 4 | Priority boost, circuit breaker | 6-8h | UX + Resilience | Pending |
| 5 | UX improvements (unified UI, badges, dialogs) | 10-14h | UX | Pending |

**Total Remaining Effort:** 16-22 hours

---

## Complete File Change Summary

### New Backend Files

| File | Purpose |
|------|---------|
| `pipeline/pipeline_cache.go` | In-memory cache for pipeline stage configuration |
| `pipeline/enqueue_buffer.go` | Channel-based async batch enqueue writer |
| `pipeline/entity_cache.go` | LRU cache for TV shows, albums, artists |
| `pipeline/circuit_breaker.go` | Per-stage circuit breaker for resilience |

### New Frontend Files

| File | Purpose |
|------|---------|
| `hooks/useEnrichmentPrioritize.ts` | Hook to boost priority via API |
| `hooks/useAutoEnrich.ts` | Auto-prioritize on media view |
| `components/library/LibraryCardDetails.tsx` | Expanded details section |
| `components/library/LibraryIssuesDialog/` | Unified issues dialog with tabs |
| `components/library/EnrichmentStages/` | Stage progress display |
| `components/media/EnrichmentBadge/` | Subtle corner badge for cards |

### Modified Files

| Phase | Files |
|-------|-------|
| 1 | `manager.go`, `job_processor.go`, `worker_pool.go`, `deps.go` |
| 2 | `common.go`, `queue_repository.go`, `enrichment_queue.sql`, `external_ids.sql` |
| 3 | `request_builder.go` |
| 4 | `enrichment.go` (handler), `routes/enrichment.go` |
| 5 | `LibraryCard.tsx`, `MediaBadges.tsx`, `ScanErrorsDialog.tsx` (deprecate) |
