# Transcoding Management API Documentation

## Overview

The Transcoding Management API provides comprehensive endpoints for managing video transcoding sessions, monitoring backend performance, and controlling playback decisions. This API supports both simple and enhanced operations with advanced filtering, pagination, and batch operations.

## Base URL

```
/api/playback
```

## API Endpoints Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/decide` | Basic playback decision |
| POST | `/decide/enhanced` | Enhanced playback decision |
| POST | `/transcode/start` | Start transcoding session |
| POST | `/transcode/start/enhanced` | Start with enhanced options |
| GET | `/transcode/sessions` | List active sessions |
| GET | `/transcode/sessions/enhanced` | List with filtering |
| POST | `/transcode/batch` | Batch operations |
| GET | `/transcode/:sessionId` | Get session info |
| PUT | `/transcode/:sessionId` | Update session |
| DELETE | `/transcode/:sessionId` | Stop session |
| GET | `/transcode/:sessionId/stream` | Stream video |
| GET | `/backends` | List backends |
| GET | `/backends/:backendId` | Get backend info |
| POST | `/backends/refresh` | Refresh plugins |
| GET | `/system/info` | System information |
| GET | `/stats` | Basic statistics |
| GET | `/health` | Health check |

## Detailed Endpoint Documentation

### 1. Playback Decision Endpoints

#### POST `/api/playback/decide`
Analyzes media file and determines whether transcoding is required.

**Request Body:**
```json
{
  "media_path": "/path/to/video.mkv",
  "device_profile": {
    "user_agent": "Mozilla/5.0...",
    "supported_codecs": ["h264", "aac"],
    "max_resolution": "1080p",
    "max_bitrate": 6000,
    "supports_hevc": false,
    "supports_av1": false,
    "supports_hdr": false,
    "client_ip": "192.168.1.100"
  }
}
```

**Response:**
```json
{
  "should_transcode": true,
  "transcode_params": {
    "input_path": "/path/to/video.mkv",
    "target_codec": "h264",
    "target_container": "mp4",
    "resolution": "1080p",
    "bitrate": 3000,
    "audio_codec": "aac",
    "quality": 23,
    "preset": "fast"
  },
  "reason": "Transcoding required: container change: mkv -> mp4"
}
```

#### POST `/api/playback/decide/enhanced`
Extended version with request tracking and customization options.

**Request Body:**
```json
{
  "media_path": "/path/to/video.mkv",
  "device_profile": {
    "user_agent": "Mozilla/5.0...",
    "supported_codecs": ["h264", "aac"],
    "max_resolution": "1080p",
    "max_bitrate": 6000,
    "supports_hevc": false,
    "supports_av1": false,
    "supports_hdr": false,
    "client_ip": "192.168.1.100"
  },
  "force_analysis": false,
  "options": {
    "priority": 5,
    "preferred_codec": "h264",
    "quality": 23,
    "preset": "fast",
    "metadata": {
      "client_version": "1.0.0",
      "request_source": "web_player"
    }
  }
}
```

**Response:**
```json
{
  "should_transcode": true,
  "transcode_params": {
    "input_path": "/path/to/video.mkv",
    "target_codec": "h264",
    "target_container": "mp4",
    "resolution": "1080p",
    "bitrate": 3000,
    "audio_codec": "aac",
    "quality": 23,
    "preset": "fast"
  },
  "reason": "Transcoding required: container change: mkv -> mp4",
  "request_id": "req_1640995200000000000",
  "process_time_ms": 15,
  "metadata": {
    "client_version": "1.0.0",
    "request_source": "web_player"
  }
}
```

### 2. Session Management Endpoints

#### POST `/api/playback/transcode/start`
Initiates a new transcoding session.

**Request Body:**
```json
{
  "input_path": "/path/to/video.mkv",
  "target_codec": "h264",
  "target_container": "mp4",
  "resolution": "1080p",
  "bitrate": 3000,
  "audio_codec": "aac",
  "audio_bitrate": 128,
  "quality": 23,
  "preset": "fast",
  "priority": 5
}
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "request": {
    "input_path": "/path/to/video.mkv",
    "target_codec": "h264",
    "target_container": "mp4",
    "resolution": "1080p",
    "bitrate": 3000
  },
  "status": "running",
  "progress": 0.0,
  "start_time": "2024-01-01T12:00:00Z",
  "backend": "ffmpeg_transcoder"
}
```

#### GET `/api/playback/transcode/sessions/enhanced`
Returns transcoding sessions with filtering and pagination.

**Query Parameters:**
- `status` (string): Filter by session status (running, completed, failed, pending)
- `backend` (string): Filter by backend name
- `limit` (int): Number of results to return (default: 20, max: 100)
- `offset` (int): Number of results to skip (default: 0)
- `sort_by` (string): Sort field (start_time, status, progress)
- `sort_order` (string): Sort order (asc, desc)

