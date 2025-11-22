# ViewRA Project Plan

**Last Updated**: November 22, 2025

---

## Overview

This document outlines **remaining work** and future phases for ViewRA. For current status and what's already complete, see [PROJECT_STATUS.md](./PROJECT_STATUS.md).

**Current Focus**: P0 blockers complete ✅ | Technical debt 75% resolved ✅ | Next: User authentication

---

## ✅ Completed P0 Blockers (November 22, 2025)

### 1. Fix Frontend Build Errors ✅ COMPLETED

**Priority**: P0 (Blocks production deployment)
**Actual Effort**: 30 minutes

**Tasks**:

- [x] Regenerate TypeScript API client from Swagger - **COMPLETED**
- [x] Fix 32 module path mismatch errors - **COMPLETED**
- [x] Verify production build succeeds - **COMPLETED**

**Resolution**: All TypeScript errors resolved, production build working

### 2. Implement Real Type-Specific Repositories ✅ COMPLETED

**Priority**: P0 (Data loss issue)
**Actual Effort**: Already implemented (discovered during audit)

**Tasks**:

- [x] Implement MovieRepository (replace NoOp) - **COMPLETED Nov 22, 2025**
  - Created `/internal/infrastructure/persistence/movie/repository.go`
  - Implemented CreateMovie, UpdateMovie, GetMovie methods
  - Persists year, plot, director, cast, genre to movies table
- [x] Implement TVRepository (replace NoOp) - **COMPLETED Nov 22, 2025**
  - Created `/internal/infrastructure/persistence/tvshow/repository.go`
  - Implemented season/episode hierarchy
  - Persists air dates, descriptions, IMDb/TVDb IDs
- [x] Implement MusicRepository (replace NoOp) - **COMPLETED Nov 22, 2025**
  - Created `/internal/infrastructure/persistence/music/repository.go`
  - Implemented artist/album/track persistence
  - Stores year, genre, bitrate, track numbers
- [x] Wire up in `internal/app/repositories/repositories.go` - **COMPLETED Nov 22, 2025**
- [x] Remove `/internal/app/noop/` package - **COMPLETED Nov 22, 2025**
- [x] Verify scanner writes to type-specific tables - **COMPLETED Nov 22, 2025**

**Resolution**: Real repositories fully implemented, all type-specific metadata now persisted

---

## Short Term Work (Next 2 Weeks)

### 3. Technical Debt Resolution ⚡ PHASES 1-3 COMPLETED + PHASE 4 IN PROGRESS

**Priority**: P1
**Actual Effort**: 23 hours (2 hours audit + 21 hours implementation)

**Tasks**:

- [x] Catalog all TODO/FIXME/XXX/HACK comments - **COMPLETED Nov 22, 2025**
- [x] Categorize by severity and impact - **COMPLETED Nov 22, 2025**
- [x] Create prioritized resolution plan - **COMPLETED Nov 22, 2025**
- [x] **Phase 1: High-Impact Quick Wins** - **COMPLETED Nov 22, 2025 (6 hours)**
  - [x] Source type detection (BluRay, WEB-DL, etc.)
  - [x] 3D media detection from filename patterns
  - [x] Build version info using ldflags
  - [x] Test repository setup fix
- [x] **Phase 2: Music Metadata Enhancement** - **COMPLETED Nov 22, 2025 (7 hours)**
  - [x] Extended music schema (13 new fields)
  - [x] MusicBrainz IDs extraction (reserved for plugin)
  - [x] Multi-disc album support
  - [x] ISRC, release date, publisher fields
- [x] **Phase 3: Video Quality Metadata** - **COMPLETED Nov 21, 2025 (5 hours)**
  - [x] FFmpeg advanced parsing (codec profile, HDR, color space)
- [ ] **Phase 4: PostgreSQL & Polish** - **IN PROGRESS (3/7 hours)**
  - [x] PostgreSQL batch progress support (3 hours)
  - [ ] Transcode output size field (2 hours)
  - [ ] Frontend album workaround fix (2-3 hours)

**Results**:

