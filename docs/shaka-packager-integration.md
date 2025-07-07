# Shaka Packager Integration for Real-Time VOD

## Overview

This document describes how Viewra integrates Shaka Packager to enable real-time VOD streaming with instant playback. The key innovation is using Shaka's live streaming features for VOD content, allowing playback to start within 2-3 seconds while transcoding continues in the background.

## Architecture

### Pipeline Flow

1. **FFmpeg Encoder** → Encodes video/audio to H.264/AAC
2. **Named Pipe (FIFO)** → Streams encoded data to Shaka
3. **Shaka Packager** → Packages into DASH/HLS segments in real-time
4. **Manifest Updates** → Dynamic manifest updates as segments become available

### Key Components

#### ShakaStreamEncoder (`shaka_stream_encoder.go`)
- Manages FFmpeg and Shaka Packager processes
- Creates named pipe for inter-process communication
- Monitors segment production
- Handles manifest updates

#### StreamingPipeline (`streaming_pipeline.go`)
- Orchestrates the encoding/packaging pipeline
- Manages session lifecycle
- Handles callbacks for segment/manifest events
- Integrates with content storage

## Configuration

### Enable Shaka Packager

In `provider.go`, set `UseShakaPackager: true`:

```go
config := &StreamingConfig{
    BaseDir:                p.baseDir,
    SegmentDuration:        4,  // 4 second segments
    BufferAhead:            12, // 12 seconds of buffer
    ManifestUpdateInterval: 2 * time.Second,
    EnableABR:              false, // Single quality for now
    ABRProfiles: []EncodingProfile{
        {Name: "720p", Width: 1280, Height: 720, VideoBitrate: 2500, Quality: 25},
    },
    UseShakaPackager: true, // Enable Shaka for low-latency
}
```

### Shaka Packager Arguments

Key arguments for low-latency VOD:

```bash
shaka-packager \
  'in=stream.pipe,stream=0,output=video.mp4,segment_template=video-$Number$.m4s' \
  'in=stream.pipe,stream=1,output=audio.mp4,segment_template=audio-$Number$.m4s' \
  --segment_duration 4 \
  --fragment_duration 4 \
  --mpd_output manifest.mpd \
  --generate_static_live_mpd \  # Key flag for live-style VOD
  --low_latency_dash_mode \     # Enable low-latency features
  --suggested_presentation_delay 8 \  # 2 segments buffer
  --time_shift_buffer_depth 3600 \   # 1 hour buffer
  --allow_approximate_segment_timeline
```

## Benefits

1. **Instant Playback**: Video starts playing within 2-3 seconds
2. **Real-Time Packaging**: Segments are available as soon as they're encoded
3. **Live Features for VOD**: Uses DASH live profile for VOD content
4. **Low Latency**: Optimized for minimal buffering
5. **Progressive Loading**: Content loads as user watches

## Testing

### Manual Test with FFmpeg + Shaka

```bash
# Create named pipe
mkfifo /tmp/test.pipe

# Start Shaka Packager (in one terminal)
shaka-packager \
  'in=/tmp/test.pipe,stream=0,output=/tmp/output/video.mp4,segment_template=/tmp/output/video-$Number$.m4s' \
  'in=/tmp/test.pipe,stream=1,output=/tmp/output/audio.mp4,segment_template=/tmp/output/audio-$Number$.m4s' \
  --segment_duration 4 \
  --mpd_output /tmp/output/manifest.mpd \
  --generate_static_live_mpd

# Start FFmpeg (in another terminal)
ffmpeg -i input.mp4 \
  -c:v libx264 -preset fast -crf 23 \
  -c:a aac -b:a 128k \
  -f mp4 -movflags frag_keyframe+empty_moov+default_base_moof \
  -frag_duration 4000000 \
  /tmp/test.pipe
```

### API Test

Use the provided test script:

```bash
./test-shaka-vod.sh
```

This will:
1. Start a transcoding session
2. Check manifest availability (should be < 3 seconds)
3. Monitor segment production
4. Verify playback readiness

## Performance Considerations

1. **Named Pipe Buffer**: The FIFO has limited buffer size. Ensure Shaka consumes data fast enough.
2. **Segment Duration**: 4 seconds provides good balance between latency and efficiency
3. **Fragment Duration**: Keep same as segment duration for VOD
4. **Suggested Presentation Delay**: 2 segments (8 seconds) allows smooth playback

## Troubleshooting

### Shaka Packager Not Starting
- Check if shaka-packager binary is in PATH
- Verify named pipe was created successfully
- Check process logs for errors

### Slow Manifest Generation
- Ensure FFmpeg is outputting fragmented MP4
- Check --fragment_duration matches segment duration
- Verify pipe is not blocking

### Playback Issues
- Ensure manifest type is "dynamic" for live features
- Check segment availability in manifest
- Verify correct MIME types are served

## Future Improvements

1. **Multi-Bitrate ABR**: Add support for adaptive bitrate streaming
2. **HLS Support**: Currently focused on DASH, add HLS support
3. **DRM Integration**: Shaka supports various DRM systems
4. **Thumbnail Generation**: Add thumbnail track support
5. **Subtitle Support**: Package subtitle tracks