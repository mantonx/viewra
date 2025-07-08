# Transcoding Guide

How to use Viewra's transcoding system for media format conversion.

## Overview

Viewra automatically transcodes media files when they're not compatible with the playback device. The system uses a plugin-based architecture with support for hardware acceleration.

## Quick Start

### Check if Transcoding is Needed

```bash
# Get playback decision
curl -X POST http://localhost:8080/api/v1/playback/decide \
  -H "Content-Type: application/json" \
  -d '{
    "media_file_id": "file123",
    "device_profile": {
      "supported_codecs": ["h264"],
      "supported_containers": ["mp4"]
    }
  }'
```

### Start Transcoding

```bash
# Start a transcoding session
curl -X POST http://localhost:8080/api/v1/transcoding/transcode \
  -H "Content-Type: application/json" \
  -d '{
    "media_file_id": "file123",
    "container": "mp4",
    "video_codec": "h264",
    "audio_codec": "aac",
    "quality": 70
  }'
```

### Monitor Progress

```bash
# Check transcoding progress
curl http://localhost:8080/api/v1/transcoding/sessions/{session_id}/progress
```

## Transcoding Options

### Video Settings

| Parameter | Options | Default | Description |
|-----------|---------|---------|-------------|
| `video_codec` | h264, h265, vp9 | h264 | Video codec |
| `quality` | 0-100 | 70 | Quality level |
| `resolution` | {width, height} | Original | Target resolution |
| `frame_rate` | Number | Original | Target FPS |

### Audio Settings

| Parameter | Options | Default | Description |
|-----------|---------|---------|-------------|
| `audio_codec` | aac, mp3, opus | aac | Audio codec |
| `audio_bitrate` | Number | 128 | Audio bitrate (kbps) |
| `audio_channels` | 1, 2, 5.1 | Original | Audio channels |

### Container Formats

| Format | Description | Use Case |
|--------|-------------|----------|
| `mp4` | MPEG-4 container | Universal compatibility |
| `webm` | WebM container | Web streaming |
| `mkv` | Matroska container | High quality storage |

## Hardware Acceleration

Enable GPU-accelerated transcoding for better performance:

```json
{
  "prefer_hardware": true,
  "hardware_type": "cuda"  // Optional: specific GPU type
}
```

### Supported Hardware

- **NVIDIA**: CUDA/NVENC acceleration
- **Intel**: Quick Sync Video (QSV)
- **AMD**: VA-API acceleration
- **Apple**: VideoToolbox (macOS)

## Transcoding Providers

View available providers:

```bash
curl http://localhost:8080/api/v1/transcoding/providers
```

Each provider has different capabilities:
- **ffmpeg_software**: Universal CPU-based transcoding
- **ffmpeg_nvidia**: NVIDIA GPU acceleration
- **ffmpeg_qsv**: Intel Quick Sync
- **ffmpeg_vaapi**: AMD/Intel on Linux

## Content Storage

Transcoded files are stored using content-addressable storage:

- Files are hashed based on content and encoding parameters
- Duplicate transcodes are automatically deduplicated
- Access transcoded content at: `/api/v1/content/{hash}/output.mp4`

## Best Practices

### 1. Use Appropriate Quality

- **Low (0-30)**: Mobile/bandwidth-constrained
- **Medium (40-60)**: Balanced quality/size
- **High (70-85)**: HD quality
- **Ultra (90-100)**: Maximum quality

### 2. Consider Hardware Acceleration

Hardware encoding is faster but may have slightly lower quality:
```json
{
  "prefer_hardware": true,
  "quality": 75  // Slightly higher for hardware encoding
}
```

### 3. Avoid Unnecessary Transcoding

Check if direct play is possible before transcoding:
1. Use `/api/v1/playback/decide` first
2. Only transcode if `method` is "transcode"
3. Consider updating client capabilities instead

### 4. Monitor Resource Usage

- Check active sessions: `GET /api/v1/transcoding/sessions`
- Limit concurrent transcodes in production
- Use hardware acceleration for multiple streams

## Error Handling

### Common Errors

**No providers available**
```json
{
  "error": "no capable providers found",
  "code": "NO_PROVIDERS"
}
```
Solution: Check provider status and capabilities

**Resource limit exceeded**
```json
{
  "error": "maximum concurrent sessions reached",
  "code": "LIMIT_EXCEEDED"
}
```
Solution: Wait for sessions to complete or increase limits

**Invalid codec combination**
```json
{
  "error": "codec not supported in container",
  "code": "INVALID_CODEC"
}
```
Solution: Use compatible codec/container combinations

## Examples

### Basic Transcoding
```bash
# Convert MKV to MP4
curl -X POST http://localhost:8080/api/v1/transcoding/transcode \
  -d '{
    "media_file_id": "movie123",
    "container": "mp4",
    "quality": 70
  }'
```

### Mobile Optimization
```bash
# Lower quality for mobile devices
curl -X POST http://localhost:8080/api/v1/transcoding/transcode \
  -d '{
    "media_file_id": "video456",
    "container": "mp4",
    "quality": 40,
    "resolution": {"width": 1280, "height": 720}
  }'
```

### Hardware Accelerated
```bash
# Use GPU for faster transcoding
curl -X POST http://localhost:8080/api/v1/transcoding/transcode \
  -d '{
    "media_file_id": "4k-video",
    "container": "mp4",
    "prefer_hardware": true,
    "quality": 80
  }'
```