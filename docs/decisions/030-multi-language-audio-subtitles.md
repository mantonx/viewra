# ADR 030: Multi-Language Audio and Subtitle Support

## Status

Proposed

## Date

November 26, 2025 (recovered December 2, 2025)

## Context

### Current State

ViewRA's video player currently has basic HLS streaming with:

- **Single audio track selection**: HLS.js extracts audio tracks from manifests, but the backend only selects and transcodes a single "best" audio track based on codec compatibility and channel count
- **No subtitle support**: No infrastructure for subtitle detection, storage, conversion, or playback
- **No language preferences**: Users cannot set preferred audio/subtitle languages at the library or account level
- **Runtime-only metadata**: Audio track information is extracted via ffprobe at playback time, not persisted in the database

### User Requirements

1. **Multi-language audio**: Users want to:
   - See all available audio tracks (different languages, commentary tracks)
   - Switch between audio tracks during playback
   - Have their preferred language auto-selected

2. **Subtitle support**: Users want to:
   - View subtitles in their preferred language
   - Have subtitles disabled by default for native-language content
   - Have subtitles auto-enabled for foreign-language content
   - Select from embedded subtitles or external subtitle files (.srt, .vtt, .ass)

### Technical Constraints

1. **HLS Protocol**: HLS supports multiple audio and subtitle tracks via `EXT-X-MEDIA` tags with `GROUP-ID` grouping
2. **Browser Compatibility**: WebVTT is the universal subtitle format for HLS; other formats require conversion
3. **FFmpeg Capabilities**: FFmpeg can extract/convert subtitles and encode multiple audio tracks
4. **HLS.js Support**: HLS.js has built-in support for audio track switching and WebVTT subtitle rendering

### Real Library Analysis

Analysis of actual ViewRA media libraries reveals:

#### Audio Track Distribution

| Codec | Frequency | Notes |
|-------|-----------|-------|
| `ac3` (Dolby Digital) | Most common | Web-compatible with transcoding |
| `dts` (DTS/DTS-HD MA) | Very common | Requires transcoding to AAC |
| `truehd` (Dolby TrueHD/Atmos) | Common in 4K | Requires transcoding; often 7.1 Atmos |
| `flac` | Some releases | Lossless stereo/mono, rare in video |
| `eac3` (E-AC3/DD+) | Streaming releases | Often Atmos-enabled |
| `aac` | Web releases | Direct playback compatible |

**Channel configurations:** 8ch (7.1 Atmos), 6ch (5.1), 2ch (stereo), 1ch (mono)

**Multi-audio prevalence:** ~47% of sampled movies have 2+ audio tracks

#### Subtitle Format Distribution

| Format | Frequency | Convertible to WebVTT |
|--------|-----------|----------------------|
| `hdmv_pgs_subtitle` (PGS/Blu-ray) | ~69% | **No** - bitmap format, burn-in only |
| `subrip` (SRT) | ~31% | **Yes** - text-based |
| `ass`/`ssa` | Rare (anime) | **Yes** - with styling loss |

**Key finding:** PGS bitmap subtitles are dominant in Blu-ray remuxes. They cannot be converted to WebVTT and must be burned into the video stream.

#### External Subtitle Files

- **Movies:** ~202 external `.srt` files found
- **TV episodes:** ~2,805 external `.srt` files found
- Pattern: `{filename}.en.srt`, `{filename}.en.hi.srt`

**Industry-standard patterns:**

| Pattern | Example | Meaning |
|---------|---------|---------|
| `{name}.{lang}.srt` | `Movie.en.srt` | English subtitles |
| `{name}.{lang}.forced.srt` | `Movie.en.forced.srt` | Forced (foreign dialogue) |
| `{name}.{lang}.sdh.srt` | `Movie.en.sdh.srt` | SDH (captions for deaf/HoH) |
| `{name}.{lang}.hi.srt` | `Movie.en.hi.srt` | Hearing impaired |
| `{name}.{lang}.cc.srt` | `Movie.en.cc.srt` | Closed captions |

## Decision

Implement a comprehensive multi-language audio and subtitle system with:

1. **Database persistence** for audio tracks and subtitle tracks metadata
2. **HLS multi-rendition support** with separate audio/subtitle playlists
3. **Library-level language preferences** for default audio and subtitle selection
4. **External subtitle file discovery** and automatic WebVTT conversion
5. **Smart auto-selection** based on content language vs. user preference

