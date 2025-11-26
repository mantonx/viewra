# Project Plan: Multi-Language Audio and Subtitle Support (ADR 0004)

## Overview

This project plan outlines the implementation of multi-language audio track selection and subtitle support for ViewRA's video player, as described in ADR 0004.

## Real Library Context

Analysis of the actual media libraries revealed important characteristics that shape this implementation:

### Audio Tracks
- **~47% of movies have 2+ audio tracks** (e.g., original language + English dub)
- **Common codecs:** DTS/DTS-HD MA, AC3, TrueHD/Atmos, EAC3, FLAC, AAC
- **Channel configs:** 7.1 Atmos (8ch), 5.1 (6ch), stereo (2ch), mono (1ch)
- **Commentary tracks:** Detected via title field (e.g., "Commentary with director...")
- **All non-AAC audio requires transcoding** for web playback

### Subtitle Tracks
- **~69% are PGS (bitmap)** - cannot convert to WebVTT, burn-in only
- **~31% are SRT (text)** - can convert to WebVTT for HLS
- **202 external .srt files** in movies library
- **2,805 external .srt files** in TV library
- **Languages:** English dominant, then Spanish, French, German, Chinese, Portuguese

### External Subtitle Compatibility (Plex/Jellyfin)
Must support common naming conventions for broader user compatibility:
- **Patterns:** `{name}.{lang}.srt`, `{name}.{lang}.forced.srt`, `{name}.{lang}.sdh.srt`
- **Language codes:** Both ISO 639-1 (`en`) and ISO 639-2/B (`eng`)
- **Directories:** `subs/`, `subtitles/`, `Subs/`, `Subtitles/`
- **Special case:** `{name}.hi.srt` = Hindi, but `{name}.en.hi.srt` = English hearing-impaired

### Key Implementation Implications
1. **PGS subtitles are the majority** - must handle gracefully (burn-in option or mark as unavailable for soft subs)
2. **External SRT files are valuable** - provide text-based alternatives to PGS
3. **Audio transcoding already works** - extend to support track selection
4. **Regional variants matter** - display "Spanish (Latin American)" not just "spa"

## Phase 1: Database & Scanning Foundation

### 1.1 Database Schema

**Files to create/modify:**
- `migrations/000023_add_track_tables.up.sql`
- `migrations/000023_add_track_tables.down.sql`
- `migrations/postgres/000022_add_track_tables.up.sql`
- `migrations/postgres/000022_add_track_tables.down.sql`

**Tasks:**
- [ ] Create `media_audio_tracks` table
- [ ] Create `media_subtitle_tracks` table
- [ ] Add language preference columns to `libraries` table
- [ ] Create appropriate indexes for track lookups
- [ ] Run `~/go/bin/sqlc generate` after schema changes

### 1.2 SQLC Queries

**Files to create/modify:**
- `internal/infrastructure/database/queries/sqlite/audio_tracks.sql`
- `internal/infrastructure/database/queries/sqlite/subtitle_tracks.sql`
- `internal/infrastructure/database/queries/postgres/audio_tracks.sql`
- `internal/infrastructure/database/queries/postgres/subtitle_tracks.sql`

**Queries needed:**
- [ ] `InsertAudioTrack` - Insert single audio track
- [ ] `UpsertAudioTracks` - Bulk upsert audio tracks for a media item
- [ ] `GetAudioTracksByMediaID` - Get all audio tracks for a media item
- [ ] `DeleteAudioTracksByMediaID` - Clear tracks before re-scan
- [ ] `InsertSubtitleTrack` - Insert single subtitle track
- [ ] `UpsertSubtitleTracks` - Bulk upsert subtitle tracks for a media item
- [ ] `GetSubtitleTracksByMediaID` - Get all subtitle tracks for a media item
- [ ] `DeleteSubtitleTracksByMediaID` - Clear tracks before re-scan
- [ ] `UpdateLibraryLanguagePreferences` - Update library language settings

### 1.3 Domain Models

**Files to create/modify:**
- `internal/domain/media/audio_track.go` (new)
- `internal/domain/media/subtitle_track.go` (new)

**Tasks:**
- [ ] Define `AudioTrack` domain struct
- [ ] Define `SubtitleTrack` domain struct
- [ ] Add helper functions for language code to name conversion (ISO 639-2 → display name)

### 1.4 Scanner Extensions

**Files to modify:**
- `internal/infrastructure/transcoding/video_info.go`
- `internal/domain/scanner/media_scanner.go`
- `internal/infrastructure/persistence/media/repository.go`

