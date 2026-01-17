# ViewRA

A self-hosted media server for organizing and streaming your personal media collections.

ViewRA brings a polished "Cinema at Home" experience to your movies, TV shows, and music. Stream anywhere with adaptive bitrate, get rich metadata from TMDb and MusicBrainz, and discover content with AI-powered search.

## Features

**Library Management**
- Automatic scanning with metadata extraction (FFmpeg, NFO, ID3)
- Movies, TV Shows, and Music support
- Image caching (WebP, multiple sizes)
- Watch progress tracking (per-user)

**Streaming**
- HLS streaming with adaptive bitrate
- On-demand transcoding via FFmpeg
- Quality selection controls
- Subtitle extraction and delivery

**Discovery**
- Global search with autocomplete
- AI-powered semantic search ("movies like Blade Runner")
- Personalized recommendations ("For You", "Because You Watched")
- Home screen with customizable widgets

**Metadata Enrichment**
- TMDb integration (movies, TV shows, artwork)
- MusicBrainz integration (music, cover art)
- Extensible plugin system for additional sources

**User Experience**
- User authentication with JWT
- Per-user watch progress and ratings (up/down/favorite)
- "Cinema at Home" design aesthetic
- Stats for Nerds panel

## Quick Start

### Prerequisites

- Go 1.21+
- Node.js 18+
- Rust/Cargo (for subtitle-extractor)
- FFmpeg

### Setup

```bash
# Clone the repository
git clone https://github.com/viewra/viewra.git
cd viewra

# Initial setup (installs Go tools, checks Rust)
make setup

# Build the subtitle extraction tool
make build-tools

# Start development server (backend :8080, frontend :5173)
make dev
```

Open http://localhost:5173 in your browser. Default credentials: `dev` / `dev`

### Production Build

```bash
# Build everything: tools, frontend, backend
make build

# Build plugins
make build-plugins
```

## Architecture

ViewRA uses Clean Architecture with Domain-Driven Design:

```
internal/
├── domain/          # Business logic (no external deps)
├── application/     # Use cases and services
├── infrastructure/  # DB, FFmpeg, filesystem
├── api/             # HTTP handlers
└── app/             # Dependency wiring
plugins/             # gRPC enrichment plugins
web/                 # React frontend (TanStack Router)
```

- **Database**: SQLite (default) or PostgreSQL
- **Streaming**: HLS with on-demand transcoding
- **Plugins**: gRPC-based plugin system with SDK

## Plugins

ViewRA ships with 8 plugins:

| Plugin | Purpose |
|--------|---------|
| TMDb | Movie/TV metadata and artwork |
| MusicBrainz | Music metadata and cover art |
| Semantic Search | AI-powered natural language search |
| Recommendations | Personalized content suggestions |
| AI Features | AI configuration + Ollama provider |
| AI Provider Anthropic | Claude integration |
| AI Provider OpenAI | GPT integration |
| AI Provider Voyage | Voyage embeddings |

Build your own plugins with the SDK in `pkg/plugin/sdk/`.

## Documentation

- [Architecture](docs/core/ARCHITECTURE.md) - System design and layers
- [Development Conventions](docs/development/CONVENTIONS.md) - Code style and patterns
- [HLS Transcoding Guide](docs/guides/HLS_TRANSCODING.md) - How streaming works
- [Plugin Development](docs/guides/PLUGIN_DEVELOPMENT.md) - Building plugins
- [Operations Guide](docs/operations/README.md) - Deployment and configuration

See [docs/README.md](docs/README.md) for the full documentation index.

## License

MIT
