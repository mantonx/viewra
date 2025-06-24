plugin_name: "ffmpeg_nvidia"
plugin_type: "transcoding"
version: "1.0.0"
enabled: true

plugin_info: {
	name: "FFmpeg NVIDIA Transcoder"
	description: "High-performance NVIDIA NVENC hardware-accelerated transcoding"
	author: "Viewra Team"
	website: "https://github.com/mantonx/viewra"
}

requirements: {
	ffmpeg: {
		version: ">=4.4"
		encoders: ["h264_nvenc", "hevc_nvenc"]
	}
	hardware: {
		nvidia_gpu: true
		driver_version: ">=470.0"
	}
}

transcoding_config: {
	priority: 90
	hardware_acceleration: "nvenc"
	max_concurrent_sessions: 4
	
	supported_codecs: {
		video: ["h264_nvenc", "hevc_nvenc", "av1_nvenc"]
		audio: ["aac", "opus", "mp3"]
	}
	
	supported_containers: ["mp4", "mkv", "webm", "dash", "hls"]
	
	quality_presets: {
		p1: {
			name: "Fastest (P1)"
			description: "Maximum speed, lowest quality - best for real-time"
			nvenc_preset: "p1"
			quality_rating: 30
			speed_rating: 10
		}
		p4: {
			name: "Fast (P4)"  
			description: "Good speed with acceptable quality"
			nvenc_preset: "p4"
			quality_rating: 50
			speed_rating: 8
		}
		p6: {
			name: "Balanced (P6)"
			description: "Balanced speed and quality for most use cases"
			nvenc_preset: "p6"
			quality_rating: 70
			speed_rating: 6
		}
		p7: {
			name: "Quality (P7)"
			description: "High quality encoding with slower speed"
			nvenc_preset: "p7"
			quality_rating: 85
			speed_rating: 3
		}
	}
}

dashboard_config: {
	sections: [
		{
			id: "overview"
			title: "NVIDIA Transcoder Overview"
			type: "stats"
			metrics: ["active_sessions", "completed_jobs", "gpu_usage", "encoder_usage"]
		},
		{
			id: "gpu_usage"
			title: "GPU Utilization"
			type: "chart"
			chart_type: "line"
			metrics: ["gpu_usage", "encoder_usage", "memory_usage"]
		},
		{
			id: "performance"
			title: "Performance Metrics"
			type: "stats"
			metrics: ["avg_fps", "throughput", "efficiency"]
		}
	]
}