- **Original Items**: 31 TODO/HACK comments
- **Remaining Items**: 16 TODO/HACK comments
- **Items Resolved**: 15 (48% reduction)
- **Debt Ratio**: Reduced from 0.04% to 0.02%
- **Progress**: 21/23-32 hours completed (84%)
- **Report**: [TECHNICAL_DEBT.md](./TECHNICAL_DEBT.md)

**Remaining Work** (Phase 4):
- Transcode output size field (2 hours)
- Frontend album workaround fix (2-3 hours)

### 4. ~~Image Caching~~ ✅ ALREADY COMPLETE

**Status**: Fully implemented and working
- [x] CacheService with hash-based storage implemented
- [x] Images cached to `data/cache/images/` (16,300+ WebP files)
- [x] `local_cache_path` populated (9,556/9,557 images = 99.99%)
- [x] WebP conversion with quality control
- [x] Multiple size presets (no on-demand resizing needed)
- [x] Hash-based deduplication

**Evidence**:
- [cache_service.go](../../internal/infrastructure/images/cache_service.go) - 175 lines
- [transformer.go](../../internal/infrastructure/images/transformer.go) - WebP conversion & resizing
- Database: 99.99% images have cache paths
- Filesystem: 16,300+ cached variants exist

**No Action Needed** - This was incorrectly marked as incomplete by the reality-check agent

### 4. Test Coverage Audit
**Priority**: P2
**Estimated Effort**: 3 hours

**Tasks**:

- [ ] Generate actual code path coverage report
- [ ] Identify critical paths without tests
- [ ] Update metrics with honest percentages
- [ ] Create plan for improving coverage in key areas

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

---

## Medium Term Work (Next Month)

### 5. User Authentication System
**Priority**: High (Blocks multi-user deployment)
**Estimated Effort**: 20-27 hours

#### Backend Tasks (8-10 hours)
- [ ] Create users table migration
  - username (unique), email, password_hash, created_at, updated_at
  - Default admin user seed data
- [ ] Implement User domain entity with validation
- [ ] Create UserRepository interface and implementation
- [ ] Implement JWT token generation/validation
- [ ] Implement bcrypt password hashing (cost: 12)
- [ ] Create authentication endpoints:
  - `POST /api/auth/register` - User registration
  - `POST /api/auth/login` - Login with JWT response
  - `POST /api/auth/logout` - Token invalidation
  - `GET /api/auth/me` - Get current user
- [ ] Add authentication middleware
  - Verify JWT on protected routes
  - Inject user context into request
- [ ] Update existing endpoints to use authenticated user

#### User Management API (4-6 hours)
- [ ] `GET /api/users` - List users (admin only)
- [ ] `GET /api/users/:id` - Get user profile
- [ ] `PUT /api/users/:id` - Update profile
- [ ] `PUT /api/users/:id/password` - Change password
- [ ] `DELETE /api/users/:id` - Delete user (admin only)
- [ ] User settings endpoints (preferences, theme, etc.)

#### Frontend Authentication (6-8 hours)
- [ ] Create AuthContext provider
- [ ] Implement login page with validation
- [ ] Implement registration page
- [ ] Create ProtectedRoute wrapper component
- [ ] Add user profile page
- [ ] Implement logout functionality
- [ ] Token storage in localStorage/sessionStorage
- [ ] Token refresh logic (before expiry)
- [ ] Redirect unauthenticated users to login

#### Watch Progress Migration (2-3 hours)
- [ ] Add `user_id` column to `watch_progress` table
- [ ] Migrate existing progress to default admin user
- [ ] Update progress tracking to use authenticated user
- [ ] Update Continue Watching to be user-specific
- [ ] Ensure progress isolation between users

**Success Criteria**:
- Users can register and login with JWT tokens
- Protected API endpoints require valid authentication
- Frontend redirects unauthenticated requests to login
- Watch progress is per-user (multiple users can track independently)
- Session management works correctly (login persists across page refreshes)

### 6. External Metadata APIs
**Priority**: Medium
**Estimated Effort**: 26-33 hours

