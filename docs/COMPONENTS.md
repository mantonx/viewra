# Viewra Component Documentation

## Backend Components

### Pipeline Manager

**Location:** `sdk/transcoding/pipeline/manager.go`

**Purpose:** Orchestrates the two-stage transcoding pipeline, coordinating between FFmpeg encoding and Shaka Packager packaging.

**Key Methods:**
```go
// NewManager creates a new pipeline manager
func NewManager(encoder Encoder, packager Packager, storage Storage, opts ...Option) *Manager

// CreateJob creates a new transcoding job
func (m *Manager) CreateJob(input string, params JobParams) (*Job, error)

// ExecuteJob runs the complete pipeline
func (m *Manager) ExecuteJob(job *Job) (*Result, error)

// GetContentHash generates deterministic hash for content
func (m *Manager) GetContentHash(mediaID, codec, resolution, container string) string
```

**Usage Example:**
```go
manager := pipeline.NewManager(
    ffmpegEncoder,
    shakaPackager,
    jobStorage,
    pipeline.WithContentStore(contentStore),
    pipeline.WithLogger(logger),
)

job, err := manager.CreateJob("/media/movie.mp4", pipeline.JobParams{
    VideoCodec: "h264",
    AudioCodec: "aac",
    Resolution: "1080p",
    Container:  "dash",
})

result, err := manager.ExecuteJob(job)
// result.ContentHash = "abc123def456..."
// result.ManifestPath = "/content-store/ab/c1/abc123def456/manifest.mpd"
```

### Content Store

**Location:** `sdk/transcoding/storage/content_store.go`

**Purpose:** Implements content-addressable storage with deduplication and CDN-friendly URLs.

**Key Methods:**
```go
// Store saves content with hash-based path
func (cs *ContentStore) Store(hash string, sourcePath string) error

// Exists checks if content already exists
func (cs *ContentStore) Exists(hash string) bool

// GetPath returns the storage path for a hash
func (cs *ContentStore) GetPath(hash string) string

// GenerateHash creates deterministic content hash
func GenerateHash(components ...string) string
```

**Directory Structure:**
```
/content-store/
├── ab/c1/abc123def456/
│   ├── manifest.mpd
│   ├── video-0001.m4s
│   └── audio-0001.m4s
└── de/f4/def456ghi789/
    ├── playlist.m3u8
    └── segment-0001.ts
```

### Shaka Packager Wrapper

**Location:** `sdk/transcoding/shaka/packager.go`

**Purpose:** Go wrapper for Shaka Packager binary, handling DASH/HLS manifest generation.

**Key Methods:**
```go
// Package creates DASH/HLS manifests from input files
func (p *Packager) Package(inputs []string, output string, options PackageOptions) error

// ValidateManifest checks manifest for VOD compliance
func (p *Packager) ValidateManifest(manifestPath string) error
```

**Configuration:**
```go
options := PackageOptions{
    Container:              "dash",
    SegmentDuration:        4,
    GenerateStaticMPD:      true,  // Critical for VOD
    HLSPlaylistType:        "vod",
    MultiPeriod:            false,
}
```

### Transcoding Service

**Location:** `internal/modules/playbackmodule/core/transcode_service.go`

**Purpose:** Manages transcoding sessions, provider lifecycle, and error recovery.

**Key Features:**
- Provider abstraction for different transcoders
- Session state management
- Progress tracking
- Error recovery with circuit breakers

**Key Methods:**
```go
// StartTranscode initiates a new transcoding session
func (s *TranscodingService) StartTranscode(ctx context.Context, req *TranscodeRequest) (*database.TranscodeSession, error)

// GetProgress returns current transcoding progress
func (s *TranscodingService) GetProgress(sessionID string) (*TranscodeProgress, error)

// CompleteSession finalizes session with content hash
func (s *TranscodingService) CompleteSession(sessionID string, result *types.PipelineResult) error
```

### Playback Planner

**Location:** `internal/modules/playbackmodule/planner.go`

**Purpose:** Makes intelligent decisions about transcoding based on device capabilities and media properties.

**Decision Logic:**
```go
func (p *PlaybackPlanner) DecidePlayback(mediaPath string, profile *DeviceProfile) (*PlaybackDecision, error) {
    // 1. Analyze media file
    mediaInfo := p.analyzer.Analyze(mediaPath)
    
    // 2. Check device compatibility
    if p.isDirectPlayCompatible(mediaInfo, profile) {
        return &PlaybackDecision{
            ShouldTranscode: false,
            Reason: "Direct play compatible",
        }, nil
    }
    
    // 3. Generate optimal transcode parameters
    params := p.generateTranscodeParams(mediaInfo, profile)
    
    // 4. Check for existing content
    hash := p.contentStore.GenerateHash(params)
    if p.contentStore.Exists(hash) {
        return &PlaybackDecision{
            ShouldTranscode: false,
            Reason: "Content already exists",
            ContentHash: hash,
        }, nil
    }
    
    return &PlaybackDecision{
        ShouldTranscode: true,
        TranscodeParams: params,
        Reason: p.incompatibilityReason,
    }, nil
}
```

## Frontend Components

### MediaPlayer

**Location:** `frontend/src/components/MediaPlayer/MediaPlayer.tsx`

**Purpose:** Main video player component using Vidstack with content-addressable URL support.

**Key Features:**
- Adaptive bitrate streaming (ABR)
- Seek-ahead functionality
- Progress saving/restoration
- Device-specific optimizations

**Props:**
```typescript
interface MediaPlayerProps {
  // Media identification
  movieId?: number;
  episodeId?: number;
  
  // Player options
  autoplay?: boolean;
  startTime?: number;
  
  // Callbacks
  onBack?: () => void;
  onComplete?: () => void;
}
```

