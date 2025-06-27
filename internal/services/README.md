# Service Registry

The service registry provides a clean architectural pattern for inter-module communication in Viewra. It allows modules to expose their functionality through well-defined interfaces without creating circular dependencies.

## Overview

The service registry pattern provides:
- **Clean separation of concerns** - Modules communicate through interfaces, not concrete implementations
- **Testability** - Services can be easily mocked for testing
- **No circular dependencies** - Modules don't import each other directly
- **Clear API boundaries** - Each service has a well-defined interface
- **Consistent pattern** - All modules follow the same pattern

## Available Services

### PlaybackService
Handles playback decisions and media analysis.
```go
service, err := services.GetService[services.PlaybackService]("playback")
```

**Interface:**
- `DecidePlayback(mediaPath string, deviceProfile *types.DeviceProfile) (*types.PlaybackDecision, error)`
- `GetMediaInfo(mediaPath string) (*types.MediaInfo, error)`
- `ValidatePlayback(mediaPath string, deviceProfile *types.DeviceProfile) error`
- `GetSupportedFormats(deviceProfile *types.DeviceProfile) []string`
- `GetRecommendedTranscodeParams(mediaPath string, deviceProfile *types.DeviceProfile) (*plugins.TranscodeRequest, error)`

### TranscodingService
Manages transcoding operations and providers.
```go
service, err := services.GetService[services.TranscodingService]("transcoding")
```

**Interface:**
- `StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*database.TranscodeSession, error)`
- `GetSession(sessionID string) (*database.TranscodeSession, error)`
- `StopSession(sessionID string) error`
- `GetProgress(sessionID string) (*plugins.TranscodingProgress, error)`
- `GetStats() (*types.TranscodingStats, error)`
- `GetProviders() []plugins.ProviderInfo`

## Usage Examples

### Registering a Service

Services should be registered during module initialization:

```go
func (m *Module) Init() error {
    // Create your service implementation
    service := NewMyServiceImpl(m.manager)
    
    // Register it with the service registry
    services.RegisterService("myservice", service)
    
    return nil
}
```

### Using a Service

To use a service from another module:

```go
// Get the service with type safety
transcodingService, err := services.GetService[services.TranscodingService]("transcoding")
if err != nil {
    // Handle service not found
    return err
}

// Use the service
session, err := transcodingService.StartTranscode(ctx, request)
```

### Must Get Pattern

For initialization code where the service must exist:

```go
// This will panic if the service is not found
transcodingService := services.MustGetService[services.TranscodingService]("transcoding")
```

## Best Practices

1. **Define Clear Interfaces**: Create focused interfaces that represent the public API of your module
2. **Register Early**: Register services during module initialization before other modules might need them
3. **Handle Errors**: Always check for errors when retrieving services unless using MustGetService
4. **No Circular Dependencies**: Design your interfaces to avoid circular dependencies between modules
5. **Use Type Parameters**: Always use the generic type parameters for compile-time type safety

## Adding a New Service

1. Define your service interface in `interfaces.go`:
```go
type MyService interface {
    DoSomething(ctx context.Context, input string) (string, error)
    GetStatus() (*Status, error)
}
```

2. Implement the interface in your module:
```go
type MyServiceImpl struct {
    manager *Manager
}

func (s *MyServiceImpl) DoSomething(ctx context.Context, input string) (string, error) {
    // Implementation
}
```

3. Register during module initialization:
```go
func (m *Module) Init() error {
    service := NewMyServiceImpl(m.manager)
    services.RegisterService("myservice", service)
    return nil
}
```

4. Document the service in this README

## Module Communication Flow

```
MediaModule → [Service Registry] → PlaybackModule
     ↓                                    ↓
[MediaService]                    [PlaybackService]
                                         ↓
                                [Service Registry]
                                         ↓
                              [TranscodingService]
                                         ↓
                              TranscodingModule
```

## Error Handling

The service registry returns specific errors:
- **Service not found**: The requested service hasn't been registered yet
- **Wrong type**: The service exists but doesn't match the requested type

Always handle these errors appropriately in production code.