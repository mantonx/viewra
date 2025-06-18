package ffmpeg_transcoder

// Plugin metadata
plugin: {
	id:          "ffmpeg_transcoder"
	name:        "FFmpeg Transcoder"
	version:     "1.0.0"
	type:        "transcoder"
	description: "FFmpeg-based video transcoding service with comprehensive codec support and streaming capabilities"
	author:      "Viewra Team"
	
	// Entry points for plugin execution
	entry_points: {
		main: "ffmpeg_transcoder"
	}
}

// Core settings
enabled: bool | *true

// FFmpeg configuration
ffmpeg: {
	// Path to FFmpeg executable
	path: string | *"ffmpeg"
	
	// Encoding preset (ultrafast, superfast, veryfast, faster, fast, medium, slow, slower, veryslow)
	preset: string | *"fast"
	
	// Number of threads (0 = auto)
	threads: int | *0
	
	// Plugin priority (higher = preferred)
	priority: int | *50
}

// Content-aware quality profiles
quality_profiles: {
	// Profile for 4K HDR content (movies, premium TV shows)
	"4k_hdr": {
		target_resolution: "2160p"
		crf_h264: 22.0
		crf_hevc: 26.0
		max_bitrate: 35000
		buffer_size: 70000
		preset: "slow"
		tune: "film"
		hdr_handling: "preserve"
		audio_bitrate: 256
		audio_codec: "eac3"
	}
	
	// Profile for 4K SDR content
	"4k_sdr": {
		target_resolution: "2160p"
		crf_h264: 23.0
		crf_hevc: 27.0
		max_bitrate: 25000
		buffer_size: 50000
		preset: "medium"
		tune: "film"
		hdr_handling: "none"
		audio_bitrate: 192
		audio_codec: "aac"
	}
	
	// Profile for 1080p content (standard TV shows, movies)
	"1080p": {
		target_resolution: "1080p"
		crf_h264: 23.0
		crf_hevc: 28.0
		max_bitrate: 8000
		buffer_size: 16000
		preset: "fast"
		tune: "film"
		hdr_handling: "tonemap"
		audio_bitrate: 128
		audio_codec: "aac"
	}
	
	// Profile for 720p (mobile, limited bandwidth)
	"720p": {
		target_resolution: "720p"
		crf_h264: 24.0
		crf_hevc: 29.0
		max_bitrate: 4000
		buffer_size: 8000
		preset: "fast"
		tune: "film"
		hdr_handling: "tonemap"
		audio_bitrate: 96
		audio_codec: "aac"
	}
	
	// Profile for 480p (very limited bandwidth, older devices)
	"480p": {
		target_resolution: "480p"
		crf_h264: 25.0
		crf_hevc: 30.0
		max_bitrate: 2000
		buffer_size: 4000
		preset: "fast"
		tune: "film"
		hdr_handling: "tonemap"
		audio_bitrate: 64
		audio_codec: "aac"
	}
}

// Device-specific optimization profiles
device_profiles: {
	// Web browsers - modern codec support
	"web_modern": {
		preferred_codecs: ["av1", "hevc", "h264"]
		containers: ["dash", "hls"]
		hdr_support: true
		max_resolution: "2160p"
		adaptive_bitrate: true
	}
	
	// Web browsers - legacy support
	"web_legacy": {
		preferred_codecs: ["h264"]
		containers: ["dash", "hls"]
		hdr_support: false
		max_resolution: "1080p"
		adaptive_bitrate: true
	}
	
	// Roku devices
	"roku": {
		preferred_codecs: ["hevc", "h264"]
		containers: ["hls", "mp4"]
		hdr_support: true
		max_resolution: "2160p"
		adaptive_bitrate: true
		audio_passthrough: ["ac3", "eac3", "dts"]
	}
	
	// NVIDIA Shield
	"nvidia_shield": {
		preferred_codecs: ["av1", "hevc", "h264"]
		containers: ["mkv", "mp4", "dash", "hls"]
		hdr_support: true
		max_resolution: "2160p"
		adaptive_bitrate: true
		audio_passthrough: ["truehd", "dts", "ac3", "eac3"]
		hardware_decode: true
	}
	
	// Apple TV
	"apple_tv": {
		preferred_codecs: ["hevc", "h264"]
		containers: ["hls", "mp4"]
		hdr_support: true
		max_resolution: "2160p"
		adaptive_bitrate: true
		audio_passthrough: ["ac3", "eac3"]
	}
	
	// Android TV / Google TV
	"android_tv": {
		preferred_codecs: ["av1", "hevc", "h264"]
		containers: ["dash", "hls", "mp4"]
		hdr_support: true
		max_resolution: "2160p"
		adaptive_bitrate: true
		audio_passthrough: ["ac3", "eac3", "dts"]
	}
}

