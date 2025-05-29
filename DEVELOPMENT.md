# Viewra Development Guide

## 🚀 Quick Start

```bash
# Set up development environment
make dev-setup

# Check environment status
make check-env

# Build all plugins with auto-detection
make build-plugins

# Build specific plugin (auto-detects CGO needs)
make build-plugin p=audiodb_enricher

# Force container build for maximum compatibility
make build-plugin p=audiodb_enricher mode=container

# Force host build for maximum speed
make build-plugin p=musicbrainz_enricher mode=host

# Test a plugin build
make test-plugin p=audiodb_enricher

# Restart backend after changes
make restart-backend

# View logs
make logs
```

## 📊 Database Configuration

The database configuration is now centralized and consistent across the entire codebase:

- **Database Location**: `viewra-data/database.db` (Docker volume mounted)
- **Environment Variables**:
  - `VIEWRA_DATA_DIR`: Data directory path (default: `/app/viewra-data`)
  - `VIEWRA_DATABASE_PATH`: Database file path (default: `/app/viewra-data/database.db`)

All database paths are handled by `backend/internal/config/database.go` to ensure consistency.

## 🔧 Bulletproof Plugin Build System

Our build system automatically handles CGO dependencies, architecture compatibility, and container deployment to ensure maximum compatibility.

### 🎯 Build Modes

#### 1. Auto Mode (Default - Recommended)

- **Smart Detection**: Automatically analyzes plugin dependencies
- **CGO Detection**: Scans for SQLite and other CGO dependencies
- **Optimal Strategy**: Chooses container build for CGO plugins, host build for others

```bash
make build-plugin p=audiodb_enricher mode=auto
```

#### 2. Container Mode

- **Maximum Compatibility**: Builds inside the target container
- **Library Consistency**: Ensures all libraries match the runtime environment
- **CGO Support**: Full CGO support with proper library linking

```bash
make build-plugin p=audiodb_enricher mode=container
```

#### 3. Host Mode

- **Maximum Speed**: Builds on the host system
- **Cross-Compilation**: Uses Go's cross-compilation for target architecture
- **Limited CGO**: May have issues with CGO-dependent plugins

```bash
make build-plugin p=musicbrainz_enricher mode=host
```

### 🔍 CGO Detection

The system automatically detects if a plugin requires CGO by analyzing:

**Go Import Patterns:**

- `github.com/mattn/go-sqlite3` - SQLite driver
- `database/sql` - SQL interfaces
- `gorm.io/driver/sqlite` - GORM SQLite driver

**go.mod Dependencies:**

- `github.com/mattn/go-sqlite3`
- `modernc.org/sqlite`
- `gorm.io/driver/sqlite`

### 📊 Build Strategy Decision Matrix

| Plugin Type     | Dependencies | Auto Mode Choice | Recommended  |
| --------------- | ------------ | ---------------- | ------------ |
| Pure Go         | No CGO deps  | Host Build       | ✅ Host      |
| SQLite Plugin   | go-sqlite3   | Container Build  | ✅ Container |
| Database Plugin | database/sql | Container Build  | ✅ Container |
| API Client      | HTTP only    | Host Build       | ✅ Host      |
| File Processing | Standard lib | Host Build       | ✅ Host      |

## 🛠️ Plugin Development

### Creating a New Plugin

1. **Copy the template:**

   ```bash
   cp -r backend/data/plugins/_template backend/data/plugins/your_plugin_name
   ```

2. **Update the template files:**

   - `main.go`: Replace `Template` with your plugin name
   - `plugin.cue`: Update plugin metadata
   - `go.mod`: Update module name

3. **Build and test:**
   ```bash
   make build-plugin p=your_plugin_name
   make test-plugin p=your_plugin_name
   ```

### Plugin Architecture Overview

The Viewra plugin system allows for extending core functionality using HashiCorp's go-plugin architecture:

- **Subprocess Isolation**: Plugins run as separate processes
- **Hot Reload**: Plugins can be updated without restarting Viewra
- **gRPC Communication**: Efficient communication between host and plugins
- **Plugin Discovery**: Automatic discovery of plugins in designated directories
- **Runtime Management**: Load, unload, and manage plugins at runtime

### Supported Plugin Service Types

Plugins can implement one or more service interfaces:

- **Core Plugin Interface** (`Implementation`):
  - All plugins must implement this base interface
  - Methods: `Initialize`, `Start`, `Stop`, `Info`, `Health`
- **Metadata Scraper Service** (`MetadataScraperService`):
  - Extract metadata from files (e.g., music tags, video resolution)
  - Methods: `CanHandle`, `ExtractMetadata`, `GetSupportedTypes`
- **Scanner Hook Service** (`ScannerHookService`):
  - React to events during media library scans
  - Methods: `OnMediaFileScanned`, `OnScanStarted`, `OnScanCompleted`
- **Database Service** (`DatabaseService`):
  - Plugins that require their own database tables
  - Methods: `GetModels`, `Migrate`, `Rollback`
- **Admin Page Service** (`AdminPageService`):
  - Expose configuration/management pages in Viewra admin interface
  - Methods: `GetAdminPages`, `RegisterRoutes`

### Plugin Configuration (CueLang)

Plugins are configured using CueLang (`.cue` files). Each plugin must have a `plugin.cue` file:

```cue
#Plugin: {
    schema_version: "1.0"
    id:             string // Unique identifier
    name:           string // Human-readable name
    version:        string // Semantic version
    description:    string
    author?:        string
    type:           "metadata_scraper" | "scanner_hook" | "database" | "admin_page"

    entry_points: {
        main: string // Plugin executable name
    }

    capabilities?: {
        metadata_scraper?: bool
        scanner_hook?:     bool
        database_service?: bool
        admin_page?:       bool
    }

    settings?: {
        // Plugin-specific settings
        api_key?: string
    }
}
```

### Plugin Standards

All plugins should follow these standards:

1. **Database Connection**: Use centralized database configuration
2. **Error Handling**: Graceful handling of CGO/database issues
3. **Logging**: Minimal debug logging, informative operational logs
4. **Configuration**: Support for API keys and runtime configuration
5. **Build Compatibility**: Use bulletproof build system for proper CGO handling

## 📋 Available Commands

### Plugin Management

```bash
# Build all plugins using auto-detection
make build-plugins

# Build all plugins with container builds (maximum compatibility)
make build-plugins-container

# Build all plugins with host builds (maximum speed)
make build-plugins-host

# Build specific plugin
make build-plugin p=PLUGIN_NAME [mode=auto|host|container] [arch=amd64|arm64]

# Test plugin build
make test-plugin p=PLUGIN_NAME

# Rebuild CGO-dependent plugins with container builds
make rebuild-troublesome

# Clean operations
make clean-binaries    # Remove all plugin binaries
make clean-plugins     # Clean build artifacts and caches
```

### Database Management

```bash
make migrate-db        # Move database to proper location
make check-db          # Verify database configuration
```

### Development

```bash
make dev-setup         # Complete development environment setup
make restart-backend   # Restart backend container
make logs              # Show backend logs
make check-env         # Check environment status
```

## 🏗️ Architecture

```
viewra/
├── backend/
│   ├── data/
│   │   └── plugins/
│   │       ├── _template/          # Plugin template
│   │       ├── audiodb_enricher/   # AudioDB plugin (CGO)
│   │       └── musicbrainz_enricher/ # MusicBrainz plugin (CGO)
│   ├── internal/
│   │   ├── config/
│   │   │   └── database.go         # Centralized DB config
│   │   └── plugins/
│   │       └── manager.go          # Plugin management
│   └── scripts/
│       └── build-plugin.sh         # Bulletproof build script
├── viewra-data/
│   └── database.db                 # SQLite database
├── Makefile                        # Development commands
└── DEVELOPMENT.md                  # This comprehensive guide
```

## ⚙️ Environment Variables

| Variable               | Default                        | Description         |
| ---------------------- | ------------------------------ | ------------------- |
| `VIEWRA_DATA_DIR`      | `/app/viewra-data`             | Data directory path |
| `VIEWRA_DATABASE_PATH` | `/app/viewra-data/database.db` | Database file path  |
| `DATABASE_TYPE`        | `sqlite`                       | Database type       |
| `PLUGIN_DIR`           | `/app/data/plugins`            | Plugin directory    |

## 🐛 Troubleshooting

### Plugin Build Issues

#### "go-sqlite3 requires cgo to work"

**Cause**: CGO plugin built with `CGO_ENABLED=0`
**Solution**: Use auto-detection or force container build:

```bash
make build-plugin p=PLUGIN_NAME mode=container
```

#### "Binary is not a Linux ELF file"

**Cause**: Cross-compilation issues on host build
**Solution**: Use container build mode:

```bash
make build-plugin p=PLUGIN_NAME mode=container
```

#### "Backend container not running"

**Cause**: Container build attempted without running container
**Solution**: Start the backend first:

```bash
docker-compose up -d backend
```

#### General Build Issues

- Run `make clean-binaries` to remove old binaries
- Check architecture with `make check-env`
- Verify Go version compatibility
- Use `make rebuild-troublesome` for CGO plugins

### Database Issues

- Check database location with `make check-db`
- Verify mount points in docker-compose.yml
- Check plugin database URL configuration
- Ensure `viewra-data/` directory exists

### Container Issues

- Restart with `make restart-backend`
- Check logs with `make logs`
- Verify container status with `make check-env`
- Ensure Docker has proper permissions

## 🎯 Development Workflow

### Initial Setup

```bash
# Complete development environment setup
make dev-setup
```

### Regular Development

```bash
# Build your plugin after changes
make build-plugin p=my_plugin

# Test the build
make test-plugin p=my_plugin

# Restart backend to reload
make restart-backend
```

### Pre-Deployment

```bash
# Ensure all plugins work with container builds
make build-plugins-container

# Check everything is working
make check-env
```

## 📈 Benefits

### Automatic Optimization

- **Smart Strategy Selection**: Chooses fastest compatible build method
- **CGO Detection**: Automatically handles complex dependency scenarios
- **Architecture Matching**: Ensures binaries work in target environment

### Developer Experience

- **Simple Commands**: One command builds any plugin correctly
- **Clear Feedback**: Detailed logging shows exactly what's happening
- **Error Recovery**: Automatic fallbacks and clear error messages

### Production Reliability

- **Container Compatibility**: Guarantees plugins work in deployment environment
- **Comprehensive Testing**: Verifies binaries at multiple levels
- **Consistent Results**: Same build process for all plugins
