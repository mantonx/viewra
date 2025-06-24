#Plugin: {
	schema_version: "1.0"

	// Plugin identification
	id:          "ffmpeg_software"
	name:        "FFmpeg Software Transcoder"
	version:     "1.0.0"
	description: "High-quality software-only video transcoding using FFmpeg"
	author:      "Viewra Team"
	website:     "https://github.com/mantonx/viewra"
	repository:  "https://github.com/mantonx/viewra"
	license:     "MIT"
	type:        "transcoder"
	tags: [
		"video",
		"transcoding",
		"ffmpeg",
		"software",
		"compatibility"
	]

	// Plugin behavior
	enabled_by_default: true

	// Plugin capabilities
	capabilities: {
		transcoding:         true
		hardware_accel:      false  // Software only
		streaming:           true
		adaptive_streaming:  true
		database_access:     false
		background_tasks:    true
		external_services:   false
		asset_management:    true
	}

	// Entry points
	entry_points: {
		main: "ffmpeg_software"
	}

	// Permissions
	permissions: [
		"filesystem:read",
		"filesystem:write",
		"process:execute"
	]

	// Plugin-specific settings
	settings: {
		// Core transcoding settings
		core: {
			enabled: bool | *true 
				@ui(title="Enable Plugin", importance=10, is_basic=true)
			priority: int & >=1 & <=100 | *50 
				@ui(title="Plugin Priority", importance=9, is_basic=true)
			output_directory: string | *"/viewra-data/transcoding" 
				@ui(title="Output Directory", importance=8, is_basic=true)
		}

		// Software encoding settings
		encoding: {
			preset: "ultrafast" | "superfast" | "veryfast" | "faster" | "fast" | "medium" | "slow" | "slower" | "veryslow" | *"medium"
				@ui(title="Encoding Preset", importance=10, is_basic=true)
			
			threads: int & >=0 & <=32 | *0
				@ui(title="Thread Count (0=auto)", importance=8)
			
			// Quality defaults
			default_crf: {
				h264: int & >=0 & <=51 | *23
					@ui(title="H.264 Default CRF", importance=7, is_basic=true)
				h265: int & >=0 & <=51 | *28
					@ui(title="H.265 Default CRF", importance=6)
				vp9: int & >=0 & <=63 | *31
					@ui(title="VP9 Default CRF", importance=5)
			}
		}

		// Session management
		sessions: {
			max_concurrent: int & >=1 & <=20 | *5
				@ui(title="Max Concurrent Sessions", importance=10, is_basic=true)
			timeout_minutes: int & >=1 & <=1440 | *120
				@ui(title="Session Timeout (minutes)", importance=9)
		}

		// Debug settings
		debug: {
			enabled: bool | *false
				@ui(title="Enable Debug Mode", importance=10)
			log_level: "debug" | "info" | "warn" | "error" | *"info"
				@ui(title="Log Level", importance=9)
		}
	}
}