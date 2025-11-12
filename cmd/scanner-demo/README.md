# Scanner Demo

A demonstration tool for the ViewRA file system scanner that shows how the walker and filter components work together to discover media files in a directory.

## Usage

```bash
# Build the demo
go build -o scanner-demo ./cmd/scanner-demo/

# Run on a media directory
./scanner-demo -path /path/to/your/media/library

# With custom timeout
./scanner-demo -path /path/to/media -timeout 60s
```

## What It Does

The demo:
1. **Walks** the directory recursively using `filesystem.Walker`
2. **Filters** files using `filesystem.Filter` to identify media
3. **Classifies** media as video or audio based on extension
4. **Reports** statistics including:
   - Number of directories and files scanned
   - Media files found (videos and audio)
   - Non-media files skipped
   - Total size of media files
   - Scan duration

## Example Output

```
╔════════════════════════════════════════════════════════╗
║         ViewRA File System Scanner Demo               ║
╚════════════════════════════════════════════════════════╝

Scanning: /media/library

═══════════════════════════════════════════════════════
                    SCAN RESULTS
═══════════════════════════════════════════════════════

⏱️  Duration:          1.2s
📁 Directories:       156
📄 Files scanned:     1,234
🎬 Media files found: 892
⏭️  Files skipped:     342
💾 Total size:        250.5 GB

🎥 VIDEO FILES (650)
─────────────────────────────────────────────────────
   Movies/Inception (2010)/Inception.mkv (15.2 GB)
   Movies/The Matrix (1999)/The Matrix.mp4 (8.5 GB)
   TV Shows/Breaking Bad/S01E01.mkv (2.1 GB)
   ... and 647 more

🎵 AUDIO FILES (242)
─────────────────────────────────────────────────────
   Music/Artist/Album/01 - Track.mp3 (5.2 MB)
   Music/Artist/Album/02 - Track.flac (42.1 MB)
   ... and 240 more

📊 Media ratio: 72.3% of files are media

✅ Scan complete!
```

## What Gets Filtered Out

The scanner automatically skips:
- **Artwork files**: poster.jpg, banner.png, fanart.jpg, cover.jpg, etc.
- **Metadata files**: .nfo, .xml, subtitles (.srt, .vtt, .ass)
- **System files**: .DS_Store, Thumbs.db, .tmp, .bak files
- **Hidden files**: Any file or directory starting with `.`
- **Directories**: Only files are counted

## Supported Media Formats

### Video (20+ formats)
`.mkv`, `.mp4`, `.avi`, `.mov`, `.wmv`, `.flv`, `.webm`, `.m4v`, `.ts`, `.mpg`, `.mpeg`, `.vob`, `.m2ts`, and more

### Audio (15+ formats)
`.mp3`, `.flac`, `.wav`, `.m4a`, `.aac`, `.ogg`, `.wma`, `.opus`, `.ape`, `.alac`, and more

## Performance

- Efficiently uses `filepath.WalkDir` (fastest Go directory walker)
- Processes files in a single pass
- Minimal memory footprint
- Context support for cancellation/timeout

## Implementation Details

This demo showcases:
- Clean architecture with domain/infrastructure separation
- Interface-based design for testability
- Context-aware operations
- Smart file filtering
- Extension-based media type detection

See the source code at [main.go](main.go) for implementation details.
