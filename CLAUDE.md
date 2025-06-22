# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Viewra is a modern media management platform with a Go backend and React frontend. It features an extensible plugin system for media processing, metadata enrichment, and transcoding capabilities.

**Tech Stack:**
- Backend: Go 1.24, Gin web framework, GORM (SQLite/PostgreSQL), gRPC
- Frontend: React 19, TypeScript, Vite, TailwindCSS
- Plugin System: HashiCorp go-plugin, CueLang configuration
- Containerization: Docker, Docker Compose

## Development Environment

**CRITICAL**: ALWAYS use Docker Compose for ALL development work. NEVER build or run the application directly on the host. The containerized environment provides:
- Consistent build environment with all dependencies
- Hot reloading via Air
- Proper volume mounts for live editing
- Database persistence
- Debugging tools

**IMPORTANT**: This project uses Docker Compose for the development environment. All development work should be done within the containerized environment using the development configuration.

### Development Setup Commands
```bash
# Use the development environment with hot reloading
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d

# Check logs
docker-compose logs backend -f

# Restart services if needed
docker-compose restart backend
```

### Key Development Files
- `docker-compose.dev.yml` - Development overrides with Air hot reloading
- `backend/Dockerfile.dev-air` - Development container with debugging tools
- `.air.toml` - Hot reload configuration (ensure paths are `./cmd/viewra/main.go`)

### Development Environment Features
- **Hot Reloading**: Air automatically rebuilds on Go file changes
- **Volume Mounts**: Source code is mounted for live editing
- **Debugging Tools**: strace, tcpdump, htop, etc. are pre-installed
- **Database Persistence**: Uses local `./viewra-data` directory

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

1. **Environment Setup**: Run `docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d` for development with hot reloading
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
- **Docker Configs**: `docker-compose.yml`, `docker-compose.dev.yml`, `backend/Dockerfile.dev-air`

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

### Development Environment Issues
- **Hot Reload Not Working**: Check Air config paths in `.air.toml` (should be `./cmd/viewra/main.go`)
- **Wrong Database**: Ensure development config uses `./viewra-data:/app/viewra-data` not Docker volume
- **Build Failures**: Check container logs with `docker-compose logs backend`

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

## Development Workflow Preferences

**CRITICAL DEVELOPMENT RULES:**

### Plugin Development
- **ALWAYS** use the build script: `make build-plugin p=PLUGIN_NAME`
- **NEVER** manually build plugins with `go build` or Docker commands
- The build script is fixed and handles permissions correctly
- Plugin builds are fast and include hot-reloading

### Backend Development
- **AVOID** restarting the backend container when possible
- **USE** Air hot-reloading for Go code changes (it's already configured)
- Air automatically rebuilds and restarts the backend on file changes
- Only restart the backend container if there are container-level issues

### Frontend Development  
- **AVOID** restarting the frontend container when possible
- **USE** Vite hot-reload for React/TypeScript changes (it's already configured)
- Vite automatically updates the browser on file changes
- Only restart if there are dependency or configuration issues
- **STORYBOOK**: No need to restart Storybook - it has fast reload and will automatically update when files change

## Tool Preferences

### File System Operations
- **PREFER**: Use MCP filesystem tools (mcp__filesystem__*) for all file operations
- These tools provide better performance and integration compared to traditional file reading tools

### Build and Development
- **ALWAYS**: Use the build script `make build-plugin p=PLUGIN_NAME` for plugin builds
- **NEVER**: Run `go build` directly for plugins or manually build with Docker
- **AVOID**: Restarting backend/frontend containers unnecessarily
- **RELY ON HOT-RELOAD**: 
  - Backend: Air automatically rebuilds Go code changes
  - Frontend: Vite automatically updates React/TypeScript changes
  - Both systems watch files and update automatically on save
- **PREFER**: MCP filesystem tools over traditional file operations
- Available MCP filesystem operations:
  - `mcp__filesystem__read_file` - Read single files
  - `mcp__filesystem__read_multiple_files` - Read multiple files efficiently
  - `mcp__filesystem__write_file` - Write files
  - `mcp__filesystem__edit_file` - Edit files with line-based replacements
  - `mcp__filesystem__create_directory` - Create directories
  - `mcp__filesystem__list_directory` - List directory contents
  - `mcp__filesystem__directory_tree` - Get recursive directory structure
  - `mcp__filesystem__move_file` - Move/rename files
  - `mcp__filesystem__search_files` - Search for files by pattern
  - `mcp__filesystem__get_file_info` - Get file metadata