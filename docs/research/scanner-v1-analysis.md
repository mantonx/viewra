# Scanner V1 Analysis

Analysis of the file system scanner from `refactor/clean-video-playback` branch to inform V2 design.

## Key Learnings

### 1. Architecture Patterns

**Good Design Decisions:**
- Clean separation of concerns with multiple scanner components
- Worker pool pattern for concurrent file processing
- Progress tracking with atomic counters (thread-safe)
- Adaptive throttling based on system resources
- Plugin system for extensible metadata extraction
- Event-driven architecture with EventBus
- Crash recovery and orphaned job handling

**Component Structure:**
```
scannermodule/
├── scanner/
│   ├── engine.go          # Core scanner interface definitions
│   ├── basic_types.go     # LibraryScanner implementation
│   ├── manager.go         # Scanner Manager with recovery
│   ├── config.go          # Scanner configuration
│   ├── dispatcher.go      # Work distribution
│   ├── progress.go        # Progress tracking
│   ├── adaptive_throttler.go  # System resource management
│   ├── system_monitor.go  # System metrics
│   └── safeguards.go      # Safety checks
├── handlers.go    # HTTP handlers
├── routes.go      # API routes
└── service.go     # Service layer
```

### 2. File Discovery Strategy

**Walk Pattern:**
- Uses `filepath.WalkDir` for efficient directory traversal
- Processes files asynchronously via worker queue
- Skips directories and non-media files early
- Smart filtering for artwork, metadata, and system files

**File Type Detection:**
```go
// Excellent pattern for filtering out artwork/metadata
artworkPatterns := []string{
    "poster.", "banner.", "thumb.", "cover.", "fanart.",
    "backdrop.", "clearlogo.", "clearart.", "disc.",
}

metadataExtensions := []string{
    ".nfo", ".xml", ".srt", ".vtt", ".ass", ".sub", ".idx"
}

systemPatterns := []string{
    ".ds_store", "thumbs.db", ".tmp", ".bak"
}
```

**Media Type Determination:**
- Library type drives primary classification
- Extension-based fallback detection
- Explicit handling of cross-type files (audio in video library, etc.)

### 3. Metadata Extraction

**Two-Phase Approach:**
1. **Technical Metadata** (FFprobe) - Always extracted first
   - Duration, codecs, bitrate, resolution
   - Directly from media file
   - Stored in media_file table

2. **Content Metadata** (Plugins) - Optional, enrichment
   - Title, artist, album (audio)
   - Movie/TV show details (video)
   - Plugin-based, extensible

**FFprobe Integration:**
```go
// Clean pattern for FFprobe usage
cmd := exec.Command("ffprobe",
    "-v", "quiet",
    "-print_format", "json",
    "-show_format",
    "-show_streams",
    filePath)
```

### 4. Progress Tracking

**Atomic Counters:**
```go
filesProcessed atomic.Int64
filesFound     atomic.Int64
filesSkipped   atomic.Int64
bytesProcessed atomic.Int64
errorsCount    atomic.Int64
```

**Progress Updates:**
- Background goroutine updates every 2 seconds
- Calculates percentage: `filesProcessed / filesFound * 100`
- Emits events via EventBus
- Updates database with current state

### 5. Worker Pool Pattern

**Key Features:**
- Configurable number of workers (defaults to CPU count)
- File queue with buffer (1000 files)
- Context-based cancellation
- Graceful shutdown with WaitGroup

**Implementation:**
```go
for i := 0; i < ls.workers; i++ {
    ls.wg.Add(1)
    go ls.fileWorker(libraryID)
}

func (ls *LibraryScanner) fileWorker(libraryID uint) {
    defer ls.wg.Done()
    for {
        select {
        case filePath, ok := <-ls.fileQueue:
            if !ok { return }
            ls.processFile(filePath, libraryID)
        case <-ls.ctx.Done():
            return
        }
    }
}
```

### 6. State Management & Recovery

**Database-Backed State:**
- `ScanJob` table tracks all scan operations
- Status: `running`, `paused`, `completed`, `failed`
- Progress fields: `files_found`, `files_processed`, `bytes_processed`

**Crash Recovery:**
1. Detect "orphaned" jobs (status=running after restart)
2. Mark as paused with recovery message
3. Auto-resume if progress >= 10 files or 1%
4. Cleanup duplicate jobs per library

**Smart Resume:**
- Treats resume similar to fresh start (walks full directory)
- Checks `last_seen` timestamp to skip existing files
- Reprocesses files that may have changed

### 7. Plugin Architecture

**Extensibility Points:**
- File handlers (match + process pattern)
- Enrichment hooks (OnMediaFileScanned)
- Metadata extraction customization
- Multiple handlers can process same file

**Handler Interface:**
```go
type FileHandler interface {
    Match(path string, fileInfo os.FileInfo) bool
    HandleFile(path string, ctx *MetadataContext) error
    GetName() string
}
```

