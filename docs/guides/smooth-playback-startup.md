# Smooth Playback Startup for On-Demand Transcoding

## Problem Statement

When playing HDR content that requires tone mapping, playback stutters at startup because:

1. **Manifest returns too early**: `WaitForManifest()` returns after only 1 segment exists
2. **HLS.js requests multiple segments**: The player immediately requests segments 0, 1, 2, 3...
3. **FFmpeg can't keep up**: 4K HDR tone mapping encodes slower than real-time (~0.3-0.5x speed)
4. **Buffer starvation**: Player exhausts available segments faster than they're generated

### Root Cause

The fundamental mismatch: **playback consumes segments faster than transcoding produces them**.

For HDR content with tone mapping:
- 4K transcoding: ~0.3-0.5x real-time (each 2s segment takes 4-6s to encode)
- 1080p transcoding: ~1.5-2x real-time (each 2s segment takes 1-1.3s to encode)
- 720p transcoding: ~3-4x real-time

## Current Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           REQUEST FLOW                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   Browser                    Backend                      FFmpeg         │
│   ───────                    ───────                      ──────         │
│                                                                          │
│   1. GET /master.m3u8  ────► Create session                              │
│                              Start FFmpeg ─────────────► Begin encode    │
│                              WaitForManifest()                           │
│                              (waits for 1 segment)       Generate seg_0  │
│   ◄──────────────────────── Return manifest                              │
│                                                                          │
│   2. GET /playlist.m3u8 ───► Return variant playlist                     │
│                                                                          │
│   3. GET /seg_000000.ts ───► WaitForSegment(0)                           │
│   ◄──────────────────────── Return segment ◄─────────── seg_0 ready     │
│                                                                          │
│   4. GET /seg_000001.ts ───► WaitForSegment(1)           Encoding seg_1  │
│      (BLOCKS - waiting)      (waiting...)                (still working) │
│   ◄──────────────────────── Return segment ◄─────────── seg_1 ready     │
│                                                                          │
│   5. Buffer stall!           Player starved              Can't keep up   │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Key Files

| File | Purpose |
|------|---------|
| `internal/infrastructure/transcoding/session/watcher.go` | `WaitForManifest()`, `WaitForSegment()` |
| `internal/application/transcode/serve_manifest.go` | Orchestrates session creation |
| `internal/infrastructure/transcoding/profile/adaptive.go` | ABR ladder definition |
| `internal/infrastructure/ffmpeg/hls/filters.go` | Tone mapping filter chains |
| `web/src/lib/hooks/useHlsPlayer.ts` | HLS.js configuration and ABR control |

## Solution Options

### Option 1: Server-Side Prebuffering

**Concept**: Wait for N segments before returning the manifest.

**Implementation**:
```go
// In WaitForManifest()
const MinSegmentsBeforeStart = 3  // Wait for segments 0, 1, 2

func (s *TranscodeSession) WaitForManifest(timeout time.Duration) error {
    // ... existing manifest check ...

    // Wait for minimum segments
    for i := 0; i < MinSegmentsBeforeStart; i++ {
        if _, err := s.WaitForSegment(firstSegmentNum + i, remainingTimeout); err != nil {
            return err
        }
    }
    return nil
}
```

**Pros**:
- Simple to implement
- Guarantees buffer exists before playback starts
- Works with any player configuration

**Cons**:
- Adds 4-8 seconds to initial startup time
- Fixed delay regardless of encode speed
- Poor UX for content that encodes quickly (non-HDR)

**Startup Time Impact**: +4-8 seconds (3 segments × 1.5-2.5s each for 4K HDR)

---

### Option 2: Adaptive Prebuffering Based on Encode Speed

**Concept**: Measure FFmpeg's encoding speed and prebuffer accordingly.

**Implementation**:
```go
type TranscodeSession struct {
    // ... existing fields ...
    encodeSpeedRatio float64  // e.g., 0.5 means encoding at 0.5x real-time
}

func (s *TranscodeSession) WaitForManifest(timeout time.Duration) error {
    // Wait for first segment and measure time
    startTime := time.Now()
    if _, err := s.WaitForSegment(firstSegmentNum, timeout); err != nil {
        return err
    }
    elapsed := time.Since(startTime)

    // Calculate encode speed (segment duration / encode time)
    s.encodeSpeedRatio = float64(s.SegmentDurationSec) / elapsed.Seconds()

    // Determine prebuffer count based on speed
    prebufferCount := 1
    if s.encodeSpeedRatio < 1.0 {
        // Encoding slower than real-time - need more prebuffer
        prebufferCount = int(math.Ceil(1.0 / s.encodeSpeedRatio)) + 1
    }

    // Wait for additional segments
    for i := 1; i < prebufferCount; i++ {
        s.WaitForSegment(firstSegmentNum + i, timeout)
    }
    return nil
}
```

**Pros**:
- Adapts to actual encoding performance
- Fast content starts quickly, slow content gets appropriate buffer
- Single source of truth (server decides)

