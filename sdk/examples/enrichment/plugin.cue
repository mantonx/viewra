plugin_name: "example_enricher"
plugin_type: "enrichment"
version: "1.0.0"
enabled: true

description: "Example enrichment plugin template"
author: "Viewra Team"

config: {
	// Example configuration schema
	api_key?: string
	base_url: string | *"https://api.example.com"
	timeout: int | *30
	max_retries: int | *3
	
	features: {
		enable_caching: bool | *true
		cache_duration: int | *3600
	}
}

// Plugin capabilities
capabilities: [
	"metadata_enrichment",
	"async_processing"
]

// Requirements
requirements: {
	network_access: true
	disk_space_mb: 10
	memory_mb: 64
}