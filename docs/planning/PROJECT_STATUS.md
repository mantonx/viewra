# ViewRA Project Status

**Last Updated**: January 17, 2026

## Current State

ViewRA is a fully functional self-hosted media server with streaming, transcoding, library management, and an extensible plugin system.

**Core Features**:

- Library scanning with metadata extraction (FFmpeg, NFO, ID3)
- HLS streaming with adaptive bitrate
- On-demand transcoding (4-tier strategy)
- Watch progress tracking (per-user)
- Movies, TV Shows, and Music support
- Image caching (WebP, multiple sizes)
- Quality selection and Stats for Nerds panel
- User authentication (JWT-based, sessions)
- Settings infrastructure with environment variable awareness
- Design system with "Cinema at Home" aesthetic
- User ratings (up/down/favorite)
- Home screen with customizable widgets
- Global search with autocomplete

**Plugin System** (8 plugins):

- **TMDb** - Movie/TV metadata and artwork from The Movie Database
- **MusicBrainz** - Music metadata and cover art
- **Semantic Search** - AI-powered natural language search
- **Recommendations** - Personalized "For You" and "Because You Watched" rows
- **AI Features** - Master AI configuration + Ollama local provider
- **AI Provider Anthropic** - Claude integration
- **AI Provider OpenAI** - GPT integration
- **AI Provider Voyage** - Voyage embeddings

**What's Missing**:

- Hardware-accelerated transcoding (NVENC, QuickSync, VAAPI)
- Multi-language audio/subtitle track selection UI
- Chapter markers and intro skip

---

## Project Metrics

| Metric | Count |
|--------|-------|
| Go source files | 869 |
| Test functions | 1,689 |
| Frontend files | 876 |
| Migrations | 16 (8 SQLite + 8 PostgreSQL) |
| API endpoints | 97 |
| Plugins | 8 |
| Domain modules | 16 |
| Application modules | 22 |

---

## Recent Work

- **Semantic Search** - Intent chips, autocomplete, targeted scan support
- **Home Screen Widgets** - Continue Watching, Recently Added, Trending, For You
- **Settings Infrastructure v2** (Dec 3) - Environment variable awareness, system profiles
- **Design System Improvements** (Dec 2) - Cinema at Home aesthetic
- **App Package Restructuring** (Dec 2) - Simplified `NewServer` from 43 params to 3 params
- **User Authentication** (Dec 2) - JWT-based auth with per-user watch progress
- **Plugin System** - Full gRPC plugin infrastructure with SDK
- **TMDb/MusicBrainz Integration** - External metadata via plugins

---

## What's Next

1. **Multi-Language Audio & Subtitles** - Track selection UI (ADR-030)
2. **Hardware Acceleration** - NVENC, QuickSync, VAAPI support
3. **Advanced Playback** - Chapter markers, intro skip

See [PROJECT_PLAN.md](PROJECT_PLAN.md) for detailed roadmap.

---

## Architecture

ViewRA uses Clean Architecture with DDD:

- **Domain** (`internal/domain/`) - 16 modules: library, media, user, settings, search, home, ratings, etc.
- **Application** (`internal/application/`) - 22 modules with services and use cases
- **Infrastructure** (`internal/infrastructure/`) - DB, FFmpeg, filesystem, streaming
- **API** (`internal/api/`) - HTTP handlers with middleware (auth, CORS, rate limiting)
- **App** (`internal/app/`) - Dependency injection container and lifecycle management

Dual database support: SQLite (default) and PostgreSQL.

Plugin architecture: gRPC-based with SDK (`pkg/plugin/sdk/`), protocol definitions (`api/proto/plugin/`).

---

## Documentation

- [PROJECT_PLAN.md](PROJECT_PLAN.md) - Upcoming work
- [TECHNICAL_DEBT.md](TECHNICAL_DEBT.md) - Known issues
- [../core/ARCHITECTURE.md](../core/ARCHITECTURE.md) - System design
- [../decisions/](../decisions/) - Architecture decisions (30 ADRs)
