# On-Demand Transcoding - Project Plan

## Executive Summary

**Project Goal:** Implement Plex/Jellyfin-style on-demand transcoding where clicking "Play" automatically starts transcoding if needed, with intelligent strategy selection (Direct Play → Remux → Remux+Audio → Transcode).

**Total Time Estimate:** 12-16 hours (MVP: 10-14 hours, Testing/Polish: 2 hours)

**Current Status:**
- Backend: 90% complete (worker pool, FFmpeg transcoding, queue management, endpoints)
- Frontend: 50% complete (Shaka Player integrated, but no DASH integration or polling)
- Gap: Auto-trigger logic, strategy selection, frontend DASH integration, progress UI

**Critical Path:** Backend strategy selection → Frontend DASH manifest handling → Polling/Progress UI → Testing

---

## Phase 1: Backend Core Strategy Implementation (5-7 hours)

### Phase 1.1: Strategy Selection & Audio Channel Detection (2-3 hours)

**Objective:** Add intelligent strategy selection that detects codec compatibility and audio channel count.

#### Task 1.1.1: Enhance VideoInfo with Audio Channels (30 min)
- **File:** `internal/infrastructure/transcoding/validation.go`
- **Changes:**
  - Add `AudioChannels int` field to `VideoInfo` struct
  - Update `getVideoInfo()` to parse audio channels from ffprobe output
  - ffprobe command: `-show_streams -select_streams a:0 -show_entries stream=channels`
- **Test:** Verify audio channels are correctly detected for stereo vs 5.1/7.1 audio
- **Priority:** CRITICAL (blocks all strategy logic)

#### Task 1.1.2: Implement Strategy Selection Logic (1 hour)
- **File:** `internal/infrastructure/transcoding/validation.go`
- **Add:**
  ```go
  type StreamStrategy int

  const (
      DirectPlay StreamStrategy = iota
      Remux
      RemuxWithAudioDownmix
      Transcode
  )

  func DetermineStreamStrategy(videoInfo *VideoInfo, targetQuality string) StreamStrategy
  func isVideoCodecCompatible(codec string) bool
  func isAudioCodecCompatible(codec string) bool
  ```
- **Logic:**
  - Direct Play: H.264/VP9/AV1 + AAC/MP3/Opus (stereo) + MP4/WebM container
  - Remux: Compatible codecs + stereo audio + non-web container
  - RemuxWithAudioDownmix: Compatible codecs + multi-channel audio (>2 channels)
  - Transcode: Incompatible codecs OR quality change requested
- **Priority:** CRITICAL (core decision logic)

#### Task 1.1.3: Add Time Estimation Helper (30 min)
- **File:** `internal/infrastructure/transcoding/validation.go`
- **Add:**
  ```go
  func EstimateTranscodeTime(strategy StreamStrategy, duration int, sourceSize int64) time.Duration
  ```
- **Estimates:**
  - Remux: 2-5 minutes (mostly I/O bound)
  - RemuxWithAudioDownmix: 5-10 minutes (audio re-encode)
  - Transcode: Based on duration (1 hour movie = 20-60 min depending on hardware)
- **Priority:** MEDIUM (nice-to-have for frontend display)

**Dependencies:** None
**Deliverable:** Strategy selection function that correctly identifies the optimal streaming approach
**Test Plan:**
- Test with MP4 H.264/AAC stereo → Should return DirectPlay
- Test with MKV H.264/AAC stereo → Should return Remux
- Test with MKV H.264/AAC 5.1 → Should return RemuxWithAudioDownmix
- Test with MKV HEVC/AC3 → Should return Transcode

---

### Phase 1.2: FFmpeg Remux Executors (2 hours)

**Objective:** Add fast remux functions that copy streams without re-encoding.

#### Task 1.2.1: Implement RemuxToDASH (45 min)
- **File:** `internal/infrastructure/transcoding/ffmpeg.go`
- **Add:**
  ```go
  func (e *executor) RemuxToDASH(ctx context.Context, opts TranscodeOptions) error
  ```
- **FFmpeg Args:**
  - `-c:v copy` (copy video, no re-encode)
  - `-c:a copy` (copy audio, no re-encode)
  - `-f dash` (DASH output format)
  - `-seg_duration 4` (4-second segments)
  - Output: `./data/transcode/dash/<media_id>/<quality>/manifest.mpd`
- **Priority:** HIGH (enables fast streaming for most media)

#### Task 1.2.2: Implement RemuxWithAudioDownmix (1 hour)
- **File:** `internal/infrastructure/transcoding/ffmpeg.go`
- **Add:**
  ```go
  func (e *executor) RemuxWithAudioDownmix(ctx context.Context, opts TranscodeOptions) error
  ```
- **FFmpeg Args:**
  - `-c:v copy` (copy video, no re-encode)
  - `-c:a aac -ac 2 -b:a 192k` (re-encode audio to stereo AAC)
  - `-af "pan=stereo|FL=FC+0.30*FL+0.30*BL|FR=FC+0.30*FR+0.30*BR"` (5.1→stereo downmix)
  - `-f dash` (DASH output format)
  - Output: `./data/transcode/dash/<media_id>/<quality>/manifest.mpd`
