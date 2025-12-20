# HLS Transcoding Guide

This guide explains how ViewRA's HLS (HTTP Live Streaming) transcoding system works, including the progressive transcoding approach, hardware acceleration, and streaming strategies.

## Architecture Overview

ViewRA uses **progressive HLS transcoding** where a single long-running FFmpeg process generates video segments on-demand. This approach, inspired by Jellyfin, provides smooth playback without buffering.

```text
┌─────────────────────────────────────────────────────────────┐
│                      Client Request                          │
│    GET /api/media/123/hls/1080p-10m/playlist.m3u8           │
└─────────────────────────────────┬───────────────────────────┘
                                  ▼
┌─────────────────────────────────────────────────────────────┐
│                  Strategy Determination                      │
│                                                              │
│  Is video compatible? ──Yes──► Direct Play (302 redirect)   │
│        │                                                     │
│        No                                                    │
│        ▼                                                     │
│  Remux only? ───Yes──► Copy codecs, remux to HLS            │
│        │                                                     │
│        No                                                    │
│        ▼                                                     │
│  Transcode: GPU-accelerated encode                          │
└─────────────────────────────────┬───────────────────────────┘
                                  ▼
┌─────────────────────────────────────────────────────────────┐
│                    Session Manager                           │
│                                                              │
│  ┌─────────────────────────────────────────────────────────┐│
│  │              Transcode Session                          ││
│  │                                                         ││
│  │  FFmpeg Process ─────► Writes segments progressively   ││
│  │      │                                                  ││
│  │      └── seg_000000.ts                                  ││
│  │      └── seg_000001.ts                                  ││
│  │      └── seg_000002.ts                                  ││
│  │      └── ...                                            ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

## Streaming Strategies

ViewRA intelligently selects the best streaming strategy based on source video and client capabilities:

| Strategy | When Used | FFmpeg Action |
|----------|-----------|---------------|
| **Direct Play** | H.264/AAC in MP4 container | 302 redirect to file |
| **Remux** | H.264 but wrong container | Copy video/audio, remux to HLS |
| **Remux + Downmix** | H.264, surround audio, stereo client | Copy video, downmix audio |
| **Remux HEVC** | HEVC-capable client, HEVC source | Copy HEVC to HLS |
| **Transcode** | Incompatible codec or resolution change | Full GPU transcode |

### Strategy Determination

```go
// Strategy is determined based on:
// 1. Source video codec (H.264, HEVC, AV1, etc.)
// 2. Client capabilities (codec support, container support)
// 3. Requested quality (same resolution = remux, different = transcode)
strategy, reason := DetermineStrategyWithCapabilities(videoInfo, clientCaps)
```

### Human-Readable Strategy Names

Each strategy has a `DisplayName()` method for UI display:

| Strategy Constant | Display Name | Typical Time |
|-------------------|--------------|--------------|
| `DirectPlay` | "Direct Play" | Instant |
| `Remux` | "Remux" | 2-5 min |
| `RemuxWithAudioDownmix` | "Remux + Audio Transcode" | 5-10 min |
| `RemuxHEVC` | "HEVC Remux" | ~50x realtime |
| `Transcode` | "Transcode" | 20-60 min |

### Strategy Analytics

Strategy decisions are persisted in the `transcode_analytics` table for debugging and performance analysis:

| Column | Description | Example |
|--------|-------------|---------|
| `strategy` | Machine-readable enum | `remux_hevc` |
| `strategy_display` | Human-readable name | `HEVC Remux` |
| `strategy_reason` | Detailed decision explanation | `HEVC video supported by client, remuxing to HLS with ac3 audio transcode to AAC` |

Debug logging shows strategy decisions:

```text
INFO strategy decision mediaID=376910 codec=hevc isHDR=false
     clientSupportsHDR=true supportedCodecs="[h264 h265 vp9 av1]"
     strategy=remux_hevc reason="HEVC video supported by client..."
