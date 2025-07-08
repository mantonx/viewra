# Viewra API Documentation

Complete API reference for the Viewra media management platform.

## Overview

**Base URL**: `http://localhost:8080/api`

**Response Format**:
```json
{
  "data": {},     // Response data
  "error": null,  // Error message (if any)
  "status": "ok"  // Status: ok or error
}
```

## Media APIs

### Libraries

**List Libraries**
```http
GET /api/v1/libraries
```

**Create Library**
```http
POST /api/v1/libraries
{
  "name": "Movies",
  "path": "/media/movies",
  "type": "movie"  // movie, tv, music
}
```

**Scan Library**
```http
POST /api/v1/libraries/{id}/scan
```

### Media Files

**List Media Files**
```http
GET /api/v1/media/files?page=1&per_page=20&type=movie
```

**Get Media File**
```http
GET /api/v1/media/files/{id}
```

**Stream Media File**
```http
GET /api/v1/media/files/{id}/stream
```
Returns direct MP4 stream or initiates transcoding if needed.

**Get Metadata**
```http
GET /api/v1/media/files/{id}/metadata
```

**Search Media**
```http
GET /api/v1/media/search?q=star+wars&type=movie&limit=10
```

## Playback APIs

### Playback Decision

**Get Playback Decision**
```http
POST /api/v1/playback/decide
{
  "media_file_id": "file123",
  "device_profile": {
    "name": "Chrome Browser",
    "supported_codecs": ["h264", "vp9"],
    "supported_containers": ["mp4", "webm"],
    "max_resolution": "1920x1080"
  }
}
```

Response:
```json
{
  "method": "direct",  // direct or transcode
  "stream_url": "/api/v1/media/files/123/stream",
  "reason": "Compatible format"
}
```

### Sessions

**Start Session**
```http
POST /api/v1/playback/sessions
{
  "media_file_id": "file123",
  "user_id": "user456",
  "device_id": "device789",
  "method": "direct"
}
```

**Get Session**
```http
GET /api/v1/playback/sessions/{id}
```

**Update Progress**
```http
PATCH /api/v1/playback/sessions/{id}
{
  "position": 120.5,
  "state": "playing"  // playing, paused, stopped
}
```

**End Session**
```http
DELETE /api/v1/playback/sessions/{id}
```

**List Active Sessions**
```http
GET /api/v1/playback/sessions
```

## Transcoding APIs

### Transcoding Operations

**Start Transcoding**
```http
POST /api/v1/transcoding/transcode
{
  "media_file_id": "file123",
  "container": "mp4",
  "video_codec": "h264",
  "audio_codec": "aac",
  "quality": 70
}
```

**Get Progress**
```http
GET /api/v1/transcoding/sessions/{id}/progress
```

Response:
```json
{
  "progress": 45.2,
  "eta": 120,
  "speed": "1.2x",
  "status": "running"
}
```

**Stop Transcoding**
```http
DELETE /api/v1/transcoding/sessions/{id}
```

### Providers

**List Providers**
```http
GET /api/v1/transcoding/providers
```

**Get Provider Info**
```http
GET /api/v1/transcoding/providers/{id}
```

## Content APIs

### Access Transcoded Content

**Get Content**
```http
GET /api/v1/content/{content_hash}/output.mp4
```

**Get Content Info**
```http
GET /api/v1/content/{content_hash}/info
```

## Plugin APIs

### Plugin Management

**List Plugins**
```http
GET /api/v1/plugins
```

**Get Plugin**
```http
GET /api/v1/plugins/{id}
```

**Enable/Disable Plugin**
```http
PATCH /api/v1/plugins/{id}
{
  "enabled": true
}
```

**Get Plugin Config**
```http
GET /api/v1/plugins/{id}/config
```

**Update Plugin Config**
```http
PUT /api/v1/plugins/{id}/config
{
  "setting1": "value1",
  "setting2": 123
}
```

## System APIs

### Health & Status

**Health Check**
```http
GET /api/health
```

**Database Status**
```http
GET /api/db-status
```

**System Info**
```http
GET /api/system/info
```

### Events

**Event Stream (SSE)**
```http
GET /api/events/stream
```
Server-Sent Events for real-time updates.

**Get Events**
```http
GET /api/events?type=playback.started&limit=50
```

### Configuration

**Get Config**
```http
GET /api/config
```

**Update Config**
```http
PUT /api/config/{section}
{
  "key": "value"
}
```

## Common Patterns

### Pagination
```http
GET /api/v1/media/files?page=2&per_page=50
```

Response includes:
```json
{
  "data": [...],
  "pagination": {
    "page": 2,
    "per_page": 50,
    "total": 245,
    "total_pages": 5
  }
}
```

### Filtering
```http
GET /api/v1/media/files?type=movie&year=2023
```

### Sorting
```http
GET /api/v1/media/files?sort=-created_at,title
```
Use `-` prefix for descending order.

## Error Codes

| Code | Description |
|------|-------------|
| 400 | Bad Request - Invalid parameters |
| 404 | Not Found - Resource doesn't exist |
| 409 | Conflict - Resource already exists |
| 422 | Unprocessable Entity - Validation failed |
| 500 | Internal Server Error |
| 503 | Service Unavailable |

## Authentication

Currently no authentication is required. This will be added in future versions.

## Rate Limiting

No rate limits are currently enforced. This may change in production deployments.