- **Priority:** HIGH (handles most professionally-produced media with surround sound)
- **Test:** Verify 5.1 audio is properly downmixed to stereo

#### Task 1.2.3: Update Executor Interface (15 min)
- **File:** `internal/infrastructure/transcoding/executor.go`
- **Changes:**
  - Add `RemuxToDASH()` method to interface
  - Add `RemuxWithAudioDownmix()` method to interface
  - Ensure worker can call these methods based on job type
- **Priority:** MEDIUM

**Dependencies:** Phase 1.1 (strategy selection)
**Deliverable:** Two new FFmpeg executors for fast remuxing
**Test Plan:**
- Remux MKV H.264/AAC stereo → Verify manifest created, video plays, time <5 min
- Remux MKV H.264/AAC 5.1 → Verify audio downmix works, stereo output plays correctly

---

### Phase 1.3: Database & Domain Model Updates (1 hour)

**Objective:** Add job type field to distinguish remux/remux_audio/transcode.

#### Task 1.3.1: Create Database Migration (15 min)
- **File:** `migrations/postgres/000006_add_job_type.up.sql`
- **SQL:**
  ```sql
  ALTER TABLE transcode_jobs
  ADD COLUMN type TEXT NOT NULL DEFAULT 'transcode'
  CHECK(type IN ('remux', 'remux_audio', 'transcode'));

  CREATE INDEX idx_transcode_jobs_type ON transcode_jobs(type);
  ```
- **File:** `migrations/postgres/000006_add_job_type.down.sql`
- **Priority:** CRITICAL (required for new functionality)

#### Task 1.3.2: Update Domain Model (15 min)
- **File:** `internal/domain/transcode/transcode_job.go`
- **Changes:**
  - Add `Type string` field to `TranscodeJob` struct
  - Add validation for type field (must be remux/remux_audio/transcode)
- **Priority:** CRITICAL

#### Task 1.3.3: Update Repository SQLC Queries (15 min)
- **File:** `internal/infrastructure/database/queries/transcode_jobs.sql`
- **Changes:**
  - Add `type` field to INSERT/UPDATE/SELECT queries
  - Regenerate SQLC code: `make generate` or `sqlc generate`
- **Priority:** CRITICAL

#### Task 1.3.4: Update API Response Models (15 min)
- **File:** `internal/api/handlers/transcode.go`
- **Changes:**
  - Add `Type string` to `TranscodeJobResponse` struct
  - Update `toTranscodeJobResponse()` to include type
  - Update OpenAPI docs if applicable
- **Priority:** HIGH

**Dependencies:** None (can run in parallel with Phase 1.1-1.2)
**Deliverable:** Database migration and updated models supporting job types
**Test Plan:**
- Run migration successfully
- Create jobs with different types
- Verify type field is persisted and returned

---

### Phase 1.4: ServeManifest Auto-Trigger Logic (1.5-2 hours)

**Objective:** Modify ServeManifest endpoint to automatically create jobs when manifest doesn't exist.

#### Task 1.4.1: Update Output Directory Structure (30 min)
- **Files:** All transcode-related handlers/services
- **Changes:**
  - Update all path construction to use `dash/` subdirectory
  - Old: `./data/transcode/<media_id>/<quality>/`
  - New: `./data/transcode/dash/<media_id>/<quality>/`
  - Add configuration: `TRANSCODE_OUTPUT_DIR` env variable (default: `./data/transcode`)
- **Rationale:** Allows future HLS support without restructuring
- **Priority:** MEDIUM (nice-to-have, but helps future-proof)

#### Task 1.4.2: Implement Auto-Trigger in ServeManifest (1-1.5 hours)
- **File:** `internal/api/handlers/transcode.go`
- **Replace current ServeManifest logic:**
  1. Check if manifest exists on disk → Serve immediately (200 OK)
  2. Check if job exists in database → Return 202 Accepted with status
  3. Get video info and determine strategy
  4. If DirectPlay → Return 302 redirect to `/api/stream/:id`
  5. Create job based on strategy (remux/remux_audio/transcode)
  6. Enqueue job
  7. Return 202 Accepted with:
     - Job type (`remux`, `remux_audio`, or `transcode`)
     - Status URL
     - Estimated time
     - Retry-After header (2 seconds)
- **Priority:** CRITICAL (core functionality)

#### Task 1.4.3: Handle Race Conditions (30 min)
- **Approach:** Use database UNIQUE constraint on `(media_id, quality)`
- **Logic:** If job creation fails due to duplicate, fetch existing job and return status
- **Priority:** HIGH (prevents duplicate jobs)

**Dependencies:** Phase 1.1 (strategy selection), Phase 1.3 (database updates)
**Deliverable:** ServeManifest endpoint that auto-creates jobs intelligently
**Test Plan:**
- Request manifest when none exists → Should return 202 with job created
- Request manifest while job in progress → Should return 202 with existing job
- Request manifest when complete → Should return 200 with manifest file
- Request manifest for compatible MP4 → Should return 302 redirect

