# ADR 026: App Package Restructuring

## Status

Proposed

## Date

December 2, 2025

## Context

The ViewRA codebase has grown organically and now has several structural pain points:

1. **`NewServer` has 30+ parameters** - Individual use cases passed instead of an aggregate
2. **Inconsistent handler creation** - Some handlers created in `NewServer`, others in `app/handlers`
3. **Wasted rebuilds** - `startup.go` rebuilds repos/services/usecases just to recover stuck scans
4. **No clear dependency graph** - Hard to understand what depends on what

This restructuring is a prerequisite for cleanly adding authentication (ADR 028) and settings (ADR 029).

## Decision

Restructure the `internal/app/` package to provide clear dependency wiring and a simplified server initialization.

### Target Structure

```text
internal/app/
├── config/           # Split config by domain
│   ├── server.go
│   ├── database.go
│   ├── transcoding.go
│   └── scanning.go
├── wire/             # Dependency wiring
│   ├── repositories.go
│   ├── services.go
│   ├── usecases.go
│   └── handlers.go
├── tasks/            # Scheduled task registration
│   ├── registry.go
│   └── cleanup.go
└── container.go      # Main container
```

### Key Changes

#### 1. Create `api.Handlers` Aggregate Struct

```go
// internal/api/handlers.go
type Handlers struct {
    Health     *HealthHandler
    Libraries  *LibraryHandler
    Media      *MediaHandler
    Transcode  *TranscodeHandler
    Progress   *ProgressHandler
    // ... all handlers
}
```

#### 2. Simplify `NewServer`

Before:
```go
func NewServer(
    config *Config,
    logger *slog.Logger,
    healthHandler *HealthHandler,
    libraryHandler *LibraryHandler,
    mediaHandler *MediaHandler,
    // ... 30+ more parameters
) *Server
```

After:
```go
func NewServer(config *Config, logger *slog.Logger, handlers *Handlers) *Server
```

#### 3. Move Wiring to `app/wire/`

```go
// internal/app/wire/handlers.go
func NewHandlers(usecases *UseCases, logger *slog.Logger) *api.Handlers {
    return &api.Handlers{
        Health:    handlers.NewHealthHandler(usecases.Health),
        Libraries: handlers.NewLibraryHandler(usecases.Library, logger),
        // ...
    }
}
```

#### 4. Extract Task Registration

```go
// internal/app/tasks/registry.go
type TaskRegistry struct {
    scheduler *scheduler.Scheduler
    tasks     []Task
}

func (r *TaskRegistry) RegisterAll(container *Container) {
    r.Register(NewTranscodeCleanupTask(container.TranscodeService))
    r.Register(NewSessionCleanupTask(container.SessionService))  // Future
    // ...
}
```

#### 5. Fix Startup to Reuse Container

Current `startup.go` rebuilds repositories and services just to call `RecoverStuckScans`. Instead:

```go
// internal/app/startup.go
func RunStartupTasks(container *Container) error {
    // Reuse existing services from container
    return container.ScanService.RecoverStuckScans(ctx)
}
```

### Container Interface

```go
// internal/app/container.go
type Container struct {
    Config      *Config
    Logger      *slog.Logger
    DB          *database.DB

    // Repositories
    Repos       *wire.Repositories

    // Services
    Services    *wire.Services

    // Use Cases
    UseCases    *wire.UseCases

    // Handlers
    Handlers    *api.Handlers

    // Tasks
    Tasks       *tasks.TaskRegistry
}

func NewContainer(config *Config, logger *slog.Logger) (*Container, error) {
    // Build dependency graph in order
}

func (c *Container) Close() error {
    // Cleanup in reverse order
}
```

## Consequences

### Positive

- Clear dependency graph
- Single place to understand wiring (`app/wire/`)
- Simplified server initialization
- No duplicate service creation
- Easier to add new handlers/services
- Foundation for auth and settings

### Negative

- Significant refactoring effort (2-3 days)
- Temporary code churn
- Need to update tests that mock individual handlers

### Neutral

- No external API changes
- No database changes
- No user-visible changes

## Alternatives Considered

### Keep Current Structure

Could add auth without restructuring, but:
- Would add more parameters to already-bloated `NewServer`
- Would continue pattern of duplicate service creation
- Technical debt compounds

### Dependency Injection Framework (Wire, Fx)

More automated but:
- Adds external dependency
- Learning curve for contributors
- Manual wiring is clear enough for current size

## Implementation

1. Create `api.Handlers` aggregate struct
2. Create `app/wire/` package with repository/service/usecase wiring
3. Update `NewServer` to accept `*Handlers`
4. Create `app/tasks/` for scheduled task registration
5. Update `startup.go` to use container
6. Update tests

**Effort**: 2-3 days

## References

- Current: `internal/app/`, `internal/api/`, `cmd/viewra/bootstrap/`
- Related: [ADR 028 - User Authentication](028-user-authentication.md)
- Related: [ADR 029 - Settings Infrastructure](029-settings-infrastructure.md)
