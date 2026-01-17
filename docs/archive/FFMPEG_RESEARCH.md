# FFmpeg Patches & Optimizations Research

This document consolidates research on FFmpeg customizations used by major media servers (Jellyfin, Plex, Emby) and proposes original improvements for ViewRA.

## Table of Contents

1. [Current ViewRA Patches](#current-viewra-patches)
2. [Competitor Analysis](#competitor-analysis)
3. [Proposed Improvements](#proposed-improvements)
4. [Implementation Roadmap](#implementation-roadmap)

---

## Current ViewRA Patches

ViewRA uses a custom FFmpeg 7.1 build with 7 patches:

| # | Patch | Category | Purpose |
|---|-------|----------|---------|
| 0001 | segment-muxer-track-start-pts | HLS | Fix A/V sync when seeking mid-stream |
| 0002 | libx265-fix-api-for-x265-4.1 | Compat | Support x265 4.1+ API changes |
| 0003 | fix-safari-hls-empty-sdtp | Safari | HEVC fMP4 SDTP box fix |
| 0004 | fix-nvdec-exceed-32-surfaces | NVIDIA | Surface pool limits for 4K |
| 0005 | add-hdr-metadata-for-nvenc-hevc | HDR | Preserve HDR10 in NVENC output |
| 0006 | pass-dovi-sidedata-to-hls-mpegts | DV | Dolby Vision metadata in HLS |
| 0007 | segment-muxer-report-keyframe-start-offset | HLS | Report actual seek position |

**Build location:** `tools/ffmpeg-viewra/`

---

## Competitor Analysis

### Jellyfin-FFmpeg (93 patches)

Jellyfin maintains the most comprehensive public FFmpeg fork. Key innovations:

#### Hardware Interop (~25 patches)
- **D3D11↔OpenCL bridges** (0009, 0012): Zero-copy between DirectX decode and OpenCL processing
- **Vulkan pipeline fixes** (0060-0063): Stable import/export for modern GPU pipelines
- **VAAPI↔DRM export** (0038, 0041): Linux GPU memory sharing

#### Tone Mapping (~15 patches)
- **SIMD `tonemapx` filter** (0057): AVX2/FMA3-optimized CPU tone mapping (3-5x faster)
- **CUDA tone mapping** (0004): Full GPU pipeline on NVIDIA
- **OpenCL BT.2390 EETF** (0007): Broadcast-standard tone mapping
- **VideoToolbox/Metal** (0050): Native Apple Silicon tone mapping

#### Platform-Specific (~20 patches)
- **Rockchip RK3588** (0046): Full HWA pipeline for ARM SBCs (4K@120fps encode)
- **Apple VideoToolbox** (0047-0053): Overlay, transpose, bwdif, AV1 decode
- **Intel QSV** (0018-0021): VPL fixes, low-power fallback

#### Subtitle & Overlay
- **OpenCL PGS overlay** (0008): GPU-accelerated bitmap subtitle burn-in
- **Vulkan PGS overlay** (0064): Universal GPU subtitle rendering
- **Sub2video perf fix** (0069): Faster text subtitle handling

#### Other Notable
- **AC-4 decoder** (0058): ATSC 3.0 audio support
- **YADIF/BWDIF OpenCL** (0082): GPU deinterlacing
- **Film grain passthrough**: AV1 grain synthesis metadata

**Source:** https://github.com/jellyfin/jellyfin-ffmpeg/tree/jellyfin/debian/patches

### Plex Transcoder

Plex maintains a closed-source FFmpeg fork with notable features:

#### EAE (Easy Audio Encoder)
- Separate subprocess for audio transcoding
- Isolates audio from video pipeline
- Located at `/tmp/pms-.../EasyAudioEncoder/`
- Allows GPU to focus purely on video

#### Architecture
- "New Transcoder" for transcoding
- "Old Transcoder" for media scanning
- Optimized binaries per use case

**Source:** https://github.com/Diagonactic/plex-new-transcoder

### Emby

Emby uses stock FFmpeg with configuration-based customization:
- Hardware encoder selection via `encoding.xml`
- Limited tone mapping (often falls back to software)
- Known issues with AMD GPU + HDR tone mapping

---

## Proposed Improvements

### Category 1: Performance

#### P1. SIMD-Optimized CPU Tone Mapping
```
Priority: HIGH
Effort: MEDIUM (port Jellyfin patch 0057)
Benefit: 3-5x faster CPU tone mapping fallback

Current: libplacebo CPU path is slow
Proposed: Port `tonemapx` filter with AVX2/FMA3 optimization
Use Case: Fallback when GPU unavailable, low-power devices
```

#### P2. VideoToolbox Scaling Fix
```
Priority: HIGH (for Mac users)
Effort: MEDIUM (port Jellyfin patches 0047-0050)
Benefit: 20-30% faster transcoding on Apple Silicon

Current: VT decode → CPU scale → VT encode (bottleneck)
Proposed: VT decode → Metal scale → VT encode (full GPU)
Patches: vf_scale_vt, vf_overlay_vt, vf_tonemap_vt
```

#### P3. Speculative Segment Pre-computation
```
Priority: MEDIUM
Effort: HIGH (novel implementation)
Benefit: Perceived instant seek

Current: Seek to unwatched position → wait for transcode
Proposed:
  - Generate low-quality "preview frames" at 10x speed
  - Show preview immediately, swap to HQ when ready
  - Keyframe index enables instant preview positioning
```

#### P4. Adaptive GOP Based on Content
```
Priority: MEDIUM
Effort: MEDIUM (FFmpeg filter + session manager changes)
Benefit: 10-15% bitrate savings

Current: Fixed 2-second GOP regardless of content
Proposed:
  - Detect scene complexity in real-time
  - Insert keyframes at scene boundaries aligned to HLS segments
  - Static dialogue → longer GOP, action → shorter GOP
```

#### P5. Parallel Subtitle Rendering Pipeline
```
Priority: MEDIUM
Effort: HIGH (architectural change)
Benefit: Near-zero subtitle overhead

Current: Subtitle burn-in blocks main encode pipeline
Proposed: Plex-style isolated subprocess for subtitle rendering
  - Main pipeline: video decode → encode
  - Subtitle pipeline: render frames → overlay queue
  - Compositor: merge at frame-accurate timestamps
```

### Category 2: Quality

#### Q1. Film Grain Synthesis for AV1
```
Priority: HIGH (for film content)
Effort: LOW (SVT-AV1 parameter)
Benefit: 30-66% bitrate savings on grainy content

Current: Grain encoded at full bitrate cost
Proposed:
  - Detect grainy content via variance analysis
  - Use SVT-AV1 film-grain parameter
  - Grain synthesized on decode (no bandwidth cost)
Parameters: film-grain-denoise=0:film-grain=20
```

#### Q2. HDR10+ Dynamic Metadata Passthrough
```
Priority: MEDIUM
Effort: MEDIUM (SEI parsing)
Benefit: Better HDR on Samsung/Panasonic TVs

Current: HDR10 static metadata only
Proposed: Parse HDR10+ SEI, inject into output stream
Challenge: Per-scene metadata adds complexity
```

#### Q3. Perceptual Quality Feedback Loop
```
Priority: LOW
Effort: HIGH (research project)
Benefit: Consistent perceptual quality

Current: Fixed CRF across all content
Proposed:
  - Real-time complexity estimation
  - Feed back to rate control
  - Boost bits for detailed scenes, save on simple scenes
```

#### Q4. Scene-Change Keyframes (HLS-Aligned)
```
Priority: MEDIUM
Effort: MEDIUM (FFmpeg modification)
Benefit: Better quality at scene transitions

Current: Scene detection disabled for HLS alignment
Proposed:
  - Detect scene changes
  - Insert keyframe at NEXT segment boundary (not immediately)
  - Example: Scene at 5.3s → keyframe at 6.0s
```

### Category 3: Reliability

#### R1. Graceful Decoder Recovery
```
Priority: HIGH
Effort: MEDIUM
Benefit: Smooth playback on damaged sources

Current: Corrupt frames → green artifacts or crash
Proposed:
  - Frame-level corruption detection
  - Skip and interpolate from nearby frames
  - Log warning but continue playback
```

#### R2. Predictive Hardware Fallback
```
Priority: MEDIUM
Effort: MEDIUM
Benefit: Seamless failover without glitches

Current: Fallback after failure occurs
Proposed:
  - Monitor GPU encoder health metrics
  - Queue depth, frame latency, memory pressure
  - Trigger fallback BEFORE visible failure
```

#### R3. Segment Integrity Verification
```
Priority: LOW
Effort: LOW
Benefit: Detect corruption on unreliable networks

Current: No verification of segment integrity
Proposed:
  - Embed checksum in playlist comments
  - #X-VIEWRA-CHECKSUM:seg_000042.ts:crc32:a1b2c3d4
  - Client can verify and request retransmit
```

#### R4. A/V Sync Heartbeat
```
Priority: LOW
Effort: MEDIUM
Benefit: Self-correcting sync over long playback

Current: A/V drift can accumulate
Proposed:
  - Embed sync markers every 60 segments
  - Synchronized timestamp in both streams
  - Client can detect and correct drift
```

### Category 4: Latency

#### L1. Low-Latency HLS (LL-HLS)
```
Priority: MEDIUM
Effort: HIGH
Benefit: Sub-second latency

Current: 2-second segments = 4+ second latency
Proposed:
  - CMAF with partial segments (200ms chunks)
  - #EXT-X-PART for partial segment advertisement
  - #EXT-X-PRELOAD-HINT for upcoming chunks
  - HTTP chunked transfer encoding
Parameters:
  -ldash 1
  -frag_type every_frame
  -format_options 'movflags=cmaf'
```

#### L2. Instant Preview on Seek
```
Priority: MEDIUM
Effort: HIGH
Benefit: Perceived instant response

Current: Seek → wait for segment generation
Proposed:
  - Pre-generated thumbnail timeline (already exists)
  - Show animated thumbnail during seek
  - Transition to video when ready
```

### Category 5: Hardware Support

#### H1. Rockchip RK3588 Support
```
Priority: LOW (niche)
Effort: HIGH (port Jellyfin patch 0046)
Benefit: ARM SBC deployment

Performance: 4K@120fps encode, 1080p@480fps
Requirements: BSP kernel 5.10/6.1, MPP + RGA
Features: Zero-copy decode→scale→tonemap→encode
```

#### H2. Intel Arc AV1 Optimization
```
Priority: MEDIUM
Effort: MEDIUM
Benefit: Best-in-class AV1 encoding

Current: Generic QSV path
Proposed:
  - Direct memory path (like CUDA hwupload)
  - AV1 encoding with film grain synthesis
  - Low-power mode for efficiency
```

#### H3. GPU Subtitle Burn-in (OpenCL/Vulkan)
```
Priority: HIGH
Effort: MEDIUM (port Jellyfin patches 0008, 0064)
Benefit: Full GPU pipeline with subtitles

Current: CPU subtitle rendering breaks GPU pipeline
Proposed:
  - OpenCL PGS overlay filter
  - Vulkan overlay as fallback
  - Keeps entire pipeline on GPU
```

---

## Implementation Roadmap

### Phase 1: Quick Wins (1-2 weeks each)

| ID | Improvement | Effort | Impact |
|----|-------------|--------|--------|
| Q1 | Film Grain Synthesis | LOW | HIGH |
| R3 | Segment Checksums | LOW | LOW |
| P1 | Port tonemapx SIMD | MEDIUM | HIGH |

### Phase 2: Core Improvements (2-4 weeks each)

| ID | Improvement | Effort | Impact |
|----|-------------|--------|--------|
| P2 | VideoToolbox Fix | MEDIUM | HIGH |
| H3 | GPU Subtitle Burn-in | MEDIUM | HIGH |
| R1 | Decoder Recovery | MEDIUM | HIGH |
| Q4 | Scene-Change Keyframes | MEDIUM | MEDIUM |

### Phase 3: Advanced Features (1-2 months each)

| ID | Improvement | Effort | Impact |
|----|-------------|--------|--------|
| L1 | LL-HLS Support | HIGH | MEDIUM |
| P3 | Speculative Pre-computation | HIGH | MEDIUM |
| P5 | Parallel Subtitle Pipeline | HIGH | MEDIUM |

### Phase 4: Research Projects

| ID | Improvement | Effort | Impact |
|----|-------------|--------|--------|
| Q3 | Perceptual Quality Feedback | HIGH | MEDIUM |
| H1 | RK3588 Support | HIGH | LOW |

---

## Comparison Matrix

| Feature | ViewRA | Jellyfin | Plex | Emby |
|---------|--------|----------|------|------|
| **Public patches** | 7 | 93 | ~20 | ~5 |
| **HDR→SDR backends** | 4 | 6 | 2 | 3 |
| **Dolby Vision** | Passthrough | Tone map | ✓ | Partial |
| **SIMD tone map** | ✗ | ✓ | ? | ✗ |
| **GPU subtitles** | ✗ | ✓ | ? | ✗ |
| **RK3588** | ✗ | ✓ | ✗ | ✗ |
| **LL-HLS** | ✗ | ✗ | ✗ | ✗ |
| **Film grain synth** | ✗ | ✗ | ✗ | ✗ |
| **EAE-style audio** | ✗ | ✗ | ✓ | ✗ |

---

## Technical Deep Dives

### A. How Jellyfin's `tonemapx` Works

The `tonemapx` filter uses SIMD instructions for CPU-based HDR→SDR conversion:

```c
// Pseudo-code for AVX2 path
void tonemap_frame_avx2(Frame *dst, Frame *src, ToneMapParams *p) {
    __m256 peak = _mm256_set1_ps(p->peak_luminance);
    __m256 scale = _mm256_set1_ps(p->sdr_peak / p->hdr_peak);

    for (int y = 0; y < height; y++) {
        for (int x = 0; x < width; x += 8) {  // Process 8 pixels at once
            __m256 rgb = load_pixels_avx2(src, x, y);
            __m256 linear = pq_to_linear_avx2(rgb);
            __m256 mapped = bt2390_eetf_avx2(linear, peak);
            __m256 sdr = linear_to_srgb_avx2(mapped);
            store_pixels_avx2(dst, x, y, sdr);
        }
    }
}
```

Key optimizations:
- Process 8 pixels per iteration (256-bit vectors)
- Inline PQ↔linear conversion
- BT.2390 EETF in SIMD
- FMA3 for fused multiply-add

### B. Plex EAE Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Plex Transcoder                       │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────────┐    ┌─────────────────────────────┐ │
│  │  Video Pipeline │    │  EAE (Audio Subprocess)     │ │
│  │                 │    │                             │ │
│  │ decode → scale  │    │ decode → resample → encode  │ │
│  │    → encode     │    │                             │ │
│  │                 │    │ /tmp/pms-.../EasyAudioEncoder│ │
│  └────────┬────────┘    └──────────────┬──────────────┘ │
│           │                            │                │
│           └────────────┬───────────────┘                │
│                        ▼                                │
│              ┌─────────────────┐                        │
│              │    HLS Muxer    │                        │
│              │  (merge A/V)    │                        │
│              └─────────────────┘                        │
└─────────────────────────────────────────────────────────┘
```

Benefits:
- Audio doesn't block video GPU pipeline
- Can use different process priorities
- Audio can be cached independently
- Failure isolation

### C. Film Grain Synthesis Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ Source Video │────▶│   Denoise    │────▶│  Encode AV1  │
│  (grainy)    │     │ (remove grain)│     │ (clean video)│
└──────────────┘     └──────────────┘     └──────┬───────┘
                                                  │
      ┌──────────────────────────────────────────┤
      │                                          │
      ▼                                          ▼
┌───────────────┐                        ┌──────────────┐
│ Analyze Grain │                        │  Bitstream   │
│   Pattern     │                        │ (compressed) │
└───────┬───────┘                        └──────────────┘
        │                                        │
        │    grain params                        │
        │    (autoregressive model)              │
        ▼                                        ▼
┌───────────────┐                        ┌──────────────┐
│ Grain Metadata│───────────────────────▶│   Decoder    │
│ (< 1 KB/frame)│     transmitted        │ + Synthesis  │
└───────────────┘      together          └──────────────┘
                                                │
                                                ▼
                                         ┌──────────────┐
                                         │ Output Video │
                                         │ (grain added)│
                                         └──────────────┘
```

Netflix results: 8274 kbps → 2804 kbps (66% reduction) with FGS.

---

## References

### Jellyfin
- Repository: https://github.com/jellyfin/jellyfin-ffmpeg
- Patches: https://github.com/jellyfin/jellyfin-ffmpeg/tree/jellyfin/debian/patches
- Features Wiki: https://github.com/jellyfin/jellyfin-ffmpeg/wiki/Features
- Hardware Acceleration: https://jellyfin.org/docs/general/post-install/transcoding/hardware-acceleration/

### Plex
- FFmpeg Mirror: https://github.com/comio/plex-ffmpeg
- New Transcoder: https://github.com/Diagonactic/plex-new-transcoder
- Hardware Streaming: https://support.plex.tv/articles/115002178853-using-hardware-accelerated-streaming/

### Emby
- Hardware Overview: https://emby.media/support/articles/Hardware-Acceleration-Overview.html

### FFmpeg
- LL-HLS: https://docs.tebi.io/streaming/ffmpeg_rl_hls.html
- Film Grain: https://norkin.org/pdf/DCC_2018_AV1_film_grain.pdf
- Super Resolution: https://github.com/MIR-MU/ffmpeg-tensorflow

### SVT-AV1
- Encoding Guide: https://gist.github.com/dvaupel/716598fc9e7c2d436b54ae00f7a34b95

---

## ViewRA-Specific Original Ideas

These ideas build on ViewRA's existing architecture and are unique opportunities not seen in competitors.

### V1. Predictive Hardware Health Monitoring

**Current:** `HardwareFallbackManager` reacts to failures after they occur.

**Proposed:** Monitor GPU encoder health proactively:

```go
// internal/infrastructure/transcoding/ffmpeg/health_monitor.go

type HardwareHealthMonitor struct {
    metrics struct {
        queueDepth     atomic.Int64  // Pending frames in encoder
        avgFrameTimeMs atomic.Int64  // Moving average encode time
        memoryPressure atomic.Int64  // GPU memory usage %
        errorRate      atomic.Int64  // Recent error rate
    }
    thresholds HealthThresholds
}

type HealthThresholds struct {
    MaxQueueDepth     int     // > 30 frames = warning
    MaxFrameTimeMs    int64   // > 100ms = degraded
    MaxMemoryPressure float64 // > 90% = critical
    MaxErrorRate      float64 // > 5% = fallback
}

func (m *HardwareHealthMonitor) ShouldFallback() bool {
    // Predictive: trigger fallback BEFORE visible failure
    score := m.calculateHealthScore()
    return score < 0.5 // < 50% health = preemptive fallback
}
```

**Benefit:** Seamless failover without user-visible glitches.

### V2. Content-Aware Session Preloading

**Current:** Sessions start when user requests playback.

**Proposed:** Preload transcode sessions based on user behavior:

```go
// Predict next likely content based on:
// 1. Watch history (continue watching)
// 2. Series episodes (autoplay next)
// 3. Recently browsed items
// 4. Time of day patterns

type SessionPreloader struct {
    predictions []PredictedWatch
    warmSessions sync.Map // Pre-warmed sessions (first 10 segments ready)
}

func (p *SessionPreloader) OnMediaBrowsed(mediaID int64) {
    // Start warm-up session at 10% priority
    go p.warmupSession(mediaID, "1080p-10m", 0) // Start from beginning
}

func (p *SessionPreloader) OnEpisodeNearing End(seriesID int64, currentEp int) {
    nextEp := p.findNextEpisode(seriesID, currentEp)
    if nextEp != nil {
        go p.warmupSession(nextEp.ID, "1080p-10m", 0)
    }
}
```

**Benefit:** Instant playback for predicted content.

### V3. Watchdog-Driven Quality Adaptation

**Current:** `ProgressWatchdog` only kills stalled processes.

**Proposed:** Use watchdog signals for quality adaptation:

```go
// Enhance ProgressWatchdog to emit quality signals

type AdaptiveWatchdog struct {
    *ProgressWatchdog
    qualitySignals chan QualitySignal
}

type QualitySignal struct {
    Type      SignalType // Degraded, Recovered, Critical
    Metric    string     // "frame_time", "buffer_size", "error_rate"
    Value     float64
    Suggested QualityAction
}

func (wd *AdaptiveWatchdog) checkProgress() {
    // Existing stall detection...

    // NEW: Performance degradation detection
    if wd.avgFrameTime > wd.targetFrameTime * 1.5 {
        wd.qualitySignals <- QualitySignal{
            Type: Degraded,
            Metric: "frame_time",
            Value: wd.avgFrameTime,
            Suggested: ReduceResolution, // Or ReduceBitrate
        }
    }
}
```

**Benefit:** Dynamic quality adjustment without restarting session.

### V4. Segment-Level Retry with Partial Success

**Current:** Session fails completely on error.

**Proposed:** Retry individual segment generation:

```go
type ResilientSegmentWatcher struct {
    *SegmentWatcher
    retryQueue   chan SegmentRetry
    maxRetries   int
    backoffMs    []int // [100, 500, 2000]
}

type SegmentRetry struct {
    SegmentNum   int
    Attempt      int
    LastError    error
    FallbackTo   string // "software", "lower_quality"
}

func (w *ResilientSegmentWatcher) OnSegmentFailed(segNum int, err error) {
    if w.isRecoverable(err) {
        w.retryQueue <- SegmentRetry{
            SegmentNum: segNum,
            Attempt: 1,
            LastError: err,
        }
    }
}

func (w *ResilientSegmentWatcher) processRetry(retry SegmentRetry) {
    // Option 1: Retry with same settings
    // Option 2: Retry with software encoding
    // Option 3: Skip segment and mark discontinuity
}
```

**Benefit:** Graceful degradation instead of complete failure.

### V5. Cross-Session Segment Sharing

**Current:** Each session generates its own segments.

**Proposed:** Share segments across sessions when possible:

```go
type SegmentCache struct {
    segments sync.Map // key: "mediaID:quality:segNum" -> SegmentRef
}

type SegmentRef struct {
    Path       string
    CreatedAt  time.Time
    RefCount   atomic.Int32
    Checksum   uint32
}

func (c *SegmentCache) GetOrGenerate(mediaID int64, quality string, segNum int) (*SegmentRef, error) {
    key := fmt.Sprintf("%d:%s:%d", mediaID, quality, segNum)

    if ref, ok := c.segments.Load(key); ok {
        seg := ref.(*SegmentRef)
        seg.RefCount.Add(1)
        return seg, nil // Reuse existing segment
    }

    // Generate new segment...
}
```

**Benefit:** Multiple viewers of same content share work.

### V6. Intelligent Codec Selection Based on Content Analysis

**Current:** Codec selected based on client support.

**Proposed:** Analyze content to choose optimal codec:

```go
type ContentAnalyzer struct {
    ffprobe *Prober
}

type ContentProfile struct {
    IsAnimated      bool    // Anime/cartoon (benefits from AV1)
    HasFilmGrain    bool    // Classic films (use grain synthesis)
    IsHighMotion    bool    // Action/sports (needs higher bitrate)
    HasSubtitles    bool    // Foreign films (consider burn-in vs soft)
    DominantColors  int     // Limited palette = better compression
    SceneChangeRate float64 // High = more keyframes needed
}

func (a *ContentAnalyzer) RecommendCodec(profile ContentProfile, clientCodecs []string) Recommendation {
    // Anime → AV1 (excellent at sharp edges, flat colors)
    // Film with grain → AV1 with grain synthesis
    // Live action → H.265 (good motion handling)
    // Fallback → H.264 (universal)
}
```

**Benefit:** Optimal codec per content type (30-50% bitrate savings).

### V7. FFmpeg Process Pool

**Current:** New FFmpeg process per session.

**Proposed:** Pool of warm FFmpeg processes:

```go
type FFmpegPool struct {
    idle     chan *FFmpegProcess
    maxIdle  int
    maxAge   time.Duration
}

type FFmpegProcess struct {
    cmd       *exec.Cmd
    stdin     io.WriteCloser
    createdAt time.Time
}

func (p *FFmpegPool) Acquire() *FFmpegProcess {
    select {
    case proc := <-p.idle:
        if time.Since(proc.createdAt) < p.maxAge {
            return proc // Reuse warm process
        }
        proc.Close()
    default:
    }
    return p.spawn() // Create new
}

func (p *FFmpegPool) Release(proc *FFmpegProcess) {
    select {
    case p.idle <- proc:
        // Returned to pool
    default:
        proc.Close() // Pool full
    }
}
```

**Benefit:** Faster session startup (skip FFmpeg init overhead).

### V8. Bandwidth Estimation from Segment Delivery

**Current:** Quality selected once at session start.

**Proposed:** Measure actual delivery speed for ABR hints:

```go
type BandwidthEstimator struct {
    samples     []BandwidthSample
    windowSize  int
}

type BandwidthSample struct {
    SegmentSize  int64
    DeliveryTime time.Duration
    Timestamp    time.Time
}

func (e *BandwidthEstimator) OnSegmentDelivered(size int64, duration time.Duration) {
    bps := float64(size*8) / duration.Seconds()
    e.samples = append(e.samples, BandwidthSample{
        SegmentSize: size,
        DeliveryTime: duration,
        Timestamp: time.Now(),
    })

    // Emit hint to client via HLS comment
    // #X-VIEWRA-BANDWIDTH-HINT:<estimated_bps>
}

func (e *BandwidthEstimator) SuggestQualityChange() *QualityChange {
    avg := e.calculateEWMA()
    currentBitrate := e.currentProfile.VideoBitrate

    if avg < float64(currentBitrate) * 0.8 {
        return &QualityChange{Direction: Down, Reason: "bandwidth"}
    }
    if avg > float64(currentBitrate) * 1.5 {
        return &QualityChange{Direction: Up, Reason: "headroom"}
    }
    return nil
}
```

**Benefit:** Server-assisted ABR with better information than client-only.

### V9. Subtitle Track Pre-extraction Cache

**Current:** Subtitle extraction on-demand via subtitle-extractor.

**Proposed:** Background extraction for popular content:

```go
type SubtitleCache struct {
    storage   string // data/cache/subtitles/
    extractor *SubtitleExtractor
}

func (c *SubtitleCache) PreExtract(mediaID int64, tracks []SubtitleTrack) {
    for _, track := range tracks {
        key := fmt.Sprintf("%d_%d.vtt", mediaID, track.Index)
        if c.exists(key) {
            continue
        }

        go func(t SubtitleTrack) {
            // Extract to WebVTT in background
            c.extractor.ExtractToVTT(mediaID, t.Index, c.pathFor(key))
        }(track)
    }
}

// Trigger on: library scan, media browsed, playback started
```

**Benefit:** Instant subtitle availability.

### V10. Multi-GPU Load Balancing

**Current:** Single GPU used for all transcoding.

**Proposed:** Distribute sessions across available GPUs:

```go
type GPULoadBalancer struct {
    gpus      []GPUDevice
    sessions  map[string]int // sessionKey -> gpuIndex
    mu        sync.Mutex
}

type GPUDevice struct {
    Index         int
    Name          string
    MemoryTotal   int64
    MemoryUsed    int64
    EncoderLoad   float64 // 0.0 - 1.0
    ActiveSessions int
}

func (lb *GPULoadBalancer) SelectGPU() (*GPUDevice, error) {
    lb.mu.Lock()
    defer lb.mu.Unlock()

    // Select GPU with lowest combined load
    var best *GPUDevice
    bestScore := math.MaxFloat64

    for i := range lb.gpus {
        gpu := &lb.gpus[i]
        score := gpu.EncoderLoad + float64(gpu.ActiveSessions)*0.2
        if score < bestScore {
            bestScore = score
            best = gpu
        }
    }

    return best, nil
}
```

**Benefit:** Better resource utilization for multi-GPU systems.

---

## Novel FFmpeg Patch Ideas

These are innovative FFmpeg modifications that would give ViewRA unique capabilities not available in stock FFmpeg or competitor forks. Unlike the previous ideas which can be implemented through configuration or application-level code, these require actual FFmpeg source modifications.

### P1. Real-Time Encoder Telemetry Stream

**Concept:** Patch FFmpeg's encoder to emit structured telemetry to a Unix socket.

```c
// libavcodec/nvenc.c - Add telemetry emission
typedef struct NvencTelemetry {
    uint64_t frame_num;
    uint32_t encode_time_us;     // Per-frame encode time
    uint32_t queue_depth;        // Pending frames in encoder
    uint32_t bitrate_actual;     // Actual output bitrate
    uint16_t qp_avg;             // Average QP for frame
    uint8_t  is_keyframe;
    uint8_t  encoder_load;       // 0-100%
} NvencTelemetry;

static void emit_telemetry(NvencContext *ctx, NvencTelemetry *t) {
    if (ctx->telemetry_fd > 0) {
        write(ctx->telemetry_fd, t, sizeof(*t));
    }
}
```

**FFmpeg Invocation:**
```bash
ffmpeg ... -telemetry_socket /tmp/viewra_$SESSION.sock ...
```

**Go Consumer:**
```go
type EncoderTelemetry struct {
    conn     net.Conn
    metrics  chan TelemetryFrame
}

func (t *EncoderTelemetry) Listen(socketPath string) {
    conn, _ := net.Dial("unix", socketPath)
    for {
        var frame TelemetryFrame
        binary.Read(conn, binary.LittleEndian, &frame)
        t.metrics <- frame  // Real-time health data!
    }
}
```

**Benefit:** Sub-millisecond latency health data vs parsing stderr (seconds of delay).

---

### P2. Segment Boundary Callback API

**Concept:** FFmpeg calls back to host process when segment is complete (before file close).

```c
// libavformat/segment.c - Add callback hook
typedef void (*segment_callback_fn)(
    void *opaque,
    const char *segment_path,
    int64_t start_pts,
    int64_t end_pts,
    int64_t size_bytes,
    int is_keyframe_aligned
);

typedef struct SegmentContext {
    // ... existing fields ...
    segment_callback_fn on_segment_complete;
    void *callback_opaque;
} SegmentContext;

// In seg_write_packet, after segment finishes:
if (seg->on_segment_complete) {
    seg->on_segment_complete(
        seg->callback_opaque,
        seg->current_segment_path,
        seg->segment_start_pts,
        seg->segment_end_pts,
        seg->current_segment_size,
        seg->starts_with_keyframe
    );
}
```

**Integration (via shared library):**
```go
// #cgo LDFLAGS: -lffmpeg_viewra
// void register_segment_callback(void* ctx, segment_callback_fn fn);
import "C"

//export onSegmentComplete
func onSegmentComplete(opaque unsafe.Pointer, path *C.char, ...) {
    session := (*TranscodeSession)(opaque)
    session.notifySegmentReady(C.GoString(path))  // Instant notification!
}
```

**Benefit:** Eliminates fsnotify polling entirely—segments available the microsecond they're ready.

---

### P3. Content Analysis Side-Channel

**Concept:** FFmpeg decoder emits content analysis as side-data during transcoding.

```c
// libavfilter/vf_analyze.c - New filter
typedef struct ContentAnalysis {
    float motion_vector_avg;     // Average motion magnitude
    float spatial_complexity;    // Edge density / texture
    float temporal_complexity;   // Scene change likelihood
    float luminance_histogram[16]; // Compressed histogram
    uint8_t film_grain_detected;
    uint8_t animation_likelihood; // 0-255
} ContentAnalysis;

// Emit as frame side-data
AVFrameSideData *sd = av_frame_new_side_data(
    frame, AV_FRAME_DATA_CONTENT_ANALYSIS, sizeof(ContentAnalysis));
memcpy(sd->data, &analysis, sizeof(analysis));
```

**Use in ViewRA:**
```go
func (s *TranscodeSession) onFrameAnalysis(analysis ContentAnalysis) {
    // Dynamic codec/quality decisions during encode
    if analysis.FilmGrainDetected && !s.grainSynthesisEnabled {
        s.enableGrainSynthesis()  // Enable mid-stream!
    }

    if analysis.MotionVectorAvg > 50 {
        s.boostBitrate(1.2)  // More bits for action scenes
    }
}
```

**Benefit:** Real-time content-aware encoding decisions.

---

### P4. Dynamic Encoder Parameter Injection

**Concept:** FFmpeg reads a control file/socket for live parameter updates.

```c
// libavcodec/nvenc.c - Add dynamic control
static void check_dynamic_params(NvencContext *ctx) {
    if (ctx->control_fd > 0) {
        DynamicParams params;
        if (read_nonblocking(ctx->control_fd, &params)) {
            if (params.target_bitrate != ctx->encode_config.rcParams.averageBitRate) {
                // Adjust on next keyframe
                ctx->pending_bitrate_change = params.target_bitrate;
            }
            if (params.force_keyframe) {
                ctx->force_idr = 1;
            }
        }
    }
}
```

**Control Protocol:**
```go
type DynamicControl struct {
    conn *net.UnixConn
}

func (c *DynamicControl) SetBitrate(bps int64) {
    msg := DynamicParams{TargetBitrate: bps}
    binary.Write(c.conn, binary.LittleEndian, &msg)
}

func (c *DynamicControl) ForceKeyframe() {
    msg := DynamicParams{ForceKeyframe: true}
    binary.Write(c.conn, binary.LittleEndian, &msg)
}
```

**Benefit:** ABR-like quality adaptation without restarting FFmpeg.

---

### P5. Predictive Keyframe Placement

**Concept:** Scene detector that predicts keyframes aligned to segment boundaries.

```c
// libavfilter/vf_scene_predict.c
typedef struct ScenePredictor {
    float scene_threshold;
    int segment_duration_frames;
    int current_segment_frame;

    // Lookahead buffer
    AVFrame *lookahead[LOOKAHEAD_FRAMES];
    int lookahead_count;
} ScenePredictor;

static int predict_optimal_keyframe(ScenePredictor *sp) {
    // Analyze next N frames for scene changes
    for (int i = 0; i < sp->lookahead_count; i++) {
        float change = calculate_scene_change(
            sp->lookahead[i], sp->lookahead[i+1]);

        if (change > sp->scene_threshold) {
            // Scene change at frame i
            // Find nearest segment boundary
            int frames_to_boundary = sp->segment_duration_frames -
                (sp->current_segment_frame % sp->segment_duration_frames);

            if (abs(i - frames_to_boundary) < 10) {
                // Close enough - align keyframe to scene change
                return i;
            }
        }
    }
    return sp->segment_duration_frames; // Default boundary
}
```

**Benefit:** Better quality at scene transitions while maintaining HLS alignment.

---

### P6. Hardware Encoder Queue Depth Exposure

**Concept:** Expose internal encoder queue metrics for health monitoring.

```c
// libavcodec/nvenc.c
typedef struct NvencQueueStats {
    int pending_input;     // Frames waiting to encode
    int pending_output;    // Encoded frames waiting to read
    int total_capacity;    // Max queue depth
    int64_t oldest_pts;    // Oldest pending frame PTS
    int64_t stall_count;   // Times queue was full
} NvencQueueStats;

int ff_nvenc_get_queue_stats(AVCodecContext *avctx, NvencQueueStats *stats);

// Also expose via AVOption for runtime query:
{ "queue_stats", "Get encoder queue statistics", OFFSET(queue_stats),
  AV_OPT_TYPE_BINARY, .flags = AV_OPT_FLAG_EXPORT | AV_OPT_FLAG_READONLY }
```

**Health Monitor Integration:**
```go
func (m *HealthMonitor) pollQueueDepth() {
    stats := m.ffmpeg.GetQueueStats()

    if stats.PendingInput > stats.TotalCapacity * 0.8 {
        m.emit(HealthWarning{
            Type: "queue_pressure",
            Value: float64(stats.PendingInput) / float64(stats.TotalCapacity),
            Suggestion: "reduce_input_rate",
        })
    }

    if stats.StallCount > m.lastStallCount {
        m.emit(HealthCritical{
            Type: "encoder_stall",
            Suggestion: "fallback_software",
        })
    }
}
```

**Benefit:** Predict encoder overload before it causes visible stuttering.

---

### P7. Zero-Copy Subtitle Compositor

**Concept:** GPU-based subtitle overlay that operates on GPU memory directly.

```c
// libavfilter/vf_overlay_gpu.c
// Instead of:
//   GPU decode → CPU → subtitle overlay → CPU → GPU encode
// Do:
//   GPU decode → GPU subtitle texture → GPU composite → GPU encode

typedef struct GPUOverlayContext {
    CUgraphicsResource video_resource;
    CUgraphicsResource subtitle_resource;
    CUfunction composite_kernel;
} GPUOverlayContext;

// CUDA kernel for compositing
__global__ void composite_subtitle(
    uchar4 *video,
    uchar4 *subtitle,
    int width, int height
) {
    int x = blockIdx.x * blockDim.x + threadIdx.x;
    int y = blockIdx.y * blockDim.y + threadIdx.y;

    if (x < width && y < height) {
        uchar4 v = video[y * width + x];
        uchar4 s = subtitle[y * width + x];

        // Alpha blend
        float alpha = s.w / 255.0f;
        video[y * width + x] = make_uchar4(
            v.x * (1-alpha) + s.x * alpha,
            v.y * (1-alpha) + s.y * alpha,
            v.z * (1-alpha) + s.z * alpha,
            255
        );
    }
}
```

**Benefit:** Full GPU pipeline with burned-in subtitles (no CPU roundtrip).

---

### P8. Segment-Level Quality Targeting

**Concept:** Per-segment quality targets based on content complexity.

```c
// libavformat/segment.c
typedef struct SegmentQualityTarget {
    int segment_index;
    float target_vmaf;        // Target perceptual quality
    int max_bitrate;          // Ceiling
    int min_bitrate;          // Floor
} SegmentQualityTarget;

// Read targets from sidecar file
static int load_quality_targets(SegmentContext *seg) {
    char targets_path[1024];
    snprintf(targets_path, sizeof(targets_path),
             "%s.quality_targets", seg->output_path);

    // Parse JSON/binary targets per segment
}

// Apply to encoder before segment starts
static void apply_segment_target(SegmentContext *seg, int seg_num) {
    SegmentQualityTarget *target = &seg->targets[seg_num];
    // Adjust rate control for this segment
}
```

**Use Case:**
```go
// Pre-analyze content in background job
func (a *Analyzer) GenerateQualityTargets(mediaID int64, profile string) {
    for segNum := 0; segNum < totalSegments; segNum++ {
        complexity := a.analyzeSegment(mediaID, segNum)
        target := SegmentQualityTarget{
            SegmentIndex: segNum,
            TargetVMAF: 92.0,  // Consistent perceptual quality
            MaxBitrate: calculateMax(complexity),
            MinBitrate: calculateMin(complexity),
        }
        targets = append(targets, target)
    }
    a.saveTargets(mediaID, profile, targets)
}
```

**Benefit:** Consistent perceptual quality across varying content.

---

### P9. Frame-Accurate Progress Reporting

**Concept:** Machine-readable progress on dedicated fd instead of stderr parsing.

```c
// ffmpeg.c - Add structured progress output
typedef struct ProgressReport {
    uint32_t magic;           // 0xFFPROG01
    uint64_t frame;
    uint64_t pts;
    uint64_t dts;
    uint32_t fps_x100;        // FPS * 100 for precision
    uint32_t speed_x100;      // Speed * 100
    uint32_t bitrate_kbps;
    uint32_t size_kb;
    uint8_t  is_keyframe;
    uint8_t  quality;         // Normalized 0-255
    uint16_t reserved;
} __attribute__((packed)) ProgressReport;

static void emit_progress(OutputStream *ost) {
    if (progress_fd > 0) {
        ProgressReport pr = {
            .magic = 0xFFPROG01,
            .frame = ost->frame_number,
            .pts = ost->last_pts,
            // ... fill all fields
        };
        write(progress_fd, &pr, sizeof(pr));
    }
}
```

**Usage:**
```bash
ffmpeg ... -progress_fd 3 ...  3>/tmp/progress.pipe
```

**Go Consumer:**
```go
type ProgressReader struct {
    pipe *os.File
}

func (r *ProgressReader) Read() (*Progress, error) {
    var pr ProgressReport
    if err := binary.Read(r.pipe, binary.LittleEndian, &pr); err != nil {
        return nil, err
    }
    return &Progress{
        Frame: pr.Frame,
        FPS: float64(pr.FpsX100) / 100.0,
        Speed: float64(pr.SpeedX100) / 100.0,
        // ... map all fields
    }, nil
}
```

**Benefit:** 100x more efficient than regex parsing stderr, zero ambiguity.

---

### P10. Graceful Degradation Mode

**Concept:** FFmpeg detects imminent failure and degrades quality rather than crashing.

```c
// libavcodec/nvenc.c
typedef enum DegradationLevel {
    DEGRADE_NONE = 0,
    DEGRADE_REDUCE_QUALITY,    // Increase QP
    DEGRADE_REDUCE_RESOLUTION, // Scale down
    DEGRADE_FALLBACK_SOFTWARE, // Switch encoder
} DegradationLevel;

static DegradationLevel check_resource_pressure(NvencContext *ctx) {
    NvencResourceStats stats;
    nvenc_get_resource_stats(ctx, &stats);

    if (stats.memory_pressure > 95) {
        return DEGRADE_REDUCE_RESOLUTION;
    }
    if (stats.encode_queue_full_ratio > 0.9) {
        return DEGRADE_REDUCE_QUALITY;
    }
    if (stats.consecutive_errors > 3) {
        return DEGRADE_FALLBACK_SOFTWARE;
    }
    return DEGRADE_NONE;
}

static void apply_degradation(NvencContext *ctx, DegradationLevel level) {
    switch (level) {
    case DEGRADE_REDUCE_QUALITY:
        ctx->encode_config.rcParams.constQP.qpInterP += 4;
        emit_degradation_event(ctx, "quality_reduced");
        break;
    case DEGRADE_REDUCE_RESOLUTION:
        ctx->pending_scale_factor = 0.75;
        emit_degradation_event(ctx, "resolution_reduced");
        break;
    case DEGRADE_FALLBACK_SOFTWARE:
        ctx->fallback_requested = 1;
        emit_degradation_event(ctx, "software_fallback");
        break;
    }
}
```

**Benefit:** Continuous playback with degraded quality beats stuttering or crashes.

---

### Patch Implementation Priority

| Patch | Difficulty | Value | Prerequisites |
|-------|------------|-------|---------------|
| P9 (Progress FD) | LOW | HIGH | None |
| P6 (Queue Depth) | MEDIUM | HIGH | NVENC understanding |
| P1 (Telemetry Socket) | MEDIUM | HIGH | Build from P9 |
| P2 (Segment Callback) | MEDIUM | HIGH | libavformat muxer knowledge |
| P4 (Dynamic Params) | MEDIUM | MEDIUM | Encoder internals |
| P10 (Graceful Degradation) | HIGH | HIGH | P6 + encoder experience |
| P3 (Content Analysis) | HIGH | MEDIUM | Filter chain knowledge |
| P8 (Segment Quality) | HIGH | MEDIUM | Rate control expertise |
| P5 (Predictive Keyframes) | HIGH | MEDIUM | Scene detection |
| P7 (Zero-Copy Subtitles) | VERY HIGH | HIGH | CUDA/GPU programming |

---

## Priority Matrix for ViewRA

Combining competitor research with original ideas:

### Immediate (Next Sprint)

| ID | Improvement | Type | Effort | Impact |
|----|-------------|------|--------|--------|
| Q1 | Film Grain Synthesis | Param change | LOW | HIGH |
| V9 | Subtitle Pre-extraction | New feature | LOW | MEDIUM |
| V1 | Health Monitoring (basic) | Enhancement | MEDIUM | HIGH |

### Short-term (1-2 Months)

| ID | Improvement | Type | Effort | Impact |
|----|-------------|------|--------|--------|
| P1 | Port tonemapx SIMD | FFmpeg patch | MEDIUM | HIGH |
| H3 | GPU Subtitle Burn-in | FFmpeg patch | MEDIUM | HIGH |
| V3 | Adaptive Watchdog | Enhancement | MEDIUM | MEDIUM |
| V8 | Bandwidth Estimation | New feature | MEDIUM | MEDIUM |

### Medium-term (3-6 Months)

| ID | Improvement | Type | Effort | Impact |
|----|-------------|------|--------|--------|
| P2 | VideoToolbox Fix | FFmpeg patch | MEDIUM | HIGH |
| V2 | Session Preloading | New feature | HIGH | HIGH |
| V5 | Segment Sharing | New feature | HIGH | MEDIUM |
| V6 | Content-Aware Codec | New feature | HIGH | HIGH |

### Long-term (Research)

| ID | Improvement | Type | Effort | Impact |
|----|-------------|------|--------|--------|
| L1 | LL-HLS Support | FFmpeg + App | HIGH | MEDIUM |
| V7 | FFmpeg Process Pool | Architecture | HIGH | MEDIUM |
| V10 | Multi-GPU Balancing | New feature | HIGH | MEDIUM |

---

*Last updated: December 2025*
