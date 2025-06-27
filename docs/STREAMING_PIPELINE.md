# Streaming Pipeline Architecture

## Overview

The Viewra streaming pipeline implements a real-time, segment-based transcoding system that enables instant playback without requiring full file processing. This document describes the architecture and flow of the streaming pipeline.

## High-Level Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│                 │     │                  │     │                 │
│   Media File    │────▶│ Streaming Pipeline│────▶│ Client Player   │
│                 │     │                  │     │                 │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │                     │
                    │  Content Storage    │
                    │ (Hash-Addressed)    │
                    │                     │
                    └─────────────────────┘
```

## Detailed Pipeline Flow

### 1. Encode → Package → Store → Serve

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          STREAMING PIPELINE                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────┐ │
│  │   ENCODE    │───▶│   PACKAGE   │───▶│    STORE    │───▶│  SERVE  │ │
│  └─────────────┘    └─────────────┘    └─────────────┘    └─────────┘ │
│        │                   │                   │                 │      │
│        ▼                   ▼                   ▼                 ▼      │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────┐ │
│  │   FFmpeg    │    │    Shaka    │    │  Content    │    │   API   │ │
│  │  Segments   │    │  Packager   │    │   Store     │    │ Gateway │ │
│  └─────────────┘    └─────────────┘    └─────────────┘    └─────────┘ │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2. Encoding Stage (StreamEncoder)

The encoding stage uses FFmpeg to create video segments in real-time:

```
Input Video
    │
    ▼
┌─────────────────────────────────────┐
│          Stream Encoder             │
├─────────────────────────────────────┤
│                                     │
│  1. Keyframe Detection              │
│     └─> Align segments on I-frames  │
│                                     │
│  2. Scene Complexity Analysis       │
│     └─> Adaptive bitrate/quality    │
│                                     │
│  3. Segment Generation              │
│     └─> 2-4 second segments         │
│                                     │
│  4. Multi-Profile Encoding (ABR)    │
│     └─> 1080p, 720p, 480p, etc.    │
│                                     │
└─────────────────────────────────────┘
    │
    ▼
Raw Segments (segment_000.mp4, segment_001.mp4, ...)
```

**FFmpeg Command Structure:**
```bash
ffmpeg -i input.mp4 \
  -f segment \
  -segment_time 4 \
  -segment_format mp4 \
  -force_key_frames "expr:gte(t,n_forced*4)" \
  -c:v libx264 -preset fast -crf 23 \
  -c:a aac -b:a 128k \
  segment_%03d.mp4
```

### 3. Packaging Stage (StreamPackager)

The packaging stage uses Shaka Packager to create DASH/HLS manifests:

```
Raw Segments
    │
    ▼
┌─────────────────────────────────────┐
│         Stream Packager             │
├─────────────────────────────────────┤
│                                     │
│  1. Segment Processing              │
│     └─> Add DASH/HLS metadata      │
│                                     │
│  2. Manifest Generation             │
│     ├─> stream.mpd (DASH)          │
│     └─> stream.m3u8 (HLS)          │
│                                     │
│  3. DRM Integration (optional)      │
│     └─> Widevine/FairPlay          │
│                                     │
│  4. Real-time Updates               │
│     └─> Live manifest updates       │
│                                     │
└─────────────────────────────────────┘
    │
    ▼
Packaged Content (init.mp4, segments/, manifest.mpd)
```

**Shaka Packager Command:**
```bash
packager \
  'in=segment_000.mp4,stream=video,output=video/segment_000.mp4' \
  'in=segment_000.mp4,stream=audio,output=audio/segment_000.mp4' \
  --mpd_output manifest.mpd \
  --hls_master_playlist_output master.m3u8
```

### 4. Storage Stage (ContentStore)

Content-addressable storage with deduplication:

```
Packaged Content
    │
    ▼
┌─────────────────────────────────────┐
│          Content Store              │
├─────────────────────────────────────┤
│                                     │
│  1. Content Hashing                 │
│     └─> SHA256 of video content     │
│                                     │
│  2. Deduplication Check             │
│     └─> Skip if already exists      │
│                                     │
│  3. Directory Structure             │
│     /content/                       │
│     └─> {hash}/                     │
│         ├─> manifest.mpd            │
│         ├─> video/                  │
│         │   └─> *.mp4              │
│         └─> audio/                  │
│             └─> *.mp4              │
│                                     │
│  4. Metadata Storage                │
│     └─> JSON metadata file          │
│                                     │
└─────────────────────────────────────┘
    │
    ▼
