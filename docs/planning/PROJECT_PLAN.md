# ViewRA Project Plan

**Last Updated**: December 2, 2025

For current status, see [PROJECT_STATUS.md](PROJECT_STATUS.md).

---

## Upcoming Work

### 1. User Authentication

**Priority**: High
**Effort**: 20-25 hours

- Users table and JWT tokens
- Login/logout/register endpoints
- Auth middleware for protected routes
- Per-user watch progress
- Frontend login UI

### 2. External Metadata APIs

**Priority**: Medium
**Effort**: 25-30 hours

- TMDb integration for movies/TV
- MusicBrainz for music metadata
- Automatic poster/backdrop downloads
- Manual match override UI

### 3. Production Readiness

**Priority**: Medium
**Effort**: 12-15 hours

- Structured logging (slog)
- Health check endpoints
- Graceful shutdown
- Docker images

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