// Quality settings
quality: {
	// CRF values for different codecs (lower = higher quality)
	crf_h264: number | *23.0
	crf_hevc: number | *28.0
	crf_av1: number | *32.0
	
	// Bitrate multipliers for adaptive streaming
	max_bitrate_multiplier: number | *1.5
	buffer_size_multiplier: number | *2.0
	
	// Quality vs speed balance (0-3, higher = slower but better quality)
	quality_speed_balance: int | *1
}

// Audio settings
audio: {
	// Default audio codec
	codec: string | *"aac"
	
	// Default audio bitrate (kbps)
	bitrate: int | *128
	
	// Audio sample rate (Hz)
	sample_rate: int | *48000
	
	// Number of audio channels
	channels: int | *2
	
	// Audio passthrough for compatible devices
	passthrough_codecs: [...string] | *["ac3", "eac3", "dts"]
	
	// Normalize audio levels
	normalize: bool | *true
}

// Subtitle settings
subtitles: {
	// Codec for burning subtitles into video
	burn_in_codec: string | *"subtitles"
	
	// Codec for soft subtitles
	soft_codec: string | *"mov_text"
	
	// Auto-detect and extract embedded subtitles
	extract_embedded: bool | *true
	
	// Languages to prioritize for subtitle extraction
	preferred_languages: [...string] | *["en", "eng"]
}

// Performance settings
performance: {
	// Maximum number of concurrent transcoding jobs
	max_concurrent_jobs: int | *25
	
	// Timeout for transcoding jobs (seconds)
	timeout_seconds: int | *7200
	
	// Clean up temporary files on exit
	cleanup_on_exit: bool | *true
	
	// Segment duration for DASH/HLS (seconds)
	segment_duration: int | *6
	
	// Look-ahead segments for adaptive streaming
	lookahead_segments: int | *3
}

// Logging configuration
logging: {
	// Log level (debug, info, warn, error)
	level: string | *"info"
	
	// Include FFmpeg output in logs
	ffmpeg_output: bool | *false
	
	// Log transcoding statistics
	log_stats: bool | *true
}

// Hardware acceleration settings
hardware: {
	// Enable hardware acceleration
	enabled: bool | *false
	
	// Hardware acceleration type (nvenc, vaapi, qsv, videotoolbox)
	type: string | *"auto"
	
	// Hardware device index
	device: int | *0
	
	// Fallback to software encoding if hardware fails
	fallback_to_software: bool | *true
}

// Codec-specific settings
codecs: {
	h264: {
		// H.264 specific options
		profile: string | *"high"
		level: string | *"4.1"
		tune: string | *"film"
		
		// Two-pass encoding for better quality
		two_pass: bool | *false
		
		// B-frame settings
		bframes: int | *3
		
		// Reference frames
		refs: int | *3
	}
	
	hevc: {
		// HEVC specific options
		profile: string | *"main"
		tier: string | *"main"
		tune: string | *"grain"
		
		// Two-pass encoding
		two_pass: bool | *false
		
		// CTU size (16, 32, 64)
		ctu_size: int | *32
		
		// Range extension for 10-bit content
		range_extension: bool | *true
	}
	
	av1: {
		// AV1 specific options
		cpu_used: int | *4
		tile_columns: int | *1
		tile_rows: int | *0
		
		// Two-pass encoding (recommended for AV1)
		two_pass: bool | *true
		
		// Film grain synthesis
		film_grain: bool | *true
	}
	
	vp9: {
		// VP9 specific options
		quality: string | *"good"
		cpu_used: int | *1
		
		// Two-pass encoding
		two_pass: bool | *true
		
		// Row-based multithreading
		row_mt: bool | *true
	}
}