Content Hash → URL Mapping
```

### 5. Serving Stage (Content API)

Client access through RESTful API:

```
Client Request
    │
    ▼
┌─────────────────────────────────────┐
│          Content API                │
├─────────────────────────────────────┤
│                                     │
│  Endpoints:                         │
│                                     │
│  /api/v1/content/{hash}/           │
│  └─> Content info & metadata        │
│                                     │
│  /api/v1/content/{hash}/manifest.mpd│
│  └─> DASH manifest                  │
│                                     │
│  /api/v1/content/{hash}/video/*.mp4│
│  └─> Video segments                 │
│                                     │
│  /api/v1/content/{hash}/audio/*.mp4│
│  └─> Audio segments                 │
│                                     │
└─────────────────────────────────────┘
    │
    ▼
HTTP Response → Player
```

## Event Flow

The pipeline uses event-driven architecture for real-time updates:

```
┌──────────────┐  SegmentReady   ┌──────────────┐  ManifestUpdated  ┌──────────┐
│   Encoder    │─────────────────▶│   Packager   │──────────────────▶│  Client  │
└──────────────┘                  └──────────────┘                    └──────────┘
       │                                 │                                    │
       │                                 │                                    │
       ▼                                 ▼                                    ▼
┌──────────────┐                  ┌──────────────┐                    ┌──────────┐
│  Event Bus   │                  │Content Store │                    │  Player  │
└──────────────┘                  └──────────────┘                    └──────────┘
```

### Event Types:
- **SegmentReady**: New segment available for packaging
- **ManifestUpdated**: Manifest has been updated with new segments
- **TranscodeCompleted**: All segments have been processed
- **TranscodeFailed**: Error occurred during processing
- **ProgressUpdate**: Encoding progress information

## Segment Prefetching & Buffering

The pipeline includes intelligent prefetching for optimal playback:

```
┌─────────────────────────────────────────────────────────┐
│                  Segment Timeline                        │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Past ←─────────── Present ──────────→ Future          │
│                                                         │
│  [Played] [Buffer] [▶Current] [Prefetch] [Encoding...] │
│     ↑         ↑        ↑          ↑          ↑         │
│     │         │        │          │          │         │
│  Cleanup   In Memory  Playing  Downloading  Creating   │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Prefetch Strategy:
1. **Startup**: Prefetch first 3 segments (12 seconds)
2. **Linear Play**: Keep 5 segments ahead
3. **Seek**: Prefetch 2 segments at seek point
4. **Adaptive**: Adjust based on network speed

## Streaming Health Monitoring

Real-time health metrics track pipeline performance:

```
┌─────────────────────────────────────┐
│      Health Monitor Dashboard       │
├─────────────────────────────────────┤
│                                     │
│  Encode Rate:    1.2x realtime     │
│  Segments Ready: 45/120             │
│  Buffer Health:  Excellent          │
│  Network Load:   2.5 MB/s           │
│  CPU Usage:      65%                │
│  Memory:         1.2 GB             │
│                                     │
│  Alerts:                            │
│  ⚠️ High latency on segment 42      │
│                                     │
└─────────────────────────────────────┘
```

## ABR (Adaptive Bitrate) Support

Multiple quality profiles for adaptive streaming:

```
Source Video (1080p)
      │
      ├─── 1080p Profile ──→ video_1080p/
      │    (4000 kbps)
      │
      ├─── 720p Profile ───→ video_720p/
      │    (2500 kbps)
      │
      ├─── 480p Profile ───→ video_480p/
      │    (1200 kbps)
      │
      └─── Audio Track ────→ audio/
           (128 kbps)

           ↓
      DASH Manifest with all profiles
```

## Session Lifecycle

```
┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
│          │    │          │    │          │    │          │
│ Starting │───▶│ Running  │───▶│Completed │───▶│ Stored   │
│          │    │          │    │          │    │          │
└──────────┘    └──────────┘    └──────────┘    └──────────┘
     │               │               │               │
     │               │               │               │
     └───────────────┴───────────────┴───────────────┘
                           │
                           ▼
                    ┌──────────┐
                    │  Failed  │
                    └──────────┘
```

### Session States:
- **Starting**: Initializing encoder and packager
- **Running**: Actively processing segments
- **Completed**: All segments processed successfully
- **Stored**: Content moved to permanent storage
- **Failed**: Error occurred, cleanup initiated

## Error Handling & Recovery

The pipeline includes comprehensive error handling:

```
┌─────────────────────────────────────┐
│         Error Detection             │
├─────────────────────────────────────┤
│                                     │
│  1. FFmpeg Process Monitoring       │
│     └─> Detect crashes/hangs        │
│                                     │
│  2. Segment Validation              │
│     └─> Verify segment integrity    │
│                                     │
│  3. Manifest Verification           │
│     └─> Ensure valid DASH/HLS       │
│                                     │
│  4. Storage Health Checks           │
│     └─> Disk space, I/O errors      │
│                                     │
└─────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────┐
│         Recovery Actions            │
├─────────────────────────────────────┤
│                                     │
│  • Restart failed process           │
│  • Re-encode corrupted segments     │
│  • Fallback to lower quality        │
│  • Circuit breaker activation       │
│                                     │
└─────────────────────────────────────┘
```

## Performance Optimizations

### 1. Fast-Start Optimization
- Keyframe alignment for quick seeking
- GOP (Group of Pictures) optimization
- Initial segment prioritization

### 2. Parallel Processing
- Concurrent segment encoding
- Parallel packaging operations
- Async event processing

### 3. Resource Management
- CPU throttling under load
- Memory-mapped file operations
- Intelligent cleanup policies

### 4. Network Optimization
- HTTP/2 segment delivery
- Range request support
- CDN-friendly URLs

## Configuration Examples

### Basic Streaming Configuration
```yaml
streaming:
  segment_duration: 4        # seconds
  buffer_ahead: 20          # seconds
  enable_abr: true
  profiles:
    - name: "1080p"
      width: 1920
      height: 1080
      bitrate: 4000
      quality: 23
    - name: "720p"
      width: 1280
      height: 720
      bitrate: 2500
      quality: 23
    - name: "480p"
      width: 854
      height: 480
      bitrate: 1200
      quality: 23
```

### Advanced Configuration
```yaml
streaming:
  segment_duration: 2        # Shorter for low latency
  buffer_ahead: 30
  manifest_update_interval: 1s
  enable_abr: true
  
  # Encoding settings
  encoder:
    preset: "fast"
    tune: "zerolatency"
    threads: 0            # Auto-detect
    
  # Packaging settings
  packager:
    chunk_duration: 2000  # milliseconds
    segment_template: "segment_$Number$.mp4"
    
  # Storage settings
  storage:
    retention_days: 30
    dedup_enabled: true
    compression: true
```

## Monitoring & Debugging

### Key Metrics to Monitor
1. **Encoding Speed**: Should be > 1.0x realtime
2. **Segment Latency**: Time from encode to available
3. **Buffer Health**: Client buffer status
4. **Error Rate**: Failed segments percentage
5. **Storage Efficiency**: Deduplication ratio

### Debug Endpoints
- `/api/v1/sessions/{id}/status` - Session details
- `/api/v1/sessions/{id}/health` - Health metrics
- `/api/v1/sessions/{id}/logs` - FFmpeg logs
- `/api/v1/content/{hash}/info` - Content metadata

## Best Practices

1. **Segment Duration**: 2-4 seconds optimal
2. **Keyframe Interval**: Match segment duration
3. **Buffer Size**: 3-5 segments minimum
4. **Quality Profiles**: 3-4 profiles for ABR
5. **Cleanup Policy**: Remove after 30 days
6. **Error Threshold**: Retry 3 times max

## Future Enhancements

1. **Low Latency Streaming**
   - CMAF (Common Media Application Format)
   - WebRTC integration
   - Sub-second latency

2. **Advanced Codecs**
   - AV1 support
   - Hardware acceleration (NVENC, QSV, VAAPI)
   - HDR transcoding

3. **Cloud Integration**
   - S3 storage backend
   - CloudFront CDN
   - Lambda@Edge processing

4. **Analytics**
   - QoE (Quality of Experience) metrics
   - Bandwidth optimization
   - Viewer behavior tracking