# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Viewra is a modern media management platform with a Go backend and React frontend. It features an extensible plugin system for media processing, metadata enrichment, and transcoding capabilities.

**Tech Stack:**
- Backend: Go 1.24, Gin web framework, GORM (SQLite/PostgreSQL), gRPC
- Frontend: React 19, TypeScript, Vite, TailwindCSS
- Plugin System: HashiCorp go-plugin, CueLang configuration
- Containerization: Docker, Docker Compose

## Common Development Commands

### Backend Development
```bash
cd backend
make test                    # Run all tests
make test-coverage          # Run tests with coverage
make build                  # Build main application
make build-plugins          # Build all plugins
make build-plugin p=PLUGIN  # Build specific plugin
make fmt                    # Format code
make lint                   # Run linter
make clean                  # Clean build artifacts
```

### Frontend Development
```bash
cd frontend
npm run dev                 # Start dev server
npm run build              # Build for production
npm run lint               # Run ESLint
npm run format             # Run Prettier
```

### Docker Development
```bash
# Root level commands
make dev-setup             # Initial development setup
make build-plugins         # Build all plugins (Docker-based)
make restart-backend       # Restart backend container
make logs                  # Show backend logs
make check-env             # Check development environment
docker-compose up -d       # Start all services
docker-compose logs -f backend  # Follow backend logs
```

### Database Management
```bash
make migrate-db            # Move database to proper location
make check-db              # Check database status
make db-web                # Start SQLite web interface (localhost:8081)
```

## Architecture Overview

### Module System
The backend uses a modular architecture with these core modules:
- **modulemanager**: Central module registry and lifecycle management
- **pluginmodule**: Plugin discovery, lifecycle, and communication
- **mediamodule**: Media file processing and library management
- **playbackmodule**: Transcoding and streaming capabilities
- **databasemodule**: Database connections and migrations
- **eventsmodule**: Event bus and messaging system

All modules implement the `Module` interface:
```go
type Module interface {
    ID() string
    Name() string
    Core() bool
    Migrate(db *gorm.DB) error
    Init() error
}
```

### Plugin System Architecture
- **Plugin SDK** (`sdk/`): Standalone Go module for plugin development
- **External Plugins** (`plugins/`): External process plugins using gRPC
- **Core Plugins** (`internal/plugins/`): Built-in plugins (FFmpeg, enrichment, etc.)
- **CUE Configuration**: Type-safe plugin configuration with validation

Plugin types:
- `MetadataScraperService`: Extract metadata from media files
- `TranscodingProvider`: Video/audio transcoding capabilities
- `EnrichmentService`: Metadata enrichment from external APIs

### Database Architecture
- Primary database: SQLite (development) / PostgreSQL (production)
- Location: `viewra-data/viewra.db`
- GORM for ORM with automatic migrations
- Plugin-specific tables managed by individual plugins

### Frontend Architecture
- React with functional components and hooks
- Jotai for state management
- TailwindCSS for styling
- Component structure: `/src/components/` organized by feature
- API layer: `/src/utils/api.ts` with centralized HTTP client

## Plugin Development

### Creating a New Plugin
1. Create directory: `plugins/your_plugin/`
2. Add `plugin.cue` configuration file
3. Implement plugin interfaces in `main.go`
4. Build with: `make build-plugin p=your_plugin`

### Plugin Configuration (CUE)
All plugins use CueLang for type-safe configuration:
```cue
plugin_name: "your_plugin"
plugin_type: "transcoding" | "enrichment" | "scanner"
version: "1.0.0"
enabled: bool | *true

config: {
    // Plugin-specific configuration
}
```

## Testing Guidelines

### Backend Testing
- Unit tests: Place alongside source files (`_test.go`)
- Integration tests: Use `testify` and `go-sqlmock`
- Plugin tests: Test both core and external plugin interfaces
- Coverage target: Maintain reasonable coverage for new code

### Frontend Testing
- Component tests: Use React Testing Library patterns
- API tests: Mock HTTP responses appropriately
- E2E tests: Focus on critical user flows

## Development Workflow

1. **Environment Setup**: Run `make dev-setup` for initial setup
2. **Plugin Development**: Use Docker-based builds for consistency
3. **Database Changes**: Create migrations in appropriate module
4. **API Changes**: Update both backend routes and frontend API layer
5. **Configuration**: Use CUE for plugin configs, environment variables for application config

## Important File Locations

- **Main Application**: `cmd/viewra/main.go`
- **Module Registration**: `internal/bootstrap/`
- **Plugin SDK**: `sdk/`
- **Database Models**: `internal/database/models.go`
- **Frontend API Layer**: `frontend/src/utils/api.ts`
- **Docker Configs**: `docker-compose.yml`, `backend/Dockerfile.dev`

## Common Patterns

### Module Registration
```go
func init() {
    modulemanager.Register(&YourModule{})
}
```

### Plugin Interface Implementation
```go
func (p *YourPlugin) GetName() string { return "your_plugin" }
func (p *YourPlugin) GetVersion() string { return "1.0.0" }
func (p *YourPlugin) GetPluginType() string { return "enrichment" }
```

### Database Migrations
```go
func (m *YourModule) Migrate(db *gorm.DB) error {
    return db.AutoMigrate(&YourModel{})
}
```

### Frontend API Calls
```typescript
import { apiCall } from '../utils/api';
const data = await apiCall<ResponseType>('/api/endpoint');
```

## Performance Considerations

- **Plugin Builds**: Always use Docker for consistency (enforced in Makefile)
- **Database**: SQLite for development, PostgreSQL recommended for production
- **Caching**: Plugins implement their own caching strategies
- **Concurrency**: Use worker pools for media processing tasks
- **Memory**: Monitor plugin memory usage, especially for transcoding

## Troubleshooting

### Plugin Build Issues
- Ensure Docker is running: `make check-env`
- For CGO plugins: Use container builds automatically
- Check plugin logs: `make logs-plugins`

### Database Issues
- Check database location: `make check-db`
- Migrate if needed: `make migrate-db`
- View database: `make db-web` (opens web interface)

### Development Environment
- Check all dependencies: `make check-env`
- Restart services: `docker-compose restart`
- Clean rebuild: `make clean && make dev-setup`