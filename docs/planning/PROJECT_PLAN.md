# ViewRA Project Plan

**Last Updated**: December 2, 2025

For current status, see [PROJECT_STATUS.md](PROJECT_STATUS.md).

---

## Upcoming Work

### 1. App Package Restructuring

**Priority**: High (prerequisite for auth)
**Effort**: 2-3 days
**ADR**: [026-app-restructuring-and-auth.md](../decisions/026-app-restructuring-and-auth.md)

Current problems:

- `NewServer` has 30+ parameters
- Inconsistent handler creation
- `startup.go` rebuilds repos/services just to recover stuck scans

Changes:

- Create `api.Handlers` aggregate struct
- Simplify `NewServer(config, logger, handlers)`
- New `app/wire/` package for dependency wiring
- New `app/tasks/` package for scheduled task registration
- Fix startup to reuse container

### 2. User Authentication

**Priority**: High
**Effort**: 3-4 days
**ADR**: [026-app-restructuring-and-auth.md](../decisions/026-app-restructuring-and-auth.md)

- JWT access tokens (15 min) + refresh tokens (7 days)
- Argon2id password hashing
- `users` and `sessions` tables
- Auth middleware for protected routes
- Per-user watch progress (`user_id` on `watch_progress`)
- Initial admin creation on first run

### 3. Settings Infrastructure

**Priority**: Medium
**Effort**: 2-3 days
**ADR**: [026-app-restructuring-and-auth.md](../decisions/026-app-restructuring-and-auth.md)

- Database-backed settings with in-memory cache
- System-wide settings (admin) and per-user settings
- Runtime reloadable without restart
- Schema endpoint for future settings UI

### 4. Plugin System

**Priority**: Medium
**Effort**: 19-25 days
**ADR**: [027-plugin-system-architecture.md](../decisions/027-plugin-system-architecture.md)

Extensible plugin system using Hashicorp go-plugin + gRPC:

**Phase 1: Foundation (4-5 days)**

- Plugin manager, process lifecycle, adaptive warm pool
- PluginCore gRPC definitions (identity, lifecycle, settings, events)
- MetadataProvider interface with TV/Music extensions
- Host services (HostData, HostStorage)
- NFO plugin as first implementation
- Permission system with category defaults

**Phase 2: Metadata Integration (3-4 days)**

- Hook into library scanner
- Search flow with confidence scores
- Metadata merging (field-level priority)
- External IDs table, source tracking
- Match locking

**Phase 3: Built-in Providers (4-5 days)**

- TMDb plugin for movies/TV (series-first matching)
- MusicBrainz plugin for music (album-first matching)
- Plugin SQLite databases with quotas
- Remove hardcoded provider code

**Phase 4: Events & Notifications (3-4 days)**

- Event dispatcher (library, playback, user, system events)
- NotificationSink category
- Webhook plugin as example
- Event queue for reliability

**Phase 5: UI Extensions (3-4 days)**

- Extension points (settings, tabs, menus, widgets, navigation, player)
- JSON schema UI rendering
- React component loading
- Settings page generation

**Phase 6: Polish (2-3 days)**

- Plugin CLI commands (install, update, list)
- Registry integration, update notifications
- Developer test harness
- Documentation and template repo

---

## Future Features

### Search & Discovery

- Global search across media types
- Recommendations based on watch history
- Collections and watchlists

### Advanced Playback

- Subtitle support (SRT, ASS, VTT)
- Multiple audio track selection
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