---

### Phase 1.5: Worker Integration (30 min)

**Objective:** Update worker to call correct executor based on job type.

#### Task 1.5.1: Update Worker Logic (30 min)
- **File:** `internal/application/transcode/queue.go` or worker file
- **Changes:**
  ```go
  switch job.Type {
  case "remux":
      err = executor.RemuxToDASH(ctx, opts)
  case "remux_audio":
      err = executor.RemuxWithAudioDownmix(ctx, opts)
  case "transcode":
      err = executor.TranscodeToDASH(ctx, opts)
  }
  ```
- **Priority:** CRITICAL

**Dependencies:** Phase 1.2 (remux executors), Phase 1.3 (database updates)
**Deliverable:** Worker that processes all three job types
**Test Plan:**
- Create remux job → Verify worker calls RemuxToDASH
- Create remux_audio job → Verify worker calls RemuxWithAudioDownmix
- Create transcode job → Verify worker calls TranscodeToDASH

---

## Phase 2: Frontend Integration (5-7 hours)

### Phase 2.1: Transcode Status Hook (1 hour)

**Objective:** Create polling hook to monitor transcode progress.

#### Task 2.1.1: Create useTranscodeStatus Hook (1 hour)
- **File:** `web/src/lib/hooks/useTranscodeStatus.ts` (new file)
- **Implementation:**
  ```typescript
  export function useTranscodeStatus(
    mediaId: number,
    quality: string,
    enabled: boolean
  ) {
    return useQuery({
      queryKey: ['transcode-status', mediaId, quality],
      queryFn: async () => {
        const response = await fetch(`/api/media/${mediaId}/transcode/${quality}`)
        if (!response.ok) throw new Error('Failed to fetch status')
        return response.json()
      },
      refetchInterval: enabled ? 2000 : false,
      enabled: enabled,
    })
  }
  ```
- **Priority:** HIGH
- **Export:** Add to `web/src/lib/hooks/index.ts`

**Dependencies:** None (can start immediately)
**Deliverable:** Working hook that polls transcode status every 2 seconds
**Test Plan:**
- Enable polling → Verify API calls every 2 seconds
- Disable polling → Verify no API calls

---

### Phase 2.2: MediaDetailsModal DASH Integration (3-4 hours)

**Objective:** Switch from direct streaming to DASH manifest handling with auto-fallback.

#### Task 2.2.1: Add Quality Selection Helper (30 min)
- **File:** `web/src/lib/utils/quality.ts` (new file)
- **Add:**
  ```typescript
  export function selectBestQuality(media: Media): string {
    const maxHeight = Math.min(media.height || 720, window.screen.height)
    if (maxHeight >= 2160) return '4k'
    if (maxHeight >= 1080) return '1080p'
    if (maxHeight >= 720) return '720p'
    return '360p'
  }
  ```
- **Priority:** MEDIUM

#### Task 2.2.2: Replace handlePlay Logic (2-3 hours)
- **File:** `web/src/components/media/MediaCard/MediaDetailsModal.tsx`
- **Major Changes:**
  1. Build DASH manifest URL instead of direct stream URL
  2. Fetch manifest with `fetch(manifestUrl, { redirect: 'manual' })`
  3. Handle 302 redirect (Direct Play) → Use direct stream URL
  4. Handle 202 Accepted (Processing) → Start polling, show progress UI
  5. Handle 200 OK (Cache hit) → Play DASH immediately
  6. Handle errors → Fall back to direct stream
- **State additions:**
  ```typescript
  const [transcodeStatus, setTranscodeStatus] = useState<'idle' | 'processing' | 'ready'>('idle')
  const [transcodeType, setTranscodeType] = useState<'remux' | 'remux_audio' | 'transcode'>()
  const [streamUrl, setStreamUrl] = useState<string>()
  ```
- **Priority:** CRITICAL

#### Task 2.2.3: Add Adaptive Timeout Logic (30 min)
- **File:** `web/src/components/media/MediaCard/MediaDetailsModal.tsx`
- **Timeouts:**
  - Remux: 2 minutes (120000ms)
  - Remux + Audio: 10 minutes (600000ms)
  - Transcode: 5 minutes (300000ms)
- **On timeout:** Call `fallbackToDirectStream()`
- **Priority:** HIGH

#### Task 2.2.4: Integrate useTranscodeStatus Hook (30 min)
- **File:** `web/src/components/media/MediaCard/MediaDetailsModal.tsx`
- **Logic:**
  - Enable polling when status is 'processing'
  - When status becomes 'completed' → Clear timeout, set streamUrl to manifest
  - When status becomes 'failed' → Fall back to direct stream
- **Priority:** HIGH