```

## Progressive Transcoding

### Why Progressive?

Earlier attempts at segment-by-segment transcoding failed because:
- Each FFmpeg invocation has ~50-100ms startup overhead
- Segment generation takes 150-250ms per segment
- HLS.js requests 10-20 segments at once to fill buffers
- Playback stalls when generation can't keep up

**Progressive transcoding** solves this by:
- Running a single long-lived FFmpeg process
- FFmpeg writes segments continuously ahead of playback
- Segments are available as soon as they're generated
- On seek, kill and restart FFmpeg from new position

### Session Management

```go
type TranscodeSession struct {
    ID            string
    MediaID       int64
    Quality       string
    StartPosition float64       // Start position in seconds

    FFmpegCmd     *exec.Cmd
    OutputDir     string        // Where segments are written
    ManifestPath  string        // Path to playlist.m3u8

    CreatedAt     time.Time
    LastAccessed  time.Time
}
```

Sessions are managed by the `SessionManager`:

```go
// Get existing session or create new one
session, err := sessionManager.GetOrCreateSession(GetOrCreateSessionParams{
    MediaID:       123,
    Quality:       "1080p-10m",
    StartPosition: 0,
    OutputDir:     "/data/transcodes/123/1080p-10m",
})
```

### Seeking

When a user seeks:

1. Client requests playlist with new start position
2. Server calculates segment number: `segNum = seekTime / segmentDuration`
3. Existing FFmpeg session is killed
4. New session starts with `-ss <seekTime>` flag
5. FFmpeg generates segments from the new position
6. Updated playlist is returned to client

```text
User seeks to 5:00 (300 seconds)
├── Request: GET /api/media/123/hls/1080p/playlist.m3u8?start=300
├── Calculate: segment 50 (300s / 6s per segment)
├── Kill existing FFmpeg
├── Start new: ffmpeg -ss 300 -i input.mkv ...
├── FFmpeg writes seg_000050.ts, seg_000051.ts, ...
└── Return updated manifest starting at segment 50
```

## Hardware Acceleration

ViewRA automatically detects and uses the best available hardware encoder.

### Supported Hardware

| Platform | Encoder | Requirements |
|----------|---------|--------------|
| **NVIDIA** | NVENC | GPU with NVENC, `nvidia-smi` |
| **Intel** | QSV | Quick Sync, `/dev/dri/renderD128` |
| **Intel/AMD** | VAAPI | VAAPI drivers, `/dev/dri/renderD128` |
| **macOS** | VideoToolbox | Metal support |
| **Fallback** | libx264 | Always available |

### GPU Pipeline Optimization

For maximum performance, ViewRA uses end-to-end GPU pipelines where possible:

**NVENC Full GPU Pipeline** (15-20x faster than software):
```text
Input → NVDEC (GPU decode) → scale_cuda → pad_cuda → NVENC (GPU encode) → Output
```

**VAAPI Full GPU Pipeline** (6x faster):
```text
Input → VAAPI decode → scale_vaapi → pad_vaapi → VAAPI encode → Output
```

**QSV Pipeline** (8x faster):
```text
Input → QSV decode → scale_qsv → QSV encode → Output
```

**VideoToolbox Pipeline** (6-8x faster, FFmpeg limitation):
```text
Input → VT decode (GPU) → hwdownload → scale (CPU) → VT encode (GPU) → Output
```

### Configuration

Hardware acceleration is enabled by default. Override via environment:

```bash
# Force software encoding
VIEWRA_HW_ACCEL=none

# Force specific hardware
VIEWRA_HW_ACCEL=nvenc
VIEWRA_HW_ACCEL=vaapi
VIEWRA_HW_ACCEL=qsv
VIEWRA_HW_ACCEL=videotoolbox
```

## Quality Profiles

ViewRA supports adaptive bitrate streaming with predefined quality profiles:

| Profile | Resolution | Video Bitrate | Audio |
|---------|------------|---------------|-------|
| `2160p-50m` | 3840×2160 | 50 Mbps | 384 kbps |
| `2160p-35m` | 3840×2160 | 35 Mbps | 384 kbps |
| `1080p-20m` | 1920×1080 | 20 Mbps | 192 kbps |
| `1080p-10m` | 1920×1080 | 10 Mbps | 192 kbps |
| `720p-5m` | 1280×720 | 5 Mbps | 128 kbps |
| `480p-2m` | 854×480 | 2 Mbps | 128 kbps |

### Single-Quality Master Playlist

ViewRA uses a **single-quality playback model** where the backend selects the optimal quality based on client capabilities. The master playlist contains only one variant:

```m3u8
#EXTM3U
#EXT-X-VERSION:3

