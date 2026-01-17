# ViewRA Project Plan

**Last Updated**: January 17, 2026

For current status, see [PROJECT_STATUS.md](PROJECT_STATUS.md).

---

## Recently Completed

### ✅ Plugin System

**Completed**: January 2026
**ADR**: [027-plugin-system-architecture.md](../decisions/027-plugin-system-architecture.md)

Full gRPC plugin system with SDK:

- Plugin SDK (`pkg/plugin/sdk/`) with base struct, enricher interface, host services
- Protocol definitions (`api/proto/plugin/`) for core, enricher, search, trending, vector search
- 8 plugins implemented: TMDb, MusicBrainz, Semantic Search, Recommendations, AI Features, AI Providers
- Plugin lifecycle management with hot reload (`make reload-plugin`)
- Event bus integration for real-time updates

### ✅ Semantic Search & Home Screen

**Completed**: January 2026

- Intent chips showing detected search intents
- Autocomplete with tiered ranking
- Targeted scan support
- Home screen widgets: Continue Watching, Recently Added, Trending, For You, Because You Watched
- Widget customization (reorder, hide)

### ✅ External Metadata Integration

**Completed**: January 2026

- TMDb plugin for movies/TV metadata and artwork
- MusicBrainz plugin for music metadata and cover art
- Trending content from TMDb for home screen

### ✅ Settings Infrastructure v2

**Completed**: December 3, 2025
**ADRs**: [032](../decisions/032-settings-infrastructure-v2.md), [033](../decisions/033-settings-ux-improvements.md)

- Environment variable awareness with source tracking
- System profile detection (CPU, memory, GPU)
- Category-level save with restart badges
- User preferences page

### ✅ Design System Improvements

**Completed**: December 2, 2025
**ADR**: [031-design-system-improvements.md](../decisions/031-design-system-improvements.md)

- "Cinema at Home" aesthetic across all UI
- Glass morphism, ambient glows, refined animations

### ✅ User Authentication

**Completed**: December 2, 2025
**ADR**: [028-user-authentication.md](../decisions/028-user-authentication.md)

- JWT-based authentication with sessions
- Per-user watch progress and ratings

---

## Upcoming Work

### 1. Multi-Language Audio & Subtitles

**Priority**: High
**ADR**: [030-multi-language-audio-subtitles.md](../decisions/030-multi-language-audio-subtitles.md)

Comprehensive audio track and subtitle support:

**Phase 1: Database & Scanning**

- Add `media_audio_tracks` and `media_subtitle_tracks` tables
- Extend scanner to extract embedded track metadata
- External subtitle file discovery (.srt, .vtt, .ass)

**Phase 2: API & Track Metadata**

- Track metadata API endpoints
- Library language preference settings
- Extended media response with tracks

**Phase 3: Multi-Audio HLS**

- Multi-audio master playlists with EXT-X-MEDIA tags
- Separate audio-only playlists per language
- Frontend audio track selector

**Phase 4: Subtitle Support**

- Subtitle extraction and WebVTT conversion
- Segmented subtitle playlists for HLS
- Frontend subtitle selector with SDH/forced indicators
- Auto-selection based on content language vs. preference

### 2. Hardware Acceleration

**Priority**: Medium

GPU-accelerated transcoding support:

- NVENC (Nvidia) - CUDA-based encoding
- QuickSync (Intel) - Intel integrated GPU
- VAAPI (Linux) - VA-API for AMD/Intel on Linux
- Automatic capability detection
- Fallback to software encoding

### 3. Advanced Playback

**Priority**: Low

- Chapter markers from MKV/MP4 metadata
- Intro skip detection and skip button
- Credits detection

---

## Future Features

### Collections & Watchlists

- User-created collections
- Watchlist management
- Sharing between users

### Deployment & Operations

- Docker Compose setup with all dependencies
- Kubernetes manifests
- Prometheus metrics endpoint
- Grafana dashboards

### Mobile Apps

- iOS app (Swift/SwiftUI)
- Android app (Kotlin/Compose)
- Offline playback support

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