**Dependencies:** Phase 2.1 (status hook), Backend Phase 1.4 (auto-trigger)
**Deliverable:** MediaDetailsModal that uses DASH with intelligent fallback
**Test Plan:**
- Click Play on MP4 H.264/AAC → Should play instantly (Direct Play)
- Click Play on MKV H.264/AAC stereo → Should show "Preparing video (very fast)..." and play within 5 min
- Click Play on MKV H.264/5.1 → Should show "Preparing with audio conversion..." and play within 10 min
- Click Play on HEVC → Should show "Converting video..." and timeout after 5 min to direct stream
- Simulate network error → Should fall back to direct stream

---

### Phase 2.3: Progress UI (1-2 hours)

**Objective:** Add visual feedback during transcoding with different messaging per job type.

#### Task 2.3.1: Create Progress Component (1-1.5 hours)
- **File:** `web/src/components/media/MediaCard/MediaDetailsModal.tsx`
- **UI Elements:**
  - Progress bar (use existing Progress component)
  - Status message (varies by job type)
  - Percentage complete
  - Estimated time remaining
  - Queue position (if in queue)
- **Messages:**
  - Remux: "Preparing video (very fast)... Usually 2-5 minutes"
  - Remux Audio: "Preparing video with audio conversion... Usually 5-10 minutes"
  - Transcode: "Converting video (this may take a while)... About X minutes remaining"
- **Priority:** HIGH

#### Task 2.3.2: Add Loading States (30 min)
- **States:**
  - "Checking compatibility..." (while fetching manifest)
  - "In queue - Position X of Y" (if job is queued)
  - Progress bar with percentage (while processing)
  - "Ready to play!" (when complete, before autoplay)
- **Priority:** MEDIUM

**Dependencies:** Phase 2.2 (DASH integration)
**Deliverable:** Polished progress UI with clear messaging
**Test Plan:**
- Start remux → Verify "Preparing video (very fast)..." message
- Start transcode → Verify "Converting video..." message
- Check progress bar updates every 2 seconds
- Verify queue position shown when multiple jobs queued

---

## Phase 3: Configuration & Polish (1-2 hours)

### Phase 3.1: Environment Configuration (30 min)

**Objective:** Make all settings configurable via environment variables.

#### Task 3.1.1: Add Configuration Variables (30 min)
- **File:** `config/config.go` or equivalent
- **Variables:**
  ```bash
  # Output directory
  TRANSCODE_OUTPUT_DIR=./data/transcode

  # Worker settings
  TRANSCODE_WORKER_COUNT=2
  TRANSCODE_MAX_QUEUE_SIZE=10

  # Video encoding
  TRANSCODE_VIDEO_PRESET=medium
  TRANSCODE_VIDEO_CRF=23

  # Audio encoding
  TRANSCODE_AUDIO_BITRATE=192k

  # Hardware acceleration
  TRANSCODE_ENABLE_HARDWARE=true
  TRANSCODE_FALLBACK_TO_SOFTWARE=true
  ```
- **Priority:** MEDIUM

**Dependencies:** None
**Deliverable:** Configurable transcode settings
**Test Plan:**
- Change TRANSCODE_OUTPUT_DIR → Verify files written to new location
- Change TRANSCODE_VIDEO_PRESET → Verify FFmpeg uses new preset

---

### Phase 3.2: Error Handling & Edge Cases (30 min-1 hour)

**Objective:** Handle edge cases gracefully.

#### Task 3.2.1: Disk Space Pre-Check (15 min)
- **File:** `internal/infrastructure/transcoding/validation.go`
- **Add:** `checkDiskSpace()` function
- **Logic:** Require 10GB free OR 2x estimated file size (whichever is larger)
- **Return:** 507 Insufficient Storage if disk full
- **Priority:** MEDIUM

#### Task 3.2.2: Concurrent Request Handling (15 min)
- **File:** `internal/api/handlers/transcode.go`
- **Logic:** Database UNIQUE constraint handles this automatically
- **Test:** Multiple users request same manifest simultaneously
- **Priority:** HIGH

#### Task 3.2.3: Source File Change Detection (15 min)
- **File:** `internal/application/transcode/create_job.go`
- **Logic:** Compare source file mtime with job created_at
- **If stale:** Delete old transcode, create new job
- **Priority:** LOW (future enhancement)

#### Task 3.2.4: Frontend Error Boundaries (15 min)
- **File:** `web/src/components/media/MediaCard/MediaDetailsModal.tsx`
- **Add:** Try-catch blocks around manifest fetch
- **Fallback:** Always fall back to direct stream on error
- **Priority:** HIGH

**Dependencies:** All previous phases
**Deliverable:** Robust error handling
**Test Plan:**
- Fill disk → Verify 507 error returned
- Delete source file during transcode → Verify job fails gracefully
- Network timeout → Verify fallback to direct stream

---

### Phase 3.3: API Documentation Updates (30 min)

**Objective:** Update OpenAPI/Swagger docs with new response formats.

#### Task 3.3.1: Update API Docs (30 min)
- **File:** `internal/api/handlers/transcode.go`
- **Updates:**
  - Document 202 Accepted response for ServeManifest
  - Document 302 redirect for Direct Play
  - Document new `type` field in TranscodeJobResponse
  - Update examples with new response formats
