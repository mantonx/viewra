# DASH/HLS DirectTranscoder Test Guide

## Overview

This guide tests the complete DASH/HLS implementation using the DirectTranscoder (not the plugin-based system).

## What We Just Fixed ✅

1. **DirectTranscoder DASH/HLS Streaming**: Fixed the critical "streaming for format not yet implemented" error
2. **DashHlsManifestReader**: Created specialized reader for manifest files
3. **Session Directory Paths**: Fixed path mismatch between DirectTranscoder and PlaybackModule
4. **Playback Planner**: Enhanced to intelligently choose DASH/HLS based on client capabilities

## Testing Steps

### 1. Basic DASH Test

```bash
# Start the Viewra backend
docker-compose up -d backend

# Test playback decision (should choose DASH for modern browsers)
curl -X POST http://localhost:8080/api/playback/decide \
  -H "Content-Type: application/json" \
  -d '{
    "media_path": "/media/your-video.mkv",
    "device_profile": {
      "user_agent": "Mozilla/5.0 (Chrome/120.0)",
      "supported_codecs": ["h264", "aac"],
      "max_resolution": "1080p",
      "max_bitrate": 8000
    }
  }'

# Expected Response:
# {
#   "should_transcode": true,
#   "transcode_params": {
#     "target_container": "dash",  <- DASH selected!
#     "target_codec": "h264",
#     "resolution": "1080p"
#   }
# }
```

### 2. Start DASH Transcoding Session

```bash
# Start a DASH transcoding session
curl -X POST http://localhost:8080/api/playback/start \
  -H "Content-Type: application/json" \
  -d '{
    "input_path": "/media/your-video.mkv",
    "target_codec": "h264",
    "target_container": "dash",
    "resolution": "1080p",
    "bitrate": 6000,
    "audio_codec": "aac",
    "quality": 23,
    "preset": "fast"
  }'

# Expected Response:
# {
#   "id": "session_abc123",
#   "status": "running",
#   "backend": "direct-ffmpeg"
# }
```

### 3. Test DASH Manifest Serving

```bash
# Get DASH manifest (should work now!)
curl http://localhost:8080/api/playback/stream/session_abc123/manifest.mpd

# Expected: Valid DASH MPD XML content
# Should start with: <?xml version="1.0"?>
# Should contain: <MPD>, <AdaptationSet>, <Representation>, etc.
```

### 4. Test Segment Serving

```bash
# List session directory contents to see segments
ls -la /tmp/viewra-transcode/session_abc123/

# Expected files:
# manifest.mpd
# init-video.m4s
# init-audio.m4s  
# chunk-video-1.m4s
# chunk-video-2.m4s
# chunk-audio-1.m4s
# chunk-audio-2.m4s

# Test segment serving
curl -I http://localhost:8080/api/playback/stream/session_abc123/segment/chunk-video-1.m4s

# Expected: 200 OK with video/iso.segment Content-Type
```

### 5. HLS Test

```bash
# Test HLS decision (for Safari/mobile)
curl -X POST http://localhost:8080/api/playback/decide \
  -H "Content-Type: application/json" \
  -d '{
    "media_path": "/media/your-video.mkv",
    "device_profile": {
      "user_agent": "Mozilla/5.0 (Safari/17.0) Mobile",
      "supported_codecs": ["h264", "aac"],
      "max_resolution": "1080p"
    }
  }'

# Should choose HLS: "target_container": "hls"

# Start HLS session
curl -X POST http://localhost:8080/api/playback/start \
  -H "Content-Type: application/json" \
  -d '{
    "input_path": "/media/your-video.mkv",
    "target_codec": "h264", 
    "target_container": "hls",
    "resolution": "720p"
  }'

# Test HLS playlist
curl http://localhost:8080/api/playback/stream/session_def456/playlist.m3u8

# Expected: Valid HLS M3U8 playlist content
# Should contain: #EXTM3U, #EXT-X-VERSION, segment_*.ts entries
```

### 6. Frontend Integration Test

```bash
# Start frontend
docker-compose up -d frontend

# Open browser to: http://localhost:5173
# Navigate to a TV episode
# Check browser console for:
# - "🎬 Adaptive streaming detected: DASH"
# - "📋 Loading manifest from: /api/playback/stream/.../manifest.mpd"
# - Successful video playback
```

## Debugging Common Issues

### Issue: "Session not found"
```bash
# Check active sessions
curl http://localhost:8080/api/playback/sessions

# Check session status
curl http://localhost:8080/api/playback/session/your_session_id
```

### Issue: "Manifest not available"
```bash
# Check FFmpeg logs
docker-compose logs backend | grep ffmpeg

# Check session directory
ls -la /tmp/viewra-transcode/session_*/
```

### Issue: Segments not found
```bash
# Verify segment files exist
find /tmp/viewra-transcode -name "*.m4s" -o -name "*.ts"

# Check segment request logs
docker-compose logs backend | grep "serving segment"
```

## Architecture Summary

```
Frontend (Shaka Player)
    ↓ Requests manifest
PlaybackModule
    ↓ Routes to handleDashManifest/handleHlsPlaylist  
SimpleTranscodeManager
    ↓ Wraps in DirectTranscoderWrapper
DirectTranscoder
    ↓ Uses DashHlsManifestReader
FFmpeg Process
    ↓ Creates manifest.mpd/playlist.m3u8 + segments
File System (/tmp/viewra-transcode/session_*)
```

## Success Criteria ✅

- [ ] Playback planner chooses DASH/HLS appropriately
- [ ] DirectTranscoder starts DASH/HLS sessions without errors
- [ ] Manifest files are served correctly
- [ ] Segment files are served correctly  
- [ ] Shaka Player successfully loads and plays adaptive streams
- [ ] No "streaming for format not yet implemented" errors
- [ ] Proper cleanup of session files

This implementation provides **full DASH/HLS support** using the DirectTranscoder! 