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

Since the master playlist only contains one variant, the quality picker uses a static list of available qualities filtered by source resolution. When user selects a different quality:

1. Capture current playback position
2. Add `?quality=<selected>` to the master playlist URL
3. Reload HLS.js with new URL
4. FFmpeg starts new session from current position

## Implementation Status

### Backend (Complete)

- [x] Parse capability query params (`screenWidth`, `screenHeight`, `bandwidth`, `codecs`)
- [x] Parse quality override param (`quality`)
- [x] Integrate `QualityRecommender` into `serve_master_playlist.go`
- [x] Return single-variant playlist based on recommendation
- [x] Clamp recommendation to source resolution

### Frontend (In Progress)

- [x] Send capabilities with master playlist request (`useMediaPlayback.ts`)
- [x] Set `startLevel: 0` in HLS.js config
- [ ] Quality picker uses static quality list filtered by source resolution
- [ ] Quality picker triggers reload with `?quality=` override

## Files Modified

| File | Change |
|------|--------|
| `web/src/lib/hooks/useMediaPlayback.ts` | Add screenWidth, screenHeight, bandwidth to URL |
| `web/src/lib/hooks/useHlsPlayer.ts` | Set `startLevel: 0` |
| `internal/api/handlers/transcode_streaming.go` | Parse capability query params |
| `internal/application/transcode/serve_master_playlist.go` | Single-variant playlist, quality recommendation |
| `internal/app/usecases/usecases.go` | Wire up QualityRecommender |

## Testing

1. Verify only 1 FFmpeg process starts on playback
2. Verify quality recommendation matches expectations for various screen/bandwidth combos
3. Verify quality override works and resumes at correct position
4. Verify quality picker shows available options based on source resolution

## Future Considerations

- Quality picker could show "recommended" badge on auto-selected quality
- Could add user preference storage ("always use 1080p on this device")
