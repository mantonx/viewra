# Transcoding SDK Structure

This SDK provides a modular architecture for video transcoding with FFmpeg. The codebase has been organized into focused packages for better maintainability and clarity.

## Directory Structure

```
transcoding/
├── abr/                 # Adaptive Bitrate (ABR) ladder generation
│   └── generator.go     # Generates optimized encoding profiles for different devices
│
├── ffmpeg/             # FFmpeg command building and utilities
│   ├── args.go         # Builds FFmpeg command arguments
│   └── ffmpeg.go       # FFmpeg-specific utilities
│
├── process/            # Process management and monitoring
│   ├── monitor.go      # Monitors transcoding processes
│   └── registry.go     # Tracks active processes globally
│
├── session/            # Session lifecycle management
│   └── manager.go      # Manages transcoding sessions
│
├── types/              # Shared types and interfaces
│   └── types.go        # Common types used across packages
│
├── validation/         # Output validation for reliable playback
│   └── validator.go    # Validates DASH/HLS output quality
│
├── config/             # Configuration management
│   └── config.go       # Configuration structures
│
├── hardware/           # Hardware acceleration detection
│   └── detector.go     # Detects available hardware encoders
│
├── progress/           # Progress tracking utilities
│   └── converter.go    # Converts FFmpeg progress output
│
├── quality/            # Quality mapping and optimization
│   └── mapper.go       # Maps quality settings
│
├── services/           # Service interfaces and implementations
│   ├── cleanup_service.go
│   ├── interfaces.go
│   └── session_manager.go
│
├── base.go             # Base transcoding interfaces
├── transcoder.go       # Main transcoder implementation
└── types.go            # Legacy types (being migrated to types/)
```

## Package Descriptions

### `abr/`
Handles adaptive bitrate ladder generation for streaming. Creates optimized encoding profiles based on:
- Source resolution
- Target devices (mobile, desktop, TV)
- Network conditions (2G to fiber)
- Quality preferences

### `ffmpeg/`
Builds and manages FFmpeg commands:
- Argument generation for different codecs
- Container-specific settings (DASH, HLS, MP4)
- Quality and performance optimization
- Keyframe alignment for smooth playback

### `process/`
Manages FFmpeg process lifecycle:
- Process registration and tracking
- Graceful shutdown with timeout
- Resource monitoring
- Zombie process cleanup

### `session/`
Handles transcoding session management:
- Session creation and tracking
- Progress monitoring
- Concurrent session handling
- Session cleanup and statistics

### `validation/`
Ensures output meets quality standards:
- DASH manifest validation
- Segment duration checks
- GOP alignment verification
- Codec compatibility validation

### `types/`
Shared types and interfaces used across the SDK:
- Common data structures
- Interface definitions
- Constants and enums

## Usage Example

```go
import (
    "github.com/viewra/viewra/sdk/transcoding"
    "github.com/viewra/viewra/sdk/transcoding/types"
)

// Create a transcoder
transcoder := transcoding.NewTranscoder("my-transcoder", "My Transcoder", "1.0", "Author", 100)

// Start transcoding
req := types.TranscodeRequest{
    InputPath:    "/path/to/input.mp4",
    OutputPath:   "/path/to/output",
    Container:    "dash",
    VideoCodec:   "libx264",
    AudioCodec:   "aac",
    Quality:      80,
    EnableABR:    true,
}

handle, err := transcoder.StartTranscode(ctx, req)
if err != nil {
    log.Fatal(err)
}

// Monitor progress
progress, err := transcoder.GetProgress(handle)
if err != nil {
    log.Printf("Progress: %.2f%%", progress.PercentComplete)
}
```

## Key Features

1. **Modular Architecture**: Each package has a focused responsibility
2. **Hardware Acceleration**: Automatic detection and usage when available
3. **ABR Support**: Intelligent bitrate ladder generation
4. **Process Safety**: Proper process management and cleanup
5. **Validation**: Ensures output quality for reliable playback
6. **Flexible Configuration**: Supports various codecs and containers

## Contributing

When adding new features:
1. Place code in the appropriate package
2. Follow the existing patterns
3. Add tests for new functionality
4. Update this README if adding new packages