**Content URL Handling:**
```typescript
const getStreamUrl = (decision: PlaybackDecision): string => {
  if (decision.content_hash) {
    // Use content-addressable storage
    const isHLS = decision.transcode_params?.target_container === 'hls';
    return isHLS 
      ? `/api/v1/content/${decision.content_hash}/playlist.m3u8`
      : `/api/v1/content/${decision.content_hash}/manifest.mpd`;
  }
  // Fallback to legacy URLs
  return decision.manifest_url || '';
};
```

### ProgressBar

**Location:** `frontend/src/components/MediaPlayer/components/ProgressBar/ProgressBar.tsx`

**Purpose:** Video progress bar with seek functionality and buffered range visualization.

**Features:**
- Hover preview time
- Buffered range display
- Seek-ahead indicators
- Smooth dragging

**Props:**
```typescript
interface ProgressBarProps {
  currentTime: number;
  duration: number;
  bufferedRanges: Array<{start: number; end: number}>;
  isSeekable: boolean;
  isSeekingAhead: boolean;
  onSeek: (progress: number) => void;
  onSeekIntent?: (time: number) => void;
  showSeekAheadIndicator?: boolean;
}
```

### MediaService

**Location:** `frontend/src/services/MediaService.ts`

**Purpose:** API client for media operations with content-addressable storage support.

**Key Methods:**
```typescript
class MediaService {
  // Start transcoding with content deduplication
  static async startTranscodingSession(
    mediaFileId: string,
    container: 'dash' | 'hls',
    videoCodec?: string
  ): Promise<{
    id: string;
    content_hash?: string;
    content_url?: string;
    manifest_url: string;
  }>
  
  // Request seek-ahead transcoding
  static async requestSeekAhead(params: {
    session_id: string;
    seek_position: number;
  }): Promise<TranscodeSession>
  
  // Get playback decision
  static async getPlaybackDecision(
    mediaPath: string,
    mediaFileId: string,
    deviceProfile: DeviceProfile
  ): Promise<PlaybackDecision>
}
```

### Hooks

#### useMediaNavigation

**Location:** `frontend/src/hooks/media/useMediaNavigation.ts`

**Purpose:** Handles media loading and navigation with content-addressable URL building.

```typescript
const { mediaId, mediaFile, currentMedia, loadingState } = useMediaNavigation({
  type: 'episode',
  episodeId: 123
});
```

#### useSeekAhead

**Location:** `frontend/src/hooks/session/useSeekAhead.ts`

**Purpose:** Manages seek-ahead functionality for buffering beyond current position.

```typescript
const { 
  requestSeekAhead, 
  isSeekAheadNeeded, 
  seekAheadState 
} = useSeekAhead();

// Check if seek-ahead needed
if (isSeekAheadNeeded(seekTime)) {
  await requestSeekAhead(seekTime);
}
```

## Plugin Components

### FFmpeg Software Plugin

**Location:** `plugins/transcoding/ffmpeg_software/`

**Purpose:** CPU-based transcoding with pipeline support.

**Configuration (plugin.cue):**
```cue
plugin_name: "ffmpeg_software"
plugin_type: "transcoding"
version: "2.0.0"

config: {
    max_concurrent_jobs: 4
    temp_directory: "/tmp/ffmpeg"
    
    encoder_settings: {
        h264: {
            preset: "medium"
            crf: 23
            profile: "high"
        }
        h265: {
            preset: "medium"
            crf: 28
        }
    }
    
    pipeline_support: {
        enabled: true
        output_format: "mp4"
        segment_duration: 4
    }
}
```

### FFmpeg NVIDIA Plugin

**Location:** `plugins/transcoding/ffmpeg_nvidia/`

**Purpose:** GPU-accelerated transcoding using NVIDIA NVENC.

**Hardware Requirements:**
- NVIDIA GPU with NVENC support
- CUDA toolkit
- nvidia-docker runtime

**Performance:**
- 5-10x faster than CPU encoding
- Lower power consumption
- Supports multiple concurrent streams

## Testing Components

### Integration Tests

**Location:** `internal/modules/playbackmodule/integration_test.go`

**Purpose:** End-to-end testing of transcoding pipeline with content storage.

**Test Coverage:**
- Pipeline execution
- Content deduplication
- Hash generation consistency
- Manifest validation
- Error recovery

### Frontend Tests

**Location:** `frontend/src/components/MediaPlayer/MediaPlayer.test.tsx`

**Test Coverage:**
- Content-addressable URL handling
- Playback state management
- Seek functionality
- Error states

### Cypress E2E Tests

**Location:** `frontend/cypress/e2e/`

**Test Scenarios:**
- DASH/HLS playback
- Content deduplication verification
- Seek-ahead functionality
- Network request monitoring
- Legacy URL migration

## Utility Components

### Hash Generator

**Location:** `sdk/transcoding/utils/hash.go`

**Purpose:** Generates deterministic SHA256 hashes for content identification.

```go
func GenerateContentHash(components ...string) string {
    // Sort components for consistency
    sort.Strings(components)
    
    // Create hash
    h := sha256.New()
    h.Write([]byte(strings.Join(components, "-")))
    
    return hex.EncodeToString(h.Sum(nil))
}
```

### URL Generator

**Location:** `sdk/transcoding/storage/url_generator.go`

**Purpose:** Creates CDN-friendly URLs for content access.

```go
type URLGenerator struct {
    baseURL string
}

func (u *URLGenerator) ManifestURL(hash, container string) string {
    if container == "hls" {
        return fmt.Sprintf("%s/%s/playlist.m3u8", u.baseURL, hash)
    }
    return fmt.Sprintf("%s/%s/manifest.mpd", u.baseURL, hash)
}
```