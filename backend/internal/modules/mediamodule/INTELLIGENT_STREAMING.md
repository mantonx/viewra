# Intelligent Video Streaming System

This document describes the new intelligent video streaming system that has replaced the basic transcoding functionality.

## Overview

The intelligent streaming system automatically decides whether to:
- Stream files directly (when compatible)
- Transcode in real-time using available plugins

This eliminates tech debt from the previous simple FFmpeg transcoding approach and provides a comprehensive, plugin-based solution.

## Architecture

### Components
- **PlaybackIntegration**: Main integration service
- **PlaybackPlanner**: Makes intelligent playback decisions
- **TranscodeManager**: Manages transcoding sessions via plugins
- **Plugin System**: Extensible transcoding backends (FFmpeg, hardware encoders, etc.)

### How It Works
1. **Request Analysis**: Analyzes user-agent, media file format, and client capabilities
2. **Decision Making**: Determines if direct streaming or transcoding is needed
3. **Execution**: Either serves file directly or starts intelligent transcoding session
4. **Streaming**: Delivers optimized video stream to client

## API Endpoints

### Primary Endpoint (Replaces old `/files/:id/transcode.mp4`)
```
GET /api/media/files/:id/stream
```
- **Intelligent Decision**: Automatically decides between direct play and transcoding
- **Device Detection**: Analyzes user-agent and capabilities
- **Quality Optimization**: Chooses best quality for client
- **Plugin Selection**: Uses best available transcoding backend

### Advanced Control
```
GET /api/media/files/:id/stream-with-decision?transcode=true&quality=720p
```
- **Explicit Control**: Force transcoding or set quality
- **Quality Options**: 480p, 720p, 1080p, 1440p, 2160p
- **Container Support**: MP4, WebM, MKV, AVI, MOV

### Playback Decision API
```
GET /api/media/files/:id/playback-decision
```
Returns decision information:
```json
{
  "should_transcode": true,
  "reason": "Container not supported by client",
  "stream_url": "/api/media/files/123/stream?transcode=true",
  "media_info": {
    "container": "mkv",
    "video_codec": "h264",
    "audio_codec": "ac3",
    "resolution": "1080p"
  },
  "transcode_params": {
    "target_codec": "h264",
    "target_container": "mp4",
    "resolution": "1080p"
  }
}
```

## Frontend Integration

### Shaka Player Update
Replace the old decision logic:

**Old Code:**
```javascript
// Old simple logic
const needsTranscode = !['mp4', 'webm'].includes(container);
const url = needsTranscode 
  ? `/api/media/files/${id}/transcode.mp4` 
  : `/api/media/files/${id}/stream`;
```

**New Code:**
```javascript
// New intelligent system
const url = `/api/media/files/${id}/stream`;
// The system handles everything automatically!

// Or with decision API for advanced control:
const decision = await fetch(`/api/media/files/${id}/playback-decision`);
const data = await decision.json();
const url = data.stream_url;
```

## Benefits

### 🚀 Performance
- **Direct Streaming**: Compatible files play immediately
- **Smart Caching**: Transcoding sessions cached for multiple clients
- **Load Balancing**: Multiple transcoding backends automatically utilized

### 🧠 Intelligence
- **Device Detection**: Automatic browser/device capability detection
- **Quality Adaptation**: Optimal quality selection based on client
- **Format Optimization**: Best container/codec for each client

### 🔧 Extensibility
- **Plugin Architecture**: Easy to add new transcoding backends
- **Hardware Acceleration**: Supports GPU transcoding plugins
- **Cloud Integration**: Can integrate cloud transcoding services

### 🛡️ Reliability
- **Fallback System**: Falls back to basic streaming if plugins unavailable
- **Error Handling**: Comprehensive error handling and logging
- **Session Management**: Automatic cleanup of completed sessions



## Monitoring

### Headers Added
- `X-Direct-Stream: true` - Indicates direct streaming (no transcoding)
- `X-Transcode-Session-ID` - Session ID for transcoded streams
- `X-Transcode-Backend` - Plugin used for transcoding
- `X-Transcode-Quality` - Quality level selected

### Logging
- Playback decisions logged with reasons
- Transcoding sessions tracked
- Performance metrics available
- Error diagnostics included

## Configuration

The system automatically:
- Discovers available transcoding plugins
- Selects optimal transcoding backend
- Manages transcoding session limits
- Handles plugin failures gracefully

No manual configuration required - it just works! 🎉 