#EXT-X-STREAM-INF:BANDWIDTH=10000000,RESOLUTION=1920x1080,CODECS="avc1.640028,mp4a.40.2"
1080p-10m/playlist.m3u8
```

This approach:

- Starts only ONE FFmpeg process (vs. 3+ with multi-variant)
- Reduces startup time by 6+ seconds
- Prevents bandwidth probe thrashing

### Quality Recommendation

The backend recommends quality based on client capabilities sent as query parameters:

```text
GET /api/media/{mediaId}/hls/master.m3u8
    ?screenWidth=1920
    &screenHeight=1080
    &bandwidth=15000000
    &codecs=h264,h265
```

**Recommendation Priority:**

1. **User override**: `?quality=1080p-10m` forces specific quality
2. **Remux preference**: For remux strategies, prefer "original" if network supports source bitrate
3. **Network-first**: Primary factor is available bandwidth
4. **Device type**: Desktop/TV devices can upgrade to 4K at 15+ Mbps
5. **Screen resolution**: Secondary factor, never exceeds screen height

### Available Qualities Response

The master playlist response includes an `AvailableQualities` list for the frontend quality picker:

```json
{
  "strategy": "serve_playlist",
  "playlistContent": "#EXTM3U...",
  "availableQualities": [
    {
      "id": "original",
      "displayName": "1080p (25 Mbps)",
      "width": 1920,
      "height": 1080,
      "bandwidth": 25000000,
      "isSelected": false,
      "isOriginalQuality": true
    },
    {
      "id": "1080p-10m",
      "displayName": "1080p (10 Mbps)",
      "width": 1920,
      "height": 1080,
      "bandwidth": 10000000,
      "isSelected": true,
      "isOriginalQuality": false
    }
  ]
}
```

**Display Name Formatting:**

- Standard: `{height}p ({mbps} Mbps)` - e.g., "1080p (10 Mbps)"
- Sub-1 Mbps: Shows decimal - e.g., "360p (0.8 Mbps)"
- 4K: Uses "4K" prefix - e.g., "4K (25 Mbps)"

### Quality Change Flow

When the user selects a different quality:

1. Frontend captures current playback position
2. Adds `?quality=<selected>&start=<position>` to master playlist URL
3. Reloads HLS.js with new URL
4. Backend kills old FFmpeg session, starts new one from position

## HDR and Tone Mapping

ViewRA automatically detects HDR content and tone maps to SDR for maximum compatibility.

### HDR Detection

HDR is detected from FFprobe metadata:
- Pixel format: `yuv420p10le` (10-bit)
- Color space: `bt2020nc`
- Color transfer: `smpte2084` (HDR10) or `arib-std-b67` (HLG)

### Tone Mapping Backends

| Backend | Performance | Quality | GPU Support |
|---------|-------------|---------|-------------|
| **libplacebo** | Fast | Excellent | NVIDIA, AMD, Intel |
| **OpenCL** | Fast | Good | NVIDIA, AMD |
| **VAAPI** | Very Fast | Good | Intel, AMD |
| **CPU (zscale)** | Slow | Excellent | All |

### Tone Mapping Algorithms

| Algorithm | Description |
|-----------|-------------|
| `bt.2390` | ITU-R reference (default, best quality) |
| `reinhard` | Smooth rolloff, film-like |
| `hable` | Uncharted 2 style, preserves contrast |
| `mobius` | Smooth clipping |

## FFmpeg Builder

The `hls.Builder` provides a fluent interface for constructing FFmpeg commands:

```go
opts := hls.Options{
    InputPath:  "/media/movie.mkv",
    OutputDir:  "/cache/transcodes/123/1080p",
    Profile:    profile1080p,
    VideoCodec: hls.CodecH264,
    VideoInfo:  videoInfo,
    ToneMappingEnabled: videoInfo.IsHDR,
}

builder := hls.NewBuilder(opts)
args := builder.
    AddLogLevel("warning").
    AddHideBanner().
    AddSeekPosition(startSeconds).
    AddHardwareAccel(hwAccelArgs).
    AddInput().
    AddStreamMapping().
    AddHardwareVideoEncoding(hls.AccelNVENC).
    AddAudioEncoding().
    AddHLSOutput().
    AddOutputFile().
    Build()

