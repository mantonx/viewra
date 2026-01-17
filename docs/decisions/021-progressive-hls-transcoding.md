# ADR 017: Progressive HLS Transcoding

## Status

Proposed

## Context

After implementing and testing segment-based on-demand transcoding (ADR-016), we discovered it cannot keep up with continuous playback. The fundamental issue is that individual FFmpeg invocations for each segment have too much overhead (~50-100ms startup + 150-250ms generation), and HLS.js players request many segments at once to fill their buffer.

### What We Learned from ADR-016

**Problems with individual segment generation:**

- Each segment takes 250-300ms to generate (even with copy codec)
- FFmpeg startup/seeking overhead adds 50-100ms per invocation
- HLS.js requests 10-20 segments immediately to fill buffer
- Even with 8 parallel workers, cannot generate fast enough
- Video buffers after 10-20 seconds when generation queue backs up

**What actually works:**

- ✅ Direct Play for compatible files (302 redirect)
- ✅ Seeking is handled well (can jump to any segment)
- ❌ Continuous playback fails (generation too slow)

### Progressive Approach

Research into effective HLS transcoding revealed:

1. **Single long-running FFmpeg process** per transcode session
2. FFmpeg transcodes continuously and writes segments progressively
3. Segments become available as FFmpeg generates them
4. When user seeks, **kill and restart** FFmpeg from new position
5. Temporary segments stored in a cache directory

This is fundamentally different from what we implemented in ADR-016.

## Decision

We will implement **Progressive HLS Transcoding** using a single long-running FFmpeg process that continuously generates segments.

### Core Architecture

```
User Request → Manifest → Start FFmpeg Session → FFmpeg writes segments progressively → Serve segments as they become available
```

**Key Components:**

1. **Transcode Session**: Long-running FFmpeg process managed as a session
2. **Session Manager**: Tracks active sessions, handles lifecycle
3. **Segment Monitor**: Watches output directory for new segments
4. **Seek Handler**: Kills old session, starts new one from seek position

### How It Works

#### Initial Playback

```
1. User requests: GET /api/media/123/hls/720p/playlist.m3u8
2. Server checks: Is video compatible? → Yes: 302 redirect to direct play
3. Server checks: Is video compatible? → No: Create transcode session
4. Start FFmpeg process:
   ffmpeg -i input.mkv \
          -c:v libx264 -preset veryfast \
          -c:a aac \
          -f hls \
          -hls_time 6 \
          -hls_segment_filename "seg_%06d.ts" \
          -hls_list_size 0 \
          output.m3u8
5. FFmpeg runs continuously, writing segments: seg_000000.ts, seg_000001.ts, ...
6. Return manifest to client
7. Client requests seg_000000.ts
8. If segment exists: serve it
9. If segment doesn't exist yet: wait with timeout (segment is being generated)
```

#### Seeking

```
1. User seeks to 5:00 (300 seconds)
2. Client requests: GET /api/media/123/hls/720p/playlist.m3u8?start=300
3. Server calculates: segment 50 (300s / 6s = 50)
4. Kill existing FFmpeg session
5. Start new FFmpeg from position 300s:
   ffmpeg -ss 300 -i input.mkv ... (same args as before)
6. FFmpeg starts writing from seg_000050.ts
7. Return updated manifest starting at segment 50
8. Client requests seg_000050.ts
9. Wait for FFmpeg to generate it, then serve
```

## Implementation Design

### 1. TranscodeSession

```go
type TranscodeSession struct {
    ID            string
    MediaID       int64
    Quality       string
    StartPosition float64  // seconds

    FFmpegCmd     *exec.Cmd
    OutputDir     string
    ManifestPath  string

    CreatedAt     time.Time
    LastAccessed  time.Time

    segmentMutex  sync.RWMutex
    generatedSegments map[int]bool  // track which segments exist
}

// Start begins the FFmpeg transcoding process
func (s *TranscodeSession) Start(ctx context.Context) error {
    args := buildFFmpegArgs(s.MediaID, s.Quality, s.StartPosition, s.OutputDir)
    s.FFmpegCmd = exec.CommandContext(ctx, "ffmpeg", args...)

    // Start process
    if err := s.FFmpegCmd.Start(); err != nil {
        return err
    }

    // Watch for segment creation in background
    go s.watchSegments()

    return nil
}

// Stop kills the FFmpeg process
func (s *TranscodeSession) Stop() error {
    if s.FFmpegCmd != nil && s.FFmpegCmd.Process != nil {
        // Send 'q' to stdin for graceful shutdown
        s.FFmpegCmd.Process.Signal(syscall.SIGTERM)

        // Wait briefly, then force kill if needed
        time.Sleep(100 * time.Millisecond)
        s.FFmpegCmd.Process.Kill()
    }
    return nil
}

// watchSegments monitors output directory for new segments
func (s *TranscodeSession) watchSegments() {
    ticker := time.NewTicker(500 * time.Millisecond)
    defer ticker.Stop()

    for range ticker.C {
        // Scan output directory for .ts files
        files, _ := filepath.Glob(filepath.Join(s.OutputDir, "seg_*.ts"))

        s.segmentMutex.Lock()
        for _, file := range files {
            segNum := parseSegmentNumber(file)
            s.generatedSegments[segNum] = true
        }
        s.segmentMutex.Unlock()
    }
}

// WaitForSegment blocks until segment is available or timeout
func (s *TranscodeSession) WaitForSegment(segmentNum int, timeout time.Duration) (string, error) {
    deadline := time.Now().Add(timeout)

    for time.Now().Before(deadline) {
        s.segmentMutex.RLock()
        exists := s.generatedSegments[segmentNum]
        s.segmentMutex.RUnlock()

        if exists {
            return s.getSegmentPath(segmentNum), nil
        }

        time.Sleep(100 * time.Millisecond)
    }

    return "", fmt.Errorf("timeout waiting for segment %d", segmentNum)
}
```

