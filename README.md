# Viewra - Your Modern Media Management Platform

Viewra is a flexible and extensible media management platform designed to help you organize, browse, and stream your media library efficiently. It features a robust backend built in Go with a modern two-stage transcoding pipeline and a responsive frontend using React.

## Key Features

- **Two-Stage Transcoding Pipeline**: Modern architecture using FFmpeg for encoding and Shaka Packager for DASH/HLS manifest generation, ensuring proper VOD support
- **Content-Addressable Storage**: Automatic deduplication of transcoded content with CDN-friendly URLs
- **Adaptive Bitrate Streaming**: Support for both DASH and HLS with multiple quality levels
- **Hardware Acceleration**: NVIDIA NVENC support through plugin architecture
- **Extensible Plugin System**: Customize and extend Viewra's functionality with powerful plugins (See [Plugin Development Guide](docs/PLUGIN_DEVELOPMENT_GUIDE.md))
- **Efficient Media Scanning**: Fast and reliable scanning of your media libraries
- **Metadata Enrichment**: Leverage plugins (like MusicBrainz) to enrich your media files with detailed metadata
- **Modern Web Interface**: A clean and responsive interface built with React and TypeScript
- **Dockerized Deployment**: Easy to deploy and manage using Docker
- **CueLang Configuration**: Type-safe and powerful configuration for plugins using CueLang

## Tech Stack

- **Backend**: Go 1.24, Gin web framework, GORM (SQLite/PostgreSQL)
- **Frontend**: React 19, TypeScript, Vite, TailwindCSS, Vidstack Media Player
- **Transcoding**: FFmpeg (encoding), Shaka Packager (DASH/HLS packaging)
- **Plugin System**: HashiCorp go-plugin, gRPC, CueLang
- **Database**: SQLite (development), PostgreSQL (production)
- **Storage**: Content-addressable filesystem with SHA256 hashing
- **Containerization**: Docker, Docker Compose

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Go 1.24+ - for backend development or running outside Docker
- Node.js 20+ and npm - for frontend development
- `jq` and `curl` (for some utility scripts)
- FFmpeg 6.0+ (included in Docker images)
- Shaka Packager 3.0+ (included in Docker images)

### Installation & Setup

1.  **Clone the repository:**

    ```bash
    git clone https://github.com/mantonx/viewra.git
    cd viewra
    ```

2.  **Development Environment (Docker Compose):**
    The easiest way to get started for development is using Docker Compose. This will set up the backend, frontend, and any necessary services.

    ```bash
    # Start the development environment with hot reloading
    docker-compose -f docker-compose.yml -f docker-compose.dev.yml up -d

    # Or use the convenient make commands
    make dev-setup     # Initial setup
    make logs          # Follow backend logs
    ```

    This starts:

    - Viewra Backend (with hot-reloading via Air)
    - Viewra Frontend (Vite dev server with hot-reloading)
    - SQLite database (persisted in `./viewra-data`)
    - Plugin system with FFmpeg and enrichment plugins

3.  **Building Plugins:**
    Plugins must be built using the provided Docker-based build system:

    ```bash
    # Build all plugins
    make build-plugins
    
    # Build a specific plugin
    make build-plugin p=ffmpeg_software
    ```

    **Note**: Never build plugins manually with `go build` - always use the make commands.

4.  **Frontend Development:**
    The frontend uses Vite with hot module replacement:
    ```bash
    cd frontend
    npm install
    npm run dev      # Development server
    npm run build    # Production build
    npm run lint     # Run ESLint
    npm run format   # Run Prettier
    ```

### Accessing Viewra

- **Frontend**: http://localhost:5173 (Vite dev server)
- **Backend API**: http://localhost:8080/api
- **Content Storage**: http://localhost:8080/api/v1/content/{hash}/
- **Database Web UI**: http://localhost:8081 (when running `make db-web`)

## Usage

Once Viewra is running, navigate to the frontend URL in your web browser. You can start by:

1. Configuring your media libraries through the admin interface
2. Initiating a scan of your libraries
3. Browsing your media collection
4. Playing videos with adaptive bitrate streaming
5. Managing transcoding settings and quality levels

### Transcoding Architecture

Viewra uses a modern two-stage transcoding pipeline:

1. **Encoding Stage**: FFmpeg encodes source videos to multiple quality levels as MP4 files
2. **Packaging Stage**: Shaka Packager creates DASH/HLS manifests from the encoded MP4s

This architecture ensures:
- Proper VOD (Video on Demand) manifest generation with `type="static"`
- Content deduplication through SHA256 hashing
- CDN-friendly URLs: `/api/v1/content/{hash}/manifest.mpd`
- Efficient storage through content-addressable architecture

For more details, see the [Architecture Documentation](docs/ARCHITECTURE.md).

## Plugin System

Viewra features a powerful plugin system that allows for significant customization and extension of its core capabilities. For detailed information on how the plugin system works and how to develop your own plugins, please refer to the [Plugin Documentation](docs/PLUGINS.md).

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for more details on how to get involved, coding standards, and the development process.

## Testing

Viewra includes comprehensive test coverage:

### Backend Testing
```bash
cd backend
make test          # Run all backend tests
make test-coverage # Generate coverage reports
make lint          # Run golangci-lint
```

### Frontend Testing
```bash
cd frontend
npm run test       # Run Jest unit tests
npm run test:e2e   # Run Cypress E2E tests
npm run storybook  # Component development with Storybook
```

### Integration Testing
The project includes integration tests for the full transcoding pipeline:
```bash
cd backend
go test ./internal/modules/playbackmodule -tags=integration
```

## Performance & Optimization

### Content-Addressable Storage
- Automatic deduplication of transcoded content
- SHA256 hashing based on: media ID + encoding profiles + output formats
- Directory sharding for filesystem scalability
- CDN-ready URL structure

### Hardware Acceleration
- NVIDIA NVENC support through `ffmpeg_nvidia` plugin
- Automatic fallback to software encoding
- Configurable quality/speed trade-offs

### Caching & Pre-warming
- Metadata caching in database
- Transcoded content persists across sessions
- Popular content pre-warming (planned)

## Documentation

### Core Documentation
- [Architecture Overview](docs/ARCHITECTURE.md) - System design and component interaction
- [Module Architecture](docs/MODULE_ARCHITECTURE.md) - Clean architecture patterns and module structure
- [API Reference](docs/API.md) - Complete API documentation
- [Database Schema](docs/DATABASE.md) - Database structure and models

### Development Guides
- [Quick Start](docs/QUICKSTART.md) - Get up and running quickly
- [Plugin Development](docs/PLUGIN_DEVELOPMENT_GUIDE.md) - Creating custom plugins
- [Transcoding Guide](docs/TRANSCODING_GUIDE.md) - Using the transcoding system
- [Debugging Guide](docs/DEBUGGING_GUIDE.md) - Troubleshooting common issues

### Deployment & Configuration
- [Deployment Guide](docs/DEPLOYMENT.md) - Production deployment procedures
- [MCP Setup](docs/MCP_SETUP.md) - Configure AI assistant tools

### Architecture Decisions
- [ADR Index](docs/adr/README.md) - Architecture Decision Records
- [Module Separation](docs/adr/0001-module-separation.md)
- [Service Registry](docs/adr/0002-service-registry-pattern.md)
- [Clean Architecture](docs/adr/0004-clean-module-architecture.md)

### Additional Resources
- [Frontend Components](docs/COMPONENTS.md) - React component documentation
- [Development with Claude](CLAUDE.md) - AI-assisted development guidelines

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details on:
- Code style and standards
- Development workflow
- Testing requirements
- Pull request process

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
