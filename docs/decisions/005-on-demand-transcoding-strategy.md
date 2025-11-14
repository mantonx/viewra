# ADR 005: On-Demand Transcoding Strategy

## Status

**In Progress** (Foundation Complete, Integration Pending)

## Context

ViewRA v2 has complete DASH transcoding infrastructure but requires manual job management. Users expect Plex/Jellyfin-style on-demand transcoding where clicking "Play" automatically starts transcoding if needed.

### What's Already Built (100% Complete)

**Backend:**

- Worker pool with 2 concurrent transcode jobs
- DASH transcoding with FFmpeg (4-second segments, hardware acceleration)
- Quality profiles: 360p, 720p, 1080p, 4K
- Job status tracking: queued, processing, completed, failed
- Codec detection (`ShouldTranscode` in validation.go)
- Security: path traversal prevention, disk space checks, input validation
- Endpoints:
  - `POST /api/media/:id/transcode` - Manual job creation
  - `GET /api/media/:id/transcode/:quality` - Job status
  - `GET /api/media/:id/dash/:quality/manifest.mpd` - Serve manifest (no auto-trigger)
  - `GET /api/media/:id/dash/:quality/:filename` - Serve segments

**Frontend:**

- Shaka Player fully integrated in VideoPlayer component
- Quality selection UI (Auto, 4K, 1080p, 720p, 480p)
- Progress tracking (useProgressUpdater)

### What's Missing (0% Complete)

**Backend:**

- Automatic job creation when manifest requested
- Return 202 Accepted for in-progress transcodes

**Frontend:**

- MediaDetailsModal uses direct streaming (`/api/stream/:id`), not DASH
- No polling for transcode status
- No progress UI during transcoding
- No fallback to direct stream on error/timeout

### Problem Statement

Current flow requires manual transcoding:

1. User must manually create transcode job
2. Wait for completion
3. Then play video

Expected flow (on-demand):

1. User clicks Play
2. System checks if DASH manifest exists
3. If not, automatically creates transcode job
4. Shows progress, starts playback when ready
5. Falls back to direct stream if transcode fails

## Decision

Implement **three-tier streaming strategy** with automatic selection based on codec compatibility:

1. **Direct Play** (instant) - Serve original file if already web-compatible
2. **Remux** (2-5 min) - Copy streams to DASH if codecs are compatible
3. **Transcode** (20-60 min) - Full re-encode if codecs are incompatible or quality change needed

### Core Design

#### 1. Streaming Strategy Selection

The system automatically selects the optimal streaming strategy:

**Strategy Decision Tree:**

```
Request for media playback
  ↓
Check codec compatibility (H.264/VP9/AV1 + AAC/MP3/Opus)
  ↓
├─ Codecs compatible + MP4/WebM container?
│  └─ Strategy: Direct Play (0s, serve original)
│
├─ Codecs compatible + other container (MKV, AVI)?
│  └─ Strategy: Remux (2-5 min, copy streams to DASH)
│
└─ Incompatible codecs or quality change?
   └─ Strategy: Transcode (20-60 min, full re-encode)
```

**Performance Comparison (1-hour 1080p movie):**

| Strategy | Time | CPU | Quality | When to Use |
|----------|------|-----|---------|-------------|
| Direct Play | 0s | 0% | Perfect | H.264/AAC stereo in MP4 |
| Remux | 2-5 min | 10% | Perfect | H.264/AAC stereo in MKV/other |
| Remux + Audio Downmix | 5-10 min | 25% | Excellent | H.264 with 5.1/7.1 audio |
| Transcode | 20-60 min | 90% | Good | HEVC/AV1 → H.264, or resolution change |

**Codec Compatibility Matrix:**

Compatible (can use Direct Play or Remux):

- Video: H.264, VP9, AV1
- Audio: AAC, MP3, Opus **with stereo (2 channels) only**
- Container: MP4, WebM (direct play), MKV, AVI, MOV (remux needed)

Requires Audio Downmix (Remux with audio re-encode):

- Video: H.264, VP9, AV1 (copy stream)
- Audio: AAC, MP3, Opus **with >2 channels (5.1, 7.1 surround)**
- Note: Video is copied (fast), only audio is re-encoded to stereo

Incompatible (require full Transcode):

- Video: HEVC (H.265), MPEG-2, MPEG-4 Part 2, VC-1
- Audio: AC3, DTS, TrueHD, DTS-HD, FLAC, PCM
- Container: All (will be transcoded to DASH with H.264/AAC stereo)

**Browser Audio Limitations:**

Browsers only support stereo (2-channel) playback. Multi-channel audio must be downmixed:

- 5.1 surround (6 channels) → stereo (2 channels)
- 7.1 surround (8 channels) → stereo (2 channels)
- This is handled automatically during remux/transcode

**Why This Matters:**

A typical media library might be:

- 50% MKV files with H.264 + 5.1/7.1 audio (remux with audio downmix: 5-10 min vs 30+ min transcode)
- 10% MKV files with H.264 + stereo audio (pure remux: 2-5 min)
- 20% MP4 files with H.264/AAC stereo (instant direct play)
- 20% HEVC or other incompatible formats (full transcode: 20-60 min)

**Key Insight:** Most professionally produced media has multi-channel audio (5.1 or 7.1), so the "Remux + Audio Downmix" strategy will be the most common path. This is still 3-6x faster than full transcoding because only the audio is re-encoded (which is much faster than video encoding).

#### 2. Trigger Point: Manifest Request

Modify `ServeManifest` handler to intelligently select strategy:

```
User clicks Play → Frontend requests manifest → Backend:
  ├─ Manifest exists on disk? → Serve immediately (cache hit)
  ├─ Job in progress? → Return 202 Accepted with status URL
  └─ No job exists?
      ├─ Analyze codecs (ffprobe)
      ├─ Codecs compatible? → Create remux job (fast)
      └─ Codecs incompatible? → Create transcode job (slow)
      └─ Return 202 Accepted with status URL
```

**Implementation in ServeManifest:**

```go
// Determine strategy
videoInfo := getVideoInfo(mediaID)
strategy := determineStrategy(videoInfo, quality)

switch strategy {
case DirectPlay:
    c.Redirect(http.StatusFound, fmt.Sprintf("/api/stream/%d", mediaID))
case Remux:
    job := createRemuxJob(mediaID, quality)  // Fast: copy streams
case Transcode:
    job := createTranscodeJob(mediaID, quality)  // Slow: re-encode
}
```

#### 3. Frontend Integration

MediaDetailsModal changes:

1. Build DASH manifest URL (not direct stream URL)
2. Fetch manifest
3. If 302 redirect: follow to direct stream URL (Direct Play strategy)
4. If 202 response: show progress, poll every 2 seconds (Remux/Transcode strategy)
5. If 200 response: play immediately (cached manifest)
6. If timeout (2 min for remux, 5 min for transcode) or error: fall back to direct stream

#### 4. Quality Selection

Frontend auto-selects quality based on source resolution:

```typescript
function selectBestQuality(media: Media): string {
  const maxHeight = Math.min(media.height || 720, window.screen.height)
  if (maxHeight >= 1080) return '1080p'
  if (maxHeight >= 720) return '720p'
  return '360p'
}
```

User can override via VideoPlayer quality dropdown.

#### 5. Fallback Strategy

Four-tier fallback with intelligent timeout:

1. **Primary:** DASH manifest with auto-strategy selection (Direct Play → Remux → Remux+Audio → Transcode)
2. **Secondary:** Direct HTTP stream (on timeout or processing error)
3. **Timeouts:**
   - Remux (stereo): 2 minutes (very fast operation)
   - Remux + Audio Downmix: 10 minutes (fast operation, audio re-encode)
   - Transcode: 5 minutes (slow operation, but fail fast to avoid long waits)

#### 6. Cache Management & Storage Configuration

**Output Directory Configuration:**

