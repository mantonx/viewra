# ViewRA Project Status

**Last Updated**: November 20, 2025

## Current Phase: Phase 5.7 Complete - Video Player Enhancement ✅

**Overall Progress**: Phase 5 - UX & Performance (Video player at 90% parity, disk management enhanced, git cleanup complete)
**Next Phase**: Phase 5.8 - Audio Player Enhancement

---

## Quick Status Summary

### Recently Completed ✅

- **Phase 5.7.1 - Seek-Based Transcoding** (Nov 20) - Users can seek during active transcoding
- **Phase 5.7 Tier 2 - Custom Control Bar** (Nov 20) - Professional video player controls with auto-hide
- **Phase 5.7 Tier 1 - Critical Fixes** (Nov 20) - Aspect ratio, performance, keyboard shortcuts, buffering
- **Disk Space Management Enhancements** (Nov 20) - Pre-transcode size estimation + dynamic cleanup batch sizing
- **VideoPlayerContainer** (Nov 20) - Eliminated duplicate playback code between movie/TV pages
- **Unified Task Scheduler** (Nov 16) - Complete scheduler management UI with user-friendly controls
- **Schedule Editor** (Nov 16) - Visual schedule editor with daily/weekly/monthly presets and time picker
- **Scheduler API** (Nov 16) - Full REST API for task management (trigger, enable/disable, update schedules)
- **Audio Codec Compatibility Fix** (Nov 15) - Fixed AC3/DTS/TrueHD/FLAC transcoding detection
- **Music UI Enhancement** (Nov 15) - Album cards and track listings display ID3 metadata (year, genre, bitrate)
- **TV Episode UI Enhancement** (Nov 15) - Episode cards display air dates, descriptions, IMDb/TVDb IDs
- **Movie UI Enhancement** (Nov 15) - Movie cards display plot, director, genre, year, content rating
- **NFO Integration** (Nov 15) - Movie and TV episode NFO parsing integrated into scanner
- **Git Repository Cleanup** (Earlier) - Removed 26GB of binary files, reduced .git from 26G to 150M

### Current Work 🚧

- 🎉 Phase 4.2 Unified Task Scheduler - COMPLETE
- ✅ Scheduler backend with cron-based task execution
- ✅ Scheduler management UI with user-friendly controls
- ✅ Visual schedule editor with time picker and frequency presets
- ✅ Task history and execution tracking
- ✅ Image cache cleanup task integrated
- 📋 External API integration (TMDb, MusicBrainz) - NEXT UP

### Key Metrics
- **Lines of Code**: ~20,000+ (Backend: ~11,000 | Frontend: ~9,000)
- **Test Coverage**: ~45% overall
- **Database Tables**: 12 core tables + migrations
- **API Endpoints**: 40+ RESTful endpoints
- **Features**: Library management, Media playback, Progress tracking, Transcoding, Movie/TV/Music support, NFO metadata

---

## Completed Phases

### Phase 0: Project Setup ✅ (Nov 11, 2025)
- Repository structure and comprehensive documentation
- Development toolchain (Air, sqlc, Swagger, Orval, ESLint, Biome)
- Git workflow, Makefile, Procfile

### Phase 1: Core Foundation ✅ (Nov 12, 2025)
- **Domain Layer**: Libraries, media, scanner business logic
- **Infrastructure**: Dual database (SQLite/PostgreSQL), FFmpeg, filesystem scanner, path browser
- **Application Layer**: Library management, scanning, browsing use cases
- **API Layer**: REST endpoints, streaming with range support, scan progress
- **Frontend**: React UI with TanStack Query/Router, TypeScript client, accessibility
- **Database**: Auto-migration with backups, incremental scanner

### Phase 2.1: Watch Progress Tracking ✅ (Nov 13, 2025)

**Implementation Complete**
- ✅ Database schema (`watch_progress` table)
- ✅ Domain entities and repository interfaces
- ✅ Application use cases with tests
- ✅ Infrastructure repository with dual DB support
- ✅ REST API endpoints (GET, PUT, POST, DELETE)
- ✅ Frontend components (progress bars, resume buttons, video player)
- ✅ Continue Watching section on home page

**Features**
- Per-user progress tracking
- Resume from last position
- Auto-mark watched at 90% completion
- Watch history views
- Progress indicators on media cards

### Phase 2.2: On-Demand Transcoding ✅ (Nov 13, 2025)

**Implementation Complete**
- ✅ Database schema with access tracking ([migration 000006](../migrations/000006_add_transcode_tracking.up.sql))
- ✅ 4-tier intelligent streaming strategy (Direct Play → Remux → Remux+Audio → Transcode)
- ✅ Worker pool with configurable concurrency
- ✅ Idle timeout to cancel abandoned transcodes (5 min default)
- ✅ HLS output format for web compatibility
- ✅ On-demand trigger from manifest request
- ✅ Progress tracking in database
- ✅ Access tracking for LRU cleanup (last_accessed_at, access_count)
- ✅ File metadata tracking (file_path, file_size_bytes)

