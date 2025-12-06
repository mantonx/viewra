# Quality Selection Simplification Plan

## Overview

Remove ABR (Adaptive Bitrate) complexity since we're doing **progressive transcoding** with only 1 video + 1 audio FFmpeg process at a time. Replace with a simpler "recommended quality" system where:

1. Backend recommends optimal quality based on client capabilities (once at playback start)
2. User can manually override to any available quality
3. Quality changes restart the video FFmpeg process (audio continues)

## Current State Analysis

### What Exists (and is complex)

**Frontend:**
- `useAutoQuality.ts` - Manages "auto mode" toggle, defers to HLS.js ABR
- `useNetworkMonitor.ts` - Collects network samples every 2 seconds
- `NetworkMonitor.ts` - Calculates throughput stats, recommends quality
- `useQualityRecommendation.ts` - Calls backend for initial recommendation
- `CapabilityDetector.ts` + 6 other files - Full client capability detection
- `QualitySelector.tsx` - Has Auto toggle, bitrate variants, complex grouping

**Backend:**
- `quality_recommender.go` - Scores profiles based on capabilities
- `adaptive_profiles.go` - ABRLadder with 14 quality variants
- `serve_master_playlist.go` - Currently filters to single "best" variant
- `session_manager.go` - Has `stopOtherQualitySessions()` but doesn't use it

### The Problem

ABR requires multiple FFmpeg processes transcoding simultaneously at different qualities. We explicitly don't want that (limits to 1 video + 1 audio). So:
- The "Auto" toggle is misleading - HLS.js ABR can't work if only 1 quality variant exists
- Network monitoring is overkill for pick-once quality selection
- Capability detection is useful but the scoring algorithm is complex

## Simplified Design

### New Mental Model

```
┌──────────────────────────────────────────────────────────────┐
│  Playback Start                                               │
│                                                               │
│  1. Frontend detects: screen size, device type, connection   │
│  2. Backend picks "recommended" quality (simple rules)       │
│  3. Master playlist includes ALL valid qualities             │
│  4. Player starts at recommended quality                     │
│  5. User can switch anytime → restarts video FFmpeg          │
└──────────────────────────────────────────────────────────────┘
```

### Quality Selection Rules (Simplified)

**Key principle**: Recommend the BEST quality the user's network can handle, regardless of screen size. Users may be casting to a TV, planning to watch later on a better display, or simply want maximum quality.

```go
func RecommendQuality(networkMbps float64, connectionType string, sourceHeight int) string {
    // For high-speed connections (ethernet or 100+ Mbps), use best quality up to source
    isFastConnection := connectionType == "ethernet" || networkMbps >= 100

    if isFastConnection {
        // Return best quality that doesn't exceed source resolution
        if sourceHeight >= 2160 {
            return "4k-60m"
        }
        if sourceHeight >= 1080 {
            return "1080p-20m"
        }
        if sourceHeight >= 720 {
            return "720p-4m"
        }
        return "480p"
    }

    // For slower connections, pick highest quality with 70% headroom
    safeNetworkMbps := networkMbps * 0.7

    // Match to ABR ladder (highest to lowest)
    qualities := []struct{ height int; mbps float64; id string }{
        {2160, 60, "4k-60m"},
        {2160, 40, "4k-40m"},
        {2160, 20, "4k-20m"},
        {1080, 20, "1080p-20m"},
        {1080, 10, "1080p-10m"},
        {1080, 4, "1080p-4m"},
        {720, 4, "720p-4m"},
        {720, 2, "720p-2m"},
        {480, 1, "480p"},
        {360, 0.8, "360p"},
    }

    for _, q := range qualities {
        if q.height <= sourceHeight && q.mbps <= safeNetworkMbps {
            return q.id
        }
    }
    return "360p" // fallback
}
```

**Example scenarios:**

