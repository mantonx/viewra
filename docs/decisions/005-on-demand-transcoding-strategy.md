# ADR 005: On-Demand Transcoding Strategy

## Status

Proposed

## Context

ViewRA v2 currently has a complete DASH transcoding infrastructure designed for manual job management, but users expect Plex/Jellyfin-style on-demand transcoding where:

1. User clicks "Play" button
2. System automatically checks if transcoding is needed
3. If needed, transcoding starts immediately
4. Playback begins as soon as first segments are available (progressive)
5. No manual job queue management

### Current Infrastructure

**Backend:**
- Worker pool with 2 concurrent transcode jobs
- DASH transcoding with FFmpeg (4-second segments)
- Quality profiles: 360p, 720p, 1080p, 4K
- Hardware acceleration (VAAPI, NVENC, QSV, VideoToolbox)
- Job status tracking: queued, processing, completed, failed
- Codec detection (`ShouldTranscode` in validation.go)
- Endpoints:
  - `POST /api/media/:id/transcode` - Create job
  - `GET /api/media/:id/transcode/:quality` - Get job status
  - `GET /api/media/:id/dash/:quality/manifest.mpd` - Serve manifest
  - `GET /api/media/:id/dash/:quality/:filename` - Serve segments

**Frontend:**
- Shaka Player for DASH playback
- VideoPlayer component with quality selection
- MediaDetailsModal with Play/Resume buttons
- Progress tracking (useProgressUpdater)

### Problem Statement

The current design requires users to:
1. Manually trigger transcoding
2. Wait for completion
3. Then play the video

This is a **batch processing model**, not an **on-demand streaming model**.

## Decision

We will implement **hybrid on-demand transcoding** that:

1. **Intelligent Quality Selection** - Auto-select best quality based on source resolution
2. **Automatic Job Creation** - Transparently create transcode jobs on Play
3. **Progressive Playback** - Start playback as soon as manifest + initial segments exist
4. **Fallback Strategy** - Direct stream if transcode fails or is too slow
5. **Cache Management** - Keep completed transcodes for future playback

### Key Design Decisions

#### 1. When to Trigger Transcoding

**Decision:** On manifest request (`GET /api/media/:id/dash/:quality/manifest.mpd`)

**Rationale:**
- Manifest request is the first step in DASH playback
- Allows quality to be determined before starting transcode
- Frontend can request different qualities without extra logic
- Natural chokepoint for transcode logic

**Flow:**
```
User clicks Play → Frontend requests manifest → Backend checks:
  ├─ Manifest exists? → Return immediately
  ├─ Job in progress? → Wait and poll (or stream partial)
  └─ No job? → Create job + return 202 Accepted with progress URL
```

#### 2. How to Detect if Transcoding is Needed

**Decision:** Check three conditions in order:

1. **Manifest exists on disk** → Skip transcoding, serve existing
2. **Job exists in database** → Check status (completed/processing/failed)
3. **Source codec/format analysis** → Use existing `ShouldTranscode()` logic

#### 3. Progressive Playback Strategy

**Decision:** Start playback as soon as manifest + first 3 segments exist (~12 seconds)

**Rationale:**
- DASH segments are 4 seconds each
- 3 segments = 12 seconds of buffer
- Shaka Player can start with partial manifest
- User sees playback within ~5-15 seconds for typical videos

#### 4. Fallback Behavior

**Decision:** Multi-tier fallback strategy

**Fallback Order:**
1. **Primary:** DASH with selected quality (if transcode succeeds)
2. **Secondary:** Direct HTTP stream (if transcode fails/timeout)
3. **Tertiary:** Lower quality DASH (if available)
4. **Error:** Show error message with retry option

**Scenarios:**

| Scenario | Behavior |
|----------|----------|
| Transcode completes quickly | Play DASH at selected quality |
| Transcode in progress (0-25%) | Show progress, wait up to 2 minutes |
| Transcode in progress (25%+) | Start playing partial DASH (progressive) |
| Transcode timeout (>2 min) | Fall back to direct stream |
| Transcode fails | Fall back to direct stream |
| Source already compatible | Redirect to direct stream immediately |
| Disk full | Fail gracefully, direct stream |
| FFmpeg error | Fail gracefully, direct stream |

#### 5. Quality Selection Strategy

**Decision:** Automatic quality selection with manual override

**Auto-Selection Logic:**
```typescript
function selectBestQuality(media: Media): string {
  const sourceHeight = media.height || 0
  const devicePixelRatio = window.devicePixelRatio || 1
  const screenHeight = window.screen.height * devicePixelRatio

  // Don't upscale
  const maxHeight = Math.min(sourceHeight, screenHeight)

  if (maxHeight >= 2160) return '4k'
  if (maxHeight >= 1080) return '1080p'
  if (maxHeight >= 720) return '720p'
  return '360p'
}
```