**Cons**:
- First segment measurement may not be representative
- Still adds delay for slow content
- Complexity in measuring speed accurately

**Startup Time Impact**: Varies - fast content: +0s, slow content: +4-10s

---

### Option 3: Start Low, Scale Up (Current Approach - Needs Refinement)

**Concept**: Start at a resolution that encodes faster than real-time, then scale up.

**Current Implementation** (in `useHlsPlayer.ts`):
```typescript
const FAST_START_CONFIG = {
  TARGET_HEIGHT: 1080,
  BUFFER_THRESHOLD_FOR_UPSCALE: 12,
  MIN_TIME_BEFORE_UPSCALE: 10000,
}

// Cap ABR to 1080p initially
hls.autoLevelCapping = capLevel
// Release cap when buffer is healthy
if (bufferLevel >= BUFFER_THRESHOLD_FOR_UPSCALE) {
  hls.autoLevelCapping = -1
}
```

**Problem with Current Implementation**:
- Picks highest bitrate at 1080p (60Mbps) which is still slow to encode
- Even 1080p-60m causes buffer stalls with tone mapping
- ABR cap release triggers switch to 4K before 4K segments exist

**Refined Implementation**:
```typescript
const FAST_START_CONFIG = {
  TARGET_HEIGHT: 720,           // Lower starting point
  TARGET_BITRATE: 4_000_000,    // Pick moderate bitrate, not highest
  BUFFER_THRESHOLD_FOR_UPSCALE: 15,
  MIN_TIME_BEFORE_UPSCALE: 12000,
}

// Find level: prefer moderate bitrate at target height
const findFastStartLevel = (levels, targetHeight, targetBitrate) => {
  const candidates = levels.filter(l => l.height <= targetHeight)
  // Sort by distance from target bitrate
  candidates.sort((a, b) =>
    Math.abs(a.bitrate - targetBitrate) - Math.abs(b.bitrate - targetBitrate)
  )
  return candidates[0]?.index ?? 0
}
```

**Pros**:
- Instant startup (no additional delay)
- Progressive quality improvement
- Good UX - video starts immediately

**Cons**:
- Quality transition can be jarring
- Complex state management for ABR control
- Doesn't guarantee 4K segments will be ready when switching

**Startup Time Impact**: +0s (immediate playback at lower quality)

---

### Option 4: Server Signals Readiness

**Concept**: Server includes segment availability in manifest or separate endpoint.

**Implementation A - Custom HLS Tag**:
```
#EXTM3U
#EXT-X-VERSION:6
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-VIEWRA-SEGMENTS-READY:3
#EXT-X-VIEWRA-ENCODE-SPEED:0.45
#EXTINF:2.0,
seg_000000.ts
#EXTINF:2.0,
seg_000001.ts
...
```

**Implementation B - Readiness Endpoint**:
```typescript
// Frontend polls readiness before starting
const checkReadiness = async (sessionId: string) => {
  const response = await fetch(`/api/transcode/${sessionId}/readiness`)
  const { segmentsReady, encodeSpeed, estimatedBufferSeconds } = await response.json()
  return estimatedBufferSeconds >= 6  // 3 segments worth
}

// In player setup
while (!await checkReadiness(sessionId)) {
  await sleep(500)
  showLoadingSpinner()
}
startPlayback()
```

**Pros**:
- Frontend knows exactly when to start
- Can show accurate loading progress
- Server has authoritative information

**Cons**:
- Requires API changes
- Additional network round-trips
- Custom HLS tags may confuse some players

**Startup Time Impact**: Variable, but with accurate progress indication

---

### Option 5: Segment Probing (Frontend)

**Concept**: Frontend probes segment availability before starting playback.

**Implementation**:
```typescript
const probeSegments = async (baseUrl: string, count: number): Promise<boolean> => {
  const probes = Array.from({ length: count }, (_, i) =>
    fetch(`${baseUrl}/seg_${String(i).padStart(6, '0')}.ts`, { method: 'HEAD' })
      .then(r => r.ok)
      .catch(() => false)
  )
  const results = await Promise.all(probes)
  return results.every(Boolean)
}

// Before starting HLS.js
const MIN_SEGMENTS = 3
while (!await probeSegments(streamUrl, MIN_SEGMENTS)) {
  await sleep(500)
  updateLoadingProgress()
}
hls.loadSource(streamUrl)
```

**Pros**:
- No backend changes required
- Works with standard HLS
- Simple implementation

**Cons**:
- Additional HTTP requests (HEAD requests)
- Doesn't know encode speed
- Race condition: segment could be partially written

**Startup Time Impact**: Same as server prebuffering, but detected client-side

---

### Option 6: Hybrid - Adaptive Quality with Server Hints

**Concept**: Combine adaptive quality with server-provided encode speed hints.

**Implementation**:

Backend adds encode speed to master playlist:
```go
// In serve_master_playlist.go
func (uc *ServeMasterPlaylistUseCase) Execute(...) {
    // Calculate estimated encode speeds based on profile + content
    for _, variant := range variants {
        encodeSpeed := estimateEncodeSpeed(videoInfo, variant.Profile)
        variant.EncodeSpeedHint = encodeSpeed
    }
}
```

