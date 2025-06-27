# ADR-0001: Separation of Playback and Transcoding Modules

Date: 2025-06-26
Status: Accepted

## Context

The original Viewra architecture had transcoding functionality tightly coupled within the playback module. This created several issues:

1. **Circular dependencies** - The playback module imported transcoding logic that also needed playback decisions
2. **Unclear boundaries** - It was difficult to determine which module was responsible for what
3. **Testing complexity** - Testing playback logic required mocking transcoding providers
4. **Scaling issues** - Both modules had different scaling requirements but were coupled together

## Decision

We decided to separate the playback and transcoding functionality into two distinct modules:

### PlaybackModule
- **Responsibility**: Makes playback decisions (direct play vs transcode)
- **Core Functions**:
  - Analyze media files
  - Compare media capabilities against device profiles
  - Determine if transcoding is needed
  - Generate playback URLs for direct play
  - Recommend transcoding parameters when needed
- **Does NOT**: Execute transcoding, manage providers, or handle sessions

### TranscodingModule
- **Responsibility**: Executes all transcoding operations
- **Core Functions**:
  - Manage transcoding providers (plugins)
  - Execute transcoding jobs
  - Track session lifecycle
  - Handle progress reporting
  - Manage temporary files and cleanup
  - Implement two-stage pipeline (encode → package)
- **Does NOT**: Make playback decisions or analyze device capabilities

## Communication Pattern

The modules communicate through a service registry pattern:

```
MediaModule
    ↓ (uses)
PlaybackService ← (registered by) PlaybackModule
    ↓ (decision: needs transcoding)
TranscodingService ← (registered by) TranscodingModule
```

## Consequences

### Positive
- **Clear separation of concerns** - Each module has a single, well-defined purpose
- **No circular dependencies** - Modules communicate through interfaces
- **Independent scaling** - Transcoding can scale separately from playback decisions
- **Better testability** - Each module can be tested in isolation
- **Cleaner API** - Each service interface is focused and minimal

### Negative
- **Additional complexity** - Service registry adds a layer of indirection
- **More modules** - Increases the number of modules to maintain
- **Migration effort** - Existing code needs to be refactored

## Implementation Details

1. **Service Interfaces** - Defined in `internal/services/interfaces.go`
2. **Service Registry** - Implemented in `internal/services/registry.go`
3. **Module Registration** - Each module registers its service during `Init()`
4. **Type Sharing** - Common types moved to `internal/types/` package

## Example Usage

```go
// Get playback decision
playbackService := services.GetService[services.PlaybackService]("playback")
decision, err := playbackService.DecidePlayback(mediaPath, deviceProfile)

if decision.ShouldTranscode {
    // Get transcoding service
    transcodingService := services.GetService[services.TranscodingService]("transcoding")
    
    // Start transcoding with recommended parameters
    params, _ := playbackService.GetRecommendedTranscodeParams(mediaPath, deviceProfile)
    session, err := transcodingService.StartTranscode(ctx, params)
}
```

## Future Considerations

- The service registry pattern can be extended to other modules (media, scanner, etc.)
- Consider adding service health checks and circuit breakers
- May want to add service versioning for backward compatibility