// Resolution and bitrate presets
resolutions: {
	"480p": {
		width: 854
		height: 480
		bitrate: 1500
		min_bitrate: 750
		max_bitrate: 2250
	}
	
	"720p": {
		width: 1280
		height: 720
		bitrate: 3000
		min_bitrate: 1500
		max_bitrate: 4500
	}
	
	"1080p": {
		width: 1920
		height: 1080
		bitrate: 6000
		min_bitrate: 3000
		max_bitrate: 9000
	}
	
	"1440p": {
		width: 2560
		height: 1440
		bitrate: 12000
		min_bitrate: 6000
		max_bitrate: 18000
	}
	
	"2160p": {
		width: 3840
		height: 2160
		bitrate: 25000
		min_bitrate: 12500
		max_bitrate: 37500
	}
}

// Content detection rules
content_detection: {
	// HDR detection keywords in filename
	hdr_keywords: [...string] | *["HDR10", "HDR", "DV", "Dolby Vision", "Dolby.Vision"]
	
	// High quality content indicators
	remux_keywords: [...string] | *["Remux", "REMUX", "BluRay", "Blu-ray", "UHD"]
	
	// Web content indicators  
	web_keywords: [...string] | *["WEBDL", "WEB-DL", "WEBRip", "WEB", "Netflix", "Amazon", "Hulu", "Disney"]
	
	// TV show vs movie detection
	tv_indicators: [...string] | *["S01E", "S02E", "Season", "Episode"]
}

// Adaptive streaming settings
adaptive: {
	// Enable adaptive bitrate streaming
	enabled: bool | *true
	
	// Bitrate ladder (kbps)
	bitrate_ladder: [...int] | *[500, 1000, 2000, 4000, 8000, 16000, 25000]
	
	// Resolution ladder for each bitrate
	resolution_ladder: {
		"500": "480p"
		"1000": "720p"
		"2000": "720p"
		"4000": "1080p"
		"8000": "1080p"
		"16000": "1440p"
		"25000": "2160p"
	}
	
	// Segment duration (seconds)
	segment_duration: int | *6
	
	// Playlist update frequency (seconds)
	playlist_update_interval: int | *30
}

// File cleanup configuration
cleanup: {
	// File retention hours (active window)
	file_retention_hours: int | *3
	
	// Extended retention for smaller files (hours)
	extended_retention_hours: int | *12
	
	// Maximum total size before emergency cleanup (GB)
	max_size_limit_gb: int | *15
	
	// File size threshold for "large" files (MB)
	large_file_size_mb: int | *1000
	
	// Cleanup interval (minutes)
	cleanup_interval_minutes: int | *30
	
	// Keep segments for active sessions longer
	active_session_retention_hours: int | *6
}

// Health monitoring thresholds
health: {
	// Maximum memory usage (bytes)
	max_memory_usage: int | *2147483648 // 2GB
	
	// Maximum CPU usage (percentage)
	max_cpu_usage: number | *90.0
	
	// Maximum error rate (percentage)
	max_error_rate: number | *10.0
	
	// Maximum response time (milliseconds)
	max_response_time: int | *45000
	
	// Health check interval (seconds)
	check_interval: int | *60
	
	// Maximum concurrent sessions before throttling
	max_concurrent_sessions: int | *25
}

// Feature flags
features: {
	// Enable subtitle burn-in
	subtitle_burn_in: bool | *true
	
	// Enable subtitle passthrough
	subtitle_passthrough: bool | *true
	
	// Enable multi-audio track support
	multi_audio_tracks: bool | *true
	
	// Enable HDR tone mapping
	hdr_tone_mapping: bool | *true
	
	// Enable Dolby Vision processing
	dolby_vision_support: bool | *false
	
	// Enable content-aware encoding
	content_aware_encoding: bool | *true
	
	// Enable two-pass encoding for high quality
	two_pass_encoding: bool | *false
	
	// Enable advanced audio processing
	advanced_audio_processing: bool | *true
	
	// Enable smart bitrate ladder selection
	smart_bitrate_ladder: bool | *true
}

// Advanced filter settings
filters: {
	// Noise reduction for lower quality sources
	denoise: {
		enabled: bool | *false
		strength: number | *0.5
	}
	
	// Sharpening for upscaled content
	sharpen: {
		enabled: bool | *false
		strength: number | *0.3
	}
	
	// Deinterlacing for interlaced sources
	deinterlace: {
		enabled: bool | *true
		method: string | *"yadif"
	}
	
	// Color correction
	color_correction: {
		enabled: bool | *false
		saturation: number | *1.0
		contrast: number | *1.0
		brightness: number | *0.0
	}
} 