package ffmpeg_transcoder

// Plugin metadata (required for discovery)
id:          "ffmpeg_transcoder"
name:        "FFmpeg Transcoder"
version:     "1.0.0"
type: "transcoder"
description: "High-performance video transcoding using FFmpeg with hardware acceleration support"
author:      "Viewra Team"
enabled_by_default: true

// Generic transcoding plugin configuration
core: {
    enabled: bool | *true 
        @ui(title="Enable Plugin", importance=10, is_basic=true)
    priority: int & >=1 & <=100 | *50 
        @ui(title="Plugin Priority", importance=9, is_basic=true)
    output_directory: string | *"/viewra-data/transcoding" 
        @ui(title="Output Directory", importance=8, is_basic=true)
}

// Generic hardware configuration  
hardware: {
    enabled: bool | *true 
        @ui(title="Enable Hardware Acceleration", importance=10, is_basic=true)
    preferred_type: "auto" | "none" | "cuda" | "vaapi" | "qsv" | "videotoolbox" | *"auto"
        @ui(title="Preferred Hardware Type", importance=9)
    device_selection: "auto" | "first" | "load-balanced" | *"auto"
        @ui(title="Device Selection", importance=8)
    fallback: bool | *true
        @ui(title="Fallback to Software", importance=7, is_basic=true)
}

// Generic session management
sessions: {
    max_concurrent: int & >=1 & <=100 | *10
        @ui(title="Max Concurrent Sessions", importance=10, is_basic=true)
    timeout_minutes: int & >=1 & <=1440 | *120
        @ui(title="Session Timeout (minutes)", importance=9)
    idle_minutes: int & >=1 & <=120 | *10
        @ui(title="Idle Timeout (minutes)", importance=8)
}

// Generic cleanup settings
cleanup: {
    enabled: bool | *true
        @ui(title="Enable Cleanup", importance=10, is_basic=true)
    retention_hours: int & >=1 & <=168 | *2
        @ui(title="Retention Hours", importance=9, is_basic=true)
    extended_hours: int & >=1 & <=720 | *8
        @ui(title="Extended Retention Hours", importance=8)
    max_size_gb: int & >=1 & <=1000 | *10
        @ui(title="Max Size (GB)", importance=7, is_basic=true)
    interval_minutes: int & >=1 & <=1440 | *30
        @ui(title="Cleanup Interval (minutes)", importance=6)
}

// Generic debug settings
debug: {
    enabled: bool | *false
        @ui(title="Enable Debug Mode", importance=10)
    log_level: "debug" | "info" | "warn" | "error" | *"info"
        @ui(title="Log Level", importance=9)
}

// FFmpeg-specific settings
ffmpeg: {
    binary_path: string | *"ffmpeg"
        @ui(title="FFmpeg Binary Path", importance=10, is_basic=true)
    probe_path: string | *"ffprobe"  
        @ui(title="FFprobe Binary Path", importance=9)
    threads: int & >=0 & <=32 | *0
        @ui(title="Thread Count (0=auto)", importance=8)
    
    // FFmpeg quality defaults
    default_crf: {
        h264: int & >=0 & <=51 | *23
            @ui(title="H.264 Default CRF", importance=7)
        h265: int & >=0 & <=51 | *28
            @ui(title="H.265 Default CRF", importance=6)
        vp9: int & >=0 & <=63 | *31
            @ui(title="VP9 Default CRF", importance=5)
        av1: int & >=0 & <=63 | *30
            @ui(title="AV1 Default CRF", importance=4)
    }
    
    default_preset: "ultrafast" | "superfast" | "veryfast" | "faster" | "fast" | "medium" | "slow" | "slower" | "veryslow" | *"fast"
        @ui(title="Default Encoding Preset", importance=7, is_basic=true)
    
    // Advanced FFmpeg options
    extra_args: [...string] | *[]
        @ui(title="Extra FFmpeg Arguments", importance=3)
    two_pass: bool | *false
        @ui(title="Enable Two-Pass Encoding", importance=4)
    audio_bitrate: int & >=64 & <=320 | *128
        @ui(title="Default Audio Bitrate (kbps)", importance=5, is_basic=true)
    log_ffmpeg_output: bool | *false
        @ui(title="Log FFmpeg Output", importance=2)
} 