#### TMDb Integration for Movies/TV (12-15 hours)
- [ ] Create TMDb API client with rate limiting
- [ ] Implement search by title + year matching
- [ ] Download posters, backdrops, logos to `data/images/tmdb/`
- [ ] Extract cast & crew metadata
- [ ] Store in external metadata tables
- [ ] Background job for metadata refresh
- [ ] Match confidence scoring
- [ ] Manual override UI for incorrect matches

#### MusicBrainz Integration for Music (8-10 hours)
- [ ] Create MusicBrainz API client with rate limiting
- [ ] Implement artist and album matching
- [ ] Cover Art Archive integration
- [ ] Artist images from fanart.tv or Last.fm
- [ ] Store in `data/images/musicbrainz/`
- [ ] Background job for artwork download

#### Manual Metadata Management UI (6-8 hours)
- [ ] Search and select correct external match
- [ ] Upload custom images for media
- [ ] Set image priorities (which to display first)
- [ ] Refresh/delete metadata
- [ ] Edit metadata fields manually

**Success Criteria**:
- Missing posters automatically downloaded from TMDb
- Cast and crew information displayed
- Music albums have cover art from MusicBrainz
- Users can override incorrect automatic matches

### 7. Production Readiness Improvements
**Priority**: Medium
**Estimated Effort**: 12-15 hours

#### Logging Infrastructure (4-5 hours)
- [ ] Replace fmt.Printf with structured logging (slog)
- [ ] Add log levels (DEBUG, INFO, WARN, ERROR)
- [ ] Log request/response in API handlers
- [ ] Log FFmpeg commands and errors
- [ ] Log database query performance (slow queries)

#### Error Handling (3-4 hours)
- [ ] Panic recovery middleware for all goroutines
- [ ] Graceful shutdown on SIGTERM/SIGINT
- [ ] Database connection health checks
- [ ] FFmpeg process error handling improvements

#### Monitoring & Health (3-4 hours)
- [ ] Health check endpoint (`GET /health`)
- [ ] Readiness endpoint (`GET /ready`)
- [ ] Metrics endpoint (`GET /metrics`) - basic counts
- [ ] Database connection pooling audit

#### Database Operations (2-3 hours)
- [ ] Automated backup script for SQLite
- [ ] Database restore documentation
- [ ] PostgreSQL connection pooling configuration
- [ ] Query performance monitoring

**Success Criteria**:
- All logs use structured format (JSON or key-value)
- Application recovers gracefully from panics
- Health endpoints return accurate status
- Database connections properly pooled

---

## Future Phases (Planned, Not Started)

### Phase 7: Advanced Features

#### Search (8-10 hours)
- [ ] Global search across all media types
- [ ] Fuzzy matching and relevance scoring
- [ ] Search history and suggestions
- [ ] Keyboard shortcut (Ctrl+K or /)

#### Dark Mode (4-6 hours)
- [ ] Theme context and persistence
- [ ] Dark color palette design
- [ ] Theme toggle UI component
- [ ] Smooth transitions between themes

#### Watchlists & Collections (10-12 hours)
- [ ] User-created collections
- [ ] Watchlist functionality
- [ ] Collection management UI
- [ ] Share collections (future enhancement)

#### Recommendations (12-15 hours)
- [ ] Based on watch history analysis
- [ ] Similar media suggestions
- [ ] "Because you watched..." sections
- [ ] Genre-based recommendations

### Phase 8: Deployment & Operations

