# On-Demand Transcoding Implementation Tasks

This document tracks the implementation of on-demand transcoding (ADR 005).

Status: **Planning** | Start Date: 2025-11-13 | Target: 4 weeks

## Quick Links
- [ADR 005: On-Demand Transcoding Strategy](./decisions/005-on-demand-transcoding-strategy.md)
- [Current Progress](#progress-tracker)
- [Critical Path](#critical-path-tasks)

---

## Phase 1: Backend Enhancements

### Task 1.1: Modify Manifest Endpoint ⭐ CRITICAL PATH
**Priority:** P0 | **Estimated:** 3-4 hours | **Status:** Not Started

**Files to Modify:**
- `internal/api/handlers/transcode.go`

**Implementation Steps:**
1. [ ] Add manifest existence check at start of `ServeManifest()`
2. [ ] Add job status check (completed/processing/failed)
3. [ ] Integrate codec analysis using `GetVideoInfo()` + `ShouldTranscode()`
4. [ ] Add automatic job creation if needed
5. [ ] Return 202 Accepted with progress URL for new jobs
6. [ ] Add fallback redirect to direct stream
7. [ ] Write unit tests for all code paths
8. [ ] Add integration test with real video file

**Acceptance Criteria:**
- [ ] Manifest endpoint automatically creates jobs when needed
- [ ] Returns existing manifest if available
- [ ] Returns 202 for in-progress transcodes
- [ ] Redirects to direct stream for compatible sources
- [ ] All tests passing (100% coverage of new code)

**Dependencies:** None (can start immediately)

---

### Task 1.2: Add Quality Recommendation Endpoint
**Priority:** P1 | **Estimated:** 2-3 hours | **Status:** Not Started

**New Endpoint:** `GET /api/media/{media_id}/quality-recommendation`

**Response Schema:**
```json
{
  "recommended": "1080p",
  "available": ["360p", "720p", "1080p"],
  "source_width": 1920,
  "source_height": 1080,
  "source_codec": "h264",
  "transcoding_required": {
    "360p": true,
    "720p": true,
    "1080p": false,
    "4k": false
  }
}
```

**Implementation Steps:**
1. [ ] Create new endpoint handler in `transcode.go`
2. [ ] Get media metadata from database
3. [ ] Analyze source using `GetVideoInfo()`
4. [ ] Determine recommended quality (don't upscale)
5. [ ] Check transcoding requirements for each quality using `ShouldTranscode()`
6. [ ] Return structured response
7. [ ] Add route to router
8. [ ] Write unit tests
9. [ ] Update Swagger documentation

**Acceptance Criteria:**
- [ ] Endpoint returns accurate recommendations
- [ ] Recommends quality ≤ source resolution
- [ ] Correctly identifies which qualities need transcoding
- [ ] Tests cover edge cases (no metadata, corrupt files)

**Dependencies:** None

---

### Task 1.3: Add Transcode Priority Field
**Priority:** P2 | **Estimated:** 2-3 hours | **Status:** Not Started

**Files to Modify:**
- `internal/domain/transcode/transcode_job.go`
- `internal/infrastructure/database/queries/sqlite/transcode_jobs.sql`
- `internal/infrastructure/database/queries/postgres/transcode_jobs.sql`
- `migrations/` (new migration files)
- `internal/application/transcode/queue.go`

**Implementation Steps:**
1. [ ] Add `priority` field to TranscodeJob struct (HIGH, NORMAL, LOW)
2. [ ] Create database migration to add `priority` column (default NORMAL)
3. [ ] Update SQLC queries to include priority
4. [ ] Regenerate SQLC code (`make sqlc-gen`)
5. [ ] Modify queue poller to ORDER BY priority DESC, created_at ASC
6. [ ] Update CreateJob to accept priority parameter
7. [ ] Set HIGH priority for on-demand jobs
8. [ ] Write tests for priority ordering
9. [ ] Test migration on both PostgreSQL and SQLite

**Acceptance Criteria:**
- [ ] Priority field added to database schema
- [ ] High priority jobs processed first
- [ ] Migration works on both databases
- [ ] Tests verify priority ordering
- [ ] Backwards compatible (existing jobs default to NORMAL)

**Dependencies:** None

---

## Phase 2: Frontend Components

### Task 2.1: Create TranscodingIndicator Component ⭐ CRITICAL PATH
**Priority:** P0 | **Estimated:** 3-4 hours | **Status:** Not Started

**New Files:**
- `web/src/components/media/TranscodingIndicator/TranscodingIndicator.tsx`
- `web/src/components/media/TranscodingIndicator/TranscodingIndicator.types.ts`
- `web/src/components/media/TranscodingIndicator/index.ts`

**Component Props:**
```typescript
interface TranscodingIndicatorProps {
  progress: number // 0-100
  status: 'queued' | 'processing' | 'finalizing'
  estimatedTimeRemaining?: number // seconds
  onCancel?: () => void
}
```

**Implementation Steps:**
1. [ ] Create component structure files
2. [ ] Add progress bar with percentage display
3. [ ] Add time remaining calculation and display
4. [ ] Add stage indicator (queued → processing → finalizing)
5. [ ] Add pulsing animation for active state
6. [ ] Style with Tailwind CSS
7. [ ] Add optional cancel button
8. [ ] Export from index
9. [ ] Write component tests

**Acceptance Criteria:**
- [ ] Shows clear progress indicator (0-100%)
- [ ] Displays accurate time estimate
- [ ] Responsive design (mobile-friendly)
- [ ] Accessible (ARIA labels, keyboard navigation)
- [ ] Smooth animations

**Dependencies:** None

---

### Task 2.2: Create useTranscodeStatus Hook ⭐ CRITICAL PATH
**Priority:** P0 | **Estimated:** 2-3 hours | **Status:** Not Started

**New File:** `web/src/lib/hooks/useTranscodeStatus.ts`

**Hook Signature:**
```typescript
function useTranscodeStatus(
  mediaId: number,
  quality: string,
  options?: {
    enabled?: boolean
    pollInterval?: number // default 2000ms
    onComplete?: (job: TranscodeJob) => void
    onError?: (error: Error) => void
  }
): {
  status: TranscodeStatus | null
  isLoading: boolean
  error: Error | null
  refetch: () => Promise<void>
}
```

**Implementation Steps:**
1. [ ] Create hook file
2. [ ] Use react-query's useQuery for polling
3. [ ] Configure 2-second poll interval
4. [ ] Add auto-stop on completion/failure
5. [ ] Add error handling with retry logic
6. [ ] Add callbacks for completion/error
7. [ ] Export hook
8. [ ] Write hook tests

**Acceptance Criteria:**
- [ ] Polls transcode status every 2 seconds
- [ ] Stops polling when complete or failed
- [ ] Handles errors gracefully with retry
- [ ] Callbacks fire appropriately
- [ ] Tests cover all scenarios

**Dependencies:** API client generated from Swagger

---

### Task 2.3: Create useQualitySelection Hook
**Priority:** P1 | **Estimated:** 2-3 hours | **Status:** Not Started

**New File:** `web/src/lib/hooks/useQualitySelection.ts`

**Hook Signature:**
```typescript
function useQualitySelection(media: Media): {
  quality: string
  autoQuality: string
  isAuto: boolean
  setQuality: (quality: string) => void
  resetToAuto: () => void
}
```

**Implementation Steps:**
1. [ ] Create hook file
2. [ ] Implement auto-selection logic (source resolution + screen size)
3. [ ] Add localStorage persistence (`quality-pref-${mediaId}`)
4. [ ] Add manual override function
5. [ ] Add reset to auto function
6. [ ] Never upscale beyond source resolution
7. [ ] Export hook
8. [ ] Write tests

**Acceptance Criteria:**
- [ ] Auto-selects appropriate quality on mount
- [ ] Persists user manual selection
- [ ] Never recommends upscaling
- [ ] Respects screen pixel ratio
- [ ] Tests cover edge cases (no metadata, 4K source on mobile)

**Dependencies:** None

---

### Task 2.4: Enhance MediaDetailsModal ⭐ CRITICAL PATH
**Priority:** P0 | **Estimated:** 4-5 hours | **Status:** Not Started

**File to Modify:** `web/src/components/media/MediaCard/MediaDetailsModal.tsx`

**New State:**
```typescript
const [transcodingStatus, setTranscodingStatus] = useState<{
  isTranscoding: boolean
  progress: number
  quality: string | null
  error: string | null
}>({ isTranscoding: false, progress: 0, quality: null, error: null })
```

**Implementation Steps:**
1. [ ] Integrate useQualitySelection hook
2. [ ] Integrate useTranscodeStatus hook (conditional on isTranscoding)
3. [ ] Add TranscodingIndicator component to UI
4. [ ] Modify handlePlay to check manifest availability
5. [ ] Add 202 Accepted handling (transcoding in progress)
6. [ ] Add manifest polling (check every 2s, max 2min)
7. [ ] Add fallback to direct stream on timeout/error
8. [ ] Add quality badges showing which need transcoding
9. [ ] Add timeout handling (2 minutes → fallback)
10. [ ] Add retry button on error
11. [ ] Update UI states (idle, checking, transcoding, playing, error)
12. [ ] Write component tests

**Acceptance Criteria:**
- [ ] Clicking Play automatically checks manifest
- [ ] Shows transcoding progress if needed
- [ ] Starts playback when manifest ready
- [ ] Falls back to direct stream on timeout (2min)
- [ ] Falls back to direct stream on error
- [ ] User can see which qualities require transcoding
- [ ] User can cancel transcode and try lower quality
- [ ] All states have appropriate UI feedback

**Dependencies:** Tasks 2.1, 2.2, 2.3

---

### Task 2.5: Update VideoPlayer Component
**Priority:** P1 | **Estimated:** 2 hours | **Status:** Not Started

**File to Modify:** `web/src/components/media/VideoPlayer/VideoPlayer.tsx`

**Implementation Steps:**
1. [ ] Handle HTTP 202 response from manifest endpoint
2. [ ] Show "Preparing video..." overlay while transcoding
3. [ ] Add retry logic for manifest loading
4. [ ] Add timeout handling (120s)
5. [ ] Fall back to direct stream if manifest never loads
6. [ ] Update error messages to be user-friendly
7. [ ] Write tests

**Acceptance Criteria:**
- [ ] Handles 202 responses gracefully
- [ ] Shows appropriate loading state
- [ ] Retries manifest loading intelligently
- [ ] Falls back on failure with clear message
- [ ] User can manually retry

**Dependencies:** Task 2.4

---

## Phase 3: Testing

### Task 3.1: Backend Integration Tests
**Priority:** P1 | **Estimated:** 4-5 hours | **Status:** Not Started

**New File:** `internal/api/handlers/transcode_integration_test.go`

**Test Scenarios:**
1. [ ] Manifest request with cached transcode (200 OK)
2. [ ] Manifest request triggers new transcode (202 Accepted)
3. [ ] Manifest request with in-progress transcode (202 with progress)
4. [ ] Manifest request with failed transcode (302 redirect to direct stream)
5. [ ] Concurrent requests for same quality (no duplicate jobs)
6. [ ] Concurrent requests for different qualities (both succeed)
7. [ ] Source already H.264 at target resolution (302 redirect)
8. [ ] Disk full scenario (503 with error, fallback to direct stream)
9. [ ] Invalid media ID (404)
10. [ ] Invalid quality parameter (400)
11. [ ] FFmpeg not installed (500 with fallback)

**Test Setup:**
- Use test fixtures (small video files in multiple codecs)
- Mock external dependencies where appropriate
- Clean up generated files after each test

**Acceptance Criteria:**
- [ ] All scenarios pass
- [ ] Tests use real video files for realistic behavior
- [ ] Tests clean up after themselves
- [ ] Coverage >90% for transcode handlers
- [ ] Tests run in CI pipeline

**Dependencies:** Task 1.1

---

### Task 3.2: Frontend Integration Tests
**Priority:** P1 | **Estimated:** 3-4 hours | **Status:** Not Started

**New File:** `web/src/components/media/__tests__/on-demand-playback.test.tsx`

**Test Scenarios:**
1. [ ] Play with cached transcode (immediate playback)
2. [ ] Play triggers new transcode (shows progress, then plays)
3. [ ] Play with in-progress transcode (shows existing progress)
4. [ ] Transcode timeout (falls back to direct stream)
5. [ ] Transcode failure (falls back with error message)
6. [ ] Direct stream (no transcode needed, immediate play)
7. [ ] Quality selection (user chooses quality)
8. [ ] Multiple quality switches (works correctly)
9. [ ] Cancel transcode (returns to modal)

**Test Tools:**
- React Testing Library
- MSW for API mocking
- Jest fake timers for polling

**Acceptance Criteria:**
- [ ] All scenarios pass
- [ ] Tests verify UI states correctly
- [ ] Tests verify API calls made correctly
- [ ] Coverage >80% for new code
- [ ] Tests run in CI pipeline

**Dependencies:** Tasks 2.1, 2.2, 2.3, 2.4, 2.5

---

### Task 3.3: End-to-End Tests
**Priority:** P2 | **Estimated:** 4-5 hours | **Status:** Not Started

**Tool:** Playwright

**Test Scenarios:**
1. [ ] Complete playback flow (new transcode)
2. [ ] Complete playback flow (cached transcode)
3. [ ] Quality switching during playback
4. [ ] Progress saving during playback
5. [ ] Resume after transcode
6. [ ] Concurrent users (multiple browsers)
7. [ ] Mobile device simulation

**Acceptance Criteria:**
- [ ] All critical paths covered
- [ ] Tests run in CI/CD pipeline
- [ ] Tests are reliable (no flakiness)
- [ ] Videos play successfully
- [ ] Progress tracked correctly

**Dependencies:** All Phase 1 and Phase 2 tasks

---

## Phase 4: Admin Panel

### Task 4.1: Create Transcodes Admin Page
**Priority:** P2 | **Estimated:** 5-6 hours | **Status:** Not Started

**New Files:**
- `web/src/pages/admin/Transcodes.tsx`
- `web/src/components/admin/TranscodesList.tsx`
- `web/src/components/admin/TranscodeDiskUsage.tsx`

**Page Features:**
- List all transcode jobs (table view)
- Filter by status (queued, processing, completed, failed)
- Filter by quality (360p, 720p, 1080p, 4K)
- Filter by date range
- Show disk space usage by quality (pie chart)
- Bulk delete operations
- Individual delete buttons
- Cache statistics (hit rate, total size)

**Implementation Steps:**
1. [ ] Create admin page route
2. [ ] Create TranscodesList component with filtering
3. [ ] Create TranscodeDiskUsage component with chart
4. [ ] Add bulk action controls
5. [ ] Add delete confirmation modals
6. [ ] Style with existing admin theme
7. [ ] Add to admin navigation menu
8. [ ] Write component tests

**Acceptance Criteria:**
- [ ] Admin can view all transcodes
- [ ] Filtering works correctly
- [ ] Admin can delete individual transcodes
- [ ] Admin can bulk delete by quality/status
- [ ] Disk usage clearly displayed with visualization
- [ ] All actions require confirmation

**Dependencies:** Task 4.2

---

### Task 4.2: Add Transcode Cleanup Endpoints
**Priority:** P2 | **Estimated:** 3-4 hours | **Status:** Not Started

**New Endpoints:**
- `DELETE /api/admin/transcode/{media_id}/{quality}` - Delete specific transcode
- `DELETE /api/admin/transcode/quality/{quality}` - Delete all for quality
- `DELETE /api/admin/transcode/before/{date}` - Delete older than date
- `GET /api/admin/transcode/disk-usage` - Get disk usage stats

**Implementation Steps:**
1. [ ] Create new handler methods in transcode.go
2. [ ] Add file deletion logic (remove directory)
3. [ ] Add database record deletion
4. [ ] Add disk usage calculation
5. [ ] Add admin authentication middleware
6. [ ] Add routes to router
7. [ ] Write unit tests
8. [ ] Update Swagger documentation

**Acceptance Criteria:**
- [ ] Endpoints delete both files and DB records atomically
- [ ] Disk usage accurately calculated
- [ ] Only admins can access endpoints
- [ ] All tests passing
- [ ] Proper error handling (file not found, permission denied)

**Dependencies:** None

---

## Phase 5: Documentation & Optimization

### Task 5.1: Update API Documentation
**Priority:** P1 | **Estimated:** 2 hours | **Status:** Not Started

**Files to Update:**
- `docs/API_SPECIFICATION.md`

**Sections to Add/Update:**
- On-demand transcoding flow diagram
- Quality recommendation endpoint
- Updated manifest endpoint behavior
- Admin transcode endpoints
- Error codes and responses

**Acceptance Criteria:**
- [ ] All new endpoints documented
- [ ] Flow diagrams added
- [ ] Examples provided
- [ ] Error scenarios documented

**Dependencies:** All backend tasks complete

---

### Task 5.2: Create User Guide
**Priority:** P2 | **Estimated:** 2-3 hours | **Status:** Not Started

**New File:** `docs/user-guide/ON_DEMAND_PLAYBACK.md`

**Sections:**
- How on-demand transcoding works
- Quality selection (auto vs manual)
- What to do if playback fails
- Understanding transcode progress
- Admin panel usage guide

**Acceptance Criteria:**
- [ ] Guide is clear and user-friendly
- [ ] Screenshots included
- [ ] Troubleshooting section added

**Dependencies:** All implementation complete

---

### Task 5.3: Update Project Documentation
**Priority:** P2 | **Estimated:** 1 hour | **Status:** Not Started

**Files to Update:**
- `README.md`
- `docs/FEATURES.md`

**Changes:**
- Add on-demand transcoding to features list
- Update screenshots if needed
- Add environment variables section
- Add troubleshooting section

**Acceptance Criteria:**
- [ ] README accurately reflects new features
- [ ] Feature list updated
- [ ] Configuration documented

**Dependencies:** All implementation complete

---

## Progress Tracker

### Week 1 (Days 1-5)
- [ ] Backend: Tasks 1.1, 1.2
- [ ] Frontend: Tasks 2.1, 2.2, 2.3

### Week 2 (Days 6-10)
- [ ] Backend: Task 1.3
- [ ] Frontend: Tasks 2.4, 2.5
- [ ] Testing: Tasks 3.1, 3.2

### Week 3 (Days 11-15)
- [ ] Testing: Task 3.3
- [ ] Admin: Tasks 4.1, 4.2
- [ ] Docs: Tasks 5.1, 5.2, 5.3

### Week 4 (Days 16-20)
- [ ] Polish & bug fixes
- [ ] Performance testing
- [ ] Staging deployment
- [ ] Production rollout

---

## Critical Path Tasks

These tasks MUST be completed in order for MVP:

1. ⭐ Task 1.1: Modify Manifest Endpoint
2. ⭐ Task 2.2: Create useTranscodeStatus Hook
3. ⭐ Task 2.1: Create TranscodingIndicator Component
4. ⭐ Task 2.4: Enhance MediaDetailsModal

Everything else can be done in parallel or deferred.

---

## Blockers

_None currently identified_

---

## Notes

- Feature flag: `ENABLE_ON_DEMAND_TRANSCODE` (default: false)
- Backwards compatible with existing manual transcode API
- Safe rollback: disable feature flag, restart services
- Monitoring: Track transcode success rate, cache hit rate, fallback usage

---

## Questions / Decisions Needed

1. Should we implement WebSocket for real-time progress? (Nice to have, defer to v1.1)
2. Should we pre-transcode popular content? (Future optimization, defer)
3. What's the disk space threshold for auto-cleanup? (Decide during Task 4.2)

---

Last Updated: 2025-11-13