Master playlist:
```
#EXTM3U
#EXT-X-VERSION:6

#EXT-X-STREAM-INF:BANDWIDTH=4000000,RESOLUTION=1280x720
#EXT-X-VIEWRA-ENCODE-SPEED:3.5
720p-4m/playlist.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=20000000,RESOLUTION=1920x1080
#EXT-X-VIEWRA-ENCODE-SPEED:1.2
1080p-20m/playlist.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=60000000,RESOLUTION=3840x2160
#EXT-X-VIEWRA-ENCODE-SPEED:0.4
4k-60m/playlist.m3u8
```

Frontend uses hints for intelligent ABR:
```typescript
hls.on(Hls.Events.MANIFEST_PARSED, () => {
  // Parse encode speed hints from levels
  const encodeSpeedByLevel = parseEncodeSpeedHints(hls.levels)

  // Find highest quality that encodes >= 1.0x real-time
  const safeStartLevel = levels.findIndex(l =>
    encodeSpeedByLevel[l.index] >= 1.0
  )

  // Start at safe level, allow upscale when buffer is healthy
  hls.startLevel = safeStartLevel
  hls.autoLevelCapping = safeStartLevel
})
```

**Pros**:
- Best of both worlds: fast start + eventual high quality
- Server provides authoritative encode speed estimates
- Frontend makes intelligent decisions

**Cons**:
- Requires backend changes
- Custom HLS extensions
- Encode speed estimates may be inaccurate

**Startup Time Impact**: +0s (starts at quality that encodes fast enough)

---

## Recommended Approach

> **Preferred Solution: Adaptive Prebuffering Based on Encode Speed**
>
> After evaluating all options, **adaptive server-side prebuffering based on measured encode speed** provides the best balance of reliability, smoothness, and user experience. This approach:
>
> - Guarantees zero buffer stalls (most reliable)
> - Adapts wait time to actual encode performance (fast content starts quickly)
> - Starts at full requested quality (no jarring quality transitions)
> - Provides accurate progress feedback to users

### The Math Behind Adaptive Prebuffering

The key insight is that buffer drains at a predictable rate based on encode speed:

```text
If encode_speed = 0.4x (4K HDR typical):
  - Playback consumes: 1 second of buffer per second
  - Encoding produces: 0.4 seconds of buffer per second
  - Net drain rate: 1 - 0.4 = 0.6 seconds per second

To sustain 10 seconds of stutter-free playback:
  initial_buffer = safe_duration × (1 - encode_speed)
  initial_buffer = 10s × 0.6 = 6 seconds
  segments_needed = ceil(6s / 2s) = 3 segments
```

**General formula:**

```text
initial_buffer_seconds = safe_playback_duration × (1 - encode_speed)
segments_to_prebuffer = ceil(initial_buffer_seconds / segment_duration)
```

### Expected Wait Times by Content Type

| Content Type | Encode Speed | Drain Rate | Buffer Needed | Segments | Est. Wait |
|--------------|--------------|------------|---------------|----------|-----------|
| 720p SDR     | 3.0x         | -2.0 (gaining) | 0s         | 1        | ~0.7s     |
| 1080p SDR    | 1.5x         | -0.5 (gaining) | 0s         | 1        | ~1.3s     |
| 1080p HDR    | 1.0x         | 0 (even)   | 0s            | 1        | ~2s       |
| 1080p-60m HDR| 0.7x         | 0.3        | 3s            | 2-3      | ~6-9s     |
| 4K HDR       | 0.4x         | 0.6        | 6s            | 4        | ~20s      |
| 4K-100m HDR  | 0.3x         | 0.7        | 7s            | 5        | ~33s      |

### Implementation

#### Backend: Dynamic Prebuffer Calculation