**Example Request:**
```
GET /api/playback/transcode/sessions/enhanced?status=running&limit=10&offset=0
```

**Response:**
```json
{
  "sessions": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "status": "running",
      "progress": 45.2,
      "start_time": "2024-01-01T12:00:00Z",
      "backend": "ffmpeg_transcoder",
      "queue_position": 0,
      "estimated_completion_ms": 90000,
      "speed": 1.2,
      "bytes_processed": 536870912,
      "bytes_remaining": 536870912,
      "created_at": "2024-01-01T12:00:00Z",
      "updated_at": "2024-01-01T12:01:30Z"
    }
  ],
  "total": 25,
  "limit": 10,
  "offset": 0,
  "has_more": true,
  "filtered_by": {
    "status": "running"
  }
}
```

#### GET `/api/playback/transcode/:sessionId`
Retrieves detailed information about a specific transcoding session.

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "request": {
    "input_path": "/path/to/video.mkv",
    "target_codec": "h264",
    "target_container": "mp4",
    "resolution": "1080p",
    "bitrate": 3000
  },
  "status": "running",
  "progress": 67.8,
  "start_time": "2024-01-01T12:00:00Z",
  "backend": "ffmpeg_transcoder",
  "stats": {
    "fps": 29.97,
    "frame": 2045,
    "bitrate": "2850kbps",
    "speed": "1.1x"
  }
}
```

#### PUT `/api/playback/transcode/:sessionId`
Updates session metadata or priority.

**Request Body:**
```json
{
  "priority": 8,
  "metadata": {
    "updated_by": "admin",
    "reason": "priority_boost"
  }
}
```

#### DELETE `/api/playback/transcode/:sessionId`
Terminates a transcoding session.

**Response:**
```json
{
  "message": "session stopped"
}
```

#### GET `/api/playback/transcode/:sessionId/stream`
Returns the live transcoded video stream.

**Response Headers:**
```
Content-Type: video/mp4
Cache-Control: no-cache
Connection: keep-alive
```

### 3. Batch Operations

#### POST `/api/playback/transcode/batch`
Performs operations on multiple sessions simultaneously.

**Request Body:**
```json
{
  "session_ids": [
    "550e8400-e29b-41d4-a716-446655440000",
    "550e8400-e29b-41d4-a716-446655440001"
  ],
  "operation": "stop",
  "force": false
}
```

**Supported Operations:**
- `stop`: Stop selected sessions
- `priority`: Update priority

**Response:**
```json
{
  "operation": "stop",
  "results": {
    "550e8400-e29b-41d4-a716-446655440000": {
      "success": true
    },
    "550e8400-e29b-41d4-a716-446655440001": {
      "success": false,
      "error": "session not found"
    }
  },
  "total": 2
}
```

### 4. Backend Management

#### GET `/api/playback/backends`
Returns information about all available transcoding backends.

**Response:**
```json
{
  "backends": [
    {
      "id": "ffmpeg_transcoder",
      "name": "FFmpeg Transcoder",
      "status": "running",
      "version": "1.0.0",
      "capabilities": {
        "name": "ffmpeg",
        "supported_codecs": ["h264", "hevc", "vp8", "vp9", "av1"],
        "supported_resolutions": ["480p", "720p", "1080p", "1440p", "2160p"],
        "supported_containers": ["mp4", "webm", "mkv"],
        "hardware_acceleration": false,
        "max_concurrent_sessions": 3,
        "priority": 50
      },
      "statistics": {
        "name": "ffmpeg",
        "priority": 50,
        "active_sessions": 2,
        "total_sessions": 125,
        "success_rate": 98.4,
        "average_speed": 1.2
      }
    }
  ]
}
```

#### GET `/api/playback/backends/:backendId`
Returns detailed information about a specific backend.

#### POST `/api/playback/backends/refresh`
Manually triggers discovery and registration of available transcoding plugins.

**Response:**
```json
{
  "message": "plugins refreshed successfully",
  "available_backends": 2,
  "backends": [
    "ffmpeg_transcoder",
    "hardware_encoder"
  ]
}
```

### 5. System Information

#### GET `/api/playback/system/info`
Returns comprehensive system-wide information.

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "2h 15m 30s",
  "total_sessions": 1247,
  "active_sessions": 5,
  "queued_sessions": 2,
  "available_backends": 2,
  "capabilities": {
    "supported_codecs": ["h264", "hevc", "vp8", "vp9", "av1"],
    "supported_resolutions": ["480p", "720p", "1080p", "1440p", "2160p"],
    "supported_containers": ["mp4", "webm", "mkv"],
    "max_concurrent_sessions": 10,
    "hardware_acceleration": true
  },
  "performance": {
    "cpu_usage_percent": 45.2,
    "memory_usage_bytes": 2147483648,
    "average_speed_mbps": 12.5,
    "last_updated": "2024-01-01T12:15:30Z"
  },
  "backends": [
    {
      "id": "ffmpeg_transcoder",
      "name": "FFmpeg Transcoder",
      "status": "running",
      "priority": 50,
      "active_sessions": 3,
      "max_sessions": 5,
      "supported_codecs": ["h264", "hevc", "vp8", "vp9", "av1"]
    }
  ]
}
```