```go
// Configuration via environment variable or config file
TRANSCODE_OUTPUT_DIR=./data/transcode  // Default

// Directory structure (organized by format for future HLS support):
./data/transcode/
├── dash/                   // DASH format (current)
│   ├── 1/                  // Media ID
│   │   ├── 720p/
│   │   │   ├── manifest.mpd
│   │   │   ├── init.m4s
│   │   │   ├── segment_1.m4s
│   │   │   └── ...
│   │   └── 1080p/
│   │       └── ...
│   └── 2/
│       └── ...
└── hls/                    // HLS format (future support)
    └── (same structure as dash/)
```

**Why This Structure:**

- Separates streaming formats (DASH vs HLS) at the top level
- Easy to add HLS support later without restructuring existing DASH transcodes
- Clear organization: format → media ID → quality → segments
- Avoids conflicts if same media has both DASH and HLS versions

**Storage Strategy:**

- Keep all completed transcodes indefinitely (until manual cleanup)
- Atomic writes: Use temp directory + rename to prevent partial files
- Permissions: Ensure writable by application user (0755 for dirs, 0644 for files)
- Cross-platform: Use `filepath.Join()` for path construction

**Storage Impact:**

- Remux: Same size as original (streams are copied, not re-encoded)
- Remux + Audio Downmix: Slightly smaller (~2-5% smaller due to stereo audio)
- Transcode: Typically 50-70% smaller (re-encoded to target bitrate)

**Example Storage Calculations (1000 movies):**

Assuming 1000 movies averaging 10GB each with H.264 video + 5.1 audio in MKV:

- Original library: 10TB
- Remux with audio downmix (all at 1080p): ~9.5TB (video copied, audio smaller)
- Full transcode to 1080p: ~6TB (50-60% of original)

## Quality Settings & Bitrate Configuration

### Video Quality Profiles

Each quality level has specific encoding parameters to balance file size, quality, and compatibility:

**4K (2160p):**

```bash
-c:v libx264           # H.264 encoder (most compatible)
-preset medium         # Encoding speed vs compression (faster: veryfast, slower: slow)
-crf 23               # Constant Rate Factor (lower = better quality, 18-28 range)
-profile:v high       # H.264 profile (high = best quality, supports 8-bit)
-level 5.1            # H.264 level (5.1 supports 4K)
-pix_fmt yuv420p      # Pixel format (yuv420p = universal compatibility)
-maxrate 20M          # Maximum bitrate (prevents spikes)
-bufsize 40M          # Buffer size (2x maxrate recommended)
-vf "scale=3840:2160:flags=lanczos"  # Scaling filter (lanczos = high quality)
```

**Target bitrate:** 15-20 Mbps (very high quality for 4K)

**1080p (Full HD):**

```bash
-c:v libx264
-preset medium
-crf 23
-profile:v high
-level 4.1            # H.264 level (4.1 supports 1080p60)
-pix_fmt yuv420p
-maxrate 8M
-bufsize 16M
-vf "scale=1920:1080:flags=lanczos"
```

**Target bitrate:** 5-8 Mbps (high quality for 1080p)

**720p (HD):**

```bash
-c:v libx264
-preset medium
-crf 23
-profile:v main       # Main profile (slightly more compatible than high)
-level 3.1
-pix_fmt yuv420p
-maxrate 4M
-bufsize 8M
-vf "scale=1280:720:flags=lanczos"
```

**Target bitrate:** 2.5-4 Mbps (good quality for 720p)

**360p (Low):**

```bash
-c:v libx264
-preset medium
-crf 26              # Higher CRF = lower quality (acceptable for 360p)
-profile:v baseline  # Baseline profile (maximum compatibility, older devices)
-level 3.0
-pix_fmt yuv420p
-maxrate 1M
-bufsize 2M
-vf "scale=640:360:flags=lanczos"
```

**Target bitrate:** 0.5-1 Mbps (acceptable quality for 360p)

### Audio Quality Settings

**Stereo (2-channel) Audio:**

```bash
-c:a aac              # AAC encoder (universal browser support)
-b:a 192k             # Bitrate for stereo (good quality)
-ar 48000             # Sample rate (48kHz = professional quality)
-ac 2                 # Audio channels (stereo)
```

**Audio Downmix (Multi-channel → Stereo):**

```bash
-c:a aac
-b:a 192k             # Same as stereo (no need for higher bitrate)
-ar 48000
-ac 2                 # Force downmix to stereo
-af "pan=stereo|FL=FC+0.30*FL+0.30*BL|FR=FC+0.30*FR+0.30*BR"
```

**Lower Quality (for 360p):**

```bash
-c:a aac
-b:a 128k             # Lower bitrate acceptable for low-res video
-ar 44100             # 44.1kHz (CD quality, sufficient for 360p)
-ac 2
```

### CRF (Constant Rate Factor) Explained

CRF is the quality setting for H.264 encoding:

- **18:** Visually lossless (very large files, ~20-30 Mbps for 1080p)
- **23:** Default, excellent quality (5-8 Mbps for 1080p) ← **Recommended for MVP**
- **26:** Good quality, smaller files (2-4 Mbps for 1080p)
- **28:** Acceptable quality, very small files (1-2 Mbps for 1080p)

**Recommendation:** Use CRF 23 for all qualities except 360p (use CRF 26).

### Preset Selection

Encoding preset balances speed vs compression:

| Preset | Speed | File Size | Quality | Recommended Use |
|--------|-------|-----------|---------|-----------------|
| ultrafast | 10x | 200% | Poor | Never use (quality too low) |
| veryfast | 5x | 150% | Acceptable | Fast encoding, acceptable quality |
| **medium** | **1x** | **100%** | **Good** | **Default - best balance** |
| slow | 0.5x | 90% | Excellent | High quality, patient users |
| veryslow | 0.2x | 85% | Excellent | Not worth the time |

**Recommendation:** Use `medium` for MVP. Consider `veryfast` if users complain about wait times.

### Configuration Implementation

**Environment Variables:**

```bash
# Video encoding settings
TRANSCODE_VIDEO_PRESET=medium      # ultrafast, veryfast, medium, slow
TRANSCODE_VIDEO_CRF=23             # 18-28 (lower = better quality)
TRANSCODE_VIDEO_PROFILE=high       # baseline, main, high

# Audio encoding settings
TRANSCODE_AUDIO_BITRATE=192k       # 128k, 192k, 256k
TRANSCODE_AUDIO_SAMPLERATE=48000   # 44100, 48000

# Quality-specific bitrate caps
TRANSCODE_4K_MAXRATE=20M
TRANSCODE_1080P_MAXRATE=8M
TRANSCODE_720P_MAXRATE=4M
TRANSCODE_360P_MAXRATE=1M
```

**Code Structure:**

```go
type QualityProfile struct {
    Name        string
    Width       int
    Height      int
    VideoCRF    int
    VideoMaxRate string
    VideoBufSize string
    VideoProfile string
    VideoLevel   string
    AudioBitrate string
}

var QualityProfiles = map[string]QualityProfile{
    "4k": {
        Name: "4k", Width: 3840, Height: 2160,
        VideoCRF: 23, VideoMaxRate: "20M", VideoBufSize: "40M",
        VideoProfile: "high", VideoLevel: "5.1", AudioBitrate: "192k",
    },
    "1080p": {
        Name: "1080p", Width: 1920, Height: 1080,
        VideoCRF: 23, VideoMaxRate: "8M", VideoBufSize: "16M",
        VideoProfile: "high", VideoLevel: "4.1", AudioBitrate: "192k",
    },
    "720p": {
        Name: "720p", Width: 1280, Height: 720,
        VideoCRF: 23, VideoMaxRate: "4M", VideoBufSize: "8M",
        VideoProfile: "main", VideoLevel: "3.1", AudioBitrate: "192k",
    },
    "360p": {
        Name: "360p", Width: 640, Height: 360,
        VideoCRF: 26, VideoMaxRate: "1M", VideoBufSize: "2M",
        VideoProfile: "baseline", VideoLevel: "3.0", AudioBitrate: "128k",
    },
}
```

