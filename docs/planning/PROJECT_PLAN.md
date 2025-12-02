# ViewRA Project Plan

**Last Updated**: December 2, 2025

For current status, see [PROJECT_STATUS.md](PROJECT_STATUS.md).

---

## Recently Completed

### ✅ App Package Restructuring

**Completed**: December 2, 2025
**ADR**: [026-app-restructuring-and-auth.md](../decisions/026-app-restructuring-and-auth.md)

Implemented:

- Created `api.Handlers` aggregate struct - `NewServer` now takes 3 params instead of 43
- Created `app/repositories/`, `app/services/`, `app/usecases/`, `app/handlers/` packages
- Moved scheduled task registration to `container.go`
- Fixed startup to reuse container instead of rebuilding dependencies
- Extracted business logic from handlers to use cases:
  - `ServeMasterPlaylistUseCase` - ABR ladder filtering and master playlist generation
  - `CacheService.GetPresetPath()` - Image cache path construction
- Fixed FFmpeg process management (Wait race condition, log classification)

---

## Upcoming Work

### 1. User Authentication

**Priority**: High
**Effort**: 3-4 days
**ADR**: [028-user-authentication.md](../decisions/028-user-authentication.md)

- JWT access tokens (15 min) + refresh tokens (7 days)
- Argon2id password hashing
- `users` and `sessions` tables
- Auth middleware for protected routes
- Per-user watch progress (`user_id` on `watch_progress`)
- Initial admin creation on first run

### 3. Settings Infrastructure

**Priority**: Medium
**Effort**: 2-3 days
**ADR**: [029-settings-infrastructure.md](../decisions/029-settings-infrastructure.md)

- Database-backed settings with in-memory cache
- System-wide settings (admin) and per-user settings
- Runtime reloadable without restart
- Schema endpoint for future settings UI

### 4. Plugin System

**Priority**: Medium
**Effort**: 26-33 days
**ADR**: [027-plugin-system-architecture.md](../decisions/027-plugin-system-architecture.md)

Extensible plugin system using Hashicorp go-plugin + gRPC, backed by Event Bus and Enrichment Queue:

**Phase 1: Core Infrastructure (5-6 days)**

- Event Bus with ring buffer, slog integration, WebSocket streaming
- Enrichment Queue tables and operations (persistent async jobs)
- Pipeline Manager with user-configurable stages per media type
- Per-stage worker pools (high concurrency for local, rate-limited for remote)

**Phase 2: Plugin Foundation (4-5 days)**

- Plugin manager and process lifecycle (adaptive warm pool)
- PluginCore gRPC definitions (identity, lifecycle, settings, events)
- Enricher interface with single `Enrich()` call and rich capabilities
- SDK Base struct with compile-time enforcement
- Host services (HostData, HostStorage)
- Permission system with category defaults

**Phase 3: First Enrichers (4-5 days)**

- NFO plugin (local file parsing)
- Local Images plugin (poster.jpg, fanart.jpg detection)
- Scanner integration (enqueue on discovery)
- Progress tracking (library-level and item-level)
- ID propagation between stages

**Phase 4: Remote Enrichers (4-5 days)**

- TMDb plugin for movies/TV
- MusicBrainz plugin for music
- Rate limiting and retry logic with exponential backoff
- Plugin SQLite databases with quotas
- Remove hardcoded provider code

**Phase 5: Events & Notifications (3-4 days)**

- Event delivery to plugins via OnEvent RPC
- NotificationSink category
- Webhook plugin as example
- Playback events integration

**Phase 6: Observability (3-4 days)**

- Correlation IDs across app/gRPC/plugin boundaries
- gRPC debug mode (VIEWRA_PLUGIN_DEBUG=1)
- Plugin health monitoring (healthy/degraded/unhealthy)
- Error categorization (network, rate_limit, not_found, parsing)
- Diagnostic export bundles

**Phase 7: UI & Polish (3-4 days)**

- Pipeline configuration UI with apply scopes
- Progress visibility UI (library + item level)
- UI extension points (settings, tabs, menus, widgets)
- Plugin CLI commands (install, update, list)
- Documentation and template repo

### 5. Multi-Language Audio & Subtitles

**Priority**: Medium
**Effort**: 8-12 days
**ADR**: [030-multi-language-audio-subtitles.md](../decisions/030-multi-language-audio-subtitles.md)

Comprehensive audio track and subtitle support:

**Phase 1: Database & Scanning (2-3 days)**

- Add `media_audio_tracks` and `media_subtitle_tracks` tables
- Extend scanner to extract embedded track metadata
- External subtitle file discovery (.srt, .vtt, .ass)

**Phase 2: API & Track Metadata (1-2 days)**

- Track metadata API endpoints
- Library language preference settings
- Extended media response with tracks

**Phase 3: Multi-Audio HLS (2-3 days)**

- Multi-audio master playlists with EXT-X-MEDIA tags
- Separate audio-only playlists per language
- Frontend audio track selector

**Phase 4: Subtitle Support (3-4 days)**

- Subtitle extraction and WebVTT conversion
- Segmented subtitle playlists for HLS
- Frontend subtitle selector with SDH/forced indicators
- Auto-selection based on content language vs. preference

---

## Future Features

### Search & Discovery

- Global search across media types
- Recommendations based on watch history
- Collections and watchlists

### Advanced Playback

- Chapter markers
- Intro skip

### Hardware Acceleration

- NVENC (Nvidia)
- QuickSync (Intel)
- VAAPI (Linux)

### Deployment

- Docker Compose setup
- Kubernetes manifests
- Monitoring with Prometheus/Grafana

---

## Development Guidelines

Before starting work:

1. Check [PROJECT_STATUS.md](PROJECT_STATUS.md) for current state
2. Review [../development/CONVENTIONS.md](../development/CONVENTIONS.md) for code style
3. Ensure dual database compatibility (SQLite + PostgreSQL)

Before marking complete:

1. Code compiles and tests pass
2. Feature works end-to-end
3. Documentation updated