### 2. SessionManager

```go
type SessionManager struct {
    sessions sync.Map  // map[string]*TranscodeSession
    logger   *slog.Logger
}

// GetOrCreateSession returns existing session or creates new one
func (m *SessionManager) GetOrCreateSession(
    mediaID int64,
    quality string,
    startPosition float64,
) (*TranscodeSession, error) {
    key := sessionKey(mediaID, quality)

    // Check for existing session
    if existing, ok := m.sessions.Load(key); ok {
        session := existing.(*TranscodeSession)

        // If seek is far from current position, restart
        if math.Abs(startPosition - session.StartPosition) > 30 {
            m.logger.Info("Restarting session for seek",
                "media_id", mediaID,
                "old_position", session.StartPosition,
                "new_position", startPosition)

            session.Stop()
            m.sessions.Delete(key)
            // Fall through to create new session
        } else {
            // Reuse existing session
            session.LastAccessed = time.Now()
            return session, nil
        }
    }

    // Create new session
    session := &TranscodeSession{
        ID:            uuid.New().String(),
        MediaID:       mediaID,
        Quality:       quality,
        StartPosition: startPosition,
        OutputDir:     filepath.Join(transcodeDir, fmt.Sprintf("%d_%s", mediaID, quality)),
        CreatedAt:     time.Now(),
        LastAccessed:  time.Now(),
        generatedSegments: make(map[int]bool),
    }

    // Create output directory
    os.MkdirAll(session.OutputDir, 0755)

    // Start FFmpeg
    if err := session.Start(context.Background()); err != nil {
        return nil, err
    }

    m.sessions.Store(key, session)
    return session, nil
}

// CleanupIdleSessions stops sessions that haven't been accessed recently
func (m *SessionManager) CleanupIdleSessions(idleTimeout time.Duration) {
    m.sessions.Range(func(key, value interface{}) bool {
        session := value.(*TranscodeSession)

        if time.Since(session.LastAccessed) > idleTimeout {
            m.logger.Info("Stopping idle session", "session_id", session.ID)
            session.Stop()
            m.sessions.Delete(key)

            // Clean up output directory
            os.RemoveAll(session.OutputDir)
        }

        return true
    })
}
```

### 3. Updated API Handlers

#### Manifest Handler

```go
func (h *TranscodeHandler) ServePlaylist(c *gin.Context) {
    mediaID := parseID(c.Param("id"))
    quality := c.Param("quality")
    startPosition := c.Query("start")  // optional, in seconds

    // Get media info
    media, err := h.mediaRepo.GetByID(c.Request.Context(), mediaID)
    if err != nil {
        c.JSON(404, gin.H{"error": "Media not found"})
        return
    }

    // Determine streaming strategy
    strategy := h.determineStrategy(media, quality)

    // Direct Play - redirect to direct stream
    if strategy == DirectPlay {
        c.Redirect(302, fmt.Sprintf("/api/stream/%d", mediaID))
        return
    }

    // Progressive transcoding - get or create session
    session, err := h.sessionManager.GetOrCreateSession(
        mediaID,
        quality,
        parseFloat(startPosition),  // defaults to 0 if not provided
    )
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to create transcode session"})
        return
    }

    // Generate manifest
    manifest := h.generateManifest(session, media.DurationSeconds)

    c.Header("Content-Type", "application/vnd.apple.mpegurl")
    c.String(200, manifest)
}
```

#### Segment Handler

```go
func (h *TranscodeHandler) ServeHLSSegment(c *gin.Context) {
    mediaID := parseID(c.Param("id"))
    quality := c.Param("quality")
    filename := c.Param("filename")
    segmentNum := parseSegmentNumber(filename)

    // Find active session
    session, err := h.sessionManager.GetSession(mediaID, quality)
    if err != nil {
        c.JSON(404, gin.H{"error": "No active transcode session"})
        return
    }

    // Wait for segment to be generated (timeout: 30 seconds)
    segmentPath, err := session.WaitForSegment(segmentNum, 30*time.Second)
    if err != nil {
        c.JSON(500, gin.H{"error": "Segment generation timeout"})
        return
    }

    // Serve segment
    c.Header("Content-Type", "video/mp2t")
    c.Header("Access-Control-Allow-Origin", "*")
    c.File(segmentPath)

    // Update access time
    session.LastAccessed = time.Now()
}
```