- **Priority:** LOW (documentation only)

**Dependencies:** None (can run anytime)
**Deliverable:** Updated API documentation

---

## Phase 4: Testing & Validation (2 hours)

### Phase 4.1: Integration Testing (1 hour)

**Test Matrix:**

| Media Type | Container | Video Codec | Audio | Expected Strategy | Expected Time |
|------------|-----------|-------------|-------|-------------------|---------------|
| Movie 1    | MP4       | H.264       | AAC Stereo | Direct Play | <1 sec |
| Movie 2    | MKV       | H.264       | AAC Stereo | Remux | 2-5 min |
| Movie 3    | MKV       | H.264       | AAC 5.1 | Remux + Audio | 5-10 min |
| Movie 4    | MKV       | HEVC        | AC3 5.1 | Transcode | 20-60 min |
| Movie 5    | AVI       | MPEG-4 Part 2 | MP3 | Transcode | 20-60 min |

**Test Cases:**

#### Test 4.1.1: Direct Play Path (10 min)
- **Setup:** MP4 file with H.264/AAC stereo
- **Steps:**
  1. Click Play
  2. Verify 302 redirect to `/api/stream/:id`
  3. Verify video plays immediately
  4. Verify no transcode job created
- **Expected:** Instant playback, no processing

#### Test 4.1.2: Remux Path (Stereo) (10 min)
- **Setup:** MKV file with H.264/AAC stereo
- **Steps:**
  1. Click Play
  2. Verify 202 Accepted response
  3. Verify "Preparing video (very fast)..." message
  4. Wait for completion
  5. Verify manifest exists
  6. Verify playback starts
  7. Click Play again → Should play instantly (cached)
- **Expected:** Processing time 2-5 minutes, then instant playback on repeat

#### Test 4.1.3: Remux + Audio Downmix Path (10 min)
- **Setup:** MKV file with H.264/AAC 5.1 surround
- **Steps:**
  1. Click Play
  2. Verify 202 Accepted response with type='remux_audio'
  3. Verify "Preparing video with audio conversion..." message
  4. Wait for completion
  5. Verify audio is stereo (check manifest or ffprobe output)
  6. Verify playback starts
- **Expected:** Processing time 5-10 minutes, audio properly downmixed

#### Test 4.1.4: Transcode Path (15 min)
- **Setup:** MKV file with HEVC/AC3 5.1
- **Steps:**
  1. Click Play
  2. Verify 202 Accepted response with type='transcode'
  3. Verify "Converting video..." message
  4. Wait 5 minutes (timeout)
  5. Verify fallback to direct stream
  6. Let transcode complete in background
  7. Click Play again → Should use DASH manifest
- **Expected:** Timeout after 5 min, fallback works, cached playback after completion

#### Test 4.1.5: Queue Management (10 min)
- **Setup:** Create multiple transcode jobs
- **Steps:**
  1. Start 3 transcode jobs simultaneously
  2. Verify only 2 run concurrently (worker limit)
  3. Verify 3rd job shows queue position
  4. Wait for 1st job to complete
  5. Verify 3rd job starts processing
- **Expected:** Queue works correctly, position updates

#### Test 4.1.6: Error Handling (5 min)
- **Setup:** Invalid media ID, missing source file
- **Steps:**
  1. Request manifest for invalid media ID → 404
  2. Delete source file, create job → Job fails
  3. Verify frontend falls back to direct stream
- **Expected:** Graceful error handling

---

### Phase 4.2: Performance Testing (30 min)

#### Test 4.2.1: Measure Remux Performance (15 min)
- **Test:** Remux 1-hour 1080p MKV H.264/AAC stereo
- **Expected:** <5 minutes
- **Record:** Actual time, file sizes (input vs output)

#### Test 4.2.2: Measure Remux + Audio Performance (15 min)
- **Test:** Remux + downmix 1-hour 1080p MKV H.264/AAC 5.1
- **Expected:** 5-10 minutes
- **Record:** Actual time, verify audio channels reduced

---

### Phase 4.3: Browser Compatibility (30 min)

**Browsers to Test:**
- Chrome (latest)
- Firefox (latest)
- Safari (desktop)
- Safari iOS (mobile)

**Test Cases:**
- DASH playback works
- Quality selection works
- Seeking works
- Progress tracking works
- Fallback to direct stream works

---

## MVP Scope Definition

### MVP (Must Have) - 10-14 hours

**Backend:**
- [x] Strategy selection logic (Direct Play, Remux, Remux+Audio, Transcode)
- [x] Audio channel detection
- [x] Remux executors (RemuxToDASH, RemuxWithAudioDownmix)
- [x] Database migration for job type
- [x] ServeManifest auto-trigger logic
- [x] Worker integration for all job types
- [x] Output directory restructuring (dash/ subdirectory)

