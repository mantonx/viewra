# Viewra API Reference

## Table of Contents
- [Overview](#overview)
- [Authentication](#authentication)
- [Content-Addressable Storage APIs](#content-addressable-storage-apis)
- [Playback APIs](#playback-apis)
- [Media APIs](#media-apis)
- [Transcoding Pipeline](#transcoding-pipeline)
- [Deprecated APIs](#deprecated-apis)

## Overview

Viewra uses a modern two-stage transcoding pipeline with content-addressable storage for efficient media delivery. All media content is served through CDN-friendly URLs using SHA256 content hashes.

### Base URL
```
http://localhost:8080/api
```

### Response Format
All API responses use JSON format with consistent error handling:

```json
{
  "data": {},      // Response data
  "error": null,   // Error message if any
  "status": "ok"   // Status: ok, error
}
```

## Authentication

Currently, Viewra APIs are open. Authentication will be added in future releases.

## Content-Addressable Storage APIs

### Get Content Manifest

Retrieve a DASH or HLS manifest from content-addressable storage.

**DASH Endpoint:**
```
GET /v1/content/{content_hash}/manifest.mpd
```

**HLS Endpoint:**
```
GET /v1/content/{content_hash}/playlist.m3u8
```

**Parameters:**
- `content_hash` (string, required): SHA256 hash of the content

**Response:**
- Content-Type: `application/dash+xml` (DASH) or `application/vnd.apple.mpegurl` (HLS)
- Body: Manifest file with proper VOD settings

**Example:**
```bash
curl http://localhost:8080/api/v1/content/abc123def456/manifest.mpd
```

### Get Media Segments

Retrieve individual media segments.

```
GET /v1/content/{content_hash}/{segment_name}
```

**Parameters:**
- `content_hash` (string, required): SHA256 hash of the content
- `segment_name` (string, required): Segment filename (e.g., `video-0001.m4s`)

**Response:**
- Content-Type: Based on segment type (`video/mp4`, `video/iso.segment`, etc.)
- Supports byte-range requests for efficient seeking

## Playback APIs

### Start Transcoding Session

Start a new transcoding session or reuse existing content.

```
POST /playback/start
```

**Request Body:**
```json
{
  "media_file_id": "file-123",
  "container": "dash",         // "dash" or "hls"
  "enable_abr": true,          // Enable adaptive bitrate
  "seek_position": 0,          // Optional: Start position in seconds
  "device_profile": {          // Optional: Device capabilities
    "user_agent": "Mozilla/5.0...",
    "supported_codecs": ["h264", "aac"],
    "max_resolution": "1080p",
    "max_bitrate": 6000,
    "supports_hevc": false,
    "supports_av1": false,
    "supports_hdr": false
  }
}
```

**Response:**
```json
{
  "id": "session-123",
  "status": "queued",
  "manifest_url": "/api/v1/content/abc123def456/manifest.mpd",
  "provider": "ffmpeg-pipeline",
  "content_hash": "abc123def456",
  "content_url": "/api/v1/content/abc123def456/"
}
```

**Notes:**
- If content already exists (same media file + encoding parameters), returns immediately with `status: "completed"`
- Content hash is deterministic based on media ID, codec, resolution, and container

### Get Session Status

Check the status of a transcoding session.

```
GET /playback/session/{session_id}
```

**Response:**
```json
{
  "ID": "session-123",
  "Status": "running",
  "Progress": 45.2,
  "Provider": "ffmpeg-pipeline",
  "ContentHash": "abc123def456",
  "Error": null,
  "CreatedAt": "2024-06-25T10:00:00Z",
  "UpdatedAt": "2024-06-25T10:01:00Z"
}
```

### Request Seek-Ahead

Create a new transcoding session starting from a specific position for seek-ahead functionality.

```
POST /playback/seek-ahead
```

**Request Body:**
```json
{
  "session_id": "original-session-123",
  "seek_position": 300  // Position in seconds
}
```

**Response:**
```json
{
  "id": "seek-session-456",
  "status": "queued",
  "manifest_url": "/api/v1/content/seek123hash/manifest.mpd",
  "content_hash": "seek123hash",
  "content_url": "/api/v1/content/seek123hash/",
  "provider": "ffmpeg-pipeline"
}
```

### Stop Transcoding Session

Stop an active transcoding session.

```
POST /playback/stop
```

**Request Body:**
```json
{
  "session_id": "session-123"
}
```

### Stop All Sessions

Stop all active transcoding sessions.

```
POST /playback/stop-all
```

**Response:**
```json
{
  "stopped_count": 3,
  "total_sessions": 5,
  "errors": []
}
```

### Get Playback Decision

Get intelligent transcoding decision based on media file and device capabilities.

```
POST /playback/decision
```

**Request Body:**
```json
{
  "media_path": "/path/to/media.mp4",
  "device_profile": {
    "user_agent": "Mozilla/5.0...",
    "supported_codecs": ["h264", "aac"],
    "max_resolution": "1080p",
    "max_bitrate": 6000
  }
}
```

**Response:**
```json
{
  "should_transcode": true,
  "reason": "Device does not support HEVC codec",
  "transcode_params": {
    "input_path": "/path/to/media.mp4",
    "container": "dash",
    "video_codec": "h264",
    "audio_codec": "aac",
    "resolution": "1080p",
    "quality": 23,
    "enable_abr": true
  },
  "content_hash": "existing123",      // If content already exists
  "content_url": "/api/v1/content/existing123/"
}
```

## Media APIs

### List Media Files

Get a list of available media files.

```
GET /media/
```

**Query Parameters:**
- `limit` (int, optional): Maximum number of results (default: 50)
- `offset` (int, optional): Pagination offset
- `type` (string, optional): Filter by type (`movie`, `episode`)

**Response:**
```json
{
  "media": [
    {
      "id": "file-123",
      "filename": "movie.mp4",
      "path": "/media/movies/movie.mp4",
      "type": "movie",
      "size": 1073741824,
      "duration": 7200,
      "created_at": "2024-06-25T10:00:00Z"
    }
  ],
  "total": 100
}
```

### Get Media Metadata

Get detailed metadata for a media file.

```
GET /media/{file_id}/metadata
```

**Response:**
```json
{
  "id": "file-123",
  "type": "episode",
  "title": "Episode Title",
  "episode_id": "ep-456",
  "season": {
    "season_number": 1,
    "tv_show": {
      "title": "Show Title",
      "id": "show-789"
    }
  },
  "video_streams": [
    {
      "codec": "h264",
      "resolution": "1920x1080",
      "bitrate": 5000000,
      "fps": 23.976
    }
  ],
  "audio_streams": [
    {
      "codec": "aac",
      "channels": 2,
      "bitrate": 128000,
      "language": "eng"
    }
  ]
}
```

## Transcoding Pipeline

### Pipeline Architecture

The transcoding pipeline uses a two-stage process:

1. **Encoding Stage** (FFmpeg)
   - Transcodes video/audio to target codecs
   - Outputs intermediate MP4 files
   - Supports hardware acceleration (NVIDIA, Intel QSV, VAAPI)

2. **Packaging Stage** (Shaka Packager)
   - Creates DASH/HLS manifests
   - Segments media files
   - Ensures proper VOD manifest settings (type="static")

### Content Hash Generation

Content hashes are generated deterministically based on:
- Media file ID
- Video codec
- Audio codec  
- Resolution
- Container format (DASH/HLS)

This ensures identical transcoding requests produce the same hash, enabling deduplication.

### Storage Structure

Content is stored in a sharded directory structure:
```
/viewra-data/content-store/
├── ab/
│   └── c1/
│       └── abc123def456/
│           ├── manifest.mpd
│           ├── video-init.mp4
│           ├── video-0001.m4s
│           ├── audio-init.mp4
│           └── audio-0001.m4s
```

## Deprecated APIs

The following APIs are deprecated and will be removed in the next major version:

### Legacy Streaming Endpoints

```
GET /playback/stream/{session_id}/manifest.mpd    # Use /v1/content/{hash}/manifest.mpd
GET /playback/stream/{session_id}/playlist.m3u8   # Use /v1/content/{hash}/playlist.m3u8
GET /playback/stream/{session_id}/{segment}       # Use /v1/content/{hash}/{segment}
```

These endpoints now return HTTP 301 redirects to the content-addressable URLs.

## Error Handling

All APIs use consistent error responses:

```json
{
  "error": "Error message",
  "code": "ERROR_CODE",
  "details": {}
}
```

Common error codes:
- `NOT_FOUND`: Resource not found
- `INVALID_REQUEST`: Invalid request parameters
- `TRANSCODE_FAILED`: Transcoding operation failed
- `STORAGE_ERROR`: Content storage operation failed

## Rate Limiting

Currently no rate limiting is implemented. This will be added in future releases.

## Webhooks

Webhook support for transcoding events is planned for future releases.