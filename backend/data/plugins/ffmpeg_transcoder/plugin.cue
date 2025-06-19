package ffmpeg_transcoder

#Plugin: {
	id:          "ffmpeg_transcoder"
	name:        "FFmpeg Transcoder"
	version:     "1.0.0"
	type:        "transcoder"
	description: "FFmpeg-based video transcoding service with comprehensive codec support and streaming capabilities"
	author:      "Viewra Team"
	enabled_by_default: true
	
	// Entry points for plugin execution
	entry_points: {
		main: "ffmpeg_transcoder"
	}
	
	settings: {
		// Plugin enabled/disabled
		enabled: bool | *true @ui(importance=10,level="basic",category="General")
		
		// Plugin priority (higher = preferred)
		priority: int | *50 @ui(importance=7,level="basic",category="General")
		
		// FFmpeg executable path
		ffmpeg_path: string | *"ffmpeg" @ui(importance=6,level="basic",category="General")
		
		// Encoding preset for speed vs quality balance
		preset: string | *"fast" @ui(importance=9,level="basic",category="Quality")
		
		// H.264 CRF value (lower = higher quality, 18-28 recommended)
		crf_h264: number | *23.0 @ui(importance=8,level="basic",category="Quality")
		
		// Quality vs speed balance (0-3, higher = slower but better quality)
		quality_speed_balance: int | *1 @ui(importance=6,level="basic",category="Quality")
		
		// Maximum concurrent transcoding jobs
		max_concurrent_jobs: int | *25 @ui(importance=8,level="basic",category="Performance")
		
		// Job timeout in seconds
		timeout_seconds: int | *7200 @ui(importance=6,level="basic",category="Performance")
		
		// Enable hardware acceleration (always auto mode)
		hardware_acceleration: bool | *true @ui(importance=7,level="basic",category="Hardware")
		
		// Hardware acceleration type (always auto - let FFmpeg choose best available)
		hardware_type: string | *"auto" @ui(importance=4,level="advanced",category="Hardware")
		
		// Fallback to software if hardware fails (compatibility setting)
		hardware_fallback: bool | *true @ui(importance=3,level="advanced",category="Hardware")
		
		// Default audio bitrate (kbps)
		audio_bitrate: int | *128 @ui(importance=6,level="basic",category="Audio")
		
		// Extract embedded subtitles
		extract_subtitles: bool | *true @ui(importance=5,level="basic",category="Subtitles")
		
		// Advanced: Number of threads (0 = auto)
		threads: int | *0 @ui(importance=4,level="advanced",category="Performance")
		
		// Advanced: Audio codec
		audio_codec: string | *"aac" @ui(importance=5,level="advanced",category="Audio")
		
		// Advanced: Audio sample rate
		audio_sample_rate: int | *48000 @ui(importance=3,level="advanced",category="Audio")
		
		// Advanced: Audio channels
		audio_channels: int | *2 @ui(importance=4,level="advanced",category="Audio")
		
		// Advanced: Normalize audio levels
		normalize_audio: bool | *true @ui(importance=4,level="advanced",category="Audio")
		
		// Advanced: H.264 profile
		h264_profile: string | *"high" @ui(importance=3,level="advanced",category="Codecs")
		
		// Advanced: H.264 level
		h264_level: string | *"4.1" @ui(importance=3,level="advanced",category="Codecs")
		
		// Advanced: H.264 tune
		h264_tune: string | *"film" @ui(importance=3,level="advanced",category="Codecs")
		
		// Advanced: Two-pass encoding for better quality
		two_pass_encoding: bool | *false @ui(importance=4,level="advanced",category="Quality")
		
		// Advanced: B-frames for H.264
		h264_bframes: int | *3 @ui(importance=3,level="advanced",category="Codecs")
		
		// Advanced: Reference frames for H.264
		h264_refs: int | *3 @ui(importance=3,level="advanced",category="Codecs")
		
		// Advanced: Subtitle burn-in codec
		subtitle_burn_codec: string | *"subtitles" @ui(importance=3,level="advanced",category="Subtitles")
		
		// Advanced: Soft subtitle codec
		subtitle_soft_codec: string | *"mov_text" @ui(importance=3,level="advanced",category="Subtitles")
		
		// Advanced: Preferred subtitle languages
		subtitle_languages: [...string] | *["en", "eng"] @ui(importance=4,level="advanced",category="Subtitles")
		
		// Advanced: Log level
		log_level: string | *"info" @ui(importance=4,level="advanced",category="Logging")
		
		// Advanced: Include FFmpeg output in logs
		log_ffmpeg_output: bool | *false @ui(importance=3,level="advanced",category="Logging")
		
		// Advanced: Log transcoding statistics
		log_stats: bool | *true @ui(importance=3,level="advanced",category="Logging")
		
		// Advanced: File retention hours
		file_retention_hours: int | *3 @ui(importance=5,level="advanced",category="Cleanup")
		
		// Advanced: Maximum storage size (GB)
		max_storage_gb: int | *15 @ui(importance=6,level="advanced",category="Cleanup")
		
		// Advanced: Cleanup interval (minutes)
		cleanup_interval_minutes: int | *30 @ui(importance=4,level="advanced",category="Cleanup")
		
		// Advanced: Enable deinterlacing
		enable_deinterlace: bool | *true @ui(importance=4,level="advanced",category="Filters")
		
		// Advanced: Deinterlacing method
		deinterlace_method: string | *"yadif" @ui(importance=3,level="advanced",category="Filters")
		
		// Advanced: Enable noise reduction
		enable_denoise: bool | *false @ui(importance=3,level="advanced",category="Filters")
		
		// Advanced: Noise reduction strength
		denoise_strength: number | *0.5 @ui(importance=3,level="advanced",category="Filters")
		
		// Advanced: Enable sharpening
		enable_sharpen: bool | *false @ui(importance=3,level="advanced",category="Filters")
		
		// Advanced: Sharpening strength
		sharpen_strength: number | *0.3 @ui(importance=3,level="advanced",category="Filters")
	}
}