**Frontend:**
- [x] useTranscodeStatus hook
- [x] MediaDetailsModal DASH integration
- [x] Progress UI with job type-specific messaging
- [x] Adaptive timeout (2/10/5 min)
- [x] Fallback to direct stream
- [x] Quality auto-selection

**Testing:**
- [x] Integration tests for all 4 strategies
- [x] Queue management verification
- [x] Error handling tests

### Post-MVP (Nice to Have) - 4-6 hours

**Not included in MVP, defer to future iterations:**
- [ ] Hardware acceleration detection (NVENC, QSV, etc.)
- [ ] Priority queue system
- [ ] Queue full behavior (reject vs bump)
- [ ] Job cancellation support
- [ ] Disk space management (LRU cleanup, retention policies)
- [ ] Admin panel for cache management
- [ ] Monitoring/analytics (queue stats, success rates)
- [ ] Multiple audio track support
- [ ] Subtitle/caption extraction
- [ ] Progressive playback (start with partial manifest)
- [ ] Source file change invalidation
- [ ] Per-media storage limits

---

## Critical Path & Dependencies

```
Phase 1.1 (Strategy Selection)
    ↓
Phase 1.2 (Remux Executors) + Phase 1.3 (Database)
    ↓
Phase 1.4 (ServeManifest Auto-Trigger)
    ↓
Phase 1.5 (Worker Integration)
    ↓
Phase 2.1 (Status Hook) → Phase 2.2 (DASH Integration) → Phase 2.3 (Progress UI)
    ↓
Phase 3 (Configuration & Polish)
    ↓
Phase 4 (Testing)
```

**Critical Path Tasks:**
1. Task 1.1.1: Audio channel detection (30 min)
2. Task 1.1.2: Strategy selection (1 hour)
3. Task 1.2.1: RemuxToDASH (45 min)
4. Task 1.2.2: RemuxWithAudioDownmix (1 hour)
5. Task 1.3: Database migration (1 hour)
6. Task 1.4.2: ServeManifest auto-trigger (1.5 hours)
7. Task 1.5.1: Worker integration (30 min)
8. Task 2.1.1: useTranscodeStatus hook (1 hour)
9. Task 2.2.2: MediaDetailsModal DASH integration (2-3 hours)
10. Task 2.3.1: Progress UI (1-1.5 hours)

**Total Critical Path Time:** 10-13 hours

---

## Risk Assessment & Mitigation

### High Risk

**Risk 1: Audio Downmix Quality Issues**
- **Impact:** Users may experience poor audio quality or missing channels
- **Probability:** Medium
- **Mitigation:**
  - Test with multiple 5.1/7.1 sources
  - Use industry-standard FFmpeg downmix filter
  - Provide option to fall back to direct stream if audio issues detected
- **Owner:** Backend developer

**Risk 2: Frontend Timeout Calibration**
- **Impact:** Timeout too short = unnecessary fallback, too long = poor UX
- **Probability:** Medium
- **Mitigation:**
  - Conservative initial values (2/10/5 min)
  - Make timeouts configurable
  - Add telemetry to measure actual transcode times
  - Adjust timeouts based on real-world data
- **Owner:** Frontend developer

**Risk 3: Race Conditions in Job Creation**
- **Impact:** Duplicate jobs created, wasted resources
- **Probability:** Low
- **Mitigation:**
  - Database UNIQUE constraint on (media_id, quality)
  - Handle duplicate errors gracefully
  - Test concurrent requests
- **Owner:** Backend developer

### Medium Risk

**Risk 4: Disk Space Exhaustion**
- **Impact:** Transcode jobs fail, service degraded
- **Probability:** Medium (depends on library size)
- **Mitigation:**
  - Pre-check disk space before starting job
  - Return 507 Insufficient Storage
  - Frontend falls back to direct stream
  - Document manual cleanup procedures
  - Post-MVP: Implement automatic cleanup
- **Owner:** Backend developer

**Risk 5: Browser Compatibility Issues**
- **Impact:** DASH playback fails on some browsers
- **Probability:** Low (Shaka Player handles most compatibility)
- **Mitigation:**
  - Test on all major browsers
  - Fallback to direct stream on DASH errors
  - Use conservative DASH settings (mp4 segments, not WebM)
- **Owner:** Frontend developer

### Low Risk

**Risk 6: FFmpeg Process Leaks**
- **Impact:** Memory/process leaks on server restart
- **Probability:** Low (already handled in existing code)
- **Mitigation:**
  - Existing process manager handles cleanup
  - Server restart marks processing jobs as failed
  - No additional work needed
- **Owner:** N/A (already handled)

---

## Rollout Plan

### Stage 1: Development (Week 1)
- Complete all Phase 1-3 tasks
- Run Phase 4.1 integration tests
- Fix any critical bugs

### Stage 2: Internal Testing (Week 2)
- Deploy to staging/test environment
- Test with real media library
- Measure actual performance vs estimates
- Gather feedback from team

### Stage 3: Beta Release (Week 3)
- Deploy to production with feature flag
- Enable for limited users
- Monitor queue performance
- Gather user feedback on UX

