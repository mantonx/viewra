# ADR 0004: Multi-Language Audio and Subtitle Support

## Status
**Proposed** - 2025-11-26

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

Analysis of the actual ViewRA media libraries reveals the following characteristics that inform our implementation:

#### Audio Track Distribution

| Codec | Frequency | Notes |
|-------|-----------|-------|
| `ac3` (Dolby Digital) | Most common | Web-compatible with transcoding |
| `dts` (DTS/DTS-HD MA) | Very common | Requires transcoding to AAC |
| `truehd` (Dolby TrueHD/Atmos) | Common in 4K | Requires transcoding; often 7.1 Atmos |
| `flac` | Some releases | Lossless stereo/mono, rare in video |
| `eac3` (E-AC3/DD+) | Streaming releases | Often Atmos-enabled |
| `aac` | Web releases | Direct playback compatible |

**Channel configurations observed:** 8ch (7.1 Atmos), 6ch (5.1), 2ch (stereo), 1ch (mono)

**Multi-audio prevalence:** ~47% of sampled movies have 2+ audio tracks

**Example: Train to Busan (Korean film)**
- Korean TrueHD 7.1 Atmos (default)
- Korean DTS-HD MA 7.1
- Korean DD 5.1 (compatibility)
- English DTS-HD MA 5.1 (dub)
- English DD 5.1 (compatibility)

**Example: Trainspotting (with commentary)**
- English DTS 5.1 "Digital 5.1 Mix" (default)
- English FLAC stereo "Dolby Stereo SR"
- English AC3 stereo "Commentary with director Danny Boyle..." (commentary track)

#### Subtitle Format Distribution

| Format | Frequency | Convertible to WebVTT |
|--------|-----------|----------------------|
| `hdmv_pgs_subtitle` (PGS/Blu-ray) | ~69% of subtitle tracks | **No** - bitmap format, burn-in only |
| `subrip` (SRT) | ~31% of subtitle tracks | **Yes** - text-based |
| `ass`/`ssa` | Rare (anime) | **Yes** - with styling loss |

**Key finding:** PGS bitmap subtitles are the dominant format in Blu-ray remuxes. This is a significant limitation since they cannot be converted to WebVTT and must be burned into the video stream.

**Subtitle language distribution (top 10):**
1. English (eng) - most common
2. Spanish (spa)
3. French (fre)
4. German (ger)
5. Chinese (chi) - both Simplified and Traditional
6. Portuguese (por) - both Brazilian and Iberian variants
7. Dutch (dut)
8. Japanese (jpn)
9. Italian (ita)
10. Swedish (swe)

**Special subtitle types observed:**
- **SDH (hearing impaired):** Indicated by `disposition.hearing_impaired=1` or title containing "SDH"
- **Forced:** Indicated by `disposition.forced=1` or title containing "FORCED" (for foreign dialogue)
- **Commentary:** Separate subtitle track for director commentary (e.g., "English (Commentary)")
- **Regional variants:** "Spanish (Castilian)" vs "Spanish (Latin American)", "Portuguese (Brazilian)" vs "Portuguese (Iberian)"

**Example: A Quiet Place** - 26 subtitle tracks, all PGS format (no text-based options)

**Example: Barbarians (German Netflix show)** - 36 SRT tracks including:
- German and English forced subtitles
- German and English full subtitles
- German and English SDH
- 30+ language translations

#### External Subtitle Files

**This library:**
- **Movies:** ~202 external `.srt` files found
- **TV episodes:** ~2,805 external `.srt` files found
- Pattern: `{filename}.en.srt`, `{filename}.en.hi.srt`

**Industry-standard patterns (Plex/Jellyfin compatibility):**

