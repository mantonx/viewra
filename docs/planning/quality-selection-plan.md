# Quality Selection Simplification Plan

## Status: IMPLEMENTED

This plan was implemented in the `refactor/single-quality-playback` branch and merged to main. See [single-quality-playback-refactor.md](../guides/single-quality-playback-refactor.md) for the full implementation guide.

## Overview

Removed ABR (Adaptive Bitrate) complexity since we're doing **progressive transcoding** with only 1 video + 1 audio FFmpeg process at a time. Replaced with a simpler "single-quality" system where:

1. Backend picks optimal quality based on client capabilities (once at playback start)
2. Master playlist returns a **single variant** (not all variants)
3. User can manually override quality via picker (triggers FFmpeg restart)
4. Quality preference is persisted per-video in `watch_progress` table

## What Was Removed

### Deleted Files
- `web/src/lib/hooks/useAutoQuality.ts` - Auto mode not needed with single-quality model
- `web/src/lib/hooks/useNetworkMonitor.ts` - Continuous monitoring not needed
- `web/src/lib/network/NetworkMonitor.ts` - No longer needed
- Complex scoring algorithms in recommender

### Simplified Components
- **QualitySelector**: Removed "Auto" toggle, shows simple list with "Original" badge
- **useHlsPlayer**: Set `startLevel: 0` since only one variant exists
- **Capability detection**: Simplified to essentials (screen, bandwidth, device type)

## Actual Implementation

### Single-Quality Model

Instead of returning all variants and letting HLS.js pick, the backend:
1. Receives client capabilities via query params
2. Runs quality recommendation algorithm
3. Returns **single-variant master playlist**
4. One FFmpeg process starts

```
GET /api/media/{id}/hls/master.m3u8?screenWidth=3840&screenHeight=2160&bandwidth=30000000
                                                                         ↓
                                         Backend recommends "4k-25m" based on capabilities
                                                                         ↓
                              #EXTM3U
                              #EXT-X-STREAM-INF:BANDWIDTH=25000000,RESOLUTION=3840x2160
                              4k-25m/playlist.m3u8
```

### Quality Recommendation Logic

Implemented in `internal/infrastructure/transcoding/profile/recommender.go`:

```go
func (r *QualityRecommender) RecommendQuality(params RecommendParams) QualityRecommendation {
    // 1. If user specified quality override, use it
    if params.QualityOverride != "" {
        return findQuality(params.QualityOverride)
    }

    // 2. For remux strategies, prefer "original" if network supports it
    if params.IsRemux && params.NetworkBandwidth >= params.SourceBitrate {
        return "original"
    }

    // 3. Network-first: find highest quality that fits within bandwidth
    // 4. Desktop/TV devices can get 4K at 15+ Mbps (upgrade from 1080p)
    // 5. Never exceed source resolution
}
```

**Key decisions:**
- Network bandwidth is the primary factor
- Desktop/TV devices can upgrade to 4K at 15+ Mbps even with 1080p screens
- Metered connections reduce quality by one tier
- Source resolution is a ceiling, not a target

### Available Qualities Response

The master playlist handler also returns an `AvailableQualities` array (via custom header or JSON response) for the quality picker:

```typescript
interface QualityOption {
  id: string;              // e.g., "1080p-10m", "original"
  displayName: string;     // e.g., "1080p (10 Mbps)"
  width: number;
  height: number;
  bandwidth: number;
  isSelected: boolean;
  isOriginalQuality: boolean;  // Shows "Original" badge
}
```

### Quality Change Flow

When user selects a different quality:
1. Frontend captures current playback position
2. Adds `?quality=<selected>` to master playlist URL
3. Reloads HLS.js with new URL
4. Backend starts new FFmpeg session from current position
5. Old FFmpeg session is terminated

### Playback Preferences Persistence

Quality (and audio/subtitle track) preferences are saved per-video in `watch_progress`:

```sql
-- Migration 000052
ALTER TABLE watch_progress ADD COLUMN selected_quality TEXT;
ALTER TABLE watch_progress ADD COLUMN selected_audio_track INTEGER;
ALTER TABLE watch_progress ADD COLUMN selected_subtitle_track INTEGER;
```

On resume, preferences are restored automatically.

## Files Modified (Implementation)

| File | Change |
|------|--------|
| `web/src/lib/hooks/useMediaPlayback.ts` | Send capabilities, handle quality selection |
| `web/src/lib/hooks/useHlsPlayer.ts` | Set `startLevel: 0`, handle quality changes |
| `web/src/lib/hooks/useProgress.ts` | Persist/restore playback preferences |
| `web/src/components/media/VideoPlayer/QualitySelector/` | Use `AvailableQualities`, show Original badge |
| `internal/api/handlers/transcode_streaming.go` | Parse capability query params |
| `internal/application/transcode/serve_master_playlist.go` | Single-variant playlist, available qualities |
| `internal/infrastructure/transcoding/profile/recommender.go` | Network-first quality recommendation |

## Testing Results

- [x] Playback starts at recommended quality
- [x] Only 1 FFmpeg process starts on playback (not 3+)
- [x] Quality selector shows all available qualities
- [x] Switching quality restarts FFmpeg at current position
- [x] Quality preference persists per-video
- [x] "Original" badge appears on source quality option
- [x] Works when `navigator.connection` unavailable (uses fallback)

## Original Plan vs Reality

| Aspect | Original Plan | Actual Implementation |
|--------|---------------|----------------------|
| Master playlist | Return ALL variants with recommended hint | Return SINGLE variant |
| Quality selection | Client-side HLS.js picks | Backend picks, client overrides |
| Preference storage | localStorage | `watch_progress` table (per-video per-user) |
| Network detection | `navigator.connection.downlink` only | Query params from frontend (more reliable) |
| Recommendation logic | Simple if/else | Network-first with device-type upgrades |

## Related Documentation

- [Single-Quality Playback Refactor](../guides/single-quality-playback-refactor.md) - Full implementation guide
- [HLS Transcoding](../guides/HLS_TRANSCODING.md) - Master playlist format, quality recommendation
- [Watch Progress Tracking](../decisions/019-watch-progress-tracking-reliability.md) - Playback preferences persistence