```go
// In watcher.go

const (
    // How long we want guaranteed stutter-free playback after start
    SafePlaybackDuration = 10.0 // seconds

    // Bounds to prevent edge cases
    MinPrebufferSegments = 1
    MaxPrebufferSegments = 8
)

func (s *TranscodeSession) WaitForManifest(timeout time.Duration) error {
    deadline := time.Now().Add(timeout)

    // Wait for first segment and measure encode speed
    firstSegStart := time.Now()
    if _, err := s.waitForSegmentInternal(s.firstSegmentNum, timeout); err != nil {
        return err
    }
    firstSegTime := time.Since(firstSegStart)

    // Calculate encode speed from first segment
    segmentDuration := float64(s.SegmentDurationSec)
    encodeSpeed := segmentDuration / firstSegTime.Seconds()
    s.EncodeSpeed = encodeSpeed

    // Calculate required prebuffer based on encode speed
    prebufferSegments := s.calculatePrebufferSegments(encodeSpeed)

    s.logger.Info("Calculated prebuffer requirement",
        "encode_speed", fmt.Sprintf("%.2fx", encodeSpeed),
        "prebuffer_segments", prebufferSegments,
        "estimated_wait_seconds", float64(prebufferSegments-1) * firstSegTime.Seconds())

    // Wait for additional segments
    for i := 1; i < prebufferSegments; i++ {
        remaining := time.Until(deadline)
        if remaining <= 0 {
            s.logger.Warn("Prebuffer timeout, starting with partial buffer",
                "segments_ready", i,
                "segments_wanted", prebufferSegments)
            break
        }

        if _, err := s.waitForSegmentInternal(s.firstSegmentNum+i, remaining); err != nil {
            s.logger.Warn("Failed to prebuffer segment, continuing",
                "segment", s.firstSegmentNum+i,
                "error", err)
            break
        }
    }

    return nil
}

func (s *TranscodeSession) calculatePrebufferSegments(encodeSpeed float64) int {
    // If encoding faster than real-time, minimal prebuffer needed
    if encodeSpeed >= 1.0 {
        return MinPrebufferSegments
    }

    // Calculate buffer needed to sustain SafePlaybackDuration
    drainRate := 1.0 - encodeSpeed
    requiredBufferSeconds := SafePlaybackDuration * drainRate

    // Convert to segments (add 1 for the segment we already waited for)
    segmentDuration := float64(s.SegmentDurationSec)
    prebufferSegments := int(math.Ceil(requiredBufferSeconds/segmentDuration)) + 1

    // Clamp to reasonable bounds
    if prebufferSegments < MinPrebufferSegments {
        return MinPrebufferSegments
    }
    if prebufferSegments > MaxPrebufferSegments {
        return MaxPrebufferSegments
    }

    return prebufferSegments
}
```

#### Frontend: SSE Progress Stream

Instead of polling, use Server-Sent Events for real-time progress updates. The frontend shows the standard video buffering spinner - no new UI needed.

```go
// GET /api/stream/{mediaId}/progress?quality=4k-60m (SSE endpoint)

type ProgressEvent struct {
    Type             string  `json:"type"`              // "progress" | "ready" | "error"
    SegmentsReady    int     `json:"segments_ready"`
    SegmentsRequired int     `json:"segments_required"`
    EncodeSpeed      float64 `json:"encode_speed"`
    ProgressPercent  int     `json:"progress_percent"`
}

func (h *StreamHandler) StreamProgress(w http.ResponseWriter, r *http.Request) {
    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }

    // Get or create session
    session := h.getOrCreateSession(r)

    // Send progress updates as segments are generated
    for {
        select {
        case <-r.Context().Done():
            return
        case <-session.SegmentReady:
            event := ProgressEvent{
                Type:             "progress",
                SegmentsReady:    session.GetSegmentCount(),
                SegmentsRequired: session.GetPrebufferRequirement(),
                EncodeSpeed:      session.EncodeSpeed,
                ProgressPercent:  session.GetProgressPercent(),
            }

            if event.SegmentsReady >= event.SegmentsRequired {
                event.Type = "ready"
            }

            data, _ := json.Marshal(event)
            fmt.Fprintf(w, "data: %s\n\n", data)
            flusher.Flush()

            if event.Type == "ready" {
                return
            }
        }
    }
}
```

```typescript
// Frontend: wait for readiness via SSE, show standard buffering spinner
const waitForReadiness = (mediaId: number, quality: string): Promise<void> => {
  return new Promise((resolve, reject) => {
    const eventSource = new EventSource(
      `/api/stream/${mediaId}/progress?quality=${quality}`
    )

    eventSource.onmessage = (event) => {
      const data: ProgressEvent = JSON.parse(event.data)

      if (data.type === 'ready') {
        eventSource.close()
        resolve()
      }
      // Progress updates happen automatically - video element shows
      // native buffering spinner while waiting
    }

    eventSource.onerror = () => {
      eventSource.close()
      reject(new Error('Stream preparation failed'))
    }
  })
}

// Usage: video shows buffering spinner naturally while we wait
const video = videoRef.current
video.src = ''  // Clear to show loading state
await waitForReadiness(mediaId, selectedQuality)
hls.loadSource(streamUrl)  // Now guaranteed to have enough buffer
```

**Key insight**: The video element's native buffering/loading spinner handles the UI automatically. No custom loading UI needed - the player just shows its normal "loading" state until the stream is ready.

### Why This Approach Wins

| Criteria | Adaptive Prebuffer | Start Low + Scale Up | Fixed Prebuffer |
|----------|-------------------|---------------------|-----------------|
| Zero stalls guaranteed | ✅ Yes | ❌ Risk of stalls | ✅ Yes |
| Fast content starts fast | ✅ ~1-2s | ✅ Instant | ❌ Fixed delay |
| Full quality from start | ✅ Yes | ❌ Starts blurry | ✅ Yes |
| No quality jumping | ✅ Yes | ❌ Jarring transitions | ✅ Yes |
| Accurate progress UI | ✅ Yes | ❌ Unpredictable | ⚠️ Estimated |
| Simple implementation | ⚠️ Moderate | ❌ Complex ABR logic | ✅ Simple |

### Alternative Phases (If Needed)

