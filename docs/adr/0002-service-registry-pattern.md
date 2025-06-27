# ADR-0002: Service Registry Pattern for Inter-Module Communication

Date: 2025-06-26
Status: Accepted

## Context

As Viewra's module system grew, we encountered several challenges with inter-module communication:

1. **Circular import cycles** - Modules directly importing each other created dependency cycles
2. **Tight coupling** - Direct imports made it difficult to change module implementations
3. **Testing difficulties** - Mocking module dependencies required complex setup
4. **Initialization order** - Module startup order became critical and fragile

## Decision

We implemented a central service registry pattern where:

1. Each module defines a public interface for its functionality
2. Modules register their service implementations at startup
3. Other modules retrieve services through the registry by name
4. The registry provides type-safe access using Go generics

## Implementation

### Service Definition
```go
// In internal/services/interfaces.go
type PlaybackService interface {
    DecidePlayback(mediaPath string, deviceProfile *types.DeviceProfile) (*types.PlaybackDecision, error)
    // ... other methods
}
```

### Service Registration
```go
// In module's Init() method
func (m *Module) Init() error {
    service := NewPlaybackServiceImpl(m.manager)
    services.RegisterService("playback", service)
    return nil
}
```

### Service Usage
```go
// In another module
playbackService, err := services.GetService[services.PlaybackService]("playback")
if err != nil {
    return err
}
decision, err := playbackService.DecidePlayback(mediaPath, profile)
```

## Registry Features

1. **Type Safety** - Generic functions ensure compile-time type checking
2. **Thread Safety** - Registry uses mutex for concurrent access
3. **Error Handling** - Clear errors for missing or mistyped services
4. **Must Pattern** - `MustGetService` for critical dependencies during init

## Consequences

### Positive
- **Eliminates circular dependencies** - Modules only depend on interface definitions
- **Enables easy mocking** - Test code can register mock implementations
- **Flexible initialization** - Module startup order is less critical
- **Clear contracts** - Interfaces document the exact API surface
- **Runtime flexibility** - Services can be swapped at runtime if needed

### Negative
- **Runtime errors** - Service availability is checked at runtime, not compile time
- **Additional abstraction** - Adds a layer between modules
- **Service discovery** - Developers need to know service names
- **Type assertions** - Internal type assertions could fail if misused

## Best Practices

1. **Early Registration** - Register services in `Init()` before other modules need them
2. **Defensive Coding** - Always check errors when retrieving services
3. **Interface Design** - Keep interfaces focused and minimal
4. **Documentation** - Document all services in `/internal/services/README.md`
5. **Naming Convention** - Use lowercase service names matching the module

## Alternative Approaches Considered

1. **Dependency Injection Framework** - Too heavy for our needs
2. **Direct Interface Passing** - Would require complex initialization logic
3. **Event-Based Communication** - Too indirect for synchronous operations
4. **Global Variables** - Poor testability and unclear dependencies

## Future Enhancements

- Add service lifecycle hooks (start, stop, health)
- Implement service versioning for compatibility
- Add metrics and observability
- Consider lazy initialization for heavy services
- Add service dependency declaration