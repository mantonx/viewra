# Viewra Transcoding API Guide

## Overview

The Viewra transcoding API provides two different ways to start transcoding sessions through the `/api/playback/start` endpoint.

## Endpoint

**POST** `/api/playback/start`

## Authentication

Currently no authentication required for development setup.

## Request Formats

### Method 1: Media File Based Request (Recommended)

Use this method when you have a media file ID from the Viewra media library.

```json
{
  "media_file_id": "test-1",
  "container": "dash",
  "seek_position": 0,
  "enable_abr": true
}
```

**Fields:**
- `media_file_id` (string, required): The ID of the media file from the database
- `container` (string, optional): Output container format ("dash", "hls", "mp4"). Default: "dash"
- `seek_position` (float, optional): Start position in seconds. Default: 0
- `enable_abr` (boolean, optional): Enable Adaptive Bitrate streaming. Default: false

### Method 2: Direct Path Based Request

Use this method when you want to transcode a file by its direct path.

```json
{
  "input_path": "/app/viewra-data/test-media/test_video_60sec.mp4",
  "container": "dash",
  "video_codec": "h264",
  "audio_codec": "aac", 
  "quality": 70,
  "speed_priority": "balanced",
  "seek": 0,
  "enable_abr": true
}
```

**Required Fields:**
- `input_path` (string): Full path to the input media file

**Optional Fields:**
- `container` (string): Output format ("dash", "hls", "mp4"). Default: "dash"
- `video_codec` (string): Video codec ("h264", "h265", "vp9"). Default: "h264"
- `audio_codec` (string): Audio codec ("aac", "opus", "mp3"). Default: "aac"
- `quality` (integer): Quality percentage 0-100. Default: 70
- `speed_priority` (string): Speed/quality tradeoff ("fastest", "balanced", "quality"). Default: "balanced"
- `seek` (integer): Start position in nanoseconds. Default: 0
- `duration` (integer): Encode duration in nanoseconds. Default: entire file
- `enable_abr` (boolean): Enable Adaptive Bitrate streaming. Default: false
- `prefer_hardware` (boolean): Prefer hardware acceleration. Default: false
- `hardware_type` (string): Specific hardware type ("cuda", "vaapi", "qsv", "videotoolbox", "amf")
- `resolution` (object): Target resolution {"width": 1920, "height": 1080}
- `frame_rate` (float): Target frame rate

## Response Format

### Success Response

```json
{
  "id": "86801028-58f1-46d2-9890-4387b1a94929",
  "status": "queued",
  "manifest_url": "/api/playback/stream/86801028-58f1-46d2-9890-4387b1a94929/manifest.mpd",
  "provider": "ffmpeg_software"
}
```

**Fields:**
- `id` (string): Unique session ID for tracking
- `status` (string): Current status ("queued", "running", "completed", "failed")
- `manifest_url` (string): URL to the streaming manifest (for DASH/HLS)
- `provider` (string): Name of the transcoding provider used

### Error Response

```json
{
  "error": "failed to start transcoding session: media file not found: record not found"
}
```

## Container Formats

| Container | Description | Output |
|-----------|-------------|---------|
| `dash` | MPEG-DASH adaptive streaming | manifest.mpd + segments |
| `hls` | HTTP Live Streaming | playlist.m3u8 + segments | 
| `mp4` | Progressive MP4 | single .mp4 file |

## Quality Settings

- `quality`: 0-100 percentage
  - 0-30: Low quality, small file size
  - 40-70: Balanced quality/size
  - 80-100: High quality, larger file size

## Speed Priority

- `fastest`: Maximum encoding speed, lower quality
- `balanced`: Good balance of speed and quality (recommended)
- `quality`: Maximum quality, slower encoding

## Hardware Acceleration

Available hardware types depend on your system:
- `cuda`: NVIDIA GPUs (nvenc/nvdec)
- `vaapi`: Intel/AMD GPUs on Linux
- `qsv`: Intel Quick Sync Video
- `videotoolbox`: macOS hardware acceleration
- `amf`: AMD hardware acceleration on Windows

## Example Usage

### Get Available Media Files

```bash
curl http://localhost:8080/api/media/files | jq '.media_files[0]'
```

### Start DASH Transcoding

```bash
curl -X POST http://localhost:8080/api/playback/start \
  -H "Content-Type: application/json" \
  -d '{
    "media_file_id": "test-1",
    "container": "dash",
    "enable_abr": true
  }'
```

### Monitor Session Status

```bash
curl http://localhost:8080/api/playback/session/SESSION_ID
```

### List All Sessions

```bash
curl http://localhost:8080/api/playback/sessions
```

### Access Streaming Content

For DASH: `http://localhost:8080/api/playback/stream/SESSION_ID/manifest.mpd`
For HLS: `http://localhost:8080/api/playback/stream/SESSION_ID/playlist.m3u8`

## Common Issues

### "input path cannot be empty"

This error occurs when using the direct path method without providing the `input_path` field. Make sure to use the exact field name `input_path` (with underscore).

### "media file not found"

This error occurs when using the media file method with an invalid `media_file_id`. Get valid IDs from `/api/media/files`.

### "no transcoding providers available"

This indicates the transcoding plugins haven't loaded yet. Check `/api/playback/health` to see provider status.

## Health Check

Check transcoding system status:

```bash
curl http://localhost:8080/api/playback/health
```

Expected healthy response:
```json
{
  "status": "healthy",
  "ready": true,
  "enabled": true,
  "initialized": true,
  "providers": {
    "count": 1,
    "names": ["FFmpeg Software Transcoder (ffmpeg_software)"]
  },
  "message": "Ready for transcoding"
}
```