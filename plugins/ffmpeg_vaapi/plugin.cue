plugin_name: "ffmpeg_vaapi"
plugin_type: "transcoding"
version: "1.0.0"
enabled: true

plugin_info: {
	name: "FFmpeg VAAPI Transcoder"
	description: "Intel VAAPI hardware-accelerated transcoding for Intel GPUs"
	author: "Viewra Team"
	website: "https://github.com/mantonx/viewra"
}

requirements: {
	ffmpeg: {
		version: ">=4.4"
		encoders: ["h264_vaapi", "hevc_vaapi", "vp9_vaapi"]
	}
	hardware: {
		intel_gpu: true
		vaapi_support: true
	}
	system: {
		platforms: ["linux"]
	}
}

transcoding_config: {
	priority: 80
	hardware_acceleration: "vaapi"
	max_concurrent_sessions: 3
	
	supported_codecs: {
		video: ["h264_vaapi", "hevc_vaapi", "vp9_vaapi"]
		audio: ["aac", "opus", "mp3"]
	}
	
	supported_containers: ["mp4", "mkv", "webm", "dash", "hls"]
	
	quality_presets: {
		ultra_fast: {
			name: "Ultra Fast"
			description: "Maximum speed encoding for real-time applications"
			vaapi_preset: "ultrafast"
			quality_rating: 40
			speed_rating: 10
		}
		fast: {
			name: "Fast"
			description: "Fast encoding with good quality balance"
			vaapi_preset: "fast"
			quality_rating: 60
			speed_rating: 8
		}
		balanced: {
			name: "Balanced"
			description: "Balanced speed and quality for general use"
			vaapi_preset: "medium"
			quality_rating: 75
			speed_rating: 6
		}
		quality: {
			name: "Quality"
			description: "High quality encoding with slower speed"
			vaapi_preset: "slow"
			quality_rating: 85
			speed_rating: 4
		}
	}
}

dashboard_config: {
	sections: [
		{
			id: "overview"
			title: "VAAPI Transcoder Overview"
			type: "stats"
			metrics: ["active_sessions", "completed_jobs", "gpu_usage", "power_usage"]
		},
		{
			id: "gpu_usage"
			title: "Intel GPU Utilization"
			type: "chart"
			chart_type: "area"
			metrics: ["gpu_usage", "memory_usage", "power_consumption"]
		}
	]
}