// Execute FFmpeg with prepared arguments
cmd := exec.Command(ffmpegPath, args...)
```

## Segment Serving

When HLS.js requests a segment:

```go
func (h *Handler) ServeSegment(c *gin.Context) {
    mediaID := c.Param("media_id")
    quality := c.Param("quality")
    segment := c.Param("segment")  // e.g., "seg_000042.ts"

    // Get active session
    session := h.sessionManager.GetSession(mediaID, quality)
    if session == nil {
        c.Status(404)
        return
    }

    // Wait for segment to be generated (with timeout)
    segmentPath, err := session.WaitForSegment(segment, 10*time.Second)
    if err != nil {
        c.Status(404)
        return
    }

    // Serve the segment file
    c.File(segmentPath)
}
```

## Cleanup

Transcode sessions are cleaned up:

1. **On idle timeout**: Sessions inactive for 10 minutes are killed
2. **On playback end**: Frontend signals playback complete
3. **On seek**: Old session killed, new one started
4. **On server shutdown**: All sessions gracefully terminated

Segment files are cleaned up by a background task:

```go
// Cleanup task runs every 30 minutes
func (m *SessionManager) CleanupOrphans() {
    // Find transcodes older than 1 hour with no active session
    // Delete segment files and output directories
}
```

## Performance Tuning

### FFmpeg Preset Selection

| Use Case | Preset | Speed | Quality |
|----------|--------|-------|---------|
| Real-time streaming | `veryfast` | High | Good |
| Batch transcoding | `medium` | Medium | Better |
| Archival | `slow` | Low | Best |

### NVENC Quality Settings

```go
// ViewRA defaults for NVENC
"-preset", "p4",          // Balanced (p1=fastest, p7=best)
"-tune", "hq",            // High quality
"-rc", "vbr",             // Variable bitrate
"-cq", "21",              // Quality level (lower=better)
"-spatial-aq", "1",       // Reduce blocking in flat areas
"-temporal-aq", "1",      // Better motion handling
"-rc-lookahead", "32",    // Frames to look ahead
```

### Memory Safety

```go
// Prevent OOM on corrupt files
"-max_alloc", "1073741824"  // 1GB max allocation
```

## Troubleshooting

### FFmpeg Logs

Enable detailed FFmpeg logging:

```bash
VIEWRA_FFMPEG_LOG_LEVEL=verbose ./viewra
```

### Check Hardware Acceleration

```bash
# List available encoders
ffmpeg -encoders | grep nvenc
ffmpeg -encoders | grep vaapi
ffmpeg -encoders | grep qsv

# Test NVENC
ffmpeg -hwaccel cuda -i test.mp4 -c:v h264_nvenc -f null -

# Test VAAPI
ffmpeg -vaapi_device /dev/dri/renderD128 -i test.mp4 -vf 'hwupload,scale_vaapi' -c:v h264_vaapi -f null -
```

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| Buffering during playback | FFmpeg too slow | Enable hardware acceleration |
| Segments not found | Session expired | Increase idle timeout |
| Poor quality | Wrong preset | Use `p4` for NVENC, `medium` for software |
| OOM crashes | Corrupt file | Add `-max_alloc` flag |
| Seek not working | Session not killed | Check `WaitForSegment` timeout |

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VIEWRA_FFMPEG_PATH` | Custom FFmpeg binary | System `ffmpeg` |
| `VIEWRA_FFPROBE_PATH` | Custom FFprobe binary | System `ffprobe` |
| `VIEWRA_FFMPEG_LIB_PATH` | LD_LIBRARY_PATH for custom builds | (none) |
| `VIEWRA_HW_ACCEL` | Force hardware acceleration type | `auto` |
| `VIEWRA_TRANSCODE_DIR` | Directory for transcode cache | `./data/transcodes` |

## See Also

- [Hardware Acceleration](../features/HARDWARE_ACCELERATION.md)
- [Tone Mapping](../features/TONE_MAPPING.md)
- [ADR 021: Progressive HLS Transcoding](../decisions/021-progressive-hls-transcoding.md)
- [ADR 005: On-Demand Transcoding Strategy](../decisions/005-on-demand-transcoding-strategy.md)
