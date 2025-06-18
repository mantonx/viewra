# PlaybackModule Documentation

> **Comprehensive Video Transcoding & Streaming Module for Viewra**

The PlaybackModule is the core transcoding and streaming engine for the Viewra media server, providing adaptive bitrate streaming, session management, and plugin-based transcoding architecture.

## 📋 Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Quick Start](#quick-start)
- [API Documentation](#api-documentation)
- [E2E Testing](#e2e-testing)
- [Critical Issues & Fixes](#critical-issues--fixes)
- [Development](#development)
- [Production Deployment](#production-deployment)

## 🎯 Overview

### Key Features

- **🎬 Adaptive Streaming**: DASH and HLS support with intelligent container selection
- **🔧 Plugin Architecture**: Extensible transcoding backend system (FFmpeg, VAAPI, QSV, NVENC)
- **📊 Session Management**: Robust, thread-safe transcoding session handling
- **🐳 Docker Ready**: Fully containerized with volume mounting support
- **⚡ Real-time Streaming**: Progressive transcoding with live streaming capabilities
- **🛡️ Error Resilience**: Comprehensive error handling and recovery

### Browser Compatibility

| Browser | Container | Status |
|---------|-----------|--------|
| Chrome/Firefox | DASH | ✅ Fully Supported |
| Safari | HLS | ✅ Fully Supported |
| Edge | DASH | ✅ Fully Supported |

## 🏗️ Architecture

### Core Components

```
PlaybackModule
├── session.go          # Session lifecycle management
├── transcode_manager.go # Core transcoding orchestration  
├── module.go           # Module interface & routing
├── types.go            # Type definitions & models
├── planner.go          # Transcoding decision logic
├── plugin_adapter.go   # Plugin integration layer
└── api_handlers.go     # HTTP API endpoints
```

### Plugin Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   PlaybackModule │────│  Plugin Manager  │────│ FFmpeg Plugin   │
│                 │    │                  │    │                 │
│ - Session Mgmt  │    │ - Discovery      │    │ - H264/H265     │
│ - API Handlers  │    │ - Lifecycle      │    │ - DASH/HLS      │
│ - Routing       │    │ - Communication  │    │ - Hardware Accel│
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## 🚀 Quick Start

### 1. Development Setup

```bash
# Start with Docker Compose
docker-compose up -d

# The backend runs on port 8080
# Frontend proxy runs on port 5173
```

### 2. Create a Transcoding Session

```bash
curl -X POST http://localhost:8080/api/playback/start \
  -H "Content-Type: application/json" \
  -d '{
    "input_path": "/path/to/video.mp4",
    "target_codec": "h264",
    "target_container": "dash",
    "resolution": "720p",
    "bitrate": 3000
  }'
```

### 3. Stream the Content

```bash
# DASH Manifest
curl http://localhost:8080/api/playback/stream/{sessionId}/manifest.mpd

# HLS Playlist  
curl http://localhost:8080/api/playbook/stream/{sessionId}/playlist.m3u8
```

## 📚 API Documentation

### Core Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/playback/start` | Create transcoding session |
| `GET` | `/api/playback/session/{id}` | Get session status |
| `DELETE` | `/api/playback/session/{id}` | Stop session |
| `GET` | `/api/playback/sessions` | List all sessions |
| `GET` | `/api/playback/health` | System health check |
| `GET` | `/api/playback/stats` | System statistics |

### Streaming Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/playback/stream/{id}/manifest.mpd` | DASH manifest |
| `GET` | `/api/playback/stream/{id}/playlist.m3u8` | HLS playlist |
| `GET` | `/api/playback/stream/{id}/{segment}` | Media segments |

### Request Examples

<details>
<summary><strong>Create DASH Session</strong></summary>

```json
POST /api/playback/start
{
  "input_path": "/media/movie.mp4",
  "target_codec": "h264",
  "target_container": "dash",
  "resolution": "1080p",
  "bitrate": 5000,
  "audio_codec": "aac",
  "preset": "medium"
}
```

Response:
```json
{
  "id": "session_1234567890",
  "status": "starting",
  "created_at": "2024-01-01T12:00:00Z"
}
```
</details>

<details>
<summary><strong>Session Status</strong></summary>

```json
GET /api/playback/session/session_1234567890

{
  "session": {
    "id": "session_1234567890",
    "status": "running",
    "progress": 0.45,
    "backend": "ffmpeg",
    "container": "dash",
    "created_at": "2024-01-01T12:00:00Z"
  }
}
```
</details>

### Status Codes

| Code | Status | Description |
|------|--------|-------------|
| `201` | Created | Session created successfully |
| `200` | OK | Request successful |
| `404` | Not Found | Session or resource not found |
| `500` | Internal Error | Server error (often: no suitable plugin) |

## 🧪 E2E Testing

Our comprehensive E2E testing suite is organized by category:

```
e2e/
├── integration/        # Core transcoding workflows
├── error_handling/     # Error scenarios & edge cases  
├── docker/            # Container & volume integration
├── plugins/           # Plugin discovery & real FFmpeg
└── performance/       # Performance & benchmarking
```

### Running E2E Tests

```bash
# Run all E2E tests
go test -v ./internal/modules/playbackmodule/e2e/...

# Run specific test categories
go test -v ./internal/modules/playbackmodule/e2e/docker
go test -v ./internal/modules/playbackmodule/e2e/error_handling
go test -v ./internal/modules/playbackmodule/e2e/plugins
```

### Test Coverage Summary

| Test Category | Status | Coverage |
|---------------|--------|----------|
| **Docker Integration** | ✅ Complete | Volume mounting, directory config |
| **DASH/HLS Streaming** | ✅ Complete | Manifest/playlist generation |
| **Session Management** | ✅ Complete | Creation, status, cleanup |
| **Error Handling** | 🔍 Issues Found | Request validation gaps |
| **Network Resilience** | ✅ Complete | Client disconnects, concurrency |
| **Plugin Architecture** | 🔍 Issues Found | Real plugin integration |

## 🚨 Critical Issues & Fixes

Based on our comprehensive E2E testing, here are the **critical areas requiring immediate attention**:

### 🔥 **Priority 1: Request Validation (Security Critical)**

**Issue**: System accepts invalid requests with 201 status instead of proper validation

```bash
# Current behavior (WRONG)
curl -X POST /api/playback/start -H "Content-Type: text/plain" -d "{}"
# Returns: 201 Created (should be 400 Bad Request)
```

**Required Fixes**:
- [ ] Add request validation middleware
- [ ] Implement proper Content-Type checking  
- [ ] Return correct HTTP status codes
- [ ] Add input sanitization

### 🔥 **Priority 2: Plugin Integration (Functionality Critical)**

**Issue**: Real plugin environment returns 500 errors while mocks work perfectly

```bash
# Mock environment: ✅ 201 Created
# Real environment: ❌ 500 Internal Server Error: "no suitable transcoding plugin found"
```

**Required Fixes**:
- [ ] Build actual FFmpeg plugin binary
- [ ] Configure plugin discovery paths
- [ ] Set up plugin registration system
- [ ] Test plugin loading and communication

### ⚠️ **Priority 3: HTTP Method Handling**

**Issue**: Returns 404 instead of 405 for unsupported HTTP methods

```bash
curl -X PUT /api/playbook/start
# Returns: 404 Not Found (should be 405 Method Not Allowed)
```

**Required Fixes**:
- [ ] Add proper method validation
- [ ] Return 405 for unsupported methods
- [ ] Include `Allow` header in responses

### ⚠️ **Priority 4: Production Hardening**

**Issues**:
- Missing rate limiting
- No authentication/authorization
- Limited monitoring/telemetry
- No resource constraints

**Required Fixes**:
- [ ] Add rate limiting middleware
- [ ] Implement authentication system
- [ ] Add comprehensive monitoring
- [ ] Set resource limits (CPU, memory, concurrent sessions)

## 💻 Development

### Prerequisites

- Go 1.21+
- Docker & Docker Compose
- FFmpeg (for real plugin testing)

### Development Workflow

```bash
# 1. Start development environment
docker-compose up -d

# 2. Run tests
go test -v ./internal/modules/playbackmodule

# 3. Run E2E tests  
go test -v ./internal/modules/playbackmodule/e2e/...

# 4. Check logs
docker-compose logs backend
```

### Adding New Features

1. **Add Core Logic**: Update `module.go`, `session.go`, or `transcode_manager.go`
2. **Add API Endpoints**: Update `api_handlers.go` and routing in `module.go`
3. **Add Types**: Update `types.go` for new data structures
4. **Add Tests**: Create unit tests and E2E tests
5. **Update Documentation**: Update this README and API docs

## 🚀 Production Deployment

### Docker Configuration

```yaml
# docker-compose.yml
services:
  backend:
    image: viewra-backend:latest
    volumes:
      - ./viewra-data:/viewra-data  # Critical: transcoding storage
    environment:
      - VIEWRA_TRANSCODING_DIR=/viewra-data/transcoding
      - VIEWRA_PLUGINS_DIR=/app/plugins
    ports:
      - "8080:8080"
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VIEWRA_TRANSCODING_DIR` | Transcoding output directory | `/viewra-data/transcoding` |
| `VIEWRA_PLUGINS_DIR` | Plugin discovery directory | `/app/plugins` |
| `VIEWRA_MAX_SESSIONS` | Maximum concurrent sessions | `10` |

### Production Checklist

- [ ] **Security**: Fix request validation (Priority 1)
- [ ] **Plugins**: Build and deploy real FFmpeg plugin (Priority 2)  
- [ ] **Monitoring**: Add telemetry and health checks
- [ ] **Performance**: Set resource limits and optimize
- [ ] **Backup**: Configure transcoding data persistence
- [ ] **Scaling**: Test horizontal scaling capabilities

## 📖 Additional Documentation

- [`docs/api.md`](docs/api.md) - Detailed API reference
- [`docs/implementation.md`](docs/implementation.md) - Implementation details
- [`e2e/README.md`](e2e/README.md) - E2E testing guide

## 🤝 Contributing

1. Follow the existing code patterns
2. Add comprehensive tests for new features
3. Update documentation
4. Ensure E2E tests pass
5. Address security considerations

## 📄 License

Part of the Viewra Media Server project. 