#### Docker & Deployment (10-12 hours)
- [ ] Docker images for backend and frontend
- [ ] Docker Compose for local deployment
- [ ] Kubernetes manifests for production
- [ ] Reverse proxy configuration (nginx/Traefik)
- [ ] SSL/TLS certificate management (Let's Encrypt)

#### Monitoring (8-10 hours)
- [ ] Prometheus metrics exporter
- [ ] Grafana dashboards
- [ ] Alerting rules (disk space, errors, etc.)
- [ ] Log aggregation (Loki or ELK)

#### Documentation (6-8 hours)
- [ ] User documentation (installation, usage)
- [ ] Admin documentation (configuration, maintenance)
- [ ] API documentation with examples
- [ ] Troubleshooting guide
- [ ] Plugin development guide (if implementing plugins)

---

## Optional Enhancements (Lower Priority)

### Audio Player Improvements
- [ ] Custom audio controls (similar to video player)
- [ ] Playlist support
- [ ] Gapless playback
- [ ] Audio visualization
- [ ] Queue management

### Advanced Media Features
- [ ] Subtitle support (SRT, ASS, VTT)
- [ ] Multiple audio track selection in UI
- [ ] Chapter markers
- [ ] Intro/outro detection and skip buttons
- [ ] Watch together / synchronized playback
- [ ] Chromecast / DLNA support

### Hardware Acceleration
- [ ] NVENC configuration for Nvidia GPUs
- [ ] QuickSync for Intel GPUs
- [ ] VideoToolbox for macOS
- [ ] VAAPI for Linux
- [ ] Fallback to software encoding when GPU unavailable

---

## Development Guidelines

### Before Starting New Work
1. Read [PROJECT_STATUS.md](./PROJECT_STATUS.md) for current state
2. Check [CONVENTIONS.md](../development/CONVENTIONS.md) for code patterns
3. Review [ARCHITECTURE.md](../core/ARCHITECTURE.md) for layer rules
4. Ensure understanding of dual database compatibility

### During Implementation
1. Write tests FIRST (TDD when possible)
2. Maintain Clean Architecture boundaries
3. Update Swagger docs when adding API endpoints
4. Test on both SQLite and PostgreSQL
5. Use structured logging (slog), not fmt.Printf

### Before Marking "Complete"
1. [ ] Code compiles without errors
2. [ ] All tests pass (100% success rate)
3. [ ] Feature works end-to-end (manually verified)
4. [ ] Documentation updated
5. [ ] No critical TODOs or blockers
6. [ ] Code reviewed (or self-reviewed carefully)

---

## Key Technical Decisions

### Completed Architecture Decision Records
- **ADR 005**: On-Demand Transcoding Strategy (4-tier approach)
- **ADR 006**: Image Handling Strategy (hash-based caching)
- **ADR 007**: Unified Task Scheduler (cron-based)
- **ADR 012**: Music Database Architecture (virtual entities)
- **ADR 014**: Library Scanner Resilience (error recovery)

### Pending Decisions
- **Authentication Strategy**: JWT vs session-based (leaning JWT)
- **Image Caching**: Implement now vs defer (needs decision)
- **External API Priority**: TMDb first or MusicBrainz first?
- **Deployment Target**: Docker Compose vs Kubernetes for initial production

---

## Success Metrics

### Current State (Nov 22, 2025)
- **Lines of Code**: ~71,000 total
- **Test Pass Rate**: 100% (188/188 tests)
- **API Endpoints**: 65+ implemented
- **Database Migrations**: 9 complete
- **Compilation**: Go ✅ | TypeScript ✅

### Goals for Next Milestone

- [x] TypeScript compilation: 0 errors - **COMPLETED Nov 22, 2025**
- [x] Real repositories: 100% implemented (NoOp removed) - **COMPLETED Nov 22, 2025**
- [x] Technical debt audit: Complete and documented - **COMPLETED Nov 22, 2025**
- [ ] User authentication: Complete and tested
- [ ] Test coverage: >60% actual code path coverage
- [ ] Production deployment: Docker images ready

---

## Resources

### Documentation
- **[PROJECT_STATUS.md](./PROJECT_STATUS.md)** - Current implementation status
- **[ARCHITECTURE.md](../core/ARCHITECTURE.md)** - System design
- **[CONVENTIONS.md](../development/CONVENTIONS.md)** - Code patterns
- **[TESTING.md](../development/TESTING.md)** - Testing strategy

### Code Reviews
- **[Backend Review (Nov 21)](../reviews/backend-code-review-2025-11-21.md)** - Comprehensive analysis

### ADRs
- **[All Decisions](../decisions/README.md)** - Complete ADR index

---

**Last Updated**: November 22, 2025
**Next Review**: After completing P0 blockers and implementing user authentication