#### Phase 2: Encode Speed Hints in Playlist

For cases where we want to avoid the first-segment measurement delay, add estimated encode speeds to the master playlist:

```m3u8
#EXT-X-STREAM-INF:BANDWIDTH=60000000,RESOLUTION=3840x2160
#EXT-X-VIEWRA-ENCODE-SPEED:0.4
4k-60m/playlist.m3u8
```

This allows the frontend to show an estimated wait time immediately.

#### Phase 3: User Preference

Add a user setting: "Prioritize smooth playback" (default) vs "Start immediately"

- **Smooth playback**: Uses adaptive prebuffering (recommended)
- **Start immediately**: Uses the "start low, scale up" approach for users who prefer instant playback over quality

## Encode Speed Estimation

Rough benchmarks for NVENC with tone mapping:

| Resolution | Bitrate | Encode Speed (HDR→SDR) | Encode Speed (SDR) |
|------------|---------|------------------------|---------------------|
| 720p       | 4 Mbps  | ~3.5x                  | ~6x                 |
| 1080p      | 10 Mbps | ~1.8x                  | ~4x                 |
| 1080p      | 20 Mbps | ~1.2x                  | ~3x                 |
| 1080p      | 60 Mbps | ~0.7x                  | ~2x                 |
| 4K         | 25 Mbps | ~0.5x                  | ~1.5x               |
| 4K         | 60 Mbps | ~0.35x                 | ~1.0x               |

**Key insight**: For HDR content, 720p-4m is the safest starting point that guarantees real-time encoding.

## Implementation Checklist

### Phase 1 Tasks
- [ ] Update `findFastStartLevel()` to pick 720p-4m (moderate quality)
- [ ] Implement gradual quality stepping (720 → 1080 → 4K)
- [ ] Increase buffer thresholds before each step
- [ ] Add logging for quality transitions
- [ ] Test with various HDR content

### Phase 2 Tasks
- [ ] Add `EstimatedEncodeSpeed` to `AdaptiveProfile`
- [ ] Create encode speed estimation function
- [ ] Add custom HLS tag to master playlist
- [ ] Parse encode speed hints in frontend
- [ ] Use hints for intelligent ABR capping

### Phase 3 Tasks
- [ ] Add user preference for startup mode
- [ ] Implement adaptive prebuffering in `WaitForManifest()`
- [ ] Add segment progress endpoint
- [ ] Show loading progress in UI

## Testing Scenarios

1. **4K HDR content** (worst case): Should start at 720p, gradually improve
2. **1080p SDR content**: Should start at 1080p, quickly reach full quality
3. **720p content**: No quality ramping needed
4. **Seeking in 4K HDR**: Should handle mid-stream quality changes
5. **Network fluctuation**: ABR should handle bandwidth changes without stutter

## Telemetry & Performance Verification

To make data-driven decisions about playback optimization, we need comprehensive telemetry from both the frontend (player) and backend (transcoder).

### Key Metrics

| Metric | Source | Description | Target |
|--------|--------|-------------|--------|
| Time to First Frame (TTFF) | Frontend | Time from play request to first frame rendered | < 2s |
| Time to Stable Playback | Frontend | Time until no buffer stalls for 10s | < 5s |
| Buffer Stall Count | Frontend | Number of `bufferStalledError` events | 0 in first 60s |
| Buffer Stall Duration | Frontend | Total seconds spent stalled | < 1s in first 60s |
| Time to Max Quality | Frontend | Time from play to reaching highest quality | < 30s |
| Quality Level Distribution | Frontend | % time at each quality level | Track over session |
| Encode Speed Ratio | Backend | Segment duration / encode time | > 1.0x |
| Segment Wait Time | Backend | Time client waited for segment | < 500ms avg |
| Rebuffer Ratio | Frontend | Stall time / total playback time | < 1% |

### Frontend Telemetry Implementation

#### HLS.js Event Listeners