### Architecture Overview

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Database Layer                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────┐    ┌─────────────────────┐    ┌─────────────────┐ │
│  │  media_audio_tracks │    │ media_subtitle_tracks│    │   libraries     │ │
│  │                     │    │                      │    │                 │ │
│  │  • media_id         │    │  • media_id          │    │  + preferred_   │ │
│  │  • stream_index     │    │  • stream_index      │    │    audio_lang   │ │
│  │  • codec            │    │  • type (embedded/   │    │  + preferred_   │ │
│  │  • language         │    │         external)    │    │    subtitle_    │ │
│  │  • title            │    │  • codec             │    │    lang         │ │
│  │  • channels         │    │  • language          │    │                 │ │
│  │  • is_default       │    │  • file_path         │    │                 │ │
│  │  • is_commentary    │    │  • is_forced         │    │                 │ │
│  └─────────────────────┘    │  • is_default        │    └─────────────────┘ │
│                             │  • is_sdh            │                        │
│                             └─────────────────────┘                         │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Backend Services                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────┐         ┌─────────────────────────┐           │
│  │  Scanner Extension      │         │  Subtitle Service       │           │
│  │                         │         │                         │           │
│  │  • Extract audio tracks │         │  • Discover .srt/.vtt/  │           │
│  │  • Extract embedded     │         │    .ass files           │           │
│  │    subtitle tracks      │         │  • Convert to WebVTT    │           │
│  │  • Persist metadata     │         │  • Segment for HLS      │           │
│  └─────────────────────────┘         └─────────────────────────┘           │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  HLS Master Playlist Generator                                       │   │
│  │                                                                      │   │
│  │  • Generate EXT-X-MEDIA tags for each audio language                │   │
│  │  • Generate EXT-X-MEDIA tags for each subtitle track                │   │
│  │  • Reference AUDIO and SUBTITLES groups in EXT-X-STREAM-INF         │   │
│  │  • Set DEFAULT/AUTOSELECT based on user preferences                 │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Database Schema

**New table: `media_audio_tracks`**

```sql
CREATE TABLE media_audio_tracks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    stream_index INTEGER NOT NULL,
    codec TEXT NOT NULL,
    codec_profile TEXT,
    channels INTEGER NOT NULL,
    channel_layout TEXT,
    sample_rate INTEGER,
    bit_rate INTEGER,
    language TEXT,
    title TEXT,
    is_default BOOLEAN DEFAULT FALSE,
    is_commentary BOOLEAN DEFAULT FALSE,
    is_descriptive BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(media_id, stream_index)
);
```

**New table: `media_subtitle_tracks`**

```sql
CREATE TABLE media_subtitle_tracks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    stream_index INTEGER,
    source_type TEXT NOT NULL CHECK(source_type IN ('embedded', 'external')),
    codec TEXT,
    language TEXT,
    title TEXT,
    file_path TEXT,
    is_default BOOLEAN DEFAULT FALSE,
    is_forced BOOLEAN DEFAULT FALSE,
    is_sdh BOOLEAN DEFAULT FALSE,
    is_commentary BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(media_id, stream_index)
);
```

**Library extensions:**

```sql
ALTER TABLE libraries ADD COLUMN preferred_audio_lang TEXT DEFAULT 'eng';
ALTER TABLE libraries ADD COLUMN preferred_subtitle_lang TEXT DEFAULT 'eng';
ALTER TABLE libraries ADD COLUMN auto_enable_subtitles TEXT DEFAULT 'foreign_only'
    CHECK(auto_enable_subtitles IN ('always', 'foreign_only', 'never'));
```

### HLS Multi-Rendition Format

```m3u8
#EXTM3U
#EXT-X-VERSION:4
#EXT-X-INDEPENDENT-SEGMENTS

# Audio renditions
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="English",LANGUAGE="en",DEFAULT=YES,URI="audio/en/playlist.m3u8"
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="Japanese",LANGUAGE="ja",DEFAULT=NO,URI="audio/ja/playlist.m3u8"
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="Commentary",LANGUAGE="en",DEFAULT=NO,URI="audio/en-commentary/playlist.m3u8"

# Subtitle renditions
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="English",LANGUAGE="en",URI="subs/en/playlist.m3u8"
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="English (SDH)",LANGUAGE="en",URI="subs/en-sdh/playlist.m3u8"
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="Japanese (Forced)",LANGUAGE="ja",FORCED=YES,URI="subs/ja-forced/playlist.m3u8"

# Video streams
#EXT-X-STREAM-INF:BANDWIDTH=4000000,RESOLUTION=1280x720,AUDIO="audio",SUBTITLES="subs"
720p/playlist.m3u8
```

### Subtitle Processing Pipeline