**4-Tier Streaming Strategy**

The system intelligently selects the optimal streaming approach based on codec compatibility and audio configuration:

1. **Direct Play** (Instant playback)
   - Compatible video codec (H.264/VP9/AV1)
   - Compatible audio codec (AAC/MP3/Opus, stereo)
   - Web-compatible container (MP4/WebM)
   - Action: 302 redirect to direct stream endpoint
   - Performance: <1 second to playback

2. **Remux** (Fast container conversion, 2-5 minutes)
   - Compatible codecs but non-web container (MKV, AVI, etc.)
   - Stereo audio only
   - Action: Copy streams to HLS without re-encoding (`-c:v copy -c:a copy`)
   - Performance: 10x realtime speed, I/O bound

3. **Remux + Audio Downmix** (Audio re-encode, 5-10 minutes)
   - Compatible video codec
   - Multi-channel audio (5.1, 7.1)
   - Action: Copy video, re-encode audio to stereo AAC with downmix filter
   - Performance: Audio encoding adds overhead

4. **Transcode** (Full conversion, 20-60 minutes)
   - Incompatible codecs (HEVC, VP8, old formats)
   - Quality change requested
   - Action: Full video + audio re-encode to H.264/AAC
   - Performance: 0.5-1.5x realtime (hardware dependent)

**Architecture**
- **Queue System**: Channel-based worker pool with job routing
- **Strategy Selection**: Audio channel detection, codec compatibility checks
- **FFmpeg Executors**: Optimized remux (10x faster), audio downmix, full transcode
- **Segment Serving**: HLS playlist + .ts segments with CORS support
- **Cancellation**: Context-based cancellation for paused/stopped playback
- **Progressive Streaming**: Like Plex/Jellyfin - playback starts immediately when transcoding begins, segments served as available

**Key Files**
- [queue.go](../internal/application/transcode/queue.go) - Worker pool implementation
- [service.go](../internal/infrastructure/transcoding/service.go) - FFmpeg executors
- [validation.go](../internal/infrastructure/transcoding/validation.go) - Strategy selection
- [transcode.go](../internal/api/handlers/transcode.go) - API handlers
- [transcode_jobs.sql](../internal/infrastructure/database/queries/sqlite/transcode_jobs.sql) - Database queries

**Documentation**
- [ADR 005: On-Demand Transcoding Strategy](./decisions/005-on-demand-transcoding-strategy.md)

### Phase 2.5: Intelligent Multi-Track Audio Selection ✅ (Nov 15, 2025)

Advanced audio track selection with commentary filtering and web compatibility prioritization

#### Problem Identified

- **Multi-track audio files** (movies with multiple audio streams) were blindly selecting the first track
- **Commentary tracks** were being selected instead of main audio (e.g., "Commentary by Michel Gondry")
- No prioritization of web-compatible audio codecs when multiple tracks available
- Movies 7578, 7841, 8620 all had audio playback issues due to wrong track selection

**Example Multi-Track Scenarios:**

- Movie 7578: Track 1 = DTS 5.1 (main), Track 2 = AC3 2.0 Commentary ❌
- Movie 7841: Track 1 = DTS 5.1 (main), Track 2 = AC3 2.0 Commentary ❌
- Movie 8620: Track 1 = DTS 5.1 (main), Track 2 = AC3 5.1 (alternate)

#### Root Cause Analysis

