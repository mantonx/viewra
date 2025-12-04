# Subtitle Pipeline Architecture

This document describes ViewRA's subtitle extraction and streaming pipeline, which uses the custom `subtitle-extractor` tool instead of FFmpeg for subtitle processing.

## Overview

ViewRA supports two categories of subtitles:

| Category | Formats | Extraction Method | Delivery |
|----------|---------|-------------------|----------|
| **Text** | SRT, ASS/SSA, WebVTT, tx3g (mov_text) | subtitle-extractor → WebVTT | Native browser `<track>` or overlay |
| **Bitmap** | PGS (Blu-ray), VobSub (DVD) | subtitle-extractor render-pgs → PNG | Image overlay (TODO) or FFmpeg burn-in |

## Components

### 1. subtitle-extractor (Rust CLI)

Location: `tools/subtitle-extractor/`

A fast, purpose-built tool for subtitle extraction that supports:

- **Containers**: MKV (Matroska), MP4/M4V/MOV, MPEG-TS/M2TS
- **Text codecs**: SRT, ASS/SSA, WebVTT, tx3g
- **Bitmap codecs**: PGS (hdmv_pgs_subtitle)

#### Commands

```bash
# List all tracks in a media file
subtitle-extractor tracks <FILE>

# Extract subtitle track (full file)
subtitle-extractor extract --track <N> --format <raw|srt|webvtt> <FILE>

# Stream subtitle frames with time bounds (uses seeking when available)
subtitle-extractor stream --track <N> --start <ms> --end <ms> --format <jsonl|raw|webvtt> <FILE>

# Build cluster index for a file (bypasses broken Cues)
subtitle-extractor index --track <N> <FILE>

# Render PGS subtitles to PNG files
subtitle-extractor render-pgs --track <N> --output <DIR> --start <ms> --end <ms> --limit <N> <FILE>
```

#### Track Numbering

- subtitle-extractor uses **1-based** MKV track numbers
- FFmpeg uses **0-based** stream indices
- Conversion: `track_number = stream_index + 1`

### 2. Go Converter

Location: `internal/infrastructure/subtitles/converter.go`

The Go layer provides:

- **Caching**: WebVTT files are cached by path hash
- **Format conversion**: Native Go SRT→WebVTT and ASS→WebVTT for external files
- **Embedded extraction**: Calls subtitle-extractor for embedded tracks

```go
type Converter struct {
    subtitleExtractorPath string
    cacheDir              string
}

// Extract embedded subtitle and convert to WebVTT
func (c *Converter) ExtractAndConvert(ctx context.Context, mediaPath string, streamIndex int) (string, error)

// Convert external SRT file to WebVTT
func (c *Converter) ConvertSRTToWebVTT(ctx context.Context, srtPath string) (string, error)

// Convert external ASS/SSA file to WebVTT
func (c *Converter) ConvertASSToWebVTT(ctx context.Context, assPath string) (string, error)

// Auto-detect format and convert
func (c *Converter) ConvertExternalSubtitle(ctx context.Context, subtitlePath string) (string, error)
```

### 3. API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /api/media/:id/subtitles/:trackId` | Get subtitle by database track ID |
| `GET /api/media/:id/subtitles/stream/:index` | Get embedded subtitle by absolute stream index |
| `GET /api/media/:id/subtitles/text/:index/stream` | Get text subtitle by relative index (among text tracks) |

### 4. Frontend Components

- **SubtitleOverlay.tsx**: Renders text subtitles as positioned overlay
- **useSubtitles.ts**: Manages subtitle track selection and preferences
- **SubtitleSelector.tsx**: UI for selecting subtitle tracks

## Current Implementation Status

### ✅ Text Subtitles (Complete)

```
┌─────────────────┐     ┌───────────────────┐     ┌─────────────┐     ┌─────────┐
│ Media File      │────▶│ subtitle-extractor│────▶│ WebVTT Cache│────▶│ Browser │
│ (MKV/MP4/TS)    │     │ stream --format   │     │ (disk)      │     │ Overlay │
│                 │     │ webvtt            │     │             │     │         │
└─────────────────┘     └───────────────────┘     └─────────────┘     └─────────┘

┌─────────────────┐     ┌───────────────────┐     ┌─────────────┐     ┌─────────┐
│ External SRT/   │────▶│ Go Native         │────▶│ WebVTT Cache│────▶│ Browser │
│ ASS File        │     │ Conversion        │     │ (disk)      │     │ Overlay │
└─────────────────┘     └───────────────────┘     └─────────────┘     └─────────┘
```

**Supported formats:**
- Embedded: SRT, ASS/SSA, WebVTT (in MKV), tx3g (in MP4)
- External: .srt, .ass, .ssa, .vtt files

### 🚧 Bitmap Subtitles (In Progress)

**Current state:** Uses FFmpeg burn-in during HLS transcode

