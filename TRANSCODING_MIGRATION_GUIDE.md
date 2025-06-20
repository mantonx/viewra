# Transcoding System Migration Guide

This guide helps you migrate from the old FFmpeg-specific transcoding system to the new generalized transcoding plugin architecture.

## Overview

The new transcoding system introduces:
- Generic quality settings (0-100% instead of CRF values)
- Provider-agnostic speed priorities
- Standardized progress reporting
- Support for multiple transcoding backends

## Configuration Migration

### Old Configuration Format
```json
{
  "enabled": true,
  "ffmpeg": {
    "path": "ffmpeg",
    "threads": 0
  },
  "transcoding": {
    "video_codec": "h264",
    "video_preset": "fast",
    "video_crf": 23,
    "audio_codec": "aac",
    "audio_bitrate": 128,
    "output_dir": "/viewra-data/transcoding"
  },
  "hardware": {
    "enabled": true,
    "type": "nvenc"
  },
  "performance": {
    "max_concurrent": 10,
    "timeout_minutes": 120
  }
}
```

### New Configuration Format
```json
{
  "core": {
    "enabled": true,
    "priority": 50,
    "output_directory": "/viewra-data/transcoding"
  },
  "hardware": {
    "enabled": true,
    "preferred_type": "cuda",
    "device_selection": "auto",
    "fallback": true
  },
  "sessions": {
    "max_concurrent": 10,
    "timeout_minutes": 120,
    "idle_minutes": 10
  },
  "cleanup": {
    "enabled": true,
    "retention_hours": 2,
    "extended_hours": 8,
    "max_size_gb": 10,
    "interval_minutes": 30
  },
  "debug": {
    "enabled": false,
    "log_level": "info"
  },
  "ffmpeg": {
    "binary_path": "ffmpeg",
    "probe_path": "ffprobe",
    "threads": 0,
    "default_crf": {
      "h264": 23,
      "h265": 28,
      "vp9": 31,
      "av1": 30
    },
    "default_preset": "fast",
    "audio_bitrate": 128,
    "two_pass": false,
    "log_ffmpeg_output": false
  }
}
```

## Quality Mapping

### CRF to Quality Percentage
The new system uses a 0-100% quality scale instead of codec-specific values:

| Old CRF | New Quality % | Description |
|---------|---------------|-------------|
| 18      | 85%          | Very High Quality |
| 23      | 75%          | High Quality (Default) |
| 28      | 65%          | Medium Quality |
| 32      | 55%          | Low Quality |
| 36      | 45%          | Very Low Quality |

Formula: `quality = 100 - (crf * 100 / 51)`

### Preset to Speed Priority
| Old Preset | New Speed Priority | Description |
|------------|-------------------|-------------|
| ultrafast  | fastest          | Maximum speed |
| superfast  | fastest          | Very fast encoding |
| veryfast   | fastest          | Fast encoding |
| faster     | balanced         | Faster than default |
| fast       | balanced         | Default balance |
| medium     | balanced         | Medium speed |
| slow       | quality          | Better quality |
| slower     | quality          | Much better quality |
| veryslow   | quality          | Best quality |

## API Changes

### Old Request Format
```json
{
  "input_path": "/path/to/input.mp4",
  "target_codec": "h264",
  "target_container": "mp4",
  "resolution": "1080p",
  "bitrate": 5000,
  "audio_codec": "aac",
  "audio_bitrate": 128,
  "quality": 23,
  "preset": "fast"
}
```

### New Request Format
```json
{
  "input_path": "/path/to/input.mp4",
  "quality": 75,
  "speed_priority": "balanced",
  "container": "mp4",
  "video_codec": "h264",
  "audio_codec": "aac",
  "resolution": {
    "width": 1920,
    "height": 1080
  },
  "prefer_hardware": true,
  "hardware_type": "cuda"
}
```

## Session Directory Changes

### Old Format
- `ffmpeg_1750335007063085318/`
- `dash_1750335007063085318/`

### New Format
- `mp4_ffmpeg_transcoder_550e8400-e29b-41d4-a716-446655440000/`
- `dash_nvenc_transcoder_6ba7b810-9dad-11d1-80b4-00c04fd430c8/`

Pattern: `[container]_[provider]_[UUID]`

## Frontend Changes

### VideoPlayer Component
Update quality selection to use percentage:
```typescript
// Old
const quality = 23; // CRF value

// New  
const quality = 75; // 75% quality
```

### Progress Display
```typescript
// Old
progress: 0.45 // 0-1 float

// New
progress: {
  percent_complete: 45,
  time_elapsed: 120000000000, // nanoseconds
  time_remaining: 180000000000,
  bytes_read: 104857600,
  bytes_written: 52428800,
  current_speed: 1.2,
  average_speed: 1.1
}
```

## Database Migration

### Update Configurations
```sql
-- Update plugin configurations to new format
UPDATE plugin_configurations 
SET version = '2.0.0',
    settings = '{"core": {...}, "hardware": {...}, ...}'
WHERE plugin_id = 'ffmpeg_transcoder';
```

### Update Active Sessions
```sql
-- Add provider field to sessions
ALTER TABLE transcode_sessions ADD COLUMN provider VARCHAR(50) DEFAULT 'ffmpeg';

-- Update existing sessions
UPDATE transcode_sessions 
SET provider = 'ffmpeg'
WHERE provider IS NULL;
```

## Plugin Development

To create a new transcoding provider:

1. Implement the `TranscodingProvider` interface
2. Create quality mapper for your provider
3. Implement hardware detection (if applicable)
4. Create the transcoding executor
5. Register with the plugin system

Example structure:
```
my_transcoder/
├── plugin.cue
├── main.go
├── internal/
│   ├── provider/
│   │   └── provider.go      # Implements TranscodingProvider
│   ├── quality/
│   │   └── mapper.go         # Quality mapping logic
│   ├── hardware/
│   │   └── detector.go       # Hardware detection
│   └── executor/
│       └── executor.go       # Transcoding execution
```

## Troubleshooting

### Common Issues

1. **Quality looks different**: The new quality scale is linear 0-100%, while CRF was logarithmic. Adjust accordingly.

2. **Hardware acceleration not working**: Check the `hardware_type` field matches your system:
   - NVIDIA: `"cuda"`
   - Intel: `"qsv"`
   - AMD: `"amf"` (Windows) or `"vaapi"` (Linux)
   - macOS: `"videotoolbox"`

3. **Sessions not found**: Session IDs now use UUIDs. Old numeric IDs won't work.

4. **Configuration not loading**: Ensure the configuration follows the new nested structure.

## Best Practices

1. **Always specify quality**: Default quality is provider-dependent
2. **Use speed priorities**: More portable than specific presets
3. **Enable hardware fallback**: Ensures transcoding works even without GPU
4. **Monitor progress**: New progress format provides richer information
5. **Clean up old sessions**: Directory naming has changed

## Support

For migration assistance:
- Check logs for specific error messages
- Verify configuration format matches examples
- Test with simple transcoding requests first
- Enable debug mode for detailed logging 