# FFmpeg Transcoding SDK

This package provides a comprehensive FFmpeg command builder for video transcoding with a focus on adaptive bitrate streaming (DASH/HLS) and high-quality output.

## Architecture

The FFmpeg SDK is organized into several key components:

### 1. Type Definitions (`types.go`)
Defines the core data structures used throughout the FFmpeg package:
- `FFmpegArgs`: Complete argument structure
- `VideoCodecArgs`: Video codec-specific settings
- `AudioCodecArgs`: Audio codec-specific settings
- `ContainerArgs`: Container format settings
- `ABRStreamConfig`: Adaptive bitrate stream configuration

### 2. Argument Reference (`args_reference.go`)
Comprehensive documentation of all FFmpeg arguments organized by category:
- **GlobalArgs**: Options that appear before input (e.g., `-y`, `-hide_banner`)
- **InputArgs**: Input file and seeking options (e.g., `-i`, `-ss`)
- **VideoEncodingArgs**: Video codec and quality settings (e.g., `-c:v`, `-crf`)
- **AudioEncodingArgs**: Audio codec and quality settings (e.g., `-c:a`, `-b:a`)
- **StreamMappingArgs**: Stream selection and mapping (e.g., `-map`)
- **ContainerOptions**: Format-specific options (e.g., DASH, HLS settings)
- **HardwareArgs**: Hardware acceleration options

### 3. Argument Builder (`args.go`)
The main builder that constructs optimized FFmpeg commands based on transcoding requests.

## Key FFmpeg Arguments Explained

### Video Quality Control

#### CRF (Constant Rate Factor)
```bash
-crf 23  # Lower = better quality, higher file size (0-51 for x264/x265)
```
- 0: Lossless
- 18-23: High quality (recommended for archival)
- 23-28: Good quality (recommended for streaming)
- 28-35: Acceptable quality (lower bandwidth)

#### Bitrate Control (VBV)
```bash
-b:v 5M         # Target bitrate
-maxrate 6M     # Maximum bitrate
-bufsize 10M    # VBV buffer size
```
VBV (Video Buffering Verifier) ensures compatibility with streaming protocols.

### Keyframe Alignment

Critical for adaptive streaming to ensure segments start with keyframes:

```bash
-g 60              # GOP size (keyframe interval in frames)
-keyint_min 60     # Minimum GOP size
-sc_threshold 0    # Disable scene change detection
-force_key_frames "expr:gte(t,n_forced*2)"  # Force keyframe every 2 seconds
```

### Container-Specific Settings

#### DASH
```bash
-f dash                                    # DASH format
-seg_duration 2                           # 2-second segments
-use_timeline 1                           # Enable SegmentTimeline
-use_template 1                           # Enable SegmentTemplate
-init_seg_name "init-$RepresentationID$.m4s"
-media_seg_name "chunk-$RepresentationID$-$Number$.m4s"
```

#### HLS
```bash
-f hls                                    # HLS format
-hls_time 2                              # 2-second segments
-hls_list_size 0                         # Keep all segments in playlist
-hls_segment_type fmp4                   # Use fragmented MP4
-hls_flags independent_segments          # Each segment is independent
```

### Audio Normalization

Prevents audio level variations and artifacts:

```bash
-af "aresample=async=1:first_pts=0"      # Audio resampling for sync
-c:a aac                                 # AAC codec
-b:a 128k                                # 128 kbps bitrate
-ar 48000                                # 48 kHz sample rate
-ac 2                                    # Stereo
```

## Usage Example

```go
import "github.com/mantonx/viewra/sdk/transcoding/ffmpeg"

// Create builder
builder := ffmpeg.NewFFmpegArgsBuilder(logger)

// Build transcoding request
req := types.TranscodeRequest{
    InputPath:    "/input/video.mp4",
    OutputPath:   "/output/stream.mpd",
    Container:    "dash",
    VideoCodec:   "h264",
    AudioCodec:   "aac",
    Quality:      720,
    EnableABR:    true,
}

// Generate FFmpeg arguments
args := builder.BuildArgs(req, req.OutputPath)

// Execute FFmpeg
cmd := exec.Command("ffmpeg", args...)
```

## Common FFmpeg Patterns

### 1. Fast First Segment
For quick stream startup, use shorter initial segments:
```bash
-min_seg_duration 1000000  # 1-second minimum
-seg_duration 2            # 2-second target
```

### 2. Closed GOPs
For better seeking and segment independence:
```bash
-flags +cgop               # Closed GOP
-g 60 -keyint_min 60      # Fixed GOP size
```

### 3. Hardware Acceleration
For faster encoding with GPU:
```bash
# NVIDIA
-hwaccel cuda -hwaccel_device 0
-c:v h264_nvenc

# Intel Quick Sync
-hwaccel qsv
-c:v h264_qsv

# Apple VideoToolbox
-hwaccel videotoolbox
-c:v h264_videotoolbox
```

### 4. Multi-pass Encoding
For optimal quality/size ratio:
```bash
# Pass 1
-pass 1 -passlogfile /tmp/pass

# Pass 2
-pass 2 -passlogfile /tmp/pass
```

## Troubleshooting

### Common Issues

1. **"Unrecognized option" errors**
   - Check FFmpeg version compatibility
   - Some options are codec or muxer specific

2. **Segment alignment issues**
   - Ensure GOP size matches segment duration × framerate
   - Disable scene change detection with `-sc_threshold 0`

3. **Audio sync problems**
   - Use audio resampling: `-af aresample=async=1`
   - Ensure consistent sample rates

4. **Manifest not found**
   - Check output directory permissions
   - Ensure container format is correct (`-f dash` or `-f hls`)

## Performance Optimization

1. **Thread Usage**
   ```bash
   -threads 0           # Auto-detect CPU threads
   -filter_threads 4    # Threads for filtering
   ```

2. **Preset Selection**
   - `ultrafast`: Real-time encoding, lower quality
   - `fast`: Good balance for live streaming
   - `medium`: Default, good quality/speed balance
   - `slow`: Better quality, suitable for VOD

3. **Buffer Sizes**
   ```bash
   -thread_queue_size 512   # Larger queue for smoother processing
   -max_muxing_queue_size 1024  # Prevent muxer queue overflow
   ```

## References

- [FFmpeg Documentation](https://ffmpeg.org/documentation.html)
- [x264 Settings Guide](https://trac.ffmpeg.org/wiki/Encode/H.264)
- [DASH Muxer Options](https://ffmpeg.org/ffmpeg-formats.html#dash-2)
- [HLS Muxer Options](https://ffmpeg.org/ffmpeg-formats.html#hls-2)