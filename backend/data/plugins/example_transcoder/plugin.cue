// Example Transcoder Plugin Configuration
// This demonstrates how to create a new transcoding provider

id:             "example_transcoder"
name:           "Example Transcoder"
version:        "1.0.0"
description:    "Example implementation of a transcoding provider"
author:         "Viewra Team"
license:        "MIT"
repository:     "https://github.com/mantonx/viewra"
homepage:       "https://viewra.app"
documentation:  "https://docs.viewra.app/plugins/example-transcoder"

// Plugin type must be "transcoder" for transcoding providers
type: "transcoder"

// Required binaries for this transcoder
dependencies: {
	binaries: ["example-encoder", "example-probe"]
	libraries: []
	services: []
}

// Entry points
entry_points: {
	main: "example_transcoder"
}

// Plugin capabilities
capabilities: {
	transcoding: {
		codecs: ["h264", "h265", "av1"]
		containers: ["mp4", "mkv", "webm"]
		hardware_acceleration: ["example_gpu"]
		max_concurrent: 5
		features: {
			subtitle_burn_in: true
			multi_audio: true
			hdr_support: false
		}
	}
}

// Configuration schema
settings: {
	// Core settings (required for all transcoders)
	enabled:          bool | *true     @ui(basic,importance=10)
	priority:         int | *50        @ui(basic,importance=9) 
	output_directory: string | *"/viewra-data/transcoding" @ui(basic,importance=8)

	// Hardware settings
	hw_enabled:       bool | *false    @ui(basic,importance=7)
	hw_device:        string | *"auto" @ui(advanced,importance=5)

	// Session management
	max_sessions:     int | *5         @ui(basic,importance=6)
	session_timeout:  int | *120       @ui(advanced,importance=4)

	// Example-specific settings
	encoder_path:     string | *"example-encoder" @ui(advanced,importance=3)
	probe_path:       string | *"example-probe"   @ui(advanced,importance=3)
	
	// Quality defaults (mapped from 0-100%)
	quality_mappings: {
		low:    int | *30    @ui(advanced,importance=2)
		medium: int | *50    @ui(advanced,importance=2)
		high:   int | *70    @ui(advanced,importance=2)
		ultra:  int | *90    @ui(advanced,importance=2)
	}

	// Debug settings
	debug:        bool | *false @ui(advanced,importance=1)
	log_commands: bool | *false @ui(advanced,importance=1)
} 