Based on [Plex subtitle documentation](https://support.plex.tv/articles/200471133-adding-local-subtitles-to-your-media/) and [Jellyfin media organization](https://jellyfin.org/docs/general/server/media/movies/), we should support:

| Pattern | Example | Meaning |
|---------|---------|---------|
| `{name}.{lang}.srt` | `Movie.en.srt` | English subtitles |
| `{name}.{lang}.forced.srt` | `Movie.en.forced.srt` | Forced (foreign dialogue) |
| `{name}.{lang}.sdh.srt` | `Movie.en.sdh.srt` | SDH (captions for deaf/HoH) |
| `{name}.{lang}.hi.srt` | `Movie.en.hi.srt` | Hearing impaired (Jellyfin) |
| `{name}.{lang}.cc.srt` | `Movie.en.cc.srt` | Closed captions |
| `{name}.default.{lang}.srt` | `Movie.default.en.srt` | Default subtitle track |
| `{name}.srt` | `Movie.srt` | No language (assume library default) |

**Language codes:** Both ISO 639-1 (2-letter: `en`, `es`, `ja`) and ISO 639-2/B (3-letter: `eng`, `spa`, `jpn`) should be recognized.

**Subtitle directories (Plex convention):**
- `subs/` or `subtitles/` subdirectory in same folder as media
- `Subs/` or `Subtitles/` (case variations)

**Note:** Our library doesn't use subtitle directories, but we should support them for other users.

#### Implications for Implementation

1. **PGS subtitle handling is critical:** Most Blu-ray content only has PGS subtitles. Options:
   - Mark as "burn-in only" and transcode with hardcoded subtitles
   - Use OCR (tesseract) to convert to text - accuracy concerns
   - Rely on external SRT files when available

2. **Audio codec transcoding required:** Most audio (DTS, TrueHD) needs transcoding to AAC for web playback

3. **Commentary track detection:** Parse title field for "commentary" or "director" keywords

4. **Regional language variants:** UI should display full title (e.g., "Spanish (Latin American)") not just language code

5. **External subtitle discovery is valuable:** 3000+ external SRT files provide text-based alternatives to PGS

### Research Summary

Based on industry best practices ([Apple HLS](https://www.radiantmediaplayer.com/blog/an-update-to-apple-hls-best-practices-end-2024.html), [Mux](https://www.mux.com/blog/subtitles-captions-webvtt-hls-and-those-magic-flags), [Martin Riedl's FFmpeg guide](https://www.martin-riedl.de/2020/05/31/using-ffmpeg-as-a-hls-streaming-server-part-9-multiple-audio-languages/)):

- **Separate audio streams**: Audio should be muxed separately from video for bandwidth efficiency
- **WebVTT segmentation**: Subtitles should be segmented to align with HLS segment boundaries
- **EXT-X-MEDIA groups**: Master playlist should declare audio/subtitle renditions with proper language tags
- **Forced subtitles**: Foreign dialogue translations should use `FORCED=YES` flag

## Decision

We will implement a comprehensive multi-language audio and subtitle system with:

1. **Database persistence** for audio tracks and subtitle tracks metadata
2. **HLS multi-rendition support** with separate audio/subtitle playlists
3. **Library-level language preferences** for default audio and subtitle selection
4. **External subtitle file discovery** and automatic WebVTT conversion
5. **Smart auto-selection** based on content language vs. user preference

### Architecture Overview

```
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
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Transcode Service (Extended)                                        │   │
│  │                                                                      │   │
│  │  • Encode separate audio-only HLS playlists per language            │   │
│  │  • Generate segmented WebVTT playlists per subtitle track           │   │
│  │  • Support selecting specific audio track for transcoding           │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Frontend (VideoPlayer)                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────┐    ┌─────────────────────────┐               │
│  │  Audio Track Selector   │    │  Subtitle Track Selector │               │
│  │                         │    │                          │               │
│  │  • Display language     │    │  • Display language      │               │
│  │    names (not codes)    │    │    names                 │               │
│  │  • Show channel info    │    │  • "Off" option          │               │
│  │  • Indicate commentary  │    │  • Indicate SDH/Forced   │               │
│  │  • Current selection    │    │  • Current selection     │               │
│  └─────────────────────────┘    └─────────────────────────┘               │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Auto-Selection Logic                                                │   │
│  │                                                                      │   │
│  │  1. Get library preferred_audio_lang                                │   │
│  │  2. Select audio track matching preference (or first available)     │   │
│  │  3. If content original_language != preferred_audio_lang:           │   │
│  │     → Auto-enable subtitles in preferred_subtitle_lang             │   │
│  │  4. Else: subtitles off by default                                  │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Core Components

#### 1. Database Schema Extensions

**New table: `media_audio_tracks`**
```sql
CREATE TABLE media_audio_tracks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    stream_index INTEGER NOT NULL,      -- FFmpeg stream index
    codec TEXT NOT NULL,                -- aac, ac3, dts, truehd, etc.
    codec_profile TEXT,                 -- e.g., "LC" for AAC-LC
    channels INTEGER NOT NULL,          -- 1, 2, 6, 8
    channel_layout TEXT,                -- stereo, 5.1, 7.1
    sample_rate INTEGER,                -- 44100, 48000
    bit_rate INTEGER,                   -- bits per second
    language TEXT,                      -- ISO 639-2 code (eng, spa, jpn)
    title TEXT,                         -- Track title from metadata
    is_default BOOLEAN DEFAULT FALSE,   -- Marked as default in source
    is_commentary BOOLEAN DEFAULT FALSE,
    is_descriptive BOOLEAN DEFAULT FALSE, -- Audio description for visually impaired
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(media_id, stream_index)
);
```

**New table: `media_subtitle_tracks`**
```sql
CREATE TABLE media_subtitle_tracks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    stream_index INTEGER,               -- FFmpeg stream index (NULL for external)
    source_type TEXT NOT NULL CHECK(source_type IN ('embedded', 'external')),
    codec TEXT,                         -- subrip, ass, webvtt, hdmv_pgs_subtitle
    language TEXT,                      -- ISO 639-2 code
    title TEXT,                         -- Track title
    file_path TEXT,                     -- For external subtitles
    is_default BOOLEAN DEFAULT FALSE,
    is_forced BOOLEAN DEFAULT FALSE,    -- Forced subtitles (foreign dialogue)
    is_sdh BOOLEAN DEFAULT FALSE,       -- Subtitles for deaf/hard of hearing
    is_commentary BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(media_id, stream_index) -- For embedded
);
```

**Alter table: `libraries`**
```sql
ALTER TABLE libraries ADD COLUMN preferred_audio_lang TEXT DEFAULT 'eng';
ALTER TABLE libraries ADD COLUMN preferred_subtitle_lang TEXT DEFAULT 'eng';
ALTER TABLE libraries ADD COLUMN auto_enable_subtitles TEXT DEFAULT 'foreign_only'
    CHECK(auto_enable_subtitles IN ('always', 'foreign_only', 'never'));
```

#### 2. Scanner Extensions

Extend the existing media scanner to extract and persist audio/subtitle track metadata:

```go
// internal/domain/scanner/track_metadata.go
type AudioTrackMetadata struct {
    StreamIndex    int
    Codec          string
    CodecProfile   string
    Channels       int
    ChannelLayout  string
    SampleRate     int
    BitRate        int64
    Language       string
    Title          string
    IsDefault      bool
    IsCommentary   bool
    IsDescriptive  bool
}

type SubtitleTrackMetadata struct {
    StreamIndex  int     // -1 for external
    SourceType   string  // "embedded" or "external"
    Codec        string
    Language     string
    Title        string
    FilePath     string  // For external files
    IsDefault    bool
    IsForced     bool
    IsSDH        bool
    IsCommentary bool
}
```

**External subtitle discovery patterns:**
- Same directory as media file
- Subdirectory named `Subs`, `Subtitles`, or `subs`
- Filename patterns: `{media_name}.{lang}.srt`, `{media_name}.srt`, `{media_name}.{lang}.forced.srt`

#### 3. HLS Multi-Rendition Support

**Extended master playlist format:**

```m3u8
#EXTM3U
#EXT-X-VERSION:4
#EXT-X-INDEPENDENT-SEGMENTS

# Audio renditions
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="English",LANGUAGE="en",DEFAULT=YES,AUTOSELECT=YES,URI="audio/en/playlist.m3u8"
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="Japanese",LANGUAGE="ja",DEFAULT=NO,AUTOSELECT=YES,URI="audio/ja/playlist.m3u8"
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="Commentary",LANGUAGE="en",DEFAULT=NO,AUTOSELECT=NO,CHARACTERISTICS="public.accessibility.describes-video",URI="audio/en-commentary/playlist.m3u8"

# Subtitle renditions
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="English",LANGUAGE="en",DEFAULT=NO,AUTOSELECT=YES,URI="subs/en/playlist.m3u8"
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="English (SDH)",LANGUAGE="en",DEFAULT=NO,AUTOSELECT=NO,CHARACTERISTICS="public.accessibility.transcribes-spoken-dialog",URI="subs/en-sdh/playlist.m3u8"
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="Spanish",LANGUAGE="es",DEFAULT=NO,AUTOSELECT=YES,URI="subs/es/playlist.m3u8"
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="Japanese (Forced)",LANGUAGE="ja",DEFAULT=NO,AUTOSELECT=NO,FORCED=YES,URI="subs/ja-forced/playlist.m3u8"

# Video streams (reference audio and subtitle groups)
#EXT-X-STREAM-INF:BANDWIDTH=800000,RESOLUTION=640x360,CODECS="avc1.4d401e,mp4a.40.2",AUDIO="audio",SUBTITLES="subs"
360p/playlist.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=4000000,RESOLUTION=1280x720,CODECS="avc1.64001f,mp4a.40.2",AUDIO="audio",SUBTITLES="subs"
720p/playlist.m3u8

#EXT-X-STREAM-INF:BANDWIDTH=8000000,RESOLUTION=1920x1080,CODECS="avc1.640028,mp4a.40.2",AUDIO="audio",SUBTITLES="subs"
1080p/playlist.m3u8
```

#### 4. Subtitle Processing Pipeline

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Source     │     │   Convert    │     │   Segment    │     │   Serve      │
│              │ ──▶ │              │ ──▶ │              │ ──▶ │              │
│  .srt/.ass/  │     │  to WebVTT   │     │  30s chunks  │     │  via HLS     │
│  embedded    │     │  (if needed) │     │  + playlist  │     │  /subs/{lang}│
└──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘
```

**FFmpeg commands for subtitle extraction/conversion:**

```bash
# Extract embedded subtitles to WebVTT
ffmpeg -i input.mkv -map 0:s:0 -c:s webvtt output.vtt

# Convert SRT to WebVTT
ffmpeg -i input.srt output.vtt

# Segment WebVTT for HLS (using segment muxer)
ffmpeg -i input.vtt -f segment -segment_time 30 -segment_list subs_en.m3u8 subs_en_%04d.vtt
```

#### 5. Frontend Track Selectors

**Audio selector enhancements:**
- Display language name (e.g., "English" not "eng")
- Show channel configuration (e.g., "5.1 Surround")
- Indicate special tracks: 🎙️ Commentary, 👁️ Audio Description
- Current selection indicator

**New subtitle selector:**
- "Off" as first option
- Display language names
- Indicate special types: 👂 SDH, 🔤 Forced
- Remember last selection per media item (localStorage)

#### 6. Auto-Selection Logic

```typescript
// Pseudo-code for track auto-selection
function selectDefaultTracks(media: Media, library: Library, audioTracks: AudioTrack[], subtitleTracks: SubtitleTrack[]) {
  // Audio selection
  const preferredAudio = audioTracks.find(t => t.language === library.preferredAudioLang && !t.isCommentary);
  const defaultAudio = preferredAudio || audioTracks.find(t => t.isDefault) || audioTracks[0];

  // Subtitle selection
  let defaultSubtitle: SubtitleTrack | null = null;

  if (library.autoEnableSubtitles === 'always') {
    defaultSubtitle = subtitleTracks.find(t => t.language === library.preferredSubtitleLang);
  } else if (library.autoEnableSubtitles === 'foreign_only') {
    const contentLang = media.originalLanguage || defaultAudio?.language;
    const isForeignContent = contentLang !== library.preferredAudioLang;

    if (isForeignContent) {
      defaultSubtitle = subtitleTracks.find(t => t.language === library.preferredSubtitleLang);
    }
  }
  // 'never' → defaultSubtitle stays null

  return { defaultAudio, defaultSubtitle };
}
```

### API Changes

**New endpoints:**

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/media/:id/tracks` | GET | Get audio and subtitle tracks for a media item |
| `/api/libraries/:id` | PATCH | Update library preferences including language settings |
| `/api/media/:id/hls/audio/:lang/playlist.m3u8` | GET | Audio-only HLS playlist for a language |
| `/api/media/:id/hls/subs/:lang/playlist.m3u8` | GET | Subtitle HLS playlist for a language |

**Extended `/api/media/:id` response:**
```json
{
  "id": 123,
  "title": "Example Movie",
  "original_language": "ja",
  "audio_tracks": [
    { "id": 1, "language": "ja", "title": "Japanese", "channels": 6, "channel_layout": "5.1", "is_default": true },
    { "id": 2, "language": "en", "title": "English Dub", "channels": 2, "channel_layout": "stereo" }
  ],
  "subtitle_tracks": [
    { "id": 1, "language": "en", "title": "English", "is_sdh": false, "is_forced": false },
    { "id": 2, "language": "en", "title": "English (SDH)", "is_sdh": true },
    { "id": 3, "language": "ja", "title": "Signs/Songs", "is_forced": true }
  ]
}
```

### Progressive Implementation Strategy

Given the complexity, we'll implement in phases:

**Phase 1: Database & Scanning (Foundation)**
- Add database tables for audio/subtitle tracks
- Extend scanner to extract and persist track metadata
- Add external subtitle file discovery

**Phase 2: API & Track Metadata (Visibility)**
- Create API endpoints for track metadata
- Return track information in media details response
- Add library language preference settings

**Phase 3: Multi-Audio HLS (Audio Switching)**
- Generate multi-audio master playlists
- Create separate audio-only playlists per language
- Update frontend audio selector

**Phase 4: Subtitle Support (Full Feature)**
- Implement subtitle extraction/conversion to WebVTT
- Generate segmented subtitle playlists
- Add frontend subtitle selector with styling
- Implement auto-selection logic

## Consequences

### Positive

- Users can enjoy content in their preferred language
- Accessibility improved with SDH and audio description support
- Reduced bandwidth when only audio track changes (not full stream)
- Aligns with industry standards (HLS multi-rendition)
- Better media library metadata for sorting/filtering

### Negative

- Increased complexity in transcoding pipeline
- More storage for pre-segmented audio/subtitle files
- Longer initial scan times to extract all track metadata
- Potential sync issues between separate audio/video segments
- PGS/image-based subtitle formats cannot be converted to WebVTT (must be burned in)

### Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| A/V sync drift with separate streams | Use consistent segment durations; test extensively |
| Subtitle timing issues after conversion | Preserve original timing; allow manual offset adjustment |
| PGS subtitles incompatible with WebVTT | Detect and mark as "burn-in only"; offer as hardcoded option |
| Large number of tracks overwhelming UI | Collapse to common languages; "More..." expansion |
| Performance impact of track extraction | Background job after initial scan; cache metadata |

## References

- [Apple HLS Authoring Specification](https://developer.apple.com/documentation/http_live_streaming/hls_authoring_specification_for_apple_devices)
- [HLS Best Practices (2024)](https://www.radiantmediaplayer.com/blog/an-update-to-apple-hls-best-practices-end-2024.html)
- [WebVTT Subtitles in HLS](https://www.vidbeo.com/blog/hls-subtitles-captions-webvtt/)
- [Subtitles, Captions, WebVTT, HLS and those magic flags - Mux](https://www.mux.com/blog/subtitles-captions-webvtt-hls-and-those-magic-flags)
- [FFmpeg HLS Multiple Audio Languages](https://www.martin-riedl.de/2020/05/31/using-ffmpeg-as-a-hls-streaming-server-part-9-multiple-audio-languages/)
- [HLS.js Documentation](https://github.com/video-dev/hls.js)
- [WebVTT API - MDN](https://developer.mozilla.org/en-US/docs/Web/API/WebVTT_API)