```
┌─────────────────┐     ┌───────────────────┐     ┌─────────────┐
│ Media File      │────▶│ FFmpeg transcode  │────▶│ HLS Stream  │
│ + PGS subtitle  │     │ with overlay      │     │ (burned in) │
└─────────────────┘     └───────────────────┘     └─────────────┘
```

**Target state:** Use subtitle-extractor render-pgs with client-side overlay

```
┌─────────────────┐     ┌───────────────────┐     ┌─────────────┐     ┌─────────┐
│ Media File      │────▶│ subtitle-extractor│────▶│ PNG Cache + │────▶│ Browser │
│ (MKV with PGS)  │     │ render-pgs        │     │ manifest    │     │ Overlay │
└─────────────────┘     └───────────────────┘     └─────────────┘     └─────────┘
```

## TODO: Complete PGS Pipeline

### Phase 1: Backend Endpoint

Add `GET /api/media/:id/subtitles/pgs/:streamIndex/render`

```go
// Response format
type PGSManifest struct {
    VideoWidth  int         `json:"video_width"`
    VideoHeight int         `json:"video_height"`
    Frames      []PGSFrame  `json:"frames"`
}

type PGSFrame struct {
    Index    int    `json:"index"`
    StartMS  int64  `json:"start_ms"`
    EndMS    int64  `json:"end_ms"`
    Width    int    `json:"width"`
    Height   int    `json:"height"`
    X        int    `json:"x"`
    Y        int    `json:"y"`
    ImageURL string `json:"image_url"`  // Relative URL to PNG
}
```

Implementation:
1. Check cache for existing render
2. Call `subtitle-extractor render-pgs --track N --output <cache_dir> <file>`
3. Read `metadata.json` from output directory
4. Return manifest with image URLs

### Phase 2: Frontend Image Overlay

Modify `SubtitleOverlay.tsx`:

```tsx
// For bitmap subtitles, fetch manifest instead of WebVTT
if (track.isBitmap) {
    const manifest = await fetchPGSManifest(mediaId, streamIndex)
    // Render <img> elements at correct times
}
```

### Phase 3: Remove FFmpeg Burn-in

1. Delete from `ffmpeg_args_builder.go`:
   - `needsSubtitleBurnIn()` function
   - All `filter_complex` subtitle overlay logic
   - `SubtitleStreamIndex` from `TranscodeOptions`

2. Update `useSubtitles.ts`:
   - Remove `requiresBurnIn: true` logic
   - Treat bitmap subtitles same as text (overlay-based)

## Performance Considerations

### Current Issue: Sequential File Reading

When extracting full subtitles (`stream` with no time bounds), subtitle-extractor reads sequentially through the entire file. This is slow on:
- Large files (50+ GB remuxes)
- Network shares (NFS, SMB)

### Solution: Cluster Index Seeking

The `ClusterCache` module (`containers/cluster_cache.rs`) provides:
- Binary search to find clusters by timestamp
- ~5-8 seeks to locate any point in the file
- Works even when MKV Cues element is broken/missing

**Current usage:**
- `stream` command with time bounds uses seeking
- Full extraction (no bounds) still reads sequentially

**Potential improvement:**
- Add `--full-extract` flag that builds index first, then extracts
- Background indexing during library scan

### Network File Performance

For files on network shares:

| Operation | Local SSD | NFS/SMB |
|-----------|-----------|---------|
| Track listing | <100ms | <500ms |
| Seek + 10s extract | <200ms | <1s |
| Full sequential read (50GB) | ~30s | 5-10 min |

## File Locations

```
tools/subtitle-extractor/
├── src/
│   ├── main.rs              # CLI entry point
│   ├── pgs.rs               # PGS decoder and PNG renderer
│   └── containers/
│       ├── mod.rs           # Container dispatcher
│       ├── mkv.rs           # MKV/WebM support
│       ├── mp4.rs           # MP4/M4V/MOV support
│       ├── ts.rs            # MPEG-TS/M2TS support
│       ├── cluster_cache.rs # MKV cluster index for seeking
│       └── ebml.rs          # EBML parser for MKV

internal/infrastructure/subtitles/
├── converter.go             # Go wrapper for subtitle-extractor

internal/api/handlers/
├── subtitles.go             # Subtitle API endpoints

web/src/components/media/VideoPlayer/
├── SubtitleOverlay.tsx      # Renders subtitles over video
├── SubtitleSelector.tsx     # Track selection UI

web/src/lib/hooks/
├── useSubtitles.ts          # Subtitle state management
```

## Building

```bash
# Build subtitle-extractor
cd tools/subtitle-extractor
cargo build --release
cp target/release/subtitle-extractor ../../bin/

# Build ViewRA (includes subtitle-extractor in PATH lookup)
make build
```

## Testing

```bash
# List tracks
./bin/subtitle-extractor tracks /path/to/movie.mkv

# Extract ASS to WebVTT
./bin/subtitle-extractor stream --track 3 --format webvtt /path/to/movie.mkv

# Render PGS to PNGs
./bin/subtitle-extractor render-pgs --track 4 --output /tmp/pgs_test --limit 10 /path/to/movie.mkv
```
