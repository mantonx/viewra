# Transcoding Plugin Migration Guide

This guide helps developers migrate existing transcoding plugins to use the new generic provider interface.

## Overview

The transcoding system has been generalized to support multiple backends beyond FFmpeg:
- VAAPI (Intel/AMD Linux)
- QSV (Intel Quick Sync)
- NVENC (NVIDIA)
- VideoToolbox (macOS)
- Cloud services (AWS, Google, etc.)

## Migration Status

### ✅ FFmpeg Plugin - COMPLETED

The FFmpeg plugin has been fully updated to implement the TranscodingProvider interface with:

1. **Real Progress Tracking**:
   - Captures FFmpeg stderr output
   - Parses progress using the progress converter
   - Reports real-time transcoding progress

2. **Proper Implementation**:
   - Implements all TranscodingProvider methods
   - Uses standardized quality mapping (0-100%)
   - Supports hardware acceleration detection
   - Dashboard integration works

3. **Fixed Issues**:
   - Progress tracking now works (was returning dummy values)
   - Compilation errors resolved
   - Removed unused interfaces and types

### Implementation Details

## Overview of Changes

The transcoding system has been completely redesigned to:
- Remove all legacy interfaces and backwards compatibility
- Use a clean, minimal API with no tech debt
- Standardize on generic quality/speed settings instead of provider-specific options
- Integrate with the new database-backed session management

## Key Interface Changes

### Old Interface (REMOVED)
```go
// TranscodingService - NO LONGER EXISTS
type TranscodingService interface {
    GetCapabilities(ctx context.Context) (*TranscodingCapabilities, error)
    StartTranscode(ctx context.Context, req *TranscodeRequest) (*TranscodeSession, error)
    GetTranscodeSession(ctx context.Context, sessionID string) (*TranscodeSession, error)
    StopTranscode(ctx context.Context, sessionID string) error
    ListActiveSessions(ctx context.Context) ([]*TranscodeSession, error)
    GetTranscodeStream(ctx context.Context, sessionID string) (io.ReadCloser, error)
}
```

### New Interface
```go
// TranscodingProvider - The ONLY interface to implement
type TranscodingProvider interface {
    // Provider information
    GetInfo() ProviderInfo
    
    // Capabilities
    GetSupportedFormats() []ContainerFormat
    GetHardwareAccelerators() []HardwareAccelerator
    GetQualityPresets() []QualityPreset
    
    // Execution
    StartTranscode(ctx context.Context, req TranscodeRequest) (*TranscodeHandle, error)
    GetProgress(handle *TranscodeHandle) (*TranscodingProgress, error)
    StopTranscode(handle *TranscodeHandle) error
    
    // Dashboard integration
    GetDashboardSections() []DashboardSection
    GetDashboardData(sectionID string) (interface{}, error)
    ExecuteDashboardAction(actionID string, params map[string]interface{}) error
}
```

## Type Changes

### TranscodeRequest
```go
// Old (with legacy fields)
type TranscodeRequest struct {
    InputPath     string
    OutputPath    string
    CodecOpts     *CodecOptions    // REMOVED
    DeviceProfile *DeviceProfile   // REMOVED
    Environment   map[string]string // REMOVED
}

// New (clean)
type TranscodeRequest struct {
    InputPath     string
    OutputPath    string
    SessionID     string
    
    // Generic settings
    Quality       int              // 0-100
    SpeedPriority SpeedPriority    // "fastest", "balanced", "quality"
    
    // Format settings
    Container     string           // mp4, mkv, dash, hls
    VideoCodec    string           // h264, h265, vp9
    AudioCodec    string           // aac, opus, mp3
    
    // Optional transforms
    Resolution    *VideoResolution // nil = keep original
    FrameRate     *float64         // nil = keep original
    Seek          time.Duration    // Start position
    Duration      time.Duration    // Encode duration
    
    // Hardware preferences
    PreferHardware bool
    HardwareType   HardwareType
    
    // Provider-specific overrides
    ProviderSettings json.RawMessage
}
```

### Session Management
```go
// Old
type TranscodeSession struct {
    ID       string
    Request  *TranscodeRequest
    Status   TranscodeStatus
    Progress *TranscodingProgress
    // ... many other fields
}

// New
type TranscodeHandle struct {
    SessionID   string
    Provider    string
    StartTime   time.Time
    Directory   string
    ProcessID   int
    Context     context.Context
    CancelFunc  context.CancelFunc
    PrivateData interface{} // Provider-specific data
}
```

## Migration Steps

### 1. Update Plugin Interface Implementation

Replace the old method that returned `TranscodingService`:
```go
// Old
func (p *MyPlugin) TranscodingService() plugins.TranscodingService {
    return newTranscodingServiceAdapter(p)
}

// New
func (p *MyPlugin) TranscodingProvider() plugins.TranscodingProvider {
    return p
}
```

### 2. Implement TranscodingProvider Methods