```text
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Source     │     │   Convert    │     │   Segment    │     │   Serve      │
│              │ ──► │              │ ──► │              │ ──► │              │
│  .srt/.ass/  │     │  to WebVTT   │     │  30s chunks  │     │  via HLS     │
│  embedded    │     │  (if needed) │     │  + playlist  │     │  /subs/{lang}│
└──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘
```

### Auto-Selection Logic

```typescript
function selectDefaultTracks(media, library, audioTracks, subtitleTracks) {
  // Audio: prefer user's language, fall back to default or first
  const preferredAudio = audioTracks.find(t =>
    t.language === library.preferredAudioLang && !t.isCommentary
  );
  const defaultAudio = preferredAudio || audioTracks.find(t => t.isDefault) || audioTracks[0];

  // Subtitles: based on auto_enable_subtitles setting
  let defaultSubtitle = null;

  if (library.autoEnableSubtitles === 'always') {
    defaultSubtitle = subtitleTracks.find(t => t.language === library.preferredSubtitleLang);
  } else if (library.autoEnableSubtitles === 'foreign_only') {
    const contentLang = media.originalLanguage || defaultAudio?.language;
    if (contentLang !== library.preferredAudioLang) {
      defaultSubtitle = subtitleTracks.find(t => t.language === library.preferredSubtitleLang);
    }
  }

  return { defaultAudio, defaultSubtitle };
}
```

### API Changes

**New endpoints:**

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/media/:id/tracks` | GET | Get audio and subtitle tracks for a media item |
| `/api/media/:id/hls/audio/:lang/playlist.m3u8` | GET | Audio-only HLS playlist |
| `/api/media/:id/hls/subs/:lang/playlist.m3u8` | GET | Subtitle HLS playlist |

**Extended media response:**

```json
{
  "id": 123,
  "title": "Example Movie",
  "original_language": "ja",
  "audio_tracks": [
    { "id": 1, "language": "ja", "title": "Japanese", "channels": 6, "is_default": true },
    { "id": 2, "language": "en", "title": "English Dub", "channels": 2 }
  ],
  "subtitle_tracks": [
    { "id": 1, "language": "en", "title": "English", "is_sdh": false },
    { "id": 2, "language": "en", "title": "English (SDH)", "is_sdh": true },
    { "id": 3, "language": "ja", "title": "Signs/Songs", "is_forced": true }
  ]
}
```

## Consequences

### Positive

- Users can enjoy content in their preferred language
- Accessibility improved with SDH and audio description support
- Reduced bandwidth when only audio track changes
- Aligns with industry standards (HLS multi-rendition)
- Better media library metadata for sorting/filtering

### Negative

- Increased complexity in transcoding pipeline
- More storage for pre-segmented audio/subtitle files
- Longer initial scan times to extract all track metadata
- PGS/image-based subtitles cannot be converted to WebVTT (must be burned in)

### Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| A/V sync drift | Use consistent segment durations; test extensively |
| Subtitle timing issues | Preserve original timing; allow manual offset |
| PGS incompatible | Detect and mark as "burn-in only" |
| Too many tracks | Collapse to common languages; "More..." expansion |

## Implementation Phases

### Phase 1: Database & Scanning (2-3 days)

1. Add database tables for audio/subtitle tracks
2. Extend scanner to extract and persist track metadata
3. Add external subtitle file discovery

### Phase 2: API & Track Metadata (1-2 days)

1. Create API endpoints for track metadata
2. Return track information in media details response
3. Add library language preference settings

### Phase 3: Multi-Audio HLS (2-3 days)

1. Generate multi-audio master playlists
2. Create separate audio-only playlists per language
3. Update frontend audio selector

### Phase 4: Subtitle Support (3-4 days)

1. Implement subtitle extraction/conversion to WebVTT
2. Generate segmented subtitle playlists
3. Add frontend subtitle selector with styling
4. Implement auto-selection logic

**Total Effort**: 8-12 days

## References

- [Apple HLS Authoring Specification](https://developer.apple.com/documentation/http_live_streaming/hls_authoring_specification_for_apple_devices)
- [HLS Best Practices (2024)](https://www.radiantmediaplayer.com/blog/an-update-to-apple-hls-best-practices-end-2024.html)
- [Subtitles, Captions, WebVTT, HLS - Mux](https://www.mux.com/blog/subtitles-captions-webvtt-hls-and-those-magic-flags)
- [FFmpeg HLS Multiple Audio Languages](https://www.martin-riedl.de/2020/05/31/using-ffmpeg-as-a-hls-streaming-server-part-9-multiple-audio-languages/)
- [HLS.js Documentation](https://github.com/video-dev/hls.js)