```typescript
// In useHlsPlayer.ts or a dedicated telemetry hook

interface PlaybackMetrics {
  sessionId: string
  mediaId: number

  // Timing
  playRequestedAt: number
  firstFrameAt: number | null
  stablePlaybackAt: number | null
  maxQualityReachedAt: number | null

  // Buffer health
  bufferStalls: Array<{ timestamp: number; duration: number; bufferLevel: number }>
  currentBufferLevel: number
  minBufferLevel: number

  // Quality
  qualityChanges: Array<{ timestamp: number; fromLevel: number; toLevel: number; reason: string }>
  qualityDistribution: Map<number, number>  // level -> seconds at level

  // Network
  fragmentLoadTimes: Array<{ segment: number; durationMs: number; bytes: number }>
  estimatedBandwidth: number
}

const setupTelemetry = (hls: Hls, metrics: PlaybackMetrics) => {
  const video = hls.media as HTMLVideoElement

  // Time to first frame
  video.addEventListener('playing', () => {
    if (!metrics.firstFrameAt) {
      metrics.firstFrameAt = Date.now()
      console.log(`[Telemetry] TTFF: ${metrics.firstFrameAt - metrics.playRequestedAt}ms`)
    }
  }, { once: true })

  // Buffer stalls
  let stallStart: number | null = null

  hls.on(Hls.Events.ERROR, (_, data) => {
    if (data.details === 'bufferStalledError') {
      stallStart = Date.now()
      metrics.bufferStalls.push({
        timestamp: stallStart,
        duration: 0,
        bufferLevel: getBufferLevel(video),
      })
    }
  })

  video.addEventListener('playing', () => {
    if (stallStart) {
      const duration = Date.now() - stallStart
      const lastStall = metrics.bufferStalls[metrics.bufferStalls.length - 1]
      if (lastStall) lastStall.duration = duration
      stallStart = null

      // Check for stable playback (10s without stalls)
      checkStablePlayback(metrics)
    }
  })

  // Quality changes
  hls.on(Hls.Events.LEVEL_SWITCHED, (_, data) => {
    const prevLevel = metrics.qualityChanges.length > 0
      ? metrics.qualityChanges[metrics.qualityChanges.length - 1].toLevel
      : -1

    metrics.qualityChanges.push({
      timestamp: Date.now(),
      fromLevel: prevLevel,
      toLevel: data.level,
      reason: hls.autoLevelEnabled ? 'abr' : 'manual',
    })

    // Check if max quality reached
    const maxLevel = hls.levels.length - 1
    if (data.level === maxLevel && !metrics.maxQualityReachedAt) {
      metrics.maxQualityReachedAt = Date.now()
      console.log(`[Telemetry] Time to max quality: ${metrics.maxQualityReachedAt - metrics.playRequestedAt}ms`)
    }
  })

  // Fragment load times (for bandwidth estimation)
  hls.on(Hls.Events.FRAG_LOADED, (_, data) => {
    const stats = data.frag.stats
    if (stats.loading) {
      metrics.fragmentLoadTimes.push({
        segment: data.frag.sn as number,
        durationMs: stats.loading.end - stats.loading.start,
        bytes: stats.total,
      })
    }
  })

  // Buffer level monitoring (every second)
  const bufferMonitor = setInterval(() => {
    const level = getBufferLevel(video)
    metrics.currentBufferLevel = level
    metrics.minBufferLevel = Math.min(metrics.minBufferLevel, level)

    // Track quality distribution
    const currentLevel = hls.currentLevel
    const existing = metrics.qualityDistribution.get(currentLevel) || 0
    metrics.qualityDistribution.set(currentLevel, existing + 1)
  }, 1000)

  return () => clearInterval(bufferMonitor)
}

const getBufferLevel = (video: HTMLVideoElement): number => {
  if (video.buffered.length === 0) return 0
  return video.buffered.end(video.buffered.length - 1) - video.currentTime
}

const checkStablePlayback = (metrics: PlaybackMetrics) => {
  if (metrics.stablePlaybackAt) return

  const now = Date.now()
  const recentStalls = metrics.bufferStalls.filter(s => now - s.timestamp < 10000)

  if (recentStalls.length === 0 && metrics.firstFrameAt) {
    metrics.stablePlaybackAt = now
    console.log(`[Telemetry] Stable playback: ${metrics.stablePlaybackAt - metrics.playRequestedAt}ms`)
  }
}
```

#### Telemetry Report Generation

```typescript
const generatePlaybackReport = (metrics: PlaybackMetrics): PlaybackReport => {
  const sessionDuration = Date.now() - metrics.playRequestedAt
  const totalStallTime = metrics.bufferStalls.reduce((sum, s) => sum + s.duration, 0)

  return {
    // Core timing metrics
    timeToFirstFrame: metrics.firstFrameAt
      ? metrics.firstFrameAt - metrics.playRequestedAt
      : null,
    timeToStablePlayback: metrics.stablePlaybackAt
      ? metrics.stablePlaybackAt - metrics.playRequestedAt
      : null,
    timeToMaxQuality: metrics.maxQualityReachedAt
      ? metrics.maxQualityReachedAt - metrics.playRequestedAt
      : null,

    // Buffer health
    bufferStallCount: metrics.bufferStalls.length,
    totalStallDuration: totalStallTime,
    rebufferRatio: totalStallTime / sessionDuration,
    minBufferLevel: metrics.minBufferLevel,

    // Quality metrics
    qualityChangeCount: metrics.qualityChanges.length,
    qualityDistribution: Object.fromEntries(metrics.qualityDistribution),
    averageQualityLevel: calculateWeightedAverageQuality(metrics.qualityDistribution),

    // Network metrics
    averageFragmentLoadTime: average(metrics.fragmentLoadTimes.map(f => f.durationMs)),
    estimatedBandwidth: calculateBandwidth(metrics.fragmentLoadTimes),

    // Session info
    sessionDuration,
    mediaId: metrics.mediaId,
  }
}
```

### Backend Telemetry Implementation

#### Transcode Session Metrics