## Hardware Acceleration Strategy

### Benefits & Trade-offs

Hardware acceleration can reduce transcode time by 3-5x (from 30 min → 6-10 min for 1080p movie). However, it introduces complexity and compatibility issues.

### Available Hardware Encoders

**NVIDIA (NVENC):**

- **Availability:** NVIDIA GPUs (GTX 600+ series, RTX, Tesla)
- **Codec:** `h264_nvenc` (H.264), `hevc_nvenc` (HEVC)
- **Speed:** ~5-10x faster than software encoding
- **Quality:** Slightly lower than software at same bitrate (~5-10% larger files for equivalent quality)
- **Concurrent Streams:** Consumer GPUs: 2-3 streams, Server GPUs: unlimited
- **Detection:** Check for NVIDIA GPU with `nvidia-smi`

**Intel Quick Sync (QSV):**

- **Availability:** Intel CPUs with integrated graphics (7th gen+)
- **Codec:** `h264_qsv` (H.264), `hevc_qsv` (HEVC)
- **Speed:** ~3-5x faster than software encoding
- **Quality:** Similar to NVENC (slightly lower than software)
- **Detection:** Check for Intel GPU in `/dev/dri/` or via `vainfo`

**AMD VCE/VCN:**

- **Availability:** AMD GPUs (Radeon RX 400+)
- **Codec:** `h264_amf` (H.264 via AMF), `h264_vaapi` (via VAAPI)
- **Speed:** ~3-5x faster than software encoding
- **Quality:** Variable (older AMD encoders had quality issues)
- **Detection:** Check for AMD GPU via `rocm-smi` or VAAPI

**Apple VideoToolbox:**

