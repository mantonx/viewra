# ADR 016: Seek Position Transcoding for HLS Progressive Streaming

## Status

**Proposed**

## Context

ViewRA v2 currently implements HLS progressive transcoding that starts from the beginning of the video and sequentially transcodes segments. This creates a significant UX limitation: **users cannot seek to positions that haven't been transcoded yet**.

### Current User Experience Problem

When a user tries to seek forward in a video:

1. **Expected Behavior**: Click on scrubber → video seeks to that position → playback starts (with brief buffering if needed)
2. **Actual Behavior**: Click on scrubber → video seeks inconsistently:
   - Sometimes jumps to exact position (if segments already transcoded)
   - Sometimes jumps only a few minutes ahead (to last transcoded segment)
   - Sometimes jumps 10+ minutes ahead (unpredictable)
   - User cannot reliably seek to any position they want

This is a **critical UX flaw** that makes the video player feel broken and unprofessional compared to industry standards.

### Current Implementation

**HLS Progressive Transcoding Flow:**

```
User clicks Play (t=00:00)
  ↓
Check if HLS manifest exists
  ↓
If not: Start transcode job
  ↓
Transcode segments sequentially:
  - segment_000.ts (00:00 - 00:04)
  - segment_001.ts (00:04 - 00:08)
  - segment_002.ts (00:08 - 00:12)
  - ... continues until end of file
  ↓
Browser can only seek within transcoded range
```

**User tries to seek to t=30:00:**
- If segments 0-450 are transcoded (00:00 - 30:00) → ✅ Seek works
- If only segments 0-200 are transcoded (00:00 - 13:20) → ❌ Seek limited to t=13:20

### Why This Happens

HLS.js (and all HLS players) can only seek to segments listed in the HLS manifest. The browser has no concept of "transcode this segment on demand" - it simply sees what's available and seeks to the nearest available segment.

Our current manifest generation creates a continuous sequence starting from segment 0, so the player can only seek within that continuous range.

### Industry Standard Behavior

**Standard approach:**
1. User seeks to any position
2. Server receives seek position via query parameter
3. Server starts transcoding from that position
4. Manifest reflects discontinuous segment availability
5. Player buffers briefly and starts playback from seek position

**Example HLS Manifest with Discontinuity:**
```m3u8
#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:4

# Initial playback from start
#EXTINF:4.0,
segment_000.ts
#EXTINF:4.0,
segment_001.ts

# User sought to 30:00 (segment 450)
#EXT-X-DISCONTINUITY
#EXTINF:4.0,
segment_450.ts
#EXTINF:4.0,
segment_451.ts
```

## Decision

Implement **on-demand transcoding from seek position** with the following architecture:

### 1. Frontend Changes

**Capture Seek Events:**

Update `VideoPlayer.tsx` to detect when user seeks ahead of transcoded range and request transcoding from that position.

```typescript
const handleSeek = async (seekTime: number) => {
  // Check if seek position is beyond transcoded segments
  const isSeekAheadOfTranscoding = seekTime > getLastTranscodedTime()

  if (isSeekAheadOfTranscoding) {
    // Request transcoding from seek position
    await api.requestTranscodeFromPosition(mediaId, seekTime)

    // Show buffering indicator
    setBuffering(true)

    // Poll for segment availability at seek position
    await pollForSegmentAvailability(seekTime)
  }

  // Perform seek
  videoRef.current.currentTime = seekTime
}
```

**Add Start Position to Playback API:**

Modify playback initialization to accept optional start position:

```typescript
// When user seeks during transcoding
playMedia(mediaId, metadata, { startPosition: 1800 }) // Start at 30:00
```

### 2. Backend API Changes

**Modify HLS Manifest Endpoint:**

Update `/api/media/:id/hls/:quality/manifest.m3u8` to accept optional `start` query parameter:

```go
// GET /api/media/:id/hls/:quality/manifest.m3u8?start=1800
func (h *TranscodingHandler) ServeHLSManifest(c *gin.Context) {
    mediaID := c.Param("id")
    quality := c.Param("quality")
    startPosition := c.Query("start") // Optional: start position in seconds

    // If start position provided and segments don't exist
    if startPosition != "" && !segmentsExistAtPosition(mediaID, quality, startPosition) {
        // Trigger transcoding from start position
        h.transcodeService.StartTranscodeFromPosition(mediaID, quality, startPosition)
    }

    // Generate manifest with available segments (may be discontinuous)
    manifest := h.generateHLSManifest(mediaID, quality)
    c.Data(200, "application/vnd.apple.mpegurl", manifest)
}
```

**Add Transcode From Position Endpoint:**

New endpoint for explicit seek-based transcoding requests:

```go
// POST /api/media/:id/transcode/seek
// Body: { "position": 1800, "quality": "1080p" }
func (h *TranscodingHandler) TranscodeFromPosition(c *gin.Context) {
    var req struct {
        Position int    `json:"position"` // Seconds
        Quality  string `json:"quality"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "invalid request"})
        return
    }

    mediaID := c.Param("id")

    // Start transcoding from position
    jobID, err := h.transcodeService.StartTranscodeFromPosition(
        mediaID,
        req.Quality,
        req.Position,
    )

    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(202, gin.H{
        "job_id": jobID,
        "message": "transcoding started from position",
    })
}
```

### 3. Transcoding Service Changes

**Modify FFmpeg Command:**

Update transcoding service to accept start position and use FFmpeg's `-ss` (seek) flag:

```go
func (s *TranscodingService) StartTranscodeFromPosition(
    mediaID string,
    quality string,
    startPosition int, // seconds
) (string, error) {
    media, err := s.mediaRepo.GetByID(mediaID)
    if err != nil {
        return "", err
    }

    // Calculate segment number for start position
    // Each segment is 4 seconds
    startSegment := startPosition / 4

    // Build FFmpeg command with seek
    cmd := exec.Command("ffmpeg",
        "-ss", fmt.Sprintf("%d", startPosition), // Seek to position BEFORE input (faster)
        "-i", media.Path,
        "-c:v", "libx264",
        "-preset", "veryfast",
        "-crf", getCRFForQuality(quality),
        "-c:a", "aac",
        "-f", "hls",
        "-hls_time", "4",
        "-hls_list_size", "0",
        "-start_number", fmt.Sprintf("%d", startSegment), // Start segment numbering from seek position
        "-hls_segment_filename", fmt.Sprintf("segment_%%03d.ts"),
        outputPath,
    )

    // Start transcoding job
    jobID := s.startJob(cmd, mediaID, quality, startSegment)
    return jobID, nil
}
```

**Important FFmpeg Flag Ordering:**

The `-ss` flag position matters for performance:

- `-ss` **before** `-i`: Fast seek (input seeking) - FFmpeg seeks in input file before decoding. Much faster but less accurate.
- `-ss` **after** `-i`: Slow seek (output seeking) - FFmpeg decodes from start then seeks. Accurate but very slow.

For this use case, we prioritize speed over frame-perfect accuracy.

### 4. HLS Manifest Generation

**Support Discontinuous Segments:**

Update manifest generator to handle non-sequential segment availability:

```go
func (s *TranscodingService) generateHLSManifest(
    mediaID string,
    quality string,
) []byte {
    segments := s.getAvailableSegments(mediaID, quality)

    var manifest strings.Builder
    manifest.WriteString("#EXTM3U\n")
    manifest.WriteString("#EXT-X-VERSION:3\n")
    manifest.WriteString("#EXT-X-TARGETDURATION:4\n")

    var lastSegmentNum int
    for i, segment := range segments {
        // Detect discontinuity (gap in segment numbers)
        if i > 0 && segment.Number != lastSegmentNum+1 {
            manifest.WriteString("#EXT-X-DISCONTINUITY\n")
        }

        manifest.WriteString(fmt.Sprintf("#EXTINF:%.1f,\n", segment.Duration))
        manifest.WriteString(fmt.Sprintf("segment_%03d.ts\n", segment.Number))

        lastSegmentNum = segment.Number
    }

    // Don't add #EXT-X-ENDLIST if transcoding still in progress
    if !s.isTranscodingComplete(mediaID, quality) {
        // Live playlist - will be updated as more segments become available
    } else {
        manifest.WriteString("#EXT-X-ENDLIST\n")
    }

    return []byte(manifest.String())
}
```

### 5. Segment Management

**Track Segment Ranges:**

Instead of assuming sequential segments, track actual available ranges:

```go
type SegmentRange struct {
    StartSegment int
    EndSegment   int
    Complete     bool
}

type TranscodeJob struct {
    ID           string
    MediaID      string
    Quality      string
    Ranges       []SegmentRange // Multiple ranges for seek-based transcoding
    Status       string
    StartedAt    time.Time
    CompletedAt  *time.Time
}
```

**Example segment availability after seeks:**

```
User plays from start: segments 0-50 transcoded
User seeks to 30:00: segments 450-500 transcoded
User seeks to 15:00: segments 225-275 transcoded

Available ranges:
- [0-50]
- [225-275]
- [450-500]

