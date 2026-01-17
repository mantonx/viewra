# Subtitle Pipeline Architecture

This document describes ViewRA's subtitle extraction and streaming pipeline, which uses the custom `subtitle-extractor` tool instead of FFmpeg for subtitle processing.

## Overview

ViewRA supports two categories of subtitles:

| Category | Formats | Extraction Method | Delivery |
|----------|---------|-------------------|----------|
| **Text** | SRT, ASS/SSA, WebVTT, tx3g (mov_text) | subtitle-extractor → WebVTT | Client-side overlay |
| **Bitmap** | PGS (Blu-ray), VobSub (DVD) | subtitle-extractor stream-pgs → WebP | Client-side image overlay |

Both subtitle types are rendered client-side, avoiding the need for video transcoding.

## Components

### 1. subtitle-extractor (Rust CLI)

Location: `tools/subtitle-extractor/`

A fast, purpose-built tool for subtitle extraction that supports:

- **Containers**: MKV (Matroska), MP4/M4V/MOV, MPEG-TS/M2TS
- **Text codecs**: SRT, ASS/SSA, WebVTT, tx3g
- **Bitmap codecs**: PGS (hdmv_pgs_subtitle), VobSub (dvd_subtitle)

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

# Stream PGS subtitles as WebP images (JSON lines to stdout)
subtitle-extractor stream-pgs --track <N> --start <ms> --end <ms> <FILE>

# [Debug] Render PGS subtitles to image files in a directory
subtitle-extractor debug-render-pgs --track <N> --output <DIR> --start <ms> --end <ms> --limit <N> <FILE>
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
- **PGS streaming**: Streams WebP frames for bitmap subtitles

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

// Stream PGS frames as WebP images
func (c *Converter) StreamPGSFrames(ctx context.Context, mediaPath string, streamIndex int, startMS, endMS int64) (<-chan PGSFrame, <-chan error)

// Get all PGS frames at once (for non-streaming use)
func (c *Converter) GetAllPGSFrames(ctx context.Context, mediaPath string, streamIndex int) ([]PGSFrame, error)
```

### 3. API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /api/media/:id/subtitles/:trackId` | Get subtitle by database track ID |
| `GET /api/media/:id/subtitles/stream/:index` | Get embedded subtitle by absolute stream index |
| `GET /api/media/:id/subtitles/text/:index/stream` | Get text subtitle by relative index (among text tracks) |
| `GET /api/media/:id/subtitles/pgs/:index` | Get all PGS frames as JSON array (WebP images) |
| `GET /api/media/:id/subtitles/pgs/:index/stream` | Stream PGS frames as JSON lines (NDJSON) |

### 4. Frontend Components

- **SubtitleOverlay.tsx**: Renders both text and bitmap subtitles as positioned overlays
  - `TextSubtitleRenderer`: Parses WebVTT and renders styled text
  - `PGSSubtitleRenderer`: Displays WebP images with proper scaling/positioning
- **useSubtitles.ts**: Manages subtitle track selection and preferences
- **SubtitleSelector.tsx**: UI for selecting subtitle tracks (both text and bitmap)

## Architecture

### Text Subtitles

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

### Bitmap Subtitles (PGS)

```
┌─────────────────┐     ┌───────────────────┐     ┌─────────────┐     ┌─────────┐
│ Media File      │────▶│ subtitle-extractor│────▶│ JSON Array  │────▶│ Browser │
│ (MKV with PGS)  │     │ stream-pgs        │     │ + WebP imgs │     │ Img     │
│                 │     │ (WebP encoding)   │     │ (base64)    │     │ Overlay │
└─────────────────┘     └───────────────────┘     └─────────────┘     └─────────┘
```

**WebP Output Format:**
```json
{
  "start_ms": 5000,
  "end_ms": 8000,
  "x": 100,
  "y": 800,
  "width": 1720,
  "height": 80,
  "image_base64": "UklGRiQA...base64 WebP data..."
}
```

**Client-side rendering:**
- WebP images are decoded and positioned using CSS transforms
- Position/size scaled to match video's display dimensions
- requestAnimationFrame ensures smooth timing synchronization

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

### Network File Performance

For files on network shares:

| Operation | Local SSD | NFS/SMB |
|-----------|-----------|---------|
| Track listing | <100ms | <500ms |
| Seek + 10s extract | <200ms | <1s |
| Full sequential read (50GB) | ~30s | 5-10 min |

### PGS Memory Considerations

- WebP encoding uses lossless mode (preserves subtitle quality)
- Base64 encoding adds ~33% overhead to image data
- Typical PGS frame: 50-200KB after WebP compression
- Full movie subtitle track: 5-50MB total (varies by subtitle density)

## File Locations

```
tools/subtitle-extractor/
├── src/
│   ├── main.rs              # CLI entry point
│   ├── pgs.rs               # PGS decoder and WebP renderer
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
├── SubtitleOverlay.tsx      # Renders subtitles over video (text + bitmap)
├── SubtitleSelector.tsx     # Track selection UI

web/src/lib/hooks/
├── useSubtitles.ts          # Subtitle state management

web/src/lib/types/
├── subtitles.ts             # Shared subtitle types
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

# Stream PGS to WebP (JSON lines)
./bin/subtitle-extractor stream-pgs --track 4 /path/to/movie.mkv | head -1

# Debug: Render PGS to files
./bin/subtitle-extractor debug-render-pgs --track 4 --output /tmp/pgs_test --limit 10 /path/to/movie.mkv
```

## API Usage Examples

```bash
# Get text subtitle as WebVTT
curl http://localhost:8080/api/media/123/subtitles/text/0/stream

# Get all PGS frames (JSON array)
curl http://localhost:8080/api/media/123/subtitles/pgs/0

# Stream PGS frames (JSON lines)
curl http://localhost:8080/api/media/123/subtitles/pgs/0/stream
```
