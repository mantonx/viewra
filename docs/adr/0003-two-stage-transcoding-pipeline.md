# ADR-0003: Two-Stage Transcoding Pipeline Architecture

Date: 2025-06-26
Status: Accepted

## Context

Modern video streaming requires adaptive bitrate streaming formats like DASH and HLS. These formats require:

1. **Multiple quality variants** - Different resolutions and bitrates
2. **Segmented output** - Video split into small chunks
3. **Manifest files** - Playlists describing available streams
4. **Proper packaging** - Specific container formats for streaming

Initially, we tried to handle everything in a single FFmpeg command, but this approach had limitations:

- FFmpeg's DASH/HLS muxers are limited and often produce non-compliant output
- Difficult to generate multiple quality variants efficiently
- Poor control over segment boundaries and keyframe alignment
- Limited support for advanced features (DRM, subtitles, etc.)

## Decision

We implemented a two-stage transcoding pipeline:

### Stage 1: Encoding (FFmpeg)
- **Purpose**: Transcode source video to intermediate format
- **Output**: High-quality MP4 files at different resolutions
- **Focus**: Video/audio codec conversion, scaling, quality control
- **Location**: `/transcode/{contentHash}/transcoding/{sessionId}/encoded/`

### Stage 2: Packaging (Shaka Packager)
- **Purpose**: Convert encoded files to streaming format
- **Output**: DASH/HLS segments and manifests
- **Focus**: Segmentation, manifest generation, encryption (if needed)
- **Location**: `/transcode/{contentHash}/transcoding/{sessionId}/packaged/`

## Architecture

```
Source Video → FFmpeg Provider → Encoded MP4s → Pipeline Provider → Shaka Packager → DASH/HLS Output
                (Stage 1)                           (Coordinator)      (Stage 2)
```

## Implementation Details

### Content-Addressable Storage
- Use SHA256 hash of source file as content identifier
- Enables deduplication of transcoded content
- Structure: `/transcode/{contentHash}/transcoding/{sessionId}/`

### Directory Structure
```
/transcode/{contentHash}/transcoding/{sessionId}/
├── encoded/          # Stage 1 output
│   ├── 1080p.mp4
│   ├── 720p.mp4
│   └── 480p.mp4
└── packaged/         # Stage 2 output
    ├── manifest.mpd  # DASH manifest
    ├── video/        # Video segments
    └── audio/        # Audio segments
```

### Pipeline Provider
- Coordinates both stages
- Monitors progress across stages
- Handles failures and cleanup
- Ensures atomic operations

## Consequences

### Positive
- **Better quality control** - Each stage optimized for its purpose
- **Industry-standard output** - Shaka Packager produces compliant streams
- **Flexibility** - Can swap either stage independently
- **Reusability** - Encoded files can be repackaged for different formats
- **Better progress tracking** - Clear stages with measurable progress

### Negative
- **More complexity** - Two tools to manage instead of one
- **Additional storage** - Intermediate files require temporary space
- **Longer processing** - Two stages take more time than single pass
- **More failure points** - Either stage can fail

## Configuration Example

```go
// Pipeline configuration
type PipelineConfig struct {
    // Stage 1: Encoding settings
    Encoding: EncodingConfig{
        Qualities: []Quality{
            {Height: 1080, Bitrate: 5000000},
            {Height: 720,  Bitrate: 3000000},
            {Height: 480,  Bitrate: 1500000},
        },
        Codec: "h264",
        Preset: "fast",
    },
    
    // Stage 2: Packaging settings
    Packaging: PackagingConfig{
        SegmentDuration: 4,
        Format: "dash", // or "hls"
        Encryption: nil, // Optional DRM
    },
}
```

## Monitoring and Progress

Progress is tracked as weighted combination:
- Stage 1 (Encoding): 70% of total progress
- Stage 2 (Packaging): 30% of total progress

## Future Enhancements

- Add parallel encoding for multiple qualities
- Implement resume capability for interrupted jobs
- Add quality-based adaptive encoding
- Support for live streaming pipeline
- Integration with CDN for output distribution