**Tasks:**
- [ ] Extend `GetVideoInfo()` to extract all audio track metadata (not just selected one)
- [ ] Extend `GetVideoInfo()` to extract all subtitle track metadata
- [ ] Add external subtitle discovery function (Plex/Jellyfin compatible)
  - Search locations:
    - Same directory as media file
    - `subs/` or `Subs/` subdirectory
    - `subtitles/` or `Subtitles/` subdirectory
  - Filename patterns to parse:
    - `{name}.{lang}.srt` → language
    - `{name}.{lang}.forced.srt` → language + forced flag
    - `{name}.{lang}.sdh.srt` or `{name}.{lang}.hi.srt` → language + SDH flag
    - `{name}.{lang}.cc.srt` → language + closed captions
    - `{name}.default.{lang}.srt` → language + default flag
    - `{name}.srt` → assume library's preferred language
  - Supported formats: `.srt`, `.vtt`, `.ass`, `.ssa`, `.sub`
- [ ] Parse language codes: both ISO 639-1 (2-letter: `en`) and ISO 639-2/B (3-letter: `eng`)
- [ ] Handle `hi` ambiguity: `Movie.hi.srt` = Hindi, `Movie.en.hi.srt` = English hearing-impaired
- [ ] Detect commentary tracks from audio title field ("commentary", "director")
- [ ] Mark PGS/VOBSUB subtitles as `bitmap_only=true` (cannot convert to WebVTT)
- [ ] Persist audio tracks during scan
- [ ] Persist subtitle tracks during scan (embedded + external)

### 1.5 Repository Layer

**Files to create/modify:**
- `internal/infrastructure/persistence/media/audio_track_repository.go` (new)
- `internal/infrastructure/persistence/media/subtitle_track_repository.go` (new)

**Tasks:**
- [ ] Implement audio track repository with CRUD operations
- [ ] Implement subtitle track repository with CRUD operations
- [ ] Add methods to media repository to fetch tracks eagerly

---

## Phase 2: API & Track Metadata

### 2.1 API Endpoints

**Files to modify:**
- `internal/api/handlers/media.go`
- `internal/api/handlers/library.go`
- `internal/api/routes/media.go`
- `internal/api/routes/library.go`

**New endpoints:**
- [ ] `GET /api/media/:id/tracks` - Return audio and subtitle tracks
- [ ] `PATCH /api/libraries/:id` - Add support for language preference updates

**Tasks:**
- [ ] Create handler for `/api/media/:id/tracks`
- [ ] Create DTO structs for track responses
- [ ] Add track data to existing media detail response
- [ ] Add library language preference update handler
- [ ] Update OpenAPI spec (`make openapi`)
- [ ] Regenerate TypeScript client (`make api-client-gen`)

### 2.2 Library Language Settings

**Files to modify:**
- `internal/application/library/library_service.go`
- `internal/infrastructure/database/queries/sqlite/library.sql`
- `internal/infrastructure/database/queries/postgres/library.sql`

**Tasks:**
- [ ] Add `UpdateLanguagePreferences` method to library service
- [ ] Add SQLC query for updating library language preferences
- [ ] Return language preferences in library detail response

### 2.3 Frontend: Library Settings UI

**Files to modify:**
- `web/src/components/settings/LibrarySettings.tsx` (or create new)
- `web/src/lib/api/` (regenerated client)

**Tasks:**
- [ ] Add language preference dropdowns to library settings
  - Preferred audio language (default: English)
  - Preferred subtitle language (default: English)
  - Auto-enable subtitles: Always / Foreign content only / Never
- [ ] Create reusable language selector component with ISO 639-2 codes
- [ ] Save preferences via API

---

## Phase 3: Multi-Audio HLS Support

### 3.1 Master Playlist Generation

**Files to modify:**
- `internal/api/handlers/transcode.go` (ServeMasterPlaylist)
- `internal/infrastructure/transcoding/session.go`

**Tasks:**
- [ ] Modify `ServeMasterPlaylist` to generate `EXT-X-MEDIA` tags for audio tracks
- [ ] Create audio group ID and reference in `EXT-X-STREAM-INF`
- [ ] Set `DEFAULT=YES` based on library language preference or source default
- [ ] Include audio track language and title in `NAME` attribute

### 3.2 Separate Audio Playlists

**Files to create/modify:**
- `internal/api/handlers/transcode.go` (new handler)
- `internal/api/routes/transcode.go`
- `internal/infrastructure/transcoding/session.go`

