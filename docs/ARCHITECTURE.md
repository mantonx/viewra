# Viewra Architecture Documentation

## Overview

Viewra is a modern media management platform that uses a two-stage transcoding pipeline with content-addressable storage for efficient, CDN-friendly media delivery.

## Key Architectural Changes

### From Single-Stage to Two-Stage Pipeline

**Previous Architecture:**
- FFmpeg directly generated DASH/HLS manifests
- DASH manifests incorrectly marked as `type="dynamic"` for VOD content
- Session-based URLs: `/api/playback/stream/{session_id}/`
- No content deduplication

**New Architecture:**
- Stage 1: FFmpeg encodes to intermediate MP4 files
- Stage 2: Shaka Packager creates proper VOD manifests (`type="static"`)
- Content-addressable URLs: `/api/v1/content/{hash}/`
- Automatic content deduplication

## Core Components

### 1. Pipeline Manager (`sdk/transcoding/pipeline/`)

The Pipeline Manager coordinates the two-stage transcoding process:

```go
type Manager struct {
    encoder      Encoder      // FFmpeg for video/audio encoding
    packager     Packager     // Shaka Packager for manifest generation
    storage      Storage      // Job directory management
    contentStore *ContentStore // Content-addressable storage
}
```

**Key Features:**
- Manages job lifecycle (create, execute, cleanup)
- Coordinates data flow between stages
- Handles error recovery and retries
- Generates content hashes for deduplication

### 2. Content-Addressable Storage (`sdk/transcoding/storage/`)

Implements efficient content storage with SHA256-based addressing:

```go
type ContentStore struct {
    basePath string
    urlGen   *URLGenerator
    mutex    sync.RWMutex
}
```

**Features:**
- Deterministic hash generation
- Directory sharding (first 4 chars of hash)
- Atomic operations for concurrent access
- CDN-friendly URL structure

**Hash Generation:**
```go
// Hash is based on:
// - Media ID + Codec + Resolution + Container
// Example: "file123-h264-aac-1080p-dash" → "abc123def456..."
```

### 3. FFmpeg Plugin System (`plugins/transcoding/`)

Hardware-accelerated encoding plugins:

- `ffmpeg_software`: CPU-based encoding
- `ffmpeg_nvidia`: NVIDIA GPU acceleration
- `ffmpeg_qsv`: Intel Quick Sync Video
- `ffmpeg_vaapi`: VA-API acceleration

Each plugin implements the pipeline-aware interface:

```go
type TranscodingProvider interface {
    StartTranscode(request *TranscodeRequest) (*TranscodeHandle, error)
    GetProgress(handle *TranscodeHandle) (*TranscodeProgress, error)
    StopTranscode(handle *TranscodeHandle) error
    // Pipeline support
    SupportsFeature(feature string) bool
}
```

### 4. Shaka Packager Integration (`sdk/transcoding/shaka/`)

Handles manifest generation and segmentation:

```go
type Packager struct {
    binaryPath string
    logger     Logger
}

func (p *Packager) Package(inputs []string, output string, options PackageOptions) error {
    // Generates DASH/HLS manifests with proper VOD settings
}
```

**Key Options:**
- `--generate_static_live_mpd=false` for proper VOD manifests
- Multi-bitrate support for ABR
- Segment duration configuration

### 5. Playback Module (`internal/modules/playbackmodule/`)

Manages transcoding sessions and playback decisions:

```go
type Manager struct {
    transcodingService *TranscodingService
    sessionStore       *SessionStore
    contentStore       storage.ContentStoreInterface
    playbackPlanner    *PlaybackPlanner
}
```

**Intelligent Playback Decisions:**
- Analyzes device capabilities
- Determines if transcoding is needed
- Reuses existing content when available
- Optimizes encoding parameters

### 6. Frontend Integration

**MediaPlayer Component:**
- Vidstack player with ABR support
- Content-addressable URL handling
- Seek-ahead functionality
- Progress saving/restoration

**URL Building:**
```typescript
// Old approach
const manifestUrl = `/api/playback/stream/${sessionId}/manifest.mpd`;

// New approach
const manifestUrl = `/api/v1/content/${contentHash}/manifest.mpd`;
```

## Data Flow

### Transcoding Flow