- 1000 Mbps ethernet + 4K source → recommend 4K-60m (ethernet = max quality)
- 1000 Mbps ethernet + 1080p source → recommend 1080p-20m (can't upscale past source)
- 100 Mbps wifi + 4K source → recommend 4K-60m (fast enough for max)
- 25 Mbps connection + 4K source → recommend 1080p-10m (safe = 17.5 Mbps)
- 5 Mbps connection + 4K source → recommend 720p-2m (safe = 3.5 Mbps)

## Implementation Plan

### Phase 1: Backend Changes

#### 1.1 Simplify `quality_recommender.go`
- Replace `scoreProfile()` with simple rule-based selection
- Keep `ClientCapabilities` struct but only use: `ScreenHeight`, `NetworkSpeedMbps`, `DeviceType`
- Remove: `scorePower()`, `scoreCodec()`, `scoreDeviceType()` complexity

#### 1.2 Update `serve_master_playlist.go`
- Change `filterVariants()` to return ALL variants <= source resolution
- Add `recommendedQuality` field to response for frontend hint

#### 1.3 Update `session_manager.go`
- Call `stopOtherQualitySessions()` when creating a new video session
- This ensures only 1 video FFmpeg per media

### Phase 2: Frontend Changes

#### 2.1 Simplify `useQualityRecommendation.ts`
- Keep capability detection but simplify what we send
- Only need: `screenHeight`, `networkSpeedMbps`, `deviceType`
- Remove bandwidth/codec scoring - backend handles it

#### 2.2 Remove ABR-related code

**Delete entirely:**
- `web/src/lib/hooks/useAutoQuality.ts` - Auto mode not needed
- `web/src/lib/hooks/useNetworkMonitor.ts` - Continuous monitoring not needed
- `web/src/lib/network/NetworkMonitor.ts` - No longer needed

**Simplify:**
- `web/src/lib/capabilities/` - Keep but remove network speed test complexity
  - Device detection: keep
  - Screen info: keep
  - Network speed: use `navigator.connection.downlink` only (no active probing)

#### 2.3 Simplify `QualitySelector.tsx`
- Remove "Auto" toggle
- Show simple quality list: "4K", "1080p", "720p", "480p", "360p"
- Mark recommended quality with badge
- Store user preference in localStorage for next session

#### 2.4 Update `useHlsPlayer.ts`
- Remove `qualityRecommendation` prop complexity
- Set `startLevel` directly based on recommended quality
- Remove ABR-related HLS.js config

#### 2.5 Update `VideoPlayer.tsx`
- Remove `useAutoQuality` hook usage
- Remove `isAutoMode` state
- Remove `handleAutoToggle`
- Simplify quality change handler

### Phase 3: API Changes

#### 3.1 Simplify `/api/adaptive/recommend` endpoint
- Accept: `{ screenHeight, networkMbps, deviceType }`
- Return: `{ recommendedQuality: "1080p-10m", reason: "..." }`

#### 3.2 Update master playlist response
- Include recommended quality hint in master playlist or as header

## Files to Delete

```
web/src/lib/hooks/useAutoQuality.ts
web/src/lib/hooks/useNetworkMonitor.ts
web/src/lib/network/NetworkMonitor.ts
web/src/lib/network/  (entire directory if only NetworkMonitor.ts)
```

## Files to Significantly Simplify

```
web/src/lib/capabilities/NetworkDetector.ts  - Remove active speed test
web/src/lib/capabilities/CapabilityDetector.ts - Remove speed test call
web/src/lib/hooks/useQualityRecommendation.ts - Simpler request/response
web/src/components/media/VideoPlayer/QualitySelector/ - Remove Auto toggle
internal/infrastructure/transcoding/quality_recommender.go - Simple rules
internal/application/transcode/serve_master_playlist.go - Return all qualities
```

## Migration Notes

1. **User preferences**: Store last-selected quality in localStorage per device
2. **Network fallback**: If `navigator.connection` unavailable, assume 50Mbps
3. **Screen detection**: Use `window.screen.height * devicePixelRatio` for effective resolution

## Testing Checklist

- [ ] Playback starts at recommended quality
- [ ] Quality selector shows all available qualities
- [ ] Switching quality stops old FFmpeg, starts new one
- [ ] Only 2 FFmpeg processes max (1 video + 1 audio)
- [ ] Quality preference persists across sessions
- [ ] Works on mobile (touch, smaller screen)
- [ ] Works when `navigator.connection` unavailable
