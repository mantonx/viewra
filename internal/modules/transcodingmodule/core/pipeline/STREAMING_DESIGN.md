# Streaming Pipeline Design

## Overview

This document outlines the transformation of our two-stage pipeline (encode → package) into a true streaming pipeline that enables instant playback.

## Current Architecture (To Be Replaced)

```
Input Video → FFmpeg (full encode) → intermediate.mp4 → Shaka Packager → DASH segments
```

Problems:
- Must wait for entire video to encode
- Large intermediate files
- High latency before playback starts

## New Streaming Architecture

```
Input Video → FFmpeg (segment mode) → Stream → Shaka (real-time) → Progressive DASH
                                          ↓
                                    Segment Events → Manifest Updates
```

## Implementation Plan

### 1. Enhance Pipeline Manager (`core/pipeline/manager.go`)
- Add streaming mode support
- Implement segment event handling
- Progressive manifest generation

### 2. Create Streaming Components
- `stream_encoder.go` - FFmpeg segment-based encoding
- `stream_packager.go` - Real-time Shaka packaging
- `segment_monitor.go` - Track segment production
- `manifest_updater.go` - Dynamic manifest updates

### 3. Update Provider (`pipeline/provider.go`)
- Add `StartStreamTranscode` method
- Implement progressive content hash calculation
- Support partial session status

## Key Features

1. **Segment-based Processing**
   - 2-4 second segments
   - Keyframe-aligned for fast seeking
   - Multiple quality levels per segment

2. **Progressive Manifests**
   - Start with partial manifest
   - Update as segments become available
   - Transition to static manifest when complete

3. **Smart Buffering**
   - Process 30 seconds ahead of viewer
   - Pause encoding if viewer stops
   - Resume on demand

4. **Zero Intermediate Files**
   - Direct pipe from FFmpeg to Shaka
   - Segments written directly to final location
   - Memory-efficient streaming

## Session Directory Structure

```
{session-id}/
├── stream.mpd          # Live manifest (updates every segment)
├── stream.m3u8         # HLS variant
├── init/
│   ├── video-360p.mp4
│   ├── video-720p.mp4
│   ├── video-1080p.mp4
│   └── audio.mp4
└── segments/
    ├── video-360p-00001.m4s
    ├── video-720p-00001.m4s
    ├── video-1080p-00001.m4s
    ├── audio-00001.m4s
    └── ... (segments added progressively)
```

## Benefits

- First segment ready in ~2 seconds
- Playback starts in <10 seconds
- No waiting for full transcode
- Efficient resource usage
- Better user experience