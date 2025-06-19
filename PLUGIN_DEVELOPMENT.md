# Plugin Development Guide

## Overview

Viewra uses a unified plugin build system that automatically detects the best build method and ensures plugins are compatible with the Docker environment. The FFmpeg transcoder plugin is enabled by default for seamless video playback.

## Quick Start

### Complete Setup (Recommended)
```bash
# Complete plugin setup with auto-detection
./scripts/setup-plugins.sh

# Or using Makefile
make setup-plugins
```

### Building Plugins Only
```bash
# Build all plugins (auto-detects Docker/host)
./scripts/build-plugins.sh

# Force Docker build (recommended)
./scripts/build-plugins.sh docker

# Build specific plugin
./scripts/build-plugins.sh auto ffmpeg_transcoder

# Using Makefile
make plugins                    # Build all plugins
make build-plugins-docker       # Force Docker build
make build-plugin-ffmpeg_transcoder  # Build specific plugin
```

## Build Modes

The unified build system supports three modes:

1. **Auto (default)**: Automatically detects if Docker is available and chooses the best method
2. **Docker**: Forces building inside the backend Docker container (recommended for production)
3. **Host**: Builds on the host system (faster for development but may have compatibility issues)

## FFmpeg Transcoder Auto-Enablement

The FFmpeg transcoder plugin is configured to be enabled by default:

- **Plugin Configuration**: `enabled_by_default: true` in `plugin.cue`
- **Database Auto-Enable**: Special handling in external plugin manager
- **Build Script Integration**: Automatically enables in database after successful build

This means you no longer need to manually enable the plugin after each rebuild.

## Development Workflow

### Daily Development
```bash
# Quick development build and restart
make dev-plugins

# Build specific plugin during development
make build-plugin-ffmpeg_transcoder

# Check plugin logs
make logs-plugins
```

### Hot Reload
The system supports hot reload for development:
- Plugins automatically reload when binaries change
- No need to restart the backend for plugin updates
- File watcher monitors plugin directory for changes

### Advanced Usage

#### Custom Build Options
```bash
# Setup with specific options
./scripts/setup-plugins.sh --build-mode docker --plugin ffmpeg_transcoder --no-restart

# Force Docker build for specific plugin
./scripts/build-plugins.sh docker ffmpeg_transcoder
```

#### Troubleshooting Builds
```bash
# Clean all plugin binaries
make clean-plugins

# Complete rebuild
make clean-plugins build-plugins-docker

# Check plugin status
./scripts/setup-plugins.sh --no-restart
```

## Architecture

### Unified Build Script (`scripts/build-plugins.sh`)
- **Auto-detection**: Chooses Docker or host build automatically
- **CGO Detection**: Automatically enables CGO for plugins that need it
- **Validation**: Ensures binaries are built correctly and executable
- **Cross-platform**: Builds Linux binaries regardless of host OS

### Setup Script (`scripts/setup-plugins.sh`)
- **Environment Verification**: Checks Docker and prerequisites
- **Service Management**: Ensures Docker services are running
- **Plugin Enablement**: Handles FFmpeg transcoder auto-enablement
- **Hot Reload Configuration**: Verifies development workflow setup

### Makefile Integration
- **Convenient Targets**: Simple commands like `make plugins`
- **Development Workflow**: `make dev-plugins` for quick iterations
- **Specific Plugin Builds**: `make build-plugin-<name>` for individual plugins
- **Logging Support**: `make logs-plugins` for debugging

## File Structure

```
scripts/
├── build-plugins.sh       # Unified build script (replaces all others)
└── setup-plugins.sh       # Complete setup and verification

backend/data/plugins/
├── ffmpeg_transcoder/
│   ├── main.go
│   ├── plugin.cue         # enabled_by_default: true
│   └── go.mod
└── other_plugins/

Makefile                   # Plugin build targets
```

## Migration from Old System

The new system replaces several old scripts:
- ❌ `scripts/build-plugins-docker.sh` (removed)
- ❌ `backend/scripts/build-plugin.sh` (removed)
- ✅ `scripts/build-plugins.sh` (unified replacement)
- ✅ `scripts/setup-plugins.sh` (enhanced)

### Migration Commands
```bash
# Old way
./scripts/build-plugins-docker.sh

# New way
./scripts/build-plugins.sh docker
# or simply
make plugins
```

## Best Practices

1. **Use Docker Builds**: For production compatibility, prefer Docker builds
2. **Auto-Detection**: Let the system choose the best build method with `auto` mode
3. **Complete Setup**: Use `./scripts/setup-plugins.sh` for initial setup
4. **Development Workflow**: Use `make dev-plugins` for quick iterations
5. **Monitor Logs**: Use `make logs-plugins` to verify plugin loading

## Troubleshooting

### Plugin Not Loading
```bash
# Check plugin status
docker-compose logs backend | grep -i plugin

# Verify plugin in database
make setup-plugins
```

### Build Failures
```bash
# Clean and rebuild
make clean-plugins plugins

# Force Docker build
make build-plugins-docker
```

### FFmpeg Transcoder Issues
```bash
# Verify enablement
./scripts/setup-plugins.sh --plugin ffmpeg_transcoder

# Check specific logs
docker-compose logs backend | grep -i ffmpeg
```

## Plugin Types

- **Transcoder Plugins**: `*_transcoder` (e.g., ffmpeg_transcoder)
- **Enrichment Plugins**: `*_enricher` (e.g., tmdb_enricher_v2)
- **Scanner Plugins**: `*_scanner` (future)

All plugin types follow the same build process and auto-enablement rules. 