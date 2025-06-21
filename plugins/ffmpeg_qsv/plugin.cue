plugin_name: "ffmpeg_qsv"
plugin_type: "transcoding"
version: "1.0.0"
enabled: true

plugin_info: {
	name: "FFmpeg QSV Transcoder"
	description: "Intel Quick Sync Video hardware acceleration for fast transcoding"
	author: "Viewra Team"
	website: "https://github.com/mantonx/viewra"
}

requirements: {
	ffmpeg: {
		version: ">=4.4"
		encoders: ["h264_qsv", "hevc_qsv", "vp9_qsv", "av1_qsv"]
	}
	hardware: {
		intel_cpu: true
		qsv_support: true
		generation: ">=7th_gen" // 7th gen Intel or newer
	}
}

transcoding_config: {
	priority: 85
	hardware_acceleration: "qsv"
	max_concurrent_sessions: 6
	
	supported_codecs: {
		video: ["h264_qsv", "hevc_qsv", "vp9_qsv", "av1_qsv"]
		audio: ["aac", "opus", "mp3"]
	}
	
	supported_containers: ["mp4", "mkv", "webm", "dash", "hls"]
	
	quality_presets: {
		speed: {
			name: "Speed"
			description: "Optimized for maximum encoding speed"
			qsv_preset: "veryfast"
			quality_rating: 45
			speed_rating: 10
		}
		balanced: {
			name: "Balanced"
			description: "Good balance of speed, quality, and file size"
			qsv_preset: "medium"
			quality_rating: 65
			speed_rating: 7
		}
		quality: {
			name: "Quality"
			description: "Optimized for better quality with moderate speed"
			qsv_preset: "slow"
			quality_rating: 80
			speed_rating: 5
		}
		veryslow: {
			name: "Best Quality"
			description: "Maximum quality encoding with slower speed"
			qsv_preset: "veryslow"
			quality_rating: 90
			speed_rating: 3
		}
	}
}

dashboard_config: {
	sections: [
		{
			id: "overview"
			title: "QSV Transcoder Overview"
			type: "stats"
			metrics: ["active_sessions", "completed_jobs", "qsv_usage", "throughput"]
		},
		{
			id: "performance"
			title: "QSV Performance Metrics"
			type: "chart"
			chart_type: "line"
			metrics: ["fps_current", "fps_average", "fps_peak", "latency"]
		}
	]
}