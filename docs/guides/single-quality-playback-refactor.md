# Single-Quality Playback Refactor

## Problem

Current playback starts 3 FFmpeg processes because:
1. Master playlist returns all 15 quality variants
2. HLS.js probes multiple qualities to estimate bandwidth
3. Backend starts FFmpeg when each variant playlist is requested

This wastes resources and adds 6+ seconds to startup time.

## Solution

Single-quality playback: Backend picks one optimal quality based on client capabilities, one FFmpeg process runs.

## New Flow

```
1. User clicks Play

2. Frontend requests master playlist with capabilities:
   GET /api/media/{mediaId}/hls/master.m3u8
       ?screenWidth=3840
       ?screenHeight=2160
       ?bandwidth=30000000
       ?codecs=h264,h265

3. Backend runs recommendation logic:
   - 4K screen + 30Mbps + HEVC support → "4k-25m"

4. Backend returns single-variant master playlist:
   #EXTM3U
   #EXT-X-STREAM-INF:BANDWIDTH=25000000,RESOLUTION=3840x2160
   4k-25m/playlist.m3u8

5. HLS.js loads ONE playlist → ONE FFmpeg starts

6. User can override quality via picker (triggers reload at current position)
```

## Quality Picker

The quality picker displays available qualities filtered by source resolution. The response includes an `AvailableQualities` array:

```typescript
interface QualityOption {
  id: string;              // e.g., "1080p-10m", "original"
  displayName: string;     // e.g., "1080p (10 Mbps)", "360p (0.8 Mbps)"
  width: number;
  height: number;
  bandwidth: number;
  isSelected: boolean;     // Currently playing quality
  isOriginalQuality: boolean; // True for source quality (shows "Original" badge)
}
```

### Display Name Formatting

- Standard: `{height}p ({mbps} Mbps)` - e.g., "1080p (10 Mbps)"
- Sub-1 Mbps: Shows one decimal - e.g., "360p (0.8 Mbps)"
- 4K: Uses "4K" prefix - e.g., "4K (25 Mbps)"

### IsOriginalQuality Flag

- **Remux scenarios**: The "original" quality option has `isOriginalQuality: true`
- **Transcode scenarios**: The highest bitrate profile matching source resolution has `isOriginalQuality: true`
- Frontend displays an "Original" badge for these options

### Quality Change Flow

When user selects a different quality:
1. Capture current playback position
2. Add `?quality=<selected>` to the master playlist URL
3. Reload HLS.js with new URL
4. FFmpeg starts new session from current position

## Quality Recommendation Algorithm

The backend recommends quality based on (in priority order):

1. **User override**: If `?quality=` param is provided, use it
2. **Remux preference**: For remux strategies, prefer "original" if network supports source bitrate
3. **Network-first**: Primary factor is available bandwidth
4. **Device type**: Desktop/TV devices can upgrade to 4K at 15+ Mbps even with 1080p screens
5. **Screen resolution**: Secondary factor, never exceeds screen height

### Bandwidth Tiers

| Network Speed | Recommended Quality |
|---------------|---------------------|
| 200+ Mbps | 4K (200 Mbps reference) |
| 100+ Mbps | 4K (100 Mbps) |
| 60+ Mbps | 4K (60 Mbps) |
| 40+ Mbps | 4K (40 Mbps) or 1080p (40 Mbps) |
| 25+ Mbps | 4K (25 Mbps) |
| 20+ Mbps | 1080p (20 Mbps) or 4K (20 Mbps) |
| 15+ Mbps | 4K (15 Mbps) for desktop/TV |
| 10+ Mbps | 1080p (10 Mbps) |
| 4+ Mbps | 720p (4 Mbps) or 1080p (4 Mbps) |
| 2+ Mbps | 720p (2 Mbps) |
| 1+ Mbps | 480p (1 Mbps) |
| < 1 Mbps | 360p (0.8 Mbps) |

### Metered Connections

On metered connections (mobile data), quality is reduced by one tier to save bandwidth.

## Playback Preferences Persistence

User quality preferences are persisted per-video in the `watch_progress` table:

- `selected_quality`: Last selected quality ID (e.g., "1080p-10m", "original")
- `selected_audio_track`: FFmpeg stream index of selected audio track
- `selected_subtitle_track`: Track ID of selected subtitle (-1 for off)

On resume, these preferences are restored automatically.

## Strategy Analytics

Transcode decisions are logged and persisted for analytics:

```go
type StreamStrategy string

const (
    DirectPlay           // "Direct Play" - instant, no processing
    Remux                // "Remux" - container change only (2-5 min)
    RemuxWithAudioDownmix // "Remux + Audio Transcode" - video copy + audio transcode (5-10 min)
    RemuxHEVC            // "HEVC Remux" - HEVC stream copy (~50x realtime)
    Transcode            // "Transcode" - full re-encode (20-60 min)
)
```

The `transcode_analytics` table stores:
- `strategy`: Machine-readable enum value (e.g., "remux_hevc")
- `strategy_display`: Human-readable name (e.g., "HEVC Remux")
- `strategy_reason`: Detailed decision explanation

## Implementation Status

### Backend (Complete)

- [x] Parse capability query params (`screenWidth`, `screenHeight`, `bandwidth`, `codecs`)
- [x] Parse quality override param (`quality`)
- [x] Integrate `QualityRecommender` into `serve_master_playlist.go`
- [x] Return single-variant playlist based on recommendation
- [x] Clamp recommendation to source resolution
- [x] Return `AvailableQualities` list filtered by source resolution
- [x] Support "original" quality for remux scenarios
- [x] Persist playback preferences with watch progress
- [x] Persist strategy decisions in transcode_analytics

### Frontend (Complete)

- [x] Send capabilities with master playlist request (`useMediaPlayback.ts`)
- [x] Set `startLevel: 0` in HLS.js config
- [x] Quality picker uses `AvailableQualities` from response
- [x] Quality picker shows "Original" badge via `isOriginalQuality` flag
- [x] Quality picker triggers reload with `?quality=` override
- [x] Restore saved quality preference on resume

## Files Modified

| File | Change |
|------|--------|
| `web/src/lib/hooks/useMediaPlayback.ts` | Send capabilities, handle quality selection |
| `web/src/lib/hooks/useHlsPlayer.ts` | Set `startLevel: 0`, handle quality changes |
| `web/src/lib/hooks/useProgress.ts` | Persist/restore playback preferences |
| `web/src/components/media/VideoPlayer/QualitySelector/` | Use `AvailableQualities`, show Original badge |
| `internal/api/handlers/transcode_streaming.go` | Parse capability query params |
| `internal/application/transcode/serve_master_playlist.go` | Single-variant playlist, quality recommendation, available qualities |
| `internal/infrastructure/transcoding/profile/recommender.go` | Network-first quality recommendation |
| `internal/infrastructure/transcoding/strategy/strategy.go` | DisplayName() for human-readable strategies |
| `internal/application/progress/update_progress.go` | Persist playback preferences |

## Database Migrations

- `000052_add_playback_preferences`: Adds `selected_quality`, `selected_audio_track`, `selected_subtitle_track` to `watch_progress`
- `000053_add_strategy_reason`: Adds `strategy_display`, `strategy_reason` to `transcode_analytics`

## Testing

1. Verify only 1 FFmpeg process starts on playback
2. Verify quality recommendation matches expectations for various screen/bandwidth combos
3. Verify quality override works and resumes at correct position
4. Verify quality picker shows available options based on source resolution
5. Verify "Original" badge appears on source quality option
6. Verify quality preference is restored on resume
7. Verify strategy analytics are persisted correctly
