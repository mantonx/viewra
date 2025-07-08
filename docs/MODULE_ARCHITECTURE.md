# Module Architecture Guide

This document provides a comprehensive guide to the module architecture used in Viewra, focusing on the three core modules that exemplify our clean architecture approach.

## Table of Contents
- [Overview](#overview)
- [Core Principles](#core-principles)
- [Module Structure](#module-structure)
- [Media Module](#media-module)
- [Playback Module](#playback-module)  
- [Transcoding Module](#transcoding-module)
- [Creating New Modules](#creating-new-modules)
- [Best Practices](#best-practices)

## Overview

Viewra uses a modular architecture where each module is a self-contained unit responsible for a specific domain. Modules follow clean architecture principles with clear separation of concerns and well-defined boundaries.

## Core Principles

### 1. Domain-Driven Design
Each module represents a bounded context with its own domain model, business rules, and language.

### 2. Dependency Inversion
- High-level modules don't depend on low-level modules
- Both depend on abstractions (interfaces)
- Dependencies flow inward: API → Service → Core

### 3. Single Responsibility
Each layer has a single, well-defined purpose:
- **API**: HTTP request/response handling
- **Service**: Orchestration and coordination
- **Core**: Business logic and rules
- **Repository**: Data persistence

### 4. Module Independence
Modules communicate through well-defined service interfaces, not direct dependencies.

## Module Structure

### Standard Directory Layout

```
internal/modules/{module_name}/
├── api/              # HTTP handlers and routes
│   ├── handlers.go   # Base handler struct
│   ├── routes.go     # Route registration
│   └── *_handlers.go # Domain-specific handlers
├── core/             # Business logic
│   └── {domain}/     # Domain-specific packages
│       ├── manager.go
│       └── types.go
├── errors/           # Error handling
│   └── errors.go     # Module-specific errors
├── models/           # Database models
│   └── models.go     # GORM models
├── repository/       # Data access
│   └── repository.go # Database operations
├── service/          # Service layer
│   └── service.go    # Service implementation
├── types/            # Shared types
│   ├── interfaces.go # Service interfaces
│   └── *.go          # Domain types
├── utils/            # Utilities
│   └── helpers.go    # Module helpers
├── module.go         # Module definition
└── README.md         # Documentation
```

### Layer Responsibilities

#### API Layer (`/api`)
- HTTP request validation
- Response formatting
- Route registration
- No business logic

#### Core Layer (`/core`)
- Business logic implementation
- Domain models
- Business rule enforcement
- Algorithm implementation

#### Service Layer (`/service`)
- Thin orchestration layer
- Coordinates between core components
- Implements service interface
- Transaction management

#### Repository Layer (`/repository`)
- Database operations
- Query building
- Data mapping
- No business logic

#### Types Layer (`/types`)
- Shared interfaces
- Data transfer objects
- Request/response types
- Configuration types

#### Errors Layer (`/errors`)
- Module-specific error types
- Error classification
- Sentinel errors
- Error helpers

## Media Module

The media module manages media libraries, files, and metadata.

### Structure
```
mediamodule/
├── api/
│   ├── handlers.go
│   ├── routes.go
│   ├── library_handlers.go
│   ├── media_handlers.go
│   └── metadata_handlers.go
├── core/
│   ├── library/
│   │   ├── manager.go      # Library business logic
│   │   └── scanner.go      # Scanning logic
│   └── metadata/
│       ├── manager.go      # Metadata management
│       └── extractor.go    # Metadata extraction
├── errors/
│   └── errors.go          # Media-specific errors
├── models/
│   └── models.go          # Media database models
├── repository/
│   ├── library_repository.go
│   └── media_repository.go
├── service/
│   └── media_service.go   # MediaService implementation
├── types/
│   ├── interfaces.go      # MediaService interface
│   ├── library.go         # Library types
│   └── metadata.go        # Metadata types
└── module.go
```

### Key Components

#### LibraryManager
- Manages media libraries
- Handles scanning operations
- Enforces library policies

#### MetadataManager  
- Extracts and manages metadata
- Coordinates with enrichment services
- Handles metadata updates

#### MediaService
- Thin wrapper providing the service interface
- Delegates to managers
- Registered in service registry

### API Endpoints
- `GET /api/v1/libraries` - List libraries
- `POST /api/v1/libraries` - Create library
- `GET /api/v1/media/files` - List media files
- `GET /api/v1/media/files/:id` - Get media details

## Playback Module

The playback module handles playback decisions, streaming, and session management.

### Structure
```
playbackmodule/
├── api/
│   ├── handlers.go
│   ├── routes.go
│   ├── decision_handlers.go
│   ├── session_handlers.go
│   └── stream_handlers.go
├── core/
│   ├── cleanup/
│   │   └── cleanup_manager.go
│   ├── history/
│   │   ├── history_manager.go
│   │   └── recommendation_tracker.go
│   ├── playback/
│   │   ├── decision_engine.go
│   │   ├── manager.go
│   │   └── transcode_deduplicator.go
│   ├── session/
│   │   └── session_manager.go
│   └── streaming/
│       ├── manager.go
│       └── progressive_handler.go
├── errors/
│   └── errors.go
├── models/
│   └── models.go
├── service/
│   └── playback_service.go
├── types/
│   ├── interfaces.go
│   ├── device.go
│   └── session.go
└── module.go
```

### Key Components

#### DecisionEngine
- Analyzes media compatibility
- Determines playback method
- Optimizes for device capabilities

#### SessionManager
- Tracks active playback sessions
- Manages session lifecycle
- Handles concurrent sessions

#### StreamingManager
- Progressive download handling
- Buffering strategies
- Stream optimization

### API Endpoints
- `POST /api/v1/playback/decide` - Get playback decision
- `GET /api/v1/playback/sessions` - List sessions
- `GET /api/v1/playback/stream/:id` - Stream content

## Transcoding Module

The transcoding module manages media transcoding through various providers.

### Structure
```
transcodingmodule/
├── api/
│   ├── handlers.go
│   ├── routes.go
│   ├── session_handlers.go
│   ├── transcode_handlers.go
│   ├── provider_handlers.go
│   └── stats_handlers.go
├── core/
│   ├── pipeline/
│   │   └── processor.go
│   ├── session/
│   │   └── store.go
│   ├── storage/
│   │   ├── content_store.go
│   │   └── content_migration.go
│   └── transcoding/
│       ├── manager.go
│       ├── provider_registry.go
│       └── queue_manager.go
├── errors/
│   └── errors.go
├── models/
│   └── models.go
├── repository/
│   └── repository.go
├── service/
│   └── transcoding_service.go
├── types/
│   ├── config.go
│   ├── interfaces.go
│   ├── profile.go
│   ├── request.go
│   ├── result.go
│   ├── session.go
│   └── status.go
└── module.go
```

### Key Components

#### TranscodingManager
- Orchestrates transcoding operations
- Manages provider selection
- Handles queue management

#### ProviderRegistry
- Manages transcoding providers
- Provider health monitoring
- Load balancing

#### ContentStore
- Content-addressable storage
- Deduplication
- Cache management

### API Endpoints
- `POST /api/v1/transcoding/transcode` - Start transcoding
- `GET /api/v1/transcoding/sessions/:id` - Get session info
- `GET /api/v1/transcoding/progress/:id` - Get progress
- `GET /api/v1/content/:hash/*` - Access transcoded content

## Creating New Modules

### Step 1: Define Module Structure
```bash
mkdir -p internal/modules/mymodule/{api,core,errors,models,repository,service,types,utils}
```

### Step 2: Create Module Definition
```go
// module.go
package mymodule

import (
    "github.com/mantonx/viewra/internal/modules/modulemanager"
)

func init() {
    Register()
}

func Register() {
    module := &Module{}
    modulemanager.Register(module)
}

type Module struct {
    // Module fields
}

func (m *Module) ID() string { return "system.mymodule" }
func (m *Module) Name() string { return "My Module" }
func (m *Module) GetVersion() string { return "1.0.0" }
func (m *Module) Core() bool { return true }
```

### Step 3: Implement Core Business Logic
```go
// core/myfeature/manager.go
package myfeature

type Manager struct {
    // Manager implementation
}

func NewManager() *Manager {
    return &Manager{}
}
```

### Step 4: Create Service Layer
```go
// service/mymodule_service.go
package service

type myModuleServiceImpl struct {
    manager *myfeature.Manager
}

func NewMyModuleService(manager *myfeature.Manager) services.MyModuleService {
    return &myModuleServiceImpl{
        manager: manager,
    }
}
```

### Step 5: Add Error Handling
```go
// errors/errors.go
package errors

type ErrorType string

const (
    ErrorTypeValidation ErrorType = "validation"
    // Add more error types
)

var (
    ErrNotFound = errors.New("not found")
    // Add more sentinel errors
)
```

### Step 6: Implement API Handlers
```go
// api/handlers.go
package api

type Handler struct {
    service services.MyModuleService
}

func NewHandler(service services.MyModuleService) *Handler {
    return &Handler{service: service}
}
```

## Best Practices

### 1. Module Independence
- Never import from other modules' internal packages
- Use service interfaces for cross-module communication
- Keep module boundaries clear

### 2. Error Handling
- Use module-specific error types
- Provide context with errors
- Use sentinel errors for common cases

### 3. Service Layer
- Keep service layer thin
- Delegate business logic to core
- Handle orchestration only

### 4. API Design
- Group related endpoints in handler files
- Use consistent naming conventions
- Validate input at API layer

### 5. Testing
- Unit test core business logic
- Integration test service layer
- Mock external dependencies

### 6. Documentation
- Keep README.md updated
- Document API endpoints
- Explain complex business logic

### 7. Configuration
- Use types for configuration
- Validate configuration on load
- Provide sensible defaults

### 8. Naming Conventions
- Packages: lowercase, no underscores
- Files: snake_case
- Types: PascalCase
- Functions: camelCase
- Interfaces: end with 'er' when appropriate

## Migration Checklist

When refactoring existing modules:

- [ ] Create standard directory structure
- [ ] Move business logic to core/
- [ ] Create domain-specific managers
- [ ] Implement thin service wrapper
- [ ] Add module-specific errors
- [ ] Organize API handlers by domain
- [ ] Update module registration
- [ ] Add comprehensive README
- [ ] Update service interfaces
- [ ] Add unit tests for core logic