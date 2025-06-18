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

// Quality settings
quality: {
	// CRF values for different codecs
	crf_h264: number | *23.0
	crf_hevc: number | *28.0
	
	// Bitrate multipliers
	max_bitrate_multiplier: number | *1.5
	buffer_size_multiplier: number | *2.0
}

// Audio settings
audio: {
	// Default audio codec
	codec: string | *"aac"
	
	// Default audio bitrate (kbps)
	bitrate: int | *128
	
	// Audio sample rate (Hz)
	sample_rate: int | *44100
	
	// Number of audio channels
	channels: int | *2
}

// Subtitle settings
subtitles: {
	// Codec for burning subtitles into video
	burn_in_codec: string | *"subtitles"
	
	// Codec for soft subtitles
	soft_codec: string | *"mov_text"
}

// Performance settings
performance: {
	// Maximum number of concurrent transcoding jobs
	max_concurrent_jobs: int | *25
	
	// Timeout for transcoding jobs (seconds)
	timeout_seconds: int | *3600
	
	// Clean up temporary files on exit
	cleanup_on_exit: bool | *true
}

// Logging configuration
logging: {
	// Log level (debug, info, warn, error)
	level: string | *"info"
	
	// Include FFmpeg output in logs
	ffmpeg_output: bool | *false
}

// Hardware acceleration (future expansion)
hardware: {
	// Enable hardware acceleration
	enabled: bool | *false
	
	// Hardware acceleration type (nvenc, vaapi, qsv)
	type: string | *"none"
	
	// Hardware device index
	device: int | *0
}

// Codec-specific settings
codecs: {
	h264: {
		// H.264 specific options
		profile: string | *"high"
		level: string | *"4.1"
		tune: string | *"film"
	}
	
	hevc: {
		// HEVC specific options
		profile: string | *"main"
		tier: string | *"main"
		tune: string | *"grain"
	}
	
	vp9: {
		// VP9 specific options
		quality: string | *"good"
		cpu_used: int | *0
	}
	
	av1: {
		// AV1 specific options
		cpu_used: int | *4
		tile_columns: int | *0
		tile_rows: int | *0
	}
}

// Resolution presets
resolutions: {
	"480p": {
		width: 854
		height: 480
		bitrate: 1500
	}
	
	"720p": {
		width: 1280
		height: 720
		bitrate: 3000
	}
	
	"1080p": {
		width: 1920
		height: 1080
		bitrate: 6000
	}
	
	"1440p": {
		width: 2560
		height: 1440
		bitrate: 12000
	}
	
	"2160p": {
		width: 3840
		height: 2160
		bitrate: 25000
	}
}

// Health monitoring thresholds
health: {
	// Maximum memory usage (bytes)
	max_memory_usage: int | *1073741824 // 1GB
	
	// Maximum CPU usage (percentage)
	max_cpu_usage: number | *80.0
	
	// Maximum error rate (percentage)
	max_error_rate: number | *5.0
	
	// Maximum response time (milliseconds)
	max_response_time: int | *30000
	
	// Health check interval (seconds)
	check_interval: int | *60
}

// Feature flags
features: {
	// Enable subtitle burn-in
	subtitle_burn_in: bool | *true
	
	// Enable subtitle passthrough
	subtitle_passthrough: bool | *true
	
	// Enable multi-audio track support
	multi_audio_tracks: bool | *true
	
	// Enable HDR support (future)
	hdr_support: bool | *false
	
	// Enable tone mapping (future)
	tone_mapping: bool | *false
	
	// Enable streaming output
	streaming_output: bool | *true
	
	// Enable segmented output for DASH/HLS
	segmented_output: bool | *true
}

// Validation constraints
#validate: {
	// Ensure preset is valid
	preset: "ultrafast" | "superfast" | "veryfast" | "faster" | "fast" | "medium" | "slow" | "slower" | "veryslow"
	
	// Ensure threads is reasonable
	threads: >=0 & <=32
	
	// Ensure priority is reasonable
	priority: >=1 & <=100
	
	// Ensure CRF values are valid
	crf_h264: >=0 & <=51
	crf_hevc: >=0 & <=51
	
	// Ensure bitrate multipliers are reasonable
	max_bitrate_multiplier: >=1.0 & <=5.0
	buffer_size_multiplier: >=1.0 & <=10.0
	
	// Ensure audio settings are valid
	audio_bitrate: >=32 & <=512
	audio_sample_rate: 8000 | 16000 | 22050 | 44100 | 48000 | 96000
	audio_channels: >=1 & <=8
	
	// Ensure performance settings are reasonable
	max_concurrent_jobs: >=1 & <=15
	timeout_seconds: >=60 & <=7200
	
	// Ensure log level is valid
	log_level: "debug" | "info" | "warn" | "error"
} 