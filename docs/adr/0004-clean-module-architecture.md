# ADR-0004: Clean Module Architecture

Date: 2024-01-08
Status: Accepted

## Context

The initial module implementation had several architectural issues:
- Inconsistent file organization across modules
- Mixed responsibilities within single files
- Unclear separation between business logic and infrastructure
- Lack of consistent patterns for error handling and service interfaces

## Decision

We have adopted a clean architecture pattern for all modules with the following structure:

### Directory Structure

```
internal/modules/{module_name}/
├── api/              # HTTP handlers and routes
├── core/             # Business logic and domain models
├── errors/           # Module-specific error handling
├── models/           # Database models
├── repository/       # Data access layer
├── service/          # Service layer (thin wrapper)
├── types/            # Shared types and interfaces
├── utils/            # Module-specific utilities
├── module.go         # Module registration and lifecycle
└── README.md         # Module documentation
```

### Core Principles

1. **Domain-Driven Design**: Core business logic lives in the `core/` directory
2. **Clean Dependencies**: Dependencies flow inward (API → Service → Core)
3. **Module Independence**: Each module is self-contained with its own error handling
4. **Consistent Patterns**: All modules follow the same organizational structure

### Implementation Details

#### Module Registration
```go
// Auto-registration pattern
func init() {
    Register()
}

func Register() {
    module := &Module{}
    modulemanager.Register(module)
}
```

#### Service Layer
- Thin wrapper around core business logic
- Uses lowercase unexported implementation types
- Delegates to managers in the core layer

#### Error Handling
- Each module has its own `errors/` package
- Structured errors with context and classification
- Domain-specific sentinel errors
- Helper functions for error creation

#### API Organization
- Domain-specific handler files (e.g., `session_handlers.go`, `library_handlers.go`)
- Clear separation of concerns
- Consistent error responses

## Consequences

### Positive
- Clear, consistent structure across all modules
- Easy to understand and navigate codebase
- Reduced coupling between modules
- Better testability
- Easier onboarding for new developers

### Negative
- Initial refactoring effort required
- More files to manage (but better organized)
- Need to maintain consistency across modules

## Examples

### Media Module
- **Core**: LibraryManager, MetadataManager
- **Service**: Thin MediaService wrapper
- **API**: Separate handlers for libraries, media files, metadata

### Playback Module
- **Core**: DecisionEngine, SessionManager, StreamingManager
- **Service**: PlaybackService coordinating core components
- **API**: Handlers for playback decisions, sessions, streaming

### Transcoding Module
- **Core**: TranscodingManager, ProviderRegistry, SessionStore
- **Service**: TranscodingService wrapper
- **API**: Separate handlers for sessions, providers, stats

## Migration Guide

When refactoring existing modules or creating new ones:

1. Create the directory structure
2. Move business logic to `core/`
3. Create thin service wrapper
4. Organize API handlers by domain
5. Implement module-specific error handling
6. Update module registration to use init() pattern