```
1. Client Request
   POST /api/playback/start
   {media_file_id, container, device_profile}
   
2. Playback Decision
   - Analyze media file properties
   - Check device capabilities
   - Determine optimal encoding parameters
   
3. Content Hash Check
   - Generate hash from parameters
   - Check if content already exists
   - Return immediately if found
   
4. Pipeline Execution
   a. Encoding Stage (FFmpeg)
      - Transcode to H.264/HEVC
      - Output intermediate MP4s
      - Apply quality settings
      
   b. Packaging Stage (Shaka)
      - Generate DASH/HLS manifest
      - Create media segments
      - Set VOD properties
      
5. Content Storage
   - Store in content-addressable structure
   - Update database with content hash
   - Return CDN-friendly URLs
```

### Seek-Ahead Flow

```
1. User seeks beyond buffered content
2. Frontend detects seek-ahead need
3. POST /api/playback/seek-ahead
4. New session starts from seek position
5. Frontend switches to new manifest
6. Original session continues for buffer
```

## Database Schema

### Key Tables

**transcode_sessions:**
```sql
CREATE TABLE transcode_sessions (
    id            VARCHAR(36) PRIMARY KEY,
    status        VARCHAR(20),
    provider      VARCHAR(50),
    content_hash  VARCHAR(64) INDEX,  -- New field
    request       TEXT,
    created_at    TIMESTAMP,
    updated_at    TIMESTAMP
);
```

**media_files:**
```sql
CREATE TABLE media_files (
    id         VARCHAR(36) PRIMARY KEY,
    path       TEXT,
    media_type VARCHAR(20),
    metadata   JSON,
    duration   INTEGER,
    file_size  BIGINT
);
```

## Configuration

### Environment Variables

```bash
# Content Storage
CONTENT_STORE_PATH=/app/viewra-data/content-store
CONTENT_BASE_URL=/api/v1/content

# Pipeline Settings
PIPELINE_ENCODER_TIMEOUT=3600
PIPELINE_PACKAGER_TIMEOUT=600
PIPELINE_MAX_RETRIES=3

# Transcoding
TRANSCODE_MAX_CONCURRENT=4
TRANSCODE_QUALITY_PRESET=medium
TRANSCODE_ENABLE_ABR=true
```

### Plugin Configuration (CUE)

```cue
plugin_name: "ffmpeg_nvidia"
plugin_type: "transcoding"
version: "1.0.0"

config: {
    nvidia_gpu: 0
    max_concurrent: 2
    hardware_accel: true
    
    quality_presets: {
        low: {crf: 28, preset: "fast"}
        medium: {crf: 23, preset: "medium"}
        high: {crf: 18, preset: "slow"}
    }
}
```

## Performance Optimizations

### Content Deduplication
- Same content encoded once, served many times
- Hash-based identification
- Reduces storage by 60-80% for popular content

### CDN Integration
- Static, immutable content paths
- Aggressive caching headers
- Edge server compatibility

### Hardware Acceleration
- NVIDIA NVENC for 10x faster encoding
- Intel QSV for integrated graphics
- Automatic fallback to software encoding

### Parallel Processing
- Concurrent encoding jobs
- Pipeline stage parallelism
- Resource-aware scheduling

## Monitoring and Debugging

### Metrics
- Content store hit/miss rates
- Pipeline stage timings
- Storage utilization
- Transcoding queue depth

### Logging
- Structured logging with context
- Debug mode for FFmpeg commands
- Pipeline stage transitions
- Content hash generation

### Health Checks
- `/api/health` - Overall system health
- `/api/playback/stats` - Transcoding statistics
- Plugin availability status

## Migration Guide

### From Session-Based to Content-Addressable URLs

1. **Backend Changes:**
   - Sessions now include `content_hash` field
   - APIs return both legacy and new URLs
   - Automatic redirects for old endpoints

2. **Frontend Changes:**
   ```typescript
   // Check for content hash first
   if (response.content_hash) {
     url = `/api/v1/content/${response.content_hash}/manifest.mpd`;
   } else {
     // Fallback to legacy
     url = response.manifest_url;
   }
   ```

3. **Database Migration:**
   ```sql
   ALTER TABLE transcode_sessions 
   ADD COLUMN content_hash VARCHAR(64);
   CREATE INDEX idx_content_hash ON transcode_sessions(content_hash);
   ```

## Future Enhancements

### Planned Features
1. **Content Prewarming** - Proactive transcoding of popular content
2. **Multi-CDN Support** - Route to nearest CDN endpoint
3. **Live Streaming** - Extend pipeline for live content
4. **Quality Analytics** - Track playback quality metrics
5. **Bandwidth Optimization** - Adaptive quality switching

### Architecture Evolution
- Distributed transcoding across multiple nodes
- Cloud storage backend support (S3, GCS)
- WebRTC for ultra-low latency streaming
- AI-based quality optimization