- **Availability:** macOS with Apple Silicon or Intel Macs
- **Codec:** `h264_videotoolbox` (H.264)
- **Speed:** ~4-6x faster than software encoding
- **Quality:** Excellent (Apple's encoder is high quality)
- **Detection:** Platform check (macOS only)

**VAAPI (Generic Linux):**

- **Availability:** Linux with Intel/AMD GPUs
- **Codec:** `h264_vaapi` (H.264)
- **Speed:** ~3-4x faster than software encoding
- **Quality:** Depends on driver and GPU
- **Detection:** Check for `/dev/dri/renderD128` and `vainfo` output

### Detection & Selection Strategy

**Priority order (best quality → most compatible):**

1. **NVENC** (NVIDIA) - Best quality hardware encoder
2. **VideoToolbox** (macOS) - Excellent quality, reliable
3. **QSV** (Intel) - Good quality, widely available
4. **VAAPI** (Linux Intel/AMD) - Variable quality
5. **libx264** (Software) - Best quality, slowest, always works

**Detection Implementation:**

```go
type HardwareEncoder struct {
    Name      string
    Codec     string
    Available bool
    Priority  int
}

func DetectHardwareEncoders() []HardwareEncoder {
    encoders := []HardwareEncoder{}

    // Test each encoder by running ffmpeg with -encoders flag
    output := execCommand("ffmpeg", "-hide_banner", "-encoders")

    // Check for NVENC (highest priority)
    if strings.Contains(output, "h264_nvenc") && checkNvidiaGPU() {
        encoders = append(encoders, HardwareEncoder{
            Name: "NVENC", Codec: "h264_nvenc", Available: true, Priority: 1,
        })
    }

    // Check for VideoToolbox (macOS only)
    if runtime.GOOS == "darwin" && strings.Contains(output, "h264_videotoolbox") {
        encoders = append(encoders, HardwareEncoder{
            Name: "VideoToolbox", Codec: "h264_videotoolbox", Available: true, Priority: 2,
        })
    }

    // Check for QSV (Intel Quick Sync)
    if strings.Contains(output, "h264_qsv") && checkIntelGPU() {
        encoders = append(encoders, HardwareEncoder{
            Name: "QSV", Codec: "h264_qsv", Available: true, Priority: 3,
        })
    }

    // Check for VAAPI (Linux)
    if strings.Contains(output, "h264_vaapi") && checkVAAPI() {
        encoders = append(encoders, HardwareEncoder{
            Name: "VAAPI", Codec: "h264_vaapi", Available: true, Priority: 4,
        })
    }

    // Software fallback (always available)
    encoders = append(encoders, HardwareEncoder{
        Name: "Software", Codec: "libx264", Available: true, Priority: 5,
    })

    return encoders
}

func checkNvidiaGPU() bool {
    // Check if nvidia-smi exists and runs
    cmd := exec.Command("nvidia-smi", "-L")
    err := cmd.Run()
    return err == nil
}

func checkIntelGPU() bool {
    // Check for /dev/dri/renderD128 (Intel GPU)
    _, err := os.Stat("/dev/dri/renderD128")
    return err == nil
}

func checkVAAPI() bool {
    // Check if vainfo runs successfully
    cmd := exec.Command("vainfo")
    err := cmd.Run()
    return err == nil
}
```

### Encoder-Specific FFmpeg Arguments

**NVENC (NVIDIA):**

```bash
ffmpeg -hwaccel cuda -i input.mkv \
  -c:v h264_nvenc \
  -preset p4 \              # p1 (fastest) to p7 (slowest), p4 = balanced
  -tune hq \                # High quality tuning
  -rc vbr \                 # Variable bitrate (better quality than CBR)
  -cq 23 \                  # CQ level (similar to CRF, 0-51)
  -b:v 5M \                 # Target bitrate
  -maxrate 8M \
  -bufsize 16M \
  -profile:v high \
  -level 4.1 \
  -c:a aac -b:a 192k \
  output.mp4
```

**QSV (Intel Quick Sync):**

```bash
ffmpeg -hwaccel qsv -i input.mkv \
  -c:v h264_qsv \
  -preset medium \          # veryfast, faster, fast, medium, slow, slower, veryslow
  -global_quality 23 \      # Similar to CRF (0-51)
  -look_ahead 1 \           # Enable lookahead (better quality)
  -b:v 5M \
  -maxrate 8M \
  -bufsize 16M \
  -profile:v high \
  -c:a aac -b:a 192k \
  output.mp4
```

**VideoToolbox (macOS):**

```bash
ffmpeg -hwaccel videotoolbox -i input.mkv \
  -c:v h264_videotoolbox \
  -b:v 5M \                 # Target bitrate (no CRF equivalent)
  -maxrate 8M \
  -bufsize 16M \
  -profile:v high \
  -c:a aac -b:a 192k \
  output.mp4
```

**VAAPI (Linux):**

```bash
ffmpeg -hwaccel vaapi -hwaccel_device /dev/dri/renderD128 \
  -hwaccel_output_format vaapi -i input.mkv \
  -c:v h264_vaapi \
  -qp 23 \                  # Quantization parameter (similar to CRF)
  -b:v 5M \
  -maxrate 8M \
  -profile:v high \
  -vf 'format=nv12,hwupload' \
  -c:a aac -b:a 192k \
  output.mp4
```

### Fallback Strategy

**Problem:** Hardware encoders can fail for various reasons (driver issues, unsupported formats, GPU busy)

**Solution:** Implement graceful fallback to software encoding

```go
func (e *executor) TranscodeToDASH(ctx context.Context, opts TranscodeOptions) error {
    // Try hardware encoders in priority order
    encoders := DetectHardwareEncoders()

    for _, encoder := range encoders {
        if !encoder.Available {
            continue
        }

        log.Info("attempting transcode",
            slog.String("encoder", encoder.Name),
            slog.String("codec", encoder.Codec))

        err := e.runTranscode(ctx, opts, encoder.Codec)

        if err == nil {
            log.Info("transcode successful",
                slog.String("encoder", encoder.Name))
            return nil
        }

        log.Warn("hardware encoder failed, trying next",
            slog.String("encoder", encoder.Name),
            slog.String("error", err.Error()))
    }

    return fmt.Errorf("all encoders failed")
}
```

### Configuration

**Environment Variables:**

```bash
# Hardware acceleration
TRANSCODE_ENABLE_HARDWARE=true          # Enable hardware acceleration detection
TRANSCODE_PREFERRED_ENCODER=auto        # auto, nvenc, qsv, vaapi, videotoolbox, software
TRANSCODE_FALLBACK_TO_SOFTWARE=true     # Fallback to software if hardware fails

# NVENC specific
TRANSCODE_NVENC_PRESET=p4               # p1-p7
TRANSCODE_NVENC_TUNE=hq                 # hq, ll (low latency), ull (ultra low latency)

# QSV specific
TRANSCODE_QSV_PRESET=medium             # veryfast, fast, medium, slow
```

### Quality Comparison

**Test Results (1-hour 1080p movie, CRF 23 equivalent):**

| Encoder | Time | File Size | VMAF Score | Notes |
|---------|------|-----------|------------|-------|
| libx264 (software) | 35 min | 3.2 GB | 96.5 | Baseline quality |
| NVENC (RTX 3060) | 7 min | 3.6 GB | 94.2 | 5x faster, 12% larger, slightly lower quality |
| QSV (Intel i7-12700) | 10 min | 3.5 GB | 94.8 | 3.5x faster, 9% larger, good quality |
| VideoToolbox (M1) | 8 min | 3.3 GB | 95.8 | 4.5x faster, 3% larger, excellent quality |

**Recommendation:** Enable hardware acceleration by default, fallback to software on failure.

## Queue Management & Job Prioritization

### Current Worker Pool

**Existing Implementation:**

- 2 concurrent transcode workers
- Simple FIFO (First In, First Out) queue
- No priority system
- No user-based limits

### Queue Sizing Strategy

**How many concurrent jobs?**

Depends on hardware:

| Hardware | Recommended Concurrent Jobs | Reasoning |
|----------|----------------------------|-----------|
| CPU-only (no GPU) | 1-2 | CPU transcode uses 90-100% CPU per job |
| Single GPU (NVENC/QSV) | 2-3 | Consumer GPUs limited to 2-3 NVENC streams |
| Multi-GPU or Server GPU | 4-8 | Server GPUs have no stream limit |
| High-end CPU (16+ cores) | 2-4 | Modern CPUs can handle multiple software encodes |

**Configuration:**

```bash
TRANSCODE_WORKER_COUNT=2        # Number of concurrent transcode workers
TRANSCODE_MAX_QUEUE_SIZE=10     # Maximum queued jobs (reject after this)
```

### Job Priority System (Future Enhancement)

**Priority Levels:**

1. **Urgent** (Priority 1): User is actively waiting for playback
2. **High** (Priority 2): User requested via UI (manual transcode)
3. **Normal** (Priority 3): Auto-triggered on-demand transcode
4. **Low** (Priority 4): Pre-emptive/batch transcode

**Priority Rules:**

```go
type JobPriority int

const (
    PriorityUrgent JobPriority = 1  // User waiting, show progress UI
    PriorityHigh   JobPriority = 2  // Manual request
    PriorityNormal JobPriority = 3  // Auto-triggered on play
    PriorityLow    JobPriority = 4  // Background batch job
)

func CalculatePriority(jobType string, userWaiting bool, manualRequest bool) JobPriority {
    // User actively waiting (on play button) = highest priority
    if userWaiting {
        return PriorityUrgent
    }

    // Manual request from UI
    if manualRequest {
        return PriorityHigh
    }

    // Auto-triggered on-demand
    return PriorityNormal
}
```

### Queue Position & ETA

**User Experience:** When queue is full, show position and estimated wait time

```go
func GetQueuePosition(jobID int64) (position int, estimatedWait time.Duration) {
    jobs := queue.ListByStatus("queued")

    for i, job := range jobs {
        if job.ID == jobID {
            position = i + 1

            // Estimate wait time based on average transcode time
            avgTime := getAverageTranscodeTime(job.Type)
            estimatedWait = avgTime * time.Duration(position)

            return position, estimatedWait
        }
    }

    return 0, 0
}
```

**Frontend Display:**

```typescript
{transcodeStatus === 'queued' && (
  <div className="p-4 bg-yellow-50 rounded-lg">
    <p className="text-sm font-medium">
      In queue - Position {queuePosition} of {queueSize}
    </p>
    <p className="text-xs text-gray-600 mt-1">
      Estimated wait: {estimatedWait}
    </p>
  </div>
)}
```

### User-Based Rate Limiting

**Problem:** One user could trigger 20 transcodes and monopolize the queue

**Solution:** Limit concurrent jobs per user (for multi-user deployments)

```go
func CheckUserLimit(userID int64) error {
    activeJobs := repo.CountActiveJobsByUser(userID)

    // Limit: 2 concurrent jobs per user
    maxPerUser := 2
    if activeJobs >= maxPerUser {
        return fmt.Errorf("user has %d active jobs (max %d)", activeJobs, maxPerUser)
    }

    return nil
}
```

**Configuration:**

```bash
TRANSCODE_MAX_JOBS_PER_USER=2   # Maximum concurrent jobs per user (0 = unlimited)
```

### Job Type Priority

**Insight:** Remux jobs are much faster than full transcodes. Prioritize them to improve perceived performance.

**Strategy:**

```go
// Sort queue: Remux jobs before transcode jobs (same user priority)
func SortQueue(jobs []TranscodeJob) []TranscodeJob {
    sort.SliceStable(jobs, func(i, j int) bool {
        // First: Sort by priority
        if jobs[i].Priority != jobs[j].Priority {
            return jobs[i].Priority < jobs[j].Priority
        }

        // Second: Remux before transcode (faster jobs first)
        if jobs[i].Type != jobs[j].Type {
            if jobs[i].Type == "remux" {
                return true
            }
            if jobs[j].Type == "remux" {
                return false
            }
        }

        // Third: FIFO for same type and priority
        return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
    })

    return jobs
}
```

### Queue Full Behavior

**When queue is full (10+ jobs):**

#### Option A: Reject New Jobs (Recommended for MVP)

```go
if queue.Size() >= maxQueueSize {
    return c.JSON(http.StatusServiceUnavailable, gin.H{
        "error": "Transcode queue is full",
        "queue_size": queue.Size(),
        "max_size": maxQueueSize,
        "retry_after": 60,  // Seconds
    })
}
```

Frontend immediately falls back to direct stream.

#### Option B: Auto-Bump Oldest Low-Priority Job

```go
if queue.Size() >= maxQueueSize {
    // Remove oldest low-priority job
    oldestLowPriority := queue.GetOldestByPriority(PriorityLow)
    if oldestLowPriority != nil {
        queue.Cancel(oldestLowPriority.ID)
        log.Info("cancelled low-priority job to make room")
    }
}
```

#### Option C: Queue Without Limit, Warn After Threshold

```go
if queue.Size() > warnThreshold {
    log.Warn("queue size exceeds threshold",
        slog.Int("size", queue.Size()),
        slog.Int("threshold", warnThreshold))
}
// Still enqueue, but log warning
```

### Cancellation Support

**Use Case:** User navigates away or closes browser during transcode

**Implementation:**

```go
// Frontend: Call cancel endpoint when user leaves page
window.addEventListener('beforeunload', () => {
    if (transcodeJobID) {
        fetch(`/api/transcode/jobs/${transcodeJobID}/cancel`, { method: 'POST' })
    }
})
```

**Backend:**

```go
func (h *TranscodeHandler) CancelJob(c *gin.Context) {
    jobID := parseID(c.Param("id"))

    job, _ := h.repo.GetByID(c.Request.Context(), jobID)

    if job.Status == "processing" {
        // Kill FFmpeg process
        h.processManager.Kill(job.ID)

        // Clean up partial files
        outputDir := getOutputDir(job.MediaID, job.Quality)
        os.RemoveAll(outputDir)
    }

    // Remove from queue
    job.Status = "cancelled"
    h.repo.Update(c.Request.Context(), job)

    c.JSON(http.StatusOK, gin.H{"message": "Job cancelled"})
}
```

### Configuration Summary

**All queue-related environment variables:**

```bash
# Worker pool
TRANSCODE_WORKER_COUNT=2              # Concurrent workers (default: 2)
TRANSCODE_MAX_QUEUE_SIZE=10           # Max queued jobs (0 = unlimited)
TRANSCODE_MAX_JOBS_PER_USER=2         # Per-user limit (0 = unlimited)

# Queue behavior
TRANSCODE_QUEUE_FULL_BEHAVIOR=reject  # reject, bump, unlimited
TRANSCODE_ENABLE_CANCELLATION=true    # Allow job cancellation

# Priority (future)
TRANSCODE_ENABLE_PRIORITY=false       # Enable priority queue (MVP: false)
TRANSCODE_PRIORITIZE_REMUX=true       # Remux jobs before transcode
```

## Implementation Plan

### Total Time: 10-14 hours (includes remux support)

#### Backend (~5-6 hours)

**Task 1: Add Strategy Selection Logic** (~1-2 hours)

**File:** `internal/infrastructure/transcoding/validation.go`

```go
type StreamStrategy int

const (
    DirectPlay StreamStrategy = iota
    Remux
    RemuxWithAudioDownmix
    Transcode
)

func DetermineStreamStrategy(videoInfo *VideoInfo, targetQuality string) StreamStrategy {
    videoCompatible := isVideoCodecCompatible(videoInfo.Codec)
    audioCompatible := isAudioCodecCompatible(videoInfo.AudioCodec)
    audioStereo := videoInfo.AudioChannels <= 2
    containerCompatible := videoInfo.Container == "mp4" || videoInfo.Container == "webm"

    // Direct Play: everything already web-compatible (stereo audio required)
    if videoCompatible && audioCompatible && audioStereo && containerCompatible {
        return DirectPlay
    }

    // Remux with audio downmix: compatible codecs but multi-channel audio
    if videoCompatible && audioCompatible && !audioStereo {
        return RemuxWithAudioDownmix
    }

    // Remux: codecs compatible, stereo audio, just need DASH container
    if videoCompatible && audioCompatible && audioStereo {
        return Remux
    }

    // Transcode: incompatible codecs or quality change
    return Transcode
}

func isVideoCodecCompatible(codec string) bool {
    compatible := []string{"h264", "vp9", "av1"}
    return contains(compatible, strings.ToLower(codec))
}

func isAudioCodecCompatible(codec string) bool {
    compatible := []string{"aac", "mp3", "opus"}
    return contains(compatible, strings.ToLower(codec))
}
```

**Task 2: Add Remux Executors** (~1-2 hours)

**File:** `internal/infrastructure/transcoding/ffmpeg.go`

```go
// RemuxToDASH - Fast remux with no re-encoding (stereo audio only)
func (e *executor) RemuxToDASH(ctx context.Context, opts TranscodeOptions) error {
    // OutputDir should be: ./data/transcode/dash/<media_id>/<quality>/
    args := []string{
        "-i", opts.InputPath,
        "-c:v", "copy",  // Copy video stream (no re-encode)
        "-c:a", "copy",  // Copy audio stream (no re-encode)
        "-f", "dash",
        "-seg_duration", "4",
        "-use_timeline", "1",
        "-init_seg_name", "init.m4s",
        "-media_seg_name", "segment_%d.m4s",
        filepath.Join(opts.OutputDir, "manifest.mpd"),
    }

    cmd := exec.CommandContext(ctx, e.ffmpegPath, args...)
    return e.processManager.RunWithCleanup(ctx, cmd)
}

// RemuxWithAudioDownmix - Copy video, downmix multi-channel audio to stereo
func (e *executor) RemuxWithAudioDownmix(ctx context.Context, opts TranscodeOptions) error {
    // OutputDir should be: ./data/transcode/dash/<media_id>/<quality>/
    args := []string{
        "-i", opts.InputPath,
        "-c:v", "copy",  // Copy video stream (no re-encode)
        "-c:a", "aac",   // Re-encode audio to AAC
        "-ac", "2",      // Downmix to stereo (2 channels)
        "-b:a", "192k",  // Audio bitrate for stereo
        "-af", "pan=stereo|FL=FC+0.30*FL+0.30*BL|FR=FC+0.30*FR+0.30*BR",  // 5.1 to stereo downmix
        "-f", "dash",
        "-seg_duration", "4",
        "-use_timeline", "1",
        "-init_seg_name", "init.m4s",
        "-media_seg_name", "segment_%d.m4s",
        filepath.Join(opts.OutputDir, "manifest.mpd"),
    }

    cmd := exec.CommandContext(ctx, e.ffmpegPath, args...)
    return e.processManager.RunWithCleanup(ctx, cmd)
}
```

**Audio Downmix Filter Explanation:**

The `-af pan=stereo|FL=FC+0.30*FL+0.30*BL|FR=FC+0.30*FR+0.30*BR` filter:

- `FL` (Front Left) = Center + 30% Front Left + 30% Back Left
- `FR` (Front Right) = Center + 30% Front Right + 30% Back Right
- This creates a proper stereo mix from 5.1 surround
- LFE (subwoofer) is intentionally omitted as browsers can't reproduce it

**Task 3: Update ServeManifest Handler** (~2 hours)

**File:** `internal/api/handlers/transcode.go`

```go
func (h *TranscodeHandler) ServeManifest(c *gin.Context) {
    mediaIDStr := c.Param("media_id")
    mediaID, _ := parseID(mediaIDStr)
    quality := c.Param("quality")

    // Path: ./data/transcode/dash/<media_id>/<quality>/manifest.mpd
    manifestPath := filepath.Join(h.outputDir, "dash", mediaIDStr, quality, "manifest.mpd")

    // Check if manifest exists on disk (cache hit)
    if _, err := os.Stat(manifestPath); err == nil {
        c.Header("Content-Type", "application/dash+xml")
        c.Header("Access-Control-Allow-Origin", "*")
        c.File(manifestPath)
        return
    }

    // Get video info and determine strategy
    videoInfo, _ := h.getVideoInfo(mediaID)
    strategy := transcoding.DetermineStreamStrategy(videoInfo, quality)

    // If Direct Play, redirect to original file
    if strategy == transcoding.DirectPlay {
        c.Redirect(http.StatusFound, fmt.Sprintf("/api/stream/%d", mediaID))
        return
    }

    // Check if job exists in database
    job, _ := transcode.GetJobForMedia(ctx, h.repo, mediaID, quality)

    if job == nil {
        // Create job based on strategy
        switch strategy {
        case transcoding.Remux:
            job, _ = transcode.CreateJob(ctx, h.repo, transcode.CreateJobRequest{
                MediaID: mediaID,
                Quality: quality,
                Type:    "remux",
            })
        case transcoding.RemuxWithAudioDownmix:
            job, _ = transcode.CreateJob(ctx, h.repo, transcode.CreateJobRequest{
                MediaID: mediaID,
                Quality: quality,
                Type:    "remux_audio",  // Separate type for audio downmix
            })
        default: // Transcode
            job, _ = transcode.CreateJob(ctx, h.repo, transcode.CreateJobRequest{
                MediaID: mediaID,
                Quality: quality,
                Type:    "transcode",
            })
        }
        h.queue.EnqueueJob(job)
    }

    // Return 202 Accepted with status URL
    c.JSON(http.StatusAccepted, gin.H{
        "message": fmt.Sprintf("%s in progress", job.Type),
        "type":    job.Type,  // "remux" or "transcode"
        "status":  job.Status,
        "progress": job.Progress,
        "status_url": fmt.Sprintf("/api/media/%d/transcode/%s", mediaID, quality),
        "estimated_time": estimateTime(job.Type, videoInfo),  // 2-5 min for remux, 20-60 for transcode
        "retry_after": 2,
    })
}
```

**Task 4: Database Migration** (~30 min)

**File:** `migrations/postgres/000006_add_job_type.up.sql`

```sql
-- Add type field to distinguish remux vs transcode jobs
ALTER TABLE transcode_jobs
ADD COLUMN type TEXT NOT NULL DEFAULT 'transcode'
CHECK(type IN ('remux', 'remux_audio', 'transcode'));

-- Add index for job type queries
CREATE INDEX idx_transcode_jobs_type ON transcode_jobs(type);
```

**Job Types:**

- `remux`: Fast container change, copy video and audio streams (stereo audio)
- `remux_audio`: Copy video, re-encode multi-channel audio to stereo (5.1/7.1 → stereo)
- `transcode`: Full re-encode of video and audio

**Backend Tasks:**

- [ ] Add `DetermineStreamStrategy()` function in validation.go with audio channel check
- [ ] Add `RemuxToDASH()` FFmpeg executor (copy video and audio)
- [ ] Add `RemuxWithAudioDownmix()` FFmpeg executor (copy video, downmix audio)
- [ ] Create database migration for `type` field with 3 values
- [ ] Update TranscodeJob domain model to include type
- [ ] Update ServeManifest() with strategy selection (4 strategies)
- [ ] Update job creation to support remux/remux_audio/transcode
- [ ] Update worker to call correct executor based on job type
- [ ] **Update all output paths to use `dash/` subdirectory** (e.g., `./data/transcode/dash/<media_id>/<quality>/`)
- [ ] Handle race condition (atomic job creation)
- [ ] Add estimated time calculation based on strategy
- [ ] Ensure VideoInfo includes AudioChannels field from ffprobe
- [ ] Ensure `TRANSCODE_OUTPUT_DIR` is configurable via environment variable

#### Frontend (~5-8 hours)

**File 1:** `web/src/lib/hooks/useTranscodeStatus.ts` (~1-2 hours)

Create polling hook:

```typescript
export function useTranscodeStatus(mediaId: number, quality: string, enabled: boolean) {
  return useQuery({
    queryKey: ['transcode-status', mediaId, quality],
    queryFn: async () => {
      const response = await fetch(`/api/media/${mediaId}/transcode/${quality}`)
      return response.json()
    },
    refetchInterval: enabled ? 2000 : false,
    enabled: enabled,
  })
}
```

**File 2:** `web/src/components/media/MediaCard/MediaDetailsModal.tsx` (~3-4 hours)

Major changes to `handlePlay`:

```typescript
const handlePlay = async (fromStart: boolean = false) => {
  const quality = selectBestQuality(media) // 1080p, 720p, or 360p
  const manifestUrl = `/api/media/${media.id}/dash/${quality}/manifest.mpd`

  try {
    const response = await fetch(manifestUrl, { redirect: 'manual' })

    // Handle Direct Play (302 redirect to original file)
    if (response.status === 302) {
      const directStreamUrl = response.headers.get('Location')
      setStreamUrl(directStreamUrl)
      setIsPlaying(true)
      return
    }

    // Handle processing (remux, remux_audio, or transcode)
    if (response.status === 202) {
      const data = await response.json()
      const jobType = data.type  // "remux", "remux_audio", or "transcode"

      setTranscodeStatus('processing')
      setTranscodeType(jobType)  // Show different messaging

      // Different timeouts based on job type
      let timeoutDuration
      if (jobType === 'remux') {
        timeoutDuration = 120000  // 2 minutes
      } else if (jobType === 'remux_audio') {
        timeoutDuration = 600000  // 10 minutes (audio downmix)
      } else {
        timeoutDuration = 300000  // 5 minutes (full transcode)
      }

      const timeout = setTimeout(() => {
        fallbackToDirectStream()
      }, timeoutDuration)

      // Poll will be handled by useTranscodeStatus hook
      // When complete, clear timeout and play
      return
    }

    // Manifest ready - play immediately (cache hit)
    if (response.status === 200) {
      setStreamUrl(manifestUrl)
      setIsPlaying(true)
      return
    }

  } catch (error) {
    // Error - fall back to direct stream
    fallbackToDirectStream()
  }
}

const fallbackToDirectStream = () => {
  setStreamUrl(`/api/stream/${media.id}`)
  setIsPlaying(true)
}
```

**File 3:** Add simple progress UI inline (~1-2 hours)

Update to show different messaging for remux vs transcode:

```typescript
{transcodeStatus === 'processing' && (
  <div className="p-4 bg-blue-50 rounded-lg">
    <p className="text-sm font-medium">
      {transcodeType === 'remux' && 'Preparing video (very fast)...'}
      {transcodeType === 'remux_audio' && 'Preparing video with audio conversion...'}
      {transcodeType === 'transcode' && 'Converting video (this may take a while)...'}
    </p>
    <div className="mt-2 bg-blue-200 rounded-full h-2">
      <div className="bg-blue-600 h-2 rounded-full" style={{width: `${progress}%`}} />
    </div>
    <p className="text-xs text-gray-600 mt-1">
      {progress}% complete
      {transcodeType === 'remux' && ' (usually 2-5 minutes)'}
      {transcodeType === 'remux_audio' && ' (usually 5-10 minutes)'}
      {transcodeType === 'transcode' && estimatedTime && ` • About ${estimatedTime} remaining`}
    </p>
  </div>
)}
```

**Frontend Tasks:**

- [ ] Create useTranscodeStatus hook with 2s polling
- [ ] Change MediaDetailsModal to use DASH manifest URLs
- [ ] Handle 302 redirect for Direct Play
- [ ] Add 202 response handling with job type detection (3 types)
- [ ] Add progress UI with different messaging for remux/remux_audio/transcode
- [ ] Implement adaptive timeout (2 min for remux, 10 min for remux_audio, 5 min for transcode)
- [ ] Add fallback to direct stream on timeout/error

## Disk Space Management & Cleanup Strategy

### Overview

Transcodes can consume significant disk space (potentially equal to or larger than the original library). A robust cleanup strategy is essential to prevent disk exhaustion.

### 1. Disk Space Monitoring

**Pre-Transcode Check:**

```go
func (s *service) checkDiskSpace(outputDir string, estimatedSize int64) error {
    var stat syscall.Statfs_t
    if err := syscall.Statfs(outputDir, &stat); err != nil {
        return err
    }

    // Available space in bytes
    availableSpace := stat.Bavail * uint64(stat.Bsize)

    // Require 10GB minimum free space OR 2x estimated transcode size, whichever is larger
    requiredSpace := max(10*1024*1024*1024, estimatedSize*2)

    if availableSpace < uint64(requiredSpace) {
        return fmt.Errorf("insufficient disk space: %d GB available, %d GB required",
            availableSpace/(1024*1024*1024), requiredSpace/(1024*1024*1024))
    }

    return nil
}
```

**Continuous Monitoring:**

- Check disk usage every 5 minutes (background goroutine)
- Log warning when disk usage exceeds 80%
- Trigger automatic cleanup when disk usage exceeds 85%
- Refuse new transcode jobs when disk usage exceeds 90%

### 2. Automatic Cleanup Strategies

#### Strategy A: LRU (Least Recently Used) - Recommended

Track access time for each transcode and delete oldest unused transcodes when space is needed.

**Implementation:**

```go
type TranscodeAccessLog struct {
    MediaID      int64
    Quality      string
    LastAccessed time.Time
    SizeBytes    int64
}

// Update access time when manifest is served
func (h *TranscodeHandler) ServeManifest(c *gin.Context) {
    // ... existing code ...

    // Track access
    s.accessLog.Update(mediaID, quality, time.Now())
}

// Cleanup old transcodes to free up space
func (s *service) CleanupLRU(targetFreeSpace int64) error {
    // Get all transcodes sorted by last access time (oldest first)
    transcodes, _ := s.accessLog.GetAllSortedByAccess()

    freedSpace := int64(0)
    for _, t := range transcodes {
        if freedSpace >= targetFreeSpace {
            break
        }

        // Delete transcode directory
        dir := filepath.Join(s.outputDir, "dash", strconv.Itoa(t.MediaID), t.Quality)
        size := getDirSize(dir)

        if err := os.RemoveAll(dir); err != nil {
            log.Error("failed to delete transcode", slog.String("dir", dir), slog.String("error", err.Error()))
            continue
        }

        // Delete job from database
        s.repo.DeleteByMediaAndQuality(context.Background(), t.MediaID, t.Quality)

        freedSpace += size
        log.Info("deleted transcode (LRU cleanup)",
            slog.Int64("media_id", t.MediaID),
            slog.String("quality", t.Quality),
            slog.Int64("freed_mb", size/(1024*1024)))
    }

    return nil
}
```

**Database Schema Addition:**

```sql
CREATE TABLE transcode_access_log (
    media_id INTEGER NOT NULL,
    quality TEXT NOT NULL,
    last_accessed TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    size_bytes BIGINT,
    PRIMARY KEY (media_id, quality)
);

CREATE INDEX idx_transcode_access_log_last_accessed ON transcode_access_log(last_accessed);
```

#### Strategy B: Time-Based Retention

Delete transcodes older than a configured threshold (e.g., 30 days) that haven't been accessed.

```go
func (s *service) CleanupOldTranscodes(retentionDays int) error {
    cutoff := time.Now().AddDate(0, 0, -retentionDays)

    // Find transcodes not accessed since cutoff
    oldTranscodes, _ := s.accessLog.GetOlderThan(cutoff)

    for _, t := range oldTranscodes {
        dir := filepath.Join(s.outputDir, "dash", strconv.Itoa(t.MediaID), t.Quality)
        os.RemoveAll(dir)
        s.repo.DeleteByMediaAndQuality(context.Background(), t.MediaID, t.Quality)
    }

    return nil
}
```

#### Strategy C: Quality-Based Cleanup

When space is tight, delete lower-priority qualities first:

1. Delete 360p transcodes first (lowest quality)
2. Then 720p
3. Then 4K (typically largest, may not be frequently used)
4. Keep 1080p as longest (most common viewing quality)

```go
func (s *service) CleanupByQualityPriority(targetFreeSpace int64) error {
    priorityOrder := []string{"360p", "4k", "720p", "1080p"}  // Delete in this order

    freedSpace := int64(0)
    for _, quality := range priorityOrder {
        if freedSpace >= targetFreeSpace {
            break
        }

        // Get all transcodes of this quality, sorted by last access
        transcodes, _ := s.accessLog.GetByQuality(quality)

        for _, t := range transcodes {
            // Delete and free space
            // ... similar to LRU cleanup ...
        }
    }

    return nil
}
```

### 3. Failed/Partial Transcode Cleanup

**On Server Startup:**

```go
func (s *service) CleanupOnStartup() error {
    // Mark all "processing" jobs as failed
    processingJobs, _ := s.repo.ListByStatus(context.Background(), "processing")
    for _, job := range processingJobs {
        job.Status = "failed"
        job.Error = "Server restarted during transcode"
        s.repo.Update(context.Background(), job)
    }

    // Delete partial transcode directories for failed jobs older than 24 hours
    failedJobs, _ := s.repo.ListByStatus(context.Background(), "failed")
    cutoff := time.Now().Add(-24 * time.Hour)

    for _, job := range failedJobs {
        if job.CreatedAt.Before(cutoff) {
            dir := filepath.Join(s.outputDir, "dash", strconv.Itoa(job.MediaID), job.Quality)
            if _, err := os.Stat(dir); !os.IsNotExist(err) {
                os.RemoveAll(dir)
                s.repo.Delete(context.Background(), job.ID)
            }
        }
    }

    return nil
}
```

**Periodic Cleanup (Run Daily):**

```go
func (s *service) RunPeriodicCleanup() {
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()

    for range ticker.C {
        // 1. Clean failed jobs older than 24 hours
        s.CleanupFailedJobs()

        // 2. Clean transcodes not in database (orphaned files)
        s.CleanupOrphanedFiles()

        // 3. Check disk usage and trigger LRU cleanup if needed
        diskUsage := s.getDiskUsage()
        if diskUsage > 0.85 {  // 85% full
            targetFreeSpace := int64(50 * 1024 * 1024 * 1024)  // Free up 50GB
            s.CleanupLRU(targetFreeSpace)
        }
    }
}
```

### 4. Source File Change Invalidation

When source file is modified (mtime changes), invalidate existing transcodes:

```go
func (s *service) InvalidateTranscodeIfStale(mediaID int64, sourceModTime time.Time) error {
    jobs, _ := s.repo.ListByMediaID(context.Background(), mediaID)

    for _, job := range jobs {
        // If job was created before source was modified, it's stale
        if job.CreatedAt.Before(sourceModTime) {
            // Delete transcode directory
            dir := filepath.Join(s.outputDir, "dash", strconv.Itoa(mediaID), job.Quality)
            os.RemoveAll(dir)

            // Delete job from database
            s.repo.Delete(context.Background(), job.ID)

            log.Info("invalidated stale transcode",
                slog.Int64("media_id", mediaID),
                slog.String("quality", job.Quality))
        }
    }

    return nil
}
```

Call this during library scan when detecting file changes.

### 5. Manual Cleanup (Admin Panel)

Future admin panel endpoints:

```go
// DELETE /api/admin/transcode/:media_id/:quality - Delete specific transcode
// DELETE /api/admin/transcode/:media_id - Delete all transcodes for media
// DELETE /api/admin/transcode/quality/:quality - Delete all transcodes of quality
// DELETE /api/admin/transcode/older-than/:days - Delete transcodes not accessed in X days
// POST /api/admin/transcode/cleanup/lru?target_gb=50 - Free up X GB using LRU
// GET /api/admin/transcode/disk-usage - Get current disk usage stats
```

### 6. Configuration

**Environment Variables:**

```bash
# Transcode output directory
TRANSCODE_OUTPUT_DIR=./data/transcode

# Maximum storage for transcodes (in GB, 0 = unlimited)
TRANSCODE_MAX_STORAGE_GB=500

# Disk usage threshold for automatic cleanup (percentage)
TRANSCODE_CLEANUP_THRESHOLD=85

# Retention policy for unused transcodes (days, 0 = keep forever)
TRANSCODE_RETENTION_DAYS=30

# Minimum free space required before starting transcode (GB)
TRANSCODE_MIN_FREE_SPACE_GB=10
```

### 7. Storage Limits Per Media

Prevent any single media from consuming excessive space:

```go
func (s *service) checkPerMediaLimit(mediaID int64) error {
    existingJobs, _ := s.repo.ListByMediaID(context.Background(), mediaID)

    totalSize := int64(0)
    for _, job := range existingJobs {
        if job.Status == "completed" {
            dir := filepath.Join(s.outputDir, "dash", strconv.Itoa(mediaID), job.Quality)
            totalSize += getDirSize(dir)
        }
    }

    // Limit: 100GB per media item (prevents one huge movie from filling disk)
    maxPerMedia := int64(100 * 1024 * 1024 * 1024)
    if totalSize > maxPerMedia {
        return fmt.Errorf("media %d has exceeded storage limit (%d GB)",
            mediaID, maxPerMedia/(1024*1024*1024))
    }

    return nil
}
```

### 8. Cleanup Priority Order

When automatic cleanup is triggered:

1. **Failed jobs** (older than 24 hours) - Free space with zero impact
2. **360p transcodes** (lowest quality, least useful) - Minimal quality impact
3. **4K transcodes** (huge files, rarely used) - Significant space savings
4. **Least recently used transcodes** (any quality) - Impact rarely-watched content
5. **720p transcodes** (keep 1080p as primary quality)
6. **1080p transcodes** (last resort, most commonly used)

### 9. Monitoring & Alerts

**Metrics to Track:**

```go
type StorageMetrics struct {
    TotalSpaceGB         float64
    UsedSpaceGB          float64
    AvailableSpaceGB     float64
    UsagePercentage      float64
    TranscodeSizeGB      float64
    TranscodeCount       int
    OldestTranscodeAge   time.Duration
    AverageTranscodeSize int64
}
```

**Alert Conditions:**

- Warning: Disk usage > 80%
- Critical: Disk usage > 90%
- Info: Automatic cleanup triggered
- Info: X GB freed by cleanup
- Error: Unable to free sufficient space

## Additional Considerations

### 1. Atomic Writes for Transcodes

**Problem:** If transcode is interrupted (server crash, kill signal), partial files remain on disk.

**Solution:**

- Use temporary directory for in-progress transcodes: `./data/transcode/.tmp/<job_id>/`
- On completion, atomically rename to final location: `./data/transcode/dash/<media_id>/<quality>/`
- Cleanup `.tmp/` directory on server restart (delete all temp directories)

### 2. Subtitle/Caption Support

**Current:** Not addressed in MVP

**Future:** Add subtitle tracks to DASH manifest
- Extract embedded subtitles from source (SRT, VTT, ASS)
- Convert to WebVTT format (browser-compatible)
- Add to DASH manifest as separate tracks
- FFmpeg: `-map 0:s` to copy subtitle streams

### 3. Multiple Audio Tracks

**Current:** Only processes first audio stream

**Consideration:** Some media has multiple audio tracks (English, Spanish, commentary)
- For MVP: Use first audio track only
- Future: Allow user to select audio track before transcoding
- FFmpeg: `-map 0:a:0` for first audio track, `-map 0:a:1` for second, etc.

### 4. Seek Performance

**DASH with segments:** Very fast seeking (jump to specific segment)
**Direct stream:** Slower seeking (must scan through file)

DASH provides better seek performance, which justifies the transcode/remux overhead for files users watch frequently.

### 5. Server Restart Recovery

**Behavior:**
- On startup, query all jobs with status="processing"
- Mark as "failed" (can be retried by user)
- Frontend will auto-retry when user clicks Play again

**Alternative:** Resume in-progress transcodes (more complex, not MVP)

### 6. Bandwidth Considerations

**DASH segments:** ~4 seconds each, download on-demand
**Direct stream:** Downloads entire file

For users with limited bandwidth, DASH is beneficial even for compatible codecs because:
- Can start playback with only first few segments downloaded
- Can switch quality mid-playback (adaptive bitrate)
- Less wasted bandwidth if user stops watching early

### 7. CDN/Reverse Proxy

**Serving segments through CDN:**
- Add proper cache headers (Cache-Control, ETag)
- Segments never change once written (safe to cache aggressively)
- Manifest can be cached with shorter TTL (5-10 minutes)

```go
// Add to ServeManifest and ServeDASHSegment
c.Header("Cache-Control", "public, max-age=31536000")  // 1 year for segments
c.Header("Cache-Control", "public, max-age=600")       // 10 min for manifest
```

### 8. Monitoring & Observability

**Metrics to track:**
- Transcode success rate by type (remux vs transcode)
- Average transcode time by quality
- Cache hit rate (manifest exists vs new job)
- Fallback rate (how often users fall back to direct stream)
- Storage usage trend

**Logging:**
- Log each strategy selection decision with reason
- Log transcode failures with ffmpeg stderr output
- Track which codecs/formats are most common in library

### 9. Browser Compatibility

**Beyond codec support:**
- Safari has quirks with MSE (Media Source Extensions)
- Some browsers limit concurrent DASH segment requests
- iOS Safari requires specific DASH profile (use `-dash_segment_type mp4`)

**Testing checklist:**
- Chrome, Firefox, Edge, Safari (desktop)
- Safari iOS, Chrome Android (mobile)
- Verify adaptive bitrate switching works
- Test seek behavior across browsers

## Edge Cases

### Concurrent Play Requests

Database UNIQUE constraint on `(media_id, quality)` prevents duplicate jobs. First request creates job, subsequent requests return existing job status.

### Transcode Fails

Frontend falls back to direct stream after timeout or on error response. Failed job can be retried (will be deleted and recreated).

### Disk Full

Backend pre-checks disk space before starting transcode. If full, returns error and frontend falls back to direct stream immediately.

### Source File Moved/Deleted

If source file is moved/deleted during transcode:
- FFmpeg will fail with error
- Job marked as failed
- User can play via direct stream if file is still accessible
- Next library scan will update media paths

## Performance Expectations

| Operation | Expected Time | Notes |
|-----------|---------------|-------|
| **Cached manifest** | <500ms | File serve from disk |
| **Direct Play** | <100ms | 302 redirect to original |
| **New remux (stereo)** | 2-5 minutes | Very fast - all streams copied |
| **New remux (5.1/7.1)** | 5-10 minutes | Fast - video copied, audio downmixed |
| **New transcode** | 20-60 minutes | Slow - full re-encode |
| **Fallback to direct stream** | <2 seconds | Always works as safety net |

**Real-world examples (1-hour 1080p movie):**

- MKV with H.264/AAC stereo: Remux in ~3 minutes
- MKV with H.264/AAC 5.1 surround: Remux with audio downmix in ~7 minutes
- HEVC (H.265) to H.264: Transcode in ~35 minutes (CPU), ~15 minutes (GPU)
- MP4 with H.264/AAC stereo: Direct Play (instant)

## Future Enhancements (Deferred)

These are **NOT** part of MVP but may be added later:

- Priority field for queue management (when we have >10 concurrent users)
- Progressive playback (start with partial manifest)
- Admin panel for cache cleanup
- Quality recommendation endpoint (currently done in frontend)
- Transcode analytics and monitoring
- Auto-cleanup of old transcodes

## Success Criteria

**Direct Play:**

- [ ] MP4 with H.264/AAC plays instantly without any processing
- [ ] User sees no delay or progress indicator

**Remux (stereo audio):**

- [ ] MKV with H.264/AAC stereo automatically creates remux job
- [ ] User sees "Preparing video (very fast)..." message
- [ ] Playback starts within 2-5 minutes
- [ ] Second play uses cached DASH manifest

**Remux + Audio Downmix (multi-channel audio):**

- [ ] MKV with H.264/AAC 5.1 automatically creates remux_audio job
- [ ] User sees "Preparing video with audio conversion..." message
- [ ] Playback starts within 5-10 minutes
- [ ] Audio is properly downmixed to stereo (no missing channels)
- [ ] Second play uses cached DASH manifest

**Transcode:**

- [ ] HEVC/incompatible codec automatically creates transcode job
- [ ] User sees "Converting video..." with longer time estimate
- [ ] Playback starts when complete or times out after 5 minutes
- [ ] Timeout triggers fallback to direct stream

**Fallback:**

- [ ] If any processing fails, user can still watch via direct stream
- [ ] Timeout works correctly (2 min for remux, 10 min for remux_audio, 5 min for transcode)
- [ ] Fallback to direct stream is seamless (no error shown to user)

## References

- [DASH Specification](https://dashif.org/docs/)
- [Shaka Player Documentation](https://shaka-player-demo.appspot.com/docs/api/tutorial-welcome.html)
- Existing implementation: `internal/application/transcode/`, `internal/infrastructure/transcoding/`

## Revision History

- 2025-11-13: Initial version - On-demand transcoding design
- 2025-11-13: Simplified to MVP scope after architecture review
- 2025-11-13: Added three-tier strategy (Direct Play → Remux → Transcode) for better performance
- 2025-11-13: Added audio channel handling - browsers only support stereo, so multi-channel audio (5.1/7.1) must be downmixed during remux, creating a fourth strategy (Remux + Audio Downmix)