```go
// In session/session.go

type TranscodeMetrics struct {
    SessionID        string
    MediaID          int64
    Profile          string

    // Timing
    SessionCreatedAt time.Time
    FFmpegStartedAt  time.Time
    FirstSegmentAt   time.Time

    // Encode performance
    SegmentEncodeTimes []SegmentTiming  // Per-segment encode times
    AverageEncodeSpeed float64          // Updated rolling average

    // Client wait times
    SegmentWaitTimes []SegmentWait     // Time clients waited for segments

    // Resource usage
    PeakMemoryMB     int
    AverageCPUPercent float64
}

type SegmentTiming struct {
    SegmentNum  int
    StartTime   time.Time
    EndTime     time.Time
    DurationMs  int64
    EncodeSpeed float64  // segment_duration / encode_time
}

type SegmentWait struct {
    SegmentNum int
    WaitTimeMs int64
    ClientIP   string
}

// Track segment generation timing
func (s *TranscodeSession) recordSegmentGenerated(segNum int) {
    now := time.Now()

    s.metricsMutex.Lock()
    defer s.metricsMutex.Unlock()

    // Calculate encode time (time since last segment or FFmpeg start)
    var encodeTime time.Duration
    if len(s.metrics.SegmentEncodeTimes) == 0 {
        encodeTime = now.Sub(s.FFmpegStartedAt)
    } else {
        lastTiming := s.metrics.SegmentEncodeTimes[len(s.metrics.SegmentEncodeTimes)-1]
        encodeTime = now.Sub(lastTiming.EndTime)
    }

    segmentDuration := time.Duration(s.SegmentDurationSec) * time.Second
    encodeSpeed := float64(segmentDuration) / float64(encodeTime)

    s.metrics.SegmentEncodeTimes = append(s.metrics.SegmentEncodeTimes, SegmentTiming{
        SegmentNum:  segNum,
        StartTime:   now.Add(-encodeTime),
        EndTime:     now,
        DurationMs:  encodeTime.Milliseconds(),
        EncodeSpeed: encodeSpeed,
    })

    // Update rolling average
    s.metrics.AverageEncodeSpeed = s.calculateRollingAverageSpeed()

    // Log if encoding slower than real-time
    if encodeSpeed < 1.0 {
        s.logger.Warn("Encoding slower than real-time",
            "segment", segNum,
            "encode_speed", fmt.Sprintf("%.2fx", encodeSpeed),
            "encode_time_ms", encodeTime.Milliseconds())
    }
}

// Track client wait times in WaitForSegment
func (s *TranscodeSession) WaitForSegment(segmentNum int, timeout time.Duration) (string, error) {
    waitStart := time.Now()
    defer func() {
        waitTime := time.Since(waitStart)
        s.recordSegmentWait(segmentNum, waitTime)
    }()

    // ... existing implementation ...
}

func (s *TranscodeSession) recordSegmentWait(segNum int, waitTime time.Duration) {
    s.metricsMutex.Lock()
    defer s.metricsMutex.Unlock()

    s.metrics.SegmentWaitTimes = append(s.metrics.SegmentWaitTimes, SegmentWait{
        SegmentNum: segNum,
        WaitTimeMs: waitTime.Milliseconds(),
    })

    // Log if client waited too long
    if waitTime > 500*time.Millisecond {
        s.logger.Warn("Client waited for segment",
            "segment", segNum,
            "wait_time_ms", waitTime.Milliseconds())
    }
}
```

#### Metrics API Endpoint

```go
// GET /api/transcode/{sessionId}/metrics
func (h *TranscodeHandler) GetSessionMetrics(w http.ResponseWriter, r *http.Request) {
    sessionID := chi.URLParam(r, "sessionId")

    session := h.sessionManager.GetSession(sessionID)
    if session == nil {
        http.Error(w, "Session not found", http.StatusNotFound)
        return
    }

    metrics := session.GetMetrics()

    response := map[string]interface{}{
        "session_id":           metrics.SessionID,
        "media_id":             metrics.MediaID,
        "profile":              metrics.Profile,
        "average_encode_speed": metrics.AverageEncodeSpeed,
        "segments_generated":   len(metrics.SegmentEncodeTimes),
        "average_wait_time_ms": calculateAverageWait(metrics.SegmentWaitTimes),
        "encode_speeds":        extractEncodeSpeeds(metrics.SegmentEncodeTimes),
    }

    json.NewEncoder(w).Encode(response)
}
```

### Telemetry Dashboard

For visualizing and analyzing playback performance:

```typescript
// Example dashboard data structure
interface DashboardData {
  // Aggregate metrics (last 24h)
  summary: {
    totalPlaybackSessions: number
    averageTTFF: number
    averageTimeToStable: number
    averageRebufferRatio: number
    sessionsWith0Stalls: number  // percentage
  }

  // Per-content-type breakdown
  byContentType: {
    '4k_hdr': ContentTypeMetrics
    '4k_sdr': ContentTypeMetrics
    '1080p_hdr': ContentTypeMetrics
    '1080p_sdr': ContentTypeMetrics
  }

  // Time series for trends
  timeSeries: {
    timestamp: number
    avgTTFF: number
    avgStalls: number
    avgEncodeSpeed: number
  }[]

  // Worst performing sessions (for debugging)
  problematicSessions: {
    sessionId: string
    mediaId: number
    stallCount: number
    rebufferRatio: number
    minEncodeSpeed: number
  }[]
}
```