### 8. Adaptive Throttling

**System Monitoring:**
- CPU usage, memory pressure
- I/O wait, network bandwidth
- Emergency brake at 95% threshold

**Dynamic Adjustment:**
- Batch size modification
- Processing delays
- Worker count scaling

### 9. Error Handling

**Resilient Processing:**
- Continue on individual file errors
- Track error count
- Log warnings vs errors appropriately
- Cleanup partial work on context cancellation

**Validation:**
- File existence checks
- Library existence validation
- Database transaction rollback
- Distributed locking for concurrent operations

## What to Keep for V2

✅ **Core Patterns:**
1. Worker pool with configurable workers
2. Atomic counters for thread-safe progress
3. Two-phase metadata extraction (technical + content)
4. Smart file type detection and filtering
5. Context-based cancellation
6. Event-driven progress updates
7. Database-backed state persistence

✅ **File Processing:**
1. `filepath.WalkDir` for efficiency
2. Artwork/metadata file filtering
3. Extension-based media type detection
4. FFprobe for technical metadata
5. Queue-based async processing

✅ **Robustness:**
1. Crash recovery with auto-resume
2. Graceful shutdown with WaitGroup
3. Progress tracking in database
4. Error counting and reporting

## What to Improve for V2

🔄 **Simplifications:**
1. Remove complex plugin architecture (start simple)
2. Drop adaptive throttling initially (YAGNI)
3. Remove file monitoring (separate feature)
4. Simplify safeguards (focus on core functionality)
5. Remove distributed locking (single instance for now)

🔄 **Clean Architecture:**
1. Follow domain-driven design patterns
2. Separate scanner infrastructure from domain logic
3. Use repository pattern consistently
4. Apply conventions (types.go, interfaces.go)
5. Type-safe with no `interface{}` unless necessary

🔄 **Modern Go Practices:**
1. Use generics where appropriate
2. Structured logging (not logger package)
3. Error wrapping with `%w`
4. Context with deadlines
5. No global variables

🔄 **Testing:**
1. Comprehensive unit tests
2. Testable design (inject dependencies)
3. Mock-friendly interfaces
4. Table-driven tests
5. Integration tests with real files

## V2 Design Proposal

### Package Structure
```
internal/
├── domain/
│   └── scanner/
│       ├── types.go       # FileInfo, ScanResult, Progress
│       ├── scanner.go     # Scanner interface
│       ├── repository.go  # ScanJob repository interface
│       ├── errors.go      # Domain errors
│       └── service.go     # Business logic
│
└── infrastructure/
    ├── filesystem/
    │   ├── types.go       # Walker, FileFilter
    │   ├── scanner.go     # Scanner implementation
    │   ├── walker.go      # Directory walker
    │   ├── filter.go      # File filtering
    │   └── parser.go      # Filename parsing
    │
    └── scanner/
        ├── types.go       # Repository struct
        ├── repository.go  # ScanJob CRUD
        └── worker.go      # Worker pool
```

### Core Interfaces
```go
// Domain
type Scanner interface {
    Scan(ctx context.Context, libraryID int64) (*ScanJob, error)
    Resume(ctx context.Context, jobID int64) error
    Pause(ctx context.Context, jobID int64) error
    GetProgress(ctx context.Context, jobID int64) (*Progress, error)
}

// Infrastructure
type Walker interface {
    Walk(ctx context.Context, root string) (<-chan FileInfo, error)
}

type FileFilter interface {
    ShouldProcess(path string, info os.FileInfo) bool
}

type FilenameParser interface {
    ParseMovie(filename string) (*MovieInfo, error)
    ParseTVEpisode(filename string) (*TVEpisodeInfo, error)
    ParseMusic(filename string) (*MusicInfo, error)
}
```

### Implementation Strategy

1. **Phase 1.5.1**: Basic directory walking and file discovery
2. **Phase 1.5.2**: Filename parsing (movies, TV, music)
3. **Phase 1.5.3**: Worker pool and progress tracking
4. **Phase 1.5.4**: Integration with FFmpeg metadata
5. **Phase 1.5.5**: Database persistence and resume capability

## Key Takeaways

1. **Start Simple**: V1 has excellent patterns but is complex. V2 should start with core functionality.
2. **Focus on Quality**: Proper testing, clean architecture, following conventions.
3. **Incremental Features**: Build worker pools, throttling, monitoring as needed, not upfront.
4. **Learn from Mistakes**: V1 had too many abstractions. V2 should be pragmatic.
5. **Domain First**: Define clear domain models before infrastructure.

## References

- V1 Scanner: `refactor/clean-video-playback:internal/modules/scannermodule/`
- FFmpeg Integration: Already complete in V2 (Phase 1.4)
- Filename Parsing: Research TMDb naming conventions, Plex naming conventions