**New endpoint:**
- [ ] `GET /api/media/:id/hls/audio/:lang/playlist.m3u8` - Audio-only HLS playlist

**Tasks:**
- [ ] Create handler for audio-only playlist requests
- [ ] Implement FFmpeg command for audio-only HLS segment generation
- [ ] Use `-map 0:a:{index}` to select specific audio track
- [ ] Store audio segments separately from video (`data/transcode/{media_id}/audio/{lang}/`)

### 3.3 Audio Transcoding

**Files to modify:**
- `internal/infrastructure/transcoding/ffmpeg_args_builder.go`
- `internal/infrastructure/transcoding/session_manager.go`

**Tasks:**
- [ ] Add method to build audio-only FFmpeg command
- [ ] Support audio track index parameter
- [ ] Transcode to AAC if source isn't web-compatible
- [ ] Manage audio transcode sessions alongside video sessions

### 3.4 Frontend Audio Selector Update

**Files to modify:**
- `web/src/components/media/VideoPlayer/VideoPlayer.tsx`
- `web/src/components/media/VideoPlayer/VideoControls.tsx`
- `web/src/components/media/VideoPlayer/AudioTrackSelector.tsx` (new or enhance existing)

**Tasks:**
- [ ] Update audio track selector to show language names instead of codes
- [ ] Display channel configuration (stereo, 5.1, 7.1)
- [ ] Indicate commentary and audio description tracks with icons
- [ ] Handle HLS.js `audioTracks` from multi-audio manifest
- [ ] Store last selected audio track per media in localStorage

---

## Phase 4: Subtitle Support

**Important Context:** ~69% of embedded subtitles in the library are PGS (bitmap) format which cannot be converted to WebVTT. The implementation must handle this gracefully.

### 4.1 Subtitle Extraction & Conversion

**Files to create:**
- `internal/infrastructure/subtitles/extractor.go`
- `internal/infrastructure/subtitles/converter.go`
- `internal/infrastructure/subtitles/segmenter.go`

**Tasks:**
- [ ] Create subtitle extractor to pull embedded subtitles via FFmpeg
- [ ] Create WebVTT converter for SRT, ASS, SSA formats
- [ ] Implement WebVTT segmenter for HLS delivery
- [ ] Handle character encoding detection and conversion (UTF-8)
- [ ] Preserve timing and styling where possible
- [ ] **Skip PGS/VOBSUB bitmap subtitles** - mark as `convertible=false` in database

### 4.2 Subtitle HLS Playlists

**Files to modify:**
- `internal/api/handlers/transcode.go`
- `internal/api/routes/transcode.go`

**New endpoint:**
- [ ] `GET /api/media/:id/hls/subs/:lang/playlist.m3u8` - Subtitle HLS playlist
- [ ] `GET /api/media/:id/hls/subs/:lang/:segment.vtt` - Individual WebVTT segment

**Tasks:**
- [ ] Create handler for subtitle playlist requests
- [ ] Generate segmented WebVTT files on demand (or cache)
- [ ] Add `EXT-X-MEDIA TYPE=SUBTITLES` to master playlist
- [ ] Set `FORCED=YES` for forced subtitle tracks
- [ ] Set `CHARACTERISTICS="public.accessibility.transcribes-spoken-dialog"` for SDH

### 4.3 Frontend Subtitle Selector

**Files to create/modify:**
- `web/src/components/media/VideoPlayer/SubtitleSelector.tsx` (new)
- `web/src/components/media/VideoPlayer/VideoControls.tsx`
- `web/src/components/media/VideoPlayer/VideoPlayer.tsx`

**Tasks:**
- [ ] Create subtitle track selector component
  - "Off" option at top
  - Language names with SDH/Forced indicators
  - Current selection indicator
- [ ] Integrate HLS.js subtitle track switching
- [ ] Apply WebVTT styling via `::cue` CSS
- [ ] Add subtitle toggle keyboard shortcut (e.g., 'c' for captions)
- [ ] Store last selected subtitle track per media in localStorage

### 4.4 Auto-Selection Logic

**Files to modify:**
- `web/src/components/media/VideoPlayer/VideoPlayer.tsx`
- `web/src/hooks/useTrackSelection.ts` (new)

**Tasks:**
- [ ] Create `useTrackSelection` hook for auto-selection logic
- [ ] Fetch library language preferences
- [ ] Compare content `original_language` with user preference
- [ ] Auto-enable subtitles for foreign content
- [ ] Apply default audio track based on preference
- [ ] Respect user manual selections (override auto)

