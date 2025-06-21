# FFmpeg Transcoder Plugin

A modular and extensible video transcoding plugin for Viewra using FFmpeg.

## Architecture

This plugin has been organized following the TMDB enricher pattern with clean separation of concerns:

```
ffmpeg_transcoder/
├── main.go                    # Plugin entry point and interface implementation
├── plugin.cue                 # Plugin configuration schema
├── go.mod                     # Module dependencies
├── README.md                  # This file
│
├── internal/                  # Internal implementation packages
│   ├── config/                # Configuration management
│   │   └── config.go          # Configuration types and validation
│   │
│   ├── models/                # Data models and types
│   │   └── models.go          # Transcoding job and session models
│   │
│   └── services/              # Business logic services
│       ├── interfaces.go      # Service interface definitions
│       └── transcoding.go     # Main transcoding service implementation
│
└── version.go                 # Version information (optional)
```

## Core Principles

### 1. Complete Modularity
- No direct imports from main application code
- Self-contained with own `go.mod`
- All dependencies explicitly declared

### 2. Plugin SDK Communication
- Uses HashiCorp go-plugin framework
- Implements `plugins.Implementation` interface
- Communicates via plugin SDK interfaces only

### 3. Clean Architecture
- Configuration layer (`internal/config`)
- Models layer (`internal/models`) 
- Services layer (`internal/services`)
- Clear interface definitions

### 4. Extensibility
- Service interfaces allow for easy testing and mocking
- Pluggable FFmpeg executor implementation
- Configurable via CUE schema

## Configuration

The plugin configuration is defined in `plugin.cue` with comprehensive settings:

- **FFmpeg Settings**: Binary path, threads, priority
- **Transcoding Defaults**: Quality, preset, codecs, output directory
- **Session Management**: Concurrent limits, cleanup policies, timeouts
- **Performance Monitoring**: Metrics collection, resource monitoring
- **Debug Options**: Logging levels, command saving, FFmpeg output

## Services

### TranscodingService
Main service that handles:
- Starting/stopping transcoding jobs
- Job progress monitoring
- System statistics
- Cleanup operations

### FFmpegExecutor
Low-level FFmpeg execution:
- Command execution with progress callbacks
- Version detection and validation
- Media file probing
- Installation verification

## Usage

The plugin is automatically loaded by the Viewra plugin system. It provides:

1. **Transcoding Capabilities**: Multi-format video transcoding
2. **Session Management**: Concurrent job handling with limits
3. **Progress Monitoring**: Real-time transcoding progress
4. **Health Monitoring**: System health and performance metrics

## Development

### Building
```bash
# From the plugin directory
go mod tidy
go build -o ffmpeg_transcoder .
```

### Testing
```bash
go test ./internal/...
```

### Configuration Validation
The plugin validates its configuration on startup and will fail to start with invalid settings.

## Integration

This plugin integrates with Viewra's playback module to provide adaptive streaming transcoding for video content. It supports:

- **Multiple Codecs**: H.264, H.265, VP8, VP9, AV1
- **Multiple Containers**: MP4, MKV, WebM, AVI
- **Quality Control**: CRF-based quality settings
- **Concurrent Processing**: Configurable session limits
- **Progress Tracking**: Real-time progress updates

The plugin follows Viewra's modular architecture principles, ensuring it can be developed, tested, and deployed independently while maintaining clean integration with the core system. 