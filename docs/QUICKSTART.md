# Viewra Quick Start Guide

## Prerequisites

- Docker and Docker Compose
- Go 1.24+ (for plugin development)
- Node.js 18+ (for frontend development)
- 8GB+ RAM recommended
- NVIDIA GPU (optional, for hardware acceleration)

## Getting Started

### 1. Clone and Setup

```bash
# Clone the repository
git clone https://github.com/viewra/viewra.git
cd viewra

# Initial setup
make dev-setup
```

This command will:
- Build all plugins
- Set up the database
- Start backend and frontend services

### 2. Access the Application

- Frontend: http://localhost:5175
- Backend API: http://localhost:8080/api
- SQLite Web UI: http://localhost:8081 (if enabled)

### 3. Development Mode

For hot-reload development:

```bash
# Start with Air hot-reload (recommended)
make dev

# Or with docker-compose directly
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up
```

## Understanding the Architecture

### Content-Addressable Storage

All transcoded content is stored using SHA256 hashes:

```
/api/v1/content/{hash}/manifest.mpd  → DASH manifest
/api/v1/content/{hash}/playlist.m3u8 → HLS playlist
/api/v1/content/{hash}/video-0001.m4s → Media segments
```

### Two-Stage Pipeline

1. **Encoding**: FFmpeg transcodes to intermediate MP4
2. **Packaging**: Shaka Packager creates DASH/HLS manifests

This ensures proper VOD manifests with `type="static"`.

## Common Development Tasks

### Building Plugins

```bash
# Build specific plugin
make build-plugin p=ffmpeg_software

# Build all plugins
make build-plugins

# List available plugins
make plugin-list
```

### Testing Transcoding

```bash
# 1. Upload a test video to viewra-data/media/
cp /path/to/test.mp4 viewra-data/media/

# 2. Scan for new media
curl -X POST http://localhost:8080/api/media/scan

# 3. Start transcoding
curl -X POST http://localhost:8080/api/playback/start \
  -H "Content-Type: application/json" \
  -d '{
    "media_file_id": "file-123",
    "container": "dash",
    "enable_abr": true
  }'

# Response includes content_hash:
# {
#   "id": "session-456",
#   "content_hash": "abc123def456",
#   "content_url": "/api/v1/content/abc123def456/"
# }
```

### Frontend Development

```bash
cd frontend

# Install dependencies
npm install

# Start dev server
npm run dev

# Run tests
npm test

# Build for production
npm run build
```

### Debugging

#### Check Logs
```bash
# Backend logs
make logs

# Plugin-specific logs
make logs-plugins

# All services
docker-compose logs -f
```

#### Common Issues

**1. Plugin Not Found**
```bash
# Rebuild plugins
make clean-plugins
make build-plugins
make restart-backend
```

**2. Database Issues**
```bash
# Check database status
make check-db

# Open SQLite Web UI
make db-web
```

**3. Transcoding Fails**
```bash
# Check FFmpeg logs
docker-compose exec backend ls -la /app/viewra-data/transcoding/
docker-compose exec backend cat /app/viewra-data/transcoding/*/ffmpeg-stderr.log
```

## API Examples

### Start Transcoding with Device Profile

```bash
curl -X POST http://localhost:8080/api/playback/start \
  -H "Content-Type: application/json" \
  -d '{
    "media_file_id": "file-123",
    "container": "dash",
    "enable_abr": true,
    "device_profile": {
      "user_agent": "Mozilla/5.0...",
      "supported_codecs": ["h264", "aac"],
      "max_resolution": "1080p",
      "max_bitrate": 6000,
      "supports_hevc": false
    }
  }'
```

### Check Content Existence

```bash
# If you know the content hash
curl http://localhost:8080/api/v1/content/abc123def456/manifest.mpd

# Returns 200 if exists, 404 if not
```

### Request Seek-Ahead

```bash
curl -X POST http://localhost:8080/api/playback/seek-ahead \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "original-session-123",
    "seek_position": 300
  }'
```

## Environment Configuration

### Key Environment Variables

```bash
# Content Storage
CONTENT_STORE_PATH=/app/viewra-data/content-store
CONTENT_BASE_URL=/api/v1/content

# Transcoding
TRANSCODE_MAX_CONCURRENT=4
TRANSCODE_QUALITY_PRESET=medium
PIPELINE_ENCODER_TIMEOUT=3600

# Development
DEBUG_MODE=true
LOG_LEVEL=debug
```

### Docker Compose Overrides

For development with content persistence:

```yaml
# docker-compose.dev.yml
services:
  backend:
    volumes:
      - ./viewra-data:/app/viewra-data  # Persist content between restarts
    environment:
      - DEBUG_MODE=true
      - LOG_LEVEL=debug
```

## Plugin Development

### Creating a New Transcoding Plugin

1. Create plugin directory:
```bash
mkdir -p plugins/transcoding/my_plugin
```

2. Add plugin.cue configuration:
```cue
plugin_name: "my_plugin"
plugin_type: "transcoding"
version: "1.0.0"
enabled: true

config: {
    max_concurrent: 2
    pipeline_support: {
        enabled: true
        output_format: "mp4"
    }
}
```

3. Implement the interface:
```go
package main

import (
    "github.com/mantonx/viewra/sdk/plugin"
    "github.com/mantonx/viewra/sdk/transcoding"
)

type MyPlugin struct{}

func (p *MyPlugin) GetInfo() plugin.Info {
    return plugin.Info{
        Name:    "my_plugin",
        Type:    plugin.TypeTranscoding,
        Version: "1.0.0",
    }
}

func (p *MyPlugin) StartTranscode(req *transcoding.Request) (*transcoding.Handle, error) {
    // Implementation
}

// ... implement other required methods

func main() {
    plugin.Serve(&MyPlugin{})
}
```

4. Build and test:
```bash
make build-plugin p=my_plugin
make restart-backend
```

## Production Deployment

### Building for Production

```bash
# Build production images
make prod-build

# Deploy
make prod-deploy
```

### Performance Tuning

1. **Enable Hardware Acceleration**
   - Use ffmpeg_nvidia plugin for NVIDIA GPUs
   - Configure GPU device in plugin.cue

2. **Optimize Content Storage**
   - Mount content-store on SSD
   - Use separate disk for transcoding temp files

3. **Scale Transcoding**
   - Increase TRANSCODE_MAX_CONCURRENT
   - Deploy multiple backend instances

4. **CDN Integration**
   - Configure CDN to cache /api/v1/content/* paths
   - Set appropriate cache headers

## Troubleshooting

### View Transcoding Progress

```bash
# Get session status
curl http://localhost:8080/api/playback/session/{session_id}

# Get all active sessions
curl http://localhost:8080/api/playback/sessions
```

### Clean Up Stuck Sessions

```bash
# Stop all sessions
curl -X POST http://localhost:8080/api/playback/stop-all

# Clean up old sessions
curl -X POST http://localhost:8080/api/playback/cleanup?max_age_hours=2
```

### Reset Everything

```bash
# Stop all services
docker-compose down

# Clean up data (WARNING: deletes all content)
rm -rf viewra-data/content-store/*
rm -rf viewra-data/transcoding/*

# Restart
make dev-setup
```

## Next Steps

1. Read the [Architecture Documentation](ARCHITECTURE.md)
2. Review the [API Reference](API_REFERENCE.md)
3. Explore the [Component Documentation](COMPONENTS.md)
4. Check out example integrations in `examples/`

## Getting Help

- GitHub Issues: Report bugs and feature requests
- Documentation: Check `docs/` directory
- Logs: Always check logs first with `make logs`