### 4.5 PGS/Bitmap Subtitle Handling (Future Enhancement)

**Note:** This is a stretch goal. ~69% of subtitles in the library are PGS format. Initial release will show these tracks but mark them as unavailable for soft subtitles.

**Files to modify:**
- `internal/infrastructure/subtitles/converter.go`
- `internal/infrastructure/transcoding/ffmpeg_args_builder.go`

**Tasks:**
- [ ] Detect PGS (Blu-ray) and VOBSUB (DVD) subtitle formats during scan
- [ ] Mark bitmap subtitles as `is_bitmap=true` in database
- [ ] UI shows PGS tracks with "Burn-in only" indicator
- [ ] **Future:** Add burn-in transcoding option (`-filter_complex "[0:v][0:s:0]overlay"`)
- [ ] **Future:** Consider OCR via tesseract for PGS→text conversion (accuracy concerns)

---

## Phase 5: Polish & Testing

### 5.1 Testing

**Files to create:**
- `internal/infrastructure/subtitles/extractor_test.go`
- `internal/infrastructure/subtitles/converter_test.go`
- `internal/api/handlers/transcode_test.go` (extend)
- `web/src/components/media/VideoPlayer/__tests__/`

**Tasks:**
- [ ] Unit tests for subtitle extraction
- [ ] Unit tests for SRT → WebVTT conversion
- [ ] Unit tests for ASS → WebVTT conversion
- [ ] Integration tests for multi-audio playlist generation
- [ ] Integration tests for subtitle playlist generation
- [ ] Frontend component tests for track selectors
- [ ] E2E tests for audio switching during playback
- [ ] E2E tests for subtitle enabling/disabling

### 5.2 Performance Optimization

**Tasks:**
- [ ] Cache extracted/converted subtitles on disk
- [ ] Lazy-load subtitle segments (only on selection)
- [ ] Background job for subtitle pre-processing after scan
- [ ] Monitor A/V sync with separate audio streams
- [ ] Add metrics for track switching latency

### 5.3 Documentation

**Tasks:**
- [ ] Update API documentation
- [ ] Add user guide for language settings
- [ ] Document supported subtitle formats
- [ ] Add troubleshooting guide for sync issues

---

## Implementation Order & Dependencies

```
Phase 1: Database & Scanning
    │
    ├──▶ Phase 2: API & Track Metadata (depends on Phase 1)
    │        │
    │        └──▶ Phase 3: Multi-Audio HLS (depends on Phase 2)
    │                │
    │                └──▶ Phase 4: Subtitle Support (depends on Phase 3)
    │                        │
    │                        └──▶ Phase 5: Polish & Testing
    │
    └──▶ (Phase 2 can start library settings UI in parallel)
```

## Key Technical Decisions

### Audio Strategy: Separate Streams vs. Muxed

**Decision**: Use separate audio streams (demuxed audio)

**Rationale**:
- Bandwidth efficient: Changing audio doesn't re-download video
- HLS.js handles switching seamlessly
- Aligns with Apple HLS best practices

**Trade-off**: More complex manifest generation, potential sync issues

### Subtitle Strategy: Segmented WebVTT

**Decision**: Convert all text-based subtitles to segmented WebVTT

**Rationale**:
- WebVTT is the only universally supported HLS subtitle format
- Segmentation aligns with HLS paradigm
- Browser-native rendering via `::cue`

**Trade-off**: Some styling lost in ASS → WebVTT conversion; PGS cannot be converted

### Storage Strategy: On-Demand with Caching

**Decision**: Generate subtitle segments on first request, cache indefinitely

**Rationale**:
- Avoids upfront processing for rarely-watched content
- Subsequent requests served from cache
- Storage footprint proportional to usage

---

## Risk Mitigation

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| A/V sync drift | Medium | High | Consistent segment duration; extensive testing |
| HLS.js subtitle bugs | Low | Medium | Test across browsers; fallback to HTML5 `<track>` |
| Performance regression in scanner | Medium | Medium | Background extraction; incremental updates |
| Complex UI overwhelming users | Low | Low | Progressive disclosure; sensible defaults |
| PGS subtitles unsupported | High | Low | Clear indication in UI; burn-in option |

---

## Success Criteria

1. Users can select from all available audio tracks during playback
2. Users can enable/disable subtitles with track selection
3. Default audio/subtitle selection respects library preferences
4. Foreign content auto-enables subtitles when configured
5. No perceptible A/V sync issues with separate audio streams
6. Subtitle timing accurate within 100ms of source
7. Scanner extracts track metadata within 5% performance overhead
