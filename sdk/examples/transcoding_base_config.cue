// Base configuration schema that ALL transcoding providers must implement
// Providers can extend this with their own specific settings

#BaseTranscodingConfig: {
    // Core settings - all providers need these
    enabled:          bool | *true         @ui(basic,importance=10,description="Enable this transcoding provider")
    priority:         int | *50            @ui(basic,importance=9,description="Provider priority (higher = preferred)")
    max_sessions:     int | *5             @ui(basic,importance=8,description="Maximum concurrent sessions")
    session_timeout:  int | *7200          @ui(basic,importance=7,description="Session timeout in seconds")
    
    // Quality presets that users can select
    quality_presets: {
        low: {
            quality:     int | *30
            description: string | *"Fast encoding, smaller file size"
        }
        medium: {
            quality:     int | *50
            description: string | *"Balanced quality and speed"
        }
        high: {
            quality:     int | *70
            description: string | *"High quality, larger file size"
        }
        ultra: {
            quality:     int | *90
            description: string | *"Best quality, slow encoding"
        }
    } @ui(basic,importance=6,description="Quality presets available to users")
    
    // Hardware acceleration settings
    hardware: {
        enabled:        bool | *true          @ui(basic,importance=5,description="Enable hardware acceleration if available")
        auto_detect:    bool | *true          @ui(advanced,description="Automatically detect available hardware")
        preferred_type: string | *"any"       @ui(advanced,description="Preferred hardware type (nvidia, intel, amd, apple, any)")
    }
    
    // Resource limits
    resources: {
        max_cpu_percent: int | *80            @ui(advanced,description="Maximum CPU usage percentage")
        max_memory_mb:   int | *2048          @ui(advanced,description="Maximum memory usage in MB")
        max_gpu_percent: int | *90            @ui(advanced,description="Maximum GPU usage percentage")
    }
    
    // Advanced settings
    debug: {
        enabled:     bool | *false            @ui(advanced,description="Enable debug logging")
        log_level:   string | *"info"         @ui(advanced,description="Log level (debug, info, warn, error)")
        save_logs:   bool | *false            @ui(advanced,description="Save logs to file")
    }
}

// Example of how a provider would extend this base configuration:
//
// ffmpeg_config: #BaseTranscodingConfig & {
//     // FFmpeg-specific settings
//     ffmpeg: {
//         path:        string | *"ffmpeg"   @ui(advanced,description="Path to FFmpeg binary")
//         threads:     int | *0             @ui(advanced,description="Number of threads (0 = auto)")
//         preset:      string | *"medium"   @ui(basic,importance=4,description="FFmpeg preset")
//     }
// } 