Manifest includes all three ranges with discontinuity markers
```

### 6. Cleanup Strategy

**Modified Cleanup Logic:**

Update cleanup service to handle discontinuous segments:

```go
func (s *CleanupService) CleanupOldSegments(mediaID string, quality string) {
    // Keep segments within active playback ranges
    // Clean up segments older than 1 hour that aren't in active ranges

    activeRanges := s.getActivePlaybackRanges(mediaID)
    allSegments := s.listSegments(mediaID, quality)

    for _, segment := range allSegments {
        if !segment.InRange(activeRanges) && segment.Age() > time.Hour {
            s.deleteSegment(segment)
        }
    }
}
```

## Consequences

### Positive

✅ **Dramatically Improved UX**: Users can seek to any position in the video, matching industry standards

✅ **Better Perceived Performance**: Instead of waiting for full file to transcode, users can skip to specific scenes immediately

✅ **More Efficient Transcoding**: Only transcode segments users actually watch, not entire files

✅ **Flexible Playback Patterns**: Users can watch intro, skip to middle, jump to end - all without waiting

✅ **Professional Feel**: Video player behaves predictably and reliably

### Negative

⚠️ **Increased Complexity**: Manifest generation and segment tracking become more complex

⚠️ **Disk Space**: May temporarily store more segments (multiple ranges instead of single sequential range)

⚠️ **Potential Edge Cases**: Need to handle rapid seeking, overlapping transcode jobs, segment gaps

⚠️ **Testing Complexity**: More scenarios to test (sequential, seek forward, seek backward, rapid seeks)

### Mitigation Strategies

**For Disk Space:**
- Implement aggressive cleanup of segments not in active playback ranges
- Limit total segments per media item (e.g., max 2000 segments = ~2 hours of content)
- Prioritize keeping most recently accessed segments

**For Rapid Seeking:**
- Debounce seek requests (wait 500ms before triggering new transcode)
- Cancel in-progress transcode jobs when user seeks to different position
- Queue seek requests and process most recent one

**For Overlapping Jobs:**
- Only allow one active transcode job per media+quality combination
- If new seek request comes in, cancel previous job and start from new position

## Implementation Plan

### Phase 1: Backend Foundation (2-3 days)
1. Add `StartTranscodeFromPosition` to transcoding service
2. Update FFmpeg command builder to support `-ss` flag
3. Implement segment range tracking (replace sequential assumption)
4. Add `/api/media/:id/transcode/seek` endpoint

### Phase 2: Manifest & Cleanup (1-2 days)
5. Update HLS manifest generator for discontinuous segments
6. Add `#EXT-X-DISCONTINUITY` marker support
7. Update cleanup service to handle segment ranges
8. Add segment range coalescing (merge adjacent ranges)

### Phase 3: Frontend Integration (1 day)
9. Update `VideoPlayer.tsx` to detect seek-ahead scenarios
10. Add API call to request transcoding from position
11. Implement buffering UI during seek transcoding
12. Add polling for segment availability

### Phase 4: Testing & Polish (1-2 days)
13. Test sequential playback (verify no regression)
14. Test seek forward to untranscoded position
15. Test rapid seeking (debouncing)
16. Test cleanup of discontinuous segments
17. Test manifest updates during active playback

**Total Estimated Time**: 5-8 days

## Alternatives Considered

### Alternative 1: Pre-transcode Entire File
**Decision**: Rejected

**Reasoning**:
- Defeats purpose of progressive transcoding
- Wastes resources transcoding content users may never watch
- Long wait time before playback can start

### Alternative 2: Force Sequential Playback Only
**Decision**: Rejected

**Reasoning**:
- Terrible UX - users expect to be able to seek
- Not competitive with modern media servers
- Users will perceive player as "broken"

### Alternative 3: Fall Back to Direct Streaming for Seeks
**Decision**: Rejected for primary flow, kept as fallback

**Reasoning**:
- Inconsistent quality (jumps between transcoded and direct)
- Defeats purpose of transcoding (codec compatibility, bandwidth savings)
- May not work at all for incompatible codecs
- However, can be useful as emergency fallback if seek-transcode fails

## References

- **ADR 005**: On-Demand Transcoding Strategy (foundation for this work)
- **ADR 015**: Player Enhancement Strategy (UX context)
- **HLS Specification**: RFC 8216 - HTTP Live Streaming
- **FFmpeg Seeking**: https://trac.ffmpeg.org/wiki/Seeking
- **User Report**: "When I scrub ahead in the video, it doesn't scrub to the exact position but 10min ahead instead"

## Notes

This ADR addresses a critical UX bug that makes the video player feel unreliable and unprofessional. The implementation builds on existing HLS progressive transcoding infrastructure (ADR 005) and requires primarily backend changes with minimal frontend updates.

The key insight is that HLS already supports discontinuous segments via `#EXT-X-DISCONTINUITY` markers - we just need to leverage this feature and update our transcoding service to start from arbitrary positions instead of always starting from the beginning.
