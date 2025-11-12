# ADR 003: Scanner Optimization Strategy

## Status
Accepted - Phase 1 Implemented (November 11, 2025)

## Context

After implementing the initial filesystem scanner with progressive counting (ADR 002), we tested it on a network-mounted CIFS share with ~33,470 music files. The current performance:

- **Throughput:** 1.17-1.20 GB/s sustained
- **Processing rate:** 98%+ (workers keeping up well)
- **Network storage:** CIFS over gigabit connection
- **Zero errors:** Clean execution

While the current implementation works well, there are several optimization opportunities to improve performance, especially for:
1. Initial full scans on large libraries
2. Incremental/rescan operations
3. Network-attached storage scenarios
4. Resource utilization

## Performance Bottleneck Analysis

### Current Bottlenecks

1. **Hash computation:** Every file is hashed for duplicate detection (~30-50% of processing time)
2. **Network round trips:** Multiple stat calls per file over CIFS
3. **No incremental scanning:** Full rescans process all files every time
4. **Fixed worker pool:** May not be optimal for all I/O patterns
5. **Sequential directory walking:** Single goroutine walks entire tree
6. **No metadata caching:** Repeated filesystem queries

## Decision

Implement a phased optimization strategy with three phases:

### Phase 1: Quick Wins (Immediate Impact)

#### 1.1 Conditional Hashing
- **Impact:** 30-50% speed improvement
- **Implementation:**
  ```go
  type CoordinatorConfig struct {
      EnableDuplicateDetection bool
      HashingStrategy          string // "always", "on_conflict", "disabled"
  }
  ```
- Hash only when:
  - Duplicate file sizes detected (same size = potential duplicate)
  - User explicitly enables full duplicate detection
  - During incremental scans (compare with existing hashes)

#### 1.2 Smart File Skipping
- **Impact:** 80%+ faster rescans
- **Implementation:**
  ```go
  type FileCache struct {
      Path    string
      ModTime time.Time
      Hash    string
      Size    int64
  }
  ```
- Compare modification time before parsing
- Skip unchanged files in incremental scans

#### 1.3 Incremental Scan Support
- **Impact:** 90%+ faster on subsequent scans
- **Implementation:**
  ```go
  type ScanCheckpoint struct {
      LastScanTime time.Time
      FileCache    map[string]FileCache
      TotalFiles   int64
  }
  ```
- Track last scan time per library
- Only process files modified since last scan

### Phase 2: Architecture Improvements (Medium-term)

#### 2.1 Metadata Caching Layer
- **Impact:** 20-30% reduction in network round trips
- **Implementation:**
  ```go
  type MetadataCache struct {
      entries sync.Map // path -> CachedMetadata
      ttl     time.Duration
  }
  ```
- Batch directory reads
- Cache filesystem metadata with TTL
- Prefetch file metadata in batches

#### 2.2 Streaming Pipeline Refactor
- **Impact:** Better progress reporting, smoother UX
- **Architecture:**
  ```
  Discovery → Parsing → Hashing → Storage
       ↓         ↓         ↓         ↓
    Workers  Workers  Workers  Workers
  ```
- Separate hasher goroutines
- Allow results to flow through pipeline
- Better resource utilization

#### 2.3 Parallel Directory Walking
- **Impact:** 20-40% faster discovery on deep hierarchies
- **Implementation:**
  ```go
  func (c *Coordinator) discoverFilesParallel(ctx context.Context, root string) {
      topDirs := getTopLevelDirs(root)
      for _, dir := range topDirs {
          go c.walkSubtree(ctx, dir, fileChan)
      }
  }
  ```
- Spawn walkers for each top-level directory
- Coordinate with semaphore to prevent overload

### Phase 3: Advanced Optimizations (Future)

#### 3.1 Adaptive Worker Pool
- **Impact:** 10-15% better resource utilization
- **Logic:**
  - Increase workers if processing rate < 80% (I/O bound)
  - Decrease workers if processing rate > 95% (CPU bound)
  - Monitor system resources (CPU, memory, I/O wait)

#### 3.2 Statistics and Profiling
- Real-time performance metrics
- Bottleneck detection
- Historical performance tracking
- Optimization recommendations

#### 3.3 Database-Backed File Cache
- Persistent cache across application restarts
- Query optimization for incremental scans
- Support for library-wide deduplication

## Implementation Priority

```
┌─────────────────────────────────────────────────────────────┐
│ Phase 1: Quick Wins (Week 1-2) ✅ COMPLETED                 │
├─────────────────────────────────────────────────────────────┤
│ ✓ Progressive counting (COMPLETED - ADR 002)                │
│ ✓ Conditional hashing (COMPLETED - Nov 11, 2025)            │
│ ✓ Smart file skipping (COMPLETED - Nov 11, 2025)            │
│ ✓ Incremental scan support (COMPLETED - Nov 11, 2025)       │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Phase 2: Architecture (Future)                              │
├─────────────────────────────────────────────────────────────┤
│ ○ Metadata caching layer                                    │
│ ○ Streaming pipeline refactor                               │
│ ○ Parallel directory walking                                │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Phase 3: Advanced (Future)                                  │
├─────────────────────────────────────────────────────────────┤
│ ○ Adaptive worker pool                                      │
│ ○ Statistics and profiling hooks                            │
│ ○ Database-backed file cache                                │
└─────────────────────────────────────────────────────────────┘
```

