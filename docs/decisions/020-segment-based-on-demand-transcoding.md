# ADR 016: Segment-Based On-Demand Transcoding

## Status

**REJECTED** - See ADR-017 for progressive transcoding approach

## Rejection Reason

After implementation and testing (2025-01-20), discovered fundamental flaw: **segment generation cannot keep up with playback consumption**.

### The Problem

- Each segment takes ~250-300ms to generate (even with remux)
- Player needs 1 segment every 6 seconds for continuous playback
- Player downloads ALL buffered segments immediately upon request (not gradually)
- Even with 8 parallel workers, generation cannot keep up when player requests 10+ segments at once
- Result: Playback buffers after ~10-20 seconds as segment generation queue backs up

### What We Learned

1. **HLS.js buffer behavior**: When player requests manifest, it immediately requests multiple segments to fill buffer (10-20 segments), not gradually
2. **FFmpeg overhead**: Each individual FFmpeg invocation has startup/seeking overhead (~50-100ms) that adds up
3. **Worker scaling limits**: Can't run unlimited parallel FFmpeg processes (resource constraints)
4. **Progressive approach**: Uses single long-running FFmpeg process, not individual segment generation

This approach works well for:

- ✅ Seeking (can jump to any position)
- ✅ Sparse playback (only generate what's watched)
- ❌ **Continuous playback** (cannot generate fast enough)

See ADR-017 for the correct progressive transcoding approach.

## Context

Currently, ViewRA uses a linear transcoding approach where videos are transcoded sequentially from start to finish. When a user seeks ahead to an unbuffered position, we restart the entire transcode from the new position, discarding all previous work.

### Current Limitations

1. **Wasteful on seeks**: Seeking to minute 31 in a video discards 30 minutes of transcoded content
2. **Slow startup for seeks**: Every seek requires starting a new transcode job from scratch
3. **Poor user experience**: Users must wait for transcoding to catch up after seeking
4. **Inefficient resource usage**: CPU/disk resources are wasted on content that may never be watched

### User Behavior Patterns

Analysis of typical video streaming behavior shows:
- Users frequently skip intros/credits (first 30s, last 2 minutes)
- Users sample content by jumping around before committing to watch
- Users rarely watch videos sequentially from start to finish
- Most videos have <50% actual watch time

### Industry Approaches

**YouTube/Netflix**: Pre-transcode entire videos at multiple qualities (storage-heavy, transcoding-heavy)

**Parallel transcoding**: Linear transcoding with parallel temporary transcodes for seeks (complex coordination)

**Segment-based**: On-demand segment generation (generates only what's needed)

## Decision

We will implement **Segment-Based On-Demand Transcoding** where individual HLS segments are generated on-demand rather than sequentially.

### Core Principles

1. **Stateless Segment Generation**: Any segment can be generated independently
2. **Cache-First Architecture**: Serve cached segments, generate on cache miss
3. **Predictive Generation**: Background-generate nearby segments for smooth playback
4. **LRU Cache Management**: Keep recently accessed segments, clean up old ones
5. **Consistent Segment Numbering**: Segment numbers remain stable across sessions

## Architecture

### Segment Calculation

```
Segment Number = floor(timestamp_seconds / segment_duration)
Segment Start Time = segment_number * segment_duration
```

Example with 6-second segments:
- Segment 0: 0:00 - 0:06
- Segment 1: 0:06 - 0:12
- Segment 180: 18:00 - 18:06

### Component Design

#### 1. Segment Cache Manager

**Responsibilities:**
- Check if segment exists in cache
- Serve cached segments
- Track segment access patterns
- Manage LRU eviction

**Storage Structure:**
```
data/transcodes/hls/segments/
├── {media_id}/
│   ├── {quality}/
│   │   ├── seg_000000.ts  (Segment 0)
│   │   ├── seg_000001.ts  (Segment 1)
│   │   ├── seg_000180.ts  (Segment 180)
│   │   └── metadata.json  (Cache metadata)
```

**Metadata Format:**
```json
{
  "media_id": 7587,
  "quality": "1080p",
  "segment_duration": 6,
  "segments": {
    "0": {
      "created_at": "2025-01-20T10:30:00Z",
      "last_accessed": "2025-01-20T10:35:00Z",
      "access_count": 5,
      "size_bytes": 2457600
    }
  }
}
```

#### 2. On-Demand Segment Generator

**Responsibilities:**
- Generate individual segments using FFmpeg
- Use input seeking (`-ss` before `-i`) for fast random access
- Handle segment generation failures gracefully
- Queue parallel generation requests

**FFmpeg Command Template:**
```bash
ffmpeg -ss {start_time} \
       -i {input_file} \
       -t {segment_duration} \
       -c:v libx264 -preset veryfast \
       -c:a aac \
       -f mpegts \
       {output_segment}
```

**Key Options:**
- `-ss` before `-i`: Input seeking (fast but less accurate)
- `-t 6`: Generate exactly 6 seconds
- `-preset veryfast`: Optimize for generation speed over compression
- `-f mpegts`: MPEG-TS format for HLS segments

#### 3. Predictive Segment Prefetcher

**Responsibilities:**
- Analyze playback patterns to predict next segments
- Background-generate segments likely to be requested soon
- Avoid blocking user requests

**Prediction Strategy:**
```go
// When segment N is requested, predict and prefetch:
// - Segments N+1 to N+5 (next 30 seconds for smooth playback)
// - Every 10th segment for 2 minutes ahead (for scrubbing)

func PredictSegments(currentSegment int) []int {
    predicted := []int{}

    // Next 5 segments (immediate playback)
    for i := 1; i <= 5; i++ {
        predicted = append(predicted, currentSegment+i)
    }

    // Every 10th segment for scrubbing
    for i := 10; i <= 20; i += 10 {
        predicted = append(predicted, currentSegment+i)
    }

    return predicted
}
```

#### 4. Dynamic Manifest Generator

**Responsibilities:**
- Generate HLS playlist manifest on-demand
- Reference segments by stable segment numbers
- Update discontinuity sequences for seeks
- Support byte-range requests for segments

**Manifest Template:**
```m3u8
#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:6
#EXT-X-MEDIA-SEQUENCE:{start_segment}

#EXTINF:6.0,
seg_{start_segment:06d}.ts
#EXTINF:6.0,
seg_{start_segment+1:06d}.ts
...
```

### API Design

#### GET /api/media/:id/hls/:quality/playlist.m3u8?start={seconds}

**Parameters:**
- `start` (optional): Start position in seconds (default: 0)

**Response:**
- Returns HLS manifest starting from calculated segment
- No job creation, no queuing, immediate response

#### GET /api/media/:id/hls/:quality/seg_{number}.ts

**Flow:**
1. Check cache for segment
2. If cached: Serve immediately
3. If not cached:
   - Queue segment generation
   - Wait for generation (with timeout)
   - Serve generated segment
4. Update segment access metadata
5. Trigger predictive prefetch

### Cache Management

#### Eviction Strategy

**LRU with size pressure:**
```go
type EvictionPolicy struct {
    MaxCacheSize    int64   // Maximum cache size in bytes
    TargetCacheSize int64   // Target size after cleanup (80% of max)
    MinAccessCount  int     // Minimum accesses to keep segment
    MaxAge          time.Duration // Maximum age regardless of access
}

func (p *EvictionPolicy) ShouldEvict(segment *CachedSegment) bool {
    // Always evict if over max age (default: 7 days)
    if time.Since(segment.CreatedAt) > p.MaxAge {
        return true
    }

    // If under pressure, evict low-access segments
    if currentCacheSize > p.MaxCacheSize {
        if segment.AccessCount < p.MinAccessCount {
            return true
        }
        // Evict LRU segments
        return segment.LastAccessed < oldestAllowedAccess
    }

    return false
}
```

#### Cache Warming

**Optional pre-generation for popular content:**
```go
// Generate key segments for quick access
func WarmCache(mediaID int64, quality string) {
    // Generate first segment (immediate playback)
    GenerateSegment(mediaID, quality, 0)

    // Generate segments at 25%, 50%, 75% (common seek points)
    duration := GetMediaDuration(mediaID)
    GenerateSegment(mediaID, quality, int(duration*0.25/6))
    GenerateSegment(mediaID, quality, int(duration*0.50/6))
    GenerateSegment(mediaID, quality, int(duration*0.75/6))
}
```

## Implementation Plan

### Phase 1: Core Infrastructure (Week 1)

**Tasks:**
1. Create segment cache storage structure
2. Implement segment number calculation utilities
3. Build segment cache manager with metadata tracking
4. Create on-demand segment generator
5. Update database schema for segment tracking

**Database Changes:**
```sql
-- New table for segment cache tracking
CREATE TABLE segment_cache (
    id INTEGER PRIMARY KEY,
    media_id INTEGER NOT NULL,
    quality TEXT NOT NULL,
    segment_number INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL,
    last_accessed_at TIMESTAMP NOT NULL,
    access_count INTEGER DEFAULT 0,
    UNIQUE(media_id, quality, segment_number)
);

CREATE INDEX idx_segment_cache_lru
    ON segment_cache(media_id, quality, last_accessed_at);
```

### Phase 2: Request Handling (Week 2)

**Tasks:**
1. Implement dynamic manifest generator
2. Update segment serving handler to use cache
3. Add queue for parallel segment generation
4. Implement segment generation with FFmpeg
5. Add comprehensive error handling

**Testing Focus:**
- Concurrent segment requests
- Cache hit/miss scenarios
- FFmpeg failure handling
- Race conditions in segment generation

### Phase 3: Optimization (Week 3)

**Tasks:**
1. Implement predictive prefetching
2. Add LRU cache eviction
3. Build cache warming for popular content
4. Add monitoring and metrics
5. Performance tuning

**Metrics to Track:**
- Cache hit rate
- Segment generation time
- Concurrent generation requests
- Cache size and eviction frequency

### Phase 4: Migration & Testing (Week 4)

**Tasks:**
1. Create migration path from linear transcoding
2. A/B test with subset of users
3. Performance comparison
4. Fix issues discovered in testing
5. Full rollout

## Consequences

### Positive

1. **Instant Seek Response**: No transcode restart needed for seeks
2. **Resource Efficiency**: Only transcode segments that are watched
3. **Better User Experience**: Smooth playback with predictive prefetching
4. **Scalability**: Linear complexity instead of full-file transcoding
5. **Disk Space Savings**: Only cache popular segments

### Negative

1. **Implementation Complexity**: More complex than linear transcoding
2. **FFmpeg Overhead**: Each segment requires FFmpeg invocation (mitigated by fast preset)
3. **Cache Management**: Need robust LRU eviction and monitoring
4. **Edge Cases**: Must handle concurrent requests, failures, cache corruption
5. **Testing Complexity**: More scenarios to test (cache misses, concurrent generation, etc.)

### Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|-----------|
| FFmpeg input seeking inaccuracy | Visible artifacts at segment boundaries | Use keyframe alignment, accept <1s inaccuracy |
| Concurrent generation of same segment | Wasted CPU, duplicate work | Lock-based generation queue, check cache before generating |
| Cache corruption | Failed playback | Verify segment integrity, regenerate on corruption |
| Disk space exhaustion | Cache fills disk | Aggressive LRU eviction, configurable cache size limits |
| Slow segment generation | Buffering during playback | Use `-preset veryfast`, implement generation timeout |

## Alternatives Considered

### 1. Current Approach (Linear Transcoding)

**Pros:**
- Simple implementation
- Predictable resource usage
- Works for sequential viewing

**Cons:**
- Wasteful on seeks
- Poor UX for non-sequential viewing
- Can't handle random access efficiently

**Decision:** Rejected - doesn't meet user behavior patterns

### 2. Parallel Transcoding

**Pros:**
- Keeps main transcode progressing
- Good for sequential viewing
- Better than current approach for seeks

**Cons:**
- Complex coordination between jobs
- Still transcodes content that may never be watched
- Resource waste for sampled viewing

**Decision:** Considered but more complex than needed

### 3. Full Pre-Transcoding (YouTube/Netflix Style)

**Pros:**
- Instant playback anywhere
- Best user experience
- No on-demand generation complexity

**Cons:**
- Massive storage requirements
- Transcode everything regardless of viewing
- Not practical for user-uploaded content

**Decision:** Rejected - not viable for MVP scale

## Success Metrics

### Performance Targets

- **Cache Hit Rate**: >80% for segments after initial playback
- **Segment Generation Time**: <2 seconds per segment (720p/1080p)
- **First Segment Ready**: <3 seconds from request
- **Seek Latency**: <1 second for cache hits, <5 seconds for cache misses
- **Disk Space Usage**: <20% of current transcode storage

### User Experience Metrics

- **Seek Buffering Time**: <3 seconds average
- **Playback Startup Time**: <2 seconds
- **Rebuffering Events**: <1 per viewing session
- **User Satisfaction**: Track seek-ahead complaints

## Related Decisions

- **ADR-015: Video Player Enhancement Strategy** - Drives need for better seek performance
- **ADR-006: Progressive HLS Transcoding** - Current linear approach we're replacing
- **ADR-013: Transcode Cache Management** - Cache management strategy applies here

## References

- [HLS Specification RFC 8216](https://datatracker.ietf.org/doc/html/rfc8216)
- [FFmpeg Seeking Documentation](https://trac.ffmpeg.org/wiki/Seeking)
- [Netflix Encoding Optimizations](https://netflixtechblog.com/high-quality-video-encoding-at-scale-d159db052746)

## Notes

- This ADR is marked as "Proposed" - implementation timeline TBD based on MVP priorities
- Consider implementing Phase 2 optimization (smart restart detection) first as simpler intermediate step
- Segment-based approach can coexist with linear transcoding during migration period
- Cache warming can be disabled initially and enabled based on usage patterns

---

**Created:** 2025-01-20
**Updated:** 2025-01-20
**Authors:** Claude Code AI
**Reviewers:** TBD