**Manual Override:**
- User can select quality from dropdown in VideoPlayer
- Quality preference stored in localStorage
- Respects user's bandwidth/performance preference

#### 6. Cache Management

**Decision:** Aggressive caching with manual cleanup

**Caching Strategy:**
- **Keep indefinitely:** All completed transcodes
- **Delete on demand:** Admin panel "Clear Transcodes" button
- **Delete on update:** If source file changes (mtime check)
- **Disk space monitor:** Warn at 90% full, auto-delete oldest at 95%

**Why Aggressive Caching:**
- Transcoding is expensive (CPU/time)
- Storage is cheap compared to CPU time
- Users typically watch same content multiple times
- Can always regenerate if needed

## Implementation Plan

### Phase 1: Backend Enhancements (Week 1)

- **Task 1.1:** Modify manifest endpoint for on-demand triggering
- **Task 1.2:** Add quality recommendation endpoint
- **Task 1.3:** Add transcode priority field

### Phase 2: Frontend Components (Week 1-2)

- **Task 2.1:** Create TranscodingIndicator component
- **Task 2.2:** Create useTranscodeStatus hook
- **Task 2.3:** Create useQualitySelection hook
- **Task 2.4:** Enhance MediaDetailsModal
- **Task 2.5:** Update VideoPlayer component

### Phase 3: Testing & Edge Cases (Week 2)

- **Task 3.1:** Backend integration tests
- **Task 3.2:** Frontend integration tests
- **Task 3.3:** End-to-end tests

### Phase 4: Admin Panel (Week 3)

- **Task 4.1:** Create Transcodes admin page
- **Task 4.2:** Add transcode cleanup endpoints

### Phase 5: Documentation & Optimization (Week 3-4)

- **Task 5.1:** Update API documentation
- **Task 5.2:** Create user guide
- **Task 5.3:** Add transcode analytics
- **Task 5.4:** Optimize worker pool

See `docs/ON_DEMAND_TRANSCODING_TASKS.md` for detailed implementation tasks.

## Edge Cases & Error Scenarios

### 1. Concurrent Play Requests
**Solution:** Each quality is independent; database prevents duplicates with UNIQUE constraint

### 2. Transcode Interrupted
**Solution:** Mark stale jobs as failed on restart; user retry triggers new job

### 3. Disk Full
**Solution:** Pre-check disk space; auto-delete oldest transcodes; fall back to direct stream

### 4. Source File Deleted
**Solution:** Serve existing transcode; mark media unavailable; next scan removes

### 5. Transcode Never Starts
**Solution:** Show queue position; 5-minute timeout → direct stream fallback

### 6. Network Interruption
**Solution:** Shaka Player auto-reconnects; progress saved every 10 seconds

## Performance Targets

| Metric | Target |
|--------|--------|
| Time to manifest (cached) | <500ms |
| Time to manifest (new job) | <5s |
| Time to first segment | <30s |
| Time to playback start | <30s |
| Transcode 1080p (1hr movie) | <30min |
| Concurrent users | 10+ |

## Success Metrics

| Metric | Target |
|--------|--------|
| User satisfaction | >90% immediate playback |
| Transcode success rate | >95% |
| Cache hit rate | >70% after 1 week |
| Fallback usage | <10% |
| Average time to playback | <15 seconds |

## Risks & Mitigations

### Risk 1: Worker Pool Overload
**Mitigation:** Monitor utilization, queue limits, priority system, throttling

### Risk 2: Disk Space Exhaustion
**Mitigation:** Pre-check space, auto-cleanup LRU, admin alerts, direct stream fallback

### Risk 3: Poor UX During Wait
**Mitigation:** Clear progress indicator, time estimates, immediate direct stream option, progressive playback

### Risk 4: Database Bottleneck
**Mitigation:** Proper indexes, in-memory caching (5s TTL), connection pooling, optimized polling

## Feature Flag

**Environment Variable:** `ENABLE_ON_DEMAND_TRANSCODE`

**Rollout Plan:**
1. Deploy with flag OFF (safe default)
2. Test in staging with flag ON
3. Enable for internal users (beta)
4. Enable for 10% of users (canary)
5. Enable for all users (100%)

## Rollback Plan

If critical issues arise:
1. Set `ENABLE_ON_DEMAND_TRANSCODE=false`
2. Restart services
3. System reverts to manual transcoding
4. No data loss (jobs remain in database)

## References

- [Plex Transcoding Documentation](https://support.plex.tv/articles/200250387-streaming-media-direct-play-and-direct-stream/)
- [Jellyfin Transcoding](https://jellyfin.org/docs/general/server/transcoding.html)
- [DASH Specification](https://dashif.org/docs/)
- [Shaka Player Documentation](https://shaka-player-demo.appspot.com/docs/api/tutorial-welcome.html)

## Revision History

- 2025-11-13: Initial version - On-demand transcoding design