## Expected Performance Improvements

### Initial Full Scan
- **Current:** ~1.2 GB/s on CIFS
- **Phase 1:** ~1.8-2.0 GB/s (40-60% faster)
- **Phase 2:** ~2.2-2.5 GB/s (80-100% faster)
- **Phase 3:** ~2.5-3.0 GB/s (100-150% faster)

### Incremental Scans (with 5% file changes)
- **Current:** 100% of full scan time
- **Phase 1:** ~5-10% of full scan time (90%+ faster)
- **Phase 2:** ~3-5% of full scan time
- **Phase 3:** ~2-3% of full scan time

### Memory Usage
- **Current:** ~50 MB baseline
- **Phase 1:** ~60 MB (file cache overhead)
- **Phase 2:** ~80 MB (metadata cache)
- **Phase 3:** ~100 MB (comprehensive caching)

## Implementation Details

### File Cache Structure
```go
type FileCacheEntry struct {
    Path         string
    Size         int64
    ModTime      time.Time
    Hash         string
    MediaType    scanner.MediaType

    // Parsed metadata (avoid reparsing)
    Title        string
    Artist       string
    Album        string
    Year         *int
    SeasonNumber *int
    EpisodeNumber *int
    TrackNumber  *int
}

type FileCacheStore interface {
    Get(path string) (*FileCacheEntry, error)
    Set(entry *FileCacheEntry) error
    Delete(path string) error
    GetByLibrary(libraryID int64) ([]*FileCacheEntry, error)
    Clear(libraryID int64) error
}
```

### Hashing Strategy Configuration
```go
const (
    HashingStrategyAlways     = "always"      // Hash every file
    HashingStrategyOnConflict = "on_conflict" // Hash only duplicate sizes
    HashingStrategyDisabled   = "disabled"    // Never hash (no duplicate detection)
)

type HashingConfig struct {
    Strategy           string
    ConflictThreshold  int64 // Size in bytes - below this, always hash
    MaxConcurrentHash  int   // Limit hasher goroutines
}
```

### Incremental Scan Flow
```
1. Load last scan checkpoint from database
2. Walk directory tree
3. For each file:
   a. Check if exists in cache
   b. Compare ModTime with cache
   c. If unchanged: Use cached metadata
   d. If changed/new: Full processing
4. Mark missing files as deleted
5. Save new checkpoint
```

## Testing Strategy

### Performance Benchmarks
- Benchmark against real-world libraries (10K, 50K, 100K files)
- Test on different storage types (local SSD, HDD, NFS, CIFS)
- Measure memory usage under load
- Profile CPU and I/O bottlenecks

### Correctness Tests
- Verify incremental scans detect all changes
- Ensure file cache consistency
- Test duplicate detection accuracy
- Validate metadata parsing preservation

### Integration Tests
- End-to-end scan workflows
- Concurrent scan handling
- Error recovery and retry logic
- Progress reporting accuracy

## Consequences

### Positive
- 40-150% faster initial scans (depending on phase)
- 90%+ faster incremental scans
- Better resource utilization
- Improved user experience with faster rescans
- Reduced network I/O for remote storage

### Negative
- Increased code complexity
- Memory overhead for caching (~50-100 MB)
- Need to manage cache invalidation
- Database schema changes for persistence
- More configuration options for users

### Risks
- Cache invalidation bugs could lead to stale data
- Aggressive caching could miss file changes
- Complexity could introduce new bugs
- Need careful testing for edge cases

## Alternatives Considered

### 1. Use Third-Party Indexing Tools
- **Rejected:** Doesn't integrate with our domain model
- Would require complex synchronization

### 2. Database-First Approach
- **Rejected:** Too slow for initial scans
- Would require two-phase scan (filesystem → database)

### 3. Event-Based File Watching
- **Deferred:** Complex to implement correctly
- OS-specific implementations (inotify, FSEvents, etc.)
- Consider for Phase 4 if needed

## References
- ADR 001: Dual Database Support Strategy
- ADR 002: Filesystem Scanner Design
- Performance test results: `/home/fictional/Projects/viewra2/docs/research/scanner-performance-cifs.txt`
- Go filepath.WalkDir documentation
- CIFS/SMB performance characteristics

## Notes
- Current implementation (progressive counting) successfully tested on 33,470 file library
- Network storage (CIFS) is the primary bottleneck for I/O operations
- Hash computation dominates CPU time (SHA256 over gigabytes of data)
- Worker pool scales well up to 8 workers on test system