### Stage 4: General Availability (Week 4)
- Enable for all users
- Monitor error rates, fallback rates
- Adjust timeouts based on telemetry
- Document user-facing features

### Stage 5: Post-Launch Iteration (Ongoing)
- Implement post-MVP features based on user feedback
- Add hardware acceleration if needed
- Implement disk space management
- Build admin panel for cache management

---

## Success Criteria

### Backend Success Criteria
- [x] Strategy selection correctly identifies all 4 strategies
- [x] Remux completes in <5 minutes for 1-hour 1080p video
- [x] Remux + Audio completes in <10 minutes for 1-hour 1080p video with 5.1 audio
- [x] Audio downmix produces valid stereo output
- [x] ServeManifest returns 202 when job created
- [x] ServeManifest returns 200 when manifest exists
- [x] ServeManifest returns 302 for Direct Play
- [x] Worker processes all three job types
- [x] No duplicate jobs created under concurrent requests
- [x] Output files written to dash/ subdirectory

### Frontend Success Criteria
- [x] MP4 H.264/AAC plays instantly (Direct Play)
- [x] Progress UI shows different messages for remux/remux_audio/transcode
- [x] Polling updates progress every 2 seconds
- [x] Timeout triggers fallback to direct stream
- [x] Cached manifest plays instantly on second view
- [x] Error states gracefully fall back to direct stream
- [x] Quality auto-selection picks appropriate level
- [x] VideoPlayer works with both DASH and direct stream URLs

### UX Success Criteria
- [x] User sees clear messaging about what's happening
- [x] User can always watch video (via fallback if needed)
- [x] Progress bar shows realistic progress
- [x] No confusing error messages
- [x] Second playback is instant (cached)

### Performance Success Criteria
- [x] Direct Play: <1 second to playback
- [x] Remux (stereo): <5 minutes to playback
- [x] Remux (multi-channel): <10 minutes to playback
- [x] Transcode fallback: <5 minutes timeout
- [x] Queue handles 2 concurrent jobs
- [x] API response time <100ms (except initial manifest request)

---

## Task Checklist (Copy to Issue Tracker)

### Backend Tasks (5-7 hours)

**Phase 1.1: Strategy Selection (2-3 hours)**
- [ ] 1.1.1: Add AudioChannels to VideoInfo and parse from ffprobe (30 min)
- [ ] 1.1.2: Implement DetermineStreamStrategy() function (1 hour)
- [ ] 1.1.3: Add EstimateTranscodeTime() helper (30 min)

**Phase 1.2: Remux Executors (2 hours)**
- [ ] 1.2.1: Implement RemuxToDASH() executor (45 min)
- [ ] 1.2.2: Implement RemuxWithAudioDownmix() executor (1 hour)
- [ ] 1.2.3: Update executor interface (15 min)

**Phase 1.3: Database Updates (1 hour)**
- [ ] 1.3.1: Create migration 000006_add_job_type.up.sql (15 min)
- [ ] 1.3.2: Add Type field to TranscodeJob domain model (15 min)
- [ ] 1.3.3: Update SQLC queries and regenerate (15 min)
- [ ] 1.3.4: Add Type to TranscodeJobResponse (15 min)

**Phase 1.4: ServeManifest Auto-Trigger (1.5-2 hours)**
- [ ] 1.4.1: Update output paths to use dash/ subdirectory (30 min)
- [ ] 1.4.2: Implement auto-trigger logic in ServeManifest (1-1.5 hours)
- [ ] 1.4.3: Handle race conditions with UNIQUE constraint (30 min)

**Phase 1.5: Worker Integration (30 min)**
- [ ] 1.5.1: Update worker to call correct executor based on job type (30 min)

### Frontend Tasks (5-7 hours)

**Phase 2.1: Status Hook (1 hour)**
- [ ] 2.1.1: Create useTranscodeStatus hook (1 hour)

**Phase 2.2: DASH Integration (3-4 hours)**
- [ ] 2.2.1: Add selectBestQuality() helper (30 min)
- [ ] 2.2.2: Replace handlePlay() logic with DASH manifest handling (2-3 hours)
- [ ] 2.2.3: Add adaptive timeout logic (30 min)
- [ ] 2.2.4: Integrate useTranscodeStatus hook (30 min)

**Phase 2.3: Progress UI (1-2 hours)**
- [ ] 2.3.1: Create progress component with job type-specific messaging (1-1.5 hours)
- [ ] 2.3.2: Add loading states (30 min)

### Configuration & Polish (1-2 hours)

**Phase 3.1: Configuration (30 min)**
- [ ] 3.1.1: Add environment variables for all settings (30 min)

**Phase 3.2: Error Handling (30 min-1 hour)**
- [ ] 3.2.1: Add disk space pre-check (15 min)
- [ ] 3.2.2: Test concurrent request handling (15 min)
- [ ] 3.2.3: Add source file change detection (15 min) [OPTIONAL]
- [ ] 3.2.4: Add frontend error boundaries (15 min)

**Phase 3.3: Documentation (30 min)**
- [ ] 3.3.1: Update API documentation (30 min)