### Performance Verification Checklist

Before and after making changes, verify these metrics:

#### Automated Tests

```typescript
// playwright or cypress test
describe('Playback Performance', () => {
  it('should achieve TTFF < 2s for 4K HDR content', async () => {
    const metrics = await playAndMeasure(HDR_4K_MEDIA_ID)
    expect(metrics.timeToFirstFrame).toBeLessThan(2000)
  })

  it('should have 0 buffer stalls in first 60s', async () => {
    const metrics = await playAndMeasure(HDR_4K_MEDIA_ID, { duration: 60000 })
    expect(metrics.bufferStallCount).toBe(0)
  })

  it('should reach max quality within 30s', async () => {
    const metrics = await playAndMeasure(HDR_4K_MEDIA_ID, { duration: 35000 })
    expect(metrics.timeToMaxQuality).toBeLessThan(30000)
  })

  it('should maintain rebuffer ratio < 1%', async () => {
    const metrics = await playAndMeasure(HDR_4K_MEDIA_ID, { duration: 120000 })
    expect(metrics.rebufferRatio).toBeLessThan(0.01)
  })
})
```

#### Manual Testing Protocol

1. **Baseline Measurement** (before changes)
   - Play 3 different 4K HDR files
   - Record TTFF, stall count, time to max quality
   - Save console logs and network timeline

2. **After Changes**
   - Repeat same 3 files
   - Compare metrics against baseline
   - Verify no regressions

3. **Edge Cases**
   - Test seeking to middle of video
   - Test network throttling (3G, slow 4G)
   - Test rapid quality switches

#### Logging for Debugging

```typescript
// Add structured logging for debugging sessions
const logPlaybackEvent = (event: string, data: Record<string, unknown>) => {
  console.log(JSON.stringify({
    type: 'playback_telemetry',
    event,
    timestamp: Date.now(),
    ...data,
  }))
}

// Usage
logPlaybackEvent('first_frame', { ttff: 1234, quality: '720p' })
logPlaybackEvent('buffer_stall', { bufferLevel: 0.5, currentQuality: '1080p' })
logPlaybackEvent('quality_change', { from: '720p', to: '1080p', reason: 'abr' })
```

### Decision Framework

Use telemetry data to guide implementation decisions:

```text
┌─────────────────────────────────────────────────────────────────┐
│                    DECISION FRAMEWORK                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  IF average_encode_speed < 1.0x for quality X:                  │
│    → Don't start at quality X                                    │
│    → Cap ABR below quality X until buffer > threshold            │
│                                                                  │
│  IF TTFF > 2s:                                                   │
│    → Consider lower starting quality                             │
│    → Check if prebuffering is too aggressive                     │
│                                                                  │
│  IF stall_count > 0 in first 60s:                               │
│    → Increase buffer thresholds before quality upgrade           │
│    → Check if quality upgrade is happening too soon              │
│                                                                  │
│  IF time_to_max_quality > 30s:                                  │
│    → Check if buffer thresholds are too conservative             │
│    → Consider faster quality stepping                            │
│                                                                  │
│  IF rebuffer_ratio > 1%:                                        │
│    → Fundamental encode speed issue                              │
│    → Need lower starting quality or server prebuffering          │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### A/B Testing Strategy

When testing different approaches:

```typescript
// Feature flags for A/B testing
const PLAYBACK_STRATEGY = {
  CONTROL: 'control',           // Current behavior
  FAST_START_720: 'fast_720',   // Start at 720p
  FAST_START_480: 'fast_480',   // Start at 480p
  PREBUFFER_3: 'prebuffer_3',   // Wait for 3 segments
} as const

// Assign strategy based on user/session
const getPlaybackStrategy = (userId: string): string => {
  const hash = simpleHash(userId)
  const strategies = Object.values(PLAYBACK_STRATEGY)
  return strategies[hash % strategies.length]
}

// Track strategy in metrics
const metrics: PlaybackMetrics = {
  strategy: getPlaybackStrategy(userId),
  // ... other metrics
}

// Analyze results by strategy
// SELECT strategy, AVG(ttff), AVG(stall_count), AVG(rebuffer_ratio)
// FROM playback_metrics
// GROUP BY strategy
```

## Metrics to Track

Summary of all metrics for tracking playback performance:

### Frontend (Player)

- Time to first frame (TTFF)
- Time to stable playback (no stalls for 10s)
- Time to reach maximum quality
- Buffer stall count and duration
- Rebuffer ratio (stall time / playback time)
- Quality level distribution
- Minimum buffer level observed
- Fragment load times

### Backend (Transcoder)

- Encode speed ratio per segment
- Average encode speed (rolling)
- Client segment wait times
- Time from session creation to first segment
- Resource usage (CPU, memory)