### 4. FFmpeg Command Building

```go
func buildFFmpegArgs(
    mediaID int64,
    quality string,
    startPosition float64,
    outputDir string,
) []string {
    profile := getQualityProfile(quality)
    media := getMedia(mediaID)

    args := []string{}

    // Hardware acceleration (if available)
    args = append(args, getHardwareAccelArgs()...)

    // Start position (if seeking)
    if startPosition > 0 {
        args = append(args, "-ss", fmt.Sprintf("%.3f", startPosition))
    }

    // Input
    args = append(args, "-i", media.FilePath)

    // Video encoding
    args = append(args,
        "-c:v", "libx264",
        "-preset", "veryfast",  // Fast encoding for real-time
        "-profile:v", "high",
        "-level", "4.1",
        "-b:v", profile.VideoBitrate,
        "-maxrate", profile.VideoMaxRate,
        "-bufsize", profile.VideoBufSize,
        "-vf", fmt.Sprintf("scale=%d:%d", profile.Width, profile.Height),
    )

    // Audio encoding
    args = append(args,
        "-c:a", "aac",
        "-b:a", profile.AudioBitrate,
        "-ac", "2",  // Stereo
    )

    // HLS settings
    args = append(args,
        "-f", "hls",
        "-hls_time", "6",  // 6-second segments
        "-hls_segment_filename", filepath.Join(outputDir, "seg_%06d.ts"),
        "-hls_list_size", "0",  // Include all segments in manifest
        "-hls_flags", "delete_segments+append_list",  // Clean up old segments
        "-start_number", fmt.Sprintf("%d", int(startPosition/6)),  // Start segment numbering from seek point
    )

    // Output manifest
    manifestPath := filepath.Join(outputDir, "playlist.m3u8")
    args = append(args, manifestPath)

    return args
}
```

## Implementation Plan

### Phase 1: Core Session Management (2 days)

1. Create `TranscodeSession` struct and lifecycle methods
2. Implement `SessionManager` with session tracking
3. Add session cleanup background task
4. Unit tests for session lifecycle

### Phase 2: Handler Integration (2 days)

1. Update manifest handler to create/reuse sessions
2. Update segment handler to wait for segment availability
3. Implement segment monitoring (watch output directory)
4. Integration tests for full playback flow

### Phase 3: Seek Handling (1 day)

1. Implement session restart logic for seeks
2. Handle edge cases (seek during startup, rapid seeks)
3. Test seeking scenarios

### Phase 4: Cleanup & Polish (1 day)

1. Add proper error handling and logging
2. Implement session timeout/cleanup
3. Add metrics (active sessions, segments generated)
4. Performance testing

## Consequences

### Positive

1. **Continuous playback works**: FFmpeg generates segments faster than playback needs them
2. **Proven approach**: Industry-standard technique for media servers
3. **Simple architecture**: Single FFmpeg process per session
4. **Resource efficient**: One FFmpeg per active user instead of N workers
5. **Seeking works**: Kill and restart from new position

### Negative

1. **Seek delay**: Must restart FFmpeg process (~1-3 seconds)
2. **Wasted work on seeks**: Discard segments from old session
3. **Session management**: Need to track and cleanup idle sessions
4. **Not ideal for scrubbing**: Rapid seeks cause many FFmpeg restarts

### Trade-offs vs ADR-016

| Aspect | ADR-016 (Individual Segments) | ADR-017 (Progressive) |
|--------|-------------------------------|----------------------|
| **Continuous playback** | ❌ Cannot keep up | ✅ Works well |
| **Seeking** | ✅ Instant (cached) | ⚠️ 1-3s delay (restart) |
| **Resource usage** | ⚠️ N workers always | ✅ 1 FFmpeg per user |
| **Complexity** | High (workers, queue) | Medium (sessions) |
| **Disk usage** | Medium (cache segments) | Low (temp segments) |
| **Scrubbing** | ✅ Cached segments | ❌ Poor (many restarts) |

**Decision:** ADR-017 is correct for continuous playback. May revisit segment caching later for scrubbing optimization.

## Success Metrics

- **Playback startup**: <3 seconds from manifest request to first segment
- **Continuous playback**: No buffering for sequential viewing
- **Seek latency**: <5 seconds from seek to playback resume
- **Resource usage**: <1 GB RAM per active transcode session
- **Session lifecycle**: Idle sessions cleaned up within 5 minutes

## Related Decisions

- **ADR-016: Segment-Based On-Demand Transcoding** - Rejected, replaced by this ADR
- **ADR-015: Video Player Enhancement Strategy** - Video player improvements
- **ADR-006: Progressive HLS Transcoding** - Original linear approach

## References

- [FFmpeg HLS Muxer Documentation](https://ffmpeg.org/ffmpeg-formats.html#hls-2)
- [HLS Specification RFC 8216](https://datatracker.ietf.org/doc/html/rfc8216)

---

**Created:** 2025-01-20
**Authors:** Claude Code AI
**Status:** Proposed - Ready for implementation