### Testing (2 hours)

**Phase 4.1: Integration Testing (1 hour)**
- [ ] 4.1.1: Test Direct Play path (10 min)
- [ ] 4.1.2: Test Remux path (stereo) (10 min)
- [ ] 4.1.3: Test Remux + Audio Downmix path (10 min)
- [ ] 4.1.4: Test Transcode path (15 min)
- [ ] 4.1.5: Test queue management (10 min)
- [ ] 4.1.6: Test error handling (5 min)

**Phase 4.2: Performance Testing (30 min)**
- [ ] 4.2.1: Measure remux performance (15 min)
- [ ] 4.2.2: Measure remux + audio performance (15 min)

**Phase 4.3: Browser Compatibility (30 min)**
- [ ] 4.3.1: Test Chrome (5 min)
- [ ] 4.3.2: Test Firefox (5 min)
- [ ] 4.3.3: Test Safari desktop (10 min)
- [ ] 4.3.4: Test Safari iOS (10 min)

---

## Additional Notes

### Estimated vs Actual Time Tracking

After completing each task, record actual time taken to improve future estimates:

| Task | Estimated | Actual | Variance | Notes |
|------|-----------|--------|----------|-------|
| 1.1.1 | 30 min | ? | ? | |
| 1.1.2 | 1 hour | ? | ? | |
| ... | ... | ... | ... | |

### Known Limitations (MVP)

1. **No hardware acceleration detection** - Uses software encoding only (slower but more compatible)
2. **No automatic cleanup** - Transcodes remain on disk indefinitely (manual cleanup required)
3. **No priority queue** - First-in-first-out only
4. **No multiple audio tracks** - Uses first audio track only
5. **No subtitle support** - Subtitles not extracted or displayed
6. **No progressive playback** - Must wait for complete manifest
7. **Fixed quality selection** - Auto-selects based on resolution, cannot override before play
8. **No admin panel** - No UI for managing transcodes or queue

### Future Enhancements (Post-MVP)

See ADR Section: "Future Enhancements (Deferred)" for full list. Priority order:
1. Hardware acceleration (3-5x speed improvement)
2. Automatic disk space management (prevents disk full)
3. Priority queue (better multi-user experience)
4. Admin panel (easier cache management)
5. Multiple audio tracks (international content)
6. Subtitle support (accessibility)

---

## Contact & Ownership

**Project Owner:** [Your Name]
**Backend Lead:** [Name]
**Frontend Lead:** [Name]
**QA Lead:** [Name]

**Sprint Duration:** 2 weeks
**Target Completion:** [Date]
**Review Date:** [Date]

---

## Appendix A: File Paths Reference

**Backend Files:**
- `/home/fictional/Projects/viewra2/internal/infrastructure/transcoding/validation.go`
- `/home/fictional/Projects/viewra2/internal/infrastructure/transcoding/ffmpeg.go`
- `/home/fictional/Projects/viewra2/internal/infrastructure/transcoding/executor.go`
- `/home/fictional/Projects/viewra2/internal/api/handlers/transcode.go`
- `/home/fictional/Projects/viewra2/internal/application/transcode/queue.go`
- `/home/fictional/Projects/viewra2/internal/application/transcode/create_job.go`
- `/home/fictional/Projects/viewra2/internal/domain/transcode/transcode_job.go`
- `/home/fictional/Projects/viewra2/migrations/postgres/000006_add_job_type.up.sql`

**Frontend Files:**
- `/home/fictional/Projects/viewra2/web/src/lib/hooks/useTranscodeStatus.ts` (new)
- `/home/fictional/Projects/viewra2/web/src/lib/utils/quality.ts` (new)
- `/home/fictional/Projects/viewra2/web/src/components/media/MediaCard/MediaDetailsModal.tsx`
- `/home/fictional/Projects/viewra2/web/src/components/media/VideoPlayer/VideoPlayer.tsx`

**Configuration Files:**
- `/home/fictional/Projects/viewra2/.env` or equivalent config file

---

## Appendix B: API Response Examples

### 202 Accepted (Job Created - Remux)
```json
{
  "message": "remux in progress",
  "type": "remux",
  "status": "queued",
  "progress": 0,
  "status_url": "/api/media/123/transcode/1080p",
  "estimated_time": "2-5 minutes",
  "retry_after": 2
}
```

### 202 Accepted (Job Created - Transcode)
```json
{
  "message": "transcode in progress",
  "type": "transcode",
  "status": "processing",
  "progress": 35,
  "status_url": "/api/media/123/transcode/1080p",
  "estimated_time": "15 minutes remaining",
  "retry_after": 2
}
```

### 302 Found (Direct Play)
```
HTTP/1.1 302 Found
Location: /api/stream/123
```

### 200 OK (Manifest Ready)
```xml
<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" ...>
  <!-- DASH manifest content -->
</MPD>
```

---

**Document Version:** 1.0
**Last Updated:** 2025-11-13
**Status:** Ready for Implementation
