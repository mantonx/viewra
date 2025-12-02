# ViewRA Project Status

**Last Updated**: December 2, 2025

## Current State

ViewRA is a functional self-hosted media server with working streaming, transcoding, and library management.

**What Works**:

- Library scanning with metadata extraction (FFmpeg, NFO, ID3)
- HLS streaming with adaptive bitrate
- On-demand transcoding (4-tier strategy)
- Watch progress tracking
- Movies, TV Shows, and Music support
- Image caching (WebP, multiple sizes)
- Quality selection and Stats for Nerds panel

**What's Missing**:

- User authentication (single-user only)
- External metadata APIs (TMDb, MusicBrainz)
- Hardware-accelerated transcoding

---

## Project Metrics

| Metric | Count |
|--------|-------|
| Go source files | 278 |
| Test functions | 297 |
| Frontend files | 395 |
| Migrations | 22 |
| API endpoints | 65+ |

---

## Recent Work (Nov 22 - Dec 2)

- Adaptive streaming with quality selection
- Stats for Nerds panel
- 4K HDR transcoding fixes
- Multi-codec support
- Ultrawide detection
- Scanner resilience improvements
- Audio player improvements

---

## What's Next

1. **User Authentication** - JWT-based auth, per-user watch progress
2. **External Metadata** - TMDb for movies/TV, MusicBrainz for music
3. **Production Readiness** - Structured logging, health checks, Docker

---

## Architecture

ViewRA uses Clean Architecture with DDD:

- **Domain** (`internal/domain/`) - Business logic, no external deps
- **Application** (`internal/application/`) - Use cases
- **Infrastructure** (`internal/infrastructure/`) - DB, FFmpeg
- **API** (`internal/api/`) - HTTP handlers

Dual database support: SQLite (default) and PostgreSQL.

---

## Documentation

- [PROJECT_PLAN.md](PROJECT_PLAN.md) - Upcoming work
- [TECHNICAL_DEBT.md](TECHNICAL_DEBT.md) - Known issues
- [../core/ARCHITECTURE.md](../core/ARCHITECTURE.md) - System design
- [../decisions/](../decisions/) - Architecture decisions