Example implementation:
```go
// GetInfo returns provider information
func (p *FFmpegTranscoderPlugin) GetInfo() plugins.ProviderInfo {
    return plugins.ProviderInfo{
        ID:          "ffmpeg_transcoder",
        Name:        "FFmpeg Transcoder",
        Description: "High-performance transcoding with hardware acceleration",
        Version:     "1.0.0",
        Author:      "Your Name",
        Priority:    100,
    }
}

// GetSupportedFormats returns supported container formats
func (p *FFmpegTranscoderPlugin) GetSupportedFormats() []plugins.ContainerFormat {
    return []plugins.ContainerFormat{
        {
            Format:      "mp4",
            MimeType:    "video/mp4",
            Extensions:  []string{".mp4"},
            Description: "MPEG-4 Container",
            Adaptive:    false,
        },
        {
            Format:      "dash",
            MimeType:    "application/dash+xml",
            Extensions:  []string{".mpd", ".m4s"},
            Description: "MPEG-DASH Adaptive Streaming",
            Adaptive:    true,
        },
        // ... more formats
    }
}
```

### 3. Update Quality Handling

Instead of provider-specific codec options, map the generic 0-100 quality to your provider's settings:

```go
// Convert 0-100 quality to FFmpeg CRF
func mapQualityToCRF(quality int, codec string) int {
    // Higher quality = lower CRF
    crf := 51 - int(float64(quality)/100.0*51)
    
    // Adjust for different codecs
    switch codec {
    case "h265", "hevc":
        crf = crf + 5 // HEVC uses higher CRF values
    case "vp9":
        crf = int(63 - float64(quality)/100.0*63) // VP9 uses 0-63 scale
    }
    
    return crf
}
```

### 4. Update Speed Priority Handling

Map the generic speed priority to your provider's presets:

```go
func mapSpeedPriorityToPreset(priority plugins.SpeedPriority) string {
    switch priority {
    case plugins.SpeedPriorityFastest:
        return "ultrafast"
    case plugins.SpeedPriorityBalanced:
        return "medium"
    case plugins.SpeedPriorityQuality:
        return "slow"
    default:
        return "medium"
    }
}
```

### 5. Handle Session State with TranscodeHandle

Instead of managing full session objects, use lightweight handles:

```go
func (p *MyPlugin) StartTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
    // Create handle
    handle := &plugins.TranscodeHandle{
        SessionID:   req.SessionID,
        Provider:    "my_provider",
        StartTime:   time.Now(),
        Directory:   sessionDir,
        Context:     ctx,
        CancelFunc:  cancelFunc,
        PrivateData: internalJobID, // Your internal tracking
    }
    
    // Store handle for later retrieval
    p.activeHandles[req.SessionID] = handle
    
    return handle, nil
}
```

### 6. Implement Dashboard Integration

Provide custom dashboard sections for monitoring:

```go
func (p *MyPlugin) GetDashboardSections() []plugins.DashboardSection {
    return []plugins.DashboardSection{
        {
            ID:          "my_plugin_status",
            Title:       "My Plugin Status",
            Description: "Current transcoding operations",
            Icon:        "film",
            Priority:    1,
        },
    }
}

func (p *MyPlugin) GetDashboardData(sectionID string) (interface{}, error) {
    switch sectionID {
    case "my_plugin_status":
        return map[string]interface{}{
            "active_sessions": len(p.activeHandles),
            "version":         p.version,
        }, nil
    default:
        return nil, fmt.Errorf("unknown section: %s", sectionID)
    }
}
```

## Common Pitfalls to Avoid

1. **Don't use CodecOptions** - Use the direct fields on TranscodeRequest
2. **Don't use DeviceProfile** - The playback module handles device detection
3. **Don't implement session storage** - The core system handles this in the database
4. **Don't use Environment map** - Use provider-specific settings if needed
5. **Don't reference Backend field** - Use Provider instead

## Testing Your Migration

1. Ensure your plugin implements `TranscodingProvider()`
2. Test quality mapping with different values (0, 50, 100)
3. Test all supported container formats
4. Verify hardware detection if applicable
5. Check dashboard integration displays correctly

## Example: Complete FFmpeg Plugin Structure

```
backend/data/plugins/ffmpeg_transcoder/
├── main.go                    # Entry point
├── go.mod
├── plugin.cue                 # Configuration schema
├── internal/
│   ├── plugin/
│   │   └── plugin.go          # Main plugin implementation
│   ├── quality/
│   │   └── mapper.go          # Quality mapping logic
│   ├── progress/
│   │   └── converter.go       # Progress parsing
│   └── services/
│       ├── transcoding.go     # Core transcoding logic
│       ├── hardware.go        # Hardware detection
│       └── cleanup.go         # Session cleanup
└── README.md
```

## Resources

- [Plugin SDK Reference](./backend/pkg/plugins/)
- [FFmpeg Plugin Example](./backend/data/plugins/ffmpeg_transcoder/)
- [Provider Interface](./backend/pkg/plugins/provider.go)
- [Transcoding Types](./backend/pkg/plugins/transcoding.go) 