#### GET `/api/playback/stats`
Returns basic transcoding statistics.

#### GET `/api/playback/health`
Returns service health status.

**Response:**
```json
{
  "status": "healthy",
  "enabled": true,
  "uptime": "2h 15m 30s"
}
```

## Error Handling

### Error Response Format
```json
{
  "error": "descriptive error message",
  "code": "ERROR_CODE",
  "request_id": "req_1640995200000000000"
}
```

### Common HTTP Status Codes
- `200 OK`: Successful operation
- `201 Created`: Resource created successfully
- `400 Bad Request`: Invalid request parameters
- `404 Not Found`: Resource not found
- `409 Conflict`: Resource conflict (e.g., session limit reached)
- `500 Internal Server Error`: Server error
- `503 Service Unavailable`: Service temporarily unavailable

### Error Codes
- `INVALID_MEDIA_PATH`: Invalid media file path
- `UNSUPPORTED_CODEC`: Codec not supported
- `SESSION_NOT_FOUND`: Session ID not found
- `SESSION_LIMIT_REACHED`: Max sessions exceeded
- `BACKEND_UNAVAILABLE`: No backends available
- `TRANSCODING_FAILED`: Transcoding process failed

## Rate Limiting

- **Session Creation**: 10 requests/minute per IP
- **Session Queries**: 100 requests/minute per IP
- **System Info**: 20 requests/minute per IP

Rate limit headers:
```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 8
X-RateLimit-Reset: 1640995260
```

## Usage Examples

### JavaScript
```javascript
// Start transcoding with enhanced options
const response = await fetch('/api/playback/transcode/start/enhanced', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    input_path: '/videos/movie.mkv',
    target_codec: 'h264',
    target_container: 'mp4',
    resolution: '1080p',
    bitrate: 3000,
    session_name: 'movie_transcode',
    metadata: {
      user_id: '12345',
      content_type: 'movie'
    }
  })
});

const session = await response.json();
console.log('Session started:', session.id);

// Monitor progress
const monitorProgress = async (sessionId) => {
  const status = await fetch(`/api/playback/transcode/${sessionId}`);
  const sessionInfo = await status.json();
  console.log(`Progress: ${sessionInfo.progress}%`);
  
  if (sessionInfo.status === 'running' && sessionInfo.progress < 100) {
    setTimeout(() => monitorProgress(sessionId), 5000);
  }
};

monitorProgress(session.id);
```

### Python
```python
import requests
import time

# Start transcoding
response = requests.post('/api/playback/transcode/start/enhanced', json={
    'input_path': '/videos/movie.mkv',
    'target_codec': 'h264',
    'target_container': 'mp4',
    'resolution': '1080p',
    'bitrate': 3000,
    'session_name': 'movie_transcode',
    'metadata': {
        'user_id': '12345',
        'content_type': 'movie'
    }
})

session = response.json()
print(f"Session started: {session['id']}")

# Monitor progress
def monitor_progress(session_id):
    while True:
        status = requests.get(f"/api/playback/transcode/{session_id}")
        session_info = status.json()
        print(f"Progress: {session_info['progress']}%")
        
        if session_info['status'] != 'running' or session_info['progress'] >= 100:
            break
            
        time.sleep(5)

monitor_progress(session['id'])
```

### cURL Examples

```bash
# Start a transcoding session
curl -X POST /api/playback/transcode/start/enhanced \
  -H "Content-Type: application/json" \
  -d '{
    "input_path": "/videos/movie.mkv",
    "target_codec": "h264",
    "target_container": "mp4",
    "resolution": "1080p",
    "bitrate": 3000
  }'

# Get session status
curl /api/playback/transcode/550e8400-e29b-41d4-a716-446655440000

# List active sessions with filtering
curl "/api/playback/transcode/sessions/enhanced?status=running&limit=5"

# Stop a session
curl -X DELETE /api/playback/transcode/550e8400-e29b-41d4-a716-446655440000

# Get system information
curl /api/playback/system/info

# Refresh backends
curl -X POST /api/playback/backends/refresh
```

This comprehensive API provides complete transcoding management capabilities with proper monitoring, control, and integration features suitable for production use. 