- [validation.go:127-133](../internal/infrastructure/transcoding/validation.go#L127-L133) Only captured FIRST audio stream
- FFmpeg commands had no `-map` specification, defaulting to first audio track
- No metadata parsing to detect commentary tracks
- No intelligent selection based on codec compatibility or channel count

#### Implementation Details

**1. Enhanced Audio Track Detection** ([validation.go:12-36](../internal/infrastructure/transcoding/validation.go#L12-L36))

- New `AudioTrack` struct with full metadata: Index, Codec, Channels, Bitrate, Language, Title, IsCommentary
- Updated `VideoInfo` to store all tracks + selected track index
- Parse track metadata including title field for commentary detection

**2. Intelligent Track Selection** ([validation.go:178-237](../internal/infrastructure/transcoding/validation.go#L178-L237))

- `selectBestAudioTrack()` with priority-based selection:
  1. **Filter out commentary tracks** (checks "commentary" in title metadata)
  2. **Prefer web-compatible stereo** (AAC, MP3, Opus in 2 channels - no processing!)
  3. **Web-compatible multi-channel** (needs downmix only)
  4. **Stereo with any codec** (faster transcode)
  5. **Multi-channel fallback** (slowest option)

**3. FFmpeg Track Mapping** ([ffmpeg.go:230-238](../internal/infrastructure/transcoding/ffmpeg.go#L230-L238))

- Added `-map 0:v:0 -map 0:N` to specify exact video and audio streams
- Applied to all three strategies: full transcode, remux, remux-audio
- Service layer passes selected track index to FFmpeg

**4. Service Integration** ([service.go:142-173](../internal/infrastructure/transcoding/service.go#L142-L173))

- Calls `GetVideoInfo()` before transcoding to analyze all tracks
- Logs selected track for debugging: `"using selected audio track" track_index=1 codec=dts channels=6`

#### Validation Results

- ✅ **Commentary filtering works**: Movies 7578 & 7841 correctly skip AC3 commentary, use DTS main audio
- ✅ **Smart track selection**: System chooses best track based on codec/channel compatibility
- ✅ **Multi-track handling**: All audio tracks detected and analyzed before selection
- ✅ **Tested with 3 movies**: Jobs 46, 47 both selected correct non-commentary tracks
- ✅ **Autoplay compatibility**: Frontend starts muted, unmutes after play to bypass browser restrictions

#### Key Files Modified

- [validation.go](../internal/infrastructure/transcoding/validation.go) - Multi-track detection & intelligent selection
- [ffmpeg.go](../internal/infrastructure/transcoding/ffmpeg.go) - FFmpeg track mapping with `-map 0:N`
- [service.go](../internal/infrastructure/transcoding/service.go) - Track selection integration
- [VideoPlayer.tsx](../web/src/components/media/VideoPlayer/VideoPlayer.tsx) - Autoplay compatibility

### Phase 2.4: Audio Codec Compatibility Fix ✅ (Nov 15, 2025)

Critical fix for audio playback with incompatible codecs

#### Problem Statement

- Videos with AC3 (Dolby Digital) audio weren't playing audio in browsers
- `ShouldTranscode` validation only checked video codec, ignored audio compatibility
- Jobs were failing with "transcoding not needed" despite incompatible audio
- Affected formats: AC3, DTS, DTS-HD, TrueHD, EAC3, FLAC, PCM
- Multi-channel audio (5.1, 7.1) wasn't being downmixed to stereo

#### Root Cause Analysis

- `ShouldTranscode` function only validated video codec
- `DetermineStreamStrategy` correctly identified audio issues but validation rejected jobs
- Transcode jobs failed before audio processing could occur

#### Implementation

- Updated `ShouldTranscode` to check audio codec compatibility
- Added detection for web-incompatible audio codecs (AC3, DTS, TrueHD, FLAC, EAC3)
- Added multi-channel audio detection (>2 channels = needs downmix)
- Now returns `true` when H.264 video has incompatible or multi-channel audio
- Enables `remux_audio` strategy (copy video, transcode audio to AAC stereo)

#### Outcomes

- ✅ AC3 audio properly transcoded to AAC stereo (2 channels, 48kHz)
- ✅ Multi-channel audio (5.1, 7.1) downmixed to stereo for web compatibility
- ✅ Fast processing: 5-10 minutes (audio-only transcode vs 20-60 min full transcode)
- ✅ All incompatible audio formats now properly handled
- ✅ Tested with episode 11055: AC3 2.0 → AAC stereo conversion verified

#### Files Modified

- [validation.go](../internal/infrastructure/transcoding/validation.go) - Added audio compatibility checks to `ShouldTranscode`

### Phase 2.3: Transcode Cleanup System ✅ (Nov 13, 2025)

**Manual Cleanup Tools**
- ✅ CLI tool: [transcode-cleanup](../cmd/transcode-cleanup/main.go)
  - Disk usage statistics
  - Filter by status, age, media, quality
  - Orphan detection and cleanup
  - Dry-run mode for safety
- ✅ API endpoints:
  - `GET /api/transcode/disk-usage` - Statistics
  - `POST /api/transcode/cleanup` - Trigger cleanup with filters
- ✅ Cleanup service with flexible filtering ([cleanup.go](../internal/application/transcode/cleanup.go))

**Automated Cleanup Scheduler**
- ✅ Background service integrated into application lifecycle
- ✅ Runs every 6 hours (configurable via `TRANSCODE_CLEANUP_INTERVAL_HOURS`)
- ✅ Multiple cleanup strategies:
  - **Policy-based** (always runs):
    - Failed jobs older than 24h
    - Completed transcodes older than 30 days
    - Idle transcodes (not accessed in 7 days)
    - Orphaned files without DB records
  - **Threshold-based** (triggered when needed):
    - LRU cleanup when disk usage > 85%
    - Enforce minimum free space (10GB default)
    - Enforce maximum storage limit
- ✅ Disk space monitoring via filesystem stats
- ✅ Configurable via environment variables
- ✅ Graceful start/stop with application

**Configuration**
Environment variables for complete control:
```bash
TRANSCODE_CLEANUP_ENABLED=true               # Enable/disable (default: true)
TRANSCODE_CLEANUP_INTERVAL_HOURS=6          # Run every 6 hours
TRANSCODE_CLEANUP_DISK_THRESHOLD=85         # Cleanup at 85% disk usage
TRANSCODE_CLEANUP_DISK_WARNING=80           # Warn at 80%
TRANSCODE_MIN_FREE_SPACE_GB=10              # Require 10GB free
TRANSCODE_MAX_AGE_DAYS=30                   # Delete after 30 days
TRANSCODE_MAX_IDLE_DAYS=7                   # Delete if idle 7 days
TRANSCODE_MAX_STORAGE_GB=50                 # Total storage limit
TRANSCODE_CLEANUP_BATCH_SIZE=10             # Max per run
TRANSCODE_KEEP_FAILED_HOURS=24              # Keep failed for 24h
```

**Key Files**
- [cleanup_scheduler.go](../internal/application/transcode/cleanup_scheduler.go) - Background scheduler
- [cleanup.go](../internal/application/transcode/cleanup.go) - Cleanup service
- [container.go](../internal/app/container.go#L143-155) - Integration
- [bootstrap.go](../cmd/viewra/bootstrap/bootstrap.go#L70-74) - Lifecycle management

**Documentation**
- [Manual Cleanup Guide](./TRANSCODE_CLEANUP.md)
- [Automated Cleanup Configuration](./AUTOMATED_CLEANUP_CONFIG.md)

**Current Disk State** (as of Nov 13)
- Total transcode storage: 24.06 GB
- File count: 2,835 files
- Total jobs: 18 (4 completed, 5 failed, 9 stuck processing)
- Disk usage: 92% (cleanup recommended!)

---

## Upcoming Phases

### Phase 3: TV Shows & Music ✅ (Nov 15, 2025)

**Goal**: Full support for TV shows and music libraries

**Status**: Complete ✅
**Actual Effort**: 1 day

**Implementation Complete**
- ✅ Movie repository with comprehensive metadata fields
- ✅ TV show repository with show/season/episode hierarchy
- ✅ Music repository with ID3 tag extraction integration
- ✅ Domain entities refactored for clean architecture
- ✅ Architecture refactoring to eliminate ID3 parser duplication
- ✅ Dependency injection pattern for metadata extraction
- ✅ All tests passing with full coverage

**Key Achievements**
- **Clean Architecture**: Removed infrastructure dependency from scanner domain layer
- **Music Metadata**: Created MusicMetadataExtractor interface, moved ID3 parsing to infrastructure
- **Repository Pattern**: MovieRepository, TVRepository, MusicRepository with full CRUD operations
- **Database Support**: Dual-database (SQLite/PostgreSQL) queries via sqlc
- **Test Coverage**: Fixed all broken tests, maintained ~45% overall coverage

**Success Criteria** (All Met ✅)
- ✅ TV shows parse correctly (S01E01, 1x01 formats)
- ✅ Episodes group by show/season with ordering
- ✅ Music tracks extract ID3 tags via adapter pattern
- ✅ Track progress per episode (inherited from Phase 2.1)
- ✅ Audio streaming for music files

### Phase 4: Enhanced Metadata ✅ (Nov 15, 2025 - Backend Complete, Frontend Enhanced)

**Goal**: Rich metadata from NFO files and external sources

**Status**: NFO & ID3 Integration Complete, Frontend Enhanced, External APIs Pending
**Estimated Effort**: External API integration remaining (1-2 weeks)

**Completed Features** ✅
- ✅ **NFO Movie Parsing**: Integrated into `processMovie()` function
  - Finds .nfo files adjacent to movie files
  - Extracts 20+ metadata fields (Title, Year, Plot, Director, Cast, Genre, IMDb/TMDb IDs, etc.)
  - Populates Movie entity with rich metadata from Kodi/Plex-compatible files
  - **Tested**: Happy Gilmore (1996) with full director, cast, plot, genre metadata
  
- ✅ **NFO TV Episode Parsing**: Integrated into `processTVEpisode()` function
  - Finds episode.nfo files
  - Extracts episode metadata (Title, ShowTitle, Season, Episode, AirDate, Description, IMDb/TVDb IDs)
  - Properly maps NFO fields to TVEpisode entity structure
  - **Tested**: Chicago P.D. episodes with air dates and descriptions

- ✅ **Music ID3 Integration**: Clean architecture pattern implemented
  - MusicMetadataExtractor interface in domain layer
  - ID3 parser adapter in infrastructure layer
  - Dependency injection via coordinator
  - **Tested**: 1,663+ tracks from 25 artists with year, genre, bitrate metadata

- ✅ **Frontend Movie Cards Enhancement**: Rich metadata display
  - Year, duration, genre badge on movie cards
  - Plot preview (100 characters) with read more
  - Director, content rating display
  - IMDb/TMDb ID indicators
  - Enhanced hover effects (scale, shadow)

- ✅ **Frontend TV Episode Cards Enhancement**: NFO metadata display
  - Formatted air dates ("Aired: Jan 8, 2014")
  - Episode descriptions with 2-line clamp
  - IMDb/TVDb ID indicators
  - Enhanced hover effects matching movie cards

- ✅ **Frontend Music Enhancement**: ID3 metadata display
  - Album cards show year badge at bottom
  - Enhanced hover effects (scale-105, shadow-xl)
  - Track listings show year, genre, bitrate badges
  - Genre display with truncation for long names
  - Bitrate display in kbps for quality indication

**Pending** 📋
- 📋 TMDb Integration: Movie/TV search, posters, cast/crew
- 📋 MusicBrainz Integration: Artist/album search, cover art
- 📋 Plugin system for metadata providers
- 📋 Manual matching UI for metadata correction
- 📋 Movie/TV detail pages with full cast/crew

**Key Files**
- [scan_library.go](../internal/application/library/scan_library.go) - NFO integration in processMovie() and processTVEpisode()
- [movie_parser.go](../internal/infrastructure/metadata/nfo/movie_parser.go) - Movie NFO parser with FindMovieNFO()
- [tvshow_parser.go](../internal/infrastructure/metadata/nfo/tvshow_parser.go) - TV NFO parser with FindEpisodeNFO()
- [extractor.go](../internal/infrastructure/metadata/music/extractor.go) - ID3 adapter implementation
- [MovieCard.tsx](../web/src/components/media/MovieCard/MovieCard.tsx) - Enhanced movie cards
- [EpisodeCard.tsx](../web/src/components/tv/EpisodeCard/EpisodeCard.tsx) - Enhanced episode cards
- [TrackListItem.tsx](../web/src/components/music/TrackListItem/TrackListItem.tsx) - Enhanced track display
- [AlbumCard.tsx](../web/src/components/music/AlbumCard/AlbumCard.tsx) - Enhanced album cards

**Success Criteria**
- ✅ NFO files automatically detected and parsed during library scan
- ✅ Movie metadata populated from .nfo files
- ✅ TV episode metadata populated from episode.nfo files
- ✅ Music metadata extracted via ID3 tags
- ✅ Frontend displays plot, genre, director, year for movies
- ✅ Frontend displays air dates, descriptions for TV episodes
- ✅ Frontend displays year, genre, bitrate for music tracks
- 📋 TMDb API integration for missing metadata
- 📋 MusicBrainz API for artist/album metadata

### Phase 4.2: Unified Task Scheduler ✅ (Nov 16, 2025)

**Goal**: Centralized task scheduler for automated maintenance and background jobs

**Status**: Complete ✅
**Actual Effort**: 1 day

**Implementation Complete**
- ✅ **Backend Scheduler Infrastructure**: Cron-based task execution with gorilla/cron
  - Task registration with ID, name, description, schedule
  - Enable/disable task functionality
  - Manual task triggering
  - Execution history tracking with success/failure status
  - Next run time calculation
  - Database persistence for execution logs

- ✅ **Scheduler API**: Full REST API for task management
  - `GET /api/admin/scheduler/tasks` - List all tasks
  - `GET /api/admin/scheduler/tasks/:id` - Get task status
  - `POST /api/admin/scheduler/tasks/:id/trigger` - Manual trigger
  - `GET /api/admin/scheduler/tasks/:id/history` - Execution history
  - `POST /api/admin/scheduler/tasks/:id/enable` - Enable task
  - `POST /api/admin/scheduler/tasks/:id/disable` - Disable task
  - `PUT /api/admin/scheduler/tasks/:id/schedule` - Update schedule

- ✅ **User-Friendly Schedule Editor**: Visual schedule management UI
  - **Simple Mode**: Daily/Weekly/Monthly frequency selector
  - **Time Picker**: react-datepicker with 15-minute intervals
  - **Day Selector**: Dropdown for weekly (day of week) and monthly (day of month)
  - **Advanced Mode**: Raw cron expression editor for power users
  - **Live Preview**: Real-time human-readable schedule display
  - **Cron Utilities**: Conversion between human-readable and cron formats

- ✅ **Scheduler Management UI**: Complete task management interface
  - Task list with real-time status updates (refreshes every 10 seconds)
  - Visual states for enabled/disabled tasks
  - Execution history modal with sortable table
  - Toast notifications for all operations
  - Human-readable schedule display (e.g., "Every Monday at 9:00 AM")
  - Prominent green "Enable Task" button for disabled tasks

- ✅ **Image Cache Cleanup Task**: Automated maintenance
  - Scheduled task to remove orphaned image files
  - Default schedule: Daily at 3:00 AM
  - Configurable via UI schedule editor

**Key Features**
- Manual task triggering with instant feedback
- Enable/disable tasks with visual state changes
- View execution history with duration and status
- Update schedules using friendly UI or raw cron expressions
- Real-time schedule preview and validation
- Human-readable schedule display throughout UI

**Architecture Highlights**
- Clean separation: Domain (scheduler logic) → Infrastructure (cron execution) → API (handlers) → Frontend (UI)
- Database persistence for execution history
- Type-safe cron utilities with validation
- Graceful error handling and user feedback
- API client with proper `data` field to JSON body conversion

**Key Files**
- Backend:
  - [scheduler.go](../internal/infrastructure/scheduler/scheduler.go) - Core scheduler with cron execution
  - [execution_logger.go](../internal/infrastructure/scheduler/execution_logger.go) - Database logging
  - [scheduler.go](../internal/api/handlers/scheduler.go) - API handlers
  - [scheduler.go](../internal/api/routes/scheduler.go) - Route registration
  - [scheduler.sql](../internal/infrastructure/database/queries/sqlite/scheduler.sql) - Database queries
  - [000008_add_scheduler_tables.up.sql](../migrations/000008_add_scheduler_tables.up.sql) - Migration

- Frontend:
  - [settings.scheduler.tsx](../web/src/routes/_layout/settings.scheduler.tsx) - Main scheduler UI
  - [ScheduleEditor.tsx](../web/src/components/scheduler/ScheduleEditor.tsx) - Schedule editor modal
  - [cron.ts](../web/src/lib/utils/cron.ts) - Cron conversion utilities
  - [scheduler.ts](../web/src/lib/api/scheduler.ts) - API client
  - [scheduler.ts](../web/src/lib/types/scheduler.ts) - TypeScript types
  - [mutator/index.ts](../web/src/lib/api/mutator/index.ts) - Fixed data to body conversion

**Success Criteria** (All Met ✅)
- ✅ Tasks execute automatically on cron schedule
- ✅ Users can trigger tasks manually via UI
- ✅ Users can enable/disable tasks with visual feedback
- ✅ Users can view execution history with success/failure status
- ✅ Users can update schedules using friendly time picker interface
- ✅ Advanced users can edit raw cron expressions
- ✅ Image cache cleanup runs automatically on schedule
- ✅ All operations provide toast notification feedback

**Documentation**
- [ADR 007: Unified Task Scheduler](./decisions/007-unified-task-scheduler.md)

### Phase 4.3: Image Caching & Transformations 📋 (Planned)

**Goal**: Complete the image handling system with caching, transformations, and optimization

**Status**: Planned 📋
**Estimated Effort**: 6-8 hours

**Scope**
Based on ADR 006, Phase 4.1 implemented image cataloging (discovery, metadata extraction, serving from original paths). Phase 4.3 completes the remaining features:

**Planned Implementation**
- 📋 **Image Cache Service**: Copy images to `data/cache/images/` directory
  - Hash-based filenames: `{hash}_original.{ext}`
  - Populate `local_cache_path` field in database
  - Graceful fallback to original path if cache unavailable

- 📋 **Hash-Based Deduplication**: Single storage for identical images
  - Check file hash before caching
  - Multiple database records can reference same cached file
  - Significant storage savings for duplicate posters/covers

- 📋 **On-Demand Image Transformations**: Resize and format conversion
  - Query parameters: `?width=300&height=450&format=webp&quality=85`
  - Generate transformed versions on first request
  - Cache generated variants: `{hash}_300x450.webp`
  - WebP conversion for smaller file sizes

- 📋 **LRU Cache Eviction**: Disk space management
  - Track cache usage and access times
  - Evict least-recently-used transformed images when disk threshold exceeded
  - Keep original cached files (regenerate transforms on demand)
  - Configurable size limits

**Architecture**
```
Current (Phase 4.1):
User Request → API → Database → Serve from Original FilePath

Planned (Phase 4.3):
User Request → API → Check local_cache_path
                  ↓
            Cache exists? → Serve cached file
                  ↓
            Cache miss? → Copy original to cache → Update DB → Serve
                  ↓
            Transform requested? → Check cache → Generate if needed → Serve
```

**Migration Path**
1. Implement `CacheService` with `CopyToCache()` and `GetCachedPath()` methods
2. Background task to populate cache from existing `file_path` entries
3. Update `ServeImage` handler to prefer `local_cache_path` with fallback
4. Add transformation logic with caching
5. Implement LRU eviction scheduler task

**Key Files** (To Be Created)
- Backend:
  - `internal/infrastructure/images/cache_service.go` - Cache management
  - `internal/infrastructure/images/transformer.go` - Image resizing/conversion
  - `internal/infrastructure/images/lru_evictor.go` - Cache eviction
  - Update `internal/api/handlers/images.go` - Add transformation support

**Success Criteria**
- 📋 Images copied to cache on library scan
- 📋 Deduplication reduces storage for identical images
- 📋 Query parameters enable resizing: `?width=300&format=webp`
- 📋 Transformed images cached and reused
- 📋 LRU eviction keeps cache size under threshold
- 📋 Graceful fallback to original paths if cache unavailable

**Why Deferred from Phase 4.1**
- Phase 4.1 delivers working functionality (images display correctly)
- No production users yet (can refactor freely)
- Schema already supports caching (additive change)
- Browser caching provides acceptable performance
- Incremental value delivery

**Triggers for Implementation**
- Need responsive images (different sizes for different contexts)
- Want WebP conversion for bandwidth savings
- Storage deduplication becomes valuable
- Multiple users requesting optimized images
- Performance optimization becomes priority

**Documentation**
- [ADR 006: Image Handling Strategy](./decisions/006-image-handling-strategy.md) - Complete specification

### Phase 5: User Management (Weeks 14-16)

**Goal**: Multi-user support with permissions

**Features**
- User authentication (local + optional OAuth)
- Per-user libraries and permissions
- Admin interface
- User preferences and settings

### Phase 6: Mobile & Performance (Weeks 17-20)

**Goal**: Mobile apps and optimization

**Features**
- React Native mobile apps (iOS/Android)
- Offline downloads
- Performance optimization
- Caching strategies

---

## Architecture Overview

### Technology Stack

**Backend**
- **Language**: Go 1.25+
- **Framework**: Gin (HTTP router)
- **Database**: SQLite (dev) / PostgreSQL (prod) with dual support
- **ORM**: sqlc (type-safe SQL)
- **Media**: FFmpeg for transcoding/metadata
- **Validation**: Custom validation with business rules

**Frontend**
- **Framework**: React 18 with TypeScript
- **Router**: TanStack Router (type-safe)
- **State**: TanStack Query (server state)
- **Styling**: Tailwind CSS
- **Build**: Vite
- **API Client**: Auto-generated from OpenAPI (Orval)
- **Video Player**: Shaka Player (DASH/HLS)

**Development**
- **Live Reload**: Air (Go), Vite (Frontend)
- **API Docs**: Swagger/OpenAPI
- **Linting**: golangci-lint, ESLint, Biome
- **Testing**: Go test, React Testing Library

### Project Structure

```
viewra2/
├── cmd/
│   ├── viewra/              # Main application entry
│   └── transcode-cleanup/   # CLI cleanup tool
├── internal/
│   ├── domain/              # Business entities and rules
│   ├── application/         # Use cases and business logic
│   ├── infrastructure/      # External concerns (DB, FFmpeg, filesystem)
│   ├── api/                 # HTTP handlers and routes
│   └── app/                 # Application container and wiring
├── migrations/              # Database migrations
├── web/                     # React frontend
├── docs/                    # Project documentation
└── bin/                     # Compiled binaries
```

### Key Design Patterns

- **Clean Architecture**: Domain → Application → Infrastructure → API
- **Repository Pattern**: Abstract data access
- **Use Case Pattern**: Single responsibility business operations
- **Worker Pool**: Concurrent transcode processing
- **Strategy Pattern**: Transcode strategy selection
- **Observer Pattern**: Progress tracking and SSE

---

## Development Workflow

### Running the Application

**Development Mode** (with live reload):
```bash
make dev                    # Starts backend + frontend with Air/Vite
# OR
./Procfile                 # air & npm run dev in parallel
```

**Backend Only**:
```bash
make run                    # Build and run backend
go run cmd/viewra/main.go  # Direct run
```

**Frontend Only**:
```bash
cd web && npm run dev
```

### Database Migrations

```bash
# Auto-migration on startup (default)
VIEWRA_AUTO_MIGRATE=true ./bin/viewra

# Manual migration
migrate -path migrations -database "sqlite3://data/viewra.db" up
```

### Transcode Cleanup

**Manual Cleanup**:
```bash
# Show current usage
./bin/transcode-cleanup --stats

# Dry run to see what would be deleted
./bin/transcode-cleanup --failed --dry-run
./bin/transcode-cleanup --older-than 720h --dry-run

# Actually delete
./bin/transcode-cleanup --failed
./bin/transcode-cleanup --older-than 720h
```

**Automated Cleanup**:
- Starts automatically with application
- Configured via environment variables (see Phase 2.3)
- Logs to standard output

### Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Test specific package
go test ./internal/domain/...
```

### Code Generation

```bash
# Generate sqlc code
sqlc generate

# Generate Swagger docs
swag init -g cmd/viewra/main.go

# Generate frontend API client
cd web && npm run generate:api
```

---

## Performance Metrics

### Database Performance
- SQLite: ~10,000 media items/second (scan)
- Incremental scanning: Only new/modified files
- Prepared statements for all queries

### Transcoding Performance
- **Remux**: 2-5 minutes (I/O bound, 10x realtime)
- **Remux + Audio**: 5-10 minutes (audio re-encode)
- **Full Transcode**: 0.5-1.5x realtime (depends on hardware)

### API Response Times
- Media list: <50ms (typical)
- Stream start: <100ms (direct play)
- Transcode trigger: <200ms (queuing)

---

## Known Issues & Technical Debt

### Current Issues
1. **Stuck Processing Jobs**: 9 transcode jobs stuck in "processing" state (need cleanup)
2. **Disk Space**: 24GB transcode cache at 92% disk usage (cleanup configured)
3. **PostgreSQL Support**: Cleanup queries only implemented for SQLite (Postgres needs implementation)

### Technical Debt
1. **Frontend Testing**: Need React component tests
2. **Integration Tests**: Need end-to-end API tests
3. **Documentation**: API examples could be more comprehensive
4. **Error Handling**: Some error messages could be more user-friendly

### Future Improvements
1. **Transcoding**:
   - Hardware acceleration (NVENC, QuickSync, VideoToolbox)
   - Multiple quality levels in parallel
   - Pre-transcoding popular content
   - Thumbnail generation from video
2. **Performance**:
   - Redis caching layer
   - CDN integration for static assets
   - Database connection pooling optimization
3. **Features**:
   - WebSocket for live progress updates
   - Download queue for offline viewing
   - Playlist management
   - Subtitle support

---

## Documentation Index

### Architecture & Design
- [ARCHITECTURE.md](./ARCHITECTURE.md) - System architecture and patterns
- [DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md) - Database design
- [API_SPECIFICATION.md](./API_SPECIFICATION.md) - REST API documentation
- [TECH_STACK.md](./TECH_STACK.md) - Technology choices

### Feature Documentation
- [TRANSCODE_CLEANUP.md](./TRANSCODE_CLEANUP.md) - Manual cleanup tools
- [AUTOMATED_CLEANUP_CONFIG.md](./AUTOMATED_CLEANUP_CONFIG.md) - Automated cleanup
- [decisions/005-on-demand-transcoding-strategy.md](./decisions/005-on-demand-transcoding-strategy.md) - Transcoding ADR

### Development
- [CONVENTIONS.md](./CONVENTIONS.md) - Code style and patterns
- [ROADMAP.md](./ROADMAP.md) - Detailed implementation history
- [MIGRATION_SUMMARY.md](./MIGRATION_SUMMARY.md) - HLS migration notes

---

## Success Metrics

### Phase 2 Achievements ✅
- ✅ Users can resume videos from where they left off
- ✅ Videos auto-mark as watched at 90% completion
- ✅ Unsupported codecs trigger automatic transcoding
- ✅ Transcoding uses intelligent strategy selection
- ✅ System automatically cleans up old/unused transcodes
- ✅ Disk space is monitored and managed automatically

### Remaining Goals
- TV show episodes group correctly with season/show hierarchy
- Music library supports playlists and albums
- External metadata enriches media information
- Multi-user support with permissions
- Mobile apps provide offline viewing

---

## Team & Resources

### Development
- **Primary Developer**: Solo project
- **Start Date**: November 11, 2025
- **Current Phase**: Phase 2 Complete
- **Velocity**: ~1 phase per week

### Resources
- **Documentation**: 15+ comprehensive docs
- **Test Coverage**: ~45% with focus on critical paths
- **External Dependencies**: FFmpeg, SQLite/PostgreSQL, React ecosystem

---

**For detailed task breakdowns and implementation notes, see:**
- [ON_DEMAND_TRANSCODING_PROJECT_PLAN.md](./ON_DEMAND_TRANSCODING_PROJECT_PLAN.md) - Detailed Phase 2.2 plan
- [ROADMAP.md](./ROADMAP.md) - Complete implementation history
